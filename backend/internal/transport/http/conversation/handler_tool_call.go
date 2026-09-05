package conversation

import (
	"errors"
	"net/http"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// GetConversationToolCallDetail godoc
// @Summary 查询工具调用结果详情
// @Description 查询当前用户指定会话运行内的持久化工具调用结果；超限字段仅返回原始大小与省略标记
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param run_id path string true "运行 ID"
// @Param tool_call_id path string true "工具调用 ID"
// @Success 200 {object} ConversationToolCallDetailResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversation-runs/{run_id}/tool-calls/{tool_call_id} [get]
func (h *Handler) GetConversationToolCallDetail(c *gin.Context) {
	runID, err := stringParam(c, "run_id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidRunID)
		return
	}
	toolCallID, err := stringParam(c, "tool_call_id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidToolCallID)
		return
	}

	item, err := h.service.GetConversationToolCallDetail(
		c.Request.Context(),
		middleware.MustUserID(c),
		runID,
		toolCallID,
	)
	if err != nil {
		if errors.Is(err, appconversation.ErrToolCallNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
		return
	}
	response.Success(c, toConversationToolCallDetailResponse(item))
}
