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

	item, err := h.service.CreateConversation(c.Request.Context(), userID, req.Title, req.Model, req.ProjectID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationProjectNotFound) {
			response.Error(c, http.StatusNotFound, "conversation project not found")
			return
		}
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
// @Param project query string false "项目筛选: all|unassigned|项目 public_id"
// @Param q query string false "搜索关键词，匹配会话元数据、项目名称和消息正文"
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
	projectFilter := normalizeConversationProjectQuery(c.Query("project"))
	searchQuery := c.Query("q")

	items, total, err := h.service.ListConversations(c.Request.Context(), userID, page, pageSize, statusFilter, starredFilter, shareFilter, projectFilter, searchQuery)
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

// GetConversationDefaultModelCandidate godoc
// @Summary 查询新会话默认模型候选
// @Description 返回后台配置的系统推荐模型，未配置时回退当前用户最近一次真实运行使用的模型
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ConversationDefaultModelCandidateResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/default-model-candidate [get]
func (h *Handler) GetConversationDefaultModelCandidate(c *gin.Context) {
	userID := middleware.MustUserID(c)

	if systemDefaultModel := h.service.GetConversationSystemDefaultModel(); systemDefaultModel != "" {
		response.Success(c, ConversationDefaultModelCandidateResponse{
			PlatformModelName: systemDefaultModel,
			Source:            "system_default",
		})
		return
	}

	run, err := h.service.GetLatestConversationRunModel(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "load default model candidate failed")
		return
	}
	if run == nil {
		response.Success(c, ConversationDefaultModelCandidateResponse{})
		return
	}

	usedAt := run.EndedAt
	if usedAt == nil {
		usedAt = &run.StartedAt
	}
	response.Success(c, ConversationDefaultModelCandidateResponse{
		PlatformModelName: run.PlatformModelName,
		Source:            "latest_run",
		UsedAt:            usedAt,
	})
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

// ExportConversation godoc
// @Summary 导出会话 JSON
// @Description 导出当前用户单个会话的元信息、消息、运行日志和可见处理轨迹
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Success 200 {object} ConversationExportResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/export [get]
func (h *Handler) ExportConversation(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	item, err := h.service.ExportConversation(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "export conversation failed")
		return
	}

	h.recordAudit(c, "export_conversation",
		"conversation",
		publicID,
		map[string]interface{}{"message_count": item.TotalMessages},
	)

	response.Success(c, toConversationExportResponse(item))
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

// RegenerateConversationTitle godoc
// @Summary 自动重新命名会话
// @Description 根据指定会话已有内容重新生成标题
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Success 200 {object} ConversationUpdateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/title/regenerate [post]
func (h *Handler) RegenerateConversationTitle(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	item, err := h.service.RegenerateConversationTitle(c.Request.Context(), userID, publicID)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidConversationTitle):
			response.Error(c, http.StatusBadRequest, "conversation has no titleable content")
			return
		case errors.Is(err, appconversation.ErrConversationNotFound):
			response.Error(c, http.StatusNotFound, "conversation not found")
			return
		default:
			response.Error(c, http.StatusInternalServerError, "regenerate conversation title failed")
			return
		}
	}

	h.recordAudit(c, "regenerate_conversation_title",
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
// @Param delete_files query bool false "是否同步删除不再被其他会话引用的会话文件"
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
	deleteFiles := c.Query("delete_files") == "true"

	result, err := h.service.DeleteConversation(c.Request.Context(), userID, publicID, appconversation.DeleteConversationOptions{
		DeleteFiles: deleteFiles,
	})
	if err != nil {
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
		map[string]interface{}{
			"deleted":            true,
			"delete_files":       deleteFiles,
			"deleted_file_count": result.DeletedFileCount,
		},
	)

	response.Success(c, toConversationDeleteResponse(result))
}
