package conversation

import (
	"strings"
	"testing"
)

func TestSanitizePublicTracePayloadRemovesKnowledgeEvidence(t *testing.T) {
	raw := `{
		"query":"internal policy",
		"file_names":["policy.md"],
		"citations":[{"file_id":"file_secret","file_name":"policy.md","preview":"confidential excerpt","score":0.9}],
		"stage":{"kind":"retrieval","status":"completed"}
	}`

	got := sanitizePublicTracePayloadJSON(raw)
	for _, secret := range []string{"file_secret", "policy.md", "confidential excerpt", "citations", "file_names"} {
		if strings.Contains(got, secret) {
			t.Fatalf("sanitizePublicTracePayloadJSON() leaked %q in %s", secret, got)
		}
	}
	if !strings.Contains(got, `"kind":"retrieval"`) || !strings.Contains(got, `"status":"completed"`) {
		t.Fatalf("sanitizePublicTracePayloadJSON() removed safe retrieval diagnostics: %s", got)
	}
}
