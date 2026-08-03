package conversation

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type canceledMediaGenerationInput struct {
	Context             context.Context
	Conversation        *model.Conversation
	UserMessage         *model.Message
	AssistantMessage    *model.Message
	ReuseUserMessage    bool
	Route               channel.ResolvedRoute
	EffectiveOptions    map[string]interface{}
	GenerateInput       llm.GenerateInput
	StartedAt           time.Time
	DurationSeconds     int64
	MetadataRefreshHint string
}

// failedMediaBillingResultInput 描述媒体上游成功后本地处理失败时需要保留的计费信息。
type failedMediaBillingResultInput struct {
	UserMessage      *model.Message
	AssistantMessage *model.Message
	Route            channel.ResolvedRoute
	EffectiveOptions map[string]interface{}
	Usage            llm.Usage
	StartedAt        time.Time
	DurationSeconds  int64
	Failure          error
}

// buildFailedMediaBillingResult 保留上游成功后发生本地处理错误时的真实计费上下文。
func buildFailedMediaBillingResult(input failedMediaBillingResultInput) *SendMessageResult {
	if input.UserMessage == nil || input.AssistantMessage == nil {
		return nil
	}
	userMessage := *input.UserMessage
	assistantMessage := *input.AssistantMessage
	if assistantMessage.SourceMessageID != nil {
		assistantMessage.InputTokens = input.Usage.InputTokens
		assistantMessage.CacheReadTokens = input.Usage.CacheReadTokens
		assistantMessage.CacheWriteTokens = input.Usage.CacheWriteTokens
	} else {
		userMessage.InputTokens = input.Usage.InputTokens
		userMessage.CacheReadTokens = input.Usage.CacheReadTokens
		userMessage.CacheWriteTokens = input.Usage.CacheWriteTokens
		userMessage.TokenUsage = input.Usage.InputTokens + input.Usage.CacheReadTokens + input.Usage.CacheWriteTokens
	}
	assistantMessage.OutputTokens = input.Usage.OutputTokens
	assistantMessage.ReasoningTokens = input.Usage.ReasoningTokens
	assistantMessage.TokenUsage = assistantMessage.InputTokens + assistantMessage.CacheReadTokens + assistantMessage.CacheWriteTokens + assistantMessage.OutputTokens + assistantMessage.ReasoningTokens
	assistantMessage.LatencyMS = time.Since(input.StartedAt).Milliseconds()
	if assistantMessage.LatencyMS < 0 {
		assistantMessage.LatencyMS = 0
	}
	assistantMessage.Status = "error"
	assistantMessage.ErrorCode = classifyRunErrorCode(input.Failure)
	assistantMessage.ErrorMessage = truncateError(messageErrorSummary(input.Failure), 255)

	return &SendMessageResult{
		UserMessage:        userMessage,
		AssistantMessage:   assistantMessage,
		Billable:           true,
		UpstreamID:         input.Route.UpstreamID,
		UpstreamName:       input.Route.UpstreamName,
		PlatformModelName:  input.Route.PlatformModelName,
		RoutedBindingCode:  input.Route.BindingCode,
		UpstreamModelName:  input.Route.UpstreamModel,
		UpstreamProtocol:   input.Route.Protocol,
		EffectiveOptions:   input.EffectiveOptions,
		UsageSpeed:         input.Usage.Speed,
		UsageServiceTier:   input.Usage.ServiceTier,
		RawUsageJSON:       input.Usage.RawUsageJSON,
		CacheWrite5mTokens: input.Usage.CacheWrite5mTokens,
		CacheWrite1hTokens: input.Usage.CacheWrite1hTokens,
		LatencyMS:          assistantMessage.LatencyMS,
		DurationSeconds:    input.DurationSeconds,
		StartedAt:          input.StartedAt,
	}
}

