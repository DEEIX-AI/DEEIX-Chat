package conversation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

const temporaryChatMaxRequestBytes = 8 << 20

// StreamTemporaryChatMessage godoc
// @Summary 流式发送临时对话消息
// @Description 由浏览器提交完整纯文本上下文；服务端不创建会话、消息、运行或断线续传记录
// @Tags chat
// @Accept json
// @Produce application/x-ndjson
// @Security BearerAuth
// @Param body body TemporaryChatMessageRequest true "临时对话参数"
// @Success 200 {string} string "NDJSON stream"
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /temporary-chat/messages/stream [post]
func (h *Handler) StreamTemporaryChatMessage(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, temporaryChatMaxRequestBytes)
	var req TemporaryChatMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			response.Error(c, http.StatusRequestEntityTooLarge, "temporary chat context is too large")
			return
		}
		response.InvalidRequestBody(c, err)
		return
	}
	req.Options = sanitizeMessageOptions(req.Options)
	input := appconversation.TemporaryChatInput{
		UserID:                  middleware.MustUserID(c),
		RequestID:               middleware.MustRequestID(c),
		SessionID:               strings.TrimSpace(req.SessionID),
		ClientRunID:             strings.TrimSpace(req.ClientRunID),
		Model:                   strings.TrimSpace(req.Model),
		Options:                 req.Options,
		SelectedToolIDs:         append([]uint(nil), req.SelectedToolIDs...),
		SkillIDs:                append([]uint(nil), req.SkillIDs...),
		KnowledgeBaseIDs:        append([]string(nil), req.KnowledgeBaseIDs...),
		HTMLVisualPromptEnabled: req.HTMLVisualPrompt,
		Messages:                make([]appconversation.TemporaryChatMessage, 0, len(req.Messages)),
	}
	for _, item := range req.Messages {
		input.Messages = append(input.Messages, appconversation.TemporaryChatMessage{
			Role:    strings.TrimSpace(item.Role),
			Content: item.Content,
		})
	}
	if err := appconversation.ValidateTemporaryChatInput(input); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid temporary chat messages")
		return
	}

	billingInput := appconversation.SendMessageBillingInput{
		UserID:            input.UserID,
		PlatformModelName: input.Model,
		ClientRunID:       input.ClientRunID,
	}
	authorization, err := h.authorizeUsage(c, billingInput)
	if err != nil {
		return
	}
	stopAuthorizationRenewal := h.startUsageAuthorizationRenewal(authorization)
	defer stopAuthorizationRenewal()

	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-store, no-cache, no-transform")
	c.Header("Pragma", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	writeEvent := func(payload map[string]interface{}) error {
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
	input.OnEvent = func(eventType string, payload map[string]interface{}) error {
		return writeEvent(normalizeStreamEventPayload(eventType, payload))
	}

	result, streamErr := h.service.StreamTemporaryChat(c.Request.Context(), input, func(delta string) error {
		return writeEvent(map[string]interface{}{"type": "delta", "delta": delta})
	})
	if streamErr != nil {
		if result != nil && result.Billable {
			billingCtx, billingCancel := context.WithTimeout(context.Background(), 10*time.Second)
			billingInput.Result = result
			billingErr := h.recordAndApplyUsageBilling(billingCtx, billingInput, result, authorization)
			billingCancel()
			if billingErr != nil && c.Request.Context().Err() == nil {
				_ = writeEvent(billingStreamErrorPayload(billingErr))
			}
		} else {
			_ = h.releaseSendMessageUsageAuthorization(authorization)
		}
		if result != nil && result.IsModerationBlocked() {
			if !result.ModerationTerminalEmitted() && c.Request.Context().Err() == nil {
				_ = writeEvent(moderationBlockedStreamPayload(result))
			}
			h.recordTemporaryChatAuditAsync(c, req, "blocked")
			return
		}
		if c.Request.Context().Err() == nil {
			_ = writeEvent(streamErrorPayload(streamErr))
		}
		h.recordTemporaryChatAuditAsync(c, req, "failed")
		return
	}

	billingCtx, billingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	billingInput.Result = result
	billingErr := h.recordAndApplyUsageBilling(billingCtx, billingInput, result, authorization)
	billingCancel()
	if billingErr != nil {
		_ = writeEvent(billingStreamErrorPayload(billingErr))
		h.recordTemporaryChatAuditAsync(c, req, "billing_failed")
		return
	}
	if result.IsModerationBlocked() {
		if !result.ModerationTerminalEmitted() {
			_ = writeEvent(moderationBlockedStreamPayload(result))
		}
		h.recordTemporaryChatAuditAsync(c, req, "blocked")
		return
	}
	_ = writeEvent(map[string]interface{}{
		"type": "completed",
		"data": toSendMessageResponse(result),
	})
	h.recordTemporaryChatAuditAsync(c, req, "completed")
}

func (h *Handler) recordTemporaryChatAuditAsync(c *gin.Context, req TemporaryChatMessageRequest, status string) {
	userID := middleware.MustUserID(c)
	requestID := middleware.MustRequestID(c)
	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()
	resourceID := temporaryChatSessionHash(req.SessionID)
	messageCount := len(req.Messages)
	characterCount := 0
	for _, item := range req.Messages {
		characterCount += len([]rune(item.Content))
	}
	go h.service.RecordAudit(context.Background(), appconversation.AuditInput{
		UserID:     userID,
		RequestID:  requestID,
		Action:     "temporary_chat.stream_message",
		Resource:   "temporary_chat",
		ResourceID: resourceID,
		ClientIP:   clientIP,
		UserAgent:  userAgent,
		Detail: map[string]interface{}{
			"status":               strings.TrimSpace(status),
			"message_count":        messageCount,
			"character_count":      characterCount,
			"selected_tool_count":  len(req.SelectedToolIDs),
			"selected_skill_count": len(req.SkillIDs),
			"knowledge_base_count": len(req.KnowledgeBaseIDs),
			"content_stored":       false,
		},
	})
}

func temporaryChatSessionHash(sessionID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(sessionID)))
	return hex.EncodeToString(digest[:16])
}
