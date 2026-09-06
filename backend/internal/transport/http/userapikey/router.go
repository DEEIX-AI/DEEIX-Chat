package userapikey

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册用户 API Key 路由。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup) {
	authRequired.GET("/me/api-keys", m.Handler.ListUserAPIKeys)
	authRequired.POST("/me/api-keys", m.Handler.CreateUserAPIKey)
	authRequired.DELETE("/me/api-keys/:id", m.Handler.RevokeUserAPIKey)
}
