package conversation

import (
	"errors"
	"strings"
	"testing"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
)

func TestMessageUsageAccumulatorCombinesObservedAndUnobservedInput(t *testing.T) {
	accumulator := &messageUsageAccumulator{}

	firstCallMessages := []llm.Message{{Role: "user", Content: "hello"}}
	accumulator.beginCall(estimateGenerateInputTokens(llm.GenerateInput{Messages: firstCallMessages}))
	accumulator.addObservedUsage(llm.Usage{InputTokens: 12, OutputTokens: 3})

	if got := accumulator.interruptedInputTokens(); got != 12 {
		t.Fatalf("expected observed input tokens after first call, got %d", got)
	}

	secondCallMessages := []llm.Message{{Role: "tool", Content: "tool result"}}
	secondCallInput := llm.GenerateInput{Messages: secondCallMessages}
	secondCallEstimate := estimateGenerateInputTokens(secondCallInput)
	accumulator.beginCall(secondCallEstimate)
	accumulator.finishCall(false, true)

	want := int64(12) + secondCallEstimate
	if got := accumulator.interruptedInputTokens(); got != want {
		t.Fatalf("expected observed plus unobserved input tokens, got %d want %d", got, want)
	}
	if got := accumulator.effectiveInputTokens(0); got != want {
		t.Fatalf("expected effective input tokens to include unobserved estimate, got %d want %d", got, want)
	}
}

func TestMessageUsageAccumulatorKeepsFullyCachedInputObserved(t *testing.T) {
	input := llm.GenerateInput{Messages: []llm.Message{
		{Role: "system", Content: strings.Repeat("long system prompt ", 400)},
		{Role: "user", Content: "hi"},
	}}
	if estimateGenerateInputTokens(input) <= 0 {
		t.Fatal("expected a positive prompt estimate for the regression input")
	}
	fullyCached := llm.Usage{CacheReadTokens: 3355, OutputTokens: 23, ReasoningTokens: 49}
	const promptFallback = int64(4068)

	tests := map[string]func(accumulator *messageUsageAccumulator){
		"streaming": func(accumulator *messageUsageAccumulator) {
			accumulator.beginCall(estimateGenerateInputTokens(input))
			accumulator.addObservedUsage(fullyCached)
			accumulator.finishCall(fullyCached.HasObservedInput(), fullyCached.HasObservedOutput())
		},
		"non-streaming": func(accumulator *messageUsageAccumulator) {
			accumulator.beginCall(estimateGenerateInputTokens(input))
			accumulator.finishCall(fullyCached.HasObservedInput(), fullyCached.HasObservedOutput())
			accumulator.setObservedUsage(fullyCached)
		},
	}
	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			accumulator := &messageUsageAccumulator{}
			run(accumulator)

			if got := accumulator.effectiveInputTokens(promptFallback); got != 0 {
				t.Fatalf("expected fully cached prompt to bill zero non-cached input, got %d", got)
			}
			if got := accumulator.interruptedInputTokens(); got != 0 {
				t.Fatalf("expected no estimated input once upstream reported cached input, got %d", got)
			}
			if got := accumulator.usage().CacheReadTokens; got != fullyCached.CacheReadTokens {
				t.Fatalf("expected cache read tokens to be preserved, got %d", got)
			}
		})
	}
}

func TestMessageUsageAccumulatorFallsBackToEstimateWithoutObservedInput(t *testing.T) {
	input := llm.GenerateInput{Messages: []llm.Message{{Role: "user", Content: "hello"}}}
	accumulator := &messageUsageAccumulator{}

	if got := accumulator.effectiveInputTokens(4068); got != 4068 {
		t.Fatalf("expected prompt fallback before any call, got %d", got)
	}

	accumulator.beginCall(estimateGenerateInputTokens(input))
	accumulator.addObservedUsage(llm.Usage{OutputTokens: 5})
	accumulator.finishCall(false, true)

	want := estimateGenerateInputTokens(input)
	if got := accumulator.effectiveInputTokens(4068); got != want {
		t.Fatalf("expected output-only usage to keep the call estimate, got %d want %d", got, want)
	}
	if got := accumulator.interruptedInputTokens(); got != want {
		t.Fatalf("expected interrupted input to include the unobserved estimate, got %d want %d", got, want)
	}
}

