package conversation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	cachememory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
)

func TestLiveSnapshotKeepsThinkStructureWithoutContent(t *testing.T) {
	recorder, _ := newSnapshotRecorder(false)
	longThink := strings.Repeat("深度推理", 4096)

	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, longThink, nil)
	recorder.appendToolSection("工具", "调用 search", &tracePayload{ToolCalls: []traceToolCall{{
		ToolCallID: "call_1", Name: "search", Status: "running",
	}}}, messageTraceStatusStreaming)

	persisted := recorder.snapshot()
	live := recorder.liveSnapshot()
	if persisted == nil || live == nil {
		t.Fatalf("expected both snapshots, got persisted=%v live=%v", persisted, live)
	}
	if len(live.Events) != len(persisted.Events) {
		t.Fatalf("live snapshot must keep every event: live=%d persisted=%d", len(live.Events), len(persisted.Events))
	}

	var liveThink, persistedThink *model.MessageTraceEvent
	for idx := range live.Events {
		if live.Events[idx].Phase == messageTraceTypeUpstreamThink {
			liveThink = &live.Events[idx]
			persistedThink = &persisted.Events[idx]
		}
	}
	if liveThink == nil || persistedThink == nil {
		t.Fatalf("expected a think event in both snapshots, got %#v", live.Events)
	}
	if persistedThink.ContentMarkdown != longThink {
		t.Fatal("persisted snapshot must keep the full think content")
	}
	if liveThink.ContentMarkdown != "" || liveThink.PayloadJSON != "" {
		t.Fatalf("live snapshot must drop think content, got %d bytes content / %d bytes payload", len(liveThink.ContentMarkdown), len(liveThink.PayloadJSON))
	}
	if liveThink.EventID != persistedThink.EventID || liveThink.RoundID != persistedThink.RoundID || liveThink.Summary == "" {
		t.Fatalf("live think event must keep identity and summary, got %#v", liveThink)
	}
	if live.UpstreamThink == nil || live.UpstreamThink.ContentMarkdown != "" || live.UpstreamThink.Summary == "" {
		t.Fatalf("live upstream think block must keep summary only, got %#v", live.UpstreamThink)
	}

	var liveTool *model.MessageTraceEvent
	for idx := range live.Events {
		if live.Events[idx].Phase == messageTraceTypeTools {
			liveTool = &live.Events[idx]
		}
	}
	if liveTool == nil || liveTool.ContentMarkdown == "" || liveTool.PayloadJSON == "" {
		t.Fatalf("tool events must keep their bounded content in the live snapshot, got %#v", liveTool)
	}
}

func TestCompleteUpstreamThinkEmitsTerminalSnapshot(t *testing.T) {
	recorder, _ := newSnapshotRecorder(false)
	var snapshots []map[string]any
	recorder.onEvent = func(eventType string, payload map[string]any) error {
		if eventType == "process_update" {
			snapshots = append(snapshots, payload)
		}
		return nil
	}

	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "第一轮分析", nil)
	roundID := recorder.upstreamThink.roundID
	recorder.completeUpstreamThink()

	if len(snapshots) == 0 {
		t.Fatal("completing a think round must push a process_update snapshot")
	}
	trace, ok := snapshots[len(snapshots)-1]["trace"].(*model.MessageProcessTrace)
	if !ok || trace == nil {
		t.Fatalf("expected trace snapshot in the last process_update, got %#v", snapshots[len(snapshots)-1])
	}
	var think *model.MessageTraceEvent
	for idx := range trace.Events {
		if trace.Events[idx].RoundID == roundID && trace.Events[idx].Phase == messageTraceTypeUpstreamThink {
			think = &trace.Events[idx]
		}
	}
	if think == nil {
		t.Fatalf("expected the completed think event in the snapshot, got %#v", trace.Events)
	}
	if think.Status != messageTraceStatusCompleted || think.EndedAt == nil {
		t.Fatalf("snapshot must carry the terminal state, got status=%q endedAt=%v", think.Status, think.EndedAt)
	}
}

