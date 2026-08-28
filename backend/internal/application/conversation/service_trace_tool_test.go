package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestSummarizeToolTracePayloadCountsFailedCalls(t *testing.T) {
	firstSummary, _, firstPayload := buildToolTrace([]model.ToolCall{{
		ToolName:  "bing_search",
		Status:    "error",
		ErrorJSON: "missing query",
	}})
	if firstSummary != "1 次工具调用，1 次失败" {
		t.Fatalf("unexpected first summary: %q", firstSummary)
	}
	_, _, secondPayload := buildToolTrace([]model.ToolCall{{
		ToolName:   "bing_search",
		Status:     "success",
		OutputJSON: `{"content":[{"type":"text","text":"ok"}]}`,
	}})

	mergeTracePayload(firstPayload, secondPayload)
	if got := summarizeToolTracePayload(firstPayload); got != "完成 2 次工具调用，1 次失败" {
		t.Fatalf("expected failed call to count in aggregate summary, got %q", got)
	}
}

func TestBuildToolTraceMarksReusedCallsAsCompleted(t *testing.T) {
	summary, markdown, payload := buildToolTrace([]model.ToolCall{{
		ToolName:   "bing_search",
		Status:     "reused",
		OutputJSON: `{"content":[{"type":"text","text":"cached"}]}`,
	}})
	if summary != "1 次工具调用已完成" {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if !strings.Contains(markdown, "已复用") {
		t.Fatalf("expected reused status in markdown, got %q", markdown)
	}
	items := normalizeTraceToolCalls(payload["tool_calls"])
	if len(items) != 1 || items[0]["status"] != "reused" {
		t.Fatalf("expected reused payload status, got %#v", items)
	}
}

func TestBuildToolTraceStoresPreviewMetadataInsteadOfFullOutput(t *testing.T) {
	largeOutput := `{"content":[{"type":"text","text":"` + strings.Repeat("x", 4096) + `"}]}`
	_, _, payload := buildToolTrace([]model.ToolCall{{
		ToolCallID: "call_1",
		ToolName:   "fetch",
		Status:     "success",
		InputJSON:  `{"url":"https://example.com/large"}`,
		OutputJSON: largeOutput,
	}})

	items := normalizeTraceToolCalls(payload["tool_calls"])
	if len(items) != 1 {
		t.Fatalf("expected one tool call, got %#v", items)
	}
	item := items[0]
	if _, ok := item["output"]; ok {
		t.Fatalf("tool trace must not store full output: %#v", item)
	}
	if _, ok := item["output_text"]; ok {
		t.Fatalf("tool trace must not store expanded output text: %#v", item)
	}
	if _, ok := item["input"]; ok {
		t.Fatalf("tool trace must not store full input: %#v", item)
	}
	if got := traceInt64(item["output_size"]); got != int64(len(largeOutput)) {
		t.Fatalf("expected output size metadata, got %d", got)
	}
	if item["output_truncated"] != true {
		t.Fatalf("expected truncated output marker, got %#v", item["output_truncated"])
	}
	if got := strings.TrimSpace(getTraceString(item["input_detail"])); got != `{"url":"https://example.com/large"}` {
		t.Fatalf("expected full small input detail, got %q", got)
	}
	detail := strings.TrimSpace(getTraceString(item["output_detail"]))
	if detail == "" || detail == largeOutput || len([]rune(detail)) > toolTraceDetailMaxChars+3 {
		t.Fatalf("expected bounded output detail, got len=%d", len([]rune(detail)))
	}
	preview := strings.TrimSpace(getTraceString(item["output_preview"]))
	if preview == "" || strings.Contains(preview, strings.Repeat("x", 512)) {
		t.Fatalf("expected compact output preview, got %q", preview)
	}
}

func TestBuildToolTraceStoresBoundedSearchPresentation(t *testing.T) {
	output := `{"content":[{"type":"text","text":"Title: 第一条新闻\nURL: https://example.com/news/1\nHighlights:\n摘要一\n\n---\n\nTitle: 第二条新闻\nURL: https://example.com/news/2\nHighlights:\n摘要二"}]}`
	_, _, payload := buildToolTrace([]model.ToolCall{{
		ToolCallID: "call_search",
		ToolName:   "web_search",
		Status:     "success",
		InputJSON:  `{"query":"今日新闻"}`,
		OutputJSON: output,
	}})

	items := normalizeTraceToolCalls(payload["tool_calls"])
	if len(items) != 1 {
		t.Fatalf("expected one tool call, got %#v", items)
	}
	rawPresentation, ok := items[0]["output_presentation"]
	if !ok {
		t.Fatalf("expected semantic output presentation, got %#v", items[0])
	}
	encoded, err := json.Marshal(rawPresentation)
	if err != nil {
		t.Fatalf("marshal output presentation: %v", err)
	}
	var presentation toolTraceOutputPresentation
	if err := json.Unmarshal(encoded, &presentation); err != nil {
		t.Fatalf("decode output presentation: %v", err)
	}
	if len(presentation.Sources) != 2 {
		t.Fatalf("expected two normalized sources, got %#v", presentation.Sources)
	}
	if presentation.Sources[0].Title != "第一条新闻" || presentation.Sources[0].URL != "https://example.com/news/1" {
		t.Fatalf("unexpected first source: %#v", presentation.Sources[0])
	}
}

func TestBuildToolTracePresentationExtractsStructuredSourcesWithoutVendorRules(t *testing.T) {
	presentation := buildToolTraceOutputPresentation(`{
		"results": [
			{"title":"Documentation","url":"https://docs.example.com/guide","snippet":"Guide"},
			{"name":"Reference","href":"https://docs.example.com/reference"}
		]
	}`)
	if presentation == nil || len(presentation.Sources) != 2 {
		t.Fatalf("expected provider-neutral structured sources, got %#v", presentation)
	}
	if presentation.Sources[1].URL != "https://docs.example.com/reference" {
		t.Fatalf("unexpected second source: %#v", presentation.Sources[1])
	}
}

func TestBuildToolTracePresentationExtractsReadableMCPText(t *testing.T) {
	presentation := buildToolTraceOutputPresentation(`{
		"content": [
			{"type":"text","text":"## 可用优惠券\n\n- 满 100 减 20\n<img src=\"https://example.com/coupon.jpg\" />\n![优惠券](https://example.com/coupon-2.jpg)\n- 满 50 减 8"}
		]
	}`)
	if presentation == nil {
		t.Fatal("expected readable MCP presentation")
	}
	if presentation.Text != "## 可用优惠券\n\n- 满 100 减 20\n\n- 满 50 减 8" {
		t.Fatalf("unexpected MCP presentation text: %q", presentation.Text)
	}
}

func TestBuildToolTracePresentationExtractsReadableGenericJSON(t *testing.T) {
	presentation := buildToolTraceOutputPresentation(`{
		"data": [
			{"message":"第一条结果"},
			{"message":"第二条结果"}
		]
	}`)
	if presentation == nil || presentation.Text != "第一条结果；第二条结果" {
		t.Fatalf("expected readable generic JSON presentation, got %#v", presentation)
	}
}

func TestBuildToolTracePresentationExtractsBoundedPlainText(t *testing.T) {
	plainText := "领券结果\n" + strings.Repeat("优惠券详情与适用条件；", 80)
	presentation := buildToolTraceOutputPresentation(plainText)
	if presentation == nil {
		t.Fatal("expected readable plain-text presentation")
	}
	textLength := len([]rune(presentation.Text))
	if textLength <= toolTracePreviewMaxChars {
		t.Fatalf("expected presentation to retain more than the compact preview, got %d characters", textLength)
	}
	if textLength > toolTracePresentationTextChars+1 {
		t.Fatalf("expected bounded plain-text presentation, got %d characters", textLength)
	}
	if !strings.HasPrefix(presentation.Text, "领券结果\n") {
		t.Fatalf("unexpected plain-text presentation: %q", presentation.Text)
	}
}

func TestBuildToolTracePresentationSkipsOversizedStructuredOutput(t *testing.T) {
	output := `{"data":"` + strings.Repeat("x", toolTracePresentationMaxInputBytes) + `"}`
	if presentation := buildToolTraceOutputPresentation(output); presentation != nil {
		t.Fatalf("expected oversized structured output to use the existing bounded fallback, got %#v", presentation)
	}
}

func TestToolTracePayloadMergesStreamingPlaceholderWithFinalCall(t *testing.T) {
	_, _, streamingPayload := buildToolTrace([]model.ToolCall{{
		ToolType:  "web_search_call",
		ToolName:  "web_search",
		Status:    "streaming",
		InputJSON: "",
	}})
	_, _, completedPayload := buildToolTrace([]model.ToolCall{{
		ToolCallID: "wsc_1",
		ToolType:   "web_search_call",
		ToolName:   "web_search",
		Status:     "success",
		InputJSON:  `{"query":"今日新闻"}`,
		OutputJSON: `[{"url":"https://example.com/news"}]`,
	}})

	mergeToolTracePayload(streamingPayload, completedPayload)
	items := normalizeTraceToolCalls(streamingPayload["tool_calls"])
	if len(items) != 1 {
		t.Fatalf("expected one merged tool call, got %#v", items)
	}
	if items[0]["tool_call_id"] != "wsc_1" || items[0]["status"] != "success" {
		t.Fatalf("expected final call to replace streaming placeholder, got %#v", items[0])
	}
	markdown := renderToolTraceMarkdownFromPayload(streamingPayload)
	if strings.Contains(markdown, "进行中") || !strings.Contains(markdown, "已完成") {
		t.Fatalf("expected rendered trace to show only final status, got %q", markdown)
	}
}

func TestBuildMessageProcessTraceDTOIncludesOrderedEvents(t *testing.T) {
	trace := buildMessageProcessTraceDTO(nil, []model.MessageTraceEventRow{
		{
			EventID:         "tools_1",
			EventType:       "tool",
			Phase:           messageTraceTypeTools,
			Status:          messageTraceStatusCompleted,
			Title:           "工具",
			Summary:         "工具完成",
			ContentMarkdown: "**fetch**：执行成功",
			Seq:             2,
		},
	})
	if trace == nil || len(trace.Events) != 1 {
		t.Fatalf("expected trace events, got %#v", trace)
	}
	if trace.Status != messageTraceStatusCompleted {
		t.Fatalf("expected completed trace status, got %q", trace.Status)
	}
	if trace.Events[0].EventID != "tools_1" || trace.Events[0].EventType != "tool" {
		t.Fatalf("unexpected event payload: %#v", trace.Events[0])
	}
}

func TestToolTraceKeepsRepeatedCallIDsInSeparateRounds(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_tool_rounds"},
	}

	appendRound := func(reasoning string, query string) {
		recorder.appendUpstreamReasoning(messageTraceThinkKindContent, reasoning, nil)
		recorder.completeUpstreamThink()
		summary, markdown, payload := buildToolTrace([]model.ToolCall{
			{
				ToolCallID: "call_1",
				ToolName:   "web_search",
				Status:     "success",
				InputJSON:  fmt.Sprintf(`{"query":%q}`, query),
				OutputJSON: `{"ok":true}`,
			},
		})
		recorder.appendToolSection(summary, markdown, payload, messageTraceStatusCompleted)
		recorder.completeTools()
	}

	appendRound("分析第一轮", "first")
	appendRound("分析第二轮", "second")

	toolEvents := make([]model.MessageTraceEvent, 0, 2)
	for _, event := range recorder.events {
		if event.EventType == "tool" {
			toolEvents = append(toolEvents, event)
		}
	}
	if len(toolEvents) != 2 {
		t.Fatalf("expected one tool event per round, got %#v", toolEvents)
	}
	if toolEvents[0].RoundID == toolEvents[1].RoundID {
		t.Fatalf("expected distinct round identities, got %q", toolEvents[0].RoundID)
	}
	if !strings.Contains(toolEvents[0].PayloadJSON, "first") || !strings.Contains(toolEvents[1].PayloadJSON, "second") {
		t.Fatalf("expected each round to retain its own payload, got %#v", toolEvents)
	}
	if recorder.tools == nil {
		t.Fatal("expected active tool block")
	}
	activePayload, err := json.Marshal(recorder.tools.payload)
	if err != nil {
		t.Fatalf("marshal active tool payload: %v", err)
	}
	if !strings.Contains(string(activePayload), "second") {
		t.Fatalf("expected active tool block to contain only the current round, got %#v", recorder.tools)
	}
}

