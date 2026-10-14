package conversation

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// SetMessageUIState godoc
// @Summary 保存消息内交互式组件的状态
// @Description 按块保存 assistant 消息里交互式组件的当前状态快照（只保存与块初始值不同的键）。blocks 以块 id 为键：出现的块整块替换，值为 null 或空对象表示清除，没出现的块保持不变
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param message_id path string true "消息 public_id"
// @Param body body SetMessageUIStateRequest true "组件状态"
// @Success 200 {object} MessageUIStateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 401 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/messages/{message_id}/ui-state [put]
func (h *Handler) SetMessageUIState(c *gin.Context) {
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

	var req SetMessageUIStateRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	result, err := h.service.SetMessageUIState(c.Request.Context(), userID, conversationID, messageID, toMessageUIStatePatches(req.Blocks))
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidMessageUIState):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		case errors.Is(err, appconversation.ErrMessageUIStateTargetInvalid):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		case errors.Is(err, appconversation.ErrConversationNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
		case errors.Is(err, appconversation.ErrMessageNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
		default:
			response.InternalError(c, err)
		}
		return
	}

	response.Success(c, MessageUIStateResponse{
		MessageID:       result.MessageID,
		MessagePublicID: result.MessagePublicID,
		UIState:         result.Blocks,
	})
}

// toMessageUIStatePatches 把按块 id 组织的请求体转成应用层的块变更列表。
func toMessageUIStatePatches(blocks map[string]json.RawMessage) []appconversation.MessageUIStateBlockPatch {
	patches := make([]appconversation.MessageUIStateBlockPatch, 0, len(blocks))
	for blockID, raw := range blocks {
		// null 表示清除该块：应用层只接受 JSON 对象，这里先把 null 归一为空串。
		state := strings.TrimSpace(string(raw))
		if state == "null" {
			state = ""
		}
		patches = append(patches, appconversation.MessageUIStateBlockPatch{
			BlockID:   blockID,
			StateJSON: state,
		})
	}
	return patches
}

// toMessageUIStateInputs 把发送请求携带的组件状态转成应用层输入；合法性由应用层判定并静默丢弃。
func toMessageUIStateInputs(items []MessageUIStateInputRequest) []appconversation.MessageUIStateInput {
	if len(items) == 0 {
		return nil
	}
	inputs := make([]appconversation.MessageUIStateInput, 0, len(items))
	for _, item := range items {
		inputs = append(inputs, appconversation.MessageUIStateInput{
			MessagePublicID: item.MessagePublicID,
			BlockID:         item.BlockID,
			Title:           item.Title,
			StateJSON:       string(item.State),
		})
	}
	return inputs
}

// toTemporaryChatUIStateInputs 是临时对话版本，按请求 messages 下标定位组件所在消息。
func toTemporaryChatUIStateInputs(items []TemporaryChatUIStateRequest) []appconversation.TemporaryChatUIStateInput {
	if len(items) == 0 {
		return nil
	}
	inputs := make([]appconversation.TemporaryChatUIStateInput, 0, len(items))
	for _, item := range items {
		inputs = append(inputs, appconversation.TemporaryChatUIStateInput{
			MessageIndex: item.MessageIndex,
			BlockID:      item.BlockID,
			Title:        item.Title,
			StateJSON:    string(item.State),
		})
	}
	return inputs
}