func TestEstimateBillableInputTokensUsesFullContextForStatefulCalls(t *testing.T) {
	fullMessages := []llm.Message{
		{Role: "system", Content: strings.Repeat("policy ", 200)},
		{Role: "user", Content: strings.Repeat("earlier question ", 200)},
		{Role: "assistant", Content: strings.Repeat("earlier answer ", 200)},
		{Role: "user", Content: "what about now?"},
	}
	tools := []llm.ToolDefinition{{
		Name:        "lookup",
		Description: "Search docs",
		InputSchema: []byte(`{"type":"object"}`),
	}}
	fullInput := llm.GenerateInput{Messages: fullMessages, Tools: tools}
	fullEstimate := estimateGenerateInputTokens(fullInput)

	statefulInput := llm.GenerateInput{
		Messages:           fullMessages[len(fullMessages)-1:],
		Tools:              tools,
		PreviousResponseID: "resp_previous",
	}
	if got := estimateBillableInputTokens(statefulInput, fullMessages); got != fullEstimate {
		t.Fatalf("expected stateful continuation to be estimated on the full context, got %d want %d", got, fullEstimate)
	}

	toolFollowUp := llm.GenerateInput{
		Messages:           []llm.Message{{Role: "tool", ToolResults: []llm.ToolResult{{ToolCallID: "call_1", ToolName: "lookup", OutputJSON: "{}"}}}},
		Tools:              tools,
		PreviousResponseID: "resp_tool_round",
	}
	followUpHistory := append(append([]llm.Message(nil), fullMessages...),
		llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ToolCallID: "call_1", ToolName: "lookup", ArgumentsJSON: "{}"}}},
		toolFollowUp.Messages[0],
	)
	if got := estimateBillableInputTokens(toolFollowUp, followUpHistory); got <= fullEstimate {
		t.Fatalf("expected stateful tool follow-up to cover the whole history plus the tool round, got %d <= %d", got, fullEstimate)
	}

	if got := estimateBillableInputTokens(fullInput, nil); got != fullEstimate {
		t.Fatalf("expected non-stateful calls to be estimated on the sent request, got %d want %d", got, fullEstimate)
	}
}

// 结算重试只在可幂等时进行：有预留靠预留状态机回读，无预留靠账本运行级幂等键回读，
// 两者都没有则只执行一次，避免重复入账。
func TestUsageRecordIsIdempotent(t *testing.T) {
	reservation := &domainbilling.UsageAuthorization{Mode: "usage", RefNo: "run_1", Reservation: &domainbilling.UsageBalanceReservation{RefNo: "run_1"}}
	if !usageRecordIsIdempotent(&domainbilling.UsageLedger{}, reservation) {
		t.Fatal("expected settlement with a reservation to be retryable")
	}
	if !usageRecordIsIdempotent(&domainbilling.UsageLedger{RefNo: "run_1"}, &domainbilling.UsageAuthorization{Mode: "self", RefNo: "run_1"}) {
		t.Fatal("expected keyed ledger without reservation to be retryable")
	}
	if usageRecordIsIdempotent(&domainbilling.UsageLedger{}, &domainbilling.UsageAuthorization{Mode: "self"}) {
		t.Fatal("expected unkeyed ledger without reservation to run once")
	}
	if usageRecordIsIdempotent(nil, nil) {
		t.Fatal("expected missing ledger to run once")
	}
}

