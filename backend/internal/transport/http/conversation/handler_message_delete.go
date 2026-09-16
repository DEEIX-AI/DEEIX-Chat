package conversation

import (
	"errors"
	"net/http"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// DeleteMessage godoc
// @Summary 删除指定消息
// @Description 删除会话中任意位置的一条消息；其子消息将重接到被删消息的父消息上，后续消息保留并向前衔接。会话第一条消息与生成中的消息不允许删除
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param message_id path string true "消息 public_id"
// @Success 200 {object} MessageDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 401 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/messages/{message_id} [delete]
func (h *Handler) DeleteMessage(c *gin.Context) {
	userID := middleware.MustUserID(c)
	conversationID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}
	messageID, err := stringParam(c, "message_id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidMessageID)
		return
	}

	result, err := h.service.DeleteMessage(c.Request.Context(), userID, conversationID, messageID)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrConversationNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
		case errors.Is(err, appconversation.ErrMessageNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
		case errors.Is(err, appconversation.ErrMessageDeleteStateInvalid):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		case errors.Is(err, appconversation.ErrMessageDeleteTargetInvalid):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		case errors.Is(err, appconversation.ErrMessageDeleteRootInvalid):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		default:
			response.InternalError(c)
		}
		return
	}

	h.recordAudit(c, "delete_message",
		"message",
		messageID,
		map[string]any{
			"conversation_id":  conversationID,
			"reparented_count": result.ReparentedMessageCount,
		},
	)

	response.Success(c, MessageDeleteResponse{
		Deleted:                true,
		ReparentedMessageCount: result.ReparentedMessageCount,
	})
}
