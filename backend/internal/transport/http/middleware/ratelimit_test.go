package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/gin-gonic/gin"
)

type rateLimitCall struct {
	Key    string
	Keys   []string
	Limit  int
	Window time.Duration
	TTL    time.Duration
}

type recordingRateLimiter struct {
	allow   bool
	sliding []rateLimitCall
	fixed   []rateLimitCall
}

func (r *recordingRateLimiter) AllowSlidingWindow(_ context.Context, key string, limit int, window time.Duration, ttl time.Duration) (bool, error) {
	r.sliding = append(r.sliding, rateLimitCall{Key: key, Limit: limit, Window: window, TTL: ttl})
	return r.allow, nil
}

func (r *recordingRateLimiter) AllowFixedWindow(_ context.Context, keys []string, limit int, ttl time.Duration) (bool, error) {
	r.fixed = append(r.fixed, rateLimitCall{Keys: keys, Limit: limit, TTL: ttl})
	return r.allow, nil
}

func TestRateLimitUsesSeparateAuthenticatedBuckets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &recordingRateLimiter{allow: true}
	router := gin.New()
	group := router.Group("/api/v1")
	group.Use(testUserContext("user"))
	group.Use(RateLimit(limiter, config.NewRuntime(config.Config{RateLimitEnabled: true, RateLimitRPM: 60})))
	group.GET("/models", okHandler)
	group.POST("/conversations/:id/messages/stream", okHandler)

	performRequest(router, http.MethodGet, "/api/v1/models")
	performRequest(router, http.MethodPost, "/api/v1/conversations/1/messages/stream")

	if len(limiter.sliding) != 2 {
		t.Fatalf("expected 2 sliding-window calls, got %d", len(limiter.sliding))
	}
	if limiter.sliding[0].Key != "ratelimit:user:42:read" || limiter.sliding[0].Limit != 600 {
		t.Fatalf("unexpected read bucket: %+v", limiter.sliding[0])
	}
	if limiter.sliding[1].Key != "ratelimit:user:42:message_generation" || limiter.sliding[1].Limit != 60 {
		t.Fatalf("unexpected generation bucket: %+v", limiter.sliding[1])
	}
}

func TestRateLimitReturnsRetryMetadataWhenBlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &recordingRateLimiter{allow: false}
	router := gin.New()
	group := router.Group("/api/v1")
	group.Use(testUserContext("user"))
	group.Use(RateLimit(limiter, config.NewRuntime(config.Config{RateLimitEnabled: true, RateLimitRPM: 60})))
	group.GET("/models", okHandler)

	response := performRequest(router, http.MethodGet, "/api/v1/models")

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", response.Code)
	}
	if response.Header().Get("Retry-After") != "60" {
		t.Fatalf("expected Retry-After=60, got %q", response.Header().Get("Retry-After"))
	}
	if response.Header().Get("X-RateLimit-Limit") != "600" {
		t.Fatalf("expected X-RateLimit-Limit=600, got %q", response.Header().Get("X-RateLimit-Limit"))
	}
}

func TestPublicRateLimitUsesRiskSpecificBuckets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &recordingRateLimiter{allow: true}
	router := gin.New()
	group := router.Group("/api/v1")
	group.Use(PublicAuthRateLimit(limiter, config.NewRuntime(config.Config{RateLimitEnabled: true, PublicAuthRateLimitRPM: 30})))
	group.GET("/settings/login-page", okHandler)
	group.POST("/auth/refresh", okHandler)
	group.POST("/auth/login", okHandler)

	performRequest(router, http.MethodGet, "/api/v1/settings/login-page")
	performRequest(router, http.MethodPost, "/api/v1/auth/refresh")
	performRequest(router, http.MethodPost, "/api/v1/auth/login")

	if len(limiter.fixed) != 3 {
		t.Fatalf("expected 3 fixed-window calls, got %d", len(limiter.fixed))
	}
	assertFixedCall(t, limiter.fixed[0], "public_read", 600)
	assertFixedCall(t, limiter.fixed[1], "ratelimit:token-refresh", 300)
	assertFixedCall(t, limiter.fixed[2], "public_auth", 30)
}

func TestRateLimitSkipsAdminUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &recordingRateLimiter{allow: false}
	router := gin.New()
	group := router.Group("/api/v1")
	group.Use(testUserContext("admin"))
	group.Use(RateLimit(limiter, config.NewRuntime(config.Config{RateLimitEnabled: true, RateLimitRPM: 60})))
	group.GET("/models", okHandler)

	response := performRequest(router, http.MethodGet, "/api/v1/models")

	if response.Code != http.StatusOK {
		t.Fatalf("expected admin request to pass, got %d", response.Code)
	}
	if len(limiter.sliding) != 0 {
		t.Fatalf("expected no limiter call for admin, got %d", len(limiter.sliding))
	}
}

func TestRateLimitCanBeDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &recordingRateLimiter{allow: false}
	runtime := config.NewRuntime(config.Config{
		RateLimitEnabled:       false,
		RateLimitRPM:           60,
		PublicAuthRateLimitRPM: 30,
	})

	authRouter := gin.New()
	authGroup := authRouter.Group("/api/v1")
	authGroup.Use(testUserContext("user"))
	authGroup.Use(RateLimit(limiter, runtime))
	authGroup.GET("/models", okHandler)
	authResponse := performRequest(authRouter, http.MethodGet, "/api/v1/models")
	if authResponse.Code != http.StatusOK {
		t.Fatalf("expected authenticated request to pass when disabled, got %d", authResponse.Code)
	}

	publicRouter := gin.New()
	publicGroup := publicRouter.Group("/api/v1")
	publicGroup.Use(PublicAuthRateLimit(limiter, runtime))
	publicGroup.POST("/auth/login", okHandler)
	publicResponse := performRequest(publicRouter, http.MethodPost, "/api/v1/auth/login")
	if publicResponse.Code != http.StatusOK {
		t.Fatalf("expected public request to pass when disabled, got %d", publicResponse.Code)
	}
	if len(limiter.sliding) != 0 || len(limiter.fixed) != 0 {
		t.Fatalf("expected limiter to be skipped when disabled, got sliding=%d fixed=%d", len(limiter.sliding), len(limiter.fixed))
	}
}

func assertFixedCall(t *testing.T, call rateLimitCall, keyPart string, limit int) {
	t.Helper()
	if len(call.Keys) != 1 {
		t.Fatalf("expected one key, got %+v", call.Keys)
	}
	if !strings.Contains(call.Keys[0], keyPart) {
		t.Fatalf("expected key to contain %q, got %q", keyPart, call.Keys[0])
	}
	if call.Limit != limit {
		t.Fatalf("expected limit %d, got %d", limit, call.Limit)
	}
}

func testUserContext(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ContextKeyUserID, uint(42))
		c.Set(ContextKeyUserRole, role)
		c.Next()
	}
}

func okHandler(c *gin.Context) {
	c.Status(http.StatusOK)
}

func performRequest(router http.Handler, method string, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
