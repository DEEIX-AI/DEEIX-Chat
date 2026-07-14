package conversation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

type persistMessageGenerationInput struct {
	SendInput                 SendMessageInput
	Conversation              *model.Conversation
	UserMessage               *model.Message
	AssistantMessage          *model.Message
	AssistantText             string
	AssistantReasoningContent string
	GeneratedImages           []llm.GeneratedImage
	InputTokens               int64
	CacheReadTokens           int64
	CacheWriteTokens          int64
	OutputTokens              int64
	ReasoningTokens           int64
	AssistantLatency          int64
	ResponseID                string
	StatefulPromptFingerprint string
	ToolCallRows              []model.ToolCall
	PersistedToolCallKeys     map[string]struct{}
	ReuseUserMessage          bool
}

type persistInterruptedMessageGenerationInput struct {
	SendInput             SendMessageInput
	UserMessage           *model.Message
	AssistantMessage      *model.Message
	AssistantText         string
	EstimatedInputTokens  int64
	UpstreamCallStarted   bool
	Usage                 llm.Usage
	AssistantLatency      int64
	Error                 error
	ToolCallRows          []model.ToolCall
	PersistedToolCallKeys map[string]struct{}
	TraceRecorder         *messageTraceRecorder
	Route                 *channel.ResolvedRoute
	EffectiveOptions      map[string]interface{}
	ServerSideToolUsage   map[string]int64
	StartedAt             time.Time
	ReuseUserMessage      bool
}

type interruptedMessageGenerationMetrics struct {
	InputTokens      int64
	OutputTokens     int64
	LatencyMS        int64
	ErrorCode        string
	ErrorMessage     string
	CacheReadTokens  int64
	CacheWriteTokens int64
	ReasoningTokens  int64
}

type persistMessageToolCallsInput struct {
	SendInput             SendMessageInput
	UserMessageID         uint
	AssistantMessageID    uint
	RunID                 string
	Rows                  []model.ToolCall
	PersistedToolCallKeys map[string]struct{}
}

func (s *Service) persistSuccessfulMessageGeneration(ctx context.Context, input persistMessageGenerationInput) error {
	if input.ReuseUserMessage {
		input.AssistantMessage.InputTokens = input.InputTokens
		input.AssistantMessage.CacheReadTokens = input.CacheReadTokens
		input.AssistantMessage.CacheWriteTokens = input.CacheWriteTokens
	} else {
		input.UserMessage.InputTokens = input.InputTokens
		input.UserMessage.CacheReadTokens = input.CacheReadTokens
		input.UserMessage.CacheWriteTokens = input.CacheWriteTokens
		input.UserMessage.TokenUsage = input.InputTokens + input.CacheReadTokens + input.CacheWriteTokens
	}

	if completed, err := s.persistAssistantImagePayloadIfPresent(ctx, input); err != nil {
		return err
	} else if completed {
		return s.finishSuccessfulMessageGeneration(ctx, input)
	}

	if !input.ReuseUserMessage {
		go func(msgID uint, inputTokens, cacheReadTokens, cacheWriteTokens int64) {
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.repo.UpdateMessageUsage(bgCtx, msgID, inputTokens, 0, cacheReadTokens, cacheWriteTokens, 0)
		}(input.UserMessage.ID, input.InputTokens, input.CacheReadTokens, input.CacheWriteTokens)
	}

	if err := s.repo.UpdateAssistantMessageCompletion(
		ctx,
		input.AssistantMessage.ID,
		repository.AssistantMessageCompletionUpdate{
			Content:          input.AssistantText,
			ReasoningContent: input.AssistantReasoningContent,
			InputTokens:      assistantCompletionInputTokens(input),
			OutputTokens:     input.OutputTokens,
			CacheReadTokens:  assistantCompletionCacheReadTokens(input),
			CacheWriteTokens: assistantCompletionCacheWriteTokens(input),
			ReasoningTokens:  input.ReasoningTokens,
			LatencyMS:        input.AssistantLatency,
			Status:           "success",
		},
	); err != nil {
		return err
	}
	input.AssistantMessage.Content = input.AssistantText
	input.AssistantMessage.ReasoningContent = input.AssistantReasoningContent
	input.AssistantMessage.TokenUsage = input.OutputTokens + input.ReasoningTokens
	input.AssistantMessage.OutputTokens = input.OutputTokens
	input.AssistantMessage.ReasoningTokens = input.ReasoningTokens
	input.AssistantMessage.LatencyMS = input.AssistantLatency
	input.AssistantMessage.Status = "success"
	if input.ReuseUserMessage {
		input.AssistantMessage.TokenUsage = input.InputTokens + input.CacheReadTokens + input.CacheWriteTokens + input.OutputTokens + input.ReasoningTokens
	}

	return s.finishSuccessfulMessageGeneration(ctx, input)
}

