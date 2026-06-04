package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

type rateLimitPolicy struct {
	Name    string
	Limit   int
	Window  time.Duration
	TTL     time.Duration
	Message string
}

const (
	defaultAuthenticatedRateLimitRPM = 60
	defaultPublicAuthRateLimitRPM    = 30
	rateLimitWindow                  = time.Minute
)

// RateLimit 基于用户维度的滑动窗口限流中间件。
func RateLimit(limiter RateLimiter, runtime *config.Runtime) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil || !rateLimitEnabled(runtime) {
			c.Next()
			return
		}

		userID, exists := c.Get(ContextKeyUserID)
		if !exists {
			c.Next()
			return
		}
		role, hasRole := c.Get(ContextKeyUserRole)
		if roleStr, ok := role.(string); hasRole && ok && domainuser.IsAdminRole(roleStr) {
			c.Next()
			return
		}

		policy := authenticatedRateLimitPolicy(c, authenticatedRateLimitRPM(runtime))
		key := fmt.Sprintf("ratelimit:user:%v:%s", userID, policy.Name)
		allowed, err := limiter.AllowSlidingWindow(c.Request.Context(), key, policy.Limit, policy.Window, policy.TTL)
		if err != nil || allowed {
			c.Next()
			return
		}
		writeRateLimitError(c, policy)
	}
}

// PublicAuthRateLimit 保护公开接口，按 IP、接口与风险等级限流。
func PublicAuthRateLimit(limiter RateLimiter, runtime *config.Runtime) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil || !rateLimitEnabled(runtime) {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		if clientIP == "" {
			clientIP = "unknown"
		}

		policy := publicRateLimitPolicy(c, publicAuthRateLimitRPM(runtime))
		key := fmt.Sprintf("ratelimit:public:%s:%s:ip:%s", policy.Name, normalizedRoutePath(c), clientIP)
		if policy.Name == "auth_refresh" {
			key = fmt.Sprintf("ratelimit:token-refresh:ip:%s", clientIP)
		}

		allowed, err := limiter.AllowFixedWindow(c.Request.Context(), []string{key}, policy.Limit, policy.TTL)
		if err != nil || allowed {
			c.Next()
			return
		}
		writeRateLimitError(c, policy)
	}
}

func rateLimitEnabled(runtime *config.Runtime) bool {
	return runtime != nil && runtime.Snapshot().RateLimitEnabled
}

func authenticatedRateLimitRPM(runtime *config.Runtime) int {
	if runtime == nil {
		return defaultAuthenticatedRateLimitRPM
	}
	if value := runtime.Snapshot().RateLimitRPM; value > 0 {
		return value
	}
	return defaultAuthenticatedRateLimitRPM
}

func publicAuthRateLimitRPM(runtime *config.Runtime) int {
	if runtime == nil {
		return defaultPublicAuthRateLimitRPM
	}
	if value := runtime.Snapshot().PublicAuthRateLimitRPM; value > 0 {
		return value
	}
	return defaultPublicAuthRateLimitRPM
}

func authenticatedRateLimitPolicy(c *gin.Context, baseRPM int) rateLimitPolicy {
	route := normalizedRoutePath(c)
	method := c.Request.Method

	switch {
	case isMediaGenerationRoute(method, route):
		return newRateLimitPolicy("media_generation", atLeast(baseRPM/2, 30), "rate limit exceeded")
	case isMessageGenerationRoute(method, route):
		return newRateLimitPolicy("message_generation", atLeast(baseRPM, defaultAuthenticatedRateLimitRPM), "rate limit exceeded")
	case isFileUploadRoute(method, route):
		return newRateLimitPolicy("file_upload", atLeast(baseRPM*2, 120), "rate limit exceeded")
	case isPollingRoute(method, route):
		return newRateLimitPolicy("polling", atLeast(baseRPM*10, 600), "rate limit exceeded")
	case method == http.MethodGet:
		return newRateLimitPolicy("read", atLeast(baseRPM*10, 600), "rate limit exceeded")
	case method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete:
		return newRateLimitPolicy("write", atLeast(baseRPM*4, 240), "rate limit exceeded")
	default:
		return newRateLimitPolicy("general", atLeast(baseRPM*6, 360), "rate limit exceeded")
	}
}

