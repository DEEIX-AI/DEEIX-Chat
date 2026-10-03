package middleware

import (
	"net/http"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// desktopWebviewOrigins 是官方桌面端（Tauri）webview 的 Origin：macOS/Linux 为 tauri://localhost，
// Windows 为 http://tauri.localhost。它们是客户端内置常量，与部署方的网页域名无关，
// 因此始终放行，避免运营方覆盖 CORS_ALLOW_ORIGIN 后桌面端无法连接远程服务器。
// 浏览器页面无法伪造这两个 Origin（自定义 scheme / 仅解析到本机回环的 .localhost）。
var desktopWebviewOrigins = []string{"tauri://localhost", "http://tauri.localhost"}

// CORS 处理跨域请求，支持逗号分隔的 Origin allowlist；桌面端 webview Origin 总是被允许。
func CORS(allowOrigin string) gin.HandlerFunc {
	allowedOrigins := append(parseAllowedOrigins(allowOrigin), desktopWebviewOrigins...)
	return func(c *gin.Context) {
		origin := strings.TrimRight(strings.TrimSpace(c.GetHeader("Origin")), "/")
		allowedOrigin := matchAllowedOrigin(origin, allowedOrigins)
		if origin != "" && allowedOrigin == "" {
			response.ErrorWithCode(c, http.StatusForbidden, "cors.origin_forbidden")
			c.Abort()
			return
		}
		if allowedOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowedOrigin)
		}

		c.Header("Vary", "Origin")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		// X-Client-Platform 由桌面/移动端携带，用于选择 refresh token 的投递方式；
		// 缺少它时预检会拒绝带该头的真实请求。
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-ID,X-Client-Platform")
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
