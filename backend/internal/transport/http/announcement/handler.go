package announcement

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	appannouncement "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/announcement"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// Handler 封装公告 HTTP 处理。
type Handler struct {
	service *appannouncement.Service
}

// NewHandler 创建公告处理器。
func NewHandler(service *appannouncement.Service) *Handler {
	return &Handler{service: service}
}

// ListAnnouncements godoc
// @Summary 获取当前公告
// @Description 登录用户获取当前可展示的站点公告列表
// @Tags announcements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} AnnouncementListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /announcements [get]
func (h *Handler) ListAnnouncements(c *gin.Context) {
	items, err := h.service.ListActive(c.Request.Context(), middleware.MustUserID(c), time.Now())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list announcements failed")
		return
	}
	response.Success(c, toAnnouncementResponses(items))
}

// DismissAnnouncementToday godoc
// @Summary 今日不再显示公告
// @Description 登录用户对当前公告版本记录今日不再显示
// @Tags announcements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "公告ID"
// @Param body body AnnouncementStateRequest true "公告版本"
// @Success 200 {object} AnnouncementDismissResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /announcements/{id}/dismiss-today [post]
func (h *Handler) DismissAnnouncementToday(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid announcement id")
		return
	}
	var req AnnouncementStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	now := time.Now()
	year, month, day := now.Date()
	dismissedUntil := time.Date(year, month, day+1, 0, 0, 0, 0, now.Location())
	if err := h.service.DismissToday(c.Request.Context(), middleware.MustUserID(c), uint(id), req.UpdatedAt, now, dismissedUntil); err != nil {
		writeAnnouncementError(c, err)
		return
	}
	response.Success(c, AnnouncementDismissDataResponse{Dismissed: true})
}

// CloseAnnouncement godoc
// @Summary 关闭公告
// @Description 登录用户关闭当前公告版本；公告更新后会重新展示
// @Tags announcements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "公告ID"
// @Param body body AnnouncementStateRequest true "公告版本"
// @Success 200 {object} AnnouncementCloseResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /announcements/{id}/close [post]
func (h *Handler) CloseAnnouncement(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid announcement id")
		return
	}
	var req AnnouncementStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	if err := h.service.Close(c.Request.Context(), middleware.MustUserID(c), uint(id), req.UpdatedAt, time.Now()); err != nil {
		writeAnnouncementError(c, err)
		return
	}
	response.Success(c, AnnouncementCloseDataResponse{Closed: true})
}

// ListAdminAnnouncements godoc
// @Summary 管理员查询公告
// @Description 分页查询站点公告
// @Tags admin-announcements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param status query string false "状态：active/inactive"
// @Param type query string false "类型：critical/warning/info/normal/general"
// @Param pinned query bool false "是否置顶"
// @Param q query string false "搜索关键词"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} AdminAnnouncementListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/announcements [get]
func (h *Handler) ListAdminAnnouncements(c *gin.Context) {
	page, pageSize := pageParams(c)
	var pinned *bool
	if raw := c.Query("pinned"); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			pinned = &parsed
		}
	}
	items, total, err := h.service.ListAdmin(c.Request.Context(), appannouncement.ListInput{
		Query:    c.Query("q"),
		Status:   c.Query("status"),
		Type:     c.Query("type"),
		Pinned:   pinned,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list announcements failed")
		return
	}
	response.SuccessPage(c, total, toAnnouncementResponses(items))
}

// CreateAnnouncement godoc
// @Summary 管理员创建公告
// @Description 创建一条 Markdown 站点公告
// @Tags admin-announcements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateAnnouncementRequest true "公告配置"
// @Success 200 {object} AnnouncementResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/announcements [post]
func (h *Handler) CreateAnnouncement(c *gin.Context) {
	var req CreateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.Create(c.Request.Context(), middleware.MustUserID(c), createInputFromRequest(req))
	if err != nil {
		writeAnnouncementError(c, err)
		return
	}
	response.Success(c, AnnouncementDataResponse{Announcement: toAnnouncementResponse(*item)})
}

// PatchAnnouncement godoc
// @Summary 管理员更新公告
// @Description 更新公告标题、内容、状态、优先级和有效期
// @Tags admin-announcements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "公告ID"
// @Param body body PatchAnnouncementRequestDoc true "公告更新字段"
// @Success 200 {object} AnnouncementResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/announcements/{id} [patch]
func (h *Handler) PatchAnnouncement(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid announcement id")
		return
	}
	var req PatchAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.Update(c.Request.Context(), uint(id), appannouncement.PatchInput{
		Title:           req.Title,
		ContentMarkdown: req.ContentMarkdown,
		Status:          req.Status,
		Type:            req.Type,
		Pinned:          req.Pinned,
		Priority:        req.Priority,
		StartsAtSet:     req.StartsAt.Set,
		StartsAt:        req.StartsAt.Value,
		ExpiresAtSet:    req.ExpiresAt.Set,
		ExpiresAt:       req.ExpiresAt.Value,
	})
	if err != nil {
		writeAnnouncementError(c, err)
		return
	}
	response.Success(c, AnnouncementDataResponse{Announcement: toAnnouncementResponse(*item)})
}

// DeleteAnnouncement godoc
// @Summary 管理员删除公告
// @Description 软删除公告
// @Tags admin-announcements
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "公告ID"
// @Success 200 {object} AnnouncementDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/announcements/{id} [delete]
func (h *Handler) DeleteAnnouncement(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid announcement id")
		return
	}
	if err := h.service.Delete(c.Request.Context(), uint(id)); err != nil {
		writeAnnouncementError(c, err)
		return
	}
	response.Success(c, AnnouncementDeleteDataResponse{Deleted: true})
}

func writeAnnouncementError(c *gin.Context, err error) {
	if errors.Is(err, appannouncement.ErrAnnouncementNotFound) {
		response.Error(c, http.StatusNotFound, "announcement not found")
		return
	}
	if errors.Is(err, appannouncement.ErrInvalidAnnouncement) {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Error(c, http.StatusInternalServerError, "announcement operation failed")
}

func pageParams(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20
	const maxPageSize = 1000
	if raw := c.Query("page"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if raw := c.Query("page_size"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}