func TestToolTraceUpdatesStreamingCallWithoutCreatingDuplicateEvent(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:       true,
			ProcessTraceVisibleToUser: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_tool_update"},
	}

	requestedSummary, requestedMarkdown, requestedPayload := buildToolTrace([]model.ToolCall{{
		ToolCallID: "call_1",
		ToolName:   "web_search",
		Status:     "requested",
		InputJSON:  `{"query":"test"}`,
	}})
	recorder.syncToolSection(requestedSummary, requestedMarkdown, requestedPayload, messageTraceStatusStreaming)
	if len(recorder.events) != 1 {
		t.Fatalf("expected one streaming event, got %#v", recorder.events)
	}
	eventID := recorder.events[0].EventID

	completedSummary, completedMarkdown, completedPayload := buildToolTrace([]model.ToolCall{{
		ToolCallID: "call_1",
		ToolName:   "web_search",
		Status:     "success",
		InputJSON:  `{"query":"test"}`,
		OutputJSON: `{"ok":true}`,
	}})
	recorder.appendToolSection(completedSummary, completedMarkdown, completedPayload, messageTraceStatusCompleted)
	recorder.completeTools()

	if len(recorder.events) != 1 {
		t.Fatalf("expected the final update to reuse the streaming event, got %#v", recorder.events)
	}
	if recorder.events[0].EventID != eventID || recorder.events[0].Status != messageTraceStatusCompleted {
		t.Fatalf("expected completed update for event %q, got %#v", eventID, recorder.events[0])
	}
	if strings.Contains(recorder.events[0].ContentMarkdown, "进行中") {
		t.Fatalf("expected final content to replace the streaming state, got %q", recorder.events[0].ContentMarkdown)
	}
}

