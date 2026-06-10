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
	InputTokens               int64
	CacheReadTokens           int64
	CacheWriteTokens          int64
	OutputTokens              int64
	ReasoningTokens           int64
	AssistantLatency          int64
	ResponseID                string
	StatefulPromptFingerprint string
	ToolCallRows              []model.ToolCall
}

type persistInterruptedMessageGenerationInput struct {
	SendInput            SendMessageInput
	UserMessage          *model.Message
	AssistantMessage     *model.Message
	AssistantText        string
	EstimatedInputTokens int64
	Usage                llm.Usage
	AssistantLatency     int64
	Error                error
	ToolCallRows         []model.ToolCall
	TraceRecorder        *messageTraceRecorder
	Route                *channel.ResolvedRoute
	EffectiveOptions     map[string]interface{}
	ServerSideToolUsage  map[string]int64
	StartedAt            time.Time
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
	SendInput          SendMessageInput
	UserMessageID      uint
	AssistantMessageID uint
	RunID              string
	Rows               []model.ToolCall
}

func (s *Service) persistSuccessfulMessageGeneration(ctx context.Context, input persistMessageGenerationInput) error {
	input.UserMessage.InputTokens = input.InputTokens
	input.UserMessage.CacheReadTokens = input.CacheReadTokens
	input.UserMessage.CacheWriteTokens = input.CacheWriteTokens
	input.UserMessage.TokenUsage = input.InputTokens + input.CacheReadTokens + input.CacheWriteTokens

	if completed, err := s.persistAssistantImagePayloadIfPresent(ctx, input); err != nil {
		return err
	} else if completed {
		return s.finishSuccessfulMessageGeneration(ctx, input)
	}

	go func(msgID uint, inputTokens, cacheReadTokens, cacheWriteTokens int64) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.repo.UpdateMessageUsage(bgCtx, msgID, inputTokens, 0, cacheReadTokens, cacheWriteTokens, 0)
	}(input.UserMessage.ID, input.InputTokens, input.CacheReadTokens, input.CacheWriteTokens)

	if err := s.repo.UpdateAssistantMessageCompletion(
		ctx,
		input.AssistantMessage.ID,
		input.AssistantText,
		input.OutputTokens,
		input.ReasoningTokens,
		input.AssistantLatency,
		"success",
		"",
		"",
	); err != nil {
		return err
	}
	input.AssistantMessage.Content = input.AssistantText
	input.AssistantMessage.TokenUsage = input.OutputTokens + input.ReasoningTokens
	input.AssistantMessage.OutputTokens = input.OutputTokens
	input.AssistantMessage.ReasoningTokens = input.ReasoningTokens
	input.AssistantMessage.LatencyMS = input.AssistantLatency
	input.AssistantMessage.Status = "success"

	return s.finishSuccessfulMessageGeneration(ctx, input)
}