func TestModerationBlockedBilledReason(t *testing.T) {
	paid := &domainbilling.UsageAuthorization{Mode: "usage", RefNo: "run_1", Reservation: &domainbilling.UsageBalanceReservation{RefNo: "run_1"}}
	selfHosted := &domainbilling.UsageAuthorization{Mode: "self", RefNo: "run_1"}
	blocked := func(billable bool) *SendMessageResult {
		return &SendMessageResult{Billable: billable, Moderation: &MessageModerationOutcome{Blocked: true, Direction: "input"}}
	}

	if got := ModerationBlockedBilledReason(blocked(true), paid); got != appbilling.BilledReasonModerationBlockedUpstreamUsage {
		t.Fatalf("expected billable blocked run with reservation to be annotated, got %q", got)
	}
	if got := ModerationBlockedBilledReason(blocked(false), paid); got != "" {
		t.Fatalf("expected blocked run without billable usage to stay unannotated, got %q", got)
	}
	if got := ModerationBlockedBilledReason(blocked(true), selfHosted); got != "" {
		t.Fatalf("expected self-hosted run to stay unannotated, got %q", got)
	}
	if got := ModerationBlockedBilledReason(&SendMessageResult{Billable: true}, paid); got != "" {
		t.Fatalf("expected non-blocked run to stay unannotated, got %q", got)
	}
	if got := ModerationBlockedBilledReason(nil, paid); got != "" {
		t.Fatalf("expected nil result to stay unannotated, got %q", got)
	}
}

func TestModerationLiveEmitterAnnotatesOutputBlocksOnly(t *testing.T) {
	paid := &domainbilling.UsageAuthorization{Mode: "usage", RefNo: "run_1", Reservation: &domainbilling.UsageBalanceReservation{RefNo: "run_1"}}
	var emitted []map[string]any
	emit := moderationLiveEmitter(func(_ string, payload map[string]any) error {
		emitted = append(emitted, payload)
		return nil
	}, paid)

	emit("moderation_blocked", map[string]any{"direction": "output"})
	emit("moderation_blocked", map[string]any{"direction": "input"})
	emit("moderation_checking", map[string]any{})

	if len(emitted) != 3 {
		t.Fatalf("expected every event to be forwarded, got %d", len(emitted))
	}
	if got := emitted[0]["billedReason"]; got != appbilling.BilledReasonModerationBlockedUpstreamUsage {
		t.Fatalf("expected output block to carry billed reason, got %v", got)
	}
	if _, ok := emitted[1]["billedReason"]; ok {
		t.Fatal("expected input block to defer the billing note to settlement")
	}
	if _, ok := emitted[2]["billedReason"]; ok {
		t.Fatal("expected non-block events to stay untouched")
	}

	var unpaid []map[string]any
	moderationLiveEmitter(func(_ string, payload map[string]any) error {
		unpaid = append(unpaid, payload)
		return nil
	}, &domainbilling.UsageAuthorization{Mode: "self"})("moderation_blocked", map[string]any{"direction": "output"})
	if _, ok := unpaid[0]["billedReason"]; ok {
		t.Fatal("expected self-hosted output block to stay unannotated")
	}
}

func TestSendMessageBillingCallCount(t *testing.T) {
	if got := sendMessageBillingCallCount(nil); got != 1 {
		t.Fatalf("expected nil result to bill a single call, got %d", got)
	}
	if got := sendMessageBillingCallCount(&SendMessageResult{}); got != 1 {
		t.Fatalf("expected interrupted run without completed calls to bill a single call, got %d", got)
	}
	if got := sendMessageBillingCallCount(&SendMessageResult{LLMCallCount: 3}); got != 3 {
		t.Fatalf("expected tool loop to bill every completed upstream call, got %d", got)
	}
}

