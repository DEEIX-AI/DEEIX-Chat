package conversation

import (
	"context"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	persistencemodels "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	persistenceconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/conversation"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCanceledTraceSettlementPersistsCompleteReasoningForReload(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:trace_cancel_settlement?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&persistencemodels.ChatRunEvent{}); err != nil {
		t.Fatalf("migrate trace table: %v", err)
	}

	repo := persistenceconversation.NewRepo(db)
	cfg := config.Config{
		ProcessTraceEnabled:            true,
		ProcessTraceVisibleToUser:      true,
		ProcessTraceStoreUpstreamThink: true,
	}
	service := &Service{cfg: config.NewRuntime(cfg), repo: repo}
	assistant := &model.Message{
		ID:             41,
		ConversationID: 17,
		UserID:         9,
		RunID:          "run_cancel_settlement",
		Role:           "assistant",
	}

	generationCtx, cancelGeneration := context.WithCancel(context.Background())
	recorder := newMessageTraceRecorder(service, generationCtx, assistant, nil)
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "嗯", nil)
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "，继续分析并保留终止前的完整思考", nil)
	cancelGeneration()
	if generationCtx.Err() == nil {
		t.Fatal("expected generation context to be canceled")
	}

	recorder.failWithContext(context.Background(), ErrMessageGenerationCanceled)

	reloaded := []model.Message{{ID: assistant.ID, Role: "assistant"}}
	reloadService := &Service{cfg: config.NewRuntime(cfg), repo: repo}
	if err := reloadService.hydrateMessageProcessTraces(context.Background(), reloaded); err != nil {
		t.Fatalf("hydrate persisted trace: %v", err)
	}
	trace := reloaded[0].ProcessTrace
	if trace == nil || trace.UpstreamThink == nil {
		t.Fatalf("expected persisted upstream reasoning after reload, got %#v", trace)
	}
	if got, want := trace.UpstreamThink.ContentMarkdown, "嗯，继续分析并保留终止前的完整思考"; got != want {
		t.Fatalf("reloaded reasoning = %q, want %q", got, want)
	}
	if trace.UpstreamThink.Status != messageTraceStatusError {
		t.Fatalf("reloaded reasoning status = %q, want %q", trace.UpstreamThink.Status, messageTraceStatusError)
	}
}
