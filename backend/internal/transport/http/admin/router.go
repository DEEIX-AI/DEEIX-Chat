package admin

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册后台管理路由（由管理员中间件保护）。
func (m *Module) RegisterRoutes(adminGroup *gin.RouterGroup) {
	adminGroup.POST("/users", m.Handler.CreateUser)
	adminGroup.GET("/users", m.Handler.ListUsers)
	adminGroup.POST("/users/import/openwebui", m.Handler.ImportOpenWebUIUsers)
	adminGroup.PATCH("/users/:id", m.Handler.PatchUser)
	adminGroup.PATCH("/users/:id/status", m.Handler.UpdateUserStatus)
	adminGroup.POST("/users/:id/reset-password", m.Handler.ResetUserPassword)
	adminGroup.POST("/users/:id/reset-2fa", m.Handler.ResetUserTwoFactor)
	adminGroup.POST("/users/:id/revoke-sessions", m.Handler.RevokeUserSessions)
	adminGroup.DELETE("/users/:id", m.Handler.DeleteUser)
	adminGroup.GET("/user-auth-events", m.Handler.ListUserAuthEvents)
	adminGroup.GET("/audit-logs", m.Handler.ListAuditLogs)
	adminGroup.GET("/call-logs", m.Handler.ListUsageLogs)
	adminGroup.GET("/payment-orders", m.Handler.ListPaymentOrders)
	adminGroup.GET("/conversation-events", m.Handler.ListConversationEvents)
	adminGroup.GET("/system-events", m.Handler.ListSystemEvents)
	adminGroup.POST("/logs/cleanup", m.Handler.CleanupLogs)
	adminGroup.GET("/conversations/export", m.Handler.ExportConversations)
	adminGroup.GET("/permission-groups", m.Handler.ListPermissionGroups)
	adminGroup.POST("/permission-groups", m.Handler.CreatePermissionGroup)
	adminGroup.PATCH("/permission-groups/:id", m.Handler.UpdatePermissionGroup)
	adminGroup.DELETE("/permission-groups/:id", m.Handler.DeletePermissionGroup)
	adminGroup.GET("/permission-groups/:id/models", m.Handler.ListGroupModels)
	adminGroup.PUT("/permission-groups/:id/models", m.Handler.SetGroupModels)
	adminGroup.GET("/permission-groups/:id/users", m.Handler.ListGroupUsers)
	adminGroup.PUT("/permission-groups/:id/users", m.Handler.SetGroupUsers)
}
