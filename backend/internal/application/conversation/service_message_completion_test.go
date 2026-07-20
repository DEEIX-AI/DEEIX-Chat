package conversation

import (
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
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

func TestCanceledGenerationEstimatesVisibleReasoningUsage(t *testing.T) {
	reasoningText := "正在分析用户请求，并检查终止时已经显示的思考内容。"
	input := persistInterruptedMessageGenerationInput{
		UserMessage:            &model.Message{},
		AssistantMessage:       &model.Message{},
		AssistantReasoningText: reasoningText,
		EstimatedInputTokens:   12,
		Error:                  ErrMessageGenerationCanceled,
		StartedAt:              time.Now(),
	}

	metrics := resolveInterruptedMessageGenerationMetrics(input)
	if metrics.OutputTokens != 0 || metrics.ReasoningTokens != estimateTokens(reasoningText) {
		t.Fatalf("expected visible reasoning to be estimated separately, got %#v", metrics)
	}
	if source := interruptedUsageSource(input, metrics); source != interruptedUsageSourceEstimated {
		t.Fatalf("usage source = %q, want estimated", source)
	}
}

func TestCanceledGenerationDoesNotDoubleCountCombinedObservedOutput(t *testing.T) {
	input := persistInterruptedMessageGenerationInput{
		UserMessage:            &model.Message{},
		AssistantMessage:       &model.Message{},
		AssistantText:          "可见回复",
		AssistantReasoningText: "可见思考内容",
		Usage:                  llm.Usage{OutputTokens: 2},
		Error:                  ErrMessageGenerationCanceled,
		StartedAt:              time.Now(),
	}

	metrics := resolveInterruptedMessageGenerationMetrics(input)
	wantOutput := resolveObservedOrHigherEstimatedTokens(
		input.Usage.OutputTokens,
		estimateTokens(input.AssistantText)+estimateTokens(input.AssistantReasoningText),
	)
	if metrics.OutputTokens != wantOutput || metrics.ReasoningTokens != 0 {
		t.Fatalf("expected combined output without duplicated reasoning, got %#v", metrics)
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
