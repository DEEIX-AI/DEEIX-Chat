package conversation

import (
	"errors"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestBuildFailedMediaBillingResultPreservesUpstreamUsage(t *testing.T) {
	result := buildFailedMediaBillingResult(failedMediaBillingResultInput{
		UserMessage:      &model.Message{ID: 1},
		AssistantMessage: &model.Message{ID: 2},
		Route: channel.ResolvedRoute{
			UpstreamID:        3,
			UpstreamName:      "primary",
			PlatformModelName: "image-model",
			BindingCode:       "binding",
			UpstreamModel:     "image-model-v1",
			Protocol:          llm.AdapterOpenAIImageGenerations,
		},
		Usage: llm.Usage{
			InputTokens:      100,
			CacheReadTokens:  20,
			CacheWriteTokens: 10,
			OutputTokens:     30,
			ReasoningTokens:  5,
			RawUsageJSON:     `{"input_tokens":100}`,
		},
		StartedAt: time.Now().Add(-time.Second),
		Failure:   errors.New("store generated artifact"),
		Billable:  true,
	})

	if result == nil || !result.Billable {
		t.Fatalf("result = %+v, want billable media result", result)
	}
	if result.UserMessage.InputTokens != 100 || result.UserMessage.CacheReadTokens != 20 || result.UserMessage.CacheWriteTokens != 10 {
		t.Fatalf("user usage = %+v", result.UserMessage)
	}
	if result.AssistantMessage.OutputTokens != 30 || result.AssistantMessage.ReasoningTokens != 5 {
		t.Fatalf("assistant usage = %+v", result.AssistantMessage)
	}
	if result.AssistantMessage.Status != "error" || result.AssistantMessage.ErrorMessage == "" {
		t.Fatalf("assistant failure state = %+v", result.AssistantMessage)
	}
	if result.PlatformModelName != "image-model" || result.RawUsageJSON == "" {
		t.Fatalf("billing attribution = %+v", result)
	}
}

func TestBuildFailedMediaBillingResultCanRemainNonBillable(t *testing.T) {
	result := buildFailedMediaBillingResult(failedMediaBillingResultInput{
		UserMessage:      &model.Message{ID: 1},
		AssistantMessage: &model.Message{ID: 2, ContentType: "video"},
		DurationSeconds:  6,
		Failure:          errors.New("store generated video"),
		Billable:         false,
	})

	if result == nil || result.Billable {
		t.Fatalf("result = %+v, want non-billable failed video result", result)
	}
}

func TestBuildFailedMediaBillingResultKeepsRetryInputOnAssistant(t *testing.T) {
	sourceMessageID := uint(9)
	result := buildFailedMediaBillingResult(failedMediaBillingResultInput{
		UserMessage:      &model.Message{ID: 1},
		AssistantMessage: &model.Message{ID: 2, SourceMessageID: &sourceMessageID},
		Usage:            llm.Usage{InputTokens: 100, CacheReadTokens: 20, CacheWriteTokens: 10},
		StartedAt:        time.Now(),
		Failure:          errors.New("persist assistant message"),
	})

	if result == nil {
		t.Fatal("expected media billing result")
	}
	if result.UserMessage.InputTokens != 0 || result.AssistantMessage.InputTokens != 100 || result.AssistantMessage.CacheReadTokens != 20 || result.AssistantMessage.CacheWriteTokens != 10 {
		t.Fatalf("retry usage attribution = user %+v assistant %+v", result.UserMessage, result.AssistantMessage)
	}
}

func TestBuildFailedMediaBillingResultKeepsCanceledStatus(t *testing.T) {
	result := buildFailedMediaBillingResult(failedMediaBillingResultInput{
		UserMessage:      &model.Message{ID: 1},
		AssistantMessage: &model.Message{ID: 2},
		Usage:            llm.Usage{InputTokens: 10, OutputTokens: 20},
		StartedAt:        time.Now(),
		Failure:          ErrMessageGenerationCanceled,
	})

	if result == nil || result.AssistantMessage.Status != "canceled" {
		t.Fatalf("canceled media billing result = %+v", result)
	}
	if result.AssistantMessage.ErrorCode != "conversation_run.canceled" {
		t.Fatalf("unexpected canceled error code: %+v", result.AssistantMessage)
	}
}
