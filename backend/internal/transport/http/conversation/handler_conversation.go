package conversation

import (
	"errors"
	"net/http"
	"strconv"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// CreateConversation godoc
// @Summary 创建会话
// @Description 创建新的聊天会话
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateConversationRequest true "会话参数"
// @Success 200 {object} ConversationCreateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations [post]
// CreateConversation 创建会话。
func (h *Handler) CreateConversation(c *gin.Context) {
	userID := middleware.MustUserID(c)

	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.CreateConversation(c.Request.Context(), userID, req.Title, req.Model)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "create conversation failed")
		return
	}

	h.recordAudit(c, "create_conversation",
		"conversation",
		strconv.FormatUint(uint64(item.ID), 10),
		map[string]string{"title": item.Title},
	)

	response.Success(c, toConversationResponse(item))
}

// ListConversations godoc
// @Summary 会话分页列表
// @Description 查询当前用户会话列表
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param status query string false "状态筛选: active|archived|all"
// @Param starred query string false "星标筛选: all|starred|unstarred"
// @Param share query string false "分享筛选: all|shared|unshared"
// @Success 200 {object} ConversationListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations [get]
// ListConversations 查询会话。
func (h *Handler) ListConversations(c *gin.Context) {
	userID := middleware.MustUserID(c)
	page, pageSize := pageParams(c)
	statusFilter := normalizeConversationStatusFilter(c.Query("status"))
	starredFilter := normalizeConversationStarredFilter(c.Query("starred"))
	shareFilter := normalizeConversationShareFilter(c.Query("share"))

	items, total, err := h.service.ListConversations(c.Request.Context(), userID, page, pageSize, statusFilter, starredFilter, shareFilter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list conversations failed")
		return
	}
	results := make([]ConversationResponse, 0, len(items))
	for i := range items {
		results = append(results, toConversationResponse(&items[i]))
	}
	response.SuccessPage(c, total, results)
}

// GetConversation godoc
// @Summary 查询会话
// @Description 查询当前用户的单个会话元信息
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Success 200 {object} ConversationUpdateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id} [get]
func (h *Handler) GetConversation(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	item, err := h.service.GetConversationByPublicID(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "get conversation failed")
		return
	}

	response.Success(c, toConversationResponse(item))
}

// RenameConversation godoc
// @Summary 重命名会话
// @Description 修改指定会话标题
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param body body RenameConversationRequest true "重命名参数"
// @Success 200 {object} ConversationUpdateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/title [patch]
func (h *Handler) RenameConversation(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var req RenameConversationRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.RenameConversation(c.Request.Context(), userID, publicID, req.Title)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidConversationTitle):
			response.Error(c, http.StatusBadRequest, "invalid conversation title")
			return
		case errors.Is(err, appconversation.ErrConversationNotFound):
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		default:
			response.Error(c, http.StatusInternalServerError, "rename conversation failed")
			return
		}
	}

	h.recordAudit(c, "rename_conversation",
		"conversation",
		item.PublicID,
		map[string]string{"title": item.Title},
	)

	response.Success(c, toConversationResponse(item))
}

// SetConversationStar godoc
// @Summary 设置会话星标
// @Description 设置指定会话是否星标
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param body body SetConversationStarRequest true "星标参数"
// @Success 200 {object} ConversationUpdateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/star [patch]
func (h *Handler) SetConversationStar(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var req SetConversationStarRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.SetConversationStar(c.Request.Context(), userID, publicID, req.Starred)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "update conversation star failed")
		return
	}

	h.recordAudit(c, "set_conversation_star",
		"conversation",
		item.PublicID,
		map[string]bool{"starred": item.IsStarred},
	)

	response.Success(c, toConversationResponse(item))
}

// SetConversationArchive godoc
// @Summary 设置会话归档
// @Description 设置指定会话归档状态
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param body body SetConversationArchiveRequest true "归档参数"
// @Success 200 {object} ConversationUpdateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/archive [patch]
func (h *Handler) SetConversationArchive(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	var req SetConversationArchiveRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.SetConversationArchived(c.Request.Context(), userID, publicID, req.Archived)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "update conversation archive failed")
		return
	}

	h.recordAudit(c, "set_conversation_archive",
		"conversation",
		item.PublicID,
		map[string]string{"status": item.Status},
	)

	response.Success(c, toConversationResponse(item))
}

// DeleteConversation godoc
// @Summary 删除会话
// @Description 删除指定会话
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Success 200 {object} ConversationDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id} [delete]
func (h *Handler) DeleteConversation(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	if err = h.service.DeleteConversation(c.Request.Context(), userID, publicID); err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "delete conversation failed")
		return
	}

	h.recordAudit(c, "delete_conversation",
		"conversation",
		publicID,
		map[string]bool{"deleted": true},
	)

	response.Success(c, ConversationDeleteResponse{Deleted: true})
}
