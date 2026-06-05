package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

const resumeActiveCheckInterval = 5 * time.Second

var reservedMessageOptionKeys = map[string]struct{}{
	"contents":          {},
	"instructions":      {},
	"input":             {},
	"messages":          {},
	"model":             {},
	"prompt":            {},
	"stream":            {},
	"system":            {},
	"systemInstruction": {},
}

func sanitizeMessageOptions(options map[string]interface{}) map[string]interface{} {
	if len(options) == 0 {
		return nil
	}
	sanitized := make(map[string]interface{}, len(options))
	for key, value := range options {
		if _, ok := reservedMessageOptionKeys[key]; ok {
			continue
		}
		sanitized[key] = value
	}
	if len(sanitized) == 0 {
		return nil
	}
	return sanitized
}

// parseSendMessageInput 解析消息发送请求的公共参数。
func (h *Handler) parseSendMessageInput(c *gin.Context) (appconversation.SendMessageInput, *model.Conversation, *SendMessageRequest, error) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return appconversation.SendMessageInput{}, nil, nil, err
	}

	var req SendMessageRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return appconversation.SendMessageInput{}, nil, nil, err
	}
	req.ClientRunID = appconversation.EnsureMessageGenerationRunID(req.ClientRunID)
	req.Options = sanitizeMessageOptions(req.Options)
	// 流式接口写入响应头前先拦截明显超限请求，避免后续只能用 NDJSON error 表达 400。
	if err = h.service.ValidateSelectedToolIDs(req.SelectedToolIDs); err != nil {
		handleSendMessageError(c, err)
		return appconversation.SendMessageInput{}, nil, nil, err
	}

	conversation, err := h.service.GetConversationByPublicID(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.Error(c, http.StatusNotFound, "conversation not found")
			return appconversation.SendMessageInput{}, nil, nil, err
		}
		response.Error(c, http.StatusInternalServerError, "load conversation failed")
		return appconversation.SendMessageInput{}, nil, nil, err
	}

	input := appconversation.SendMessageInput{
		UserID:                  userID,
		ConversationID:          conversation.ID,
		RequestID:               middleware.MustRequestID(c),
		ContentType:             req.ContentType,
		Content:                 req.Content,
		PlatformModelName:       req.Model,
		Options:                 req.Options,
		ClientRunID:             req.ClientRunID,
		FileIDs:                 req.FileIDs,
		SelectedToolIDs:         req.SelectedToolIDs,
		HTMLVisualPromptEnabled: req.HTMLVisualPromptEnabled,
		HTMLVisualColorMode:     req.HTMLVisualColorMode,
		ParentMessagePublicID:   req.ParentMessagePublicID,
		SourceMessagePublicID:   req.SourceMessagePublicID,
		BranchReason:            req.BranchReason,
	}

	return input, conversation, &req, nil
}

func sendMessageBillingInput(
	userID uint,
	conversation *model.Conversation,
	req *SendMessageRequest,
	result *appconversation.SendMessageResult,
) appconversation.SendMessageBillingInput {
	input := appconversation.SendMessageBillingInput{
		UserID:            userID,
		PlatformModelName: strings.TrimSpace(req.Model),
		ClientRunID:       strings.TrimSpace(req.ClientRunID),
		Result:            result,
	}
	if conversation != nil {
		input.ConversationID = conversation.ID
		input.ConversationModel = conversation.Model
	}
	return input
}

func (h *Handler) reserveSendMessageUsageBalance(c *gin.Context, conversation *model.Conversation, req *SendMessageRequest) (*domainbilling.UsageBalanceReservation, error) {
	reservation, err := h.service.ReserveSendMessageUsageBalance(
		c.Request.Context(),
		sendMessageBillingInput(middleware.MustUserID(c), conversation, req, nil),
	)
	if err != nil {
		if errors.Is(err, billing.ErrUsageBalanceInsufficient) {
			response.Error(c, http.StatusPaymentRequired, "usage balance is insufficient")
			return nil, err
		}
		if errors.Is(err, billing.ErrModelPricingRequired) {
			response.Error(c, http.StatusPaymentRequired, "model pricing is required")
			return nil, err
		}
		response.Error(c, http.StatusInternalServerError, "usage balance reservation failed")
		return nil, err
	}
	return reservation, nil
}

func shouldReleaseReservationAfterBillingError(err error) bool {
	return appconversation.ShouldReleaseSendMessageUsageReservationAfterBillingError(err)
}

func (h *Handler) releaseSendMessageUsageReservation(reservation *domainbilling.UsageBalanceReservation, description string) error {
	if reservation == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.service.ReleaseSendMessageUsageReservation(ctx, reservation, description)
}

