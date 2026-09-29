package admin

import (
	"github.com/gin-gonic/gin"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
)

// RegisterRoutes 注册后台管理路由（由管理员中间件保护）。
// 订单与兑换记录属于计费门禁，账号与权限组只在多用户部署里有对象；能力键的含义见 docs/ARCHITECTURE.md §4。
func (m *Module) RegisterRoutes(adminGroup *gin.RouterGroup, gate middleware.FeatureGate) {
	adminGroup.GET("/audit-logs", m.Handler.ListAuditLogs)
	adminGroup.GET("/usage-statistics", m.Handler.GetUsageStatistics)
	adminGroup.GET("/call-logs", m.Handler.ListUsageLogs)
	adminGroup.GET("/conversation-events", m.Handler.ListConversationEvents)
	adminGroup.GET("/conversation-events/:id", m.Handler.GetConversationEvent)
	adminGroup.POST("/logs/cleanup", m.Handler.CleanupLogs)
	adminGroup.GET("/conversations/export", m.Handler.ExportConversations)

	billingGating := adminGroup.Group("", gate.Require("billingGating"))
	billingGating.GET("/payment-orders", m.Handler.ListPaymentOrders)
	billingGating.GET("/redemptions", m.Handler.ListRedemptions)

	multiUser := adminGroup.Group("", gate.Require("multiUser"))
	multiUser.POST("/users", m.Handler.CreateUser)
	multiUser.GET("/users", m.Handler.ListUsers)
	multiUser.POST("/users/import/openwebui", m.Handler.ImportOpenWebUIUsers)
	multiUser.PATCH("/users/:id", m.Handler.PatchUser)
	multiUser.PATCH("/users/:id/status", m.Handler.UpdateUserStatus)
	multiUser.POST("/users/:id/reset-password", m.Handler.ResetUserPassword)
	multiUser.POST("/users/:id/reset-2fa", m.Handler.ResetUserTwoFactor)
	multiUser.POST("/users/:id/revoke-sessions", m.Handler.RevokeUserSessions)
	multiUser.DELETE("/users/:id", m.Handler.DeleteUser)
	multiUser.GET("/user-auth-events", m.Handler.ListUserAuthEvents)
	multiUser.GET("/permission-groups", m.Handler.ListPermissionGroups)
	multiUser.POST("/permission-groups", m.Handler.CreatePermissionGroup)
	multiUser.PATCH("/permission-groups/:id", m.Handler.UpdatePermissionGroup)
	multiUser.DELETE("/permission-groups/:id", m.Handler.DeletePermissionGroup)
	multiUser.GET("/permission-groups/:id/models", m.Handler.ListGroupModels)
	multiUser.PUT("/permission-groups/:id/models", m.Handler.SetGroupModels)
	multiUser.GET("/models/:modelID/permission-groups", m.Handler.ListModelPermissionGroups)
	multiUser.PUT("/models/:modelID/permission-groups", m.Handler.SetModelPermissionGroups)
	multiUser.GET("/permission-groups/:id/users", m.Handler.ListGroupUsers)
	multiUser.PUT("/permission-groups/:id/users", m.Handler.SetGroupUsers)
}
