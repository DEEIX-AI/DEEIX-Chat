package contentmoderation

import (
	"github.com/gin-gonic/gin"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
)

// RegisterRoutes 在管理端路由组下注册路由。
func (m *Module) RegisterRoutes(adminGroup *gin.RouterGroup, gate middleware.FeatureGate) {
	if m == nil || m.Handler == nil {
		return
	}
	group := adminGroup.Group("/content-moderation", gate.Require("contentModeration"))
	group.GET("/config", m.Handler.GetConfig)
	group.PUT("/config", m.Handler.UpdateConfig)
	group.POST("/probe", m.Handler.Probe)
	group.GET("/stats", m.Handler.GetStats)
	group.GET("/events", m.Handler.ListEvents)
	group.GET("/events/:eventID", m.Handler.GetEvent)
	group.GET("/events/:eventID/images/:index", m.Handler.GetEventImage)
}
