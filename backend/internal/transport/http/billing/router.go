package billing

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册计费公开回调路由。
func (m *Module) RegisterPublicRoutes(publicGroup *gin.RouterGroup) {
	publicGroup.POST("/billing/payments/stripe/webhook", m.Handler.StripeWebhook)
	publicGroup.GET("/billing/payments/epay/notify", m.Handler.EPayNotify)
	publicGroup.POST("/billing/payments/epay/notify", m.Handler.EPayNotify)
}

// RegisterRoutes 注册计费域路由。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup) {
	authRequired.GET("/billing/config", m.Handler.GetBillingConfig)
	authRequired.GET("/billing/account", m.Handler.GetBillingAccount)
	authRequired.GET("/billing/overview", m.Handler.GetBillingOverview)
	authRequired.GET("/billing/plans", m.Handler.ListPlans)
	authRequired.POST("/billing/subscriptions", m.Handler.Subscribe)
	authRequired.POST("/billing/payments/checkout", m.Handler.CreateCheckout)
	authRequired.POST("/billing/redemptions", m.Handler.RedeemCode)
	authRequired.GET("/billing/usage", m.Handler.ListUsage)
	authRequired.GET("/billing/usage/monthly", m.Handler.ListMonthlyUsage)
	authRequired.GET("/billing/usage/daily", m.Handler.ListDailyUsage)
}

// RegisterAdminRoutes 注册管理员侧计费路由。
func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup) {
	adminGroup.GET("/billing/config", m.Handler.GetBillingConfig)
	adminGroup.PATCH("/billing/config", m.Handler.PatchBillingConfig)
	adminGroup.GET("/billing/plans", m.Handler.ListPlans)
	adminGroup.PATCH("/billing/plans/:id", m.Handler.UpdatePlan)
	adminGroup.PATCH("/billing/accounts/:user_id/balance", m.Handler.UpdateBillingAccountBalance)
	adminGroup.GET("/billing/redemption-codes", m.Handler.ListRedemptionCodes)
	adminGroup.POST("/billing/redemption-codes", m.Handler.CreateRedemptionCodes)
	adminGroup.POST("/billing/redemption-codes/batch-delete", m.Handler.BatchDeleteRedemptionCodes)
	adminGroup.GET("/billing/redemption-codes/:id/code", m.Handler.RevealRedemptionCode)
	adminGroup.PATCH("/billing/redemption-codes/:id", m.Handler.PatchRedemptionCode)
	adminGroup.DELETE("/billing/redemption-codes/:id", m.Handler.DeleteRedemptionCode)
	adminGroup.GET("/billing/model-prices", m.Handler.ListModelPricing)
	adminGroup.GET("/billing/model-prices/official", m.Handler.FetchOfficialPricing)
	adminGroup.PUT("/billing/model-prices", m.Handler.UpsertModelPricing)
}
