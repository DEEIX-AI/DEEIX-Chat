package middleware

import (
	"net/http"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// CORS 处理跨域请求，支持逗号分隔的 Origin allowlist。
func CORS(allowOrigin string) gin.HandlerFunc {
	allowedOrigins := parseAllowedOrigins(allowOrigin)
	return func(c *gin.Context) {
		origin := strings.TrimRight(strings.TrimSpace(c.GetHeader("Origin")), "/")
		allowedOrigin := matchAllowedOrigin(origin, allowedOrigins)
		if origin != "" && allowedOrigin == "" {
			response.ErrorWithCode(c, http.StatusForbidden, "cors.origin_forbidden", "origin is not allowed")
			c.Abort()
			return
		}
		if allowedOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowedOrigin)
		}

		c.Header("Vary", "Origin")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "X-Request-ID")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func parseAllowedOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	results := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimRight(strings.TrimSpace(part), "/")
		if value == "" {
			continue
		}
		results = append(results, value)
	}
	if len(results) == 0 {
		return []string{"*"}
	}
	return results
}

func matchAllowedOrigin(origin string, allowed []string) string {
	if origin == "" {
		return ""
	}
	for _, item := range allowed {
		if item == "*" {
			return origin
		}
		if strings.EqualFold(origin, item) {
			return item
		}
	}
	return ""
}
