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

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
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

func sanitizeMessageOptions(options map[string]any) map[string]any {
	if len(options) == 0 {
		return nil
	}
	sanitized := make(map[string]any, len(options))
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
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
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
			response.ErrorFrom(c, http.StatusNotFound, err)
			return appconversation.SendMessageInput{}, nil, nil, err
		}
		response.InternalError(c)
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

// beginUsageSession 在写入响应头前预留预算并启动续租；失败时已写出 HTTP 错误响应。
func (h *Handler) beginUsageSession(c *gin.Context, input appconversation.SendMessageBillingInput) (*appconversation.UsageSession, bool) {
	session, err := h.service.BeginUsageSession(c.Request.Context(), input)
	if err != nil {
		handleSendMessageError(c, err)
		return nil, false
	}
	return session, true
}

// beginMessageUsageSession 在预留失败时先把终态业务拒绝持久化为会话消息，再写出 HTTP 错误响应。
func (h *Handler) beginMessageUsageSession(
	c *gin.Context,
	input appconversation.SendMessageInput,
	billingInput appconversation.SendMessageBillingInput,
) (*appconversation.UsageSession, bool) {
	session, err := h.service.BeginUsageSession(c.Request.Context(), billingInput)
	if err == nil {
		return session, true
	}
	if persistErr := h.service.PersistMessageUsageRejection(c.Request.Context(), input, err); persistErr != nil {
		handleSendMessageError(c, persistErr)
		return nil, false
	}
	handleSendMessageError(c, err)
	return nil, false
}

// billableResult 判断运行是否产生了需要结算的上游用量。
func billableResult(result *appconversation.SendMessageResult) bool {
	return result != nil && result.Billable
}

// recordSendMessageAudit 记录审计日志（同步，供非流式路径使用）。
func (h *Handler) recordSendMessageAudit(c *gin.Context, conversation *model.Conversation, req *SendMessageRequest, result *appconversation.SendMessageResult, action string) {
	h.recordSendMessageAuditCtx(c.Request.Context(), appconversation.SendMessageAuditInput{
		UserID:         middleware.MustUserID(c),
		RequestID:      middleware.MustRequestID(c),
		ClientIP:       c.ClientIP(),
		UserAgent:      c.Request.UserAgent(),
		Action:         action,
		ContentType:    req.ContentType,
		ConversationID: conversation.ID,
		FileIDs:        req.FileIDs,
		Result:         result,
	})
}

// recordStreamSendMessageAuditAsync 在 Handler 返回前提取 gin.Context 值，goroutine 内不持有 gin.Context。
func (h *Handler) recordStreamSendMessageAuditAsync(
	c *gin.Context,
	conversation *model.Conversation,
	req *SendMessageRequest,
	result *appconversation.SendMessageResult,
	action string,
) {
	requestCtx := c.Request.Context()
	bgUserID := middleware.MustUserID(c)
	bgRequestID := middleware.MustRequestID(c)
	bgClientIP := c.ClientIP()
	bgUserAgent := c.Request.UserAgent()
	go func() {
		auditCtx, cancel := background.WithTimeout(requestCtx, asyncAuditTimeout)
		defer cancel()
		h.recordSendMessageAuditCtx(auditCtx, appconversation.SendMessageAuditInput{
			UserID:         bgUserID,
			RequestID:      bgRequestID,
			ClientIP:       bgClientIP,
			UserAgent:      bgUserAgent,
			Action:         action,
			ContentType:    req.ContentType,
			ConversationID: conversation.ID,
			FileIDs:        req.FileIDs,
			Result:         result,
		})
	}()
}

// recordSendMessageAuditCtx 接受结构化审计输入，可在 goroutine 中安全调用（不依赖 gin.Context）。
func (h *Handler) recordSendMessageAuditCtx(ctx context.Context, input appconversation.SendMessageAuditInput) {
	h.service.RecordSendMessageAudit(ctx, input)
}

