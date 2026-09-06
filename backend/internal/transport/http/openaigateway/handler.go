package openaigateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var errGatewayDisabled = apperr.New("auth.user_api_keys_disabled", "user API keys are disabled")

// Handler 提供 OpenAI 兼容网关。
type Handler struct {
	conversation *appconversation.Service
	channel      *channel.Service
	cfg          *config.Runtime
}

// NewHandler 创建处理器。
func NewHandler(conversation *appconversation.Service, channelService *channel.Service, cfg *config.Runtime) *Handler {
	return &Handler{conversation: conversation, channel: channelService, cfg: cfg}
}

type chatCompletionsRequest struct {
	Model       string                         `json:"model"`
	Messages    []chatCompletionsMessage       `json:"messages"`
	Stream      bool                           `json:"stream"`
	Temperature *float64                       `json:"temperature"`
	TopP        *float64                       `json:"top_p"`
	MaxTokens   *int                           `json:"max_tokens"`
	Options     map[string]any                 `json:"options"`
}

type chatCompletionsMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// ChatCompletions godoc
// @Summary OpenAI-compatible chat completions
// @Tags openai-gateway
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body chatCompletionsRequest true "OpenAI chat completions body"
// @Success 200 {object} map[string]any
// @Router /v1/chat/completions [post]
func (h *Handler) ChatCompletions(c *gin.Context) {
	if !h.cfg.Snapshot().UserAPIKeysEnabled {
		response.ErrorFrom(c, http.StatusForbidden, errGatewayDisabled)
		return
	}
	var req chatCompletionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "invalid request body")
		return
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		writeOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	messages, err := mapOpenAIMessages(req.Messages)
	if err != nil || len(messages) == 0 {
		writeOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "messages are required")
		return
	}
	options := cloneOptions(req.Options)
	if req.Temperature != nil {
		options["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		options["top_p"] = *req.TopP
	}
	if req.MaxTokens != nil {
		options["max_tokens"] = *req.MaxTokens
	}
	userID := middleware.MustUserID(c)
	clientRunID := "api_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	input := appconversation.TemporaryChatInput{
		UserID:      userID,
		RequestID:   middleware.MustRequestID(c),
		ClientRunID: clientRunID,
		Model:       modelName,
		Options:     options,
		Messages:    messages,
	}
	if err := appconversation.ValidateTemporaryChatInput(input); err != nil {
		writeOpenAIError(c, http.StatusBadRequest, "invalid_request_error", "invalid messages")
		return
	}

	session, err := h.conversation.BeginUsageSession(c.Request.Context(), appconversation.SendMessageBillingInput{
		UserID:            userID,
		PlatformModelName: modelName,
		ClientRunID:       clientRunID,
	})
	if err != nil {
		writeOpenAIErrorFromApp(c, err)
		return
	}
	defer session.Close()
	input.UsageAuthorization = session.Authorization()

	generationCtx, releaseLifecycle, ok := h.conversation.AcquireMessageGenerationLifecycle(c.Request.Context())
	if !ok {
		_ = session.Finish(c.Request.Context(), nil)
		writeOpenAIError(c, http.StatusServiceUnavailable, "server_error", "service unavailable")
		return
	}
	defer releaseLifecycle()

	if req.Stream {
		h.streamChatCompletions(c, generationCtx, session, input, modelName, clientRunID)
		return
	}
	h.syncChatCompletions(c, generationCtx, session, input, modelName, clientRunID)
}

