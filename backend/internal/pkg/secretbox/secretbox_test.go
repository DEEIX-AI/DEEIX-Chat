package secretbox

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

const (
	testKey    = "test-data-encryption-key-0123456789abcdef"
	testOldKey = "old-data-encryption-key-0123456789abcdef"
)

func mustKeyring(t *testing.T, active string, previous ...string) *Keyring {
	t.Helper()
	ring, err := NewKeyring(active, previous...)
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	return ring
}

func legacyAEAD(t *testing.T, secret string) cipher.AEAD {
	t.Helper()
	key := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	return gcm
}

// legacyEncrypt 复刻引入 Keyring 之前的实现，确认存量数据仍可读取。
func legacyEncrypt(t *testing.T, secret string, plaintext string) string {
	t.Helper()
	gcm := legacyAEAD(t, secret)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	return prefix + base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil))
}

// legacyDecrypt 同上：确认新写入的数据仍能被旧版本读取，升级后可以回退。
func legacyDecrypt(t *testing.T, secret string, payload string) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(payload, prefix))
	if err != nil {
		t.Fatal(err)
	}
	gcm := legacyAEAD(t, secret)
	plaintext, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		t.Fatalf("the previous implementation cannot read the payload: %v", err)
	}
	return string(plaintext)
}

func TestEncryptStringRoundTrip(t *testing.T) {
	ring := mustKeyring(t, testKey)
	encrypted, err := ring.EncryptString("  sk-test-secret  ")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encrypted, "v1:") || strings.Contains(encrypted, "sk-test-secret") {
		t.Fatalf("unexpected payload %q", encrypted)
	}
	decrypted, err := ring.DecryptString(encrypted)
	if err != nil || decrypted != "sk-test-secret" {
		t.Fatalf("got %q, %v", decrypted, err)
	}
}

func TestEncryptBytesRoundTrip(t *testing.T) {
	ring := mustKeyring(t, testKey)
	plaintext := []byte{0x00, 0xff, 0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, ' '}
	encrypted, err := ring.Encrypt(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := ring.Decrypt(encrypted)
	if err != nil || !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("got %x, %v", decrypted, err)
	}
}

func TestEmptyValuesStayEmpty(t *testing.T) {
	ring := mustKeyring(t, testKey)
	if encrypted, err := ring.EncryptString("   "); err != nil || encrypted != "" {
		t.Fatalf("encrypting blank: %q, %v", encrypted, err)
	}
	if decrypted, err := ring.DecryptString("  "); err != nil || decrypted != "" {
		t.Fatalf("decrypting blank: %q, %v", decrypted, err)
	}
}

func TestEncryptUsesFreshNonce(t *testing.T) {
	ring := mustKeyring(t, testKey)
	first, _ := ring.EncryptString("same")
	second, _ := ring.EncryptString("same")
	if first == second {
		t.Fatal("two encryptions of the same value produced the same payload")
	}
}

// 格式与引入 Keyring 之前完全一致：存量数据可读，新数据也能被旧版本读取。
func TestFormatMatchesThePreviousImplementation(t *testing.T) {
	ring := mustKeyring(t, testKey)
	if got, err := ring.DecryptString(legacyEncrypt(t, testKey, "stored-before")); err != nil || got != "stored-before" {
		t.Fatalf("existing payload: %q, %v", got, err)
	}
	encrypted, _ := ring.EncryptString("written-after")
	if got := legacyDecrypt(t, testKey, encrypted); got != "written-after" {
		t.Fatalf("previous implementation read %q", got)
	}
}

func TestDecryptRejectsMalformedOrTamperedPayloads(t *testing.T) {
	ring := mustKeyring(t, testKey)
	encrypted, _ := ring.EncryptString("secret")
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(encrypted, prefix))
	raw[len(raw)-1] ^= 0x01
	tampered := prefix + base64.StdEncoding.EncodeToString(raw)
	for _, payload := range []string{`{"keys":[]}`, "v1:!!!", "v1:AAAA", tampered} {
		if _, err := ring.DecryptString(payload); err == nil {
			t.Fatalf("accepted %q", payload)
		}
	}
	if _, err := mustKeyring(t, testOldKey).DecryptString(encrypted); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("a different key decrypted the payload: %v", err)
	}
}

func TestRotation(t *testing.T) {
	oldPayload, _ := mustKeyring(t, testOldKey).EncryptString("token")

	rotated := mustKeyring(t, testKey, testOldKey)
	if got, err := rotated.DecryptString(oldPayload); err != nil || got != "token" {
		t.Fatalf("old payload after rotation: %q, %v", got, err)
	}
	fresh, _ := rotated.EncryptString("token")
	if got := legacyDecrypt(t, testKey, fresh); got != "token" {
		t.Fatalf("new payloads must use the active key, got %q", got)
	}

	if _, err := mustKeyring(t, testKey).DecryptString(oldPayload); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("expected ErrDecrypt once the old key is removed, got %v", err)
	}
}