// handleSendMessageError 把消息发送 / 生成 / 计费路径上的错误写成 HTTP 错误响应，映射规则见 describeSendMessageError。
func handleSendMessageError(c *gin.Context, err error) {
	response.ErrorDescribed(c, describeSendMessageError(err))
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
	session, ok := h.beginMessageUsageSession(c, input, buildBillingInput(billingRequestInput{
		UserID:            middleware.MustUserID(c),
		Conversation:      conversation,
		PlatformModelName: req.Model,
		ClientRunID:       req.ClientRunID,
	}))
	if !ok {
		return
	}
	defer session.Close()
	input.UsageAuthorization = session.Authorization()

	result, err := h.service.SendMessage(c.Request.Context(), input)
	if billingErr := session.Finish(c.Request.Context(), result); billingErr != nil {
		handleSendMessageError(c, billingErr)
		return
	}
	// 已产生可计费用量的失败运行仍以持久化结果返回，错误状态由消息本身承载。
	if err != nil && !billableResult(result) {
		handleSendMessageError(c, err)
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
	session, ok := h.beginMessageUsageSession(c, input, buildBillingInput(billingRequestInput{
		UserID:            middleware.MustUserID(c),
		Conversation:      conversation,
		PlatformModelName: req.Model,
		ClientRunID:       req.ClientRunID,
	}))
	if !ok {
		return
	}
	defer session.Close()
	input.UsageAuthorization = session.Authorization()
	generationCtx, releaseLifecycle, ok := h.service.AcquireMessageGenerationLifecycle(
		background.Detach(c.Request.Context()),
	)
	if !ok {
		_ = session.Finish(c.Request.Context(), nil)
		response.ErrorWithCode(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable)
		return
	}
	defer releaseLifecycle()

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
		payload, owned := h.service.PublishMessageGenerationEvent(generationCtx, input.ClientRunID, payload)
		if !owned {
			return false, nil
		}
		return true, writeStreamEvent(payload)
	}

	// 将中间事件（含 moderation_*）通过 NDJSON 推送给客户端。
	input.OnEvent = func(eventType string, payload map[string]any) error {
		owned, flushErr := flushStreamEvent(normalizeStreamEventPayload(eventType, payload))
		if !owned {
			return appconversation.ErrMessageGenerationInterrupted
		}
		return flushErr
	}

	defer h.service.FinishMessageGeneration(generationCtx, input.ClientRunID)
	result, err := h.service.StreamMessage(generationCtx, input, func(delta string) error {
		owned, flushErr := flushStreamEvent(map[string]any{
			"type":  "delta",
			"delta": delta,
		})
		if !owned {
			return appconversation.ErrMessageGenerationInterrupted
		}
		return flushErr
	})

	if err == nil && result != nil && result.IsModerationBlocked() {
		// Guarantee a terminal event even if live OnEvent path missed emit.
		if !result.ModerationTerminalEmitted() {
			_, _ = flushStreamEvent(moderationBlockedStreamPayload(result, session.Authorization()))
		}
		// 终态事件已发出，结算/释放失败由应用层记日志并标记对账，不能再向流推送第二个终态事件。
		_ = session.Finish(c.Request.Context(), result)
		h.recordStreamSendMessageAuditAsync(c, conversation, req, result, "stream_message")
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
		if result != nil {
			h.recordStreamSendMessageAuditAsync(c, conversation, req, result, "stream_message")
		}
		return
	}
	_, _ = flushStreamEvent(map[string]any{
		"type": "completed",
		"data": toSendMessageResponse(result),
	})
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
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidRunID)
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
		response.InternalError(c)
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
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidRunID)
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
		response.ErrorFrom(c, http.StatusNotFound, errGenerationStreamNotFound)
		return
	}
	defer unsubscribe()

	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	isTerminal := func(payload map[string]any) bool {
		eventType, _ := payload["type"].(string)
		return eventType == "completed" || eventType == "error" || eventType == "moderation_blocked"
	}
	terminalWritten := false
	writeEvent := func(payload map[string]any) bool {
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
		_ = writeEvent(streamErrorPayload(appconversation.ErrMessageGenerationInterrupted))
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
				_ = writeEvent(streamErrorPayload(appconversation.ErrMessageGenerationInterrupted))
				return
			}
		case event, ok := <-events:
			if !ok {
				if !terminalWritten && !isActive() {
					h.service.MarkMessageGenerationInterrupted(c.Request.Context(), userID, runID)
					_ = writeEvent(streamErrorPayload(appconversation.ErrMessageGenerationInterrupted))
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
