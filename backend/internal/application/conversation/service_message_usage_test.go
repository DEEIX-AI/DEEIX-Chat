package conversation

import (
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestMessageUsageAccumulatorCombinesObservedAndUnobservedInput(t *testing.T) {
	accumulator := &messageUsageAccumulator{}

	firstCallMessages := []llm.Message{{Role: "user", Content: "hello"}}
	accumulator.beginCall(llm.GenerateInput{Messages: firstCallMessages})
	accumulator.addObservedUsage(llm.Usage{InputTokens: 12, OutputTokens: 3})

	if got := accumulator.interruptedInputTokens(); got != 12 {
		t.Fatalf("expected observed input tokens after first call, got %d", got)
	}

	secondCallMessages := []llm.Message{{Role: "tool", Content: "tool result"}}
	secondCallInput := llm.GenerateInput{Messages: secondCallMessages}
	secondCallEstimate := estimateGenerateInputTokens(secondCallInput)
	accumulator.beginCall(secondCallInput)
	accumulator.finishCall(false)

	want := int64(12) + secondCallEstimate
	if got := accumulator.interruptedInputTokens(); got != want {
		t.Fatalf("expected observed plus unobserved input tokens, got %d want %d", got, want)
	}
	if got := accumulator.effectiveInputTokens(0); got != want {
		t.Fatalf("expected effective input tokens to include unobserved estimate, got %d want %d", got, want)
	}
}

func TestResolveObservedOrHigherEstimatedTokensKeepsLargerEstimate(t *testing.T) {
	if got := resolveObservedOrHigherEstimatedTokens(40, 96); got != 96 {
		t.Fatalf("expected larger input estimate, got %d", got)
	}
	if got := resolveObservedOrHigherEstimatedOutputTokens(2, "hello world this is a longer streamed response"); got <= 2 {
		t.Fatalf("expected output estimate to cover partial observed usage, got %d", got)
	}
}

func TestResolveObservedOrEstimatedTokensPrefersObservedUsage(t *testing.T) {
	if got := resolveObservedOrEstimatedTokens(40, 96); got != 40 {
		t.Fatalf("expected successful usage path to prefer observed tokens, got %d", got)
	}
	if got := resolveObservedOrEstimatedOutputTokens(7, "hello world this is a longer streamed response"); got != 7 {
		t.Fatalf("expected successful output usage path to prefer observed tokens, got %d", got)
	}
}

func TestEstimateGenerateInputTokensIncludesInstructionsAndTools(t *testing.T) {
	input := llm.GenerateInput{
		Messages:     []llm.Message{{Role: "user", Content: "hello"}},
		Instructions: "answer tersely",
		Tools: []llm.ToolDefinition{{
			Name:        "lookup",
			Description: "Search docs",
			InputSchema: []byte(`{"type":"object","properties":{"query":{"type":"string"}}}`),
		}},
	}

	messageOnly := estimatePromptTokens(input.Messages)
	withInputShape := estimateGenerateInputTokens(input)
	if withInputShape <= messageOnly {
		t.Fatalf("expected generate input estimate to include instructions and tools, got %d <= %d", withInputShape, messageOnly)
	}

	input.DisableTools = true
	withoutTools := estimateGenerateInputTokens(input)
	if withoutTools >= withInputShape {
		t.Fatalf("expected disabled tools to be excluded from estimate, got %d >= %d", withoutTools, withInputShape)
	}
}

func TestEstimateGenerateInputTokensIncludesToolCallsAndResults(t *testing.T) {
	withoutTools := estimatePromptTokens([]llm.Message{{Role: "assistant"}})
	withTools := estimatePromptTokens([]llm.Message{
		{
			Role: "assistant",
			ToolCalls: []llm.ToolCall{{
				ToolCallID:    "call_1",
				ToolName:      "fetch_transcript",
				ArgumentsJSON: `{"url":"https://example.com/video"}`,
			}},
		},
		{
			Role: "tool",
			ToolResults: []llm.ToolResult{{
				ToolCallID: "call_1",
				ToolName:   "fetch_transcript",
				OutputJSON: strings.Repeat("subtitle ", 100),
			}},
		},
	})
	if withTools <= withoutTools {
		t.Fatalf("expected tool calls and results to increase prompt estimate, got %d <= %d", withTools, withoutTools)
	}
}

func TestResolveToolResultTokenBudgetUsesRemainingModelContext(t *testing.T) {
	generateInput := llm.GenerateInput{
		Tools: []llm.ToolDefinition{{
			Name:        "fetch_transcript",
			Description: "Fetch a video transcript",
			InputSchema: []byte(`{"type":"object"}`),
		}},
	}
	pendingAssistant := llm.Message{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ToolCallID:    "call_1",
			ToolName:      "fetch_transcript",
			ArgumentsJSON: "{}",
		}},
	}
	capabilities := `{"contextWindow":64000,"maxOutputTokens":12000}`
	shortBudget := resolveToolResultTokenBudget(
		generateInput,
		[]llm.Message{{Role: "user", Content: "summarize this video"}},
		pendingAssistant,
		"custom-model",
		capabilities,
	)
	longBudget := resolveToolResultTokenBudget(
		generateInput,
		[]llm.Message{{Role: "user", Content: strings.Repeat("history ", 20_000)}},
		pendingAssistant,
		"custom-model",
		capabilities,
	)
	effectiveBudget := int64(llm.EffectiveContextBudgetFromCapabilities("custom-model", capabilities))
	if shortBudget <= 0 || shortBudget >= effectiveBudget {
		t.Fatalf("expected short prompt to leave a bounded tool budget, got %d of %d", shortBudget, effectiveBudget)
	}
	if longBudget >= shortBudget {
		t.Fatalf("expected longer prompt to reduce tool result budget, got %d >= %d", longBudget, shortBudget)
	}
	withOldHistoryBudget := resolveToolResultTokenBudget(
		generateInput,
		[]llm.Message{
			{Role: "user", Content: strings.Repeat("old history ", 20_000)},
			{Role: "assistant", Content: "old answer"},
			{Role: "user", Content: "summarize this video"},
		},
		pendingAssistant,
		"custom-model",
		capabilities,
	)
	if withOldHistoryBudget != shortBudget {
		t.Fatalf("expected old history to be reclaimable for the current tool result, got %d want %d", withOldHistoryBudget, shortBudget)
	}
}