func TestToolTraceStartsNewRoundAfterPreviousToolRoundCloses(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_tool_without_next_think"},
	}

	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "分析", nil)
	recorder.completeUpstreamThink()
	firstSummary, firstMarkdown, firstPayload := buildToolTrace([]model.ToolCall{{
		ToolCallID: "call_1",
		ToolName:   "web_search",
		Status:     "success",
		InputJSON:  `{"query":"first"}`,
	}})
	recorder.appendToolSection(firstSummary, firstMarkdown, firstPayload, messageTraceStatusCompleted)
	firstRoundID := recorder.tools.roundID
	recorder.completeTools()

	secondSummary, secondMarkdown, secondPayload := buildToolTrace([]model.ToolCall{{
		ToolCallID: "call_2",
		ToolName:   "web_search",
		Status:     "success",
		InputJSON:  `{"query":"second"}`,
	}})
	recorder.appendToolSection(secondSummary, secondMarkdown, secondPayload, messageTraceStatusCompleted)
	recorder.completeTools()

	if recorder.tools == nil || recorder.tools.roundID == firstRoundID {
		t.Fatalf("expected a new standalone tool round after the previous round closed, got %#v", recorder.tools)
	}
	toolEvents := make([]model.MessageTraceEvent, 0, 2)
	for _, event := range recorder.events {
		if event.EventType == "tool" {
			toolEvents = append(toolEvents, event)
		}
	}
	if len(toolEvents) != 2 {
		t.Fatalf("expected two tool events, got %#v", toolEvents)
	}
}

func TestTracePayloadJSONBoundsOversizedPayload(t *testing.T) {
	secret := strings.Repeat("x", maxTracePayloadBytes+1)
	serialized := tracePayloadJSON(map[string]interface{}{"upstream_debug": secret})
	if len(serialized) >= maxTracePayloadBytes {
		t.Fatalf("expected bounded trace payload, got %d bytes", len(serialized))
	}
	if strings.Contains(serialized, secret[:1024]) {
		t.Fatal("oversized trace payload must not retain original content")
	}
	if !strings.Contains(serialized, `"payloadOmitted":true`) {
		t.Fatalf("expected payload omission marker, got %s", serialized)
	}
}

func TestProcessTraceStaysStreamingUntilNextVisiblePhase(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_1"},
	}

	recorder.appendProcessSection("文件已就绪", "**文件上下文**：已纳入。", nil, messageTraceStatusStreaming)
	recorder.recordPromptTrace(&model.MessagePromptTrace{Mode: "full", SentMessageCount: 2})

	if recorder.process == nil || recorder.process.status != messageTraceStatusStreaming {
		t.Fatalf("expected process trace to stay streaming after prompt trace, got %#v", recorder.process)
	}
	if trace := recorder.snapshot(); trace == nil || trace.Process == nil || trace.Process.Status != messageTraceStatusStreaming {
		t.Fatalf("expected visible snapshot to stay streaming, got %#v", trace)
	}

	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "开始思考", nil)

	if recorder.process.status != messageTraceStatusCompleted {
		t.Fatalf("expected process trace to complete when reasoning starts, got %q", recorder.process.status)
	}
}

func TestUpstreamReasoningPayloadStoresMetadataOnly(t *testing.T) {
	draft := &messageTraceDraft{
		summary:         "思考摘要",
		contentMarkdown: strings.Repeat("完整思考内容", 256),
		payload:         map[string]interface{}{},
	}
	mergeUpstreamReasoningPayload(draft, messageTraceThinkKindContent, map[string]interface{}{
		"event_type":        "response.reasoning_text.done",
		"item_id":           "reasoning_1",
		"status":            "completed",
		"signature":         "opaque-signature",
		"encrypted_content": strings.Repeat("encrypted", 256),
	})

	encoded := tracePayloadJSON(draft.payload)
	for _, value := range []string{"完整思考内容", "思考摘要", "opaque-signature", "encrypted"} {
		if strings.Contains(encoded, value) {
			t.Fatalf("reasoning trace payload must not duplicate content or opaque continuation data: %q", value)
		}
	}
	for _, value := range []string{"content_text", "response.reasoning_text.done", "reasoning_1", "completed"} {
		if !strings.Contains(encoded, value) {
			t.Fatalf("reasoning trace payload must retain display metadata: %q", value)
		}
	}
}

func TestFinalReasoningSnapshotReconcilesCompletedStreamEvent(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_reconcile"},
	}
	payload := map[string]interface{}{
		"event_type": "response.reasoning_text.done",
		"item_id":    "reasoning_1",
		"status":     "completed",
	}
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "streamed", payload)
	recorder.completeUpstreamThink()
	firstEventID := recorder.upstreamThink.eventID

	recorder.reconcileStructuredThink("streamed and finalized", "", payload)
	recorder.completeUpstreamThink()

	thinkEvents := traceEventsByType(recorder.events, "think")
	if len(thinkEvents) != 1 {
		t.Fatalf("expected final snapshot to update the streamed reasoning event, got %#v", thinkEvents)
	}
	if thinkEvents[0].EventID != firstEventID || thinkEvents[0].ContentMarkdown != "streamed and finalized" {
		t.Fatalf("expected canonical reasoning event %q to contain final text, got %#v", firstEventID, thinkEvents[0])
	}
}

func TestFinalReasoningSnapshotStartsNewRoundForDifferentItem(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_reconcile_items"},
	}
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "first", map[string]interface{}{"item_id": "reasoning_1"})
	recorder.completeUpstreamThink()
	recorder.reconcileStructuredThink("second", "", map[string]interface{}{"item_id": "reasoning_2"})
	recorder.completeUpstreamThink()

	thinkEvents := traceEventsByType(recorder.events, "think")
	if len(thinkEvents) != 2 {
		t.Fatalf("expected distinct upstream reasoning items to remain separate, got %#v", thinkEvents)
	}
	if thinkEvents[0].RoundID == thinkEvents[1].RoundID {
		t.Fatalf("expected different reasoning items to use distinct rounds, got %#v", thinkEvents)
	}
}

