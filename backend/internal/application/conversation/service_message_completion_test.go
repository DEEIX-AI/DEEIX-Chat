package conversation

import (
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
)

func TestCanceledGenerationWithObservedUsageIsRetainedForBilling(t *testing.T) {
	input := persistInterruptedMessageGenerationInput{
		UserMessage:          &model.Message{},
		AssistantMessage:     &model.Message{},
		EstimatedInputTokens: 12,
		Usage:                llm.Usage{InputTokens: 40, ReasoningTokens: 6},
		AssistantLatency:     25,
		Error:                ErrMessageGenerationCanceled,
		StartedAt:            time.Now(),
	}

	if !shouldPersistInterruptedMessageGeneration(input) {
		t.Fatal("expected canceled generation with observed usage to be retained")
	}
	metrics := resolveInterruptedMessageGenerationMetrics(input)
	if metrics.InputTokens != 40 || metrics.ReasoningTokens != 6 {
		t.Fatalf("expected observed usage to be preserved, got %#v", metrics)
	}
	if status := retainedGenerationStatus(input.Error); status != "canceled" {
		t.Fatalf("expected canceled status, got %q", status)
	}
}

func TestCanceledGenerationUsesEstimatedTotalWhenObservedUsageIsPartial(t *testing.T) {
	input := persistInterruptedMessageGenerationInput{
		UserMessage:          &model.Message{},
		AssistantMessage:     &model.Message{},
		EstimatedInputTokens: 96,
		Usage:                llm.Usage{InputTokens: 40},
		Error:                ErrMessageGenerationCanceled,
		StartedAt:            time.Now(),
	}

	metrics := resolveInterruptedMessageGenerationMetrics(input)
	if metrics.InputTokens != 96 {
		t.Fatalf("expected estimated total input tokens to cover partial observed usage, got %#v", metrics)
	}
}

func TestCanceledGenerationWithoutUsageOrOutputIsNotRetained(t *testing.T) {
	input := persistInterruptedMessageGenerationInput{
		UserMessage:      &model.Message{},
		AssistantMessage: &model.Message{},
		Error:            ErrMessageGenerationCanceled,
		StartedAt:        time.Now(),
	}

	if shouldPersistInterruptedMessageGeneration(input) {
		t.Fatal("expected empty canceled generation to stay non-billable")
	}
}

func TestCanceledGenerationAfterUpstreamCallUsesEstimatedInputFallback(t *testing.T) {
	input := persistInterruptedMessageGenerationInput{
		UserMessage:          &model.Message{},
		AssistantMessage:     &model.Message{},
		EstimatedInputTokens: 32,
		UpstreamCallStarted:  true,
		Error:                ErrMessageGenerationCanceled,
		StartedAt:            time.Now(),
	}

	if !shouldPersistInterruptedMessageGeneration(input) {
		t.Fatal("expected canceled upstream call to be retained with estimated input usage")
	}
	metrics := resolveInterruptedMessageGenerationMetrics(input)
	if metrics.InputTokens != 32 || metrics.OutputTokens != 0 {
		t.Fatalf("expected estimated input fallback without output charge, got %#v", metrics)
	}
}

// interruptedInputFromAccumulator 按发送链路的实际接线构造中断落库入参：输入与输出侧的预估
// 都来自用量累加器，而不是在落库阶段用整条消息的全文重新估算。
func interruptedInputFromAccumulator(accumulator *messageUsageAccumulator, visibleText string, reasoningText string) persistInterruptedMessageGenerationInput {
	estimatedOutputTokens, estimatedReasoningTokens := accumulator.interruptedOutputTokens()
	return persistInterruptedMessageGenerationInput{
		UserMessage:              &model.Message{},
		AssistantMessage:         &model.Message{},
		AssistantText:            visibleText,
		AssistantReasoningText:   reasoningText,
		EstimatedInputTokens:     accumulator.interruptedInputTokens(),
		EstimatedOutputTokens:    estimatedOutputTokens,
		EstimatedReasoningTokens: estimatedReasoningTokens,
		UpstreamCallStarted:      true,
		Usage:                    accumulator.usage(),
		Error:                    ErrMessageGenerationCanceled,
		StartedAt:                time.Now(),
	}
}

func TestCanceledGenerationEstimatesVisibleReasoningUsage(t *testing.T) {
	reasoningText := "正在分析用户请求，并检查终止时已经显示的思考内容。"
	accumulator := &messageUsageAccumulator{}
	accumulator.beginCall(12)
	accumulator.recordCallReasoningText(reasoningText)
	input := interruptedInputFromAccumulator(accumulator, "", reasoningText)
	if !shouldPersistInterruptedMessageGeneration(input) {
		t.Fatal("reasoning-only visible output must be retained for moderation")
	}

	metrics := resolveInterruptedMessageGenerationMetrics(input)
	if metrics.InputTokens != 12 || metrics.OutputTokens != 0 || metrics.ReasoningTokens != tokenestimate.Estimate(reasoningText) {
		t.Fatalf("expected visible reasoning to be estimated separately, got %#v", metrics)
	}
	if source := interruptedUsageSource(input, metrics); source != interruptedUsageSourceEstimated {
		t.Fatalf("usage source = %q, want estimated", source)
	}
}

