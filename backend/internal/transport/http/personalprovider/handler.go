package personalprovider

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	appadmin "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	apppersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/personalprovider"
	domainpersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/personalprovider"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// Handler 处理用户自带 Key 的模型服务接口。
type Handler struct {
	service     *apppersonalprovider.Service
	userLabeler userLabelResolver
}

// userLabelResolver 批量解析用户展示名，供管理员列表显示服务所属用户。
type userLabelResolver interface {
	ResolveUserLabels(ctx context.Context, userIDs []uint) map[uint]appadmin.UserLabel
}

// SetUserLabelResolver 注入用户展示名解析；未注入时管理员列表只返回用户 ID。
func (h *Handler) SetUserLabelResolver(resolver userLabelResolver) {
	h.userLabeler = resolver
}

// NewHandler 创建处理器。
func NewHandler(service *apppersonalprovider.Service) *Handler {
	return &Handler{service: service}
}

// GetAccess godoc
// @Summary 查询个人模型服务是否可用
// @Description 返回当前用户能否添加自己的模型服务、能否通过链接导入、数量上限与可选协议
// @Tags personal-providers
// @Produce json
// @Security BearerAuth
// @Success 200 {object} PersonalProviderAccessResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /me/model-providers/access [get]
func (h *Handler) GetAccess(c *gin.Context) {
	access, err := h.service.GetAccess(c.Request.Context(), middleware.MustUserID(c))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, PersonalProviderAccessResponse{
		Enabled:        access.Enabled,
		MaxPerUser:     access.MaxPerUser,
		Protocols:      access.Protocols,
		ModelProtocols: access.ModelProtocols,
	})
}

// List godoc
// @Summary 查询我的模型服务
// @Description 返回当前用户添加的模型服务；响应不包含 API Key
// @Tags personal-providers
// @Produce json
// @Security BearerAuth
// @Success 200 {object} PersonalProviderListResponseDoc
// @Failure 403 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /me/model-providers [get]
func (h *Handler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context(), middleware.MustUserID(c))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, PersonalProviderListResponse{Providers: toProviderResponses(items)})
}

// Probe godoc
// @Summary 检测模型服务
// @Description 使用地址与 API Key 拉取上游模型列表，不保存任何内容。地址必须是 HTTPS 公网地址
// @Tags personal-providers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body PersonalProviderProbeRequest true "候选配置"
// @Success 200 {object} PersonalProviderModelsResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Failure 429 {object} ErrorDoc
// @Failure 502 {object} ErrorDoc
// @Router /me/model-providers/probe [post]
func (h *Handler) Probe(c *gin.Context) {
	var req PersonalProviderProbeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	models, err := h.service.Probe(c.Request.Context(), middleware.MustUserID(c), apppersonalprovider.CandidateInput{
		Protocol: req.Protocol,
		BaseURL:  req.BaseURL,
		APIKey:   req.APIKey,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, PersonalProviderModelsResponse{Models: toAvailableModelResponses(models)})
}

// Create godoc
// @Summary 添加模型服务
// @Description 检测并保存模型服务。API Key 加密保存，之后任何接口都不会返回明文
// @Tags personal-providers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreatePersonalProviderRequest true "服务配置"
// @Success 200 {object} PersonalProviderResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 429 {object} ErrorDoc
// @Failure 502 {object} ErrorDoc
// @Router /me/model-providers [post]
func (h *Handler) Create(c *gin.Context) {
	var req CreatePersonalProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.Create(c.Request.Context(), middleware.MustUserID(c), apppersonalprovider.CreateInput{
		Name:     req.Name,
		Icon:     req.Icon,
		Protocol: req.Protocol,
		BaseURL:  req.BaseURL,
		APIKey:   req.APIKey,
		Models:   toModelInputs(req.Models),
		FromLink: req.Source == domainpersonalprovider.SourceLink,
	}, requestMeta(c))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, PersonalProviderDataResponse{Provider: toProviderResponse(*item)})
}

