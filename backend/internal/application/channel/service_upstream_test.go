package channel

import (
	"encoding/json"
	"testing"
)

func TestValidateUpstreamBaseURLAllowsAdministratorConfiguredPrivateOrigin(t *testing.T) {
	service := &Service{}
	if err := service.validateUpstreamBaseURL("http://new-api:3000/v1"); err != nil {
		t.Fatalf("private model endpoint rejected: %v", err)
	}
	if err := service.validateUpstreamBaseURL("http://169.254.169.254/latest/meta-data"); err == nil {
		t.Fatal("metadata endpoint must remain blocked")
	}
}

func TestUpdateAPIKeysByIDsRemovesSelectedKeys(t *testing.T) {
	raw := `{"strategy":"round_robin","keys":[{"key":"sk-first","status":"active"},{"key":"sk-second","status":"inactive","note":"old"},{"key":"sk-third","status":"active"}]}`
	secret := "test-secret"

	got, err := updateAPIKeysByIDs(raw, []string{apiKeyID(secret, 1, "sk-second")}, nil, secret)
	if err != nil {
		t.Fatalf("updateAPIKeysByIDs returned error: %v", err)
	}

	var payload apiKeysPayload
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if payload.Strategy != "round_robin" {
		t.Fatalf("strategy = %q, want round_robin", payload.Strategy)
	}
	if len(payload.Keys) != 2 {
		t.Fatalf("keys len = %d, want 2", len(payload.Keys))
	}
	if payload.Keys[0].Key != "sk-first" || payload.Keys[1].Key != "sk-third" {
		t.Fatalf("unexpected keys after deletion: %#v", payload.Keys)
	}
}

func TestUpdateAPIKeysByIDsRejectsRemovingAllKeys(t *testing.T) {
	raw := `{"strategy":"round_robin","keys":[{"key":"sk-first","status":"active"}]}`
	secret := "test-secret"

	if _, err := updateAPIKeysByIDs(raw, []string{apiKeyID(secret, 0, "sk-first")}, nil, secret); err == nil {
		t.Fatal("expected error when deleting every key")
	}
}

func TestUpdateAPIKeysByIDsRejectsRemovingAllActiveKeys(t *testing.T) {
	raw := `{"strategy":"round_robin","keys":[{"key":"sk-first","status":"active"},{"key":"sk-second","status":"inactive"}]}`
	secret := "test-secret"

	if _, err := updateAPIKeysByIDs(raw, []string{apiKeyID(secret, 0, "sk-first")}, nil, secret); err == nil {
		t.Fatal("expected error when deleting every active key")
	}
}

func TestUpdateAPIKeysByIDsRejectsInvalidID(t *testing.T) {
	raw := `{"strategy":"round_robin","keys":[{"key":"sk-first","status":"active"}]}`

	if _, err := updateAPIKeysByIDs(raw, []string{"missing-key-id"}, nil, "test-secret"); err == nil {
		t.Fatal("expected error for invalid key id")
	}
}

func TestUpdateAPIKeysByIDsHandlesDuplicateKeys(t *testing.T) {
	raw := `{"strategy":"round_robin","keys":[{"key":"sk-same","status":"active"},{"key":"sk-same","status":"active"},{"key":"sk-third","status":"active"}]}`
	secret := "test-secret"

	got, err := updateAPIKeysByIDs(raw, []string{apiKeyID(secret, 1, "sk-same")}, nil, secret)
	if err != nil {
		t.Fatalf("updateAPIKeysByIDs returned error: %v", err)
	}

	var payload apiKeysPayload
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(payload.Keys) != 2 {
		t.Fatalf("keys len = %d, want 2", len(payload.Keys))
	}
	if payload.Keys[0].Key != "sk-same" || payload.Keys[1].Key != "sk-third" {
		t.Fatalf("unexpected keys after deleting duplicate: %#v", payload.Keys)
	}
}

func TestUpdateAPIKeysByIDsAppendsKeys(t *testing.T) {
	raw := `{"strategy":"round_robin","keys":[{"key":"sk-first","status":"active"}]}`
	addRaw := `{"strategy":"round_robin","keys":[{"key":"sk-second","status":"active"}]}`

	got, err := updateAPIKeysByIDs(raw, nil, &addRaw, "test-secret")
	if err != nil {
		t.Fatalf("updateAPIKeysByIDs returned error: %v", err)
	}

	var payload apiKeysPayload
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(payload.Keys) != 2 {
		t.Fatalf("keys len = %d, want 2", len(payload.Keys))
	}
	if payload.Keys[0].Key != "sk-first" || payload.Keys[1].Key != "sk-second" {
		t.Fatalf("unexpected keys after append: %#v", payload.Keys)
	}
}

func TestUpdateAPIKeysByIDsAllowsReplaceOneByDeleteAndAppend(t *testing.T) {
	raw := `{"strategy":"round_robin","keys":[{"key":"sk-old","status":"active"},{"key":"sk-keep","status":"active"}]}`
	addRaw := `{"strategy":"round_robin","keys":[{"key":"sk-new","status":"active"}]}`
	secret := "test-secret"

	got, err := updateAPIKeysByIDs(raw, []string{apiKeyID(secret, 0, "sk-old")}, &addRaw, secret)
	if err != nil {
		t.Fatalf("updateAPIKeysByIDs returned error: %v", err)
	}

	var payload apiKeysPayload
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(payload.Keys) != 2 {
		t.Fatalf("keys len = %d, want 2", len(payload.Keys))
	}
	if payload.Keys[0].Key != "sk-keep" || payload.Keys[1].Key != "sk-new" {
		t.Fatalf("unexpected keys after replace: %#v", payload.Keys)
	}
}

func TestMaskAPIKeyViewsReturnsStableIndexes(t *testing.T) {
	raw := `{"strategy":"round_robin","keys":[{"key":"sk-1234567890","status":"active"},{"key":"short","status":"inactive","note":"old"}]}`
	secret := "test-secret"

	got := maskAPIKeyViews(raw, secret)
	if len(got) != 2 {
		t.Fatalf("items len = %d, want 2", len(got))
	}
	if got[0].ID != apiKeyID(secret, 0, "sk-1234567890") || got[0].Index != 0 || got[0].KeyMasked != "sk-1****7890" || got[0].Status != "active" {
		t.Fatalf("unexpected first item: %#v", got[0])
	}
	if got[1].ID != apiKeyID(secret, 1, "short") || got[1].Index != 1 || got[1].KeyMasked != "****" || got[1].Status != "inactive" || got[1].Note != "old" {
		t.Fatalf("unexpected second item: %#v", got[1])
	}
}

func TestValidateAPIKeysRejectsNoActiveKey(t *testing.T) {
	raw := `{"strategy":"round_robin","keys":[{"key":"sk-first","status":"inactive"}]}`

	if err := validateAPIKeys(raw); err == nil {
		t.Fatal("expected error when no active key is configured")
	}
}