func assistantCompletionInputTokens(input persistMessageGenerationInput) int64 {
	if input.ReuseUserMessage {
		return input.InputTokens
	}
	return 0
}

func assistantCompletionCacheReadTokens(input persistMessageGenerationInput) int64 {
	if input.ReuseUserMessage {
		return input.CacheReadTokens
	}
	return 0
}

func assistantCompletionCacheWriteTokens(input persistMessageGenerationInput) int64 {
	if input.ReuseUserMessage {
		return input.CacheWriteTokens
	}
	return 0
}

// persistAssistantImagePayloadIfPresent 保存结构化图片或兼容旧文本图片载荷。
func (s *Service) persistAssistantImagePayloadIfPresent(ctx context.Context, input persistMessageGenerationInput) (bool, error) {
	var normalized *assistantImageContentNormalization
	var err error
	if len(input.GeneratedImages) > 0 {
		normalized, err = s.normalizeAssistantGeneratedImages(
			ctx,
			input.SendInput.UserID,
			input.SendInput.ConversationID,
			input.AssistantMessage.ID,
			successfulMessageGenerationModelName(input),
			input.GeneratedImages,
		)
	} else {
		normalized, err = s.normalizeAssistantImageContent(
			ctx,
			input.SendInput.UserID,
			input.SendInput.ConversationID,
			input.AssistantMessage.ID,
			successfulMessageGenerationModelName(input),
			input.AssistantText,
		)
	}
	if err != nil || normalized == nil {
		return false, err
	}
	contentType := "image"
	content := normalized.Content
	if len(input.GeneratedImages) > 0 && strings.TrimSpace(input.AssistantText) != "" {
		contentType = "mixed"
		content = strings.TrimSpace(input.AssistantText) + "\n\n" + normalized.Content
	}

	if input.ReuseUserMessage {
		if err := s.repo.CompleteAssistantMessageWithGeneratedAttachments(
			ctx,
			input.AssistantMessage.ID,
			repository.AssistantMessageCompletionUpdate{
				ContentType:      contentType,
				Content:          content,
				ReasoningContent: input.AssistantReasoningContent,
				InputTokens:      input.InputTokens,
				OutputTokens:     input.OutputTokens,
				CacheReadTokens:  input.CacheReadTokens,
				CacheWriteTokens: input.CacheWriteTokens,
				ReasoningTokens:  input.ReasoningTokens,
				LatencyMS:        input.AssistantLatency,
				Status:           "success",
			},
			normalized.AttachmentRows,
		); err != nil {
			return false, err
		}
	} else {
		if err := s.repo.CompleteAssistantMessageWithAttachments(
			ctx,
			input.UserMessage.ID,
			repository.MessageUsageUpdate{
				InputTokens:      input.InputTokens,
				CacheReadTokens:  input.CacheReadTokens,
				CacheWriteTokens: input.CacheWriteTokens,
			},
			input.AssistantMessage.ID,
			repository.AssistantMessageCompletionUpdate{
				ContentType:      contentType,
				Content:          content,
				ReasoningContent: input.AssistantReasoningContent,
				OutputTokens:     input.OutputTokens,
				ReasoningTokens:  input.ReasoningTokens,
				LatencyMS:        input.AssistantLatency,
				Status:           "success",
			},
			normalized.AttachmentRows,
		); err != nil {
			return false, err
		}
	}

	input.AssistantMessage.ContentType = contentType
	input.AssistantMessage.Content = content
	input.AssistantMessage.ReasoningContent = input.AssistantReasoningContent
	if input.ReuseUserMessage {
		input.AssistantMessage.InputTokens = input.InputTokens
		input.AssistantMessage.CacheReadTokens = input.CacheReadTokens
		input.AssistantMessage.CacheWriteTokens = input.CacheWriteTokens
	}
	input.AssistantMessage.TokenUsage = input.OutputTokens + input.ReasoningTokens
	if input.ReuseUserMessage {
		input.AssistantMessage.TokenUsage += input.InputTokens + input.CacheReadTokens + input.CacheWriteTokens
	}
	input.AssistantMessage.OutputTokens = input.OutputTokens
	input.AssistantMessage.ReasoningTokens = input.ReasoningTokens
	input.AssistantMessage.LatencyMS = input.AssistantLatency
	input.AssistantMessage.Status = "success"
	input.AssistantMessage.Attachments = marshalAttachmentSnapshots(normalized.AttachmentSnapshots)
	return true, nil
}

