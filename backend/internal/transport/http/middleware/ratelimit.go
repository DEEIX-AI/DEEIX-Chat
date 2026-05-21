package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

// RateLimiter 封装 HTTP middleware 所需的限流存储能力。
type RateLimiter interface {
	AllowSlidingWindow(ctx context.Context, key string, limit int, window time.Duration, ttl time.Duration) (bool, error)
	AllowFixedWindow(ctx context.Context, keys []string, limit int, ttl time.Duration) (bool, error)
}

// RateLimit 基于用户维度的滑动窗口限流中间件。
func RateLimit(limiter RateLimiter, runtime *config.Runtime) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil {
			c.Next()
			return
		}

		userID, exists := c.Get(ContextKeyUserID)
		if !exists {
			c.Next()
			return
		}
		role, hasRole := c.Get(ContextKeyUserRole)
		if roleStr, ok := role.(string); hasRole && ok && roleStr == domainuser.RoleSuperAdmin {
			c.Next()
			return
		}
		requestsPerMinute := 60
		if runtime != nil {
			if value := runtime.Snapshot().RateLimitRPM; value > 0 {
				requestsPerMinute = value
			}
		}

		key := fmt.Sprintf("ratelimit:user:%v", userID)
		allowed, err := limiter.AllowSlidingWindow(c.Request.Context(), key, requestsPerMinute, time.Minute, 2*time.Minute)
		if err != nil || allowed {
			c.Next()
			return
		}
		response.Error(c, http.StatusTooManyRequests, "rate limit exceeded")
		c.Abort()
	}
}

// PublicAuthRateLimit 保护公开鉴权接口，按 IP 和登录主体进行限流。
func PublicAuthRateLimit(limiter RateLimiter, runtime *config.Runtime) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil {
			c.Next()
			return
		}
		requestsPerMinute := 30
		if runtime != nil {
			if value := runtime.Snapshot().PublicAuthRateLimitRPM; value > 0 {
				requestsPerMinute = value
			}
		}

		clientIP := c.ClientIP()
		if clientIP == "" {
			clientIP = "unknown"
		}
		path := c.Request.URL.Path
		if path == "" {
			path = "unknown"
		}
		if path == "/api/v1/auth/refresh" {
			refreshLimit := requestsPerMinute * 4
			if refreshLimit < 120 {
				refreshLimit = 120
			}
			allowed, err := limiter.AllowFixedWindow(
				c.Request.Context(),
				[]string{fmt.Sprintf("ratelimit:token-refresh:ip:%s", clientIP)},
				refreshLimit,
				time.Minute,
			)
			if err != nil || allowed {
				c.Next()
				return
			}
			response.Error(c, http.StatusTooManyRequests, "too many refresh attempts")
			c.Abort()
			return
		}

		keys := []string{
			fmt.Sprintf("ratelimit:public-auth:%s:ip:%s", path, clientIP),
		}
		allowed, err := limiter.AllowFixedWindow(c.Request.Context(), keys, requestsPerMinute, time.Minute)
		if err != nil || allowed {
			c.Next()
			return
		}
		response.Error(c, http.StatusTooManyRequests, "too many authentication attempts")
		c.Abort()
	}
}
