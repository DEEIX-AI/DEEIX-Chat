package user

import appuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"

// Handler 预留用户域 HTTP 处理器（当前用户管理在 admin 模块暴露）。
type Handler struct {
	service *appuser.Service
}

// NewHandler 创建处理器。
func NewHandler(service *appuser.Service) *Handler {
	return &Handler{service: service}
}
