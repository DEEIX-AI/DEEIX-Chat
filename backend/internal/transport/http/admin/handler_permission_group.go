package admin

import (
	"errors"
	"net/http"
	"strconv"

	appadmin "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) permissionGroupIDParam(c *gin.Context) (uint, bool) {
	parsedID, err := strconv.ParseUint(c.Param("id"), 10, strconv.IntSize)
	if err != nil || parsedID == 0 {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidPermissionGroupID)
		return 0, false
	}
	return uint(parsedID), true
}

func (h *Handler) permissionGroupModelIDParam(c *gin.Context) (uint, bool) {
	parsedID, err := strconv.ParseUint(c.Param("modelID"), 10, strconv.IntSize)
	if err != nil || parsedID == 0 {
		response.ErrorFrom(c, http.StatusBadRequest, errInvalidModelID)
		return 0, false
	}
	return uint(parsedID), true
}

func (h *Handler) writePermissionGroupError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appadmin.ErrInvalidPermissionGroupName),
		errors.Is(err, appadmin.ErrInvalidPermissionGroupRateMultiplier),
		errors.Is(err, appadmin.ErrInvalidPermissionGroupModels),
		errors.Is(err, appadmin.ErrInvalidPermissionGroupUsers):
		response.ErrorFrom(c, http.StatusBadRequest, err)
	case errors.Is(err, appadmin.ErrDefaultPermissionGroupDeleteNotAllowed),
		errors.Is(err, appadmin.ErrDefaultPermissionGroupUsersImmutable),
		errors.Is(err, appadmin.ErrPermissionGroupReferencedByPlan):
		response.ErrorFrom(c, http.StatusConflict, err)
	case errors.Is(err, appadmin.ErrPermissionGroupNotFound):
		response.ErrorFrom(c, http.StatusNotFound, err)
	default:
		response.InternalError(c)
	}
}

// ListPermissionGroups godoc
// @Summary 管理员列出权限组
// @Description 返回全部模型访问权限组，默认组优先
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} PermissionGroupListResponseDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/permission-groups [get]
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

// CreatePermissionGroup godoc
// @Summary 管理员创建权限组
// @Description 创建模型访问权限组并设置计费倍率
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreatePermissionGroupRequest true "权限组参数"
// @Success 200 {object} PermissionGroupDataResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/permission-groups [post]
func (h *Handler) CreatePermissionGroup(c *gin.Context) {
	var req CreatePermissionGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	item, err := h.service.CreatePermissionGroup(c.Request.Context(), req.Name, req.Description, req.RateMultiplierPercent)
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, PermissionGroupDataResponse{Group: toPermissionGroupResponse(*item)})
}

// UpdatePermissionGroup godoc
// @Summary 管理员更新权限组
// @Description 更新权限组名称、说明与计费倍率
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "权限组ID"
// @Param body body UpdatePermissionGroupRequest true "权限组参数"
// @Success 200 {object} PermissionGroupDataResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/permission-groups/{id} [patch]
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

// DeletePermissionGroup godoc
// @Summary 管理员删除权限组
// @Description 删除权限组及其模型、用户关联；默认组和被套餐引用的权限组不可删除
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "权限组ID"
// @Success 200 {object} DeletePermissionGroupResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/permission-groups/{id} [delete]
func (h *Handler) DeletePermissionGroup(c *gin.Context) {
	groupID, ok := h.permissionGroupIDParam(c)
	if !ok {
		return
	}
	summary, err := h.service.DeletePermissionGroup(c.Request.Context(), groupID)
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, DeletePermissionGroupResponse{
		Deleted: true,
		Summary: toPermissionGroupDeleteSummaryResponse(*summary),
	})
}

// ListGroupModels godoc
// @Summary 管理员列出权限组模型
// @Description 返回权限组授权的平台模型 ID 集合
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "权限组ID"
// @Success 200 {object} GroupModelsResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/permission-groups/{id}/models [get]
func (h *Handler) ListGroupModels(c *gin.Context) {
	groupID, ok := h.permissionGroupIDParam(c)
	if !ok {
		return
	}
	ids, rules, err := h.service.ListGroupModels(c.Request.Context(), groupID)
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, GroupModelsResponse{
		ModelIDs: ids,
		Rules:    toPermissionGroupModelRuleResponses(rules),
	})
}

