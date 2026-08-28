package conversation

import (
	"context"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type temporaryPersistenceRepositoryStub struct {
	repository.ConversationRepository
	traceWrites      int
	traceEventWrites int
	toolCallWrites   int
}

func (s *temporaryPersistenceRepositoryStub) UpsertConversationMessageTrace(context.Context, *model.MessageTrace) error {
	s.traceWrites++
	return nil
}

func (s *temporaryPersistenceRepositoryStub) UpsertConversationMessageTraceEvent(context.Context, *model.MessageTraceEventRow) error {
	s.traceEventWrites++
	return nil
}

func (s *temporaryPersistenceRepositoryStub) CreateConversationToolCall(context.Context, *model.ToolCall) error {
	s.toolCallWrites++
	return nil
}

func TestValidateTemporaryChatInput(t *testing.T) {
	valid := TemporaryChatInput{
		UserID:      1,
		SessionID:   "session",
		ClientRunID: "run",
		Model:       "model",
		Messages: []TemporaryChatMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
			{Role: "user", Content: "continue"},
		},
	}
	if err := ValidateTemporaryChatInput(valid); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}

	tests := map[string]TemporaryChatInput{
		"assistant last": {
			UserID: 1, SessionID: "session", ClientRunID: "run", Model: "model",
			Messages: []TemporaryChatMessage{{Role: "user", Content: "hello"}, {Role: "assistant", Content: "hi"}},
		},
		"consecutive roles": {
			UserID: 1, SessionID: "session", ClientRunID: "run", Model: "model",
			Messages: []TemporaryChatMessage{{Role: "user", Content: "one"}, {Role: "user", Content: "two"}},
		},
		"system role": {
			UserID: 1, SessionID: "session", ClientRunID: "run", Model: "model",
			Messages: []TemporaryChatMessage{{Role: "system", Content: "secret"}},
		},
		"duplicate knowledge base": {
			UserID: 1, SessionID: "session", ClientRunID: "run", Model: "model",
			KnowledgeBaseIDs: []string{"kb-one", "kb-one"},
			Messages:         []TemporaryChatMessage{{Role: "user", Content: "hello"}},
		},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateTemporaryChatInput(input); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}

func TestStripTemporaryChatProviderStateOptions(t *testing.T) {
	input := map[string]interface{}{
		"temperature":          0.5,
		"store":                true,
		"cache_control":        map[string]interface{}{"type": "ephemeral"},
		"cachedContent":        "cachedContents/example",
		"prompt_cache_options": map[string]interface{}{"mode": "explicit"},
	}
	result := stripTemporaryChatProviderStateOptions(input)
	if result["temperature"] != 0.5 {
		t.Fatalf("ordinary generation option lost: %#v", result)
	}
	for _, key := range []string{"store", "cache_control", "cachedContent", "prompt_cache_options"} {
		if _, ok := result[key]; ok {
			t.Fatalf("provider state option %q was retained: %#v", key, result)
		}
	}
	if _, ok := input["store"]; !ok {
		t.Fatal("input map must not be mutated")
	}
}

func TestEnforceTemporaryGenerateInput(t *testing.T) {
	input := llm.GenerateInput{
		PreviousResponseID:  "response-1",
		PromptCacheKey:      "cache-1",
		ResponsesBackground: true,
	}

	result := enforceTemporaryGenerateInput(input)
	if !result.Ephemeral {
		t.Fatal("temporary generation must remain ephemeral")
	}
	if result.PreviousResponseID != "" || result.PromptCacheKey != "" || result.ResponsesBackground {
		t.Fatalf("temporary generation retained provider state: %#v", result)
	}
}

func TestStripTemporaryMessageCacheControls(t *testing.T) {
	cacheControl := &llm.CacheControl{Type: "ephemeral", TTL: "1h"}
	input := []llm.Message{{
		Role:         "system",
		Content:      "temporary context",
		CacheControl: cacheControl,
		Parts: []llm.ContentPart{{
			Kind:         "text",
			Text:         "temporary context",
			CacheControl: cacheControl,
		}},
	}}
	result := stripTemporaryMessageCacheControls(input)
	if result[0].CacheControl != nil || result[0].Parts[0].CacheControl != nil {
		t.Fatalf("temporary message retained a provider cache marker: %#v", result[0])
	}
	if input[0].CacheControl == nil || input[0].Parts[0].CacheControl == nil {
		t.Fatal("input messages must not be mutated")
	}
}

func TestEphemeralTraceEmitsWithoutPersistence(t *testing.T) {
	repo := &temporaryPersistenceRepositoryStub{}
	service := &Service{
		cfg: config.NewRuntime(config.Config{
			ProcessTraceEnabled:         true,
			ProcessTraceVisibleToUser:   true,
			ProcessTracePersistInflight: true,
		}),
		repo: repo,
	}
	eventCount := 0
	promptTraceVisible := false
	recorder := newEphemeralMessageTraceRecorder(service, t.Context(), &model.Message{
		PublicID: "temporary-message",
		UserID:   1,
		RunID:    "temporary-run",
	}, func(eventType string, payload map[string]interface{}) error {
		eventCount++
		if eventType == "process_update" {
			if trace, ok := payload["trace"].(*model.MessageProcessTrace); ok && trace.PromptTrace != nil {
				promptTraceVisible = true
			}
		}
		return nil
	})
	recorder.recordPromptTrace(&model.MessagePromptTrace{
		Mode:               "full",
		SentTokenEstimate:  12,
		FullMessageCount:   1,
		SentMessageCount:   1,
		TotalTokenEstimate: 12,
		Blocks: []model.MessagePromptTraceBlock{{
			Kind:          string(PromptBlockTranscript),
			Title:         "历史对话",
			TokenEstimate: 12,
			SourceCount:   1,
		}},
	})
	recorder.appendProcessSection("检索", "完成", nil, messageTraceStatusCompleted)
	recorder.complete()
	recorder.waitForPendingPersistence(t.Context())

	if eventCount == 0 {
		t.Fatal("expected ephemeral trace to remain visible to the current browser")
	}
	if !promptTraceVisible {
		t.Fatal("expected ephemeral prompt context to be emitted to the current browser")
	}
	if repo.traceWrites != 0 || repo.traceEventWrites != 0 {
		t.Fatalf("ephemeral trace was persisted: trace=%d event=%d", repo.traceWrites, repo.traceEventWrites)
	}
}
