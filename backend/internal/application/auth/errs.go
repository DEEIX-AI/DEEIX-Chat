package auth

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// ProviderEmailConflictActionSignInThenBind identifies the safe sign-in-then-bind recovery flow.
const ProviderEmailConflictActionSignInThenBind = "sign_in_then_bind"

var (
	// ErrInvalidCredentials 用户名或密码错误。
	ErrInvalidCredentials = apperr.New("auth.invalid_credentials", "invalid username or password")
	// ErrAccountLocked 账户已被锁定。
	ErrAccountLocked = apperr.NewMasked("auth.invalid_credentials", "invalid username or password", "account locked")
	// ErrInvalidTimeZone 用户时区格式非法。
	ErrInvalidTimeZone = apperr.New("user.invalid_time_zone", "invalid time zone")
	// ErrInvalidLocale 用户语言区域非法。
	ErrInvalidLocale = apperr.New("user.invalid_locale", "invalid user locale")
	// ErrInvalidAppearancePreferences 外观偏好 JSON 非法。
	ErrInvalidAppearancePreferences = apperr.New("request.invalid_appearance_preferences", "invalid appearance preferences")
	// ErrInvalidAvatarURL 用户头像地址格式非法。
	ErrInvalidAvatarURL = apperr.New("user.invalid_avatar_url", "invalid avatar url")
	// ErrInvalidUsername 用户名格式非法。
	ErrInvalidUsername = apperr.New("user.invalid_username", "invalid username")
	// ErrUsernameTaken 用户名已被占用。
	ErrUsernameTaken = apperr.New("user.username_already_exists", "username already exists")
	// ErrUsernameChangeUsed 用户名自主修改次数已用完。
	ErrUsernameChangeUsed = apperr.New("user.username_change_used", "username change already used")
	// ErrUsernameChangeRequired 初始化用户名必须修改。
	ErrUsernameChangeRequired = apperr.New("auth.username_change_required", "username change required")
	// ErrInvalidLocation 当前会话位置数据非法。
	ErrInvalidLocation = apperr.NewMasked("request.invalid_location_payload", "invalid location payload", "invalid location")
	// ErrDeleteSuperAdminNotAllowed 禁止自助删除超级管理员。
	ErrDeleteSuperAdminNotAllowed = apperr.NewMasked("user.superadmin_delete_protected", "superadmin account deletion is not allowed", "superadmin account deletion not allowed")
	// ErrAccountDeleteVerificationRequired 删除账号必须先完成安全验证。
	ErrAccountDeleteVerificationRequired = apperr.New("user.account_delete_verification_required", "account deletion requires verification")
	// ErrSecurityVerificationMethodUnavailable 所选安全验证方式当前不可用。
	ErrSecurityVerificationMethodUnavailable = apperr.New("auth.verification_method_unavailable", "verification method is unavailable")
	// ErrSecurityVerificationEmailInvalid 用户邮箱缺失或格式非法，无法完成邮箱验证。
	ErrSecurityVerificationEmailInvalid = apperr.NewMasked("auth.invalid_email", "invalid email", "user email is invalid")
	// ErrSecurityVerificationCodeInvalid 安全验证码错误或已过期。
	ErrSecurityVerificationCodeInvalid = apperr.New("auth.verification_code_invalid", "verification code is invalid or expired")
	// ErrInvalidRefreshToken 无效刷新令牌。
	ErrInvalidRefreshToken = apperr.New("auth.invalid_refresh_token", "invalid refresh token")
	// ErrSessionRevoked 会话已吊销。
	ErrSessionRevoked = apperr.NewMasked("auth.session_invalid", "session invalid", "session revoked")
	// ErrLastLoginMethodNotAllowed 禁止移除账号最后一种可用登录方式。
	ErrLastLoginMethodNotAllowed = apperr.NewMasked("auth.last_login_method_required", "set a password or bind another identity provider first", "cannot unlink the last available login method")
	// ErrIdentityNotFound 表示当前用户绑定身份不存在。
	ErrIdentityNotFound = apperr.New("identity.not_found", "identity not found")
	// ErrIdentityProviderDeleteConflict 表示删除身份源会让用户失去最后一种登录方式。
	ErrIdentityProviderDeleteConflict = apperr.NewMasked("identity_provider.delete_conflict", "deleting this identity provider would remove the only login method for some users", "identity provider delete conflict")
	// ErrIdentityProviderSuperAdminDefaultRoleNotAllowed 表示非 superadmin 不允许设置 superadmin 默认角色。
	ErrIdentityProviderSuperAdminDefaultRoleNotAllowed = apperr.New("auth.provider_superadmin_default_role_protected", "only superadmin can set superadmin default role")
	// ErrTwoFactorSetupExpired 两步验证设置已过期。
	ErrTwoFactorSetupExpired = apperr.New("auth.two_factor_expired", "two factor setup expired")
	// ErrTwoFactorSetupNotStarted 当前没有待确认的两步验证设置。
	ErrTwoFactorSetupNotStarted = apperr.New("auth.two_factor_not_started", "two factor setup not started")
	// ErrTwoFactorSetupNotPersisted 两步验证确认后持久化状态未生效。
	ErrTwoFactorSetupNotPersisted = apperr.NewMasked("internal.error", "internal server error", "two factor setup not persisted")
	// ErrTwoFactorAlreadyEnabled indicates that two-factor authentication is already enabled.
	ErrTwoFactorAlreadyEnabled = apperr.New("auth.two_factor_already_enabled", "two factor authentication is already enabled")
	// ErrTwoFactorChallengeExpired 登录二次验证挑战已过期。
	ErrTwoFactorChallengeExpired = apperr.New("auth.two_factor_expired", "two factor challenge expired")
	// ErrPasswordResetFailed 表示密码重置失败，避免暴露邮箱或账号状态。
	ErrPasswordResetFailed = apperr.New("auth.password_reset_failed", "password reset failed")
	// ErrProviderEmailConflict 表示第三方身份邮箱已存在但不能安全自动合并。
	ErrProviderEmailConflict = apperr.NewMasked("auth.provider_email_conflict", "provider email belongs to another account", "provider email conflict")

	// Stable registration and verification errors form the public auth API contract.
	ErrEmailRegistrationDisabled = apperr.New("auth.email_registration_disabled", "email registration is disabled")
	ErrEmailVerificationDisabled = apperr.New("auth.email_verification_disabled", "email verification is disabled")
	ErrEmailAlreadyExists        = apperr.New("auth.email_already_exists", "email already exists")
	ErrVerificationCodeRecent    = apperr.New("auth.verification_code_recent", "verification code was sent recently")
	ErrVerificationCodeAttempts  = apperr.New("auth.verification_code_attempts_exceeded", "verification code attempts exceeded")
	ErrInvalidEmail              = apperr.New("auth.invalid_email", "invalid email")
	ErrCurrentEmailNotVerified   = apperr.NewMasked("auth.email_not_verified", "email is not verified", "current email cannot be verified")
	ErrEmailUnchanged            = apperr.New("auth.email_unchanged", "new email must be different")
	ErrEmailAliasNotAllowed      = apperr.New("auth.email_alias_not_allowed", "email aliases are not allowed")
	ErrEmailDomainNotAllowed     = apperr.New("auth.email_domain_not_allowed", "email domain is not allowed")
	ErrEmailBootstrapNotAllowed  = apperr.New("auth.email_bootstrap_not_allowed", "email bootstrap is not allowed")
	ErrPasswordReuse             = apperr.New("auth.password_reuse_not_allowed", "new password must be different from the bootstrap password")
	ErrUserIDRequired            = apperr.New("request.required", "user id is required")
	ErrUnauthorized              = apperr.New("auth.unauthorized", "unauthorized")

	// Turnstile and SMTP failures are returned by authentication endpoints with
	// stable codes so callers do not need to inspect internal text.
	ErrTurnstileNotConfigured = apperr.New("auth.turnstile_not_configured", "turnstile is not configured")
	ErrTurnstileRequired      = apperr.New("auth.turnstile_required", "turnstile verification is required")
	ErrTurnstileInvalid       = apperr.New("auth.turnstile_invalid", "turnstile token is invalid")
	ErrTurnstileTokenTooLong  = apperr.NewMasked("auth.turnstile_invalid", "turnstile token is invalid", "turnstile token is too long")
	ErrTurnstileFailed        = apperr.New("auth.turnstile_invalid", "turnstile verification failed")
	ErrSMTPNotConfigured      = apperr.New("settings.smtp_invalid", "smtp is not configured")
	ErrSMTPFromInvalid        = apperr.New("settings.smtp_invalid", "smtp from is invalid")
	ErrSMTPAuthUnsupported    = apperr.New("settings.smtp_invalid", "smtp auth is not supported")

	// Provider configuration and OAuth flow errors are public validation or
	// authentication outcomes. Upstream details remain internal to logs.
	ErrProviderAuthBridgeUnavailable     = apperr.New("auth.provider_invalid", "provider auth bridge is not configured")
	ErrThirdPartyLoginDisabled           = apperr.New("auth.provider_login_disabled", "third-party login is disabled")
	ErrProviderLoginDisabled             = apperr.New("auth.provider_login_disabled", "provider login is disabled")
	ErrProviderRegistrationDisabled      = apperr.New("auth.provider_registration_disabled", "provider registration is disabled")
	ErrProviderBindingFlowRequired       = apperr.New("auth.provider_bind_endpoint_required", "provider binding must use the authenticated binding flow")
	ErrProviderBindEndpointRequired      = apperr.New("auth.provider_bind_endpoint_required", "provider bind must use account binding endpoint")
	ErrAuthorizationCodeRequired         = apperr.New("auth.authorization_code_required", "authorization code is required")
	ErrProviderIDRequired                = apperr.New("auth.provider_id_required", "provider id is required")
	ErrProviderOrderInvalid              = apperr.New("auth.provider_order_invalid", "provider ids must be unique")
	ErrProviderTypeInvalid               = apperr.New("auth.provider_type_invalid", "provider type must be oidc or oauth2")
	ErrProviderNameRequired              = apperr.New("auth.provider_name_required", "provider name is required")
	ErrProviderSlugRequired              = apperr.New("auth.provider_slug_required", "provider slug is required")
	ErrProviderDefaultRoleInvalid        = apperr.New("auth.provider_default_role_invalid", "default role must be user, admin or superadmin")
	ErrProviderLogoURLInvalid            = apperr.New("auth.provider_logo_url_invalid", "logo url must be a valid http(s) or absolute path")
	ErrProviderRegistrationRequiresLogin = apperr.New("auth.provider_registration_requires_login", "provider registration requires provider login to be enabled")
	ErrProviderClientIDRequired          = apperr.New("auth.provider_client_id_required", "client id is required")
	ErrProviderClientSecretRequired      = apperr.New("auth.provider_client_secret_required", "client secret is required")
	ErrProviderOIDCIssuerRequired        = apperr.New("auth.provider_oidc_issuer_required", "OIDC issuer url or discovery url is required")
	ErrProviderOAuthURLsRequired         = apperr.New("auth.provider_oauth_urls_required", "OAuth2 auth url, token url and userinfo url are required")
	ErrProviderAuthURLNotConfigured      = apperr.New("auth.provider_auth_url_not_configured", "provider auth url is not configured")
	ErrProviderSubjectMissing            = apperr.New("auth.provider_subject_missing", "provider subject is missing")
	ErrProviderIdentityConflict          = apperr.New("auth.provider_identity_conflict", "provider identity is already bound to another account")
	ErrProviderAlreadyBound              = apperr.New("auth.provider_already_bound", "provider is already bound")
	ErrProviderAccountNotRegistered      = apperr.New("auth.provider_account_not_registered", "provider account is not registered")
	ErrOAuthIntentMismatch               = apperr.New("auth.oauth_intent_mismatch", "oauth intent mismatch")
	ErrOAuthStateInvalid                 = apperr.New("auth.oauth_state_invalid", "invalid oauth state")
	ErrOAuthStateMismatch                = apperr.NewMasked("auth.oauth_state_invalid", "invalid oauth state", "oauth state mismatch")
	ErrOAuthStateExpired                 = apperr.New("auth.oauth_state_expired", "oauth state expired")
	ErrInvalidRedirectURI                = apperr.New("auth.invalid_redirect_uri", "invalid redirect uri")
	ErrRedirectURIOriginNotAllowed       = apperr.New("auth.invalid_redirect_uri", "redirect uri origin is not allowed")
	ErrInvalidPKCE                       = apperr.New("auth.invalid_pkce", "invalid pkce parameters")
	ErrPKCEChallengeRequired             = apperr.NewMasked("auth.invalid_pkce", "invalid sign-in verification parameters", "valid pkce code challenge is required")
	ErrPKCEVerifierRequired              = apperr.NewMasked("auth.invalid_pkce", "invalid sign-in verification parameters", "valid pkce code verifier is required")
	ErrPKCEMismatch                      = apperr.NewMasked("auth.invalid_pkce", "invalid sign-in verification parameters", "pkce code verifier mismatch")
	ErrProviderEmailInvalid              = apperr.NewMasked("auth.invalid_email", "invalid email", "provider email is invalid")
	ErrProviderEndpointInvalid           = apperr.New("auth.provider_invalid", "provider endpoint is invalid")
	ErrProviderUpstreamFailed            = apperr.New("auth.provider_upstream_failed", "provider authentication failed")
	ErrProviderAuthenticationFailed      = apperr.New("auth.provider_invalid", "provider authentication failed")
	ErrProviderCallbackMisconfigured     = apperr.New("auth.provider_callback_misconfigured", "provider auth bridge callback url is not configured")
	ErrProviderBridgeStateInvalid        = apperr.New("auth.oauth_state_invalid", "invalid provider auth bridge state")
	ErrProviderBridgeStateMismatch       = apperr.New("auth.oauth_state_invalid", "provider auth bridge state mismatch")
	ErrProviderBridgeStateExpired        = apperr.New("auth.oauth_state_expired", "provider auth bridge state expired")
	ErrProviderAuthorizationDenied       = apperr.NewMasked("auth.provider_invalid", "provider authentication failed", "provider authorization was denied")
	ErrProviderCallbackInvalid           = apperr.NewMasked("auth.provider_invalid", "provider authentication failed", "provider callback did not include an authorization code")
	ErrProviderTransactionExpired        = apperr.NewMasked("auth.oauth_state_expired", "oauth state expired", "provider authorization transaction expired or already used")
	ErrProviderTransactionMismatch       = apperr.NewMasked("auth.oauth_state_invalid", "invalid oauth state", "provider authorization transaction mismatch")
	ErrProviderGrantRequired             = apperr.NewMasked("auth.invalid_pkce", "invalid sign-in verification parameters", "valid provider authorization grant is required")
	ErrProviderGrantExpired              = apperr.NewMasked("auth.oauth_state_expired", "oauth state expired", "provider authorization grant expired, invalid, or already used")
	ErrProviderGrantMismatch             = apperr.NewMasked("auth.oauth_state_invalid", "invalid oauth state", "provider authorization grant mismatch")
	ErrProviderClientStateInvalid        = apperr.NewMasked("auth.invalid_pkce", "invalid sign-in verification parameters", "valid client state is required")
	ErrProviderAuthClientUnsupported     = apperr.NewMasked("auth.provider_invalid", "provider authentication failed", "unsupported provider auth client")
	ErrProviderNativeRedirectInvalid     = apperr.NewMasked("auth.invalid_redirect_uri", "invalid callback URL", "invalid native redirect uri")
	ErrProviderDesktopRedirectInvalid    = apperr.NewMasked("auth.invalid_redirect_uri", "invalid callback URL", "invalid desktop redirect uri")
	ErrProviderCallbackBaseInvalid       = apperr.NewMasked("auth.provider_callback_misconfigured", "provider auth bridge callback url is not configured", "provider auth bridge requires PUBLIC_API_BASE_URL")
	ErrProviderCallbackBaseDirty         = apperr.NewMasked("auth.provider_callback_misconfigured", "provider auth bridge callback url is not configured", "provider auth bridge requires a clean PUBLIC_API_BASE_URL")
)

// IdentityProviderDeleteConflictError 携带身份源删除冲突的受影响用户数量。
type IdentityProviderDeleteConflictError struct {
	DependentUsers int
}

// Error returns the stable public identity-provider conflict error message.
func (e *IdentityProviderDeleteConflictError) Error() string {
	return ErrIdentityProviderDeleteConflict.Error()
}

// Unwrap exposes the sentinel used by transport error mapping.
func (e *IdentityProviderDeleteConflictError) Unwrap() error {
	return ErrIdentityProviderDeleteConflict
}

// ProviderEmailConflictError 携带第三方登录邮箱冲突的安全绑定引导信息。
type ProviderEmailConflictError struct {
	ProviderSlug string
	Email        string
	Action       string
}

// Error returns the stable public provider-email conflict error message.
func (e *ProviderEmailConflictError) Error() string {
	return ErrProviderEmailConflict.Error()
}

// Unwrap exposes the sentinel used by transport error mapping.
func (e *ProviderEmailConflictError) Unwrap() error {
	return ErrProviderEmailConflict
}
