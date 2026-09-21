package uicomponent

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册组件用户侧路由。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup) {
	authRequired.GET("/ui-components", m.Handler.ListVisible)
	authRequired.GET("/ui-components/mine", m.Handler.ListMine)
	authRequired.POST("/ui-components/mine", m.Handler.CreateMine)
	authRequired.PATCH("/ui-components/mine/:id", m.Handler.PatchMine)
	authRequired.DELETE("/ui-components/mine/:id", m.Handler.DeleteMine)
}

// RegisterAdminRoutes 注册组件管理路由。
func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup) {
	adminGroup.GET("/ui-components", m.Handler.ListAdmin)
	adminGroup.POST("/ui-components", m.Handler.CreateAdmin)
	adminGroup.PATCH("/ui-components/:id", m.Handler.PatchAdmin)
	adminGroup.DELETE("/ui-components/:id", m.Handler.DeleteAdmin)
}