func (s *Service) completeCanceledMediaGeneration(input canceledMediaGenerationInput) (*SendMessageResult, error) {
	if input.Context == nil || input.UserMessage == nil || input.AssistantMessage == nil {
		return nil, ErrMessageGenerationCanceled
	}
	persistCtx := context.WithoutCancel(input.Context)
	latencyMS := time.Since(input.StartedAt).Milliseconds()
	if latencyMS < 0 {
		latencyMS = 0
	}
	inputTokens := estimateGenerateInputTokens(input.GenerateInput)
	errorCode := classifyRunErrorCode(ErrMessageGenerationCanceled)
	errorMessage := truncateError(ErrMessageGenerationCanceled.Error(), 255)

	if input.ReuseUserMessage {
		if err := s.repo.CompleteAssistantMessageWithGeneratedAttachments(
			persistCtx,
			input.AssistantMessage.ID,
			repository.AssistantMessageCompletionUpdate{
				ContentType:  input.AssistantMessage.ContentType,
				Content:      "",
				InputTokens:  inputTokens,
				LatencyMS:    latencyMS,
				Status:       "canceled",
				ErrorCode:    errorCode,
				ErrorMessage: errorMessage,
			},
			nil,
		); err != nil {
			return nil, err
		}
		input.AssistantMessage.InputTokens = inputTokens
		input.AssistantMessage.TokenUsage = inputTokens
	} else {
		if err := s.repo.CompleteAssistantMessageWithAttachments(
			persistCtx,
			input.UserMessage.ID,
			repository.MessageUsageUpdate{InputTokens: inputTokens},
			input.AssistantMessage.ID,
			repository.AssistantMessageCompletionUpdate{
				ContentType:  input.AssistantMessage.ContentType,
				Content:      "",
				LatencyMS:    latencyMS,
				Status:       "canceled",
				ErrorCode:    errorCode,
				ErrorMessage: errorMessage,
			},
			nil,
		); err != nil {
			return nil, err
		}
		input.UserMessage.InputTokens = inputTokens
		input.UserMessage.TokenUsage = inputTokens
	}

	input.AssistantMessage.Content = ""
	input.AssistantMessage.LatencyMS = latencyMS
	input.AssistantMessage.Status = "canceled"
	input.AssistantMessage.ErrorCode = errorCode
	input.AssistantMessage.ErrorMessage = errorMessage

	if input.MetadataRefreshHint == "" && input.Conversation != nil {
		input.MetadataRefreshHint = s.resolveConversationMetadataRefreshHint(persistCtx, *input.Conversation, *input.UserMessage)
	}
	return &SendMessageResult{
		UserMessage:         *input.UserMessage,
		AssistantMessage:    *input.AssistantMessage,
		MetadataRefreshHint: input.MetadataRefreshHint,
		Billable:            true,
		UpstreamID:          input.Route.UpstreamID,
		UpstreamName:        input.Route.UpstreamName,
		PlatformModelName:   input.Route.PlatformModelName,
		RoutedBindingCode:   input.Route.BindingCode,
		UpstreamModelName:   input.Route.UpstreamModel,
		UpstreamProtocol:    input.Route.Protocol,
		EffectiveOptions:    input.EffectiveOptions,
		LatencyMS:           latencyMS,
		DurationSeconds:     input.DurationSeconds,
		StartedAt:           input.StartedAt,
	}, nil
}

func (s *Service) isCanceledMediaGeneration(ctx context.Context, runID string, err error) bool {
	return errors.Is(ctx.Err(), context.Canceled) ||
		s.isMessageGenerationCanceled(ctx, runID) ||
		isMessageGenerationCanceledError(err)
}

// applyMediaRunUsage 将可计费媒体结果同步到运行日志。
func applyMediaRunUsage(run *model.Run, result *SendMessageResult) {
	if run == nil || result == nil {
		return
	}
	run.InputTokens = sendMessageBillingInputTokens(result)
	run.CacheReadTokens = sendMessageBillingCacheReadTokens(result)
	run.CacheWriteTokens = sendMessageBillingCacheWriteTokens(result)
	run.OutputTokens = result.AssistantMessage.OutputTokens
	run.ReasoningTokens = result.AssistantMessage.ReasoningTokens
}

func mediaDurationSecondsFromOptions(options map[string]interface{}) int64 {
	for _, key := range []string{"durationSeconds", "duration_seconds", "duration"} {
		if seconds := mediaDurationSecondsFromValue(options[key]); seconds > 0 {
			return seconds
		}
	}
	return 0
}

func mediaDurationSecondsFromValue(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return positiveSeconds(int64(v))
	case int64:
		return positiveSeconds(v)
	case float64:
		return ceilPositiveSeconds(v)
	case float32:
		return ceilPositiveSeconds(float64(v))
	case string:
		text := strings.TrimSpace(strings.ToLower(v))
		text = strings.TrimSuffix(text, "seconds")
		text = strings.TrimSuffix(text, "second")
		text = strings.TrimSuffix(text, "secs")
		text = strings.TrimSuffix(text, "sec")
		text = strings.TrimSuffix(text, "s")
		parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return 0
		}
		return ceilPositiveSeconds(parsed)
	default:
		return 0
	}
}

func ceilPositiveSeconds(value float64) int64 {
	if value <= 0 {
		return 0
	}
	seconds := int64(value)
	if float64(seconds) < value {
		seconds++
	}
	return seconds
}

func positiveSeconds(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return value
}
