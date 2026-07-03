package admin

import (
	"errors"
	"net/http"
	"strconv"

	appadmin "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) permissionGroupIDParam(c *gin.Context) (uint, bool) {
	parsedID, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if err != nil || parsedID == 0 {
		response.Error(c, http.StatusBadRequest, "invalid permission group id")
		return 0, false
	}
	return uint(parsedID), true
}

func (h *Handler) writePermissionGroupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appadmin.ErrInvalidPermissionGroupCode),
		errors.Is(err, appadmin.ErrInvalidPermissionGroupName),
		errors.Is(err, appadmin.ErrInvalidPermissionGroupRateMultiplier):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	case errors.Is(err, appadmin.ErrDefaultPermissionGroupDeleteNotAllowed),
		errors.Is(err, appadmin.ErrPermissionGroupReferencedByPlan):
		response.ErrorFrom(c, http.StatusConflict, err)
	case errors.Is(err, repository.ErrNotFound):
		response.Error(c, http.StatusNotFound, "permission group not found")
	default:
		response.Error(c, http.StatusInternalServerError, "permission group operation failed")
	}
}

// ListPermissionGroups 列出全部权限组。
func (h *Handler) ListPermissionGroups(c *gin.Context) {
	items, err := h.service.ListPermissionGroups(c.Request.Context())
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	results := make([]PermissionGroupResponse, 0, len(items))
	for _, item := range items {
		results = append(results, toPermissionGroupResponse(item))
	}
	response.Success(c, PermissionGroupListResponse{Results: results})
}

// CreatePermissionGroup 创建权限组。
func (h *Handler) CreatePermissionGroup(c *gin.Context) {
	var req CreatePermissionGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CreatePermissionGroup(c.Request.Context(), req.Code, req.Name, req.Description, req.RateMultiplierPercent)
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, PermissionGroupDataResponse{Group: toPermissionGroupResponse(*item)})
}

// UpdatePermissionGroup 更新权限组名称与说明。
func (h *Handler) UpdatePermissionGroup(c *gin.Context) {
	groupID, ok := h.permissionGroupIDParam(c)
	if !ok {
		return
	}
	var req UpdatePermissionGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.UpdatePermissionGroup(c.Request.Context(), groupID, req.Name, req.Description, req.RateMultiplierPercent)
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, PermissionGroupDataResponse{Group: toPermissionGroupResponse(*item)})
}

// DeletePermissionGroup 删除权限组。
func (h *Handler) DeletePermissionGroup(c *gin.Context) {
	groupID, ok := h.permissionGroupIDParam(c)
	if !ok {
		return
	}
	if err := h.service.DeletePermissionGroup(c.Request.Context(), groupID); err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, DeletePermissionGroupResponse{Deleted: true})
}

// ListGroupModels 列出权限组授权的平台模型 ID。
func (h *Handler) ListGroupModels(c *gin.Context) {
	groupID, ok := h.permissionGroupIDParam(c)
	if !ok {
		return
	}
	ids, err := h.service.ListGroupModels(c.Request.Context(), groupID)
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, GroupModelsResponse{ModelIDs: ids})
}

// SetGroupModels 全量替换权限组授权的平台模型集合。
func (h *Handler) SetGroupModels(c *gin.Context) {
	groupID, ok := h.permissionGroupIDParam(c)
	if !ok {
		return
	}
	var req SetGroupModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	if err := h.service.SetGroupModels(c.Request.Context(), groupID, req.ModelIDs); err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	ids, err := h.service.ListGroupModels(c.Request.Context(), groupID)
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, GroupModelsResponse{ModelIDs: ids})
}

// ListGroupUsers 列出权限组内的用户 ID。
func (h *Handler) ListGroupUsers(c *gin.Context) {
	groupID, ok := h.permissionGroupIDParam(c)
	if !ok {
		return
	}
	ids, err := h.service.ListGroupUsers(c.Request.Context(), groupID)
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, GroupUsersResponse{UserIDs: ids})
}

// SetGroupUsers 全量替换权限组内的用户集合。
func (h *Handler) SetGroupUsers(c *gin.Context) {
	groupID, ok := h.permissionGroupIDParam(c)
	if !ok {
		return
	}
	var req SetGroupUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	if err := h.service.SetGroupUsers(c.Request.Context(), groupID, req.UserIDs); err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	ids, err := h.service.ListGroupUsers(c.Request.Context(), groupID)
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, GroupUsersResponse{UserIDs: ids})
}
