package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func runCORS(t *testing.T, allowOrigin string, method string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, router := gin.CreateTestContext(recorder)
	router.Use(CORS(allowOrigin))
	router.Handle(method, "/api/v1/auth/refresh", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(method, "/api/v1/auth/refresh", strings.NewReader("{}"))
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	context.Request = request
	router.ServeHTTP(recorder, request)
	return recorder
}

// 桌面端 webview 的 Origin 与服务器不同源，预检必须允许其携带 X-Client-Platform，
// 否则带该头的请求会被浏览器直接拦截。
func TestCORSAllowsClientPlatformHeader(t *testing.T) {
	recorder := runCORS(t, "tauri://localhost", http.MethodOptions, map[string]string{
		"Origin":                         "tauri://localhost",
		"Access-Control-Request-Method":  http.MethodPost,
		"Access-Control-Request-Headers": "content-type,x-client-platform",
	})

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "tauri://localhost" {
		t.Fatalf("allow-origin = %q, want tauri://localhost", got)
	}
	allowed := strings.ToLower(recorder.Header().Get("Access-Control-Allow-Headers"))
	if !strings.Contains(allowed, strings.ToLower(ClientPlatformHeaderName)) {
		t.Fatalf("allow-headers = %q, want it to include %s", allowed, ClientPlatformHeaderName)
	}
	for _, required := range []string{"authorization", "content-type", "x-request-id"} {
		if !strings.Contains(allowed, required) {
			t.Fatalf("allow-headers = %q, want it to keep %s", allowed, required)
		}
	}
}

// ClientPlatformHeaderName 在 middleware 中重复声明，避免 auth 包与 middleware 互相依赖。
const ClientPlatformHeaderName = "X-Client-Platform"

func TestCORSAllowsBothTauriOrigins(t *testing.T) {
	allowOrigin := "http://localhost:3000,tauri://localhost,http://tauri.localhost"
	for _, origin := range []string{"tauri://localhost", "http://tauri.localhost"} {
		recorder := runCORS(t, allowOrigin, http.MethodGet, map[string]string{"Origin": origin})
		if recorder.Code != http.StatusOK {
			t.Fatalf("origin %s: status = %d, want %d", origin, recorder.Code, http.StatusOK)
		}
		if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Fatalf("origin %s: allow-origin = %q", origin, got)
		}
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	recorder := runCORS(t, "tauri://localhost", http.MethodGet, map[string]string{
		"Origin": "https://evil.example.com",
	})

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d for a non-allowlisted origin", recorder.Code, http.StatusForbidden)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("must not echo allow-origin for a rejected origin")
	}
}
