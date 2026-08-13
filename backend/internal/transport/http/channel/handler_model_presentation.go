package channel

import (
	"errors"
	"net/http"
	"strings"

	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// ListModelVendors godoc
// @Summary 管理员查询模型技术厂商
// @Description 分页查询模型技术厂商目录；技术厂商是路由、权限和计费使用的稳定身份
// @Tags llm
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param q query string false "搜索 key 或名称"
// @Success 200 {object} ModelVendorListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/model-vendors [get]
func (h *Handler) ListModelVendors(c *gin.Context) {
	page, pageSize := pageParams(c)
	items, total, err := h.service.ListModelVendors(c.Request.Context(), page, pageSize, c.Query("q"))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list model vendors failed")
		return
	}
	results := make([]ModelVendorResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toModelVendorResponse(item))
	}
	response.SuccessPage(c, total, results)
}

// CreateModelVendor godoc
// @Summary 管理员创建模型技术厂商
// @Description 创建新的稳定技术厂商身份；创建后可供平台模型选择
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateModelVendorRequest true "技术厂商参数"
// @Success 200 {object} ModelVendorDataResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Router /admin/llm/model-vendors [post]
func (h *Handler) CreateModelVendor(c *gin.Context) {
	var request CreateModelVendorRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CreateModelVendor(c.Request.Context(), appchannel.CreateModelVendorInput{
		Key: request.Key, Name: request.Name, Icon: request.Icon,
	})
	if err != nil {
		writeModelVendorError(c, err, "create model vendor failed")
		return
	}
	response.Success(c, ModelVendorDataResponse{Vendor: toModelVendorResponse(*item)})
}

// UpdateModelVendor godoc
// @Summary 管理员更新模型技术厂商
// @Description 更新厂商展示名称和图标；稳定技术 key 不可修改
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "技术厂商 key"
// @Param body body UpdateModelVendorRequest true "技术厂商参数"
// @Success 200 {object} ModelVendorDataResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Router /admin/llm/model-vendors/{key} [patch]
func (h *Handler) UpdateModelVendor(c *gin.Context) {
	var request UpdateModelVendorRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.UpdateModelVendor(c.Request.Context(), strings.TrimSpace(c.Param("key")), appchannel.UpdateModelVendorInput{
		Name: request.Name, Icon: request.Icon,
	})
	if err != nil {
		writeModelVendorError(c, err, "update model vendor failed")
		return
	}
	response.Success(c, ModelVendorDataResponse{Vendor: toModelVendorResponse(*item)})
}

// ListModelDisplayGroups godoc
// @Summary 管理员查询模型展示分组
// @Description 分页查询自定义展示分组；未绑定分组的模型继续按技术厂商展示
// @Tags llm
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param q query string false "搜索名称"
// @Success 200 {object} ModelDisplayGroupListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/model-display-groups [get]
func (h *Handler) ListModelDisplayGroups(c *gin.Context) {
	page, pageSize := pageParams(c)
	items, total, err := h.service.ListModelDisplayGroups(c.Request.Context(), page, pageSize, c.Query("q"))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "list model display groups failed")
		return
	}
	results := make([]ModelDisplayGroupResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toModelDisplayGroupResponse(item))
	}
	response.SuccessPage(c, total, results)
}

// CreateModelDisplayGroup godoc
// @Summary 管理员创建模型展示分组
// @Description 创建仅影响用户界面归类的自定义模型分组
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateModelDisplayGroupRequest true "展示分组参数"
// @Success 200 {object} ModelDisplayGroupDataResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Router /admin/llm/model-display-groups [post]
func (h *Handler) CreateModelDisplayGroup(c *gin.Context) {
	var request CreateModelDisplayGroupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CreateModelDisplayGroup(c.Request.Context(), appchannel.CreateModelDisplayGroupInput{
		Name: request.Name, Icon: request.Icon, ModelIDs: request.ModelIDs,
	})
	if err != nil {
		writeModelDisplayGroupError(c, err, "create model display group failed")
		return
	}
	response.Success(c, ModelDisplayGroupDataResponse{Group: toModelDisplayGroupResponse(*item)})
}

// UpdateModelDisplayGroup godoc
// @Summary 管理员更新模型展示分组
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "展示分组 ID"
// @Param body body UpdateModelDisplayGroupRequest true "展示分组参数"
// @Success 200 {object} ModelDisplayGroupDataResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Router /admin/llm/model-display-groups/{id} [patch]
func (h *Handler) UpdateModelDisplayGroup(c *gin.Context) {
	groupID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid model display group id")
		return
	}
	var request UpdateModelDisplayGroupRequest
	if err = c.ShouldBindJSON(&request); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.UpdateModelDisplayGroup(c.Request.Context(), groupID, appchannel.UpdateModelDisplayGroupInput{
		Name: request.Name, Icon: request.Icon, ModelIDs: request.ModelIDs,
	})
	if err != nil {
		writeModelDisplayGroupError(c, err, "update model display group failed")
		return
	}
	response.Success(c, ModelDisplayGroupDataResponse{Group: toModelDisplayGroupResponse(*item)})
}

// SetModelsDisplayGroup godoc
// @Summary 管理员批量设置模型展示分组
// @Description 在单个事务中将指定模型归入展示分组；displayGroupID 为 0 时恢复按技术厂商展示
// @Tags llm
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body SetModelsDisplayGroupRequest true "批量归组参数"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Router /admin/llm/models/display-group [patch]
func (h *Handler) SetModelsDisplayGroup(c *gin.Context) {
	var request SetModelsDisplayGroupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	if err := h.service.SetModelsDisplayGroup(c.Request.Context(), request.ModelIDs, *request.DisplayGroupID); err != nil {
		writeModelDisplayGroupError(c, err, "set model display group failed")
		return
	}
	response.Success(c, nil)
}

// DeleteModelDisplayGroup godoc
// @Summary 管理员删除模型展示分组
// @Description 删除展示分组后，关联模型恢复按技术厂商展示
// @Tags llm
// @Produce json
// @Security BearerAuth
// @Param id path int true "展示分组 ID"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Router /admin/llm/model-display-groups/{id} [delete]
func (h *Handler) DeleteModelDisplayGroup(c *gin.Context) {
	groupID, err := uintParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid model display group id")
		return
	}
	if err = h.service.DeleteModelDisplayGroup(c.Request.Context(), groupID); err != nil {
		writeModelDisplayGroupError(c, err, "delete model display group failed")
		return
	}
	response.Success(c, nil)
}

func writeModelVendorError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, appchannel.ErrInvalidModelVendor):
		response.Error(c, http.StatusBadRequest, "invalid model vendor")
	case errors.Is(err, appchannel.ErrModelVendorNotFound):
		response.Error(c, http.StatusNotFound, "model vendor not found")
	case errors.Is(err, appchannel.ErrModelVendorConflict):
		response.Error(c, http.StatusConflict, "model vendor already exists")
	default:
		response.Error(c, http.StatusInternalServerError, fallback)
	}
}

func writeModelDisplayGroupError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, appchannel.ErrInvalidModelDisplayGroup):
		response.Error(c, http.StatusBadRequest, "invalid model display group")
	case errors.Is(err, appchannel.ErrModelDisplayGroupNotFound):
		response.Error(c, http.StatusNotFound, "model display group not found")
	case errors.Is(err, appchannel.ErrModelDisplayGroupConflict):
		response.Error(c, http.StatusConflict, "model display group already exists")
	default:
		response.Error(c, http.StatusInternalServerError, fallback)
	}
}