func TestMessageRequestMaxOutputTokensReadsProviderSpecificKeys(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		want    int64
	}{
		{name: "no limit", options: map[string]any{"temperature": 0.7}, want: 0},
		{name: "openai chat max_tokens", options: map[string]any{"max_tokens": float64(4096)}, want: 4096},
		{name: "openai responses max_output_tokens", options: map[string]any{"max_output_tokens": 1024}, want: 1024},
		{name: "openai completions max_completion_tokens", options: map[string]any{"max_completion_tokens": "2048"}, want: 2048},
		{name: "gemini nested generationConfig", options: map[string]any{"generationConfig": map[string]any{"maxOutputTokens": int64(512)}}, want: 512},
		{name: "explicit limit wins over other keys", options: map[string]any{"max_output_tokens": 300, "max_tokens": 900}, want: 300},
		{name: "non-positive limit ignored", options: map[string]any{"max_tokens": 0}, want: 0},
		{name: "unparseable limit ignored", options: map[string]any{"max_tokens": "many"}, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := messageRequestMaxOutputTokens(tc.options); got != tc.want {
				t.Fatalf("messageRequestMaxOutputTokens() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestClassifyRunErrorCodeMapsInsufficientBalance(t *testing.T) {
	if got := classifyRunErrorCode(appbilling.ErrUsageBalanceInsufficient); got != messageUsageBalanceErrorCode {
		t.Fatalf("classifyRunErrorCode() = %q, want %q", got, messageUsageBalanceErrorCode)
	}
	wrapped := wrapTemporaryGenerationError(appbilling.ErrUsageBalanceInsufficient)
	if wrapped != appbilling.ErrUsageBalanceInsufficient {
		t.Fatalf("expected temporary budget failure to stay unwrapped, got %v", wrapped)
	}
	if got := wrapTemporaryGenerationError(ErrUpstreamEmptyResponse); !errors.Is(got, ErrUpstreamRequestFailed) || !errors.Is(got, ErrUpstreamEmptyResponse) {
		t.Fatalf("expected upstream failure to be wrapped, got %v", got)
	}
}

// 工具循环再次调用前的预算形状必须覆盖“已产生 + 即将产生”的全部成本：已计费输入（含未上报调用的
// 预估补齐）加下一次调用的预估输入，已观测的缓存读写与输出（可见 + 思考）加请求限定的最大输出。
func TestFollowUpUsageBudgetEstimateAccumulatesObservedAndNextCall(t *testing.T) {
	accumulator := &messageUsageAccumulator{}
	accumulator.beginCall(1_000)
	accumulator.addObservedUsage(llm.Usage{InputTokens: 800, CacheReadTokens: 2_000, CacheWriteTokens: 300, OutputTokens: 120, ReasoningTokens: 60})
	accumulator.finishCall(true, true)
	// 第二次调用上游完全没上报用量：输入按请求形状预估，输出按已产出文本预估，预算校验必须把两者都算进去。
	accumulator.beginCall(500)
	unobservedVisibleText := strings.Repeat("streamed answer ", 40)
	accumulator.recordCallVisibleText(unobservedVisibleText)
	accumulator.finishCall(false, false)
	unobservedOutputTokens := tokenestimate.Estimate(unobservedVisibleText)
	if unobservedOutputTokens <= 0 {
		t.Fatal("expected a positive output estimate for the unobserved call")
	}

	nextInput := llm.GenerateInput{Messages: []llm.Message{
		{Role: "user", Content: "hello"},
		{Role: "tool", Content: strings.Repeat("tool result ", 50)},
	}}
	nextInputTokens := estimateBillableInputTokens(nextInput, nextInput.Messages)
	if nextInputTokens <= 0 {
		t.Fatal("expected a positive estimate for the next call")
	}
	got := followUpUsageBudgetEstimate(
		accumulator.billedUsage(),
		nextInputTokens,
		map[string]any{"max_tokens": 4096},
	)
	want := usageBudgetEstimate{
		InputTokens:      800 + 500 + nextInputTokens,
		CacheReadTokens:  2_000,
		CacheWriteTokens: 300,
		OutputTokens:     120 + unobservedOutputTokens + 60 + 4096,
	}
	if got != want {
		t.Fatalf("followUpUsageBudgetEstimate() = %+v, want %+v", got, want)
	}
	if outputTokens, reasoningTokens := accumulator.effectiveOutputTokens(); outputTokens != 120+unobservedOutputTokens || reasoningTokens != 60 {
		t.Fatalf("effectiveOutputTokens() = (%d, %d), want (%d, 60)", outputTokens, reasoningTokens, 120+unobservedOutputTokens)
	}
}

func TestResolveObservedOrHigherEstimatedTokensKeepsLargerEstimate(t *testing.T) {
	if got := resolveObservedOrHigherEstimatedTokens(40, 96); got != 96 {
		t.Fatalf("expected larger input estimate, got %d", got)
	}
	if got, _ := estimateOutputUsage(2, 0, "hello world this is a longer streamed response", ""); got <= 2 {
		t.Fatalf("expected output estimate to cover partial observed usage, got %d", got)
	}
}

// 中断时输出侧只补齐“最后一次上报之后”的尾段：工具循环里已完成的调用以上报值为准，
// 被中断的当前调用才按其尾段文本预估。否则用整条消息全文预估会把已观测的输出再算一遍。
func TestMessageUsageAccumulatorInterruptedOutputOnlyEstimatesCurrentCallTail(t *testing.T) {
	accumulator := &messageUsageAccumulator{}

	accumulator.beginCall(100)
	accumulator.recordCallVisibleText("first call visible answer that was fully reported by upstream")
	accumulator.recordCallReasoningText("first call reasoning that was fully reported by upstream")
	accumulator.addObservedUsage(llm.Usage{InputTokens: 100, OutputTokens: 5, ReasoningTokens: 3})
	accumulator.finishCall(true, true)

	tailText := strings.Repeat("streamed tail before interruption ", 20)
	tailReasoning := strings.Repeat("streamed reasoning before interruption ", 20)
	accumulator.beginCall(120)
	accumulator.recordCallVisibleText(tailText)
	accumulator.recordCallReasoningText(tailReasoning)

	gotOutput, gotReasoning := accumulator.interruptedOutputTokens()
	wantOutput := int64(5) + tokenestimate.Estimate(tailText)
	wantReasoning := int64(3) + tokenestimate.Estimate(tailReasoning)
	if gotOutput != wantOutput || gotReasoning != wantReasoning {
		t.Fatalf("interruptedOutputTokens() = (%d, %d), want (%d, %d)", gotOutput, gotReasoning, wantOutput, wantReasoning)
	}

	// 当前调用中途只上报了合并输出（未拆分思考）时，尾段思考并入输出预估且不与上报值叠加：
	// 总量不变，已完成调用的观测值原样保留。
	accumulator.addObservedUsage(llm.Usage{OutputTokens: 1})
	gotOutput, gotReasoning = accumulator.interruptedOutputTokens()
	if gotOutput != 5+tokenestimate.Estimate(tailText)+tokenestimate.Estimate(tailReasoning) || gotReasoning != 3 {
		t.Fatalf("partial combined output must fold the tail reasoning without stacking, got (%d, %d)", gotOutput, gotReasoning)
	}

	// 当前调用上报的输出已经覆盖尾段时以上报值为准。
	accumulator.addObservedUsage(llm.Usage{OutputTokens: 10_000, ReasoningTokens: 10_000})
	gotOutput, gotReasoning = accumulator.interruptedOutputTokens()
	if gotOutput != 5+1+10_000 || gotReasoning != 3+10_000 {
		t.Fatalf("observed output covering the tail must win, got (%d, %d)", gotOutput, gotReasoning)
	}
}

// 未上报输出用量的已完成调用按其文本预估计入，成功与中断路径口径一致。
func TestMessageUsageAccumulatorEstimatesUnobservedCompletedOutput(t *testing.T) {
	accumulator := &messageUsageAccumulator{}

	firstText := strings.Repeat("unreported first call ", 30)
	accumulator.beginCall(50)
	accumulator.recordCallVisibleText(firstText)
	accumulator.finishCall(false, false)

	accumulator.beginCall(60)
	accumulator.recordCallVisibleText("reported second call")
	accumulator.addObservedUsage(llm.Usage{InputTokens: 60, OutputTokens: 4})
	accumulator.finishCall(true, true)

	wantOutput := tokenestimate.Estimate(firstText) + 4
	if gotOutput, gotReasoning := accumulator.effectiveOutputTokens(); gotOutput != wantOutput || gotReasoning != 0 {
		t.Fatalf("effectiveOutputTokens() = (%d, %d), want (%d, 0)", gotOutput, gotReasoning, wantOutput)
	}
	if gotOutput, gotReasoning := accumulator.interruptedOutputTokens(); gotOutput != wantOutput || gotReasoning != 0 {
		t.Fatalf("interruptedOutputTokens() = (%d, %d), want (%d, 0)", gotOutput, gotReasoning, wantOutput)
	}
}

func TestEstimateOutputUsageFoldsReasoningIntoCombinedObservedOutput(t *testing.T) {
	visible := strings.Repeat("visible ", 40)
	reasoning := strings.Repeat("thinking ", 40)

	gotOutput, gotReasoning := estimateOutputUsage(2, 0, visible, reasoning)
	if gotOutput != tokenestimate.Estimate(visible)+tokenestimate.Estimate(reasoning) || gotReasoning != 0 {
		t.Fatalf("combined observed output must fold reasoning into the output estimate, got (%d, %d)", gotOutput, gotReasoning)
	}

	gotOutput, gotReasoning = estimateOutputUsage(2, 1, visible, reasoning)
	if gotOutput != tokenestimate.Estimate(visible) || gotReasoning != tokenestimate.Estimate(reasoning) {
		t.Fatalf("split observed usage must estimate both sides separately, got (%d, %d)", gotOutput, gotReasoning)
	}

	gotOutput, gotReasoning = estimateOutputUsage(0, 0, visible, reasoning)
	if gotOutput != tokenestimate.Estimate(visible) || gotReasoning != tokenestimate.Estimate(reasoning) {
		t.Fatalf("unreported usage must estimate both sides from text, got (%d, %d)", gotOutput, gotReasoning)
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

func TestMaxPromptTokenEstimateKeepsFullContextForStatefulContinuation(t *testing.T) {
	fullContext := int64(96_000)
	statefulDelta := int64(1_200)

	if got := maxPromptTokenEstimate(statefulDelta, fullContext); got != fullContext {
		t.Fatalf("expected full context estimate to win over stateful delta, got %d", got)
	}
}

func TestFitGenerateInputToModelBudgetTrimsWholeOldestTurnsAfterFinalAssembly(t *testing.T) {
	input := llm.GenerateInput{
		Messages: []llm.Message{
			{Role: "system", Content: "stable files, skills, and policy"},
			{Role: "user", Content: strings.Repeat("old question ", 5_000)},
			{Role: "assistant", Content: strings.Repeat("old answer ", 5_000)},
			{Role: "user", Content: "recent question"},
			{Role: "assistant", Content: "recent answer"},
			{Role: "user", Content: "current question"},
		},
		Instructions: "native provider instructions",
		Tools: []llm.ToolDefinition{{
			Name:        "search_docs",
			Description: "Search the configured knowledge sources",
			InputSchema: []byte(`{"type":"object","properties":{"query":{"type":"string"}}}`),
		}},
	}
	capabilities := `{"contextWindow":32000,"maxOutputTokens":4000}`

	fitted, fit := fitGenerateInputToModelBudget(input, "custom-model", capabilities, 128_000, true)

	if !fit.Trimmed || fit.Exceeded {
		t.Fatalf("expected prompt to be trimmed within budget, got %#v", fit)
	}
	if fit.TokensAfter > fit.Budget {
		t.Fatalf("expected final request shape within budget, got %d > %d", fit.TokensAfter, fit.Budget)
	}
	if len(fitted.Messages) != 4 {
		t.Fatalf("expected the oldest complete turn to be removed, got %#v", fitted.Messages)
	}
	if fitted.Messages[0].Role != "system" || fitted.Messages[1].Content != "recent question" || fitted.Messages[3].Content != "current question" {
		t.Fatalf("expected system prefix and complete recent/current turns, got %#v", fitted.Messages)
	}
	if fitted.Instructions != input.Instructions || len(fitted.Tools) != 1 {
		t.Fatalf("expected fixed request content to remain intact, got %#v", fitted)
	}
}

func TestFitGenerateInputToModelBudgetReportsRequiredContentOverflow(t *testing.T) {
	input := llm.GenerateInput{Messages: []llm.Message{
		{Role: "system", Content: strings.Repeat("required policy ", 5_000)},
		{Role: "user", Content: strings.Repeat("current input ", 5_000)},
	}}

	fitted, fit := fitGenerateInputToModelBudget(
		input,
		"custom-model",
		`{"contextWindow":4096,"maxOutputTokens":1}`,
		128_000,
		true,
	)

	if fit.Trimmed || !fit.Exceeded {
		t.Fatalf("expected required-only overflow without destructive trimming, got %#v", fit)
	}
	if len(fitted.Messages) != len(input.Messages) {
		t.Fatalf("expected required messages to remain intact, got %#v", fitted.Messages)
	}
}

func TestFitGenerateInputToModelBudgetCanBeDisabled(t *testing.T) {
	input := llm.GenerateInput{Messages: []llm.Message{
		{Role: "user", Content: strings.Repeat("history ", 10_000)},
		{Role: "assistant", Content: strings.Repeat("answer ", 10_000)},
		{Role: "user", Content: "current"},
	}}

	fitted, fit := fitGenerateInputToModelBudget(input, "custom-model", `{"contextWindow":4096}`, 128_000, false)

	if fit.Trimmed || fit.Exceeded || fit.Budget != 0 || len(fitted.Messages) != len(input.Messages) {
		t.Fatalf("expected disabled budget fitting to preserve input, got %#v %#v", fit, fitted.Messages)
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
		128_000,
	)
	longBudget := resolveToolResultTokenBudget(
		generateInput,
		[]llm.Message{{Role: "user", Content: strings.Repeat("history ", 20_000)}},
		pendingAssistant,
		"custom-model",
		capabilities,
		128_000,
	)
	effectiveBudget := int64(domainchannel.EffectiveContextBudgetFromCapabilitiesWithFallback("custom-model", capabilities, config.DefaultContextWindowFallbackTokens))
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
		128_000,
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
	effectiveBudget := int64(domainchannel.EffectiveContextBudgetFromCapabilitiesWithFallback("custom-model", capabilities, config.DefaultContextWindowFallbackTokens))
	baseBudget := resolveToolResultTokenBudget(
		llm.GenerateInput{},
		[]llm.Message{{Role: "user", Content: "summarize this video"}},
		pendingAssistant,
		"custom-model",
		capabilities,
		128_000,
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
		128_000,
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
		128_000,
	)
	largeBudget := resolveToolResultTokenBudget(
		llm.GenerateInput{},
		withLargePreviousResult,
		pendingAssistant,
		"custom-model",
		capabilities,
		128_000,
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
		128_000,
	)
	if !changed {
		t.Fatal("expected oversized current tool round to be rebalanced")
	}
	effectiveBudget := int64(domainchannel.EffectiveContextBudgetFromCapabilitiesWithFallback("custom-model", capabilities, config.DefaultContextWindowFallbackTokens))
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
	trimmed, changed := trimToolFollowUpHistory(llm.GenerateInput{}, messages, "custom-model", capabilities, 128_000)
	if !changed {
		t.Fatal("expected oversized follow-up context to trim old history")
	}
	if len(trimmed) != 4 || trimmed[0].Role != "system" || trimmed[1].Content != "summarize this video" {
		t.Fatalf("expected system prefix and current turn only, got %#v", trimmed)
	}
	if tokens := estimateToolFollowUpInputTokens(llm.GenerateInput{}, trimmed); tokens >
		int64(domainchannel.EffectiveContextBudgetFromCapabilitiesWithFallback("custom-model", capabilities, config.DefaultContextWindowFallbackTokens)) {
		t.Fatalf("expected trimmed follow-up within effective model budget, got %d tokens", tokens)
	}
}

func TestSendMessageBillingDurationSeconds(t *testing.T) {
	videoResult := &SendMessageResult{
		AssistantMessage: model.Message{ContentType: "video", Status: "success"},
		DurationSeconds:  5,
		UpstreamProtocol: llm.AdapterXAIVideo,
		Billable:         true,
	}
	if got := sendMessageBillingDurationSeconds(videoResult); got != 5 {
		t.Fatalf("expected explicit duration seconds to win, got %d", got)
	}
	textResult := &SendMessageResult{
		AssistantMessage: model.Message{ContentType: "video", Status: "success"},
		DurationSeconds:  5,
		UpstreamProtocol: llm.AdapterXAIResponses,
		Billable:         true,
	}
	if got := sendMessageBillingDurationSeconds(textResult); got != 0 {
		t.Fatalf("expected non-video protocol duration to be ignored, got %d", got)
	}
	if got := sendMessageBillingDurationSeconds(&SendMessageResult{
		AssistantMessage: model.Message{ContentType: "video", Status: "success"},
		UpstreamProtocol: llm.AdapterXAIVideo,
		Billable:         true,
	}); got != 0 {
		t.Fatalf("expected missing video duration to remain zero, got %d", got)
	}
	if got := sendMessageBillingDurationSeconds(&SendMessageResult{
		AssistantMessage: model.Message{ContentType: "video", Status: "error"},
		DurationSeconds:  6,
		UpstreamProtocol: llm.AdapterXAIVideo,
		Billable:         true,
	}); got != 0 {
		t.Fatalf("expected failed video duration to remain zero, got %d", got)
	}
}

func TestMediaDurationSecondsFromOptions(t *testing.T) {
	if got := mediaDurationSecondsFromOptions(map[string]any{"durationSeconds": float64(5)}); got != 5 {
		t.Fatalf("expected numeric duration seconds, got %d", got)
	}
	if got := mediaDurationSecondsFromOptions(map[string]any{"duration": "5.2s"}); got != 6 {
		t.Fatalf("expected string duration to round up, got %d", got)
	}
	if got := mediaDurationSecondsFromOptions(map[string]any{"duration": "bad"}); got != 0 {
		t.Fatalf("expected invalid duration to be ignored, got %d", got)
	}
	if got := mediaDurationSecondsFromOptions(map[string]any{
		"generation_config": map[string]any{
			"video_config": map[string]any{"duration_seconds": 7},
		},
	}); got != 7 {
		t.Fatalf("expected nested video duration seconds, got %d", got)
	}
}

func TestWithDefaultMediaVideoDurationInjectsOnlySupportedProtocol(t *testing.T) {
	xaiOptions := withDefaultMediaVideoDuration(nil, llm.AdapterXAIVideo)
	if got := mediaDurationSecondsFromOptions(xaiOptions); got != 6 {
		t.Fatalf("expected xAI request default duration, got %d", got)
	}
	explicit := map[string]any{"duration": 9}
	if got := withDefaultMediaVideoDuration(explicit, llm.AdapterXAIVideo); got["duration"] != 9 {
		t.Fatalf("explicit duration was overwritten: %#v", got)
	}
	if got := withDefaultMediaVideoDuration(nil, llm.AdapterGeminiInteractions); got != nil {
		t.Fatalf("unsupported duration parameter was injected: %#v", got)
	}
}

func TestResolveGeneratedVideoDurationsSumsEveryArtifact(t *testing.T) {
	durations, total := resolveGeneratedVideoDurations([]llm.GeneratedVideo{
		{DurationSeconds: 4},
		{},
	}, 6)
	if total != 10 || len(durations) != 2 || durations[0] != 4 || durations[1] != 6 {
		t.Fatalf("unexpected generated video durations: %#v total=%d", durations, total)
	}
}
