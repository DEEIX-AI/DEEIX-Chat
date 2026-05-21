package audit

import appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"

// Handler 预留审计域 HTTP 处理器。
type Handler struct{}

// NewHandler 创建处理器。
func NewHandler(_ *appaudit.Service) *Handler {
	return &Handler{}
}