func TestLiveSnapshotStaysBelowStreamPayloadLimitAcrossRounds(t *testing.T) {
	recorder, _ := newSnapshotRecorder(false)
	store := cachememory.New()
	registry := newGenerationStreamRegistry(store, generationStreamOptions{
		Retention:        time.Minute,
		ActiveTTL:        time.Minute,
		MaxEvents:        64,
		SubscriberBuffer: 4,
	})
	runID := EnsureMessageGenerationRunID("")
	ctx, cleanup := registerTestGeneration(t, registry, runID, 7, "conv_test", func() {})
	defer cleanup()

	// 三轮各 96 KB 的思考文本远超 128 KB 的流事件上限；实时快照仍必须完整到达客户端。
	thinkChunk := strings.Repeat("r", 96*1024)
	for round := 0; round < 3; round++ {
		recorder.appendUpstreamReasoning(messageTraceThinkKindContent, thinkChunk, nil)
		recorder.completeUpstreamThink()
		recorder.appendToolSection("工具", "调用 search", &tracePayload{ToolCalls: []traceToolCall{{
			ToolCallID: "call_" + string(rune('1'+round)), Name: "search", Status: "success", OutputPreview: "ok",
		}}}, messageTraceStatusCompleted)
		recorder.completeTools()
	}

	rawPersisted, err := json.Marshal(recorder.snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if len(rawPersisted) <= generationStreamMaxPayloadBytes {
		t.Fatalf("test setup must exceed the stream payload limit, persisted snapshot is only %d bytes", len(rawPersisted))
	}
	rawTrace, err := json.Marshal(recorder.liveSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	if len(rawTrace) > generationStreamMaxPayloadBytes {
		t.Fatalf("live snapshot must stay under the stream payload limit, got %d bytes", len(rawTrace))
	}
	published := publishTestGeneration(t, registry, ctx, runID, map[string]any{
		"type":   "process_update",
		"status": "streaming",
		"trace":  json.RawMessage(rawTrace),
	})
	if published["payloadTruncated"] == true {
		t.Fatalf("live trace snapshot must not be compacted, payload was %d bytes", len(rawTrace))
	}
	if _, ok := published["trace"]; !ok {
		t.Fatalf("live process_update must keep its trace, got %#v", published)
	}
	if len(recorder.liveSnapshot().Events) != 6 {
		t.Fatalf("expected 3 think + 3 tool events, got %d", len(recorder.liveSnapshot().Events))
	}
}

func TestOversizedThinkTextIsChunkedInsteadOfDropped(t *testing.T) {
	recorder, _ := newSnapshotRecorder(false)
	var deltas []map[string]any
	recorder.onEvent = func(eventType string, payload map[string]any) error {
		if eventType == "upstream_think_delta" {
			deltas = append(deltas, payload)
		}
		return nil
	}

	// 非流式返回会把整段思考一次性同步进来；此前超过 16 KB 的单段文本会被直接丢弃。
	bigThink := strings.Repeat("思考", upstreamThinkLiveReplaceBytes)
	recorder.syncStructuredThink(bigThink, "", nil)
	recorder.completeUpstreamThink()

	var rebuilt strings.Builder
	for _, payload := range deltas {
		if replace, ok := payload["contentMarkdown"].(string); ok && replace != "" {
			rebuilt.Reset()
			rebuilt.WriteString(replace)
			continue
		}
		if delta, ok := payload["delta"].(string); ok {
			rebuilt.WriteString(delta)
		}
	}
	if rebuilt.String() != bigThink {
		t.Fatalf("live deltas must reassemble the full think text, got %d bytes want %d", rebuilt.Len(), len(bigThink))
	}
	for idx, payload := range deltas {
		for _, key := range []string{"delta", "contentMarkdown"} {
			if value, ok := payload[key].(string); ok && len(value) > upstreamThinkLiveReplaceBytes {
				t.Fatalf("delta %d %s exceeds the live event limit: %d bytes", idx, key, len(value))
			}
			if value, ok := payload[key].(string); ok && !utf8.ValidString(value) {
				t.Fatalf("delta %d %s was split inside a UTF-8 sequence", idx, key)
			}
		}
	}
}

func TestSplitUpstreamThinkLiveTextRespectsRuneBoundaries(t *testing.T) {
	text := strings.Repeat("字", 10)
	chunks := splitUpstreamThinkLiveText(text, 7)
	if strings.Join(chunks, "") != text {
		t.Fatalf("chunks must concatenate to the original text, got %q", chunks)
	}
	for _, chunk := range chunks {
		if len(chunk) > 7 || !utf8.ValidString(chunk) {
			t.Fatalf("chunk %q violates the limit or rune boundary", chunk)
		}
	}
	if got := splitUpstreamThinkLiveText("", 7); len(got) != 1 || got[0] != "" {
		t.Fatalf("empty text must yield a single empty chunk, got %q", got)
	}
}
