package httpx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

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
