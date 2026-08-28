package conversation

import (
	"strings"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
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

func TestTraceBlockResponseCarriesStartedAtWhenSet(t *testing.T) {
	started := time.Now().Add(-time.Minute)
	resp := toTraceBlockResponse(&model.MessageTraceBlock{
		Title:       "模型思考",
		Status:      "completed",
		RoundID:     "round_1",
		StartedAt:   started,
		UpdatedAt:   started.Add(30 * time.Second),
		PayloadJSON: `{"tool_calls":[]}`,
	})
	if resp == nil || resp.StartedAt == nil || !resp.StartedAt.Equal(started) {
		t.Fatalf("expected startedAt in block response, got %#v", resp)
	}
}

func TestTraceBlockResponseOmitsZeroStartedAt(t *testing.T) {
	resp := toTraceBlockResponse(&model.MessageTraceBlock{
		Title:       "处理",
		Status:      "completed",
		PayloadJSON: `{"tool_calls":[]}`,
	})
	if resp == nil || resp.StartedAt != nil {
		t.Fatalf("expected omitted startedAt for zero value, got %#v", resp)
	}
	if resp.PayloadJSON == "" {
		t.Fatalf("expected payload to survive mapping, got %#v", resp)
	}
}
