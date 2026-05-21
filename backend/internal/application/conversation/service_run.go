package conversation

import (
	"context"
	"errors"
	"strings"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"go.uber.org/zap"
)

type messageSendRunState struct {
	service          *Service
	run              *model.Run
	startedAt        time.Time
	userMessage      **model.Message
	assistantMessage **model.Message
	traceRecorder    **messageTraceRecorder
	result           **SendMessageResult
	traceContext     context.Context
}

func newMessageSendRunState(
	service *Service,
	input SendMessageInput,
	conversation *model.Conversation,
	startedAt time.Time,
	runID string,
) *messageSendRunState {
	return &messageSendRunState{
		service:   service,
		startedAt: startedAt,
		run: &model.Run{
			RunID:              runID,
			RequestID:          strings.TrimSpace(input.RequestID),
			UserID:             input.UserID,
			ConversationID:     input.ConversationID,
			TaskType:           "chat",
			Endpoint:           llm.EndpointResponses,
			Provider:           strings.TrimSpace(conversation.Provider),
			ProviderProtocol:   "",
			UpstreamID:         0,
			UpstreamModelID:    0,
			UpstreamName:       "",
			RequestedModelName: strings.TrimSpace(conversation.Model),
			PlatformModelName:  "",
			RoutedBindingCode:  "",
			ModelVendor:        "",
			ModelIcon:          "",
			UpstreamModelName:  "",
			InputTokens:        0,
			OutputTokens:       0,
			CacheReadTokens:    0,
			CacheWriteTokens:   0,
			ReasoningTokens:    0,
			ToolCallsCount:     0,
			Status:             "error",
			ErrorCode:          "",
			ErrorMessage:       "",
			StartedAt:          startedAt,
			EndedAt:            nil,
		},
	}
}

func (r *messageSendRunState) bind(
	userMessage **model.Message,
	assistantMessage **model.Message,
	traceRecorder **messageTraceRecorder,
	result **SendMessageResult,
	traceContext context.Context,
) {
	r.userMessage = userMessage
	r.assistantMessage = assistantMessage
	r.traceRecorder = traceRecorder
	r.result = result
	r.traceContext = traceContext
}

func (r *messageSendRunState) finalize(ctx context.Context, retErr error) {
	if r == nil || r.service == nil || r.run == nil {
		return
	}
	finalizeCtx := ctx
	var finalizeCancel context.CancelFunc
	if ctx.Err() != nil {
		finalizeCtx, finalizeCancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer finalizeCancel()
	}

	r.finalizeRun(retErr)
	r.finalizeUserMessage(finalizeCtx, retErr)
	r.finalizeAssistantMessage(finalizeCtx, retErr)
	r.createRun(finalizeCtx)
}

func (r *messageSendRunState) finalizeRun(retErr error) {
	endedAt := time.Now()
	r.run.EndedAt = &endedAt
	r.run.TotalLatencyMS = endedAt.Sub(r.startedAt).Milliseconds()
	if r.run.TotalLatencyMS < 0 {
		r.run.TotalLatencyMS = 0
	}
	switch {
	case retErr == nil:
		r.run.Status = "success"
	case errors.Is(retErr, ErrMessageGenerationCanceled):
		r.run.Status = "canceled"
		r.run.ErrorCode = classifyRunErrorCode(retErr)
		r.run.ErrorMessage = truncateError(retErr.Error(), 255)
	default:
		r.run.Status = "error"
		r.run.ErrorCode = classifyRunErrorCode(retErr)
		r.run.ErrorMessage = truncateError(retErr.Error(), 255)
	}
}

func (r *messageSendRunState) finalizeUserMessage(ctx context.Context, retErr error) {
	userMessage := r.currentUserMessage()
	if userMessage == nil {
		return
	}
	messageStatus := "success"
	messageErrorCode := ""
	messageErrorMessage := ""
	if retErr != nil && !errors.Is(retErr, ErrMessageGenerationCanceled) {
		messageStatus = "error"
		messageErrorCode = classifyRunErrorCode(retErr)
		messageErrorMessage = truncateError(messageErrorSummary(retErr), 255)
	}
	if err := r.service.repo.UpdateMessageState(ctx, userMessage.ID, messageStatus, messageErrorCode, messageErrorMessage); err != nil {
		r.service.logger.Error("update_message_state_failed",
			zap.String("trace_id", traceid.FromContext(r.traceContext)),
			zap.Uint("message_id", userMessage.ID),
			zap.Error(err),
		)
	}
	userMessage.Status = messageStatus
	userMessage.ErrorCode = messageErrorCode
	userMessage.ErrorMessage = messageErrorMessage
	if result := r.currentResult(); result != nil {
		result.UserMessage.Status = messageStatus
		result.UserMessage.ErrorCode = messageErrorCode
		result.UserMessage.ErrorMessage = messageErrorMessage
	}
}

func (r *messageSendRunState) finalizeAssistantMessage(ctx context.Context, retErr error) {
	if retErr == nil {
		return
	}
	assistantMessage := r.currentAssistantMessage()
	if assistantMessage == nil {
		return
	}
	messageStatus := "error"
	if errors.Is(retErr, ErrMessageGenerationCanceled) {
		messageStatus = "canceled"
	}
	messageErrorCode := classifyRunErrorCode(retErr)
	messageErrorMessage := truncateError(messageErrorSummary(retErr), 255)
	if err := r.service.repo.UpdateMessageState(ctx, assistantMessage.ID, messageStatus, messageErrorCode, messageErrorMessage); err != nil {
		r.service.logger.Error("update_assistant_message_state_failed",
			zap.String("trace_id", traceid.FromContext(r.traceContext)),
			zap.Uint("message_id", assistantMessage.ID),
			zap.Error(err),
		)
	}
	assistantMessage.Status = messageStatus
	assistantMessage.ErrorCode = messageErrorCode
	assistantMessage.ErrorMessage = messageErrorMessage
	if traceRecorder := r.currentTraceRecorder(); traceRecorder != nil {
		traceRecorder.fail(retErr)
	}
	if result := r.currentResult(); result != nil {
		result.AssistantMessage.Status = messageStatus
		result.AssistantMessage.ErrorCode = messageErrorCode
		result.AssistantMessage.ErrorMessage = messageErrorMessage
	}
}

func (r *messageSendRunState) createRun(ctx context.Context) {
	if err := r.service.repo.CreateConversationRun(ctx, r.run); err != nil {
		r.service.logger.Error("create_conversation_run_failed",
			zap.String("trace_id", traceid.FromContext(r.traceContext)),
			zap.String("run_id", r.run.RunID),
			zap.Error(err),
		)
	}
}

func (r *messageSendRunState) currentUserMessage() *model.Message {
	if r.userMessage == nil {
		return nil
	}
	return *r.userMessage
}

func (r *messageSendRunState) currentAssistantMessage() *model.Message {
	if r.assistantMessage == nil {
		return nil
	}
	return *r.assistantMessage
}

func (r *messageSendRunState) currentTraceRecorder() *messageTraceRecorder {
	if r.traceRecorder == nil {
		return nil
	}
	return *r.traceRecorder
}

func (r *messageSendRunState) currentResult() *SendMessageResult {
	if r.result == nil {
		return nil
	}
	return *r.result
}