func TestResolveToolResultTokenBudgetDoesNotExceedRemainingContext(t *testing.T) {
	pendingAssistant := llm.Message{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ToolCallID:    "call_1",
			ToolName:      "fetch_transcript",
			ArgumentsJSON: "{}",
		}},
	}
	capabilities := `{"contextWindow":32000,"maxOutputTokens":4000}`
	effectiveBudget := int64(llm.EffectiveContextBudgetFromCapabilities("custom-model", capabilities))
	baseBudget := resolveToolResultTokenBudget(
		llm.GenerateInput{},
		[]llm.Message{{Role: "user", Content: "summarize this video"}},
		pendingAssistant,
		"custom-model",
		capabilities,
	)
	fillerTokens := baseBudget - 512
	if fillerTokens <= 0 {
		t.Fatalf("expected enough baseline budget for boundary test, got %d", baseBudget)
	}
	messages := []llm.Message{{
		Role:    "user",
		Content: "summarize this video " + strings.Repeat("x", int(fillerTokens*4)),
	}}
	remaining := resolveToolResultTokenBudget(
		llm.GenerateInput{},
		messages,
		pendingAssistant,
		"custom-model",
		capabilities,
	)
	if remaining < 0 || remaining >= 1024 {
		t.Fatalf("expected strict sub-1024 remaining budget, got %d", remaining)
	}

	placeholder := llm.Message{Role: "tool", ToolResults: []llm.ToolResult{{
		ToolCallID: "call_1",
		ToolName:   "fetch_transcript",
		OutputJSON: "{}",
	}}}
	fixedTokens := estimateToolFollowUpInputTokens(
		llm.GenerateInput{},
		append(messages, pendingAssistant, placeholder),
	)
	if fixedTokens+remaining > effectiveBudget {
		t.Fatalf("expected tool budget to stay within effective context, got %d > %d", fixedTokens+remaining, effectiveBudget)
	}
}

func TestResolveToolResultTokenBudgetReclaimsPreviousToolPayloads(t *testing.T) {
	pendingAssistant := llm.Message{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ToolCallID:    "call_2",
			ToolName:      "fetch_details",
			ArgumentsJSON: "{}",
		}},
	}
	capabilities := `{"contextWindow":32000,"maxOutputTokens":4000}`
	withSmallPreviousResult := []llm.Message{
		{Role: "user", Content: "summarize this video"},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{ToolCallID: "call_1", ToolName: "fetch_transcript"}}},
		{Role: "tool", ToolResults: []llm.ToolResult{{
			ToolCallID: "call_1",
			ToolName:   "fetch_transcript",
			OutputJSON: "{}",
		}}},
	}
	withLargePreviousResult := append([]llm.Message(nil), withSmallPreviousResult...)
	withLargePreviousResult[2].ToolResults = []llm.ToolResult{{
		ToolCallID: "call_1",
		ToolName:   "fetch_transcript",
		OutputJSON: strings.Repeat("subtitle ", 20_000),
	}}

	smallBudget := resolveToolResultTokenBudget(
		llm.GenerateInput{},
		withSmallPreviousResult,
		pendingAssistant,
		"custom-model",
		capabilities,
	)
	largeBudget := resolveToolResultTokenBudget(
		llm.GenerateInput{},
		withLargePreviousResult,
		pendingAssistant,
		"custom-model",
		capabilities,
	)
	if largeBudget != smallBudget {
		t.Fatalf("expected previous payloads to be reclaimable, got %d want %d", largeBudget, smallBudget)
	}
}