// recordAndApplySendMessageBilling 统一记录账单并把快照回填到当前响应消息，避免流式和非流式口径分叉。
func (h *Handler) recordAndApplySendMessageBilling(
	ctx context.Context,
	userID uint,
	conversation *model.Conversation,
	req *SendMessageRequest,
	result *appconversation.SendMessageResult,
	reservation *domainbilling.UsageBalanceReservation,
) error {
	usageLedger, err := h.service.RecordSendMessageBilling(
		ctx,
		sendMessageBillingInput(userID, conversation, req, result),
		reservation,
	)
	if err != nil {
		return err
	}
	appconversation.ApplyUsageBilling(&result.AssistantMessage, usageLedger)
	return nil
}

// recordSendMessageAudit 记录审计日志（同步，供非流式路径使用）。
func (h *Handler) recordSendMessageAudit(c *gin.Context, conversation *model.Conversation, req *SendMessageRequest, result *appconversation.SendMessageResult, action string) {
	h.recordSendMessageAuditCtx(
		c.Request.Context(),
		middleware.MustUserID(c),
		middleware.MustRequestID(c),
		c.ClientIP(),
		c.Request.UserAgent(),
		conversation, req, result, action,
	)
}

// recordStreamSendMessageAuditAsync 在 Handler 返回前提取 gin.Context 值，goroutine 内不持有 gin.Context。
func (h *Handler) recordStreamSendMessageAuditAsync(
	c *gin.Context,
	conversation *model.Conversation,
	req *SendMessageRequest,
	result *appconversation.SendMessageResult,
	action string,
) {
	bgUserID := middleware.MustUserID(c)
	bgRequestID := middleware.MustRequestID(c)
	bgClientIP := c.ClientIP()
	bgUserAgent := c.Request.UserAgent()
	go h.recordSendMessageAuditCtx(
		context.Background(),
		bgUserID, bgRequestID, bgClientIP, bgUserAgent,
		conversation, req, result, action,
	)
}

// recordSendMessageAuditCtx 接受显式参数，可在 goroutine 中安全调用（不依赖 gin.Context）。
func (h *Handler) recordSendMessageAuditCtx(
	ctx context.Context,
	userID uint,
	requestID string,
	clientIP string,
	userAgent string,
	conversation *model.Conversation,
	req *SendMessageRequest,
	result *appconversation.SendMessageResult,
	action string,
) {
	h.service.RecordSendMessageAudit(
		ctx,
		appconversation.SendMessageAuditInput{
			UserID:         userID,
			RequestID:      requestID,
			ClientIP:       clientIP,
			UserAgent:      userAgent,
			Action:         action,
			ContentType:    req.ContentType,
			ConversationID: conversation.ID,
			FileIDs:        req.FileIDs,
			Result:         result,
		},
	)
}

func handleSendMessageBillingError(c *gin.Context, err error) {
	if errors.Is(err, billing.ErrUsageBalanceInsufficient) {
		response.Error(c, http.StatusPaymentRequired, "usage balance is insufficient")
		return
	}
	if errors.Is(err, billing.ErrModelPricingRequired) {
		response.Error(c, http.StatusPaymentRequired, "model pricing is required")
		return
	}
	response.Error(c, http.StatusInternalServerError, "record billing failed")
}

func mapBillingStreamError(err error) streamError {
	status := http.StatusInternalServerError
	message := "record billing failed"
	if errors.Is(err, billing.ErrUsageBalanceInsufficient) {
		status = http.StatusPaymentRequired
		message = "usage balance is insufficient"
	}
	if errors.Is(err, billing.ErrModelPricingRequired) {
		status = http.StatusPaymentRequired
		message = "model pricing is required"
	}
	code := response.InferErrorCode(status, message)
	return streamError{
		Status:  status,
		Code:    code,
		Message: response.PublicErrorMessage(status, code, message),
	}
}

func billingStreamErrorPayload(err error) map[string]interface{} {
	mapped := mapBillingStreamError(err)
	return map[string]interface{}{
		"type":      "error",
		"message":   mapped.Message,
		"errorCode": mapped.Code,
	}
}

