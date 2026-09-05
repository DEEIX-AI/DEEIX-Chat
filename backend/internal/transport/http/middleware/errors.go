package middleware

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 中间件直接判定的鉴权与限流错误。
// 错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errAdminPermissionRequired     = apperr.New("auth.admin_required", "admin permission required")
	errAuthorizationHeaderRequired = apperr.New("auth.invalid_token", "authorization header is required")
	errForbidden                   = apperr.New("auth.forbidden", "forbidden")
	errInvalidAuthorizationHeader  = apperr.New("auth.invalid_token", "invalid authorization header")
	errInvalidToken                = apperr.New("auth.invalid_token", "invalid token")
	errInvalidTokenType            = apperr.New("auth.invalid_token", "invalid token type")
	errSessionInvalid              = apperr.New("auth.session_invalid", "session invalid")

	errRateLimitExceeded        = apperr.New("rate_limit.exceeded", "rate limit exceeded")
	errRefreshRateLimitExceeded = apperr.New("rate_limit.refresh_exceeded", "too many refresh attempts")
	errAuthRateLimitExceeded    = apperr.New("rate_limit.authentication_exceeded", "too many authentication attempts")
)