func (h *Handler) syncChatCompletions(
	c *gin.Context,
	generationCtx context.Context,
	session *appconversation.UsageSession,
	input appconversation.TemporaryChatInput,
	modelName string,
	completionID string,
) {
	var builder strings.Builder
	result, streamErr := h.conversation.StreamTemporaryChat(generationCtx, input, func(delta string) error {
		builder.WriteString(delta)
		return nil
	})
	billingErr := session.Finish(c.Request.Context(), result)
	if streamErr != nil {
		writeOpenAIErrorFromApp(c, streamErr)
		return
	}
	if billingErr != nil {
		writeOpenAIErrorFromApp(c, billingErr)
		return
	}
	if result != nil && result.IsModerationBlocked() {
		writeOpenAIError(c, http.StatusBadRequest, "content_filter", "content blocked by moderation")
		return
	}
	content := builder.String()
	if content == "" && result != nil {
		content = result.AssistantMessage.Content
	}
	usage := map[string]any{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
	if result != nil {
		usage = map[string]any{
			"prompt_tokens":     result.AssistantMessage.InputTokens,
			"completion_tokens": result.AssistantMessage.OutputTokens,
			"total_tokens":      result.AssistantMessage.InputTokens + result.AssistantMessage.OutputTokens,
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"id":      completionID,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   modelName,
		"choices": []gin.H{{
			"index":         0,
			"finish_reason": "stop",
			"message": gin.H{
				"role":    "assistant",
				"content": content,
			},
		}},
		"usage": usage,
	})
}

func (h *Handler) streamChatCompletions(
	c *gin.Context,
	generationCtx context.Context,
	session *appconversation.UsageSession,
	input appconversation.TemporaryChatInput,
	modelName string,
	completionID string,
) {
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-store, no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	writeChunk := func(payload any) error {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(c.Writer, "data: "); err != nil {
			return err
		}
		if _, err := c.Writer.Write(encoded); err != nil {
			return err
		}
		if _, err := io.WriteString(c.Writer, "\n\n"); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}

	created := time.Now().Unix()
	_ = writeChunk(gin.H{
		"id":      completionID,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   modelName,
		"choices": []gin.H{{
			"index": 0,
			"delta": gin.H{"role": "assistant"},
		}},
	})

	result, streamErr := h.conversation.StreamTemporaryChat(generationCtx, input, func(delta string) error {
		if delta == "" {
			return nil
		}
		return writeChunk(gin.H{
			"id":      completionID,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   modelName,
			"choices": []gin.H{{
				"index": 0,
				"delta": gin.H{"content": delta},
			}},
		})
	})
	clientConnected := c.Request.Context().Err() == nil
	billingErr := session.Finish(c.Request.Context(), result)
	if streamErr != nil || billingErr != nil || (result != nil && result.IsModerationBlocked()) {
		if clientConnected {
			msg := "generation failed"
			if streamErr != nil {
				msg = streamErr.Error()
			} else if billingErr != nil {
				msg = billingErr.Error()
			} else {
				msg = "content blocked by moderation"
			}
			_ = writeChunk(gin.H{"error": gin.H{"message": msg, "type": "server_error"}})
		}
		return
	}
	_ = writeChunk(gin.H{
		"id":      completionID,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   modelName,
		"choices": []gin.H{{
			"index":         0,
			"delta":         gin.H{},
			"finish_reason": "stop",
		}},
	})
	_, _ = io.WriteString(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()
}

// ListModels godoc
// @Summary OpenAI-compatible model list
// @Tags openai-gateway
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /v1/models [get]
func (h *Handler) ListModels(c *gin.Context) {
	if !h.cfg.Snapshot().UserAPIKeysEnabled {
		response.ErrorFrom(c, http.StatusForbidden, errGatewayDisabled)
		return
	}
	items, err := h.channel.ListActiveModels(c.Request.Context(), middleware.MustUserID(c))
	if err != nil {
		writeOpenAIError(c, http.StatusInternalServerError, "server_error", "failed to list models")
		return
	}
	data := make([]gin.H, 0, len(items))
	now := time.Now().Unix()
	for _, item := range items {
		data = append(data, gin.H{
			"id":       item.PlatformModelName,
			"object":   "model",
			"created":  now,
			"owned_by": item.Vendor,
		})
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

func mapOpenAIMessages(items []chatCompletionsMessage) ([]appconversation.TemporaryChatMessage, error) {
	out := make([]appconversation.TemporaryChatMessage, 0, len(items))
	for _, item := range items {
		role := strings.TrimSpace(item.Role)
		if role == "" {
			return nil, errors.New("empty role")
		}
		content, err := flattenMessageContent(item.Content)
		if err != nil {
			return nil, err
		}
		out = append(out, appconversation.TemporaryChatMessage{Role: role, Content: content})
	}
	return out, nil
}

func flattenMessageContent(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString, nil
	}
	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", err
	}
	var builder strings.Builder
	for _, part := range parts {
		if typ, _ := part["type"].(string); typ == "text" {
			if text, ok := part["text"].(string); ok {
				builder.WriteString(text)
			}
		}
	}
	return builder.String(), nil
}

func cloneOptions(options map[string]any) map[string]any {
	if len(options) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(options))
	for key, value := range options {
		cloned[key] = value
	}
	return cloned
}

func writeOpenAIError(c *gin.Context, status int, errType string, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
			"code":    nil,
		},
	})
}

func writeOpenAIErrorFromApp(c *gin.Context, err error) {
	if err == nil {
		writeOpenAIError(c, http.StatusInternalServerError, "server_error", "unknown error")
		return
	}
	msg := err.Error()
	status := http.StatusBadRequest
	errType := "invalid_request_error"
	var appErr *apperr.Error
	if errors.As(err, &appErr) && appErr != nil {
		msg = appErr.Error()
		code := appErr.Code()
		switch {
		case strings.Contains(code, "billing"), strings.Contains(code, "balance"), strings.Contains(code, "quota"):
			status = http.StatusPaymentRequired
			errType = "insufficient_quota"
		case strings.Contains(code, "forbidden"), strings.Contains(code, "disabled"):
			status = http.StatusForbidden
			errType = "permission_error"
		case strings.Contains(code, "not_found"), strings.Contains(code, "route"):
			status = http.StatusNotFound
			errType = "invalid_request_error"
		}
	}
	writeOpenAIError(c, status, errType, fmt.Sprintf("%v", msg))
}
