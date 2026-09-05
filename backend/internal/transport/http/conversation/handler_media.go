package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
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
	h.streamMediaVideo(c, appconversation.MediaVideoTaskGeneration)
}

// StreamVideoExtension 处理会话内视频扩展流式状态接口。
// @Summary 扩展会话视频
// @Tags Conversations
// @Accept json
// @Produce application/x-ndjson
// @Param id path string true "会话 Public ID"
// @Param payload body MediaVideoExtensionRequest true "视频扩展请求"
// @Success 200 {string} string "NDJSON stream"
// @Failure 400 {object} response.Envelope
// @Failure 401 {object} response.Envelope
// @Failure 404 {object} response.Envelope
// @Router /conversations/{id}/media/videos/extensions/stream [post]
func (h *Handler) StreamVideoExtension(c *gin.Context) {
	h.streamMediaVideo(c, appconversation.MediaVideoTaskExtension)
}

type mediaVideoTransportRequest struct {
	Prompt                string
	Model                 string
	Options               map[string]any
	ClientRunID           string
	FileIDs               []string
	ParentMessagePublicID string
	SourceMessagePublicID string
	BranchReason          string
}

// streamMediaVideo 统一视频生成与扩展的 HTTP、授权和事件转发流程。
func (h *Handler) streamMediaVideo(c *gin.Context, taskType appconversation.MediaVideoTaskType) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}
	conversation, err := h.service.GetConversationByPublicID(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
		return
	}
	var req mediaVideoTransportRequest
	if taskType == appconversation.MediaVideoTaskExtension {
		var payload MediaVideoExtensionRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			response.InvalidRequestBody(c, err)
			return
		}
		req = mediaVideoTransportRequest{
			Prompt:                payload.Prompt,
			Model:                 payload.Model,
			Options:               payload.Options,
			ClientRunID:           payload.ClientRunID,
			FileIDs:               []string{payload.SourceVideoFileID},
			ParentMessagePublicID: payload.ParentMessagePublicID,
			SourceMessagePublicID: payload.SourceMessagePublicID,
			BranchReason:          payload.BranchReason,
		}
	} else {
		var payload MediaVideoRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			response.InvalidRequestBody(c, err)
			return
		}
		req = mediaVideoTransportRequest(payload)
	}
	req.ClientRunID = appconversation.EnsureMessageGenerationRunID(req.ClientRunID)
	req.Options = sanitizeMessageOptions(req.Options)
	session, ok := h.beginUsageSession(c, buildBillingInput(billingRequestInput{
		UserID:            userID,
		Conversation:      conversation,
		PlatformModelName: req.Model,
		ClientRunID:       req.ClientRunID,
	}))
	if !ok {
		return
	}
	defer session.Close()
	generationCtx, releaseLifecycle, ok := h.service.AcquireMessageGenerationLifecycle(
		background.Detach(c.Request.Context()),
	)
	if !ok {
		_ = session.Finish(c.Request.Context(), nil)
		response.ErrorWithCode(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable)
		return
	}
	defer releaseLifecycle()

	h.streamMediaTask(
		c,
		generationCtx,
		req.ClientRunID,
		session,
		func(onEvent func(string, map[string]any) error) (*appconversation.SendMessageResult, error) {
			return h.service.StreamMediaVideo(generationCtx, appconversation.MediaVideoInput{
				UserID:                userID,
				ConversationID:        conversation.ID,
				RequestID:             middleware.MustRequestID(c),
				TaskType:              taskType,
				Prompt:                req.Prompt,
				PlatformModelName:     req.Model,
				Options:               req.Options,
				ClientRunID:           req.ClientRunID,
				FileIDs:               req.FileIDs,
				ParentMessagePublicID: req.ParentMessagePublicID,
				SourceMessagePublicID: req.SourceMessagePublicID,
				BranchReason:          req.BranchReason,
				UsageAuthorization:    session.Authorization(),
				OnEvent:               onEvent,
			})
		},
	)
}

