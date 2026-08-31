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

const (
	resumeActiveCheckInterval         = 5 * time.Second
	usageAuthorizationRenewalInterval = 30 * time.Minute
)

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
		SkillIDs:                req.SkillIDs,
		KnowledgeBaseIDs:        req.KnowledgeBaseIDs,
		HTMLVisualPromptEnabled: req.HTMLVisualPromptEnabled,
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
		input.Conversation = conversation
	}
	return input
}

func (h *Handler) reserveUsage(c *gin.Context, input appconversation.SendMessageBillingInput) (*domainbilling.UsageAuthorization, error) {
	return h.service.AuthorizeSendMessageUsage(
		c.Request.Context(),
		input,
	)
}

// authorizeUsage 在写入流式响应头前完成请求级计费授权。
func (h *Handler) authorizeUsage(c *gin.Context, input appconversation.SendMessageBillingInput) (*domainbilling.UsageAuthorization, error) {
	authorization, err := h.reserveUsage(c, input)
	if err != nil {
		handleUsageAuthorizationError(c, err)
		return nil, err
	}
	return authorization, nil
}

// authorizeMessageUsage 将终态拒绝的持久化委托给应用层，再把授权结果转换为 HTTP 响应。
func (h *Handler) authorizeMessageUsage(
	c *gin.Context,
	input appconversation.SendMessageInput,
	billingInput appconversation.SendMessageBillingInput,
) (*domainbilling.UsageAuthorization, error) {
	authorization, err := h.reserveUsage(c, billingInput)
	if err == nil {
		return authorization, nil
	}
	if persistErr := h.service.PersistMessageUsageRejection(c.Request.Context(), input, err); persistErr != nil {
		handleSendMessageError(c, persistErr)
		return nil, persistErr
	}
	handleUsageAuthorizationError(c, err)
	return nil, err
}

// releaseSendMessageUsageAuthorization 使用独立短上下文释放未消费的预算。
func (h *Handler) releaseSendMessageUsageAuthorization(authorization *domainbilling.UsageAuthorization) error {
	if authorization == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.service.ReleaseSendMessageUsageAuthorization(ctx, authorization)
}