func TestStreamingReasoningItemChangeClosesPriorToolRound(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_stream_items"},
	}
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "first reasoning", map[string]interface{}{"item_id": "reasoning_1"})
	toolSummary, toolMarkdown, toolPayload := buildToolTrace([]model.ToolCall{{
		ToolCallID: "search_1",
		ToolType:   "web_search_call",
		ToolName:   "web_search",
		Status:     "success",
	}})
	recorder.syncToolSection(toolSummary, toolMarkdown, toolPayload, messageTraceStatusCompleted)
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "second reasoning", map[string]interface{}{"item_id": "reasoning_2"})
	recorder.complete()

	if len(recorder.events) != 3 {
		t.Fatalf("expected think/tool/think events, got %#v", recorder.events)
	}
	if recorder.events[0].EventType != "think" || recorder.events[1].EventType != "tool" || recorder.events[2].EventType != "think" {
		t.Fatalf("expected interleaved think/tool/think order, got %#v", recorder.events)
	}
	if recorder.events[0].RoundID != recorder.events[1].RoundID || recorder.events[1].ParentEventID != recorder.events[0].EventID {
		t.Fatalf("expected tool to remain attached to the first reasoning item, got %#v", recorder.events)
	}
	if recorder.events[2].RoundID == recorder.events[0].RoundID {
		t.Fatalf("expected the next reasoning item to start a new round, got %#v", recorder.events)
	}
	for _, event := range recorder.events {
		if event.Status != messageTraceStatusCompleted {
			t.Fatalf("expected settled trace events, got %#v", recorder.events)
		}
	}
}

func TestStreamingFinalizationDoesNotReplayObservedTraceEvents(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_stream_finalize"},
	}
	reasoningPayload := map[string]interface{}{"item_id": "reasoning_1", "status": "completed"}
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "streamed reasoning", reasoningPayload)
	recorder.completeUpstreamThink()
	toolSummary, toolMarkdown, toolPayload := buildToolTrace([]model.ToolCall{{
		ToolCallID: "search_1",
		ToolType:   "web_search_call",
		ToolName:   "web_search",
		Status:     "success",
	}})
	recorder.syncToolSection(toolSummary, toolMarkdown, toolPayload, messageTraceStatusCompleted)

	finalizeStreamingOutputTrace(recorder, &llm.GenerateOutput{
		Reasoning: &llm.ReasoningOutput{ItemID: "reasoning_1", Status: "completed", Text: "streamed reasoning"},
		ServerToolCalls: []llm.ToolCall{{
			ToolCallID: "search_1",
			ToolType:   "web_search_call",
			ToolName:   "web_search",
			Status:     "completed",
		}},
	}, "run_stream_finalize", map[string]string{"id:search_1": "success"})

	if got := len(traceEventsByType(recorder.events, "think")); got != 1 {
		t.Fatalf("expected one reasoning event after final reconciliation, got %d", got)
	}
	if got := len(traceEventsByType(recorder.events, "tool")); got != 1 {
		t.Fatalf("expected one observed tool event without final replay, got %d", got)
	}
}

func TestStreamingFinalizationAddsUnobservedServerToolsBeforeFinalReasoning(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_stream_missing_tool_event"},
	}
	finalizeStreamingOutputTrace(recorder, &llm.GenerateOutput{
		Reasoning: &llm.ReasoningOutput{ItemID: "reasoning_final", Status: "completed", Text: "final reasoning"},
		ServerToolCalls: []llm.ToolCall{{
			ToolCallID: "search_1",
			ToolType:   "web_search_call",
			ToolName:   "web_search",
			Status:     "completed",
		}},
	}, "run_stream_missing_tool_event", nil)

	if len(recorder.events) != 2 || recorder.events[0].EventType != "tool" || recorder.events[1].EventType != "think" {
		t.Fatalf("expected missing live server tool to be restored before final reasoning, got %#v", recorder.events)
	}
}

func TestStreamingFinalizationRestoresOnlyMissingServerTools(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_stream_partial_tools"},
	}
	firstSummary, firstMarkdown, firstPayload := buildToolTrace([]model.ToolCall{{
		ToolCallID: "search_1",
		ToolType:   "web_search_call",
		ToolName:   "web_search",
		Status:     "success",
	}})
	recorder.syncToolSection(firstSummary, firstMarkdown, firstPayload, messageTraceStatusCompleted)

	finalizeStreamingOutputTrace(recorder, &llm.GenerateOutput{
		ServerToolCalls: []llm.ToolCall{
			{ToolCallID: "search_1", ToolType: "web_search_call", ToolName: "web_search", Status: "completed"},
			{ToolCallID: "code_1", ToolType: "code_interpreter_call", ToolName: "code_interpreter", Status: "completed"},
		},
	}, "run_stream_partial_tools", map[string]string{"id:search_1": "success"})

	toolEvents := traceEventsByType(recorder.events, "tool")
	if len(toolEvents) != 1 {
		t.Fatalf("expected missing final tool to merge into the live round, got %#v", toolEvents)
	}
	toolCalls := normalizeTraceToolCalls(traceEventPayload(t, toolEvents[0])["tool_calls"])
	if len(toolCalls) != 2 {
		t.Fatalf("expected both observed and restored tool calls, got %#v", toolCalls)
	}
}

func TestStreamingFinalizationReconcilesNonTerminalServerTool(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_stream_pending_tool"},
	}
	streamSummary, streamMarkdown, streamPayload := buildToolTrace([]model.ToolCall{{
		ToolCallID: "search_1",
		ToolType:   "web_search_call",
		ToolName:   "web_search",
		Status:     "streaming",
	}})
	recorder.syncToolSection(streamSummary, streamMarkdown, streamPayload, messageTraceStatusStreaming)

	finalizeStreamingOutputTrace(recorder, &llm.GenerateOutput{
		ServerToolCalls: []llm.ToolCall{{
			ToolCallID: "search_1",
			ToolType:   "web_search_call",
			ToolName:   "web_search",
			Status:     "completed",
			OutputJSON: `{"ok":true}`,
		}},
	}, "run_stream_pending_tool", map[string]string{"id:search_1": "streaming"})

	toolEvents := traceEventsByType(recorder.events, "tool")
	if len(toolEvents) != 1 {
		t.Fatalf("expected final tool state to reconcile in place, got %#v", toolEvents)
	}
	toolCalls := normalizeTraceToolCalls(traceEventPayload(t, toolEvents[0])["tool_calls"])
	if len(toolCalls) != 1 || getTraceString(toolCalls[0]["status"]) != "success" {
		t.Fatalf("expected completed final tool state, got %#v", toolCalls)
	}
}