// Update godoc
// @Summary 更新模型服务
// @Description 修改名称、更换 API Key、调整启用的模型或启用/停用。更换 Key 或调整模型时会重新检测
// @Tags personal-providers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "服务 ID"
// @Param body body UpdatePersonalProviderRequest true "更新字段"
// @Success 200 {object} PersonalProviderResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 403 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 429 {object} ErrorDoc
// @Failure 502 {object} ErrorDoc
// @Router /me/model-providers/{id} [patch]
func (h *Handler) Update(c *gin.Context) {
	var req UpdatePersonalProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	var models *[]apppersonalprovider.ModelInput
	if req.Models != nil {
		inputs := toModelInputs(*req.Models)
		models = &inputs
	}
	item, err := h.service.Update(c.Request.Context(), middleware.MustUserID(c), c.Param("id"), apppersonalprovider.UpdateInput{
		Name:    req.Name,
		Icon:    req.Icon,
		APIKey:  req.APIKey,
		Models:  models,
		Enabled: req.Enabled,
	}, requestMeta(c))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, PersonalProviderDataResponse{Provider: toProviderResponse(*item)})
}

// Delete godoc
// @Summary 删除模型服务
// @Description 删除服务及其加密保存的 API Key。功能被关闭后仍可删除
// @Tags personal-providers
// @Produce json
// @Security BearerAuth
// @Param id path string true "服务 ID"
// @Success 200 {object} PersonalProviderDeleteResponseDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /me/model-providers/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), middleware.MustUserID(c), c.Param("id"), requestMeta(c)); err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, PersonalProviderDeleteResponse{Deleted: true})
}

// ListModels godoc
// @Summary 拉取模型服务的可用模型
// @Description 使用已保存的 API Key 重新拉取上游模型列表
// @Tags personal-providers
// @Produce json
// @Security BearerAuth
// @Param id path string true "服务 ID"
// @Success 200 {object} PersonalProviderModelsResponseDoc
// @Failure 403 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 429 {object} ErrorDoc
// @Failure 502 {object} ErrorDoc
// @Router /me/model-providers/{id}/models [get]
func (h *Handler) ListModels(c *gin.Context) {
	models, err := h.service.ListAvailableModels(c.Request.Context(), middleware.MustUserID(c), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, PersonalProviderModelsResponse{Models: toAvailableModelResponses(models)})
}

// AdminList godoc
// @Summary 管理员查询用户模型服务
// @Description 分页列出所有用户添加的模型服务；只有元数据与打码提示，不包含 API Key
// @Tags personal-providers
// @Produce json
// @Security BearerAuth
// @Param q query string false "按名称、域名或所属用户（用户名、显示名、邮箱）搜索"
// @Param status query string false "状态：active/disabled/suspended"
// @Param host query string false "精确域名"
// @Param owner_user_id query int false "所属用户 ID"
// @Param page query int false "页码（从 1 开始，默认 1）"
// @Param page_size query int false "每页数量（1-1000，默认 20）"
// @Success 200 {object} AdminPersonalProviderPageResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/model-providers [get]
func (h *Handler) AdminList(c *gin.Context) {
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	ownerUserID := uint(0)
	if raw := c.Query("owner_user_id"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			response.InvalidQueryParam(c, "owner_user_id")
			return
		}
		ownerUserID = uint(parsed)
	}
	items, total, err := h.service.AdminList(c.Request.Context(), apppersonalprovider.AdminListInput{
		Query:       c.Query("q"),
		Status:      c.Query("status"),
		Host:        c.Query("host"),
		OwnerUserID: ownerUserID,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	labels := map[uint]appadmin.UserLabel{}
	if h.userLabeler != nil && len(items) > 0 {
		ownerIDs := make([]uint, 0, len(items))
		for _, item := range items {
			ownerIDs = append(ownerIDs, item.OwnerUserID)
		}
		labels = h.userLabeler.ResolveUserLabels(c.Request.Context(), ownerIDs)
	}
	response.SuccessPage(c, total, toAdminProviderResponses(items, labels))
}

// AdminSetSuspended godoc
// @Summary 管理员停用或恢复用户模型服务
// @Description 停用后用户不能自行启用，该服务的模型立即不可用
// @Tags personal-providers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body AdminSuspendPersonalProvidersRequest true "服务 ID 与目标状态"
// @Success 200 {object} PersonalProviderAffectedResponseDoc
// @Failure 400 {object} ErrorDoc
// @Router /admin/model-providers/suspend [post]
func (h *Handler) AdminSetSuspended(c *gin.Context) {
	var req AdminSuspendPersonalProvidersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	affected, err := h.service.AdminSetSuspended(c.Request.Context(), middleware.MustUserID(c), req.IDs, req.Suspended, requestMeta(c))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, PersonalProviderAffectedResponse{Affected: affected})
}