// handleSendMessageError 处理发送消息错误的公共方法。
func handleSendMessageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appconversation.ErrConversationNotFound):
		response.Error(c, http.StatusNotFound, "conversation not found")
	case errors.Is(err, appconversation.ErrInvalidFileReference):
		response.Error(c, http.StatusBadRequest, "invalid file reference")
	case errors.Is(err, appconversation.ErrTooManyMessageFiles):
		response.Error(c, http.StatusBadRequest, "too many files in one message")
	case errors.Is(err, appconversation.ErrTooManySelectedTools):
		response.Error(c, http.StatusBadRequest, "too many selected tools")
	case errors.Is(err, appconversation.ErrInvalidMessageBranch):
		response.Error(c, http.StatusBadRequest, "invalid message branch")
	case errors.Is(err, appconversation.ErrFileProcessingNotReady):
		response.Error(c, http.StatusBadRequest, "file processing not ready")
	case errors.Is(err, appconversation.ErrFileTooLargeForFullContext):
		response.Error(c, http.StatusBadRequest, "file too large for full context")
	case errors.Is(err, appconversation.ErrEmbeddingUnavailable):
		response.Error(c, http.StatusBadRequest, "embedding unavailable for current file capability")
	case errors.Is(err, appconversation.ErrModelRouteNotConfigured):
		response.Error(c, http.StatusServiceUnavailable, "model route not configured")
	case errors.Is(err, appconversation.ErrUpstreamEmptyResponse):
		response.Error(c, http.StatusBadGateway, "model returned empty response")
	case errors.Is(err, appconversation.ErrUpstreamRequestFailed):
		if code := appconversation.MessageErrorCode(err); code != "" {
			response.ErrorWithCode(c, http.StatusBadGateway, code, mapClientErrorMessage(err))
			return
		}
		response.Error(c, http.StatusBadGateway, mapClientErrorMessage(err))
	default:
		response.Error(c, http.StatusInternalServerError, "send message failed")
	}
}

// SendMessage godoc
// @Summary 发送消息
// @Description 在会话中发送消息，支持文件/图片等多模态附件
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param body body SendMessageRequest true "消息参数"
// @Success 200 {object} SendMessageResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/messages [post]
// SendMessage 发送消息。
func (h *Handler) SendMessage(c *gin.Context) {
	input, conversation, req, err := h.parseSendMessageInput(c)
	if err != nil {
		return
	}
	if err = h.ensureBillingModelAccess(c, conversation, req); err != nil {
		return
	}
	reservation, err := h.reserveSendMessageUsageBalance(c, conversation, req)
	if err != nil {
		return
	}

	result, err := h.service.SendMessage(c.Request.Context(), input)
	if err != nil {
		if result != nil {
			if !result.Billable {
				if releaseErr := h.releaseSendMessageUsageReservation(reservation, "模型调用失败退回预扣"); releaseErr != nil {
					handleSendMessageBillingError(c, releaseErr)
					return
				}
				handleSendMessageError(c, err)
				return
			}
			if billingErr := h.recordAndApplySendMessageBilling(c.Request.Context(), middleware.MustUserID(c), conversation, req, result, reservation); billingErr != nil {
				if shouldReleaseReservationAfterBillingError(billingErr) {
					_ = h.releaseSendMessageUsageReservation(reservation, "计费失败退回预扣")
				}
				handleSendMessageBillingError(c, billingErr)
				return
			}
			h.recordSendMessageAudit(c, conversation, req, result, "send_message")
			response.Success(c, toSendMessageResponse(result))
			return
		}
		if releaseErr := h.releaseSendMessageUsageReservation(reservation, "模型调用失败退回预扣"); releaseErr != nil {
			handleSendMessageBillingError(c, releaseErr)
			return
		}
		handleSendMessageError(c, err)
		return
	}

	if err := h.recordAndApplySendMessageBilling(c.Request.Context(), middleware.MustUserID(c), conversation, req, result, reservation); err != nil {
		if shouldReleaseReservationAfterBillingError(err) {
			_ = h.releaseSendMessageUsageReservation(reservation, "计费失败退回预扣")
		}
		handleSendMessageBillingError(c, err)
		return
	}
	h.recordSendMessageAudit(c, conversation, req, result, "send_message")
	response.Success(c, toSendMessageResponse(result))
}

