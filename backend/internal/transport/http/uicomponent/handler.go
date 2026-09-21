package uicomponent

import (
	"errors"
	"net/http"
	"strconv"

	appuicomponent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/uicomponent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/queryparam"
	"github.com/gin-gonic/gin"
)

// Handler 封装交互式组件 HTTP 处理。
type Handler struct {
	service *appuicomponent.Service
}

// NewHandler 创建组件处理器。
func NewHandler(service *appuicomponent.Service) *Handler {
	return &Handler{service: service}
}

// ListVisible godoc
// @Summary 查询当前用户可用的交互式组件
// @Description 返回已启用的内置、平台组件与当前用户自定义组件，含渲染源，用于会话勾选与消息渲染
// @Tags ui-components
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param q query string false "搜索关键词"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} UIComponentPageResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /ui-components [get]
func (h *Handler) ListVisible(c *gin.Context) {
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	items, total, err := h.service.ListVisible(c.Request.Context(), middleware.MustUserID(c), appuicomponent.ListInput{
		Query:    c.Query("q"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.SuccessPage(c, total, toResponses(items))
}

// ListMine godoc
// @Summary 查询我的自定义组件
// @Tags ui-components
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param q query string false "搜索关键词"
// @Param enabled query bool false "是否启用"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} UIComponentPageResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /ui-components/mine [get]
func (h *Handler) ListMine(c *gin.Context) {
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	items, total, err := h.service.ListMine(c.Request.Context(), middleware.MustUserID(c), appuicomponent.ListInput{
		Query:    c.Query("q"),
		Enabled:  queryparam.OptionalBool(c.Query("enabled")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.SuccessPage(c, total, toResponses(items))
}

// CreateMine godoc
// @Summary 创建我的自定义组件
// @Tags ui-components
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body WriteUIComponentRequest true "组件内容"
// @Success 200 {object} UIComponentResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /ui-components/mine [post]
func (h *Handler) CreateMine(c *gin.Context) {
	var req WriteUIComponentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CreateUser(c.Request.Context(), middleware.MustUserID(c), writeInputFromRequest(req))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, UIComponentDataResponse{Component: toResponse(*item)})
}

// PatchMine godoc
// @Summary 更新我的自定义组件
// @Tags ui-components
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "组件ID"
// @Param body body PatchUIComponentRequest true "更新字段"
// @Success 200 {object} UIComponentResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /ui-components/mine/{id} [patch]
func (h *Handler) PatchMine(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req PatchUIComponentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.UpdateUser(c.Request.Context(), middleware.MustUserID(c), id, patchInputFromRequest(req))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, UIComponentDataResponse{Component: toResponse(*item)})
}

// DeleteMine godoc
// @Summary 删除我的自定义组件
// @Tags ui-components
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "组件ID"
// @Success 200 {object} UIComponentDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /ui-components/mine/{id} [delete]
func (h *Handler) DeleteMine(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.service.DeleteUser(c.Request.Context(), middleware.MustUserID(c), id); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, UIComponentDeleteDataResponse{Deleted: true})
}

// ListAdmin godoc
// @Summary 查询内置与平台组件
// @Tags admin/ui-components
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param q query string false "搜索关键词"
// @Param scope query string false "作用域：builtin 或 platform，留空为全部"
// @Param enabled query bool false "是否启用"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} UIComponentPageResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/ui-components [get]
func (h *Handler) ListAdmin(c *gin.Context) {
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	items, total, err := h.service.ListAdmin(c.Request.Context(), appuicomponent.ListInput{
		Query:    c.Query("q"),
		Scope:    c.Query("scope"),
		Enabled:  queryparam.OptionalBool(c.Query("enabled")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.SuccessPage(c, total, toResponses(items))
}

// CreateAdmin godoc
// @Summary 创建平台组件
// @Tags admin/ui-components
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body WriteUIComponentRequest true "组件内容"
// @Success 200 {object} UIComponentResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/ui-components [post]
func (h *Handler) CreateAdmin(c *gin.Context) {
	var req WriteUIComponentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CreatePlatform(c.Request.Context(), middleware.MustUserID(c), writeInputFromRequest(req))
	if err != nil {
		writeError(c, err)
		return
	}
	h.service.RecordAudit(c.Request.Context(), auditInput(c, "ui_component.create_platform", item.ID, map[string]any{"name": item.Name}))
	response.Success(c, UIComponentDataResponse{Component: toResponse(*item)})
}

// PatchAdmin godoc
// @Summary 更新内置或平台组件
// @Description 内置组件只允许修改启用状态、描述与排序
// @Tags admin/ui-components
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "组件ID"
// @Param body body PatchUIComponentRequest true "更新字段"
// @Success 200 {object} UIComponentResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/ui-components/{id} [patch]
func (h *Handler) PatchAdmin(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req PatchUIComponentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.UpdateAdmin(c.Request.Context(), middleware.MustUserID(c), id, patchInputFromRequest(req))
	if err != nil {
		writeError(c, err)
		return
	}
	h.service.RecordAudit(c.Request.Context(), auditInput(c, "ui_component.update", item.ID, map[string]any{"name": item.Name, "scope": item.Scope}))
	response.Success(c, UIComponentDataResponse{Component: toResponse(*item)})
}

// DeleteAdmin godoc
// @Summary 删除平台组件
// @Description 内置组件受保护，不允许删除
// @Tags admin/ui-components
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "组件ID"
// @Success 200 {object} UIComponentDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/ui-components/{id} [delete]
func (h *Handler) DeleteAdmin(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.service.DeleteAdmin(c.Request.Context(), middleware.MustUserID(c), id); err != nil {
		writeError(c, err)
		return
	}
	h.service.RecordAudit(c.Request.Context(), auditInput(c, "ui_component.delete_platform", id, nil))
	response.Success(c, UIComponentDeleteDataResponse{Deleted: true})
}

func writeInputFromRequest(req WriteUIComponentRequest) appuicomponent.WriteInput {
	return appuicomponent.WriteInput{
		Name:           req.Name,
		Version:        req.Version,
		Description:    req.Description,
		PropsSummary:   req.PropsSummary,
		PropsSchema:    req.PropsSchema,
		RendererSource: req.RendererSource,
		Enabled:        req.Enabled,
		SortOrder:      req.SortOrder,
	}
}

func patchInputFromRequest(req PatchUIComponentRequest) appuicomponent.PatchInput {
	return appuicomponent.PatchInput{
		Name:           req.Name,
		Version:        req.Version,
		Description:    req.Description,
		PropsSummary:   req.PropsSummary,
		PropsSchema:    req.PropsSchema,
		RendererSource: req.RendererSource,
		Enabled:        req.Enabled,
		SortOrder:      req.SortOrder,
	}
}

func idParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if err != nil || id == 0 {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidComponentID)
		return 0, false
	}
	return uint(id), true
}

func auditInput(c *gin.Context, action string, resourceID uint, detail any) appuicomponent.AuditInput {
	return appuicomponent.AuditInput{
		UserID:     middleware.MustUserID(c),
		RequestID:  middleware.MustRequestID(c),
		Action:     action,
		ResourceID: strconv.FormatUint(uint64(resourceID), 10),
		ClientIP:   c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		Detail:     detail,
	}
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appuicomponent.ErrComponentNotFound):
		response.ErrorFrom(c, http.StatusNotFound, err)
	case errors.Is(err, appuicomponent.ErrComponentConflict):
		response.ErrorFrom(c, http.StatusConflict, err)
	case errors.Is(err, appuicomponent.ErrInvalidComponent), errors.Is(err, appuicomponent.ErrBuiltinProtected):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	default:
		response.InternalError(c)
	}
}
