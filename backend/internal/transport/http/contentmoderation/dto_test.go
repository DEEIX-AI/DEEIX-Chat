package contentmoderation

import (
	"encoding/json"
	"strings"
	"testing"

	appcm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
)

func TestServiceConfigResponseJSONContract(t *testing.T) {
	response := toConfigResponse(&appcm.ServiceConfig{
		Enabled:        true,
		BaseURL:        "https://api.openai.com/v1",
		APIKeyMasked:   "sk-a...mnop",
		HasAPIKey:      true,
		Model:          "omni-moderation-latest",
		TimeoutSeconds: 10,
		MaxConcurrency: 4,
		QueueCapacity:  256,
		Policy: appcm.Policy{
			InputTextCategories: []string{"hate"},
			Version:             2,
		},
	})
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, field := range []string{`"enabled":true`, `"baseUrl"`, `"apiKeyMasked"`, `"inputTextCategories"`, `"version":2`} {
		if !strings.Contains(text, field) {
			t.Fatalf("config JSON missing %s: %s", field, text)
		}
	}
	if strings.Contains(text, `"policyVersion"`) || strings.Contains(text, `"BaseURL"`) || strings.Contains(text, `"APIKey"`) {
		t.Fatalf("config JSON contains an internal or duplicate field: %s", text)
	}
}

func TestUpdateConfigRequestMapsToApplicationInput(t *testing.T) {
	var request ContentModerationUpdateConfigRequest
	if err := json.Unmarshal([]byte(`{"enabled":false,"baseUrl":"https://example.com/v1","clearAPIKey":true,"policy":{"inputTextCategories":["hate"]}}`), &request); err != nil {
		t.Fatal(err)
	}
	input := request.toApplicationInput()
	if input.Enabled == nil || *input.Enabled {
		t.Fatalf("expected explicit false enabled value, got %#v", input.Enabled)
	}
	if input.BaseURL == nil || *input.BaseURL != "https://example.com/v1" || !input.ClearAPIKey {
		t.Fatalf("unexpected config input: %#v", input)
	}
	if input.Policy == nil || len(input.Policy.InputTextCategories) != 1 {
		t.Fatalf("unexpected policy input: %#v", input.Policy)
	}
}

func TestEventDetailResponseHidesInternalFields(t *testing.T) {
	response := toEventDetailResponse(&appcm.EventDetail{
		Event: domaincm.Event{
			PublicID:      "evt-1",
			EncryptedText: "secret-ciphertext",
		},
		DecryptedText: "reviewable text",
		Images: []domaincm.IsolatedImageMeta{{
			Index:       0,
			StoragePath: "moderation/private/object",
		}},
	}, "User 1", "user1")
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"secret-ciphertext", "EncryptedText", "storagePath", "moderation/private/object"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("event detail leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"decryptedText":"reviewable text"`) {
		t.Fatalf("event detail contract changed: %s", text)
	}
	if response.Event.Categories == nil || response.CategoryScores == nil || response.Images == nil {
		t.Fatalf("required collections must not serialize as null: %#v", response)
	}
	if strings.Contains(text, `"categoriesJSON"`) || strings.Count(text, `"userLabel"`) > 1 || strings.Count(text, `"username"`) > 1 {
		t.Fatalf("event detail contains storage-shaped or duplicate fields: %s", text)
	}
}
