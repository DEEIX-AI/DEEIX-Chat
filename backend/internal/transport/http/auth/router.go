package auth

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册无需登录的鉴权路由。
func (m *Module) RegisterPublicRoutes(api *gin.RouterGroup) {
	api.GET("/auth/login-options", m.Handler.LoginOptions)
	api.POST("/auth/login", m.Handler.Login)
	api.POST("/auth/2fa/email/start", m.Handler.StartTwoFactorEmailVerification)
	api.POST("/auth/2fa/verify", m.Handler.VerifyTwoFactorLogin)
	api.POST("/auth/register/email/start", m.Handler.StartEmailRegistration)
	api.POST("/auth/register/email/complete", m.Handler.CompleteEmailRegistration)
	api.POST("/auth/password/reset/start", m.Handler.StartPasswordReset)
	api.POST("/auth/password/reset/complete", m.Handler.CompletePasswordReset)
	api.POST("/auth/refresh", m.Handler.RefreshToken)
	api.GET("/auth/providers/:slug/start", m.Handler.StartProviderLogin)
	api.GET("/auth/providers/:slug/callback", m.Handler.ProviderCallback)
	api.POST("/auth/providers/:slug/callback", m.Handler.CompleteProviderLogin)
}

// RegisterProtectedRoutes 注册需登录的鉴权路由。
func (m *Module) RegisterProtectedRoutes(authRequired *gin.RouterGroup) {
	authRequired.GET("/me", m.Handler.Me)
	authRequired.PATCH("/me", m.Handler.PatchMe)
	authRequired.PATCH("/me/username", m.Handler.PatchUsername)
	authRequired.POST("/me/onboarding/complete", m.Handler.CompleteOnboarding)
	authRequired.POST("/me/delete/start", m.Handler.StartAccountDeleteVerification)
	authRequired.DELETE("/me", m.Handler.DeleteMe)
	authRequired.POST("/auth/password/change/start", m.Handler.StartPasswordChangeVerification)
	authRequired.POST("/auth/password/change/complete", m.Handler.ChangePassword)
	authRequired.POST("/me/email/bootstrap/start", m.Handler.StartEmailBootstrap)
	authRequired.POST("/me/email/bootstrap/complete", m.Handler.CompleteEmailBootstrap)
	authRequired.POST("/me/email/verify-current/start", m.Handler.StartCurrentEmailVerification)
	authRequired.POST("/me/email/verify-current/complete", m.Handler.CompleteCurrentEmailVerification)
	authRequired.POST("/me/email/change/start-current", m.Handler.StartCurrentEmailChange)
	authRequired.POST("/me/email/change/start-new", m.Handler.StartNewEmailChange)
	authRequired.POST("/me/email/change/complete", m.Handler.CompleteEmailChange)
	authRequired.GET("/me/2fa", m.Handler.CurrentTwoFactorStatus)
	authRequired.POST("/me/2fa/setup/start", m.Handler.StartCurrentTwoFactorSetup)
	authRequired.POST("/me/2fa/setup/confirm", m.Handler.ConfirmCurrentTwoFactorSetup)
	authRequired.DELETE("/me/2fa/setup", m.Handler.CancelCurrentTwoFactorSetup)
	authRequired.POST("/me/2fa/recovery/regenerate", m.Handler.RegenerateCurrentTwoFactorRecoveryCodes)
	authRequired.POST("/me/2fa/disable", m.Handler.DisableCurrentTwoFactor)
	authRequired.GET("/me/identities", m.Handler.ListCurrentUserIdentities)
	authRequired.POST("/me/identities/providers/:slug/callback", m.Handler.CompleteProviderBind)
	authRequired.DELETE("/me/identities/:identity_id", m.Handler.DeleteCurrentUserIdentity)
	authRequired.GET("/auth/sessions", m.Handler.CurrentSessions)
	authRequired.PUT("/auth/sessions/current/location", m.Handler.UpdateCurrentSessionLocation)
	authRequired.POST("/auth/sessions/:session_id/logout", m.Handler.LogoutSession)
	authRequired.POST("/auth/logout", m.Handler.Logout)
	authRequired.POST("/auth/logout-all", m.Handler.LogoutAll)
}

func (m *Module) RegisterAdminRoutes(adminGroup *gin.RouterGroup) {
	adminGroup.GET("/auth/providers", m.Handler.ListIdentityProviders)
	adminGroup.POST("/auth/providers", m.Handler.CreateIdentityProvider)
	adminGroup.PATCH("/auth/provider-order", m.Handler.ReorderIdentityProviders)
	adminGroup.PATCH("/auth/providers/:provider_id", m.Handler.UpdateIdentityProvider)
	adminGroup.DELETE("/auth/providers/:provider_id", m.Handler.DeleteIdentityProvider)
}
