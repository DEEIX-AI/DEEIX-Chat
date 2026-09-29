package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/gin-gonic/gin"
)

func capabilitiesOf(t *testing.T, cfg config.Config) (map[string]bool, http.Header) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg.AppName = "test"
	cfg.JWTSecret = "test-jwt-secret-value"
	engine, err := NewEngine(config.NewRuntime(cfg), nil, Modules{}, nil, nil)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data struct {
			Features map[string]bool `json:"features"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Data.Features, recorder.Header()
}

// 契约（docs/ARCHITECTURE.md §4）：公开、可缓存、扁平布尔对象。
func TestCapabilitiesEndpoint(t *testing.T) {
	features, header := capabilitiesOf(t, config.Config{})
	if got := header.Get("Cache-Control"); got != "public, max-age=300" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if want := (config.Config{}).Capabilities().Flags(); len(features) != len(want) {
		t.Fatalf("expected %d features, got %d: %v", len(want), len(features), features)
	}
	for name, enabled := range features {
		if !enabled {
			t.Errorf("server mode: %s should be true", name)
		}
	}

	local, _ := capabilitiesOf(t, config.Config{LocalMode: true})
	for name, enabled := range local {
		if want := name == "usageMetering"; enabled != want {
			t.Errorf("local mode: %s = %v, want %v", name, enabled, want)
		}
	}
}
