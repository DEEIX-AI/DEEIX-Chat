package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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

// StreamImageGeneration 处理会话内图片生成流式状态接口。
func (h *Handler) StreamImageGeneration(c *gin.Context) {
	h.streamMediaImage(c, appconversation.MediaImageTaskGeneration)
}

// StreamImageEdit 处理会话内图片编辑流式状态接口。
func (h *Handler) StreamImageEdit(c *gin.Context) {
	h.streamMediaImage(c, appconversation.MediaImageTaskEdit)
}

// StreamVideoGeneration 处理会话内视频生成流式状态接口。
func (h *Handler) StreamVideoGeneration(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}
	conversation, err := h.service.GetConversationByPublicID(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "load conversation failed")
		return
	}
	var req MediaVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.ClientRunID = appconversation.EnsureMessageGenerationRunID(req.ClientRunID)
	req.Options = sanitizeMessageOptions(req.Options)
	if err = h.ensureMediaVideoBillingModelAccess(c, conversation, &req); err != nil {
		return
	}
	reservation, err := h.reserveMediaVideoUsageBalance(c, conversation, &req)
	if err != nil {
		return
	}

	h.streamMediaTask(
		c,
		req.ClientRunID,
		reservation,
		"视频生成失败退回预扣",
		func(onEvent func(string, map[string]interface{}) error) (*appconversation.SendMessageResult, error) {
			return h.service.StreamMediaVideo(c.Request.Context(), appconversation.MediaVideoInput{
				UserID:                userID,
				ConversationID:        conversation.ID,
				RequestID:             middleware.MustRequestID(c),
				Prompt:                req.Prompt,
				PlatformModelName:     req.Model,
				Options:               req.Options,
				ClientRunID:           req.ClientRunID,
				FileIDs:               req.FileIDs,
				ParentMessagePublicID: req.ParentMessagePublicID,
				SourceMessagePublicID: req.SourceMessagePublicID,
				BranchReason:          req.BranchReason,
				OnEvent:               onEvent,
			})
		},
		func(result *appconversation.SendMessageResult) appconversation.SendMessageBillingInput {
			return mediaVideoBillingInput(userID, conversation, &req, result)
		},
	)
}

// streamMediaImage 只负责 HTTP 绑定、计费预扣和 NDJSON 事件转发，图片业务由 application 执行。
func (h *Handler) streamMediaImage(c *gin.Context, taskType appconversation.MediaImageTaskType) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}
	conversation, err := h.service.GetConversationByPublicID(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "load conversation failed")
		return
	}
	var req MediaImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	req.ClientRunID = appconversation.EnsureMessageGenerationRunID(req.ClientRunID)
	req.Options = sanitizeMessageOptions(req.Options)
	if err = h.ensureMediaImageBillingModelAccess(c, conversation, &req); err != nil {
		return
	}
	reservation, err := h.reserveMediaImageUsageBalance(c, conversation, &req)
	if err != nil {
		return
	}

	h.streamMediaTask(
		c,
		req.ClientRunID,
		reservation,
		"图片生成失败退回预扣",
		func(onEvent func(string, map[string]interface{}) error) (*appconversation.SendMessageResult, error) {
			return h.service.StreamMediaImage(c.Request.Context(), appconversation.MediaImageInput{
				UserID:                userID,
				ConversationID:        conversation.ID,
				RequestID:             middleware.MustRequestID(c),
				TaskType:              taskType,
				Prompt:                req.Prompt,
				PlatformModelName:     req.Model,
				Options:               req.Options,
				ClientRunID:           req.ClientRunID,
				FileIDs:               req.FileIDs,
				MaskFileID:            req.MaskFileID,
				ParentMessagePublicID: req.ParentMessagePublicID,
				SourceMessagePublicID: req.SourceMessagePublicID,
				BranchReason:          req.BranchReason,
				OnEvent:               onEvent,
			})
		},
		func(result *appconversation.SendMessageResult) appconversation.SendMessageBillingInput {
			return mediaImageBillingInput(userID, conversation, &req, result)
		},
	)
}

