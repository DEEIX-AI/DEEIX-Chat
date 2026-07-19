package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders 为所有 HTTP 响应补充通用安全响应头。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		setHeaderIfEmpty(header, "X-Content-Type-Options", "nosniff")
		setHeaderIfEmpty(header, "X-Frame-Options", "DENY")
		setHeaderIfEmpty(header, "Permissions-Policy", "camera=(), geolocation=(), payment=(), usb=(), serial=(), bluetooth=(), browsing-topics=()")
		c.Next()
	}
}

func setHeaderIfEmpty(header http.Header, key string, value string) {
	if header.Get(key) != "" {
		return
	}
	header.Set(key, value)
}
