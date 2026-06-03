package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AccessLog 输出请求访问日志。
func AccessLog(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if shouldSkipAccessLog(c.Request.URL.Path) {
			c.Next()
			return
		}

		started := time.Now()
		c.Next()

		latency := time.Since(started)
		status := c.Writer.Status()
		traceID := MustTraceID(c)
		requestID := MustRequestID(c)
		userID := MustUserID(c)
		path := c.Request.URL.Path
		if rawQuery := c.Request.URL.RawQuery; rawQuery != "" {
			path = path + "?" + rawQuery
		}

		message := fmt.Sprintf(
			"[AccessLog] %s %s\nStartTime: %s\nEndTime: %s\nLatency: %d\nClientIP: %s\nResponse: %d %d\nRequestID: %s\nTraceID: %s\nUserID: %d\nUserAgent: %s",
			c.Request.Method,
			path,
			started.Format(time.RFC3339),
			started.Add(latency).Format(time.RFC3339),
			latency.Milliseconds(),
			c.ClientIP(),
			status,
			c.Writer.Size(),
			requestID,
			traceID,
			userID,
			c.Request.UserAgent(),
		)

		if status >= 500 {
			logger.Error(message)
		} else if status >= 400 {
			logger.Warn(message)
		} else {
			logger.Info(message)
		}
	}
}

func shouldSkipAccessLog(requestPath string) bool {
	if requestPath == "/healthz" {
		return true
	}
	return !isBackendAccessLogPath(requestPath)
}

func isBackendAccessLogPath(requestPath string) bool {
	return requestPath == "/readyz" ||
		requestPath == "/api" ||
		strings.HasPrefix(requestPath, "/api/") ||
		requestPath == "/swagger" ||
		strings.HasPrefix(requestPath, "/swagger/")
}
