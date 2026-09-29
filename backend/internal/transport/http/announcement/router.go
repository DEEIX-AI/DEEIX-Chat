package announcement

import (
	"github.com/gin-gonic/gin"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
)

// RegisterRoutes 注册公告用户侧路由。
// 列表接口在能力关闭时返回空列表而不是 404：客户端会轮询它，老版本把 404 当成错误。
// 这是能力契约（docs/ARCHITECTURE.md §4）里唯一的例外。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup, gate middleware.FeatureGate) {
	authRequired.GET("/announcements", func(c *gin.Context) {
		if !gate.Enabled("announcements") {
			response.Success(c, []AnnouncementResponse{})
			return
		}
		m.Handler.ListAnnouncements(c)
	})

	announcements := authRequired.Group("", gate.Require("announcements"))
	announcements.POST("/announcements/:id/dismiss-today", m.Handler.DismissAnnouncementToday)
	announcements.POST("/announcements/:id/close", m.Handler.CloseAnnouncement)
}

// RegisterAdminRoutes 注册公告管理路由。
func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup, gate middleware.FeatureGate) {
	announcements := adminGroup.Group("", gate.Require("announcements"))
	announcements.GET("/announcements", m.Handler.ListAdminAnnouncements)
	announcements.POST("/announcements", m.Handler.CreateAnnouncement)
	announcements.PATCH("/announcements/:id", m.Handler.PatchAnnouncement)
	announcements.DELETE("/announcements/:id", m.Handler.DeleteAnnouncement)
}
