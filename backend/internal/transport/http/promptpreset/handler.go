package promptpreset

import (
	"errors"
	"net/http"
	"strconv"

	apppromptpreset "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/promptpreset"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// Handler 封装预制提示词 HTTP 处理。
type Handler struct {
	service *apppromptpreset.Service
}

// NewHandler 创建预制提示词处理器。
func NewHandler(service *apppromptpreset.Service) *Handler {
	return &Handler{service: service}
}

// ListVisiblePromptPresets godoc
// @Summary 查询当前用户可用预制提示词
// @Description 返回管理员内置和当前用户自定义的已启用提示词，用于 slash 选择器
// @Tags prompt-presets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param q query string false "搜索关键词"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} PromptPresetPageResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /prompt-presets [get]
func (h *Handler) ListVisiblePromptPresets(c *gin.Context) {
	page, pageSize := pageParams(c)
	items, total, err := h.service.ListVisible(c.Request.Context(), middleware.MustUserID(c), apppromptpreset.ListInput{
		Query:    c.Query("q"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writePromptPresetError(c, err)
		return
	}
	response.SuccessPage(c, total, toPromptPresetResponses(items))
}

// ListMyPromptPresets godoc
// @Summary 查询我的自定义提示词
// @Description 分页查询当前用户自定义提示词
// @Tags prompt-presets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param q query string false "搜索关键词"
// @Param enabled query bool false "是否启用"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} PromptPresetPageResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /prompt-presets/mine [get]
func (h *Handler) ListMyPromptPresets(c *gin.Context) {
	page, pageSize := pageParams(c)
	items, total, err := h.service.ListMine(c.Request.Context(), middleware.MustUserID(c), apppromptpreset.ListInput{
		Query:    c.Query("q"),
		Enabled:  boolQuery(c, "enabled"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writePromptPresetError(c, err)
		return
	}
	response.SuccessPage(c, total, toPromptPresetResponses(items))
}

// CreateMyPromptPreset godoc
// @Summary 创建我的自定义提示词
// @Tags prompt-presets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body WritePromptPresetRequest true "提示词配置"
// @Success 200 {object} PromptPresetResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /prompt-presets/mine [post]
func (h *Handler) CreateMyPromptPreset(c *gin.Context) {
	var req WritePromptPresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CreateUser(c.Request.Context(), middleware.MustUserID(c), writeInputFromRequest(req))
	if err != nil {
		writePromptPresetError(c, err)
		return
	}
	response.Success(c, PromptPresetDataResponse{PromptPreset: toPromptPresetResponse(*item)})
}

// PatchMyPromptPreset godoc
// @Summary 更新我的自定义提示词
// @Tags prompt-presets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "提示词ID"
// @Param body body PatchPromptPresetRequest true "更新字段"
// @Success 200 {object} PromptPresetResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /prompt-presets/mine/{id} [patch]
func (h *Handler) PatchMyPromptPreset(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req PatchPromptPresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.UpdateUser(c.Request.Context(), middleware.MustUserID(c), id, patchInputFromRequest(req))
	if err != nil {
		writePromptPresetError(c, err)
		return
	}
	response.Success(c, PromptPresetDataResponse{PromptPreset: toPromptPresetResponse(*item)})
}

// DeleteMyPromptPreset godoc
// @Summary 删除我的自定义提示词
// @Tags prompt-presets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "提示词ID"
// @Success 200 {object} PromptPresetDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /prompt-presets/mine/{id} [delete]
func (h *Handler) DeleteMyPromptPreset(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.service.DeleteUser(c.Request.Context(), middleware.MustUserID(c), id); err != nil {
		writePromptPresetError(c, err)
		return
	}
	response.Success(c, PromptPresetDeleteDataResponse{Deleted: true})
}

// ListAdminPromptPresets godoc
// @Summary 管理员查询内置提示词
// @Tags admin-prompt-presets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param q query string false "搜索关键词"
// @Param enabled query bool false "是否启用"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} PromptPresetPageResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/prompt-presets [get]
func (h *Handler) ListAdminPromptPresets(c *gin.Context) {
	page, pageSize := pageParams(c)
	items, total, err := h.service.ListAdminBuiltin(c.Request.Context(), apppromptpreset.ListInput{
		Query:    c.Query("q"),
		Enabled:  boolQuery(c, "enabled"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writePromptPresetError(c, err)
		return
	}
	response.SuccessPage(c, total, toPromptPresetResponses(items))
}

// CreateAdminPromptPreset godoc
// @Summary 管理员创建内置提示词
// @Tags admin-prompt-presets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body WritePromptPresetRequest true "提示词配置"
// @Success 200 {object} PromptPresetResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/prompt-presets [post]
func (h *Handler) CreateAdminPromptPreset(c *gin.Context) {
	var req WritePromptPresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	userID := middleware.MustUserID(c)
	item, err := h.service.CreateBuiltin(c.Request.Context(), userID, writeInputFromRequest(req))
	if err != nil {
		writePromptPresetError(c, err)
		return
	}
	h.service.RecordAudit(c.Request.Context(), auditInput(c, "prompt_preset.create_builtin", item.ID, map[string]interface{}{"trigger": item.Trigger}))
	response.Success(c, PromptPresetDataResponse{PromptPreset: toPromptPresetResponse(*item)})
}

// PatchAdminPromptPreset godoc
// @Summary 管理员更新内置提示词
// @Tags admin-prompt-presets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "提示词ID"
// @Param body body PatchPromptPresetRequest true "更新字段"
// @Success 200 {object} PromptPresetResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/prompt-presets/{id} [patch]
func (h *Handler) PatchAdminPromptPreset(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req PatchPromptPresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.UpdateBuiltin(c.Request.Context(), middleware.MustUserID(c), id, patchInputFromRequest(req))
	if err != nil {
		writePromptPresetError(c, err)
		return
	}
	h.service.RecordAudit(c.Request.Context(), auditInput(c, "prompt_preset.update_builtin", item.ID, map[string]interface{}{"trigger": item.Trigger}))
	response.Success(c, PromptPresetDataResponse{PromptPreset: toPromptPresetResponse(*item)})
}

// DeleteAdminPromptPreset godoc
// @Summary 管理员删除内置提示词
// @Tags admin-prompt-presets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "提示词ID"
// @Success 200 {object} PromptPresetDeleteResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/prompt-presets/{id} [delete]
func (h *Handler) DeleteAdminPromptPreset(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.service.DeleteBuiltin(c.Request.Context(), middleware.MustUserID(c), id); err != nil {
		writePromptPresetError(c, err)
		return
	}
	h.service.RecordAudit(c.Request.Context(), auditInput(c, "prompt_preset.delete_builtin", id, nil))
	response.Success(c, PromptPresetDeleteDataResponse{Deleted: true})
}

func writeInputFromRequest(req WritePromptPresetRequest) apppromptpreset.WriteInput {
	return apppromptpreset.WriteInput{
		Title:       req.Title,
		Trigger:     req.Trigger,
		Description: req.Description,
		Content:     req.Content,
		Enabled:     req.Enabled,
		SortOrder:   req.SortOrder,
	}
}

func patchInputFromRequest(req PatchPromptPresetRequest) apppromptpreset.PatchInput {
	return apppromptpreset.PatchInput{
		Title:       req.Title,
		Trigger:     req.Trigger,
		Description: req.Description,
		Content:     req.Content,
		Enabled:     req.Enabled,
		SortOrder:   req.SortOrder,
	}
}

func idParam(c *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, "invalid prompt preset id")
		return 0, false
	}
	return uint(id), true
}

func boolQuery(c *gin.Context, key string) *bool {
	raw := c.Query(key)
	if raw == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func pageParams(c *gin.Context) (int, int) {
	page := 1
	pageSize := 20
	const maxPageSize = 100
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

func auditInput(c *gin.Context, action string, resourceID uint, detail interface{}) apppromptpreset.AuditInput {
	return apppromptpreset.AuditInput{
		UserID:     middleware.MustUserID(c),
		RequestID:  middleware.MustRequestID(c),
		Action:     action,
		ResourceID: strconv.FormatUint(uint64(resourceID), 10),
		ClientIP:   c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		Detail:     detail,
	}
}

func writePromptPresetError(c *gin.Context, err error) {
	if errors.Is(err, apppromptpreset.ErrPromptPresetNotFound) {
		response.Error(c, http.StatusNotFound, "prompt preset not found")
		return
	}
	if errors.Is(err, apppromptpreset.ErrPromptPresetConflict) {
		response.Error(c, http.StatusConflict, "prompt preset trigger already exists")
		return
	}
	if errors.Is(err, apppromptpreset.ErrInvalidPromptPreset) {
		response.ErrorFrom(c, http.StatusBadRequest, err)
		return
	}
	response.Error(c, http.StatusInternalServerError, "prompt preset operation failed")
}