// StreamMessage godoc
// @Summary 流式发送消息
// @Description 在会话中发送消息并以 NDJSON 流式返回 assistant 增量文本
// @Tags chat
// @Accept json
// @Produce application/x-ndjson
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param body body SendMessageRequest true "消息参数"
// @Success 200 {string} string "NDJSON stream"
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/messages/stream [post]
func (h *Handler) StreamMessage(c *gin.Context) {
	input, conversation, req, err := h.parseSendMessageInput(c)
	if err != nil {
		return
	}
	if err = h.ensureBillingModelAccess(c, conversation, req); err != nil {
		return
	}
	reservation, err := h.reserveSendMessageUsageBalance(c, conversation, req)
	if err != nil {
		return
	}

	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	var clientDisconnected atomic.Bool
	flushStreamEvent := func(payload map[string]interface{}) error {
		payload = h.service.PublishMessageGenerationEvent(input.ClientRunID, payload)
		if clientDisconnected.Load() {
			return nil
		}
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return marshalErr
		}
		if _, writeErr := c.Writer.Write(append(encoded, '\n')); writeErr != nil {
			clientDisconnected.Store(true)
			return writeErr
		}
		c.Writer.Flush()
		return nil
	}

	// 有附件时先推送文件处理事件，提升用户体验感知。
	if len(req.FileIDs) > 0 {
		_ = flushStreamEvent(map[string]interface{}{
			"type":    "file_proc",
			"message": "正在处理附件…",
		})
	}

	// 将中间事件（rag_search 等）通过 NDJSON 推送给客户端。
	input.OnEvent = func(eventType string, payload map[string]interface{}) error {
		_ = flushStreamEvent(normalizeStreamEventPayload(eventType, payload))
		return nil
	}

	result, err := h.service.StreamMessage(c.Request.Context(), input, func(delta string) error {
		_ = flushStreamEvent(map[string]interface{}{
			"type":  "delta",
			"delta": delta,
		})
		return nil
	})
	if err != nil {
		if result != nil {
			if !result.Billable {
				if releaseErr := h.releaseSendMessageUsageReservation(reservation, "模型调用失败退回预扣"); releaseErr != nil {
					_ = flushStreamEvent(billingStreamErrorPayload(releaseErr))
					h.service.FinishMessageGeneration(input.ClientRunID)
					return
				}
				payload := streamErrorPayload(err)
				payload["data"] = toSendMessageResponse(result)
				if debug := appconversation.MessageErrorDebug(err); debug != nil {
					payload["debug"] = debug
				}
				_ = flushStreamEvent(payload)
				h.service.FinishMessageGeneration(input.ClientRunID)
				h.recordStreamSendMessageAuditAsync(c, conversation, req, result, "stream_message")
				return
			}
			billingCtx, billingCancel := context.WithTimeout(context.Background(), 10*time.Second)
			billingErr := h.recordAndApplySendMessageBilling(billingCtx, middleware.MustUserID(c), conversation, req, result, reservation)
			billingCancel()
			if billingErr != nil {
				if shouldReleaseReservationAfterBillingError(billingErr) {
					_ = h.releaseSendMessageUsageReservation(reservation, "计费失败退回预扣")
				}
				payload := billingStreamErrorPayload(billingErr)
				payload["data"] = toSendMessageResponse(result)
				_ = flushStreamEvent(payload)
				h.service.FinishMessageGeneration(input.ClientRunID)
				return
			}
			payload := streamErrorPayload(err)
			payload["data"] = toSendMessageResponse(result)
			if debug := appconversation.MessageErrorDebug(err); debug != nil {
				payload["debug"] = debug
			}
			_ = flushStreamEvent(payload)
			h.service.FinishMessageGeneration(input.ClientRunID)
			h.recordStreamSendMessageAuditAsync(c, conversation, req, result, "stream_message")
			return
		}
		if releaseErr := h.releaseSendMessageUsageReservation(reservation, "模型调用失败退回预扣"); releaseErr != nil {
			_ = flushStreamEvent(billingStreamErrorPayload(releaseErr))
			h.service.FinishMessageGeneration(input.ClientRunID)
			return
		}
		payload := streamErrorPayload(err)
		if debug := appconversation.MessageErrorDebug(err); debug != nil {
			payload["debug"] = debug
		}
		_ = flushStreamEvent(payload)
		h.service.FinishMessageGeneration(input.ClientRunID)
		return
	}

	billingCtx, billingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	billingErr := h.recordAndApplySendMessageBilling(billingCtx, middleware.MustUserID(c), conversation, req, result, reservation)
	billingCancel()
	if billingErr != nil {
		if shouldReleaseReservationAfterBillingError(billingErr) {
			_ = h.releaseSendMessageUsageReservation(reservation, "计费失败退回预扣")
		}
		_ = flushStreamEvent(billingStreamErrorPayload(billingErr))
		h.service.FinishMessageGeneration(input.ClientRunID)
		return
	}

	_ = flushStreamEvent(map[string]interface{}{
		"type": "completed",
		"data": toSendMessageResponse(result),
	})
	h.service.FinishMessageGeneration(input.ClientRunID)
	h.recordStreamSendMessageAuditAsync(c, conversation, req, result, "stream_message")
}

