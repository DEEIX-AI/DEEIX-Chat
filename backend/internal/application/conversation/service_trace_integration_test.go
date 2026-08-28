package conversation

import (
	"context"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	persistencemodels "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	persistenceconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type assistantEditTraceRepository struct {
	repository.ConversationRepository
	message model.Message
}

func (r *assistantEditTraceRepository) GetMessageByPublicIDForUser(
	context.Context,
	uint,
	string,
) (*model.Message, error) {
	item := r.message
	return &item, nil
}

func (r *assistantEditTraceRepository) UpdateAssistantMessageContent(
	_ context.Context,
	_ uint,
	_ string,
	content string,
	editedAt time.Time,
) (*model.Message, error) {
	r.message.Content = content
	r.message.EditedAt = &editedAt
	item := r.message
	return &item, nil
}

func (r *assistantEditTraceRepository) GetUserMessageFeedbackMap(
	context.Context,
	uint,
	[]uint,
) (map[uint]string, error) {
	return map[uint]string{}, nil
}

func (r *assistantEditTraceRepository) GetMessageFeedbackCounts(
	context.Context,
	[]uint,
) (map[uint]map[string]int64, error) {
	return map[uint]map[string]int64{}, nil
}

func (r *assistantEditTraceRepository) ListConversationMessageTracesByMessageIDs(
	context.Context,
	[]uint,
) ([]model.MessageTrace, error) {
	return []model.MessageTrace{{
		MessageID:       r.message.ID,
		TraceType:       messageTraceTypeProcess,
		Title:           "Processing complete",
		ContentMarkdown: "Retained processing details",
		Status:          messageTraceStatusCompleted,
	}}, nil
}

func (r *assistantEditTraceRepository) ListConversationMessageTraceEventsByMessageIDs(
	context.Context,
	[]uint,
) ([]model.MessageTraceEventRow, error) {
	return []model.MessageTraceEventRow{{
		MessageID: r.message.ID,
		EventID:   "event_tool_1",
		EventType: "tool",
		Title:     "Tool complete",
		Status:    messageTraceStatusCompleted,
		Seq:       1,
	}}, nil
}

func TestAssistantEditResponseRetainsProcessTrace(t *testing.T) {
	repo := &assistantEditTraceRepository{
		message: model.Message{
			ID:             41,
			ConversationID: 17,
			UserID:         9,
			PublicID:       "message_assistant_edit",
			Role:           "assistant",
			Status:         "success",
			Content:        "before",
		},
	}
	service := &Service{
		cfg: config.NewRuntime(config.Config{
			ProcessTraceEnabled:       true,
			ProcessTraceVisibleToUser: true,
		}),
		repo: repo,
	}

	updated, err := service.UpdateAssistantMessageContent(
		context.Background(),
		repo.message.UserID,
		repo.message.PublicID,
		"after",
	)
	if err != nil {
		t.Fatalf("edit assistant message: %v", err)
	}
	if updated.Content != "after" || updated.EditedAt == nil {
		t.Fatalf("unexpected edited message: %#v", updated)
	}
	if updated.ProcessTrace == nil || updated.ProcessTrace.Process == nil {
		t.Fatalf("edited response lost process trace: %#v", updated.ProcessTrace)
	}
	if len(updated.ProcessTrace.Events) != 1 || updated.ProcessTrace.Events[0].EventID != "event_tool_1" {
		t.Fatalf("edited response lost execution events: %#v", updated.ProcessTrace)
	}
}

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

func TestToolTraceRoundsSurviveReload(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:trace_tool_rounds?mode=memory&cache=shared"), &gorm.Config{})
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
		ID:             42,
		ConversationID: 17,
		UserID:         9,
		RunID:          "run_tool_round_reload",
		Role:           "assistant",
	}
	recorder := newMessageTraceRecorder(service, context.Background(), assistant, nil)

	appendRound := func(callID string) {
		summary, markdown, payload := buildToolTrace([]model.ToolCall{{
			ToolCallID: callID,
			ToolName:   "web_search",
			Status:     "success",
			InputJSON:  `{"query":"test"}`,
			OutputJSON: `{"ok":true}`,
		}})
		recorder.appendToolSection(summary, markdown, payload, messageTraceStatusCompleted)
		recorder.completeTools()
	}
	appendRound("call_1")
	appendRound("call_2")
	recorder.complete()

	reloaded := []model.Message{{ID: assistant.ID, Role: "assistant"}}
	if err := service.hydrateMessageProcessTraces(context.Background(), reloaded); err != nil {
		t.Fatalf("hydrate persisted trace: %v", err)
	}
	trace := reloaded[0].ProcessTrace
	if trace == nil {
		t.Fatal("expected persisted trace")
	}
	toolEvents := make([]model.MessageTraceEvent, 0, 2)
	for _, event := range trace.Events {
		if event.EventType == "tool" {
			toolEvents = append(toolEvents, event)
		}
	}
	if len(toolEvents) != 2 {
		t.Fatalf("expected two persisted tool rounds, got %#v", trace.Events)
	}
	if toolEvents[0].RoundID == toolEvents[1].RoundID {
		t.Fatalf("expected distinct persisted round identities, got %#v", toolEvents)
	}
}