func successfulMessageGenerationModelName(input persistMessageGenerationInput) string {
	if input.Conversation != nil {
		if value := strings.TrimSpace(input.Conversation.Model); value != "" {
			return value
		}
	}
	return strings.TrimSpace(input.SendInput.PlatformModelName)
}

func (s *Service) finishSuccessfulMessageGeneration(ctx context.Context, input persistMessageGenerationInput) error {
	if err := s.persistMessageToolCalls(ctx, persistMessageToolCallsInput{
		SendInput:             input.SendInput,
		UserMessageID:         input.UserMessage.ID,
		AssistantMessageID:    input.AssistantMessage.ID,
		RunID:                 input.AssistantMessage.RunID,
		Rows:                  input.ToolCallRows,
		PersistedToolCallKeys: input.PersistedToolCallKeys,
	}); err != nil {
		return err
	}

	// 非默认分支不代表会话主链，不能覆盖后续默认消息使用的 Responses 状态。
	if normalizeBranchReason(input.SendInput.BranchReason) == "default" {
		s.updateStatefulResponseAsync(input.SendInput.ConversationID, input.ResponseID, input.StatefulPromptFingerprint)
	}
	if input.ReuseUserMessage {
		s.embedMessagePairAsync(input.SendInput, nil, input.AssistantMessage)
	} else {
		s.embedMessagePairAsync(input.SendInput, input.UserMessage, input.AssistantMessage)
	}

	return nil
}