func traceEventsByType(events []model.MessageTraceEvent, eventType string) []model.MessageTraceEvent {
	filtered := make([]model.MessageTraceEvent, 0, len(events))
	for _, event := range events {
		if event.EventType == eventType {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func traceEventPayload(t *testing.T, event model.MessageTraceEvent) map[string]interface{} {
	t.Helper()
	payload := make(map[string]interface{})
	if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
		t.Fatalf("decode trace event payload: %v", err)
	}
	return payload
}

func TestUpstreamThinkingDeltaIsCoalescedBetweenFlushes(t *testing.T) {
	eventCount := 0
	var events []map[string]interface{}
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_1"},
		onEvent: func(eventType string, payload map[string]interface{}) error {
			if eventType == "upstream_think_delta" {
				eventCount++
				events = append(events, payload)
			}
			return nil
		},
	}

	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "a", nil)
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "b", nil)
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "c", nil)

	if eventCount != 1 {
		t.Fatalf("expected dense thinking deltas to be coalesced after first flush, got %d events", eventCount)
	}
	if len(events) != 1 || events[0]["delta"] != "a" {
		t.Fatalf("expected first live event to carry only first delta, got %#v", events)
	}
	startedAt, ok := events[0]["startedAt"].(time.Time)
	if !ok || recorder.upstreamThink == nil || !startedAt.Equal(recorder.upstreamThink.startedAt) {
		t.Fatalf("expected live event to carry the authoritative thinking start, got %#v", events[0]["startedAt"])
	}
	if _, ok := events[0]["endedAt"]; ok {
		t.Fatalf("streaming thinking event must not carry endedAt, got %#v", events[0]["endedAt"])
	}
	if _, ok := events[0]["trace"]; ok {
		t.Fatalf("live thinking delta must not carry full trace: %#v", events[0])
	}
	if _, ok := events[0]["block"]; ok {
		t.Fatalf("live thinking delta must not carry full block: %#v", events[0])
	}
	if recorder.upstreamThink == nil || recorder.upstreamThink.contentMarkdown != "abc" {
		t.Fatalf("expected full reasoning to remain in memory snapshot, got %#v", recorder.upstreamThink)
	}

	recorder.completeUpstreamThink()
	if eventCount != 2 {
		t.Fatalf("expected completion to emit final thinking snapshot, got %d events", eventCount)
	}
	if events[1]["delta"] != "bc" || events[1]["status"] != messageTraceStatusCompleted {
		t.Fatalf("expected completion to flush coalesced delta with completed status, got %#v", events[1])
	}
	completedStartedAt, ok := events[1]["startedAt"].(time.Time)
	if !ok || !completedStartedAt.Equal(startedAt) {
		t.Fatalf("expected all deltas in one thinking round to retain startedAt, got %v then %v",
			events[0]["startedAt"], events[1]["startedAt"])
	}
	endedAt, ok := events[1]["endedAt"].(time.Time)
	if !ok || recorder.upstreamThink == nil || recorder.upstreamThink.endedAt == nil || !endedAt.Equal(*recorder.upstreamThink.endedAt) {
		t.Fatalf("expected completed thinking event to carry the authoritative end, got %#v", events[1]["endedAt"])
	}
}

func TestFailedUpstreamThinkingFlushesBufferedContent(t *testing.T) {
	var events []map[string]interface{}
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_cancel"},
		onEvent: func(eventType string, payload map[string]interface{}) error {
			if eventType == "upstream_think_delta" {
				events = append(events, payload)
			}
			return nil
		},
	}
	recorderCtx, cancelRecorder := context.WithCancel(context.Background())
	recorder.ctx = recorderCtx
	cancelRecorder()

	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "嗯", nil)
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, "，继续分析完整内容", nil)
	recorder.failWithContext(context.Background(), ErrMessageGenerationCanceled)

	if len(events) != 2 {
		t.Fatalf("expected cancellation to flush the buffered reasoning, got %d events", len(events))
	}
	if events[1]["delta"] != "，继续分析完整内容" || events[1]["status"] != messageTraceStatusError {
		t.Fatalf("unexpected terminal reasoning event: %#v", events[1])
	}
	trace := recorder.snapshot()
	if trace == nil || trace.UpstreamThink == nil || trace.UpstreamThink.ContentMarkdown != "嗯，继续分析完整内容" {
		t.Fatalf("expected complete reasoning snapshot after cancellation, got %#v", trace)
	}
}

func TestUpstreamThinkingLiveDeltaSkipsOversizedContent(t *testing.T) {
	var events []map[string]interface{}
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:            true,
			ProcessTraceVisibleToUser:      true,
			ProcessTraceStoreUpstreamThink: true,
		},
		assistant: &model.Message{ID: 1, ConversationID: 2, UserID: 3, RunID: "run_1"},
		onEvent: func(eventType string, payload map[string]interface{}) error {
			if eventType == "upstream_think_delta" {
				events = append(events, payload)
			}
			return nil
		},
	}

	largeDelta := strings.Repeat("x", upstreamThinkLiveReplaceBytes+1)
	recorder.appendUpstreamReasoning(messageTraceThinkKindContent, largeDelta, nil)

	if len(events) != 1 {
		t.Fatalf("expected one lightweight status event, got %d", len(events))
	}
	if _, ok := events[0]["delta"]; ok {
		t.Fatalf("oversized thinking delta must not be sent in live event: %#v", events[0])
	}
	if _, ok := events[0]["contentMarkdown"]; ok {
		t.Fatalf("oversized thinking content must not be sent in live event: %#v", events[0])
	}
	if recorder.upstreamThink == nil || recorder.upstreamThink.contentMarkdown != largeDelta {
		t.Fatal("expected oversized thinking content to remain available for final trace")
	}
}

func TestBuildMessageProcessTraceDTOExtractsPromptTrace(t *testing.T) {
	payload := map[string]interface{}{
		"prompt_trace": messagePromptTracePayload(&model.MessagePromptTrace{
			Mode:                  "stateful",
			PromptFingerprint:     "fp_1",
			StatefulUsed:          true,
			TotalTokenEstimate:    120,
			SentTokenEstimate:     20,
			FullMessageCount:      6,
			SentMessageCount:      1,
			StatefulSavedMessages: 5,
			StatefulSavedTokens:   100,
			Blocks: []model.MessagePromptTraceBlock{{
				Kind:          string(PromptBlockStableContext),
				Title:         "稳定文件上下文",
				TokenEstimate: 80,
				Cacheable:     true,
				SourceCount:   1,
				SourceRefs: []model.MessagePromptTraceSourceRef{{
					SourceType: string(model.ContextArtifactSummary),
					SourceID:   "summary",
					Title:      "上下文摘要",
					ArtifactID: 77,
				}},
			}},
		}),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}
	trace := buildMessageProcessTraceDTO([]model.MessageTrace{{
		TraceType:       messageTraceTypeProcess,
		Status:          messageTraceStatusCompleted,
		Title:           "处理",
		Summary:         "已规划上下文",
		ContentMarkdown: "**上下文规划**：续接发送",
		PayloadJSON:     string(raw),
	}}, nil)

	if trace == nil || trace.PromptTrace == nil {
		t.Fatalf("expected prompt trace, got %#v", trace)
	}
	if !trace.PromptTrace.StatefulUsed || trace.PromptTrace.SentMessageCount != 1 || len(trace.PromptTrace.Blocks) != 1 {
		t.Fatalf("unexpected prompt trace: %#v", trace.PromptTrace)
	}
	if got := trace.PromptTrace.Blocks[0].SourceRefs[0].ArtifactID; got != 77 {
		t.Fatalf("expected prompt trace source artifact id to survive payload, got %d", got)
	}
}

