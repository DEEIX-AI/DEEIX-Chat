package contentmoderation

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	appadmin "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	appcm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/contentmoderation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type userLabelResolver interface {
	ResolveUserLabels(ctx context.Context, userIDs []uint) map[uint]appadmin.UserLabel
}

// Handler 提供管理端内容审核 API。
type Handler struct {
	service           *appcm.Service
	userLabelResolver userLabelResolver
}

// NewHandler 创建 HTTP 处理器。
func NewHandler(service *appcm.Service) *Handler {
	return &Handler{service: service}
}

// SetUserLabelResolver 为事件列表/详情注入批量用户标签解析。
func (h *Handler) SetUserLabelResolver(resolver userLabelResolver) {
	h.userLabelResolver = resolver
}

func (h *Handler) resolveUserLabels(ctx context.Context, userIDs []uint) map[uint]appadmin.UserLabel {
	if h.userLabelResolver == nil {
		return map[uint]appadmin.UserLabel{}
	}
	return h.userLabelResolver.ResolveUserLabels(ctx, userIDs)
}

func parseOptionalRFC3339(c *gin.Context, key string) (*time.Time, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		response.InvalidQueryParam(c, key)
		return nil, false
	}
	return &parsed, true
}

// GetConfig godoc
// @Summary 查询内容审核配置
// @Description 返回当前内容审核配置与可选审核类别目录
// @Tags admin-content-moderation
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ContentModerationConfigResponseDoc
// @Failure 403 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/content-moderation/config [get]
func (h *Handler) GetConfig(c *gin.Context) {
	cfg, err := h.service.GetConfig(c.Request.Context(), middleware.MustUserRole(c))
	if err != nil {
		writeError(c, err)
		return
	}
	categories := appcm.CategoryCatalog()
	response.Success(c, ContentModerationConfigDataResponse{
		Config: toConfigResponse(cfg),
		Categories: ContentModerationCategoryCatalogResponse{
			Text:  categories["text"],
			Image: categories["image"],
		},
	})
}

// UpdateConfig godoc
// @Summary 更新内容审核配置
// @Description 保存内容审核服务、策略与队列配置；启用时必须提供审核服务配置与策略
// @Tags admin-content-moderation
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ContentModerationUpdateConfigRequest true "内容审核配置"
// @Success 200 {object} ContentModerationConfigUpdateResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/content-moderation/config [put]
func (h *Handler) UpdateConfig(c *gin.Context) {
	var req ContentModerationUpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	cfg, err := h.service.UpdateConfig(c.Request.Context(), middleware.MustUserRole(c), req.toApplicationInput())
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, ContentModerationConfigUpdateDataResponse{Config: toConfigResponse(cfg)})
}

// Probe godoc
// @Summary 探测内容审核服务
// @Description 使用当前配置向审核服务发送探测请求，验证连通性以及文本与图片审核能力
// @Tags admin-content-moderation
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ContentModerationProbeResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/content-moderation/probe [post]
func (h *Handler) Probe(c *gin.Context) {
	result, err := h.service.Probe(c.Request.Context(), middleware.MustUserRole(c))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, toProbeResponse(result))
}

// GetStats godoc
// @Summary 查询内容审核每日统计
// @Description 按天汇总指定时间范围内的内容审核结果
// @Tags admin-content-moderation
// @Produce json
// @Security BearerAuth
// @Param from query string false "起始时间（RFC3339）"
// @Param to query string false "结束时间（RFC3339）"
// @Success 200 {object} ContentModerationStatsResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/content-moderation/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	from, ok := parseOptionalRFC3339(c, "from")
	if !ok {
		return
	}
	to, ok := parseOptionalRFC3339(c, "to")
	if !ok {
		return
	}
	filter := appcm.StatsFilter{From: from, To: to}
	items, err := h.service.GetStats(c.Request.Context(), middleware.MustUserRole(c), filter)
	if err != nil {
		writeError(c, err)
		return
	}
	out := make([]ContentModerationDailyStatResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toDailyStatResponse(item))
	}
	response.Success(c, ContentModerationStatsDataResponse{Items: out})
}

// parseOptionalUserID 解析可选的 userId 查询参数。
// 为空表示不过滤（UserID 0）。非法值返回 400 且 ok=false。
func parseOptionalUserID(c *gin.Context) (uint, bool) {
	raw := strings.TrimSpace(c.Query("user_id"))
	if raw == "" {
		return 0, true
	}

	// 将位宽限制为平台 uint 宽度，避免在 32 位平台上截断。
	parsed, err := strconv.ParseUint(raw, 10, strconv.IntSize)
	if err != nil || parsed == 0 {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidUserid)
		return 0, false
	}

	return uint(parsed), true
}

