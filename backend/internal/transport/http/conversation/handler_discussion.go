package conversation

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DiscussionSendRequest 是讨论消息发送请求。
type DiscussionSendRequest struct {
	ContentType           string            `json:"contentType" binding:"required,oneof=text markdown"`
	Content               string            `json:"content" binding:"required"`
	TemplatePublicID      string            `json:"templatePublicID" binding:"required,max=32"`
	Models                map[string]string `json:"models,omitempty"`
	ClientRunID           string            `json:"clientRunID,omitempty" binding:"omitempty,max=64"`
	ParentMessagePublicID string            `json:"parentMessagePublicID,omitempty" binding:"omitempty,max=32"`
	SourceMessagePublicID string            `json:"sourceMessagePublicID,omitempty" binding:"omitempty,max=32"`
	BranchReason          string            `json:"branchReason,omitempty" binding:"omitempty,oneof=default retry edit"`
}

// StreamDiscussionMessage godoc
// @Summary 流式发送多模型讨论消息
// @Description 按后台讨论模板编排多模型轮次，并流式返回讨论事件与最终总结
// @Tags chat
// @Accept json
// @Produce application/x-ndjson
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param body body DiscussionSendRequest true "讨论参数"
// @Success 200 {string} string "NDJSON stream"
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/messages/discussion/stream [post]
func (h *Handler) StreamDiscussionMessage(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID := strings.TrimSpace(c.Param("id"))
	var req DiscussionSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	conversation, err := h.service.GetConversationByPublicID(c.Request.Context(), userID, publicID)
	if err != nil {
		handleSendMessageError(c, err)
		return
	}
	runID := strings.TrimSpace(req.ClientRunID)
	if runID == "" {
		runID = "run_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	generationCtx, releaseLifecycle, ok := h.service.AcquireMessageGenerationLifecycle(
		background.Detach(c.Request.Context()),
	)
	if !ok {
		response.ErrorWithCode(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable)
		return
	}
	defer releaseLifecycle()
	if generationCtx == nil {
		generationCtx = background.Detach(c.Request.Context())
	}
	defer h.service.FinishMessageGeneration(generationCtx, runID)
	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	writeEvent := func(payload map[string]any) error {
		if published, owned := h.service.PublishMessageGenerationEvent(generationCtx, runID, payload); owned {
			payload = published
		}
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return marshalErr
		}
		if _, writeErr := c.Writer.Write(append(encoded, '\n')); writeErr != nil {
			return writeErr
		}
		c.Writer.Flush()
		return nil
	}
	input := appconversation.DiscussionSendInput{
		UserID:                userID,
		ConversationID:        conversation.ID,
		RequestID:             middleware.MustRequestID(c),
		ContentType:           req.ContentType,
		Content:               req.Content,
		TemplatePublicID:      req.TemplatePublicID,
		ModelBindings:         req.Models,
		ParentMessagePublicID: req.ParentMessagePublicID,
		SourceMessagePublicID: req.SourceMessagePublicID,
		BranchReason:          req.BranchReason,
		ClientRunID:           runID,
		OnEvent: func(eventType string, payload map[string]any) error {
			return writeEvent(normalizeStreamEventPayload(eventType, payload))
		},
	}
	result, err := h.service.StreamDiscussionMessage(generationCtx, input)
	if err != nil {
		if errors.Is(err, appconversation.ErrMessageGenerationCanceled) {
			return
		}
		_ = writeEvent(streamErrorPayload(c, err))
		return
	}
	_ = writeEvent(map[string]any{
		"type": "completed",
		"data": toSendMessageResponse(result, middleware.MustRequestID(c)),
	})
}