// CancelMessageGeneration godoc
// @Summary 取消流式生成
// @Description 仅在用户显式点击暂停时取消对应 run；浏览器刷新或断开连接不会调用此接口
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param run_id path string true "运行 ID"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Router /conversation-runs/{run_id}/cancel [post]
func (h *Handler) CancelMessageGeneration(c *gin.Context) {
	runID, err := stringParam(c, "run_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid run id")
		return
	}
	canceled := h.service.CancelMessageGeneration(c.Request.Context(), middleware.MustUserID(c), runID)
	response.Success(c, CancelMessageGenerationResponse{Canceled: canceled})
}

// ResumeMessageGenerationStream godoc
// @Summary 恢复流式生成订阅
// @Description 页面刷新后按 run_id 重新订阅仍在运行的生成流，返回 NDJSON 事件
// @Tags chat
// @Produce application/x-ndjson
// @Security BearerAuth
// @Param run_id path string true "运行 ID"
// @Param after query int false "已接收的最后事件序号"
// @Success 200 {string} string "NDJSON stream"
// @Failure 404 {object} ErrorDoc
// @Router /conversation-runs/{run_id}/stream [get]
func (h *Handler) ResumeMessageGenerationStream(c *gin.Context) {
	runID, err := stringParam(c, "run_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid run id")
		return
	}
	afterSeq, _ := strconv.ParseInt(strings.TrimSpace(c.Query("after")), 10, 64)
	if afterSeq < 0 {
		afterSeq = 0
	}
	userID := middleware.MustUserID(c)
	replay, events, unsubscribe, ok := h.service.SubscribeMessageGeneration(
		c.Request.Context(),
		userID,
		runID,
		afterSeq,
	)
	if !ok {
		h.service.MarkMessageGenerationInterrupted(c.Request.Context(), userID, runID)
		response.Error(c, http.StatusNotFound, "generation stream not found")
		return
	}
	defer unsubscribe()

	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	isTerminal := func(payload map[string]interface{}) bool {
		eventType, _ := payload["type"].(string)
		return eventType == "completed" || eventType == "error"
	}
	terminalWritten := false
	writeEvent := func(payload map[string]interface{}) bool {
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return true
		}
		if _, writeErr := c.Writer.Write(append(encoded, '\n')); writeErr != nil {
			return false
		}
		c.Writer.Flush()
		if isTerminal(payload) {
			terminalWritten = true
		}
		return true
	}

	for _, event := range replay {
		if !writeEvent(event.Payload) {
			return
		}
	}
	if terminalWritten {
		return
	}

	isActive := func() bool {
		return h.service.HasActiveMessageGeneration(c.Request.Context(), runID)
	}
	if !isActive() {
		h.service.MarkMessageGenerationInterrupted(c.Request.Context(), userID, runID)
		_ = writeEvent(streamErrorPayloadWithCode("conversation_run.stream_interrupted", "generation stream was interrupted; retry this message"))
		return
	}
	activeTicker := time.NewTicker(resumeActiveCheckInterval)
	defer func() {
		activeTicker.Stop()
	}()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-activeTicker.C:
			if !isActive() {
				h.service.MarkMessageGenerationInterrupted(c.Request.Context(), userID, runID)
				_ = writeEvent(streamErrorPayloadWithCode("conversation_run.stream_interrupted", "generation stream was interrupted; retry this message"))
				return
			}
		case event, ok := <-events:
			if !ok {
				if !terminalWritten && !isActive() {
					h.service.MarkMessageGenerationInterrupted(c.Request.Context(), userID, runID)
					_ = writeEvent(streamErrorPayloadWithCode("conversation_run.stream_interrupted", "generation stream was interrupted; retry this message"))
				}
				return
			}
			if !writeEvent(event.Payload) {
				return
			}
			if terminalWritten {
				return
			}
		}
	}
}

func (h *Handler) ensureBillingModelAccess(c *gin.Context, conversation *model.Conversation, req *SendMessageRequest) error {
	if err := h.service.EnsureSendMessageBillingAccess(
		c.Request.Context(),
		sendMessageBillingInput(middleware.MustUserID(c), conversation, req, nil),
	); err != nil {
		if errors.Is(err, billing.ErrPeriodCreditExceeded) {
			response.Error(c, http.StatusPaymentRequired, "period usage credit exceeded")
			return err
		}
		if errors.Is(err, billing.ErrModelPricingRequired) {
			response.Error(c, http.StatusPaymentRequired, "model pricing is required")
			return err
		}
		if errors.Is(err, billing.ErrUsageBalanceInsufficient) {
			response.Error(c, http.StatusPaymentRequired, "usage balance is insufficient")
			return err
		}
		response.Error(c, http.StatusInternalServerError, "billing access check failed")
		return err
	}
	return nil
}
