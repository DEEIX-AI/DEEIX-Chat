package conversation

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// UpdateMessage godoc
// @Summary 更新消息内容
// @Description 更新当前用户会话中的 assistant 消息内容，并标记为已编辑
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "消息 public_id"
// @Param body body UpdateMessageRequest true "消息内容"
// @Success 200 {object} MessageResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /messages/{id} [patch]
func (h *Handler) UpdateMessage(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid message id")
		return
	}

	var req UpdateMessageRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.UpdateAssistantMessageContent(c.Request.Context(), userID, publicID, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidMessageContent):
			response.Error(c, http.StatusBadRequest, "invalid message content")
			return
		case errors.Is(err, appconversation.ErrMessageEditTargetInvalid):
			response.Error(c, http.StatusBadRequest, "message edit target invalid")
			return
		case errors.Is(err, appconversation.ErrMessageEditStateInvalid):
			response.Error(c, http.StatusBadRequest, "message edit state invalid")
			return
		case errors.Is(err, appconversation.ErrMessageNotFound):
			response.Error(c, http.StatusNotFound, "message not found")
			return
		default:
			response.Error(c, http.StatusInternalServerError, "update message failed")
			return
		}
	}

	h.recordAudit(c, "update_message",
		"message",
		item.PublicID,
		map[string]interface{}{
			"role": item.Role,
		},
	)

	run := model.Run{}
	runID := strings.TrimSpace(item.RunID)
	if runID != "" {
		runs, runErr := h.service.ListConversationRunsByRunIDs(c.Request.Context(), userID, item.ConversationID, []string{runID})
		if runErr == nil && len(runs) > 0 {
			run = runs[0]
		}
	}
	response.Success(c, toMessageResponseWithRun(*item, run))
}

// SetMessageFeedback godoc
// @Summary 设置消息反馈
// @Description 对 assistant 消息设置点赞/点踩，传空 feedback 表示取消反馈
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "消息 public_id"
// @Param body body SetMessageFeedbackRequest true "反馈参数"
// @Success 200 {object} MessageFeedbackResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /messages/{id}/feedback [put]
func (h *Handler) SetMessageFeedback(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid message id")
		return
	}

	var req SetMessageFeedbackRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	result, err := h.service.SetMessageFeedback(c.Request.Context(), userID, publicID, req.Feedback)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidMessageFeedback):
			response.Error(c, http.StatusBadRequest, "invalid message feedback")
			return
		case errors.Is(err, appconversation.ErrMessageFeedbackTargetInvalid):
			response.Error(c, http.StatusBadRequest, "message feedback target invalid")
			return
		case errors.Is(err, appconversation.ErrMessageNotFound):
			response.Error(c, http.StatusNotFound, "message not found")
			return
		default:
			response.Error(c, http.StatusInternalServerError, "set message feedback failed")
			return
		}
	}

	h.recordAudit(c, "set_message_feedback",
		"message",
		result.MessagePublicID,
		map[string]interface{}{
			"feedback": req.Feedback,
		},
	)

	response.Success(c, toMessageFeedbackResponse(result))
}

// ListMessages godoc
// @Summary 查询会话消息
// @Description 查询会话内消息列表
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} MessageListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/messages [get]
// ListMessages 查询消息。
func (h *Handler) ListMessages(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	page, pageSize := messagePageParams(c)
	var beforeID uint
	if rawBeforeID := strings.TrimSpace(c.Query("before_id")); rawBeforeID != "" {
		parsed, parseErr := strconv.ParseUint(rawBeforeID, 10, 64)
		if parseErr != nil || parsed == 0 {
			response.Error(c, http.StatusBadRequest, "invalid before message id")
			return
		}
		beforeID = uint(parsed)
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

	var items []model.Message
	var total int64
	if beforeID > 0 {
		items, total, err = h.service.ListMessagesBeforeID(c.Request.Context(), userID, conversation.ID, beforeID, pageSize)
	} else if c.Query("tail") == "true" {
		items, total, err = h.service.ListRecentMessages(c.Request.Context(), userID, conversation.ID, pageSize)
	} else {
		items, total, err = h.service.ListMessages(c.Request.Context(), userID, conversation.ID, page, pageSize)
	}
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "list messages failed")
		return
	}
	runModels := map[string]model.Run{}
	runIDs := collectMessageRunIDs(items)
	if len(runIDs) > 0 {
		runs, runErr := h.service.ListConversationRunsByRunIDs(c.Request.Context(), userID, conversation.ID, runIDs)
		if runErr != nil {
			if errors.Is(runErr, appconversation.ErrConversationNotFound) {
				response.Error(c, http.StatusNotFound, "conversation not found")
				return
			}
			response.Error(c, http.StatusInternalServerError, "list conversation runs failed")
			return
		}
		for _, run := range runs {
			if runID := strings.TrimSpace(run.RunID); runID != "" {
				runModels[runID] = run
			}
		}
	}
	msgResults := make([]MessageResponse, 0, len(items))
	for _, m := range items {
		msgResults = append(msgResults, toMessageResponseWithRunAndFallback(m, runModels[strings.TrimSpace(m.RunID)], conversation.Model))
	}
	response.SuccessPage(c, total, msgResults)
}

// collectMessageRunIDs 提取消息列表中的运行 ID，并保持首次出现顺序。
func collectMessageRunIDs(items []model.Message) []string {
	seen := make(map[string]struct{}, len(items))
	runIDs := make([]string, 0, len(items))
	for _, item := range items {
		runID := strings.TrimSpace(item.RunID)
		if runID == "" {
			continue
		}
		if _, ok := seen[runID]; ok {
			continue
		}
		seen[runID] = struct{}{}
		runIDs = append(runIDs, runID)
	}
	return runIDs
}

// ListConversationRuns godoc
// @Summary 查询会话运行日志
// @Description 查询会话内模型调用运行日志（tokens/时长/错误）
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} ConversationRunListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/runs [get]
// ListConversationRuns 查询运行日志。
func (h *Handler) ListConversationRuns(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	page, pageSize := pageParams(c)
	conversation, err := h.service.GetConversationByPublicID(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "load conversation failed")
		return
	}

	items, total, err := h.service.ListConversationRuns(c.Request.Context(), userID, conversation.ID, page, pageSize)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "list conversation runs failed")
		return
	}
	runResults := make([]RunResponse, 0, len(items))
	for _, r := range items {
		runResults = append(runResults, toRunResponse(r))
	}
	response.SuccessPage(c, total, runResults)
}