// streamMediaImage 只负责 HTTP 绑定、计费预算预留和 NDJSON 事件转发，图片业务由 application 执行。
func (h *Handler) streamMediaImage(c *gin.Context, taskType appconversation.MediaImageTaskType) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}
	conversation, err := h.service.GetConversationByPublicID(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
		return
	}
	var req MediaImageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	req.ClientRunID = appconversation.EnsureMessageGenerationRunID(req.ClientRunID)
	req.Options = sanitizeMessageOptions(req.Options)
	session, ok := h.beginUsageSession(c, buildBillingInput(billingRequestInput{
		UserID:            userID,
		Conversation:      conversation,
		PlatformModelName: req.Model,
		ClientRunID:       req.ClientRunID,
	}))
	if !ok {
		return
	}
	defer session.Close()
	generationCtx, releaseLifecycle, ok := h.service.AcquireMessageGenerationLifecycle(
		background.Detach(c.Request.Context()),
	)
	if !ok {
		_ = session.Finish(c.Request.Context(), nil)
		response.ErrorWithCode(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable)
		return
	}
	defer releaseLifecycle()

	h.streamMediaTask(
		c,
		generationCtx,
		req.ClientRunID,
		session,
		func(onEvent func(string, map[string]any) error) (*appconversation.SendMessageResult, error) {
			return h.service.StreamMediaImage(generationCtx, appconversation.MediaImageInput{
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
				UsageAuthorization:    session.Authorization(),
				OnEvent:               onEvent,
			})
		},
	)
}

// streamMediaTask 统一媒体任务的 NDJSON 事件转发与计费收口：运行结束后由 session 结算或释放预算。
func (h *Handler) streamMediaTask(
	c *gin.Context,
	generationCtx context.Context,
	clientRunID string,
	session *appconversation.UsageSession,
	run func(onEvent func(string, map[string]any) error) (*appconversation.SendMessageResult, error),
) {
	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	var clientDisconnected atomic.Bool
	writeStreamEvent := func(payload map[string]any) error {
		if clientDisconnected.Load() {
			return nil
		}
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return marshalErr
		}
		if _, writeErr := c.Writer.Write(append(encoded, '\n')); writeErr != nil {
			clientDisconnected.Store(true)
			return nil
		}
		c.Writer.Flush()
		return nil
	}
	flushStreamEvent := func(payload map[string]any) (bool, error) {
		payload, owned := h.service.PublishMessageGenerationEvent(generationCtx, clientRunID, payload)
		if !owned {
			return false, nil
		}
		return true, writeStreamEvent(payload)
	}

	defer h.service.FinishMessageGeneration(generationCtx, clientRunID)
	result, err := run(func(eventType string, payload map[string]any) error {
		owned, flushErr := flushStreamEvent(normalizeStreamEventPayload(eventType, payload))
		if !owned {
			return appconversation.ErrMessageGenerationInterrupted
		}
		return flushErr
	})

	if err == nil && result != nil && result.IsModerationBlocked() {
		if !result.ModerationTerminalEmitted() {
			_, _ = flushStreamEvent(moderationBlockedStreamPayload(result, session.Authorization()))
		}
		// 终态事件已发出，结算/释放失败由应用层记日志并标记对账，不能再向流推送第二个终态事件。
		_ = session.Finish(c.Request.Context(), result)
		return
	}
	if billingErr := session.Finish(c.Request.Context(), result); billingErr != nil {
		payload := streamErrorPayloadWithResult(billingErr, result)
		if owned, _ := flushStreamEvent(payload); !owned {
			_ = writeStreamEvent(payload)
		}
		return
	}
	if err != nil {
		payload := streamErrorPayloadWithResult(err, result)
		if owned, _ := flushStreamEvent(payload); !owned {
			_ = writeStreamEvent(payload)
		}
		return
	}
	if result == nil {
		return
	}
	if result.AssistantMessage.Status == "canceled" {
		_, _ = flushStreamEvent(streamErrorPayloadWithResult(appconversation.ErrMessageGenerationCanceled, result))
		return
	}
	_, _ = flushStreamEvent(map[string]any{
		"type": "completed",
		"data": toSendMessageResponse(result),
	})
}
