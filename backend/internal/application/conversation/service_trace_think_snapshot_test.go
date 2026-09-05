package conversation

import (
	"context"
	"strings"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// traceRecordRepoStub 记录 trace 行写入，用于验证"内存快照始终更新、DB 落盘受节流"这条边界。
type traceRecordRepoStub struct {
	repository.ConversationRepository
	traceRows      []model.MessageTrace
	traceEventRows []model.MessageTraceEventRow
}

func (s *traceRecordRepoStub) UpsertConversationMessageTrace(_ context.Context, item *model.MessageTrace) error {
	if item != nil {
		s.traceRows = append(s.traceRows, *item)
	}
	return nil
}

func (s *traceRecordRepoStub) UpsertConversationMessageTraceEvent(_ context.Context, item *model.MessageTraceEventRow) error {
	if item != nil {
		s.traceEventRows = append(s.traceEventRows, *item)
	}
	return nil
}

func newSnapshotRecorder(persistInflight bool) (*messageTraceRecorder, *traceRecordRepoStub) {
	stub := &traceRecordRepoStub{}
	return &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
			ProcessTracePersistInflight:    persistInflight,
		},
		ctx:       context.Background(),
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_snapshot"},
		service:   &Service{repo: stub},
	}, stub
}

func TestStreamingThinkFlushUpdatesMemorySnapshotWithoutInFlightPersistence(t *testing.T) {
	recorder, stub := newSnapshotRecorder(false)

	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "第一轮深度分析", nil)

	if len(stub.traceRows) != 0 || len(stub.traceEventRows) != 0 {
		t.Fatalf("in-flight persistence must stay disabled when ProcessTracePersistInflight=false, got %d rows / %d events",
			len(stub.traceRows), len(stub.traceEventRows))
	}
	trace := recorder.snapshot()
	if trace == nil || len(trace.Events) != 1 {
		t.Fatalf("expected one in-memory think snapshot event, got %#v", trace)
	}
	event := trace.Events[0]
	if event.EventType != "think" || event.Stage != messageTraceStageThink {
		t.Fatalf("unexpected snapshot event: %#v", event)
	}
	if event.RoundID == "" || event.RoundID != recorder.upstreamThink.roundID {
		t.Fatalf("expected snapshot roundID to follow live draft, got %q want %q", event.RoundID, recorder.upstreamThink.roundID)
	}
	if event.StartedAt.IsZero() || !event.StartedAt.Equal(recorder.upstreamThink.startedAt) {
		t.Fatalf("expected snapshot event startedAt to carry live draft start, got %v want %v", event.StartedAt, recorder.upstreamThink.startedAt)
	}
	if event.Status != messageTraceStatusStreaming {
		t.Fatalf("expected streaming status in live snapshot, got %q", event.Status)
	}
	if event.UpdatedAt.IsZero() {
		t.Fatal("expected updatedAt on live snapshot event")
	}
	if trace.UpstreamThink == nil || !trace.UpstreamThink.StartedAt.Equal(recorder.upstreamThink.startedAt) {
		t.Fatalf("expected block-level startedAt in snapshot, got %#v", trace.UpstreamThink)
	}
}

func TestStreamingThinkInflightPersistenceWritesRoundIdentityAndStaysThrottled(t *testing.T) {
	recorder, stub := newSnapshotRecorder(true)

	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "思考内容", nil)

	if len(stub.traceRows) != 1 || len(stub.traceEventRows) != 1 {
		t.Fatalf("expected first throttle window to persist trace + event rows, got %d / %d",
			len(stub.traceRows), len(stub.traceEventRows))
	}
	row := stub.traceRows[0]
	if row.RoundID == "" || row.RoundID != recorder.upstreamThink.roundID {
		t.Fatalf("expected persisted row roundID to match live draft, got %q want %q", row.RoundID, recorder.upstreamThink.roundID)
	}
	if row.StartedAt.IsZero() || !row.StartedAt.Equal(recorder.upstreamThink.startedAt) {
		t.Fatalf("expected persisted row startedAt to match live draft, got %v want %v", row.StartedAt, recorder.upstreamThink.startedAt)
	}
	if row.Status != messageTraceStatusStreaming {
		t.Fatalf("expected streaming status row, got %q", row.Status)
	}

	// 同一节流窗口内的后续 flush 必须跳过 DB 落盘，但内存快照仍要更新。
	recorder.flushUpstreamThinkLiveUpdate(recorder.upstreamThink, upstreamThinkLiveUpdateOptions{Force: true, PersistSnapshot: true})
	if len(stub.traceRows) != 1 || len(stub.traceEventRows) != 1 {
		t.Fatalf("in-flight persistence must stay throttled inside window, got %d / %d rows",
			len(stub.traceRows), len(stub.traceEventRows))
	}
	if len(recorder.snapshot().Events) != 1 {
		t.Fatalf("memory snapshot must still update on every flush, got %#v", recorder.snapshot().Events)
	}
}

