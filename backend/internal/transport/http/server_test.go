package httpx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/gin-gonic/gin"
)

func TestVersionEndpointIsPublicAndUncached(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine, err := NewEngine(config.NewRuntime(config.Config{AppName: "test", JWTSecret: "test-jwt-secret-value"}), nil, Modules{}, nil, nil)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate" {
		t.Fatalf("expected version no-store cache header, got %q", got)
	}
	if got := recorder.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("expected version pragma no-cache, got %q", got)
	}
	if !strings.Contains(recorder.Body.String(), `"buildID"`) {
		t.Fatalf("expected version response to include buildID, got %q", recorder.Body.String())
	}
}

func TestFrontendStaticFallbackServesExportedPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("index"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "chat.html"), []byte("chat page"), 0o644); err != nil {
		t.Fatalf("write chat: %v", err)
	}

	engine := gin.New()
	registerFrontendStatic(engine, root, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/chat?conversation_id=demo", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if strings.TrimSpace(recorder.Body.String()) != "chat page" {
		t.Fatalf("expected chat page, got %q", recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("expected exported page no-cache, got %q", got)
	}
}

func TestFrontendStaticCachesNextExportData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "__next._tree.txt"), []byte("tree"), 0o644); err != nil {
		t.Fatalf("write next data: %v", err)
	}

	engine := gin.New()
	registerFrontendStatic(engine, root, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/__next._tree.txt?conversation_id=demo&_rsc=abc", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=86400, stale-while-revalidate=604800" {
		t.Fatalf("expected next export data cache header, got %q", got)
	}
}

func TestFrontendStaticCachesImmutableBuildAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	chunkDir := filepath.Join(root, "_next", "static", "chunks")
	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		t.Fatalf("create chunk dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(chunkDir, "app.js"), []byte("chunk"), 0o644); err != nil {
		t.Fatalf("write chunk: %v", err)
	}

	engine := gin.New()
	registerFrontendStatic(engine, root, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/_next/static/chunks/app.js", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("expected immutable cache header, got %q", got)
	}
}

func TestFrontendStaticFallbackSkipsAPIPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("index"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	engine := gin.New()
	registerFrontendStatic(engine, root, nil)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
	engine.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "index") {
		t.Fatalf("api path should not serve frontend fallback: %q", recorder.Body.String())
	}
}

func TestSwaggerEnabledByEnvironment(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{env: "", want: false},
		{env: "dev", want: true},
		{env: " DEV ", want: true},
		{env: "development", want: true},
		{env: "staging", want: false},
		{env: "prod", want: false},
		{env: "production", want: false},
		{env: " PROD ", want: false},
	}

	for _, tt := range tests {
		if got := swaggerEnabled(tt.env); got != tt.want {
			t.Fatalf("swaggerEnabled(%q) = %v, want %v", tt.env, got, tt.want)
		}
	}
}
