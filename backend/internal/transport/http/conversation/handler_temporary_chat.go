package conversation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const temporaryChatMaxRequestBytes = 8 << 20

// StreamTemporaryChatMessage godoc
// @Summary 流式发送临时对话消息
// @Description 由浏览器提交完整上下文和可选请求级附件；服务端不创建会话、消息、运行、文件或断线续传记录
// @Tags chat
// @Accept json,multipart/form-data
// @Produce application/x-ndjson
// @Security BearerAuth
// @Param body body TemporaryChatMessageRequest true "临时对话参数"
// @Success 200 {string} string "NDJSON stream"
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /temporary-chat/messages/stream [post]
func (h *Handler) StreamTemporaryChatMessage(c *gin.Context) {
	req, attachments, closeAttachments, ok := h.bindTemporaryChatRequest(c)
	if !ok {
		return
	}
	defer closeAttachments()
	req.Options = sanitizeMessageOptions(req.Options)
	input := appconversation.TemporaryChatInput{
		UserID:                   middleware.MustUserID(c),
		RequestID:                middleware.MustRequestID(c),
		SessionID:                strings.TrimSpace(req.SessionID),
		ClientRunID:              strings.TrimSpace(req.ClientRunID),
		Model:                    strings.TrimSpace(req.Model),
		Options:                  req.Options,
		SelectedToolIDs:          append([]uint(nil), req.SelectedToolIDs...),
		SkillIDs:                 append([]uint(nil), req.SkillIDs...),
		KnowledgeBaseIDs:         append([]string(nil), req.KnowledgeBaseIDs...),
		HTMLVisualPromptEnabled:  req.HTMLVisualPrompt,
		Messages:                 make([]appconversation.TemporaryChatMessage, 0, len(req.Messages)),
		Attachments:              attachments,
		ReleaseAttachmentSources: closeAttachments,
	}
	for _, item := range req.Messages {
		input.Messages = append(input.Messages, appconversation.TemporaryChatMessage{
			Role:    strings.TrimSpace(item.Role),
			Content: item.Content,
		})
	}
	if err := appconversation.ValidateTemporaryChatInput(input); err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidTemporaryChatMessages)
		return
	}

	session, ok := h.beginUsageSession(c, appconversation.SendMessageBillingInput{
		UserID:            input.UserID,
		PlatformModelName: input.Model,
		ClientRunID:       input.ClientRunID,
	})
	if !ok {
		return
	}
	defer session.Close()
	input.UsageAuthorization = session.Authorization()
	generationCtx, releaseLifecycle, ok := h.service.AcquireMessageGenerationLifecycle(c.Request.Context())
	if !ok {
		_ = session.Finish(c.Request.Context(), nil)
		response.ErrorWithCode(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable)
		return
	}
	defer releaseLifecycle()

	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-store, no-cache, no-transform")
	c.Header("Pragma", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	writeEvent := func(payload map[string]any) error {
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
	input.OnEvent = func(eventType string, payload map[string]any) error {
		return writeEvent(normalizeStreamEventPayload(eventType, payload))
	}

	result, streamErr := h.service.StreamTemporaryChat(generationCtx, input, func(delta string) error {
		return writeEvent(map[string]any{"type": "delta", "delta": delta})
	})
	clientConnected := func() bool { return c.Request.Context().Err() == nil }

	billingErr := session.Finish(c.Request.Context(), result)
	if billingErr != nil && clientConnected() {
		_ = writeEvent(streamErrorPayload(billingErr))
	}
	if streamErr == nil && billingErr != nil {
		h.recordTemporaryChatAuditAsync(c, req, len(input.Attachments), "billing_failed")
		return
	}
	if result != nil && result.IsModerationBlocked() {
		if !result.ModerationTerminalEmitted() && clientConnected() {
			_ = writeEvent(moderationBlockedStreamPayload(result, session.Authorization()))
		}
		h.recordTemporaryChatAuditAsync(c, req, len(input.Attachments), "blocked")
		return
	}
	if streamErr != nil {
		if clientConnected() {
			_ = writeEvent(streamErrorPayload(streamErr))
		}
		h.recordTemporaryChatAuditAsync(c, req, len(input.Attachments), "failed")
		return
	}
	_ = writeEvent(map[string]any{
		"type": "completed",
		"data": toSendMessageResponse(result),
	})
	h.recordTemporaryChatAuditAsync(c, req, len(input.Attachments), "completed")
}

func (h *Handler) bindTemporaryChatRequest(c *gin.Context) (
	TemporaryChatMessageRequest,
	[]appconversation.TemporaryChatAttachment,
	func(),
	bool,
) {
	noop := func() {}
	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, temporaryChatMaxRequestBytes)
		var req TemporaryChatMessageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			writeTemporaryChatBindError(c, err)
			return TemporaryChatMessageRequest{}, nil, noop, false
		}
		return req, nil, noop, true
	}

	policy, err := h.service.GetChatFilePolicy(c.Request.Context(), middleware.MustUserID(c))
	if err != nil {
		response.InternalError(c)
		return TemporaryChatMessageRequest{}, nil, noop, false
	}
	maxUploadBytes := policy.MaxUploadFileBytes
	if maxUploadBytes <= 0 {
		maxUploadBytes = 20 * 1024 * 1024
	}
	requestLimit := int64(temporaryChatMaxRequestBytes)
	if maxUploadBytes > (math.MaxInt64-requestLimit)/appconversation.TemporaryChatMaxAttachments {
		requestLimit = math.MaxInt64
	} else {
		requestLimit += maxUploadBytes * appconversation.TemporaryChatMaxAttachments
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, requestLimit)
	if err = c.Request.ParseMultipartForm(1 << 20); err != nil {
		writeTemporaryChatBindError(c, err)
		return TemporaryChatMessageRequest{}, nil, noop, false
	}
	removeMultipartFiles := func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}

	var req TemporaryChatMessageRequest
	if err = json.Unmarshal([]byte(c.PostForm("payload")), &req); err != nil {
		removeMultipartFiles()
		response.InvalidRequestBody(c, err)
		return TemporaryChatMessageRequest{}, nil, noop, false
	}
	if err = binding.Validator.ValidateStruct(req); err != nil {
		removeMultipartFiles()
		response.InvalidRequestBody(c, err)
		return TemporaryChatMessageRequest{}, nil, noop, false
	}
	var messageIndexes []int
	if err = json.Unmarshal([]byte(c.PostForm("attachmentMessageIndexes")), &messageIndexes); err != nil {
		removeMultipartFiles()
		response.InvalidRequestBody(c, err)
		return TemporaryChatMessageRequest{}, nil, noop, false
	}
	fileHeaders := c.Request.MultipartForm.File["attachments"]
	if len(fileHeaders) == 0 || len(fileHeaders) != len(messageIndexes) || len(fileHeaders) > appconversation.TemporaryChatMaxAttachments {
		removeMultipartFiles()
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileReference)
		return TemporaryChatMessageRequest{}, nil, noop, false
	}
	maxFilesPerMessage := policy.MaxMessageFiles
	if maxFilesPerMessage <= 0 {
		maxFilesPerMessage = 10
	}
	counts := make(map[int]int)
	opened := make([]multipart.File, 0, len(fileHeaders))
	var closeOnce sync.Once
	closeAll := func() {
		closeOnce.Do(func() {
			for _, file := range opened {
				_ = file.Close()
			}
			removeMultipartFiles()
		})
	}
	attachments := make([]appconversation.TemporaryChatAttachment, 0, len(fileHeaders))
	for index, header := range fileHeaders {
		messageIndex := messageIndexes[index]
		if messageIndex < 0 || messageIndex >= len(req.Messages) || strings.TrimSpace(req.Messages[messageIndex].Role) != "user" {
			closeAll()
			response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileReference)
			return TemporaryChatMessageRequest{}, nil, noop, false
		}
		counts[messageIndex]++
		if counts[messageIndex] > maxFilesPerMessage {
			closeAll()
			response.ErrorFrom(c, http.StatusBadRequest, errTooManyFilesInOneMessage)
			return TemporaryChatMessageRequest{}, nil, noop, false
		}
		file, openErr := header.Open()
		if openErr != nil {
			closeAll()
			response.ErrorFrom(c, http.StatusBadRequest, errInvalidFileReference)
			return TemporaryChatMessageRequest{}, nil, noop, false
		}
		opened = append(opened, file)
		attachments = append(attachments, appconversation.TemporaryChatAttachment{
			MessageIndex: messageIndex,
			FileName:     header.Filename,
			MimeType:     header.Header.Get("Content-Type"),
			DeclaredSize: header.Size,
			Reader:       file,
		})
	}
	return req, attachments, closeAll, true
}