func (s *Service) persistAssistantImagePayloadIfPresent(ctx context.Context, input persistMessageGenerationInput) (bool, error) {
	normalized, err := s.normalizeAssistantImageContent(
		ctx,
		input.SendInput.UserID,
		input.SendInput.ConversationID,
		input.AssistantMessage.ID,
		successfulMessageGenerationModelName(input),
		input.AssistantText,
	)
	if err != nil || normalized == nil {
		return false, err
	}

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
			ContentType:     "image",
			Content:         normalized.Content,
			OutputTokens:    input.OutputTokens,
			ReasoningTokens: input.ReasoningTokens,
			LatencyMS:       input.AssistantLatency,
			Status:          "success",
		},
		normalized.AttachmentRows,
	); err != nil {
		return false, err
	}

	input.AssistantMessage.ContentType = "image"
	input.AssistantMessage.Content = normalized.Content
	input.AssistantMessage.TokenUsage = input.OutputTokens + input.ReasoningTokens
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
		SendInput:          input.SendInput,
		UserMessageID:      input.UserMessage.ID,
		AssistantMessageID: input.AssistantMessage.ID,
		RunID:              input.AssistantMessage.RunID,
		Rows:               input.ToolCallRows,
	}); err != nil {
		return err
	}

	s.updateStatefulResponseAsync(input.SendInput.ConversationID, input.ResponseID, input.StatefulPromptFingerprint)
	s.maybeGenerateConversationMetadataAsync(*input.Conversation, *input.UserMessage, *input.AssistantMessage)
	s.embedMessagePairAsync(input.SendInput, input.UserMessage, input.AssistantMessage)

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
	if err := s.repo.UpdateAssistantMessageCompletion(
		persistCtx,
		input.AssistantMessage.ID,
		input.AssistantText,
		metrics.OutputTokens,
		metrics.ReasoningTokens,
		metrics.LatencyMS,
		"interrupted",
		metrics.ErrorCode,
		metrics.ErrorMessage,
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
		SendInput:          input.SendInput,
		UserMessageID:      input.UserMessage.ID,
		AssistantMessageID: input.AssistantMessage.ID,
		RunID:              input.AssistantMessage.RunID,
		Rows:               input.ToolCallRows,
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

// shouldPersistInterruptedMessageGeneration 只在已有可展示内容或可追踪工具结果时保留中断消息。
func shouldPersistInterruptedMessageGeneration(input persistInterruptedMessageGenerationInput) bool {
	if input.Error == nil || input.UserMessage == nil || input.AssistantMessage == nil {
		return false
	}
	if errors.Is(input.Error, ErrMessageGenerationCanceled) {
		return false
	}
	hasRetainedToolTrace := len(input.ToolCallRows) > 0 || len(input.ServerSideToolUsage) > 0
	return strings.TrimSpace(input.AssistantText) != "" || hasRetainedToolTrace
}

// resolveInterruptedMessageGenerationMetrics 统一处理中断消息的真实 usage 与估算兜底。
func resolveInterruptedMessageGenerationMetrics(input persistInterruptedMessageGenerationInput) interruptedMessageGenerationMetrics {
	inputTokens := input.Usage.InputTokens
	if inputTokens <= 0 {
		inputTokens = input.EstimatedInputTokens
	}
	outputTokens := input.Usage.OutputTokens
	if outputTokens <= 0 && strings.TrimSpace(input.AssistantText) != "" {
		outputTokens = estimateTokens(input.AssistantText)
	}
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

// applyInterruptedMessageGenerationState 同步内存消息对象，保证后续响应、run 记录和持久化状态一致。
func applyInterruptedMessageGenerationState(input persistInterruptedMessageGenerationInput, metrics interruptedMessageGenerationMetrics) {
	input.UserMessage.Status = "success"
	input.UserMessage.ErrorCode = ""
	input.UserMessage.ErrorMessage = ""
	input.UserMessage.InputTokens = metrics.InputTokens
	input.UserMessage.CacheReadTokens = metrics.CacheReadTokens
	input.UserMessage.CacheWriteTokens = metrics.CacheWriteTokens
	input.UserMessage.TokenUsage = metrics.InputTokens + metrics.CacheReadTokens + metrics.CacheWriteTokens

	input.AssistantMessage.Content = input.AssistantText
	input.AssistantMessage.TokenUsage = metrics.OutputTokens + metrics.ReasoningTokens
	input.AssistantMessage.OutputTokens = metrics.OutputTokens
	input.AssistantMessage.ReasoningTokens = metrics.ReasoningTokens
	input.AssistantMessage.LatencyMS = metrics.LatencyMS
	input.AssistantMessage.Status = "interrupted"
	input.AssistantMessage.ErrorCode = metrics.ErrorCode
	input.AssistantMessage.ErrorMessage = metrics.ErrorMessage
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
	if err := s.repo.CreateConversationToolCalls(ctx, rows); err != nil {
		return err
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