// 兑换码索引必须与之前的 HMAC 一致，更换主密钥后旧索引仍能命中。
func TestLookupHashes(t *testing.T) {
	ring := mustKeyring(t, testKey, testOldKey)
	hashes := ring.LookupHashes("ABCD-EFGH")
	if len(hashes) != 2 {
		t.Fatalf("expected one hash per key, got %d", len(hashes))
	}
	for i, secret := range []string{testKey, testOldKey} {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte("ABCD-EFGH"))
		if want := hex.EncodeToString(mac.Sum(nil)); hashes[i] != want {
			t.Fatalf("hash %d: got %s want %s", i, hashes[i], want)
		}
	}
}

func TestNewKeyring(t *testing.T) {
	if _, err := NewKeyring("  "); !errors.Is(err, ErrNoKey) {
		t.Fatalf("expected ErrNoKey, got %v", err)
	}
	ring := mustKeyring(t, testKey, "", testKey, testOldKey)
	if len(ring.keys) != 2 {
		t.Fatalf("expected duplicates and blanks to be dropped, got %d keys", len(ring.keys))
	}
	var nilRing *Keyring
	if _, err := nilRing.EncryptString("x"); !errors.Is(err, ErrNoKey) {
		t.Fatalf("nil keyring: %v", err)
	}
}

func TestOpenReportsWhetherTheCurrentKeyWasUsed(t *testing.T) {
	oldPayload, _ := mustKeyring(t, testOldKey).EncryptString("old")
	ring := mustKeyring(t, testKey, testOldKey)
	newPayload, _ := ring.EncryptString("new")
	if plaintext, current, err := ring.Open(newPayload); err != nil || !current || string(plaintext) != "new" {
		t.Fatalf("current-key payload: %q %v %v", plaintext, current, err)
	}
	if plaintext, current, err := ring.Open(oldPayload); err != nil || current || string(plaintext) != "old" {
		t.Fatalf("previous-key payload: %q %v %v", plaintext, current, err)
	}
	if ring.PreviousKeyCount() != 1 || mustKeyring(t, testKey, testKey, " ").PreviousKeyCount() != 0 {
		t.Fatal("previous key count must ignore duplicates and blanks")
	}
}

func TestReencrypt(t *testing.T) {
	plaintext := " keeps surrounding spaces "
	oldPayload, _ := mustKeyring(t, testOldKey).Encrypt([]byte(plaintext))
	ring := mustKeyring(t, testKey, testOldKey)

	next, changed, err := ring.Reencrypt(oldPayload)
	if err != nil || !changed {
		t.Fatalf("Reencrypt: %v %v", changed, err)
	}
	if got := legacyDecrypt(t, testKey, next); got != plaintext {
		t.Fatalf("re-encrypted payload holds %q", got)
	}
	if again, changed, err := ring.Reencrypt(next); err != nil || changed || again != "" {
		t.Fatalf("current payloads must be left alone: %q %v %v", again, changed, err)
	}
	if _, changed, err := ring.Reencrypt(""); err != nil || changed {
		t.Fatalf("empty payload: %v %v", changed, err)
	}
	foreign, _ := mustKeyring(t, "unrelated-data-encryption-key-0123456789").EncryptString("x")
	if _, _, err := ring.Reencrypt(foreign); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("expected ErrDecrypt for a payload no key can open, got %v", err)
	}
}

func TestBoundPayloadsOnlyOpenWithTheirBinding(t *testing.T) {
	ring := mustKeyring(t, testKey)
	payload, err := ring.EncryptStringBound(" sk-secret ", "provider:1:abc")
	if err != nil || !strings.HasPrefix(payload, prefix) {
		t.Fatalf("EncryptStringBound: %q %v", payload, err)
	}
	if got, err := ring.DecryptStringBound(payload, "provider:1:abc"); err != nil || got != "sk-secret" {
		t.Fatalf("same binding: %q %v", got, err)
	}
	if _, err := ring.DecryptStringBound(payload, "provider:2:abc"); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("other binding must fail, got %v", err)
	}
	if _, err := ring.DecryptString(payload); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("unbound decrypt must fail, got %v", err)
	}
	unbound, _ := ring.EncryptString("sk-secret")
	if _, err := ring.DecryptStringBound(unbound, "provider:1:abc"); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("an unbound payload must not open with a binding, got %v", err)
	}
	if got, err := ring.EncryptStringBound("", "provider:1:abc"); err != nil || got != "" {
		t.Fatalf("empty value: %q %v", got, err)
	}
}

func TestReencryptBoundKeepsTheBinding(t *testing.T) {
	oldPayload, _ := mustKeyring(t, testOldKey).EncryptStringBound("sk-secret", "b")
	ring := mustKeyring(t, testKey, testOldKey)
	if _, _, err := ring.Reencrypt(oldPayload); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("unbound re-encryption must not open a bound payload, got %v", err)
	}
	next, changed, err := ring.ReencryptBound(oldPayload, "b")
	if err != nil || !changed {
		t.Fatalf("ReencryptBound: %v %v", changed, err)
	}
	if got, err := mustKeyring(t, testKey).DecryptStringBound(next, "b"); err != nil || got != "sk-secret" {
		t.Fatalf("re-encrypted payload: %q %v", got, err)
	}
	if _, err := mustKeyring(t, testKey).DecryptStringBound(next, "c"); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("the binding must survive re-encryption, got %v", err)
	}
}