// AdminSuspendHost godoc
// @Summary 管理员按域名停用用户模型服务
// @Description 停用指定域名下的全部用户模型服务；如需阻止今后再添加，请同时把域名加入禁止列表
// @Tags personal-providers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body AdminSuspendPersonalProviderHostRequest true "域名"
// @Success 200 {object} PersonalProviderAffectedResponseDoc
// @Failure 400 {object} ErrorDoc
// @Router /admin/model-providers/suspend-host [post]
func (h *Handler) AdminSuspendHost(c *gin.Context) {
	var req AdminSuspendPersonalProviderHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	affected, err := h.service.AdminSuspendHost(c.Request.Context(), middleware.MustUserID(c), req.Host, requestMeta(c))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, PersonalProviderAffectedResponse{Affected: affected})
}

// AdminDelete godoc
// @Summary 管理员删除用户模型服务
// @Description 删除指定服务及其加密保存的 API Key
// @Tags personal-providers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body AdminDeletePersonalProvidersRequest true "服务 ID"
// @Success 200 {object} PersonalProviderAffectedResponseDoc
// @Failure 400 {object} ErrorDoc
// @Router /admin/model-providers/batch-delete [post]
func (h *Handler) AdminDelete(c *gin.Context) {
	var req AdminDeletePersonalProvidersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	affected, err := h.service.AdminDelete(c.Request.Context(), middleware.MustUserID(c), req.IDs, requestMeta(c))
	if err != nil {
		writeError(c, err)
		return
	}
	response.Success(c, PersonalProviderAffectedResponse{Affected: affected})
}

func requestMeta(c *gin.Context) apppersonalprovider.RequestMeta {
	return apppersonalprovider.RequestMeta{
		RequestID: middleware.MustRequestID(c),
		ClientIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}
}

var errorStatuses = []struct {
	err    error
	status int
}{
	{apppersonalprovider.ErrFeatureDisabled, http.StatusForbidden},
	{apppersonalprovider.ErrNotFound, http.StatusNotFound},
	{apppersonalprovider.ErrLimitReached, http.StatusConflict},
	{apppersonalprovider.ErrDuplicate, http.StatusConflict},
	{apppersonalprovider.ErrSuspended, http.StatusConflict},
	{apppersonalprovider.ErrInvalidName, http.StatusBadRequest},
	{apppersonalprovider.ErrInvalidIcon, http.StatusBadRequest},
	{apppersonalprovider.ErrInvalidProtocol, http.StatusBadRequest},
	{apppersonalprovider.ErrInvalidBaseURL, http.StatusBadRequest},
	{apppersonalprovider.ErrBlockedHost, http.StatusBadRequest},
	{apppersonalprovider.ErrInvalidAPIKey, http.StatusBadRequest},
	{apppersonalprovider.ErrInvalidModels, http.StatusBadRequest},
	{apppersonalprovider.ErrInvalidModelProtocol, http.StatusBadRequest},
	{apppersonalprovider.ErrUpstreamRejected, http.StatusBadGateway},
	{apppersonalprovider.ErrUpstreamUnavailable, http.StatusBadGateway},
	{apppersonalprovider.ErrModelUnavailable, http.StatusServiceUnavailable},
}

func writeError(c *gin.Context, err error) {
	for _, entry := range errorStatuses {
		if errors.Is(err, entry.err) {
			response.ErrorFrom(c, entry.status, entry.err)
			return
		}
	}
	response.InternalError(c, err)
}