// SetGroupModels godoc
// @Summary 管理员设置权限组模型
// @Description 全量替换权限组授权的平台模型 ID 集合与动态访问规则
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "权限组ID"
// @Param body body SetGroupModelsRequest true "平台模型 ID 集合"
// @Success 200 {object} GroupModelsResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/permission-groups/{id}/models [put]
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
	if err := h.service.SetGroupModels(c.Request.Context(), groupID, req.ModelIDs, toPermissionGroupModelRules(req.Rules)); err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	ids, rules, err := h.service.ListGroupModels(c.Request.Context(), groupID)
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, GroupModelsResponse{
		ModelIDs: ids,
		Rules:    toPermissionGroupModelRuleResponses(rules),
	})
}

// ListModelPermissionGroups godoc
// @Summary 管理员列出模型权限组
// @Description 返回平台模型的手动权限组与动态规则命中的有效权限组
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param modelID path int true "平台模型ID"
// @Success 200 {object} ModelPermissionGroupsResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/models/{modelID}/permission-groups [get]
func (h *Handler) ListModelPermissionGroups(c *gin.Context) {
	modelID, ok := h.permissionGroupModelIDParam(c)
	if !ok {
		return
	}
	manualGroupIDs, matchedGroupIDs, effectiveGroupIDs, unassigned, err := h.service.ListModelPermissionGroups(c.Request.Context(), modelID)
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, ModelPermissionGroupsResponse{
		ManualGroupIDs:    manualGroupIDs,
		MatchedGroupIDs:   matchedGroupIDs,
		EffectiveGroupIDs: effectiveGroupIDs,
		Unassigned:        unassigned,
	})
}

// SetModelPermissionGroups godoc
// @Summary 管理员设置模型手动权限组
// @Description 全量替换平台模型的手动权限组，不影响权限组动态规则
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param modelID path int true "平台模型ID"
// @Param body body SetModelPermissionGroupsRequest true "权限组 ID 集合"
// @Success 200 {object} ModelPermissionGroupsResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/models/{modelID}/permission-groups [put]
func (h *Handler) SetModelPermissionGroups(c *gin.Context) {
	modelID, ok := h.permissionGroupModelIDParam(c)
	if !ok {
		return
	}
	var req SetModelPermissionGroupsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	if err := h.service.SetModelPermissionGroups(c.Request.Context(), modelID, req.GroupIDs); err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	manualGroupIDs, matchedGroupIDs, effectiveGroupIDs, unassigned, err := h.service.ListModelPermissionGroups(c.Request.Context(), modelID)
	if err != nil {
		h.writePermissionGroupError(c, err)
		return
	}
	response.Success(c, ModelPermissionGroupsResponse{
		ManualGroupIDs:    manualGroupIDs,
		MatchedGroupIDs:   matchedGroupIDs,
		EffectiveGroupIDs: effectiveGroupIDs,
		Unassigned:        unassigned,
	})
}

// ListGroupUsers godoc
// @Summary 管理员列出权限组用户
// @Description 返回权限组内的用户 ID 集合
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "权限组ID"
// @Success 200 {object} GroupUsersResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/permission-groups/{id}/users [get]
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

// SetGroupUsers godoc
// @Summary 管理员设置权限组用户
// @Description 全量替换权限组内的用户 ID 集合
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "权限组ID"
// @Param body body SetGroupUsersRequest true "用户 ID 集合"
// @Success 200 {object} GroupUsersResponseDoc
// @Failure 400 {object} ErrorDoc
// @Failure 404 {object} ErrorDoc
// @Failure 409 {object} ErrorDoc
// @Failure 500 {object} ErrorDoc
// @Router /admin/permission-groups/{id}/users [put]
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