func publicRateLimitPolicy(c *gin.Context, baseRPM int) rateLimitPolicy {
	route := normalizedRoutePath(c)
	method := c.Request.Method

	switch {
	case route == "/auth/refresh":
		return newRateLimitPolicy("auth_refresh", atLeast(baseRPM*10, 300), "too many refresh attempts")
	case isPublicAuthReadRoute(method, route):
		return newRateLimitPolicy("public_auth_read", atLeast(baseRPM*10, 300), "rate limit exceeded")
	case strings.HasPrefix(route, "/auth/"):
		return newRateLimitPolicy("public_auth", baseRPM, "too many authentication attempts")
	case strings.HasPrefix(route, "/billing/payments/"):
		return newRateLimitPolicy("payment_callback", atLeast(baseRPM*20, 600), "rate limit exceeded")
	case route == "/settings/login-page" || strings.HasPrefix(route, "/shared-conversations/"):
		return newRateLimitPolicy("public_read", atLeast(baseRPM*20, 600), "rate limit exceeded")
	default:
		return newRateLimitPolicy("public", atLeast(baseRPM*10, 300), "rate limit exceeded")
	}
}

func newRateLimitPolicy(name string, limit int, message string) rateLimitPolicy {
	if limit <= 0 {
		limit = defaultAuthenticatedRateLimitRPM
	}
	return rateLimitPolicy{
		Name:    name,
		Limit:   limit,
		Window:  rateLimitWindow,
		TTL:     2 * rateLimitWindow,
		Message: message,
	}
}

func normalizedRoutePath(c *gin.Context) string {
	route := strings.TrimSpace(c.FullPath())
	if route == "" && c.Request != nil && c.Request.URL != nil {
		route = strings.TrimSpace(c.Request.URL.Path)
	}
	route = strings.TrimPrefix(route, "/api/v1")
	if route == "" {
		return "/"
	}
	if !strings.HasPrefix(route, "/") {
		return "/" + route
	}
	return route
}

func isMessageGenerationRoute(method string, route string) bool {
	return method == http.MethodPost && (strings.HasSuffix(route, "/messages") ||
		strings.HasSuffix(route, "/messages/stream") ||
		strings.HasSuffix(route, "/cancel"))
}

func isMediaGenerationRoute(method string, route string) bool {
	return method == http.MethodPost && strings.Contains(route, "/media/images/")
}

func isFileUploadRoute(method string, route string) bool {
	return method == http.MethodPost && route == "/files"
}

func isPollingRoute(method string, route string) bool {
	if method != http.MethodGet {
		return false
	}
	return strings.HasSuffix(route, "/processing") ||
		strings.HasSuffix(route, "/runs") ||
		strings.HasSuffix(route, "/stream")
}

func isPublicAuthReadRoute(method string, route string) bool {
	if method != http.MethodGet {
		return false
	}
	return route == "/auth/login-options" || strings.HasSuffix(route, "/logo")
}

func writeRateLimitError(c *gin.Context, policy rateLimitPolicy) {
	retryAfter := int(policy.Window.Seconds())
	if retryAfter <= 0 {
		retryAfter = 60
	}
	c.Header("Retry-After", strconv.Itoa(retryAfter))
	c.Header("X-RateLimit-Limit", strconv.Itoa(policy.Limit))
	c.Header("X-RateLimit-Remaining", "0")
	c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(policy.Window).Unix(), 10))
	response.Error(c, http.StatusTooManyRequests, policy.Message)
	c.Abort()
}

func atLeast(value int, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}