func TestCanceledGenerationDoesNotDoubleCountCombinedObservedOutput(t *testing.T) {
	visibleText := "可见回复"
	reasoningText := "可见思考内容"
	accumulator := &messageUsageAccumulator{}
	accumulator.beginCall(8)
	accumulator.recordCallVisibleText(visibleText)
	accumulator.recordCallReasoningText(reasoningText)
	accumulator.addObservedUsage(llm.Usage{OutputTokens: 2})
	input := interruptedInputFromAccumulator(accumulator, visibleText, reasoningText)

	metrics := resolveInterruptedMessageGenerationMetrics(input)
	wantOutput := resolveObservedOrHigherEstimatedTokens(
		input.Usage.OutputTokens,
		tokenestimate.Estimate(visibleText)+tokenestimate.Estimate(reasoningText),
	)
	if metrics.OutputTokens != wantOutput || metrics.ReasoningTokens != 0 {
		t.Fatalf("expected combined output without duplicated reasoning, got %#v", metrics)
	}
	if source := interruptedUsageSource(input, metrics); source != interruptedUsageSourceMixed {
		t.Fatalf("usage source = %q, want mixed", source)
	}
}

// 工具循环中断时，已完成调用上报过的输出不能再被整条消息的全文预估覆盖一遍：
// 只有被中断的当前调用尾段才走文本预估。
func TestCanceledToolLoopOnlyEstimatesInterruptedCallTail(t *testing.T) {
	firstText := "第一轮调用完整回复，上游已上报用量。"
	tailText := "第二轮调用在中断前已经流出的可见文本。"
	accumulator := &messageUsageAccumulator{}
	accumulator.beginCall(40)
	accumulator.recordCallVisibleText(firstText)
	accumulator.addObservedUsage(llm.Usage{InputTokens: 40, OutputTokens: 6})
	accumulator.finishCall(true, true)
	accumulator.beginCall(55)
	accumulator.recordCallVisibleText(tailText)
	input := interruptedInputFromAccumulator(accumulator, firstText+tailText, "")

	metrics := resolveInterruptedMessageGenerationMetrics(input)
	if metrics.InputTokens != 40+55 {
		t.Fatalf("expected observed first call plus estimated second call input, got %#v", metrics)
	}
	if want := int64(6) + tokenestimate.Estimate(tailText); metrics.OutputTokens != want || metrics.ReasoningTokens != 0 {
		t.Fatalf("expected reported first call output plus tail estimate (%d), got %#v", want, metrics)
	}
	if source := interruptedUsageSource(input, metrics); source != interruptedUsageSourceMixed {
		t.Fatalf("usage source = %q, want mixed", source)
	}
}

func TestCanceledGenerationRecoveredUsageIsAuthoritative(t *testing.T) {
	input := persistInterruptedMessageGenerationInput{
		UserMessage:            &model.Message{},
		AssistantMessage:       &model.Message{},
		AssistantText:          "这是一段明显更长的可见回复，用于确认恢复值不会被估算覆盖。",
		AssistantReasoningText: "这是一段明显更长的思考内容，用于确认恢复值不会被估算覆盖。",
		Usage:                  llm.Usage{InputTokens: 10, OutputTokens: 48, ReasoningTokens: 16},
		UsageRecovered:         true,
		Error:                  ErrMessageGenerationCanceled,
		StartedAt:              time.Now(),
	}

	metrics := resolveInterruptedMessageGenerationMetrics(input)
	if metrics.InputTokens != 10 || metrics.OutputTokens != 48 || metrics.ReasoningTokens != 16 {
		t.Fatalf("expected recovered usage to remain authoritative, got %#v", metrics)
	}
	if source := interruptedUsageSource(input, metrics); source != interruptedUsageSourceRecovered {
		t.Fatalf("usage source = %q, want recovered", source)
	}
}

func TestCanceledGenerationRecoveredUsageRetainsEarlierEstimatedInput(t *testing.T) {
	input := persistInterruptedMessageGenerationInput{
		UserMessage:          &model.Message{},
		AssistantMessage:     &model.Message{},
		EstimatedInputTokens: 38,
		Usage:                llm.Usage{InputTokens: 24, OutputTokens: 12},
		UsageRecovered:       true,
		Error:                ErrMessageGenerationCanceled,
		StartedAt:            time.Now(),
	}

	metrics := resolveInterruptedMessageGenerationMetrics(input)
	if metrics.InputTokens != 38 || metrics.OutputTokens != 12 {
		t.Fatalf("expected recovered current-call usage plus earlier estimated input, got %#v", metrics)
	}
	if source := interruptedUsageSource(input, metrics); source != interruptedUsageSourceMixed {
		t.Fatalf("usage source = %q, want mixed", source)
	}
}

// 中断的生成同样要保住已产出的推理内容：落库更新是无条件覆盖，
// 若不带上 ReasoningContent 就会把该轮推理抹成空字符串，
// 而 interrupted 状态的 assistant 消息仍会进入后续轮次的上下文。
func TestInterruptedGenerationRetainsReasoningContent(t *testing.T) {
	assistant := &model.Message{ReasoningContent: "stale"}
	input := persistInterruptedMessageGenerationInput{
		UserMessage:            &model.Message{},
		AssistantMessage:       assistant,
		AssistantText:          "部分可见回复",
		AssistantReasoningText: "  中断前已产出的思考内容  ",
		Error:                  ErrMessageGenerationCanceled,
		StartedAt:              time.Now(),
	}

	applyInterruptedMessageGenerationState(input, resolveInterruptedMessageGenerationMetrics(input))

	if assistant.ReasoningContent != "中断前已产出的思考内容" {
		t.Fatalf("expected trimmed reasoning to be retained, got %q", assistant.ReasoningContent)
	}
}

func TestModerationOutputTextIncludesVisibleReasoningWithoutDuplicates(t *testing.T) {
	got := moderationOutputText("assistant answer", "visible reasoning", "visible reasoning")
	want := "assistant answer\n\nvisible reasoning"
	if got != want {
		t.Fatalf("moderation output=%q, want %q", got, want)
	}
}
