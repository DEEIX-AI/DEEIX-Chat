package conversation

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

const maxConversationSearchQueryRunes = 200

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
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
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
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	statusFilter := normalizeConversationStatusFilter(c.Query("status"))
	starredFilter := normalizeConversationStarredFilter(c.Query("starred"))
	shareFilter := normalizeConversationShareFilter(c.Query("share"))
	projectFilter := c.Query("project")
	searchQuery := c.Query("q")

	items, total, err := h.service.ListConversations(c.Request.Context(), appconversation.ListConversationsInput{
		UserID:        userID,
		Page:          page,
		PageSize:      pageSize,
		StatusFilter:  statusFilter,
		StarredFilter: starredFilter,
		ShareFilter:   shareFilter,
		ProjectFilter: projectFilter,
		SearchQuery:   searchQuery,
	})
	if err != nil {
		response.InternalError(c)
		return
	}
	results := make([]ConversationResponse, 0, len(items))
	for i := range items {
		results = append(results, toConversationResponse(&items[i]))
	}
	response.SuccessPage(c, total, results)
}

// SearchConversations godoc
// @Summary 搜索会话
// @Description 分页搜索当前用户的会话标题、元数据、项目和消息正文，并返回是否还有下一页
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param q query string false "搜索关键词；为空时返回最近会话"
// @Success 200 {object} ConversationSearchListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/search [get]
// SearchConversations 搜索会话。
func (h *Handler) SearchConversations(c *gin.Context) {
	userID := middleware.MustUserID(c)
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	searchQuery := strings.TrimSpace(c.Query("q"))
	if len([]rune(searchQuery)) > maxConversationSearchQueryRunes {
		response.ErrorWithCode(c, http.StatusBadRequest, response.CodeRequestInvalidQuery)
		return
	}
	items, hasMore, err := h.service.SearchConversations(
		c.Request.Context(),
		userID,
		page,
		pageSize,
		searchQuery,
	)
	if err != nil {
		response.InternalError(c)
		return
	}
	results := make([]ConversationSearchResultResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toConversationSearchResultResponse(item))
	}
	response.Success(c, ConversationSearchPageResponse{
		HasMore: hasMore,
		Results: results,
	})
}

// GetConversationDefaultModelCandidate godoc
// @Summary 查询新会话默认模型候选
// @Description 返回后台配置的新会话系统推荐模型；未配置时返回空候选
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ConversationDefaultModelCandidateResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/default-model-candidate [get]
func (h *Handler) GetConversationDefaultModelCandidate(c *gin.Context) {
	if systemDefaultModel := h.service.GetConversationSystemDefaultModel(); systemDefaultModel != "" {
		response.Success(c, ConversationDefaultModelCandidateResponse{
			PlatformModelName: systemDefaultModel,
			Source:            "system_default",
		})
		return
	}

	response.Success(c, ConversationDefaultModelCandidateResponse{})
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
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}

	item, err := h.service.GetConversationByPublicID(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
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
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}

	item, err := h.service.ExportConversation(c.Request.Context(), userID, publicID)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
		return
	}

	h.recordAudit(c, "export_conversation",
		"conversation",
		publicID,
		map[string]any{"message_count": item.TotalMessages},
	)

	response.Success(c, toConversationExportResponse(item))
}

