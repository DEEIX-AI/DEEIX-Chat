package conversation

import (
	"context"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestExecuteToolCallRejectsToolsNotEnabledForRun(t *testing.T) {
	svc := &Service{}
	_, err := svc.executeToolCall(context.Background(), ExecuteToolInput{
		ToolName:      "memory.upsert",
		ArgumentsJSON: `{"memory_key":"k","value":"v"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "not enabled for this run") {
		t.Fatalf("expected disabled tool error, got %v", err)
	}
}

func TestExecuteAssistantToolCallsStopsWhenToolNotEnabledForRun(t *testing.T) {
	svc := &Service{}
	result := svc.executeAssistantToolCalls(context.Background(), executeAssistantToolCallsInput{
		RunID: "run_1",
		ToolCalls: []llm.ToolCall{{
			ToolCallID:    "toolu_1",
			ToolType:      "function",
			ToolName:      "web_search",
			ArgumentsJSON: `{"query":"weather"}`,
			Status:        "requested",
		}},
	})

	if result.FatalErr == nil || !strings.Contains(result.FatalErr.Error(), "not enabled for this run") {
		t.Fatalf("expected fatal disabled tool error, got %v", result.FatalErr)
	}
	if len(result.Rows) != 1 || result.Rows[0].Status != "error" || result.Rows[0].ToolName != "web_search" {
		t.Fatalf("expected one failed tool row, got %#v", result.Rows)
	}
	if len(result.ToolResults) != 1 || result.ToolResults[0].Status != "error" {
		t.Fatalf("expected failed model tool result, got %#v", result.ToolResults)
	}
}

func TestResolveMaxLLMCallsPerRunRequiresFollowUpRound(t *testing.T) {
	svc := &Service{cfg: config.NewRuntime(config.Config{MCPMaxLLMCallsPerRun: 1})}
	if got := svc.resolveMaxLLMCallsPerRun(); got != 2 {
		t.Fatalf("expected minimum LLM calls per run to be 2, got %d", got)
	}
}

func TestValidateSelectedToolIDsUsesRuntimeLimit(t *testing.T) {
	service := &Service{cfg: config.NewRuntime(config.Config{MCPMaxSelectedToolsPerMessage: 2})}

	if err := service.ValidateSelectedToolIDs([]uint{1, 2}); err != nil {
		t.Fatalf("expected two selected tools to pass, got %v", err)
	}
	if err := service.ValidateSelectedToolIDs([]uint{1, 2, 3}); err != ErrTooManySelectedTools {
		t.Fatalf("expected ErrTooManySelectedTools, got %v", err)
	}
}

func TestDiffLLMUsageTreatsStreamUsageAsCallCumulative(t *testing.T) {
	previous := llm.Usage{
		InputTokens:     10,
		OutputTokens:    3,
		CacheReadTokens: 2,
		ReasoningTokens: 1,
		Speed:           "standard",
		ServiceTier:     "default",
	}
	current := llm.Usage{
		InputTokens:     18,
		OutputTokens:    7,
		CacheReadTokens: 2,
		ReasoningTokens: 4,
		Speed:           "fast",
		ServiceTier:     "priority",
	}

	got := diffLLMUsage(current, previous)
	if got.InputTokens != 8 || got.OutputTokens != 4 || got.CacheReadTokens != 0 || got.ReasoningTokens != 3 {
		t.Fatalf("unexpected usage delta: %#v", got)
	}
	if got.Speed != "fast" || got.ServiceTier != "priority" {
		t.Fatalf("expected latest usage metadata to be kept, got %#v", got)
	}
}

func TestAddServerSideToolUsageAggregatesPositiveCounts(t *testing.T) {
	got := addServerSideToolUsage(
		map[string]int64{"web_search": 1, "ignored": 0},
		map[string]int64{"web_search": 2, "code_interpreter": 1, " ": 3},
	)

	if got["web_search"] != 3 || got["code_interpreter"] != 1 {
		t.Fatalf("unexpected server-side tool usage: %#v", got)
	}
	if _, ok := got["ignored"]; ok {
		t.Fatalf("expected non-positive usage to be ignored: %#v", got)
	}
}

func TestSyncUpstreamOutputThinkingDoesNotReturnThinkingOnlyContent(t *testing.T) {
	output := &llm.GenerateOutput{
		Text: "<think>Need to call a tool.</think>",
		ToolCalls: []llm.ToolCall{{
			ToolCallID:    "call_1",
			ToolType:      "function",
			ToolName:      "memory.list",
			ArgumentsJSON: "{}",
			Status:        "requested",
		}},
	}

	if got := syncUpstreamOutputThinking(nil, output); got != "" {
		t.Fatalf("expected thinking-only tool call content to stay out of assistant text, got %q", got)
	}
}

func TestOutputReasoningContentPrefersStructuredReasoning(t *testing.T) {
	output := &llm.GenerateOutput{
		Reasoning: &llm.ReasoningOutput{Text: "need a tool"},
		Text:      "<think>fallback</think>",
	}

	got := outputReasoningContent(output)
	if got != "need a tool" {
		t.Fatalf("expected structured reasoning content, got %q", got)
	}

	got = outputReasoningContent(&llm.GenerateOutput{Text: "<think>fallback</think>"})
	if got != "fallback" {
		t.Fatalf("expected parsed thinking fallback, got %q", got)
	}
}

func TestToolRunFinalAnswerMissingWhenBudgetEndsWithStructuredToolCall(t *testing.T) {
	output := &llm.GenerateOutput{
		ToolCalls: []llm.ToolCall{{
			ToolCallID:    "call_1",
			ToolType:      "function",
			ToolName:      "search",
			ArgumentsJSON: `{"query":"mcp"}`,
			Status:        "requested",
		}},
	}

	if !toolRunFinalAnswerMissing(output, true, 5, 5, 1) {
		t.Fatalf("expected exhausted tool run with pending tool call to be missing a final answer")
	}
}

func TestToolRunFinalAnswerMissingAcceptsNaturalFinalAnswer(t *testing.T) {
	text := "没有更多工具调用空间时，应基于已获取的结果直接回答。"

	if toolRunFinalAnswerMissing(&llm.GenerateOutput{Text: text}, true, 5, 5, 1) {
		t.Fatalf("expected natural final answer to be accepted")
	}
}
