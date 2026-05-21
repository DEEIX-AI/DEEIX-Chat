package usersettings

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册用户配置路由（需要登录）。
func (m *Module) RegisterRoutes(authGroup *gin.RouterGroup) {
	g := authGroup.Group("/user/settings")
	g.GET("", m.Handler.GetSettings)
	g.PATCH("", m.Handler.PatchSettings)
}