// ListEvents godoc
// @Summary 分页查询内容审核事件
// @Description 按结果、方向、模态、类别、用户、运行与时间范围筛选审核事件，按事件 ID 倒序返回
// @Tags admin-content-moderation
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码（从 1 开始，默认 1）"
// @Param page_size query int false "每页数量（1-1000，默认 20）"
// @Param query query string false "按事件、用户、运行、模型、结果或摘要精确搜索"
// @Param result query string false "审核结果筛选（hit/failed_open/passed）"
// @Param direction query string false "方向筛选（input/output）"
// @Param modality query string false "模态筛选（text/image）"
// @Param category query string false "类别筛选"
// @Param user_id query int false "用户 ID"
// @Param run_id query string false "运行 ID"
// @Param from query string false "起始时间（RFC3339）"
// @Param to query string false "结束时间（RFC3339）"
// @Success 200 {object} ContentModerationEventListResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/content-moderation/events [get]
func (h *Handler) ListEvents(c *gin.Context) {
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	userID, ok := parseOptionalUserID(c)
	if !ok {
		return
	}
	from, ok := parseOptionalRFC3339(c, "from")
	if !ok {
		return
	}
	to, ok := parseOptionalRFC3339(c, "to")
	if !ok {
		return
	}
	input := appcm.EventListInput{
		Query:     c.Query("query"),
		Direction: c.Query("direction"),
		Modality:  c.Query("modality"),
		Result:    c.Query("result"),
		Category:  c.Query("category"),
		UserID:    userID,
		RunID:     c.Query("run_id"),
		From:      from,
		To:        to,
		Page:      page,
		PageSize:  pageSize,
	}
	items, total, err := h.service.ListEvents(c.Request.Context(), middleware.MustUserRole(c), input)
	if err != nil {
		writeError(c, err)
		return
	}
	userIDs := make([]uint, 0, len(items))
	for _, item := range items {
		userIDs = append(userIDs, item.UserID)
	}
	userLabels := h.resolveUserLabels(c.Request.Context(), userIDs)
	out := make([]ContentModerationEventResponse, 0, len(items))
	for _, item := range items {
		label := userLabels[item.UserID]
		out = append(out, toEventResponse(item, label.Label, label.Username))
	}
	response.Success(c, ContentModerationEventListDataResponse{
		Items:    out,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetEvent godoc
// @Summary 查询内容审核事件详情
// @Description 返回单个审核事件；文本仍在保留期内时一并返回解密后的文本，并记录审计日志
// @Tags admin-content-moderation
// @Produce json
// @Security BearerAuth
// @Param eventID path string true "审核事件 ID"
// @Success 200 {object} ContentModerationEventDetailResponseDoc
// @Failure 403 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/content-moderation/events/{eventID} [get]
func (h *Handler) GetEvent(c *gin.Context) {
	detail, err := h.service.GetEventDetail(
		c.Request.Context(),
		middleware.MustUserRole(c),
		c.Param("eventID"),
	)
	if err != nil {
		writeError(c, err)
		return
	}
	label := appadmin.UserLabel{}
	if detail != nil {
		label = h.resolveUserLabels(c.Request.Context(), []uint{detail.Event.UserID})[detail.Event.UserID]
		h.service.RecordReviewAudit(c.Request.Context(), appcm.ReviewAuditInput{
			ActorUserID: middleware.MustUserID(c),
			RequestID:   middleware.MustRequestID(c),
			Action:      "content_moderation.event.view",
			EventID:     detail.Event.PublicID,
			ClientIP:    c.ClientIP(),
			UserAgent:   c.Request.UserAgent(),
			Detail:      map[string]bool{"retainedTextAvailable": detail.TextAvailable},
		})
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, toEventDetailResponse(detail, label.Label, label.Username))
}

// GetEventImage godoc
// @Summary 读取审核事件的隔离图片
// @Description 返回审核事件中按序号指定的隔离图片原始内容，并记录审计日志
// @Tags admin-content-moderation
// @Produce octet-stream
// @Security BearerAuth
// @Param eventID path string true "审核事件 ID"
// @Param index path int true "图片序号（从 0 开始）"
// @Success 200 {file} binary
// @Failure 400 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/content-moderation/events/{eventID}/images/{index} [get]
func (h *Handler) GetEventImage(c *gin.Context) {
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil || index < 0 {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidImageIndex)
		return
	}
	data, mimeType, err := h.service.OpenEventImage(
		c.Request.Context(),
		middleware.MustUserRole(c),
		c.Param("eventID"),
		index,
	)
	if err != nil {
		writeError(c, err)
		return
	}
	h.service.RecordReviewAudit(c.Request.Context(), appcm.ReviewAuditInput{
		ActorUserID: middleware.MustUserID(c),
		RequestID:   middleware.MustRequestID(c),
		Action:      "content_moderation.event_image.view",
		EventID:     c.Param("eventID"),
		ClientIP:    c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
		Detail:      map[string]int{"imageIndex": index},
	})
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, mimeType, data)
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appcm.ErrSuperAdminRequired):
		response.ErrorFrom(c, http.StatusForbidden, appcm.ErrSuperAdminRequired)
	case errors.Is(err, appcm.ErrAdminRequired):
		response.ErrorFrom(c, http.StatusForbidden, appcm.ErrAdminRequired)
	case errors.Is(err, appcm.ErrEventNotFound):
		response.ErrorFrom(c, http.StatusNotFound, appcm.ErrEventNotFound)
	case errors.Is(err, appcm.ErrServiceConfigRequired):
		response.ErrorFrom(c, http.StatusBadRequest, appcm.ErrServiceConfigRequired)
	case errors.Is(err, appcm.ErrInvalidBaseURL):
		// 端口层哨兵不携带 API 契约，统一按无效配置对外返回。
		response.ErrorFrom(c, http.StatusBadRequest, appcm.ErrInvalidConfig)
	case errors.Is(err, appcm.ErrInvalidModel),
		errors.Is(err, appcm.ErrInvalidTimeout),
		errors.Is(err, appcm.ErrInvalidConcurrency),
		errors.Is(err, appcm.ErrInvalidQueueCapacity),
		errors.Is(err, appcm.ErrInvalidCategories),
		errors.Is(err, appcm.ErrImageTextOnlyCategory),
		errors.Is(err, appcm.ErrInvalidConfig):
		response.ErrorFrom(c, http.StatusBadRequest, appcm.ErrInvalidConfig)
	case errors.Is(err, appcm.ErrProbeFailed):
		response.ErrorFrom(c, http.StatusBadRequest, appcm.ErrProbeFailed)
	case errors.Is(err, appcm.ErrInvalidEventFilter):
		response.ErrorFrom(c, http.StatusBadRequest, appcm.ErrInvalidEventFilter)
	default:
		response.InternalError(c)
	}
}