func (h *Handler) streamMediaTask(
	c *gin.Context,
	clientRunID string,
	reservation *domainbilling.UsageBalanceReservation,
	failureReleaseReason string,
	run func(onEvent func(string, map[string]interface{}) error) (*appconversation.SendMessageResult, error),
	billingInput func(result *appconversation.SendMessageResult) appconversation.SendMessageBillingInput,
) {
	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	var clientDisconnected atomic.Bool
	flushStreamEvent := func(payload map[string]interface{}) error {
		payload = h.service.PublishMessageGenerationEvent(clientRunID, payload)
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

	result, err := run(func(eventType string, payload map[string]interface{}) error {
		_ = flushStreamEvent(normalizeStreamEventPayload(eventType, payload))
		return nil
	})
	if err != nil {
		if releaseErr := h.releaseSendMessageUsageReservation(reservation, failureReleaseReason); releaseErr != nil {
			_ = flushStreamEvent(billingStreamErrorPayload(releaseErr))
			h.service.FinishMessageGeneration(clientRunID)
			return
		}
		_ = flushStreamEvent(streamErrorPayload(err))
		h.service.FinishMessageGeneration(clientRunID)
		return
	}

	billingCtx, billingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	usageLedger, billingErr := h.service.RecordSendMessageBilling(
		billingCtx,
		billingInput(result),
		reservation,
	)
	billingCancel()
	if billingErr != nil {
		if shouldReleaseReservationAfterBillingError(billingErr) {
			_ = h.releaseSendMessageUsageReservation(reservation, "计费失败退回预扣")
		}
		_ = flushStreamEvent(billingStreamErrorPayload(billingErr))
		h.service.FinishMessageGeneration(clientRunID)
		return
	}
	appconversation.ApplyUsageBilling(&result.AssistantMessage, usageLedger)

	_ = flushStreamEvent(map[string]interface{}{
		"type": "completed",
		"data": toSendMessageResponse(result),
	})
	h.service.FinishMessageGeneration(clientRunID)
}

// mediaImageBillingInput 构造媒体任务复用消息计费链路所需的上下文。
func mediaImageBillingInput(
	userID uint,
	conversation *model.Conversation,
	req *MediaImageRequest,
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

// mediaVideoBillingInput 构造视频任务复用消息计费链路所需的上下文。
func mediaVideoBillingInput(
	userID uint,
	conversation *model.Conversation,
	req *MediaVideoRequest,
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

// ensureMediaImageBillingModelAccess 在进入流式响应前校验模型是否可计费使用。
func (h *Handler) ensureMediaImageBillingModelAccess(c *gin.Context, conversation *model.Conversation, req *MediaImageRequest) error {
	if err := h.service.EnsureSendMessageBillingAccess(
		c.Request.Context(),
		mediaImageBillingInput(middleware.MustUserID(c), conversation, req, nil),
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

// ensureMediaVideoBillingModelAccess 在进入流式响应前校验模型是否可计费使用。
func (h *Handler) ensureMediaVideoBillingModelAccess(c *gin.Context, conversation *model.Conversation, req *MediaVideoRequest) error {
	if err := h.service.EnsureSendMessageBillingAccess(
		c.Request.Context(),
		mediaVideoBillingInput(middleware.MustUserID(c), conversation, req, nil),
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

// reserveMediaImageUsageBalance 在上游调用前预扣余额，失败直接返回 HTTP 错误。
func (h *Handler) reserveMediaImageUsageBalance(c *gin.Context, conversation *model.Conversation, req *MediaImageRequest) (*domainbilling.UsageBalanceReservation, error) {
	reservation, err := h.service.ReserveSendMessageUsageBalance(
		c.Request.Context(),
		mediaImageBillingInput(middleware.MustUserID(c), conversation, req, nil),
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

// reserveMediaVideoUsageBalance 在上游调用前预扣余额，失败直接返回 HTTP 错误。
func (h *Handler) reserveMediaVideoUsageBalance(c *gin.Context, conversation *model.Conversation, req *MediaVideoRequest) (*domainbilling.UsageBalanceReservation, error) {
	reservation, err := h.service.ReserveSendMessageUsageBalance(
		c.Request.Context(),
		mediaVideoBillingInput(middleware.MustUserID(c), conversation, req, nil),
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
