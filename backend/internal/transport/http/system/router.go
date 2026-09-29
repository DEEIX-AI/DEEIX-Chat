package system

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册不需要登录的系统路由。
func (m *Module) RegisterPublicRoutes(public *gin.RouterGroup) {
	public.GET("/capabilities", m.Handler.GetCapabilities)
}
