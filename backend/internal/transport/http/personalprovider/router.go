package personalprovider

import (
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// probeRequestsPerMinute 限制每个用户每分钟让服务器请求外部地址的次数（检测、添加、更换 Key、刷新模型）。
const probeRequestsPerMinute = 10

// Module 聚合个人模型服务 HTTP 处理器。
type Module struct {
	Handler *Handler
	limiter middleware.RateLimiter
}

// NewModule 创建模块。limiter 为 nil 时不限速（仅用于测试）。
func NewModule(handler *Handler, limiter middleware.RateLimiter) *Module {
	return &Module{Handler: handler, limiter: limiter}
}

// RegisterRoutes 注册用户侧路由。会让服务器访问外部地址的接口额外挂一层始终启用的限速。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup) {
	probeLimit := middleware.UpstreamProbeRateLimit(m.limiter, probeRequestsPerMinute)
	group := authRequired.Group("/me/model-providers")
	group.GET("/access", m.Handler.GetAccess)
	group.GET("", m.Handler.List)
	group.POST("/probe", probeLimit, m.Handler.Probe)
	group.POST("", probeLimit, m.Handler.Create)
	group.PATCH("/:id", probeLimit, m.Handler.Update)
	group.DELETE("/:id", m.Handler.Delete)
	group.GET("/:id/models", probeLimit, m.Handler.ListModels)
}

// RegisterAdminRoutes 注册管理员治理路由。
func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup) {
	adminGroup.GET("/model-providers", m.Handler.AdminList)
	adminGroup.POST("/model-providers/suspend", m.Handler.AdminSetSuspended)
	adminGroup.POST("/model-providers/suspend-host", m.Handler.AdminSuspendHost)
	adminGroup.POST("/model-providers/batch-delete", m.Handler.AdminDelete)
}
