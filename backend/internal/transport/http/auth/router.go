package auth

import (
	"github.com/gin-gonic/gin"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
)

// 能力键的含义与每条路由的归属见 docs/ARCHITECTURE.md §4。

// RegisterPublicRoutes 注册无需登录的鉴权路由。
func (m *Module) RegisterPublicRoutes(api *gin.RouterGroup, gate middleware.FeatureGate) {
	api.GET("/auth/login-options", m.Handler.LoginOptions)
	api.POST("/auth/login", m.Handler.Login)
	api.POST("/auth/2fa/email/start", m.Handler.StartTwoFactorEmailVerification)
	api.POST("/auth/2fa/verify", m.Handler.VerifyTwoFactorLogin)
	api.POST("/auth/refresh", m.Handler.RefreshToken)

	registration := api.Group("", gate.Require("registration"))
	registration.POST("/auth/register/email/start", m.Handler.StartEmailRegistration)
	registration.POST("/auth/register/email/complete", m.Handler.CompleteEmailRegistration)

	accountSecurity := api.Group("", gate.Require("accountSecurity"))
	accountSecurity.POST("/auth/password/reset/start", m.Handler.StartPasswordReset)
	accountSecurity.POST("/auth/password/reset/complete", m.Handler.CompletePasswordReset)

	identityProviders := api.Group("", gate.Require("identityProviders"))
	identityProviders.POST("/auth/providers/:slug/authorize", m.Handler.StartProviderAuthBridge)
	identityProviders.GET("/auth/providers/:slug/callback", m.Handler.ProviderCallback)
	identityProviders.POST("/auth/providers/:slug/exchange", m.Handler.ExchangeProviderAuthBridgeGrant)
}

// RegisterProtectedRoutes 注册需登录的鉴权路由。
func (m *Module) RegisterProtectedRoutes(authRequired *gin.RouterGroup, gate middleware.FeatureGate) {
	authRequired.GET("/me", m.Handler.Me)
	authRequired.PATCH("/me", m.Handler.PatchMe)
	authRequired.PATCH("/me/username", m.Handler.PatchUsername)
	authRequired.POST("/me/onboarding/complete", m.Handler.CompleteOnboarding)
	authRequired.POST("/auth/logout", m.Handler.Logout)

	accountSecurity := authRequired.Group("", gate.Require("accountSecurity"))
	accountSecurity.POST("/me/delete/start", m.Handler.StartAccountDeleteVerification)
	accountSecurity.DELETE("/me", m.Handler.DeleteMe)
	accountSecurity.POST("/auth/password/change/start", m.Handler.StartPasswordChangeVerification)
	accountSecurity.POST("/auth/password/change/complete", m.Handler.ChangePassword)
	accountSecurity.POST("/me/email/bootstrap/start", m.Handler.StartEmailBootstrap)
	accountSecurity.POST("/me/email/bootstrap/complete", m.Handler.CompleteEmailBootstrap)
	accountSecurity.POST("/me/email/verify-current/start", m.Handler.StartCurrentEmailVerification)
	accountSecurity.POST("/me/email/verify-current/complete", m.Handler.CompleteCurrentEmailVerification)
	accountSecurity.POST("/me/email/change/start-current", m.Handler.StartCurrentEmailChange)
	accountSecurity.POST("/me/email/change/start-new", m.Handler.StartNewEmailChange)
	accountSecurity.POST("/me/email/change/complete", m.Handler.CompleteEmailChange)
	accountSecurity.GET("/me/2fa", m.Handler.CurrentTwoFactorStatus)
	accountSecurity.POST("/me/2fa/setup/start", m.Handler.StartCurrentTwoFactorSetup)
	accountSecurity.POST("/me/2fa/setup/confirm", m.Handler.ConfirmCurrentTwoFactorSetup)
	accountSecurity.DELETE("/me/2fa/setup", m.Handler.CancelCurrentTwoFactorSetup)
	accountSecurity.POST("/me/2fa/recovery/regenerate", m.Handler.RegenerateCurrentTwoFactorRecoveryCodes)
	accountSecurity.POST("/me/2fa/disable", m.Handler.DisableCurrentTwoFactor)
	accountSecurity.GET("/auth/sessions", m.Handler.CurrentSessions)
	accountSecurity.PUT("/auth/sessions/current/location", m.Handler.UpdateCurrentSessionLocation)
	accountSecurity.POST("/auth/sessions/:session_id/logout", m.Handler.LogoutSession)
	accountSecurity.POST("/auth/logout-all", m.Handler.LogoutAll)

	identityProviders := authRequired.Group("", gate.Require("identityProviders"))
	identityProviders.GET("/me/identities", m.Handler.ListCurrentUserIdentities)
	identityProviders.POST("/me/identities/providers/:slug/authorize", m.Handler.StartProviderBindBridge)
	identityProviders.POST("/me/identities/providers/:slug/exchange", m.Handler.ExchangeProviderBindBridgeGrant)
	identityProviders.DELETE("/me/identities/:identity_id", m.Handler.DeleteCurrentUserIdentity)
}

// RegisterAdminRoutes 注册仅限管理员的身份提供方路由。
func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup, gate middleware.FeatureGate) {
	identityProviders := adminGroup.Group("", gate.Require("identityProviders"))
	identityProviders.GET("/auth/providers", m.Handler.ListIdentityProviders)
	identityProviders.POST("/auth/providers", m.Handler.CreateIdentityProvider)
	identityProviders.PATCH("/auth/provider-order", m.Handler.ReorderIdentityProviders)
	identityProviders.PATCH("/auth/providers/:provider_id", m.Handler.UpdateIdentityProvider)
	identityProviders.DELETE("/auth/providers/:provider_id", m.Handler.DeleteIdentityProvider)
}

// RegisterLocalRoutes 注册仅在本地 sidecar 模式下存在的路由。
// 服务器部署永远不会挂载它们，因此不存在被误暴露到公网的可能。
func (m *Module) RegisterLocalRoutes(api *gin.RouterGroup) {
	api.POST("/auth/local/exchange", m.Handler.ExchangeLocalGrant)
}