func TestRebalanceToolFollowUpResultsFitsCurrentRoundWithinContext(t *testing.T) {
	capabilities := `{"contextWindow":32000,"maxOutputTokens":4000}`
	firstOutput := "FIRST_HEAD " + strings.Repeat("first result ", 10_000) + " FIRST_TAIL"
	secondOutput := "SECOND_HEAD " + strings.Repeat("second result ", 10_000) + " SECOND_TAIL"
	messages := []llm.Message{
		{Role: "system", Content: "policy"},
		{Role: "user", Content: "compare both tool results"},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{ToolCallID: "call_1", ToolName: "first"}}},
		{Role: "tool", ToolResults: []llm.ToolResult{{ToolCallID: "call_1", ToolName: "first", OutputJSON: firstOutput}}},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{ToolCallID: "call_2", ToolName: "second"}}},
		{Role: "tool", ToolResults: []llm.ToolResult{{ToolCallID: "call_2", ToolName: "second", OutputJSON: secondOutput}}},
	}

	rebalanced, changed := rebalanceToolFollowUpResults(
		llm.GenerateInput{},
		messages,
		"custom-model",
		capabilities,
	)
	if !changed {
		t.Fatal("expected oversized current tool round to be rebalanced")
	}
	effectiveBudget := int64(llm.EffectiveContextBudgetFromCapabilities("custom-model", capabilities))
	if tokens := estimateToolFollowUpInputTokens(llm.GenerateInput{}, rebalanced); tokens > effectiveBudget {
		t.Fatalf("expected rebalanced follow-up within context, got %d > %d", tokens, effectiveBudget)
	}
	for _, item := range []struct {
		result llm.ToolResult
		head   string
		tail   string
	}{
		{result: rebalanced[3].ToolResults[0], head: "FIRST_HEAD", tail: "FIRST_TAIL"},
		{result: rebalanced[5].ToolResults[0], head: "SECOND_HEAD", tail: "SECOND_TAIL"},
	} {
		if !strings.Contains(item.result.OutputJSON, item.head) || !strings.Contains(item.result.OutputJSON, item.tail) {
			t.Fatalf("expected rebalanced result to preserve head and tail, got %q", item.result.OutputJSON)
		}
	}
	if messages[3].ToolResults[0].OutputJSON != firstOutput || messages[5].ToolResults[0].OutputJSON != secondOutput {
		t.Fatal("expected rebalancing to leave the source messages unchanged")
	}
}

func TestTrimToolFollowUpHistoryRemovesOldCompleteTurns(t *testing.T) {
	capabilities := `{"contextWindow":32000,"maxOutputTokens":4000}`
	messages := []llm.Message{
		{Role: "system", Content: "policy"},
		{Role: "user", Content: strings.Repeat("old history ", 20_000)},
		{Role: "assistant", Content: "old answer"},
		{Role: "user", Content: "summarize this video"},
		{Role: "assistant", ToolCalls: []llm.ToolCall{{ToolCallID: "call_1", ToolName: "fetch_transcript"}}},
		{Role: "tool", ToolResults: []llm.ToolResult{{
			ToolCallID: "call_1",
			ToolName:   "fetch_transcript",
			OutputJSON: strings.Repeat("subtitle ", 2000),
		}}},
	}
	trimmed, changed := trimToolFollowUpHistory(llm.GenerateInput{}, messages, "custom-model", capabilities)
	if !changed {
		t.Fatal("expected oversized follow-up context to trim old history")
	}
	if len(trimmed) != 4 || trimmed[0].Role != "system" || trimmed[1].Content != "summarize this video" {
		t.Fatalf("expected system prefix and current turn only, got %#v", trimmed)
	}
	if tokens := estimateToolFollowUpInputTokens(llm.GenerateInput{}, trimmed); tokens >
		int64(llm.EffectiveContextBudgetFromCapabilities("custom-model", capabilities)) {
		t.Fatalf("expected trimmed follow-up within effective model budget, got %d tokens", tokens)
	}
}

func TestSendMessageBillingDurationSeconds(t *testing.T) {
	if got := sendMessageBillingDurationSeconds(&SendMessageResult{DurationSeconds: 5}, 1200); got != 5 {
		t.Fatalf("expected explicit duration seconds to win, got %d", got)
	}
	if got := sendMessageBillingDurationSeconds(&SendMessageResult{}, 1201); got != 2 {
		t.Fatalf("expected latency to be rounded up to seconds, got %d", got)
	}
	if got := sendMessageBillingDurationSeconds(&SendMessageResult{}, 0); got != 0 {
		t.Fatalf("expected empty duration for zero latency, got %d", got)
	}
}

func TestMediaDurationSecondsFromOptions(t *testing.T) {
	if got := mediaDurationSecondsFromOptions(map[string]interface{}{"durationSeconds": float64(5)}); got != 5 {
		t.Fatalf("expected numeric duration seconds, got %d", got)
	}
	if got := mediaDurationSecondsFromOptions(map[string]interface{}{"duration": "5.2s"}); got != 6 {
		t.Fatalf("expected string duration to round up, got %d", got)
	}
	if got := mediaDurationSecondsFromOptions(map[string]interface{}{"duration": "bad"}); got != 0 {
		t.Fatalf("expected invalid duration to be ignored, got %d", got)
	}
}