// startUsageAuthorizationRenewal 为长时间运行的调用持续刷新预算租约。
func (h *Handler) startUsageAuthorizationRenewal(authorization *domainbilling.UsageAuthorization) func() {
	if authorization == nil || authorization.Reservation == nil {
		return func() {}
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(usageAuthorizationRenewalInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := h.service.RenewSendMessageUsageAuthorization(ctx, authorization)
				cancel()
				if err != nil {
					continue
				}
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}

// recordAndApplySendMessageBilling 统一记录账单并把快照回填到当前响应消息，避免流式和非流式口径分叉。
func (h *Handler) recordAndApplySendMessageBilling(
	ctx context.Context,
	userID uint,
	conversation *model.Conversation,
	req *SendMessageRequest,
	result *appconversation.SendMessageResult,
	authorization *domainbilling.UsageAuthorization,
) error {
	return h.recordAndApplyUsageBilling(
		ctx,
		sendMessageBillingInput(userID, conversation, req, result),
		result,
		authorization,
	)
}

func (h *Handler) recordAndApplyUsageBilling(
	ctx context.Context,
	billingInput appconversation.SendMessageBillingInput,
	result *appconversation.SendMessageResult,
	authorization *domainbilling.UsageAuthorization,
) error {
	usageLedger, err := h.service.RecordSendMessageBilling(ctx, billingInput, authorization)
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
	if errors.Is(err, billing.ErrUsageConcurrencyLimitExceeded) {
		response.Error(c, http.StatusTooManyRequests, "usage concurrency limit exceeded")
		return
	}
	if errors.Is(err, billing.ErrUsageReservationConflict) {
		response.Error(c, http.StatusConflict, "usage reservation already exists")
		return
	}
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

func handleUsageAuthorizationError(c *gin.Context, err error) {
	if errors.Is(err, billing.ErrUsageConcurrencyLimitExceeded) {
		response.Error(c, http.StatusTooManyRequests, "usage concurrency limit exceeded")
		return
	}
	if errors.Is(err, billing.ErrUsageReservationConflict) {
		response.Error(c, http.StatusConflict, "usage reservation already exists")
		return
	}
	if errors.Is(err, billing.ErrUsageBalanceInsufficient) {
		response.Error(c, http.StatusPaymentRequired, "usage balance is insufficient")
		return
	}
	if errors.Is(err, billing.ErrModelPricingRequired) {
		response.Error(c, http.StatusPaymentRequired, "model pricing is required")
		return
	}
	response.Error(c, http.StatusInternalServerError, "usage balance reservation failed")
}

func mapBillingStreamError(err error) streamError {
	status := http.StatusInternalServerError
	message := "record billing failed"
	if errors.Is(err, billing.ErrUsageConcurrencyLimitExceeded) {
		status = http.StatusTooManyRequests
		message = "usage concurrency limit exceeded"
	}
	if errors.Is(err, billing.ErrUsageReservationConflict) {
		status = http.StatusConflict
		message = "usage reservation already exists"
	}
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
	case errors.Is(err, appconversation.ErrFileNotFound):
		response.Error(c, http.StatusNotFound, "file not found")
	case errors.Is(err, appconversation.ErrFileTooLarge):
		response.Error(c, http.StatusRequestEntityTooLarge, "file too large")
	case errors.Is(err, appconversation.ErrTooManyMessageFiles):
		response.Error(c, http.StatusBadRequest, "too many files in one message")
	case errors.Is(err, appconversation.ErrTooManySelectedTools):
		response.Error(c, http.StatusBadRequest, "too many selected tools")
	case errors.Is(err, appconversation.ErrMultipleImageAttachmentProcessors):
		response.Error(c, http.StatusBadRequest, "multiple image attachment processors selected")
	case errors.Is(err, appconversation.ErrImageAttachmentProcessingFailed):
		response.Error(c, http.StatusBadGateway, "image attachment processing failed")
	case errors.Is(err, appconversation.ErrTooManySelectedSkills):
		response.Error(c, http.StatusBadRequest, "too many selected skills")
	case errors.Is(err, appconversation.ErrSkillNotFound):
		response.Error(c, http.StatusNotFound, "skill not found")
	case errors.Is(err, appconversation.ErrInvalidSkillUse):
		response.Error(c, http.StatusBadRequest, "invalid skill use")
	case errors.Is(err, appconversation.ErrInvalidMessageBranch):
		response.Error(c, http.StatusBadRequest, "invalid message branch")
	case errors.Is(err, appconversation.ErrFileProcessingNotReady):
		response.Error(c, http.StatusBadRequest, "file processing not ready")
	case errors.Is(err, appconversation.ErrFileTooLargeForFullContext):
		response.Error(c, http.StatusBadRequest, "file too large for full context")
	case errors.Is(err, appconversation.ErrEmbeddingUnavailable):
		response.Error(c, http.StatusBadRequest, "embedding unavailable for current file capability")
	case errors.Is(err, appconversation.ErrInvalidKnowledgeBaseReference):
		response.ErrorWithCode(c, http.StatusBadRequest, appconversation.MessageErrorCodeKnowledgeBaseInvalidReference, "invalid knowledge base reference")
	case errors.Is(err, appconversation.ErrKnowledgeBaseUnavailable):
		response.ErrorWithCode(c, http.StatusServiceUnavailable, appconversation.MessageErrorCodeKnowledgeBaseUnavailable, "knowledge base retrieval is unavailable")
	case errors.Is(err, appconversation.ErrKnowledgeBaseNotReady):
		response.ErrorWithCode(c, http.StatusConflict, appconversation.MessageErrorCodeKnowledgeBaseNotReady, "selected knowledge base has no ready files")
	case errors.Is(err, appconversation.ErrModelRouteNotConfigured):
		response.Error(c, http.StatusServiceUnavailable, "model route not configured")
	case errors.Is(err, appconversation.ErrGeneratedMediaArtifactUnavailable):
		response.ErrorWithCode(c, http.StatusBadGateway, appconversation.MessageErrorCode(err), "generated media artifact is temporarily unavailable")
	case errors.Is(err, appconversation.ErrUpstreamEmptyResponse):
		response.Error(c, http.StatusBadGateway, "model returned empty response")
	case appconversation.IsUpstreamRateLimitError(err):
		response.ErrorWithCode(c, http.StatusTooManyRequests, appconversation.MessageErrorCodeUpstreamRateLimited, "upstream rate limited")
	case errors.Is(err, appconversation.ErrUpstreamRequestFailed):
		if code := appconversation.MessageErrorCode(err); code != "" {
			response.ErrorWithCode(c, http.StatusBadGateway, code, mapClientErrorMessage(err))
			return
		}
		response.Error(c, http.StatusBadGateway, mapClientErrorMessage(err))
	case errors.Is(err, appconversation.ErrMessageParentDeleted):
		response.ErrorWithCode(c, http.StatusConflict, "conversation.message_parent_deleted", "the message being replied to was deleted, please retry")
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
	authorization, err := h.authorizeMessageUsage(c, input, sendMessageBillingInput(middleware.MustUserID(c), conversation, req, nil))
	if err != nil {
		return
	}
	stopAuthorizationRenewal := h.startUsageAuthorizationRenewal(authorization)
	defer stopAuthorizationRenewal()

	result, err := h.service.SendMessage(c.Request.Context(), input)
	if err != nil {
		if result != nil {
			if !result.Billable {
				if releaseErr := h.releaseSendMessageUsageAuthorization(authorization); releaseErr != nil {
					handleSendMessageBillingError(c, releaseErr)
					return
				}
				handleSendMessageError(c, err)
				return
			}
			if billingErr := h.recordAndApplySendMessageBilling(c.Request.Context(), middleware.MustUserID(c), conversation, req, result, authorization); billingErr != nil {
				handleSendMessageBillingError(c, billingErr)
				return
			}
			h.recordSendMessageAudit(c, conversation, req, result, "send_message")
			response.Success(c, toSendMessageResponse(result))
			return
		}
		if releaseErr := h.releaseSendMessageUsageAuthorization(authorization); releaseErr != nil {
			handleSendMessageBillingError(c, releaseErr)
			return
		}
		handleSendMessageError(c, err)
		return
	}

	if err := h.recordAndApplySendMessageBilling(c.Request.Context(), middleware.MustUserID(c), conversation, req, result, authorization); err != nil {
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
	authorization, err := h.authorizeMessageUsage(c, input, sendMessageBillingInput(middleware.MustUserID(c), conversation, req, nil))
	if err != nil {
		return
	}
	stopAuthorizationRenewal := h.startUsageAuthorizationRenewal(authorization)
	defer stopAuthorizationRenewal()

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

	// 将中间事件（含 moderation_*）通过 NDJSON 推送给客户端。
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
	if err == nil && result != nil && result.IsModerationBlocked() {
		// Guarantee a terminal event even if live OnEvent path missed emit.
		if !result.ModerationTerminalEmitted() {
			_ = flushStreamEvent(moderationBlockedStreamPayload(result))
		}
		if result.Billable {
			billingCtx, billingCancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = h.recordAndApplySendMessageBilling(billingCtx, middleware.MustUserID(c), conversation, req, result, authorization)
			billingCancel()
		} else {
			_ = h.releaseSendMessageUsageAuthorization(authorization)
		}
		h.service.FinishMessageGeneration(input.ClientRunID)
		h.recordStreamSendMessageAuditAsync(c, conversation, req, result, "stream_message")
		return
	}
	if err != nil {
		if result != nil {
			if !result.Billable {
				if releaseErr := h.releaseSendMessageUsageAuthorization(authorization); releaseErr != nil {
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
			billingErr := h.recordAndApplySendMessageBilling(billingCtx, middleware.MustUserID(c), conversation, req, result, authorization)
			billingCancel()
			if billingErr != nil {
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
		if releaseErr := h.releaseSendMessageUsageAuthorization(authorization); releaseErr != nil {
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
	billingErr := h.recordAndApplySendMessageBilling(billingCtx, middleware.MustUserID(c), conversation, req, result, authorization)
	billingCancel()
	if billingErr != nil {
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

// StreamActiveMessageGenerations godoc
// @Summary Stream active conversation generations
// @Description Sends an authoritative snapshot followed by live user-scoped run state events; the snapshot is re-sent periodically for client-side reconciliation
// @Tags chat
// @Produce text/event-stream
// @Security BearerAuth
// @Success 200 {object} ActiveMessageGenerationEventResponse
// @Failure 500 {object} ErrorDoc
// @Router /conversation-runs/stream [get]
func (h *Handler) StreamActiveMessageGenerations(c *gin.Context) {
	userID := middleware.MustUserID(c)
	snapshot, events, unsubscribe, err := h.service.SubscribeActiveMessageGenerations(
		c.Request.Context(),
		userID,
	)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to subscribe to active conversation generations")
		return
	}
	defer unsubscribe()

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	writeEvent := func(payload ActiveMessageGenerationEventResponse) bool {
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return true
		}
		if _, writeErr := c.Writer.Write([]byte("data: ")); writeErr != nil {
			return false
		}
		if _, writeErr := c.Writer.Write(encoded); writeErr != nil {
			return false
		}
		if _, writeErr := c.Writer.Write([]byte("\n\n")); writeErr != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}
	writeSnapshot := func(items []appconversation.ActiveMessageGeneration) bool {
		runs := make([]ActiveMessageGenerationResponse, 0, len(items))
		for _, item := range items {
			runs = append(runs, ActiveMessageGenerationResponse{
				RunID:                item.RunID,
				ConversationPublicID: item.ConversationPublicID,
			})
		}
		return writeEvent(ActiveMessageGenerationEventResponse{Type: "snapshot", Runs: runs})
	}

	if !writeSnapshot(snapshot) {
		return
	}

	// 周期重发权威快照兼作心跳：增量事件在断线间隙丢失时，客户端可在一个周期内对账清除失效运行。
	snapshotTicker := time.NewTicker(20 * time.Second)
	defer snapshotTicker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-h.shutdown.Done():
			// 关停排空：订阅流立即退出，客户端按既有退避逻辑重连。
			return
		case <-snapshotTicker.C:
			latest, listErr := h.service.ListActiveMessageGenerations(c.Request.Context(), userID)
			if listErr != nil {
				// 快照查询失败时退回注释帧，仅维持连接心跳。
				if _, writeErr := c.Writer.Write([]byte(": keepalive\n\n")); writeErr != nil {
					return
				}
				c.Writer.Flush()
				continue
			}
			if !writeSnapshot(latest) {
				return
			}
		case event, ok := <-events:
			if !ok {
				return
			}
			if !writeEvent(ActiveMessageGenerationEventResponse{
				Type:                 event.Type,
				RunID:                event.RunID,
				ConversationPublicID: event.ConversationPublicID,
			}) {
				return
			}
		}
	}
}

// ResumeMessageGenerationStream godoc
// @Summary 恢复流式生成订阅
// @Description 页面刷新后按 run_id 重新订阅仍在运行的生成流，返回 NDJSON 事件
// @Tags chat
// @Produce application/x-ndjson
// @Security BearerAuth
// @Param run_id path string true "运行 ID"
// @Param after query int false "已接收的最后事件序号"
// @Param snapshot query bool false "是否返回正文与当前思考轮次的权威内容快照"
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
	includeSnapshots, _ := strconv.ParseBool(strings.TrimSpace(c.Query("snapshot")))
	replay, events, unsubscribe, ok := h.service.SubscribeMessageGeneration(
		c.Request.Context(),
		userID,
		runID,
		afterSeq,
		includeSnapshots,
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
		return eventType == "completed" || eventType == "error" || eventType == "moderation_blocked"
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
		case <-h.shutdown.Done():
			// 关停排空：观看流立即退出，生成本体不受影响；客户端重连后经 Redis 重放续传。
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
