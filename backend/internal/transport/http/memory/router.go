package memory

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册记忆域路由。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup) {
	authRequired.GET("/memories/profile", m.Handler.ListUserMemories)
	authRequired.PUT("/memories/profile", m.Handler.UpsertUserMemory)
	authRequired.DELETE("/memories/profile/:memory_key", m.Handler.DeleteUserMemory)
}
