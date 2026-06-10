package admin

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册后台管理路由（由管理员中间件保护）。
func (m *Module) RegisterRoutes(adminGroup *gin.RouterGroup) {
	adminGroup.POST("/users", m.Handler.CreateUser)
	adminGroup.GET("/users", m.Handler.ListUsers)
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
}
