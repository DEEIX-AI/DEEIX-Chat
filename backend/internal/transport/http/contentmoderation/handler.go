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
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type userLabelResolver interface {
	ResolveUserLabels(ctx context.Context, userIDs []uint) map[uint]appadmin.UserLabel
}

// Handler exposes admin content-moderation APIs.
type Handler struct {
	service           *appcm.Service
	userLabelResolver userLabelResolver
}

// NewHandler creates the HTTP handler.
func NewHandler(service *appcm.Service) *Handler {
	return &Handler{service: service}
}

// SetUserLabelResolver injects batch user-label resolution for event lists/details.
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

func parsePagination(c *gin.Context) (page int, pageSize int, ok bool) {
	page = 1
	pageSize = 20
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			response.ErrorFrom(c, http.StatusBadRequest, errInvalidPage)
			return 0, 0, false
		}
		page = parsed
	}
	if raw := strings.TrimSpace(c.Query("pageSize")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			response.ErrorFrom(c, http.StatusBadRequest, errInvalidPagesize)
			return 0, 0, false
		}
		pageSize = parsed
	}
	return page, pageSize, true
}

// GetConfig godoc
// @Summary Get content moderation config
// @Tags admin-content-moderation
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ContentModerationConfigResponseDoc
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
// @Summary Update content moderation config
// @Tags admin-content-moderation
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ContentModerationUpdateConfigRequest true "Content moderation configuration"
// @Success 200 {object} ContentModerationConfigUpdateResponseDoc
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
// @Summary Probe content moderation service
// @Tags admin-content-moderation
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ContentModerationProbeResponseDoc
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
// @Summary Get content moderation daily stats
// @Tags admin-content-moderation
// @Produce json
// @Security BearerAuth
// @Param from query string false "Start time (RFC3339)"
// @Param to query string false "End time (RFC3339)"
// @Success 200 {object} ContentModerationStatsResponseDoc
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

// parseOptionalUserID parses the optional userId query parameter.
// Empty means no filter (UserID 0). Invalid values yield 400 and ok=false.
func parseOptionalUserID(c *gin.Context) (uint, bool) {
	raw := strings.TrimSpace(c.Query("userId"))
	if raw == "" {
		return 0, true
	}

	// Limit bit size to the platform's uint width to avoid truncation on 32-bit.
	parsed, err := strconv.ParseUint(raw, 10, strconv.IntSize)
	if err != nil || parsed == 0 {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidUserid)
		return 0, false
	}

	return uint(parsed), true
}

// ListEvents godoc
// @Summary List content moderation events
// @Tags admin-content-moderation
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param pageSize query int false "Page size"
// @Param query query string false "Exact event, user, run, model, result, or summary search"
// @Param result query string false "Result filter"
// @Param direction query string false "Direction filter"
// @Param modality query string false "Modality filter"
// @Param category query string false "Category filter"
// @Param userId query int false "User ID"
// @Param runId query string false "Run ID"
// @Param from query string false "Start time (RFC3339)"
// @Param to query string false "End time (RFC3339)"
// @Success 200 {object} ContentModerationEventListResponseDoc
// @Router /admin/content-moderation/events [get]
func (h *Handler) ListEvents(c *gin.Context) {
	page, pageSize, ok := parsePagination(c)
	if !ok {
		return
	}
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
		RunID:     c.Query("runId"),
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
// @Summary Get content moderation event detail
// @Tags admin-content-moderation
// @Produce json
// @Security BearerAuth
// @Param eventID path string true "Moderation event ID"
// @Success 200 {object} ContentModerationEventDetailResponseDoc
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
// @Summary Stream a isolated moderation image
// @Tags admin-content-moderation
// @Produce octet-stream
// @Security BearerAuth
// @Param eventID path string true "Moderation event ID"
// @Param index path int true "Image index"
// @Success 200 {file} binary
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
		response.ErrorWithCode(c, http.StatusBadRequest, "content_moderation.config_required")
	case errors.Is(err, appcm.ErrInvalidBaseURL),
		errors.Is(err, appcm.ErrInvalidModel),
		errors.Is(err, appcm.ErrInvalidTimeout),
		errors.Is(err, appcm.ErrInvalidConcurrency),
		errors.Is(err, appcm.ErrInvalidQueueCapacity),
		errors.Is(err, appcm.ErrInvalidCategories),
		errors.Is(err, appcm.ErrImageTextOnlyCategory),
		errors.Is(err, appcm.ErrInvalidConfig):
		response.ErrorWithCode(c, http.StatusBadRequest, "content_moderation.invalid_config")
	case errors.Is(err, appcm.ErrProbeFailed):
		response.ErrorWithCode(c, http.StatusBadRequest, "content_moderation.probe_failed")
	case errors.Is(err, appcm.ErrInvalidEventFilter):
		response.ErrorWithCode(c, http.StatusBadRequest, response.CodeRequestInvalidQuery)
	default:
		response.InternalError(c)
	}
}
