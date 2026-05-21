package usersettings

import (
	"net/http"

	appusersettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/usersettings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// Handler 封装用户配置 HTTP 处理。
type Handler struct {
	service *appusersettings.Service
}

// NewHandler 创建处理器。
func NewHandler(service *appusersettings.Service) *Handler {
	return &Handler{service: service}
}

// GetSettings godoc
// @Summary 获取当前用户的配置
// @Description 返回当前用户全部个人偏好配置，缺失项以默认值填充
// @Tags user/settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserSettingsResponseDoc
// @Failure 500 {object} response.Envelope
// @Router /user/settings [get]
func (h *Handler) GetSettings(c *gin.Context) {
	userID := middleware.MustUserID(c)
	data, err := h.service.ListSettings(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "load settings failed")
		return
	}
	response.Success(c, UserSettingsResponse{Settings: data})
}

// PatchSettings godoc
// @Summary 更新当前用户的配置
// @Description 批量更新用户个人偏好配置，返回更新后的全量配置
// @Tags user/settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body PatchSettingsRequest true "更新项"
// @Success 200 {object} UserSettingsResponseDoc
// @Failure 400 {object} response.Envelope
// @Failure 500 {object} response.Envelope
// @Router /user/settings [patch]
func (h *Handler) PatchSettings(c *gin.Context) {
	var req PatchSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}

	userID := middleware.MustUserID(c)
	data, err := h.service.PatchSettings(c.Request.Context(), userID, req.Settings)
	if err != nil {
		if appusersettings.IsValidationError(err) {
			response.ErrorFrom(c, http.StatusBadRequest, err)
		} else {
			response.Error(c, http.StatusInternalServerError, "settings update failed")
		}
		return
	}
	response.Success(c, UserSettingsResponse{Settings: data})
}
