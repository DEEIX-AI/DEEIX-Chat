package auth

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 由 transport 层直接判定的请求错误（路径参数、必填校验、运行时依赖缺失等）。
// 错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errInvalidCurrentPassword    = apperr.New("auth.invalid_current_password", "invalid current password")
	errInvalidIdentityID         = apperr.New("identity.invalid_id", "invalid identity id")
	errInvalidRefreshToken       = apperr.New("auth.invalid_refresh_token", "invalid refresh token")
	errInvalidTwoFactorChallenge = apperr.New("request.invalid_two_factor_challenge", "invalid two factor challenge")
	errInvalidTwoFactorCode      = apperr.New("auth.invalid_two_factor_code", "invalid two factor code")
	errPasswordResetFailed       = apperr.New("auth.password_reset_failed", "password reset failed")
	errUnauthorized              = apperr.New("auth.unauthorized", "unauthorized")
)