func TestStreamingToolUpdatesKeepLatestSnapshotAndThrottleSideEffects(t *testing.T) {
	recorder, stub := newSnapshotRecorder(true)
	emitted := 0
	recorder.onEvent = func(eventType string, _ map[string]any) error {
		if eventType == "process_update" {
			emitted++
		}
		return nil
	}

	streamTool := func(input string, status string) {
		summary, markdown, payload := buildToolTrace([]model.ToolCall{{
			ToolCallID: "call_1",
			ToolType:   "server_tool_use",
			ToolName:   "web_search",
			Status:     status,
			InputJSON:  input,
		}})
		recorder.syncToolSection(summary, markdown, payload, traceStatusFromToolStatus(status))
	}

	streamTool(`{"query":"first"}`, "streaming")
	streamTool(`{"query":"second"}`, "streaming")
	if len(stub.traceRows) != 1 || len(stub.traceEventRows) != 1 {
		t.Fatalf("expected in-flight tool persistence to be throttled, got %d / %d", len(stub.traceRows), len(stub.traceEventRows))
	}
	if emitted != 1 {
		t.Fatalf("expected live tool updates inside one window to be coalesced, got %d", emitted)
	}
	events := recorder.snapshot().Events
	if len(events) != 1 || !strings.Contains(events[0].PayloadJSON, `\"second\"`) {
		t.Fatalf("expected memory snapshot to retain latest tool input, got %#v", events)
	}

	// Terminal state bypasses both throttles so clients and durable storage see completion immediately.
	recorder.service = nil
	streamTool(`{"query":"second"}`, "success")
	if emitted != 2 {
		t.Fatalf("expected terminal tool update to emit immediately, got %d", emitted)
	}
}

func TestThinkSnapshotUsesFreshStartedAtPerRound(t *testing.T) {
	recorder, stub := newSnapshotRecorder(false)
	// 本用例只验证内存快照，complete 会触发后台持久化 goroutine（写 stub），
	// 去掉 service 避免跨 goroutine 写入与断言产生竞态。
	recorder.service = nil

	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "第一轮思考", nil)
	firstStart := recorder.upstreamThink.startedAt
	firstRound := recorder.upstreamThink.roundID
	firstEventID := recorder.upstreamThink.eventID

	recorder.completeUpstreamThink()
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "第二轮思考", nil)
	// 第二轮首段内容落在上一轮 completion 的 live flush 窗口内不会自动落快照，强制 flush 一次。
	recorder.flushUpstreamThinkLiveUpdate(recorder.upstreamThink, upstreamThinkLiveUpdateOptions{Force: true, PersistSnapshot: true})

	if recorder.upstreamThink.roundID == firstRound || recorder.upstreamThink.eventID == firstEventID {
		t.Fatalf("expected a fresh round identity after completion, round=%q event=%q",
			recorder.upstreamThink.roundID, recorder.upstreamThink.eventID)
	}
	if recorder.upstreamThink.startedAt.Equal(firstStart) {
		t.Fatalf("expected fresh startedAt per round, got %v", recorder.upstreamThink.startedAt)
	}
	events := recorder.snapshot().Events
	if len(events) != 2 {
		t.Fatalf("expected snapshots for both rounds, got %#v", events)
	}
	if !events[0].StartedAt.Equal(firstStart) || !events[1].StartedAt.Equal(recorder.upstreamThink.startedAt) {
		t.Fatalf("snapshot startedAt must follow each round's own draft, round1=%v round2=%v",
			events[0].StartedAt, events[1].StartedAt)
	}
	if events[0].Status != messageTraceStatusCompleted || events[0].EndedAt == nil {
		t.Fatalf("expected completed first round snapshot with endedAt, got %#v", events[0])
	}
	if events[1].Status != messageTraceStatusStreaming {
		t.Fatalf("expected streaming second round snapshot, got %#v", events[1])
	}
	if len(stub.traceRows) != 0 {
		t.Fatalf("in-flight persistence must stay disabled, got %d rows", len(stub.traceRows))
	}
}

func TestBuildMessageProcessTraceDTOBlockCarriesStartedAt(t *testing.T) {
	started := time.Now().Add(-2 * time.Minute)
	trace := buildMessageProcessTraceDTO([]model.MessageTrace{{
		TraceType:       messageTraceTypeUpstreamThink,
		Status:          messageTraceStatusCompleted,
		Title:           "模型思考",
		Summary:         "完成",
		ContentMarkdown: "思考内容",
		RoundID:         "round_1",
		StartedAt:       started,
	}}, nil)
	if trace == nil || trace.UpstreamThink == nil {
		t.Fatalf("expected upstream think block, got %#v", trace)
	}
	if !trace.UpstreamThink.StartedAt.Equal(started) {
		t.Fatalf("expected hydrated block startedAt, got %v want %v", trace.UpstreamThink.StartedAt, started)
	}
	if trace.Status != messageTraceStatusCompleted {
		t.Fatalf("expected completed status, got %q", trace.Status)
	}
}
