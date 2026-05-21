package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const defaultContentSecurityPolicy = "default-src 'self'; base-uri 'self'; object-src 'self' blob:; frame-ancestors 'none'; form-action 'self'; img-src 'self' data: blob:; media-src 'self' data: blob:; font-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; connect-src 'self' http: https: ws: wss: blob:; worker-src 'self' blob:; frame-src 'self' blob:"

// SecurityHeaders 为所有 HTTP 响应补充安全响应头；文件内容响应可在 handler 中覆盖为更严格策略。
func SecurityHeaders(env string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		setHeaderIfEmpty(header, "X-Content-Type-Options", "nosniff")
		setHeaderIfEmpty(header, "X-Frame-Options", "DENY")
		setHeaderIfEmpty(header, "Referrer-Policy", "no-referrer")
		setHeaderIfEmpty(header, "Permissions-Policy", "camera=(), geolocation=(), payment=(), usb=(), serial=(), bluetooth=(), browsing-topics=()")
		setHeaderIfEmpty(header, "Content-Security-Policy", defaultContentSecurityPolicy)
		if strings.EqualFold(strings.TrimSpace(env), "prod") || strings.EqualFold(strings.TrimSpace(env), "production") {
			setHeaderIfEmpty(header, "Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

func setHeaderIfEmpty(header http.Header, key string, value string) {
	if header.Get(key) != "" {
		return
	}
	header.Set(key, value)
}
