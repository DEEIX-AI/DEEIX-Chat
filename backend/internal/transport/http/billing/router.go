package billing

import (
	"github.com/gin-gonic/gin"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
)

// 计量（用量、价格、概览）始终可用；门禁（套餐、余额、兑换、支付）挂在 billingGating 之后。
// 归属见 docs/ARCHITECTURE.md §4。

// RegisterPublicRoutes 注册计费公开回调路由。
func (m *Module) RegisterPublicRoutes(publicGroup *gin.RouterGroup, gate middleware.FeatureGate) {
	gating := publicGroup.Group("", gate.Require("billingGating"))
	gating.POST("/billing/payments/stripe/webhook", m.Handler.StripeWebhook)
	gating.GET("/billing/payments/epay/notify", m.Handler.EPayNotify)
	gating.POST("/billing/payments/epay/notify", m.Handler.EPayNotify)
}

// RegisterRoutes 注册计费域路由。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup, gate middleware.FeatureGate) {
	authRequired.GET("/billing/config", m.Handler.GetBillingConfig)
	authRequired.GET("/billing/account", m.Handler.GetBillingAccount)
	authRequired.GET("/billing/overview", m.Handler.GetBillingOverview)
	authRequired.GET("/billing/usage", m.Handler.ListUsage)
	authRequired.GET("/billing/usage/monthly", m.Handler.ListMonthlyUsage)
	authRequired.GET("/billing/usage/daily", m.Handler.ListDailyUsage)

	gating := authRequired.Group("", gate.Require("billingGating"))
	gating.GET("/billing/plans", m.Handler.ListPlans)
	gating.POST("/billing/subscriptions", m.Handler.Subscribe)
	gating.POST("/billing/payments/checkout", m.Handler.CreateCheckout)
	gating.POST("/billing/redemptions", m.Handler.RedeemCode)
}

// RegisterAdminRoutes 注册管理员侧计费路由。
func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup, gate middleware.FeatureGate) {
	adminGroup.GET("/billing/config", m.Handler.GetBillingConfig)
	// PATCH 保留：价格相关键照常，billing.mode 等锁定键由 settings 服务拒绝。
	adminGroup.PATCH("/billing/config", m.Handler.PatchBillingConfig)
	adminGroup.GET("/billing/model-prices", m.Handler.ListModelPricing)
	adminGroup.PUT("/billing/model-prices", m.Handler.UpsertModelPricing)
	adminGroup.GET("/billing/official-pricing/openrouter", m.Handler.GetOpenRouterOfficialPricing)

	gating := adminGroup.Group("", gate.Require("billingGating"))
	gating.GET("/billing/plans", m.Handler.ListPlans)
	gating.PATCH("/billing/plans/:id", m.Handler.UpdatePlan)
	gating.PATCH("/billing/accounts/:user_id/balance", m.Handler.UpdateBillingAccountBalance)
	gating.GET("/billing/redemption-codes", m.Handler.ListRedemptionCodes)
	gating.POST("/billing/redemption-codes", m.Handler.CreateRedemptionCodes)
	gating.POST("/billing/redemption-codes/batch-delete", m.Handler.BatchDeleteRedemptionCodes)
	gating.GET("/billing/redemption-codes/:id/code", m.Handler.RevealRedemptionCode)
	gating.PATCH("/billing/redemption-codes/:id", m.Handler.PatchRedemptionCode)
	gating.DELETE("/billing/redemption-codes/:id", m.Handler.DeleteRedemptionCode)
}