// persistInterruptedMessageGeneration 在模型调用已经产生可见内容或工具轨迹后失败时，保留本轮 assistant 消息。
// 显式取消由取消流程单独处理，避免把用户主动停止误标为异常中断。
func (s *Service) persistInterruptedMessageGeneration(ctx context.Context, input persistInterruptedMessageGenerationInput) *SendMessageResult {
	if !shouldPersistInterruptedMessageGeneration(input) {
		return nil
	}
	persistCtx := ctx
	var cancel context.CancelFunc
	if persistCtx == nil || persistCtx.Err() != nil {
		persistCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	metrics := resolveInterruptedMessageGenerationMetrics(input)

	if input.ReuseUserMessage {
		input.AssistantMessage.InputTokens = metrics.InputTokens
		input.AssistantMessage.CacheReadTokens = metrics.CacheReadTokens
		input.AssistantMessage.CacheWriteTokens = metrics.CacheWriteTokens
	} else {
		if err := s.repo.UpdateMessageUsage(
			persistCtx,
			input.UserMessage.ID,
			metrics.InputTokens,
			0,
			metrics.CacheReadTokens,
			metrics.CacheWriteTokens,
			0,
		); err != nil {
			s.logger.Warn("persist_interrupted_user_usage_failed",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.Uint("message_id", input.UserMessage.ID),
				zap.Error(err),
			)
		}
		if err := s.repo.UpdateMessageState(persistCtx, input.UserMessage.ID, "success", "", ""); err != nil {
			s.logger.Warn("persist_interrupted_user_state_failed",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.Uint("message_id", input.UserMessage.ID),
				zap.Error(err),
			)
		}
	}
	if err := s.repo.UpdateAssistantMessageCompletion(
		persistCtx,
		input.AssistantMessage.ID,
		repository.AssistantMessageCompletionUpdate{
			Content:          input.AssistantText,
			InputTokens:      interruptedCompletionInputTokens(input, metrics),
			OutputTokens:     metrics.OutputTokens,
			CacheReadTokens:  interruptedCompletionCacheReadTokens(input, metrics),
			CacheWriteTokens: interruptedCompletionCacheWriteTokens(input, metrics),
			ReasoningTokens:  metrics.ReasoningTokens,
			LatencyMS:        metrics.LatencyMS,
			Status:           retainedGenerationStatus(input.Error),
			ErrorCode:        metrics.ErrorCode,
			ErrorMessage:     metrics.ErrorMessage,
		},
	); err != nil {
		s.logger.Error("persist_interrupted_assistant_completion_failed",
			zap.String("trace_id", traceid.FromContext(ctx)),
			zap.Uint("message_id", input.AssistantMessage.ID),
			zap.Error(err),
		)
		return nil
	}
	applyInterruptedMessageGenerationState(input, metrics)

	if err := s.persistMessageToolCalls(persistCtx, persistMessageToolCallsInput{
		SendInput:             input.SendInput,
		UserMessageID:         input.UserMessage.ID,
		AssistantMessageID:    input.AssistantMessage.ID,
		RunID:                 input.AssistantMessage.RunID,
		Rows:                  input.ToolCallRows,
		PersistedToolCallKeys: input.PersistedToolCallKeys,
	}); err != nil {
		s.logger.Warn("persist_interrupted_tool_calls_failed",
			zap.String("trace_id", traceid.FromContext(ctx)),
			zap.Uint("message_id", input.AssistantMessage.ID),
			zap.Error(err),
		)
	}
	if input.TraceRecorder != nil {
		input.TraceRecorder.fail(input.Error)
		input.TraceRecorder.attachToMessage(input.AssistantMessage)
	}

	return buildInterruptedSendMessageResult(input, metrics)
}

// shouldPersistInterruptedMessageGeneration 只在已有可计费用量、可展示内容或可追踪工具结果时保留中断消息。
func shouldPersistInterruptedMessageGeneration(input persistInterruptedMessageGenerationInput) bool {
	if input.Error == nil || input.UserMessage == nil || input.AssistantMessage == nil {
		return false
	}
	hasRetainedToolTrace := len(input.ToolCallRows) > 0 || len(input.ServerSideToolUsage) > 0
	hasObservedUsage := input.Usage.InputTokens > 0 ||
		input.Usage.OutputTokens > 0 ||
		input.Usage.CacheReadTokens > 0 ||
		input.Usage.CacheWriteTokens > 0 ||
		input.Usage.ReasoningTokens > 0
	hasEstimatedCanceledInput := errors.Is(input.Error, ErrMessageGenerationCanceled) &&
		input.UpstreamCallStarted &&
		input.EstimatedInputTokens > 0
	return strings.TrimSpace(input.AssistantText) != "" || hasRetainedToolTrace || hasObservedUsage || hasEstimatedCanceledInput
}

// resolveInterruptedMessageGenerationMetrics 统一处理中断消息的真实 usage 与估算兜底。
func resolveInterruptedMessageGenerationMetrics(input persistInterruptedMessageGenerationInput) interruptedMessageGenerationMetrics {
	inputTokens := resolveObservedOrHigherEstimatedTokens(input.Usage.InputTokens, input.EstimatedInputTokens)
	outputTokens := resolveObservedOrHigherEstimatedOutputTokens(input.Usage.OutputTokens, input.AssistantText)
	latencyMS := input.AssistantLatency
	if latencyMS < 0 {
		latencyMS = time.Since(input.StartedAt).Milliseconds()
	}
	if latencyMS < 0 {
		latencyMS = 0
	}
	return interruptedMessageGenerationMetrics{
		InputTokens:      inputTokens,
		OutputTokens:     outputTokens,
		LatencyMS:        latencyMS,
		ErrorCode:        classifyRunErrorCode(input.Error),
		ErrorMessage:     truncateError(messageErrorSummary(input.Error), 255),
		CacheReadTokens:  input.Usage.CacheReadTokens,
		CacheWriteTokens: input.Usage.CacheWriteTokens,
		ReasoningTokens:  input.Usage.ReasoningTokens,
	}
}

func interruptedCompletionInputTokens(input persistInterruptedMessageGenerationInput, metrics interruptedMessageGenerationMetrics) int64 {
	if input.ReuseUserMessage {
		return metrics.InputTokens
	}
	return 0
}

func interruptedCompletionCacheReadTokens(input persistInterruptedMessageGenerationInput, metrics interruptedMessageGenerationMetrics) int64 {
	if input.ReuseUserMessage {
		return metrics.CacheReadTokens
	}
	return 0
}

func interruptedCompletionCacheWriteTokens(input persistInterruptedMessageGenerationInput, metrics interruptedMessageGenerationMetrics) int64 {
	if input.ReuseUserMessage {
		return metrics.CacheWriteTokens
	}
	return 0
}

// applyInterruptedMessageGenerationState 同步内存消息对象，保证后续响应、run 记录和持久化状态一致。
func applyInterruptedMessageGenerationState(input persistInterruptedMessageGenerationInput, metrics interruptedMessageGenerationMetrics) {
	if !input.ReuseUserMessage {
		input.UserMessage.Status = "success"
		input.UserMessage.ErrorCode = ""
		input.UserMessage.ErrorMessage = ""
		input.UserMessage.InputTokens = metrics.InputTokens
		input.UserMessage.CacheReadTokens = metrics.CacheReadTokens
		input.UserMessage.CacheWriteTokens = metrics.CacheWriteTokens
		input.UserMessage.TokenUsage = metrics.InputTokens + metrics.CacheReadTokens + metrics.CacheWriteTokens
	}

	input.AssistantMessage.Content = input.AssistantText
	if input.ReuseUserMessage {
		input.AssistantMessage.InputTokens = metrics.InputTokens
		input.AssistantMessage.CacheReadTokens = metrics.CacheReadTokens
		input.AssistantMessage.CacheWriteTokens = metrics.CacheWriteTokens
	}
	input.AssistantMessage.TokenUsage = metrics.OutputTokens + metrics.ReasoningTokens
	if input.ReuseUserMessage {
		input.AssistantMessage.TokenUsage += metrics.InputTokens + metrics.CacheReadTokens + metrics.CacheWriteTokens
	}
	input.AssistantMessage.OutputTokens = metrics.OutputTokens
	input.AssistantMessage.ReasoningTokens = metrics.ReasoningTokens
	input.AssistantMessage.LatencyMS = metrics.LatencyMS
	input.AssistantMessage.Status = retainedGenerationStatus(input.Error)
	input.AssistantMessage.ErrorCode = metrics.ErrorCode
	input.AssistantMessage.ErrorMessage = metrics.ErrorMessage
}

func retainedGenerationStatus(err error) string {
	if errors.Is(err, ErrMessageGenerationCanceled) {
		return "canceled"
	}
	return "interrupted"
}

// buildInterruptedSendMessageResult 构造中断回复响应，供 handler 继续走计费和前端展示链路。
func buildInterruptedSendMessageResult(input persistInterruptedMessageGenerationInput, metrics interruptedMessageGenerationMetrics) *SendMessageResult {
	result := &SendMessageResult{
		UserMessage:         *input.UserMessage,
		AssistantMessage:    *input.AssistantMessage,
		Billable:            true,
		EffectiveOptions:    input.EffectiveOptions,
		UsageSpeed:          input.Usage.Speed,
		UsageServiceTier:    input.Usage.ServiceTier,
		RawUsageJSON:        input.Usage.RawUsageJSON,
		CacheWrite5mTokens:  input.Usage.CacheWrite5mTokens,
		CacheWrite1hTokens:  input.Usage.CacheWrite1hTokens,
		ServerSideToolUsage: input.ServerSideToolUsage,
		LatencyMS:           metrics.LatencyMS,
		StartedAt:           input.StartedAt,
	}
	if input.Route != nil {
		result.UpstreamID = input.Route.UpstreamID
		result.UpstreamName = input.Route.UpstreamName
		result.PlatformModelName = input.Route.PlatformModelName
		result.RoutedBindingCode = input.Route.BindingCode
		result.UpstreamModelName = input.Route.UpstreamModel
		result.UpstreamProtocol = input.Route.Protocol
	}
	return result
}

// persistMessageToolCalls 持久化工具调用并写入上下文 artifact，成功和中断路径共用同一套归属规则。
func (s *Service) persistMessageToolCalls(ctx context.Context, input persistMessageToolCallsInput) error {
	rows := normalizeMessageToolCallRows(input)
	if len(rows) == 0 {
		return nil
	}
	unpersisted := make([]model.ToolCall, 0, len(rows))
	for _, row := range rows {
		if _, ok := input.PersistedToolCallKeys[toolCallPersistenceKey(row)]; !ok {
			unpersisted = append(unpersisted, row)
		}
	}
	if len(unpersisted) > 0 {
		if err := s.repo.CreateConversationToolCalls(ctx, unpersisted); err != nil {
			return err
		}
	}
	s.persistToolContextArtifacts(ctx, toolContextArtifactInput{
		ConversationID: input.SendInput.ConversationID,
		UserID:         input.SendInput.UserID,
		MessageID:      input.UserMessageID,
		RunID:          input.RunID,
		Rows:           rows,
	})
	return nil
}

// normalizeMessageToolCallRows 补齐工具调用归属字段，避免不同路径写入的 trace 缺少 message/run 关联。
func normalizeMessageToolCallRows(input persistMessageToolCallsInput) []model.ToolCall {
	if len(input.Rows) == 0 {
		return nil
	}
	rows := append([]model.ToolCall(nil), input.Rows...)
	for i := range rows {
		if rows[i].ConversationID == 0 {
			rows[i].ConversationID = input.SendInput.ConversationID
		}
		if rows[i].UserID == 0 {
			rows[i].UserID = input.SendInput.UserID
		}
		if rows[i].MessageID == 0 {
			rows[i].MessageID = input.AssistantMessageID
		}
		if strings.TrimSpace(rows[i].RunID) == "" {
			rows[i].RunID = input.RunID
		}
	}
	return rows
}

func (s *Service) updateStatefulResponseAsync(conversationID uint, responseID string, promptFingerprint string) {
	respID := strings.TrimSpace(responseID)
	if respID == "" {
		return
	}
	fingerprint := strings.TrimSpace(promptFingerprint)
	if fingerprint == "" {
		return
	}
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.repo.UpdateConversationStatefulResponse(bgCtx, conversationID, respID, fingerprint)
	}()
}

func (s *Service) embedMessagePairAsync(input SendMessageInput, userMessage *model.Message, assistantMessage *model.Message) {
	cfg := s.cfg.Snapshot()
	if !cfg.EmbeddingEnabled || !cfg.MessageEmbeddingEnabled {
		return
	}
	go func() {
		asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.embedMessagePair(asyncCtx, input.ConversationID, input.UserID, userMessage, assistantMessage)
	}()
}
