package channel

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

const maxModelIconUploadRequestBytes = appchannel.MaxModelIconBytes + 256*1024

// UploadModelIconAsset godoc
// @Summary 管理员上传模型展示图标
// @Description 上传 PNG、JPEG 或 WebP 图标；后端校验内容、尺寸并按 SHA-256 去重
// @Tags llm
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "图标文件，最大 1 MiB"
// @Success 200 {object} ModelIconAssetResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 413 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/icon-assets [post]
func (h *Handler) UploadModelIconAsset(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxModelIconUploadRequestBytes)
	header, err := c.FormFile("file")
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			response.ErrorFrom(c, http.StatusRequestEntityTooLarge, errModelIconFileTooLarge)
			return
		}
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidModelIconFile)
		return
	}
	file, err := header.Open()
	if err != nil {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidModelIconFile)
		return
	}
	defer file.Close()

	item, err := h.service.UploadModelIconAsset(c.Request.Context(), middleware.MustUserID(c), file)
	if err != nil {
		switch {
		case errors.Is(err, appchannel.ErrModelIconFileTooLarge):
			response.ErrorFrom(c, http.StatusRequestEntityTooLarge, err)
		case errors.Is(err, appchannel.ErrInvalidModelIconFile):
			response.ErrorFrom(c, http.StatusBadRequest, err)
		default:
			response.InternalError(c)
		}
		return
	}
	response.Success(c, toModelIconAssetResponse(*item))
}

// ListModelIconAssets godoc
// @Summary 管理员查询已上传的模型展示图标
// @Description 分页查询仍在图标库中的已上传图标，按上传时间倒序返回
// @Tags llm
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} ModelIconAssetListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/llm/icon-assets [get]
func (h *Handler) ListModelIconAssets(c *gin.Context) {
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	items, total, err := h.service.ListModelIconAssets(c.Request.Context(), page, pageSize)
	if err != nil {
		response.InternalError(c)
		return
	}
	results := make([]ModelIconAssetListItemResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toModelIconAssetListItemResponse(item))
	}
	response.SuccessPage(c, total, results)
}

// DeleteModelIconAsset godoc
// @Summary 管理员从图标库移除已上传图标
// @Description 仅允许移除无引用图标；立即从图标库隐藏，持续 24 小时无引用后物理清理
// @Tags llm
// @Produce json
// @Security BearerAuth
// @Param public_id path string true "图标公开 ID"
// @Success 200 {object} response.SuccessDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ModelIconAssetDeleteConflictDoc
// @Router /admin/llm/icon-assets/{public_id} [delete]
func (h *Handler) DeleteModelIconAsset(c *gin.Context) {
	err := h.service.RequestModelIconAssetDeletion(c.Request.Context(), c.Param("public_id"))
	if err == nil {
		response.Success(c, nil)
		return
	}
	var inUse *appchannel.ModelIconAssetInUseError
	if errors.As(err, &inUse) {
		references := inUse.References
		response.ErrorWithDetails(c, http.StatusConflict, "llm.model_icon_asset_in_use", ModelIconAssetDeleteConflictDetails{
			ReferenceCount: references.Total(), Models: references.Models, Vendors: references.Vendors,
			DisplayGroups: references.DisplayGroups, ConversationRuns: references.ConversationRuns,
		})
		return
	}
	if errors.Is(err, appchannel.ErrModelIconAssetNotFound) {
		response.ErrorFrom(c, http.StatusNotFound, err)
		return
	}
	response.InternalError(c)
}

