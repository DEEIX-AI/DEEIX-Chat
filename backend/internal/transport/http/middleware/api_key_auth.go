package middleware

import (
	"context"
	"net/http"
	"strings"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	domainuserapikey "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/userapikey"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// APIKeyAuthenticator 校验用户网关 API Key。
type APIKeyAuthenticator interface {
	Authenticate(ctx context.Context, rawKey string) (*domainuserapikey.AuthenticatedKey, error)
}

type APIKeyUserLookup interface {
	GetByID(ctx context.Context, userID uint) (*domainuser.User, error)
}

// APIKeyAuthMiddleware 解析 Authorization: Bearer sk-… 并写入用户上下文（无 session）。
func APIKeyAuthMiddleware(authenticator APIKeyAuthenticator, users APIKeyUserLookup) gin.HandlerFunc {
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
		rawKey := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
		if rawKey == "" || !strings.HasPrefix(rawKey, "sk-") {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidToken)
			c.Abort()
			return
		}
		if authenticator == nil {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidToken)
			c.Abort()
			return
		}
		authenticated, err := authenticator.Authenticate(c.Request.Context(), rawKey)
		if err != nil || authenticated == nil {
			response.ErrorFrom(c, http.StatusUnauthorized, errInvalidToken)
			c.Abort()
			return
		}
		username := ""
		role := domainuser.RoleUser
		if users != nil {
			if user, userErr := users.GetByID(c.Request.Context(), authenticated.UserID); userErr == nil && user != nil {
				username = user.Username
				if strings.TrimSpace(user.Role) != "" {
					role = user.Role
				}
			}
		}
		c.Set(ContextKeyUserID, authenticated.UserID)
		c.Set(ContextKeyUsername, username)
		c.Set(ContextKeyUserRole, role)
		c.Set(ContextKeySessionID, "")
		c.Set(ContextKeyInitialSecurityRequired, false)
		c.Next()
	}
}