type userExportManifest struct {
	Type      string `json:"_type"`
	Complete  bool   `json:"complete"`
	Exported  int64  `json:"exported"`
	Failed    int    `json:"failed"`
	FailedIDs []uint `json:"failedIDs,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ExportAllConversations godoc
// @Summary 导出当前用户全部对话
// @Description 流式导出当前用户全部会话及消息为 NDJSON 文件
// @Tags chat
// @Produce application/x-ndjson
// @Security BearerAuth
// @Success 200 {string} string "NDJSON stream"
// @Failure 500 {object} ErrorDoc
// @Router /conversations/export [get]
func (h *Handler) ExportAllConversations(c *gin.Context) {
	userID := middleware.MustUserID(c)

	h.recordAudit(c, "export_all_conversations", "conversation", "", map[string]any{"scope": "user"})

	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="my-conversations-%s.jsonl"`, time.Now().UTC().Format("20060102-150405")))
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)

	const batchSize = 50
	var lastID uint
	encoder := json.NewEncoder(c.Writer)
	exported := int64(0)
	var failedIDs []uint
	writeManifest := func(complete bool, exportErr string) {
		_ = encoder.Encode(userExportManifest{
			Type:      "export_manifest",
			Complete:  complete,
			Exported:  exported,
			Failed:    len(failedIDs),
			FailedIDs: failedIDs,
			Error:     exportErr,
		})
		c.Writer.Flush()
	}

	for {
		if c.Request.Context().Err() != nil {
			return
		}
		conversations, err := h.service.ListUserConversationsAfterID(c.Request.Context(), userID, lastID, batchSize)
		if err != nil {
			writeManifest(false, "failed to list conversations")
			return
		}
		if len(conversations) == 0 {
			break
		}
		for i := range conversations {
			result, err := h.service.ExportUserConversationData(c.Request.Context(), userID, &conversations[i])
			if err != nil {
				failedIDs = append(failedIDs, conversations[i].ID)
				continue
			}
			if err := encoder.Encode(ToConversationExportResponse(result)); err != nil {
				return
			}
			exported++
		}
		c.Writer.Flush()
		lastID = conversations[len(conversations)-1].ID
		if len(conversations) < batchSize {
			break
		}
	}

	writeManifest(true, "")
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
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
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
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		case errors.Is(err, appconversation.ErrConversationNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		default:
			response.InternalError(c)
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
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}

	item, err := h.service.RegenerateConversationTitle(c.Request.Context(), userID, publicID)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidConversationTitle):
			response.ErrorFrom(c, http.StatusBadRequest, errConversationNoTitleableContent)
			return
		case errors.Is(err, appconversation.ErrConversationNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		default:
			response.InternalError(c)
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

// UpdateConversationLabels godoc
// @Summary 更新会话标签
// @Description 替换指定会话的标签；传入空数组可清空标签
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param body body UpdateConversationLabelsRequest true "会话标签"
// @Success 200 {object} ConversationUpdateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/labels [patch]
func (h *Handler) UpdateConversationLabels(c *gin.Context) {
	userID := middleware.MustUserID(c)
	publicID, err := stringParam(c, "id")
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}

	var req UpdateConversationLabelsRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	if req.Labels == nil {
		response.ErrorFrom(c, http.StatusBadRequest, errLabelsRequired)
		return
	}

	item, err := h.service.UpdateConversationLabels(c.Request.Context(), userID, publicID, *req.Labels)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrInvalidConversationLabels):
			response.ErrorFrom(c, http.StatusBadRequest, err)
			return
		case errors.Is(err, appconversation.ErrConversationNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		default:
			response.InternalError(c)
			return
		}
	}

	h.recordAudit(c, "update_conversation_labels",
		"conversation",
		item.PublicID,
		map[string]string{"labelsJSON": item.LabelsJSON},
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
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}

	var req SetConversationStarRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.SetConversationStar(c.Request.Context(), userID, publicID, *req.Starred)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
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
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}

	var req SetConversationArchiveRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	item, err := h.service.SetConversationArchived(c.Request.Context(), userID, publicID, *req.Archived)
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
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
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidConversationID)
		return
	}
	deleteFiles := c.Query("delete_files") == "true"

	result, err := h.service.DeleteConversation(c.Request.Context(), userID, publicID, appconversation.DeleteConversationOptions{
		DeleteFiles: deleteFiles,
	})
	if err != nil {
		if errors.Is(err, appconversation.ErrConversationNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
		return
	}

	h.recordAudit(c, "delete_conversation",
		"conversation",
		publicID,
		map[string]any{
			"deleted":            true,
			"delete_files":       deleteFiles,
			"deleted_file_count": result.DeletedFileCount,
		},
	)

	response.Success(c, toConversationDeleteResponse(result))
}

// ForkConversationFromMessage godoc
// @Summary 从指定消息 fork 新会话
// @Description 仅允许从助手消息 fork；将会话从开头到指定助手消息（含）的祖先链复制为一个新会话，保留历史展示轨迹；不携带原会话的运行记录与计费，附件以引用方式复用
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 public_id"
// @Param message_id path string true "消息 public_id"
// @Success 200 {object} ConversationUpdateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 401 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /conversations/{id}/messages/{message_id}/fork [post]
func (h *Handler) ForkConversationFromMessage(c *gin.Context) {
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

	result, err := h.service.ForkConversationFromMessage(c.Request.Context(), userID, conversationID, messageID)
	if err != nil {
		switch {
		case errors.Is(err, appconversation.ErrConversationNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
		case errors.Is(err, appconversation.ErrMessageNotFound):
			response.ErrorFrom(c, http.StatusNotFound, err)
		case errors.Is(err, appconversation.ErrMessageForkStateInvalid):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		case errors.Is(err, appconversation.ErrMessageForkTargetInvalid):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		case errors.Is(err, appconversation.ErrMessageForkHistoryIncomplete):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		default:
			response.InternalError(c)
		}
		return
	}

	h.recordAudit(c, "fork_conversation",
		"conversation",
		result.PublicID,
		map[string]any{
			"source_conversation_id": conversationID,
			"source_message_id":      messageID,
		},
	)

	response.Success(c, toConversationResponse(result))
}
