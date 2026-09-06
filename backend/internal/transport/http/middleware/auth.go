package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/token"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// AccessSessionState 是 access token 会话校验后的运行时主体状态。
type AccessSessionState struct {
	Role                    string
	InitialSecurityRequired bool
}

// SessionValidator 校验 access token 对应会话是否有效，并返回最新主体状态。
type SessionValidator interface {
	ValidateAccessSession(
		ctx context.Context,
		userID uint,
		sessionID string,
		accessIssuedAt time.Time,
		auditCtx requestmeta.SessionAuditContext,
	) (AccessSessionState, error)
}

// AuthMiddleware 校验 JWT 并写入用户上下文。
func AuthMiddleware(jwtSecret string, validator SessionValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := c.GetHeader("Authorization")
		if authorization == "" {
			response.ErrorFrom(c, http.StatusUnauthorized, errAuthorizationHeaderRequired)
			c.Abort()
			return
		}

		if !strings.HasPrefix(authorization, "Bearer ") {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidAuthorizationHeader)
			c.Abort()
			return
		}
		tokenText := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
		if tokenText == "" {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidAuthorizationHeader)
			c.Abort()
			return
		}

		claims, err := token.Parse(jwtSecret, tokenText)
		if err != nil {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidToken)
			c.Abort()
			return
		}
		if claims.TokenType != "" && claims.TokenType != "access" {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidTokenType)
			c.Abort()
			return
		}
		role := claims.Role
		initialSecurityRequired := false
		auditCtx := ResolveSessionAuditContext(c)
		if validator != nil {
			var issuedAt time.Time
			if claims.IssuedAt != nil {
				issuedAt = claims.IssuedAt.Time
			}
			state, validateErr := validator.ValidateAccessSession(c.Request.Context(), claims.UserID, claims.SessionID, issuedAt, auditCtx)
			if validateErr != nil {
				response.ErrorFrom(c, http.StatusUnauthorized, errSessionInvalid)
				c.Abort()
				return
			}
			if strings.TrimSpace(state.Role) != "" {
				role = state.Role
			}
			initialSecurityRequired = state.InitialSecurityRequired
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyUserRole, role)
		c.Set(ContextKeySessionID, claims.SessionID)
		c.Set(ContextKeyInitialSecurityRequired, initialSecurityRequired)
		c.Next()
	}
}

// InitialSecurityGate 在强制改密/初始引导完成前，仅放行安全完成所需接口。
func InitialSecurityGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		required, _ := c.Get(ContextKeyInitialSecurityRequired)
		if required != true {
			c.Next()
			return
		}
		if isInitialSecurityAllowedRoute(c) {
			c.Next()
			return
		}
		response.ErrorFrom(c, http.StatusForbidden, errInitialSecurityRequired)
		c.Abort()
	}
}

func isInitialSecurityAllowedRoute(c *gin.Context) bool {
	route := normalizedRoutePath(c)
	method := c.Request.Method
	switch {
	case method == http.MethodGet && route == "/me":
		return true
	case method == http.MethodPatch && (route == "/me" || route == "/me/username"):
		return true
	case method == http.MethodPost && route == "/me/onboarding/complete":
		return true
	case method == http.MethodPost && (route == "/auth/password/change/start" || route == "/auth/password/change/complete"):
		return true
	case method == http.MethodPost && strings.HasPrefix(route, "/me/email/"):
		return true
	case method == http.MethodGet && route == "/me/2fa":
		return true
	case method == http.MethodPost && strings.HasPrefix(route, "/me/2fa/"):
		return true
	case method == http.MethodDelete && route == "/me/2fa/setup":
		return true
	case method == http.MethodGet && route == "/auth/sessions":
		return true
	case method == http.MethodPut && route == "/auth/sessions/current/location":
		return true
	case method == http.MethodPost && (route == "/auth/logout" || route == "/auth/logout-all" || strings.HasPrefix(route, "/auth/sessions/")):
		return true
	default:
		return false
	}
}

// AdminOnly 限制管理员权限。
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := c.Get(ContextKeyUserRole)
		if !ok {
			response.ErrorFrom(c, http.StatusForbidden, errForbidden)
			c.Abort()
			return
		}

		roleStr, roleOK := role.(string)
		if !roleOK || !domainuser.IsAdminRole(roleStr) {
			response.ErrorFrom(c, http.StatusForbidden, errAdminPermissionRequired)
			c.Abort()
			return
		}

		c.Next()
	}
}