// GetModelIconAsset godoc
// @Summary 读取模型展示图标
// @Description 公开读取经过后端校验的不可变模型展示图标
// @Tags llm
// @Produce image/png,image/jpeg,image/webp
// @Param public_id path string true "图标公开 ID"
// @Success 200 {file} binary
// @Failure 404 {object} ErrorDoc
// @Router /llm/icon-assets/{public_id} [get]
func (h *Handler) GetModelIconAsset(c *gin.Context) {
	info, err := h.service.GetModelIconAssetInfo(c.Request.Context(), c.Param("public_id"))
	if err != nil {
		if errors.Is(err, appchannel.ErrModelIconAssetNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
		return
	}

	item, err := h.service.OpenModelIconAsset(c.Request.Context(), *info)
	if err != nil {
		if errors.Is(err, appchannel.ErrModelIconAssetNotFound) {
			response.ErrorFrom(c, http.StatusNotFound, err)
			return
		}
		response.InternalError(c)
		return
	}
	defer item.Reader.Close()

	etag := fmt.Sprintf("\"%s\"", info.SHA256)
	if c.GetHeader("If-None-Match") == etag {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Header("ETag", etag)
		c.Status(http.StatusNotModified)
		return
	}

	c.Header("Content-Type", item.ContentType)
	c.Header("Content-Disposition", "inline")
	c.Header("Content-Length", fmt.Sprintf("%d", item.SizeBytes))
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("ETag", etag)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cross-Origin-Resource-Policy", "cross-origin")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, item.Reader)
}

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
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	items, total, err := h.service.ListModelVendors(c.Request.Context(), page, pageSize, c.Query("q"))
	if err != nil {
		response.InternalError(c)
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
		writeModelVendorError(c, err)
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
		writeModelVendorError(c, err)
		return
	}
	response.Success(c, ModelVendorDataResponse{Vendor: toModelVendorResponse(*item)})
}

// DeleteModelVendor godoc
// @Summary 管理员删除自定义模型技术厂商
// @Description 仅允许删除未被平台模型引用的非内置厂商；冲突响应包含关联模型预览
// @Tags llm
// @Produce json
// @Security BearerAuth
// @Param key path string true "技术厂商 key"
// @Success 200 {object} response.SuccessDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ModelVendorDeleteConflictDoc
// @Router /admin/llm/model-vendors/{key} [delete]
func (h *Handler) DeleteModelVendor(c *gin.Context) {
	err := h.service.DeleteModelVendor(c.Request.Context(), strings.TrimSpace(c.Param("key")))
	if err == nil {
		response.Success(c, nil)
		return
	}
	var blocked *appchannel.ModelVendorDeleteBlockedError
	if errors.As(err, &blocked) {
		models := make([]ModelVendorReferenceResponse, 0, len(blocked.Models))
		for _, model := range blocked.Models {
			models = append(models, ModelVendorReferenceResponse{
				ID: model.ID, PlatformModelName: model.PlatformModelName,
			})
		}
		code := "llm.model_vendor_in_use"
		if blocked.Reason == appchannel.ModelVendorDeleteReasonBuiltIn {
			code = "llm.model_vendor_builtin"
		}
		response.ErrorWithDetails(c, http.StatusConflict, code, ModelVendorDeleteConflictDetails{
			Reason: blocked.Reason, ReferenceCount: blocked.ReferenceCount, Models: models,
		})
		return
	}
	writeModelVendorError(c, err)
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
	page, pageSize := pagination.Parse(c.Query("page"), c.Query("page_size"))
	items, total, err := h.service.ListModelDisplayGroups(c.Request.Context(), page, pageSize, c.Query("q"))
	if err != nil {
		response.InternalError(c)
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
		writeModelDisplayGroupError(c, err)
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
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidModelDisplayGroupID)
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
		writeModelDisplayGroupError(c, err)
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
		writeModelDisplayGroupError(c, err)
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
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidModelDisplayGroupID)
		return
	}
	if err = h.service.DeleteModelDisplayGroup(c.Request.Context(), groupID); err != nil {
		writeModelDisplayGroupError(c, err)
		return
	}
	response.Success(c, nil)
}

func writeModelVendorError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appchannel.ErrInvalidModelVendor):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	case errors.Is(err, appchannel.ErrModelVendorNotFound):
		response.ErrorFrom(c, http.StatusNotFound, errModelVendorNotFound)
	case errors.Is(err, appchannel.ErrModelVendorConflict):
		response.ErrorFrom(c, http.StatusConflict, err)
	case errors.Is(err, appchannel.ErrModelIconAssetNotFound):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	case errors.Is(err, appchannel.ErrInvalidModelIconReference):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	default:
		response.InternalError(c)
	}
}

func writeModelDisplayGroupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appchannel.ErrInvalidModelDisplayGroup):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	case errors.Is(err, appchannel.ErrModelDisplayGroupNotFound):
		response.ErrorFrom(c, http.StatusNotFound, errModelDisplayGroupNotFound)
	case errors.Is(err, appchannel.ErrModelDisplayGroupConflict):
		response.ErrorFrom(c, http.StatusConflict, err)
	case errors.Is(err, appchannel.ErrModelIconAssetNotFound):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	case errors.Is(err, appchannel.ErrInvalidModelIconReference):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	default:
		response.InternalError(c)
	}
}
