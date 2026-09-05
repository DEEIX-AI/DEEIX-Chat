package user

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册用户域公开路由。
func (m *Module) RegisterPublicRoutes(public *gin.RouterGroup) {
	public.GET("/users/:public_id/avatar", m.Handler.GetAvatar)
}

// RegisterRoutes 注册用户域登录态路由。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup) {
	authRequired.GET("/user/stats/activity", m.Handler.GetDailyActivity)
}
