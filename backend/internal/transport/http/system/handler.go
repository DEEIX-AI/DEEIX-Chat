package system

import (
	"github.com/gin-gonic/gin"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
)

// Handler 处理系统级公开请求。
type Handler struct {
	runtime *config.Runtime
}

// NewHandler 创建 system HTTP 处理器。
func NewHandler(runtime *config.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

// GetCapabilities godoc
// @Summary 查询服务器能力声明
// @Description 返回这台服务器提供哪些功能。客户端据此决定显示什么；能力关闭的功能其端点返回 404 feature.disabled。
// @Tags system
// @Produce json
// @Success 200 {object} CapabilitiesResponseDoc
// @Router /capabilities [get]
func (h *Handler) GetCapabilities(c *gin.Context) {
	// 能力位由部署形态决定，变化以天计；允许中间层与其他客户端缓存。
	c.Header("Cache-Control", "public, max-age=300")
	response.Success(c, CapabilitiesResponse{Features: h.runtime.Snapshot().Capabilities()})
}
