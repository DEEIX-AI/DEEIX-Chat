package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

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
	authorization, err := h.authorizeUsage(c, mediaVideoBillingInput(userID, conversation, &req, nil))
	if err != nil {
		return
	}
	stopAuthorizationRenewal := h.startUsageAuthorizationRenewal(authorization)
	defer stopAuthorizationRenewal()

	h.streamMediaTask(
		c,
		req.ClientRunID,
		authorization,
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

// streamMediaImage 只负责 HTTP 绑定、计费预算预留和 NDJSON 事件转发，图片业务由 application 执行。
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
	authorization, err := h.authorizeUsage(c, mediaImageBillingInput(userID, conversation, &req, nil))
	if err != nil {
		return
	}
	stopAuthorizationRenewal := h.startUsageAuthorizationRenewal(authorization)
	defer stopAuthorizationRenewal()

	h.streamMediaTask(
		c,
		req.ClientRunID,
		authorization,
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
	authorization *domainbilling.UsageAuthorization,
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
		if result == nil || !result.Billable {
			if releaseErr := h.releaseSendMessageUsageAuthorization(authorization); releaseErr != nil {
				_ = flushStreamEvent(billingStreamErrorPayload(releaseErr))
				h.service.FinishMessageGeneration(clientRunID)
				return
			}
			_ = flushStreamEvent(streamErrorPayload(err))
			h.service.FinishMessageGeneration(clientRunID)
			return
		}

		billingCtx, billingCancel := context.WithTimeout(context.Background(), 10*time.Second)
		usageLedger, billingErr := h.service.RecordSendMessageBilling(billingCtx, billingInput(result), authorization)
		billingCancel()
		if billingErr != nil {
			payload := billingStreamErrorPayload(billingErr)
			payload["data"] = toSendMessageResponse(result)
			_ = flushStreamEvent(payload)
			h.service.FinishMessageGeneration(clientRunID)
			return
		}
		appconversation.ApplyUsageBilling(&result.AssistantMessage, usageLedger)
		payload := streamErrorPayload(err)
		payload["data"] = toSendMessageResponse(result)
		_ = flushStreamEvent(payload)
		h.service.FinishMessageGeneration(clientRunID)
		return
	}

	billingCtx, billingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	usageLedger, billingErr := h.service.RecordSendMessageBilling(
		billingCtx,
		billingInput(result),
		authorization,
	)
	billingCancel()
	if billingErr != nil {
		_ = flushStreamEvent(billingStreamErrorPayload(billingErr))
		h.service.FinishMessageGeneration(clientRunID)
		return
	}
	appconversation.ApplyUsageBilling(&result.AssistantMessage, usageLedger)

	if result.AssistantMessage.Status == "canceled" {
		payload := streamErrorPayload(appconversation.ErrMessageGenerationCanceled)
		payload["data"] = toSendMessageResponse(result)
		_ = flushStreamEvent(payload)
		h.service.FinishMessageGeneration(clientRunID)
		return
	}

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
		input.Conversation = conversation
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
		input.Conversation = conversation
	}
	return input
}
