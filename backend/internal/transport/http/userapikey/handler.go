package userapikey

import (
	"net/http"
	"time"

	appuserapikey "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/userapikey"
	domainuserapikey "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/userapikey"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// Handler 封装用户 API Key HTTP 处理。
type Handler struct {
	service *appuserapikey.Service
}

// NewHandler 创建处理器。
func NewHandler(service *appuserapikey.Service) *Handler {
	return &Handler{service: service}
}

// CreateUserAPIKeyRequest 创建 API Key 请求。
type CreateUserAPIKeyRequest struct {
	Name string `json:"name" binding:"required,max=128"`
}

// UserAPIKeyResponse 列表/详情响应（不含明文）。
type UserAPIKeyResponse struct {
	PublicID   string     `json:"publicId"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"keyPrefix"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// CreateUserAPIKeyResponse 创建响应（含一次性明文）。
type CreateUserAPIKeyResponse struct {
	UserAPIKeyResponse
	Key string `json:"key"`
}

// ListUserAPIKeys godoc
// @Summary 列出当前用户的 API Key
// @Tags user-api-keys
// @Security BearerAuth
// @Success 200 {object} UserAPIKeyListResponseDoc
// @Router /me/api-keys [get]
func (h *Handler) ListUserAPIKeys(c *gin.Context) {
	items, err := h.service.List(c.Request.Context(), middleware.MustUserID(c))
	if err != nil {
		handleUserAPIKeyError(c, err)
		return
	}
	views := make([]UserAPIKeyResponse, 0, len(items))
	for _, item := range items {
		views = append(views, toUserAPIKeyResponse(item))
	}
	response.Success(c, views)
}

// CreateUserAPIKey godoc
// @Summary 创建 API Key（明文仅返回一次）
// @Tags user-api-keys
// @Security BearerAuth
// @Param body body CreateUserAPIKeyRequest true "参数"
// @Success 200 {object} CreateUserAPIKeyResponseDoc
// @Router /me/api-keys [post]
func (h *Handler) CreateUserAPIKey(c *gin.Context) {
	var req CreateUserAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.InvalidRequestBody(c, err)
		return
	}
	result, err := h.service.Create(c.Request.Context(), middleware.MustUserID(c), req.Name)
	if err != nil {
		handleUserAPIKeyError(c, err)
		return
	}
	response.Success(c, CreateUserAPIKeyResponse{
		UserAPIKeyResponse: toUserAPIKeyResponse(result.Key),
		Key:                result.Plaintext,
	})
}

// RevokeUserAPIKey godoc
// @Summary 吊销 API Key
// @Tags user-api-keys
// @Security BearerAuth
// @Param id path string true "API Key public ID"
// @Success 200 {object} RevokeUserAPIKeyResponseDoc
// @Router /me/api-keys/{id} [delete]
func (h *Handler) RevokeUserAPIKey(c *gin.Context) {
	publicID := c.Param("id")
	if err := h.service.Revoke(c.Request.Context(), middleware.MustUserID(c), publicID); err != nil {
		handleUserAPIKeyError(c, err)
		return
	}
	response.Success(c, map[string]bool{"revoked": true})
}

func handleUserAPIKeyError(c *gin.Context, err error) {
	switch {
	case err == appuserapikey.ErrDisabled:
		response.ErrorFrom(c, http.StatusForbidden, err)
	case err == appuserapikey.ErrInvalidName, err == appuserapikey.ErrKeyLimitReached:
		response.ErrorFrom(c, http.StatusBadRequest, err)
	case err == appuserapikey.ErrKeyNotFound:
		response.ErrorFrom(c, http.StatusNotFound, err)
	default:
		response.InternalError(c)
	}
}

func toUserAPIKeyResponse(item domainuserapikey.UserAPIKey) UserAPIKeyResponse {
	return UserAPIKeyResponse{
		PublicID:   item.PublicID,
		Name:       item.Name,
		KeyPrefix:  item.KeyPrefix,
		LastUsedAt: item.LastUsedAt,
		RevokedAt:  item.RevokedAt,
		ExpiresAt:  item.ExpiresAt,
		CreatedAt:  item.CreatedAt,
	}
}

// UserAPIKeyListResponseDoc swagger 文档。
type UserAPIKeyListResponseDoc struct {
	ErrorMsg string               `json:"errorMsg"`
	Data     []UserAPIKeyResponse `json:"data"`
}

// CreateUserAPIKeyResponseDoc swagger 文档。
type CreateUserAPIKeyResponseDoc struct {
	ErrorMsg string                   `json:"errorMsg"`
	Data     CreateUserAPIKeyResponse `json:"data"`
}

// RevokeUserAPIKeyResponseDoc swagger 文档。
type RevokeUserAPIKeyResponseDoc struct {
	ErrorMsg string         `json:"errorMsg"`
	Data     map[string]any `json:"data"`
}

// ErrorDoc 错误响应。
type ErrorDoc struct {
	ErrorMsg  string `json:"errorMsg"`
	ErrorCode string `json:"errorCode,omitempty"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	Data      any    `json:"data"`
}
