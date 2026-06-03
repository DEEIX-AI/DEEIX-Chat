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

// SessionValidator 校验 access token 对应会话是否有效。
type SessionValidator interface {
	ValidateAccessSession(
		ctx context.Context,
		userID uint,
		sessionID string,
		accessIssuedAt time.Time,
		auditCtx requestmeta.SessionAuditContext,
	) error
}

// AuthMiddleware 校验 JWT 并写入用户上下文。
func AuthMiddleware(jwtSecret string, validator SessionValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := c.GetHeader("Authorization")
		if authorization == "" {
			response.Error(c, http.StatusUnauthorized, "missing Authorization header")
			c.Abort()
			return
		}

		if !strings.HasPrefix(authorization, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, "invalid Authorization header")
			c.Abort()
			return
		}
		tokenText := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
		if tokenText == "" {
			response.Error(c, http.StatusUnauthorized, "invalid Authorization header")
			c.Abort()
			return
		}

		claims, err := token.Parse(jwtSecret, tokenText)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}
		if claims.TokenType != "" && claims.TokenType != "access" {
			response.Error(c, http.StatusUnauthorized, "invalid token type")
			c.Abort()
			return
		}
		auditCtx := ResolveSessionAuditContext(c)
		if validator != nil {
			var issuedAt time.Time
			if claims.IssuedAt != nil {
				issuedAt = claims.IssuedAt.Time
			}
			if err = validator.ValidateAccessSession(c.Request.Context(), claims.UserID, claims.SessionID, issuedAt, auditCtx); err != nil {
				response.Error(c, http.StatusUnauthorized, "session invalid")
				c.Abort()
				return
			}
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Set(ContextKeyUserRole, claims.Role)
		c.Set(ContextKeySessionID, claims.SessionID)
		c.Next()
	}
}

// AdminOnly 限制管理员权限。
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := c.Get(ContextKeyUserRole)
		if !ok {
			response.Error(c, http.StatusForbidden, "forbidden")
			c.Abort()
			return
		}

		roleStr, roleOK := role.(string)
		if !roleOK || !domainuser.IsAdminRole(roleStr) {
			response.Error(c, http.StatusForbidden, "admin permission required")
			c.Abort()
			return
		}

		c.Next()
	}
}
