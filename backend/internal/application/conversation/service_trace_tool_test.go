package conversation

import (
	"encoding/json"
	"strings"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
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

func TestToolOutputTextUsesReadableSearchResults(t *testing.T) {
	raw := `[{"url":"https://example.com/a"},{"title":"新闻","url":"https://example.com/b"}]`
	if got := toolOutputText(raw); got != "https://example.com/a；新闻 https://example.com/b" {
		t.Fatalf("expected readable search result text, got %q", got)
	}
}

func TestCitationsToolOutputJSONBuildsSearchOutput(t *testing.T) {
	raw := citationsToolOutputJSON([]string{"https://example.com/a", " ", "https://example.com/b"})
	if got := toolOutputText(raw); got != "https://example.com/a；https://example.com/b" {
		t.Fatalf("expected citations output text, got %q from raw %q", got, raw)
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

func TestBudgetToolOutputForModelKeepsSmallResults(t *testing.T) {
	raw := `{"content":[{"type":"text","text":"small result"}]}`
	if got := budgetToolOutputForModel(raw, 100); got != raw {
		t.Fatalf("expected small tool result to stay unchanged, got %q", got)
	}
}

func TestBudgetToolOutputForModelWrapsLargeResults(t *testing.T) {
	raw := `{"content":[{"type":"text","text":"` + strings.Repeat("a", 80) + `"}]}`
	got := budgetToolOutputForModel(raw, 40)
	if !strings.Contains(got, "truncated_for_model") {
		t.Fatalf("expected budgeted result marker, got %q", got)
	}
	if !strings.Contains(got, "full result is retained") {
		t.Fatalf("expected retention note, got %q", got)
	}
}
