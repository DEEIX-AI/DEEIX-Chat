package announcement

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册公告用户侧路由。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup) {
	authRequired.GET("/announcements", m.Handler.ListAnnouncements)
	authRequired.POST("/announcements/:id/dismiss-today", m.Handler.DismissAnnouncementToday)
	authRequired.POST("/announcements/:id/close", m.Handler.CloseAnnouncement)
}

// RegisterAdminRoutes 注册公告管理路由。
func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup) {
	adminGroup.GET("/announcements", m.Handler.ListAdminAnnouncements)
	adminGroup.POST("/announcements/generate", m.Handler.GenerateAnnouncementDraft)
	adminGroup.POST("/announcements", m.Handler.CreateAnnouncement)
	adminGroup.PATCH("/announcements/:id", m.Handler.PatchAnnouncement)
	adminGroup.DELETE("/announcements/:id", m.Handler.DeleteAnnouncement)
}
