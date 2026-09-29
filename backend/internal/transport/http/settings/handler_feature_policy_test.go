package settings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/gin-gonic/gin"
)

func getFeaturePolicyForTest(t *testing.T, cfg config.Config) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := &Handler{runtime: config.NewRuntime(cfg)}
	router := gin.New()
	router.GET("/settings/feature-policy", handler.GetFeaturePolicy)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/settings/feature-policy", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body.Data
}

func TestGetFeaturePolicyReturnsDesktopDownloadEntry(t *testing.T) {
	data := getFeaturePolicyForTest(t, config.Config{
		KnowledgeBaseEnabled:   true,
		ProcessTraceEnabled:    false,
		DesktopDownloadEnabled: true,
		DesktopDownloadURL:     "https://example.com/download",
	})
	if data["knowledgeBaseEnabled"] != true || data["processTraceEnabled"] != false {
		t.Fatalf("unexpected feature switches: %+v", data)
	}
	if data["desktopDownloadEnabled"] != true || data["desktopDownloadURL"] != "https://example.com/download" {
		t.Fatalf("unexpected desktop download entry: %+v", data)
	}
}

func TestGetFeaturePolicyHidesDesktopDownloadEntry(t *testing.T) {
	cases := map[string]config.Config{
		"disabled":        {DesktopDownloadEnabled: false, DesktopDownloadURL: "https://example.com/download"},
		"invalid url":     {DesktopDownloadEnabled: true, DesktopDownloadURL: "javascript:alert(1)"},
		"credentials url": {DesktopDownloadEnabled: true, DesktopDownloadURL: "https://user:pass@example.com/download"},
		"empty url":       {DesktopDownloadEnabled: true, DesktopDownloadURL: ""},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			data := getFeaturePolicyForTest(t, cfg)
			if data["desktopDownloadEnabled"] != false || data["desktopDownloadURL"] != "" {
				t.Fatalf("expected hidden desktop download entry, got %+v", data)
			}
		})
	}
}