func TestBuildAttachmentProcessTraceIncludesTypedFileRefs(t *testing.T) {
	summary, markdown, payload := buildAttachmentProcessTrace("auto", []AttachmentInput{
		{
			FileID:      "file_img",
			Kind:        "image",
			FileName:    "diagram.png",
			MimeType:    "image/png",
			ContextMode: fileContextModeDirectImage,
		},
		{
			FileID:      "file_full",
			Kind:        "document",
			FileName:    "brief.md",
			MimeType:    "text/markdown",
			ContextMode: fileContextModeFull,
		},
		{
			FileID:      "file_rag",
			Kind:        "document",
			FileName:    "spec.pdf",
			MimeType:    "application/pdf",
			ContextMode: fileContextModeRAG,
		},
		{
			FileID:      "file_skip",
			Kind:        "document",
			FileName:    "huge.pdf",
			MimeType:    "application/pdf",
			ContextMode: fileContextModeSkipped,
		},
	})
	if summary != "已纳入 3 个文件，未纳入 1 个文件" {
		t.Fatalf("expected skipped files to be excluded from included count, got %q", summary)
	}
	if !strings.Contains(markdown, "纳入 3 个文件，未纳入 1 个文件") {
		t.Fatalf("expected markdown detail to show included and skipped counts, got %q", markdown)
	}
	if payload == nil {
		t.Fatal("expected attachment trace payload")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal attachment payload failed: %v", err)
	}
	var parsed struct {
		FileMode   string                   `json:"file_mode"`
		FileRefs   []attachmentTraceFileRef `json:"file_refs"`
		TraceStage struct {
			Kind          string `json:"kind"`
			Status        string `json:"status"`
			IncludedCount int    `json:"included_count"`
			SkippedCount  int    `json:"skipped_count"`
		} `json:"trace_stage"`
		FileGroupRefs struct {
			DirectImages []attachmentTraceFileRef `json:"direct_images"`
			Adaptive     []attachmentTraceFileRef `json:"adaptive"`
			Retrieval    []attachmentTraceFileRef `json:"retrieval"`
			Skipped      []attachmentTraceFileRef `json:"skipped"`
		} `json:"file_group_refs"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal attachment payload failed: %v", err)
	}
	if parsed.FileMode != "auto" || len(parsed.FileRefs) != 4 {
		t.Fatalf("unexpected attachment payload: %#v", parsed)
	}
	if parsed.TraceStage.Kind != processTraceKindFileContext || parsed.TraceStage.Status != processTraceStatusReady {
		t.Fatalf("expected file context trace stage, got %#v", parsed.TraceStage)
	}
	if parsed.TraceStage.IncludedCount != 3 || parsed.TraceStage.SkippedCount != 1 {
		t.Fatalf("expected trace stage counts to match attachment groups, got %#v", parsed.TraceStage)
	}
	if parsed.FileRefs[0].FileID != "file_img" || parsed.FileRefs[0].FileName != "diagram.png" {
		t.Fatalf("expected flat file refs to include image identity, got %#v", parsed.FileRefs)
	}
	if len(parsed.FileGroupRefs.DirectImages) != 1 || parsed.FileGroupRefs.DirectImages[0].FileID != "file_img" {
		t.Fatalf("expected direct image group ref, got %#v", parsed.FileGroupRefs.DirectImages)
	}
	if len(parsed.FileGroupRefs.Adaptive) != 1 || parsed.FileGroupRefs.Adaptive[0].FileID != "file_full" {
		t.Fatalf("expected adaptive group ref for auto full-context file, got %#v", parsed.FileGroupRefs.Adaptive)
	}
	if len(parsed.FileGroupRefs.Retrieval) != 1 || parsed.FileGroupRefs.Retrieval[0].FileID != "file_rag" {
		t.Fatalf("expected retrieval group ref, got %#v", parsed.FileGroupRefs.Retrieval)
	}
	if len(parsed.FileGroupRefs.Skipped) != 1 || parsed.FileGroupRefs.Skipped[0].FileID != "file_skip" {
		t.Fatalf("expected skipped group ref, got %#v", parsed.FileGroupRefs.Skipped)
	}
}

func TestBuildAttachmentProcessTraceSummaryWhenAllFilesSkipped(t *testing.T) {
	summary, markdown, _ := buildAttachmentProcessTrace("auto", []AttachmentInput{
		{
			FileID:      "file_skip",
			Kind:        "document",
			FileName:    "huge.pdf",
			MimeType:    "application/pdf",
			ContextMode: fileContextModeSkipped,
		},
	})
	if summary != "未纳入 1 个文件" {
		t.Fatalf("expected all-skipped summary, got %q", summary)
	}
	if strings.Contains(markdown, "已就绪，纳入 1 个文件") {
		t.Fatalf("markdown should not claim skipped file was included: %q", markdown)
	}
	if !strings.Contains(markdown, "文件已就绪，未纳入 1 个文件") {
		t.Fatalf("expected markdown detail to show skipped count, got %q", markdown)
	}
}

func TestBuildCompactionProcessTraceUsesReadableLines(t *testing.T) {
	_, markdown, payload := buildCompactionProcessTrace(&model.ContextSnapshot{
		FromTurn:      1,
		ToTurn:        8,
		SourceTokens:  2400,
		SummaryTokens: 420,
	})
	want := strings.Join([]string{
		"**上下文压缩**：对话已压缩并生成滚动摘要。",
		"- 压缩区间：第 1-8 轮。",
		"- Tokens 缩减：2400 → 420。",
	}, "\n")
	if markdown != want {
		t.Fatalf("unexpected compaction markdown:\n%s", markdown)
	}
	stage, ok := payload[processTracePayloadStage].(map[string]interface{})
	if !ok {
		t.Fatalf("expected compaction trace stage payload, got %#v", payload)
	}
	if stage["kind"] != processTraceKindCompaction || stage["status"] != processTraceStatusCompleted {
		t.Fatalf("unexpected compaction trace stage: %#v", stage)
	}
}

func TestTraceRecorderContinuesProcessTraceAfterRequestCompletion(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:       true,
			ProcessTraceVisibleToUser: true,
		},
		assistant: &model.Message{},
		onEvent:   func(string, map[string]interface{}) error { return nil },
	}
	recorder.appendProcessSection("准备完成", "**准备**：请求已完成。", nil, messageTraceStatusStreaming)

	recorder.completeForBackgroundContinuation()
	if recorder.process == nil || recorder.process.status != messageTraceStatusCompleted {
		t.Fatalf("expected request trace to be completed before background work, got %#v", recorder.process)
	}
	if recorder.onEvent != nil {
		t.Fatal("background continuation must not emit into the closed request stream")
	}

	recorder.appendProcessSection(
		"上下文已压缩",
		"**上下文压缩**：后台压缩完成。",
		map[string]interface{}{processTracePayloadStage: map[string]interface{}{
			"kind":   processTraceKindCompaction,
			"status": processTraceStatusCompleted,
		}},
		messageTraceStatusStreaming,
	)
	if recorder.process.status != messageTraceStatusStreaming || recorder.process.endedAt != nil {
		t.Fatalf("expected background append to reopen the process trace, got %#v", recorder.process)
	}

	recorder.complete()
	if recorder.process.status != messageTraceStatusCompleted || recorder.process.endedAt == nil {
		t.Fatalf("expected background trace to complete, got %#v", recorder.process)
	}
	if !strings.Contains(recorder.process.contentMarkdown, "后台压缩完成") {
		t.Fatalf("expected persisted process content to include background compaction, got %q", recorder.process.contentMarkdown)
	}
}

func TestTraceRecorderKeepsAndReplacesPendingCompactionStage(t *testing.T) {
	recorder := &messageTraceRecorder{
		cfg: config.Config{
			ProcessTraceEnabled:       true,
			ProcessTraceVisibleToUser: true,
		},
		assistant: &model.Message{},
		onEvent:   func(string, map[string]interface{}) error { return nil },
	}
	recorder.appendProcessSection("准备完成", "**准备**：请求已完成。", nil, messageTraceStatusStreaming)
	pendingSummary, pendingPayload := buildPendingCompactionProcessTrace()
	recorder.setCompactionProcessStage(pendingSummary, "", pendingPayload)

	recorder.completeForBackgroundContinuation()
	if recorder.process == nil || recorder.process.status != messageTraceStatusStreaming || recorder.process.endedAt != nil {
		t.Fatalf("expected pending compaction to keep the process trace open, got %#v", recorder.process)
	}
	if recorder.onEvent != nil {
		t.Fatal("background continuation must not emit into the closed request stream")
	}
	stages := normalizeProcessTraceStagePayloads(recorder.process.payload[processTracePayloadStages])
	if len(stages) != 1 || stages[0]["kind"] != processTraceKindCompaction || stages[0]["status"] != processTraceStatusPending {
		t.Fatalf("expected one pending compaction stage, got %#v", stages)
	}

	completedSummary, completedMarkdown, completedPayload := buildCompactionProcessTrace(&model.ContextSnapshot{
		FromTurn:      1,
		ToTurn:        4,
		SourceTokens:  2000,
		SummaryTokens: 300,
	})
	recorder.setCompactionProcessStage(completedSummary, completedMarkdown, completedPayload)
	recorder.complete()

	stages = normalizeProcessTraceStagePayloads(recorder.process.payload[processTracePayloadStages])
	if len(stages) != 1 || stages[0]["status"] != processTraceStatusCompleted {
		t.Fatalf("expected pending stage to be replaced in place, got %#v", stages)
	}
	if recorder.process.status != messageTraceStatusCompleted || recorder.process.endedAt == nil {
		t.Fatalf("expected completed process trace, got %#v", recorder.process)
	}
	if !strings.Contains(recorder.process.contentMarkdown, "Tokens 缩减：2000 → 300") {
		t.Fatalf("expected completed compaction detail, got %q", recorder.process.contentMarkdown)
	}
}

func TestBuildFailedCompactionProcessTraceUsesStructuredStatus(t *testing.T) {
	_, payload := buildFailedCompactionProcessTrace()
	stage, ok := payload[processTracePayloadStage].(map[string]interface{})
	if !ok || stage["kind"] != processTraceKindCompaction || stage["status"] != processTraceStatusFailed {
		t.Fatalf("unexpected failed compaction stage: %#v", payload)
	}
}

func TestMergeTracePayloadAppendsProcessTraceStages(t *testing.T) {
	payload := map[string]interface{}{}
	mergeTracePayload(payload, map[string]interface{}{
		processTracePayloadStage: map[string]interface{}{
			"kind":   processTraceKindFileContext,
			"status": processTraceStatusReady,
		},
	})
	mergeTracePayload(payload, map[string]interface{}{
		processTracePayloadStage: map[string]interface{}{
			"kind":   processTraceKindRetrieval,
			"status": processTraceStatusCompleted,
		},
	})
	stages := normalizeProcessTraceStagePayloads(payload[processTracePayloadStages])
	if len(stages) != 2 {
		t.Fatalf("expected two accumulated trace stages, got %#v", payload)
	}
	if stages[0]["kind"] != processTraceKindFileContext || stages[1]["kind"] != processTraceKindRetrieval {
		t.Fatalf("trace stages were not preserved in append order: %#v", stages)
	}
}

func TestSummarizeToolTraceDraftMatchesRenderedRows(t *testing.T) {
	draft := &messageTraceDraft{
		contentMarkdown: strings.Join([]string{
			"**fetch**：执行失败；10497ms；context deadline exceeded",
			"**fetch**：执行失败；10581ms；context deadline exceeded",
			"**fetch**：执行失败；10464ms；context deadline exceeded",
		}, "\n"),
		payload: map[string]interface{}{
			"tool_calls": []map[string]interface{}{
				{"name": "fetch", "status": "error"},
			},
		},
	}

	if got := summarizeToolTraceDraft(draft); got != "完成 3 次工具调用，3 次失败" {
		t.Fatalf("expected summary to match rendered rows, got %q", got)
	}
}

func TestToolOutputPreviewUsesMCPTextContent(t *testing.T) {
	raw := `{"content":[{"type":"text","text":"找到 3 条相关结果"}]}`
	if got := toolOutputPreview(raw); got != "找到 3 条相关结果" {
		t.Fatalf("expected MCP text content preview, got %q", got)
	}
}

func TestToolOutputPreviewUsesMCPStructuredContent(t *testing.T) {
	raw := `{"structuredContent":{"results":[{"title":"DEEIX Chat 文档","url":"https://example.com/docs"}]}}`
	if got := toolOutputPreview(raw); got != "DEEIX Chat 文档 https://example.com/docs" {
		t.Fatalf("expected MCP structured content preview, got %q", got)
	}
}

func TestToolOutputPreviewParsesJSONTextBlock(t *testing.T) {
	raw := `{"content":[{"type":"text","text":"{\"results\":[{\"title\":\"搜索结果\",\"url\":\"https://example.com\"}]}"}]}`
	if got := toolOutputPreview(raw); got != "搜索结果 https://example.com" {
		t.Fatalf("expected JSON text block preview, got %q", got)
	}
}

func TestToolOutputPreviewFallsBackForNonMCPJSON(t *testing.T) {
	raw := `{"items":[{"message":"普通 JSON 结果"}]}`
	if got := toolOutputPreview(raw); got != "普通 JSON 结果" {
		t.Fatalf("expected generic JSON preview fallback, got %q", got)
	}
}

func TestServerSideOnlyToolsRenderBeforeFinalThinking(t *testing.T) {
	output := &llm.GenerateOutput{
		ServerToolCalls: []llm.ToolCall{{ToolType: "x_search_call", ToolName: "x_search"}},
		Reasoning:       &llm.ReasoningOutput{Text: "final reasoning"},
	}
	if !shouldSyncServerToolsBeforeThinking(output) {
		t.Fatal("expected server-side-only tool response to render tools before final thinking")
	}
	output.ToolCalls = []llm.ToolCall{{ToolType: "function", ToolName: "memory.save"}}
	if shouldSyncServerToolsBeforeThinking(output) {
		t.Fatal("expected local tool-call response to keep thinking before tool execution")
	}
}

func TestToolExecutionLedgerNormalizesArguments(t *testing.T) {
	ledger := newToolExecutionLedger()
	row := model.ToolCall{
		ToolCallID: "call_1",
		ToolName:   "bing_search",
		Status:     "success",
		InputJSON:  `{"query":"DEEIX Chat","count":3}`,
		OutputJSON: `{"content":[{"type":"text","text":"ok"}]}`,
	}
	record := toolExecutionRecord{
		row: row,
		result: llm.ToolResult{
			ToolCallID: row.ToolCallID,
			ToolName:   row.ToolName,
			OutputJSON: row.OutputJSON,
			Status:     row.Status,
		},
	}

	ledger.store(row.ToolName, row.InputJSON, record)
	if _, ok := ledger.lookup("BING_SEARCH", `{"count":3,"query":"DEEIX Chat"}`); !ok {
		t.Fatal("expected ledger lookup to ignore JSON field order and tool name case")
	}
}

func TestBudgetToolOutputForModelKeepsLargeReadableResultWhenItFits(t *testing.T) {
	subtitle := "HEAD " + strings.Repeat("subtitle content ", 4000) + "TAIL"
	raw := `{"content":[{"type":"text","text":"` + subtitle + `"}]}`
	prepared := modelToolOutputForModel(raw)
	if prepared != raw {
		t.Fatalf("expected large readable MCP JSON to remain unchanged, got %d chars", len(prepared))
	}
	if got := budgetToolOutputForModel(prepared, 20_000); got != raw {
		t.Fatalf("expected readable result to remain complete within token budget, got %d chars", len(got))
	}
}

func TestBudgetToolOutputForModelTruncatesByTokenBudget(t *testing.T) {
	raw := "HEAD " + strings.Repeat("subtitle content ", 1000) + "TAIL"
	got := budgetToolOutputForModel(raw, 800)
	if !strings.Contains(got, "omitted to fit the model context") {
		t.Fatalf("expected model-context marker, got %q", got)
	}
	if !strings.Contains(got, "HEAD") || !strings.Contains(got, "TAIL") {
		t.Fatalf("expected token budget to preserve head and tail, got %q", got)
	}
	if tokens := estimateTokens(got); tokens > 800 {
		t.Fatalf("expected result within token budget, got %d tokens", tokens)
	}
}

func TestBudgetToolOutputForModelTruncatesCJKWithinTokenBudget(t *testing.T) {
	raw := "开头 " + strings.Repeat("字幕内容", 2000) + " 结尾"
	got := budgetToolOutputForModel(raw, 500)
	if !strings.Contains(got, "开头") || !strings.Contains(got, "结尾") {
		t.Fatalf("expected CJK result to preserve head and tail")
	}
	if tokens := estimateTokens(got); tokens > 500 {
		t.Fatalf("expected CJK result within token budget, got %d tokens", tokens)
	}
}

func TestModelToolOutputForModelOmitsNestedOpaquePayload(t *testing.T) {
	raw := `{"content":[{"type":"text","text":"subtitle"},{"type":"image","data":"` + strings.Repeat("A", 4096) + `"}]}`
	got := modelToolOutputForModel(raw)
	if !strings.Contains(got, "subtitle") || !strings.Contains(got, "Opaque tool payload omitted") {
		t.Fatalf("expected readable text and opaque payload summary, got %q", got)
	}
	if strings.Contains(got, strings.Repeat("A", 512)) {
		t.Fatalf("expected nested opaque payload to be removed, got %d chars", len(got))
	}
}

func TestSanitizeOpaqueToolOutputOmitsSmallDataURL(t *testing.T) {
	raw := `{"content":[{"type":"text","text":"ok"}],"structuredContent":{"image":"data:image/png;base64,QUJD"}}`
	got := sanitizeOpaqueToolOutput(raw)
	if !strings.Contains(got, `"text":"ok"`) || !strings.Contains(got, "Opaque tool payload omitted") {
		t.Fatalf("expected readable output with the data URL removed, got %q", got)
	}
	if strings.Contains(got, ";base64,") {
		t.Fatalf("expected data URL bytes to be removed, got %q", got)
	}
}

func TestModelToolOutputForModelPreservesLargeInteger(t *testing.T) {
	raw := `{"id":9007199254740993,"image":"` + strings.Repeat("A", 4096) + `"}`
	got := modelToolOutputForModel(raw)
	if !strings.Contains(got, `"id":9007199254740993`) {
		t.Fatalf("expected large integer to retain its exact value, got %q", got)
	}
	if !strings.Contains(got, "Opaque tool payload omitted") {
		t.Fatalf("expected opaque payload to remain sanitized, got %q", got)
	}
}

func TestEnforceToolResultAggregateBudgetSupportsZeroBudget(t *testing.T) {
	slots := []toolExecutionSlot{{
		result: llm.ToolResult{OutputJSON: "tool output", Error: "tool error"},
	}}
	enforceToolResultAggregateBudget(slots, 0)
	if tokens := toolResultModelTokens(slots[0].result); tokens != 0 {
		t.Fatalf("expected zero-budget result to be empty, got %d tokens", tokens)
	}
}

func TestEnforceToolResultAggregateBudgetKeepsSmallResults(t *testing.T) {
	large := "HEAD " + strings.Repeat("large result ", 1000) + " TAIL"
	small := strings.Repeat("small ", 40)
	slots := []toolExecutionSlot{
		{
			row: model.ToolCall{
				ToolCallID: "call_a",
				ToolName:   "tool_a",
				RunID:      "run_1",
				Status:     "success",
				OutputJSON: large,
			},
			result: llm.ToolResult{ToolCallID: "call_a", OutputJSON: large, Status: "success"},
		},
		{
			row: model.ToolCall{
				ToolCallID: "call_b",
				ToolName:   "tool_b",
				RunID:      "run_1",
				Status:     "success",
				OutputJSON: large,
			},
			result: llm.ToolResult{ToolCallID: "call_b", OutputJSON: large, Status: "success"},
		},
		{
			row: model.ToolCall{
				ToolCallID: "call_c",
				ToolName:   "tool_c",
				RunID:      "run_1",
				Status:     "success",
				OutputJSON: small,
			},
			result: llm.ToolResult{ToolCallID: "call_c", OutputJSON: small, Status: "success"},
		},
	}

	enforceToolResultAggregateBudget(slots, 1000)

	total := int64(0)
	for _, slot := range slots {
		total += toolResultModelTokens(slot.result)
	}
	if total > 1000 {
		t.Fatalf("expected aggregate model output within budget, got %d tokens", total)
	}
	if slots[2].result.OutputJSON != small {
		t.Fatalf("expected small result to remain complete, got %q", slots[2].result.OutputJSON)
	}
	for _, index := range []int{0, 1} {
		if !strings.Contains(slots[index].result.OutputJSON, "HEAD") || !strings.Contains(slots[index].result.OutputJSON, "TAIL") {
			t.Fatalf("expected large result %d to retain head and tail", index)
		}
	}
}