func writeTemporaryChatBindError(c *gin.Context, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) || errors.Is(err, multipart.ErrMessageTooLarge) {
		response.ErrorFrom(c, http.StatusRequestEntityTooLarge, errTemporaryChatContextTooLarge)
		return
	}
	response.InvalidRequestBody(c, err)
}

func (h *Handler) recordTemporaryChatAuditAsync(c *gin.Context, req TemporaryChatMessageRequest, attachmentCount int, status string) {
	requestCtx := c.Request.Context()
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
	go func() {
		auditCtx, cancel := background.WithTimeout(requestCtx, asyncAuditTimeout)
		defer cancel()
		h.service.RecordAudit(auditCtx, appconversation.AuditInput{
			ActorUserID: userID,
			RequestID:   requestID,
			Action:      "temporary_chat.stream_message",
			Resource:    "temporary_chat",
			ResourceID:  resourceID,
			IP:          clientIP,
			UserAgent:   userAgent,
			Detail: map[string]any{
				"status":               strings.TrimSpace(status),
				"message_count":        messageCount,
				"character_count":      characterCount,
				"selected_tool_count":  len(req.SelectedToolIDs),
				"selected_skill_count": len(req.SkillIDs),
				"knowledge_base_count": len(req.KnowledgeBaseIDs),
				"attachment_count":     attachmentCount,
				"content_stored":       false,
			},
		})
	}()
}

func temporaryChatSessionHash(sessionID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(sessionID)))
	return hex.EncodeToString(digest[:16])
}
