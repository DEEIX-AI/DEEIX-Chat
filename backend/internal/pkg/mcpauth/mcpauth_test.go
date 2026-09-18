package mcpauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// verifyForTest 在测试内独立实现校验逻辑，与生产包保持解耦，
// 覆盖签名往返、篡改与过期三种场景。
func verifyForTest(secret string, token string, now time.Time) (Payload, error) {
	if !strings.HasPrefix(token, "v1.") {
		return Payload{}, errors.New("bad prefix")
	}
	parts := strings.SplitN(strings.TrimPrefix(token, "v1."), ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Payload{}, errors.New("bad format")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return Payload{}, errors.New("bad signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Payload{}, err
	}
	var payload Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Payload{}, err
	}
	if now.Unix() >= payload.ExpiresAt {
		return Payload{}, errors.New("expired")
	}
	return payload, nil
}

func TestSignVerifyRoundTrip(t *testing.T) {
	secret := "test-secret"
	payload := Payload{
		UserID:         42,
		ConversationID: 7,
		RequestID:      "req_1",
		ExpiresAt:      time.Now().Add(DefaultTTL).Unix(),
	}
	token, err := Sign(secret, payload)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	if !strings.HasPrefix(token, "v1.") {
		t.Fatalf("expected token prefix v1., got %q", token)
	}
	got, err := verifyForTest(secret, token, time.Now())
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if got.UserID != payload.UserID || got.ConversationID != payload.ConversationID || got.RequestID != payload.RequestID {
		t.Fatalf("unexpected payload: %#v", got)
	}
}

func TestSignProducesFreshExpiry(t *testing.T) {
	payload := Payload{UserID: 1, ExpiresAt: time.Now().Add(DefaultTTL).Unix()}
	first, err := Sign("secret", payload)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	second, err := Sign("secret", payload)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	if first != second {
		t.Fatalf("expected deterministic signing for identical payload, got different tokens")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	payload := Payload{UserID: 1, ExpiresAt: time.Now().Add(DefaultTTL).Unix()}
	token, err := Sign("secret-a", payload)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	if _, err := verifyForTest("secret-b", token, time.Now()); err == nil {
		t.Fatal("expected verification to fail for wrong secret")
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	payload := Payload{UserID: 1, ExpiresAt: time.Now().Add(DefaultTTL).Unix()}
	token, err := Sign("secret", payload)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	parts := strings.SplitN(strings.TrimPrefix(token, "v1."), ".", 2)
	tampered := "v1.eyJ1c2VyX2lkIjoyfQ." + parts[1]
	if _, err := verifyForTest("secret", tampered, time.Now()); err == nil {
		t.Fatal("expected verification to fail for tampered payload")
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	payload := Payload{UserID: 1, ExpiresAt: time.Now().Add(-time.Minute).Unix()}
	token, err := Sign("secret", payload)
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	if _, err := verifyForTest("secret", token, time.Now()); err == nil {
		t.Fatal("expected verification to fail for expired token")
	}
}

func TestVerifyRejectsMalformedToken(t *testing.T) {
	cases := []string{
		"",
		"garbage",
		"v1.",
		"v1.abc",
		"v1.abc.def.extra",
	}
	for _, token := range cases {
		if _, err := verifyForTest("secret", token, time.Now()); err == nil {
			t.Fatalf("expected error for token %q", token)
		}
	}
}

func TestSignRejectsEmptySecret(t *testing.T) {
	if _, err := Sign("", Payload{UserID: 1, ExpiresAt: 1}); err == nil {
		t.Fatal("expected error for empty secret")
	}
	if _, err := Sign("  ", Payload{UserID: 1, ExpiresAt: 1}); err == nil {
		t.Fatal("expected error for blank secret")
	}
}
