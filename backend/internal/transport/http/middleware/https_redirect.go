package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// HTTPSRedirect 将生产环境的外部 HTTP 请求永久重定向到同主机 HTTPS。
func HTTPSRedirect(env string) gin.HandlerFunc {
	production := strings.EqualFold(strings.TrimSpace(env), "prod") ||
		strings.EqualFold(strings.TrimSpace(env), "production")

	return func(c *gin.Context) {
		if !production || c.Request.URL.Path == "/healthz" || c.Request.URL.Path == "/readyz" || requestUsesHTTPS(c) {
			c.Next()
			return
		}

		target := *c.Request.URL
		target.Scheme = "https"
		target.Host = c.Request.Host
		c.Redirect(http.StatusPermanentRedirect, target.String())
		c.Abort()
	}
}

func requestUsesHTTPS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	proto, _, _ := strings.Cut(c.GetHeader("X-Forwarded-Proto"), ",")
	return strings.EqualFold(strings.TrimSpace(proto), "https")
}
