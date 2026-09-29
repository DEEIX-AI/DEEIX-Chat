package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/gin-gonic/gin"
)

func TestRequireFeature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	serve := func(cfg config.Config, feature string) *httptest.ResponseRecorder {
		engine := gin.New()
		engine.GET("/x", RequireFeature(config.NewRuntime(cfg), feature), func(c *gin.Context) {
			c.String(http.StatusOK, "reached")
		})
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/x", nil))
		return recorder
	}

	if got := serve(config.Config{}, "multiUser"); got.Code != http.StatusOK || got.Body.String() != "reached" {
		t.Fatalf("enabled feature must pass through, got %d %q", got.Code, got.Body.String())
	}

	got := serve(config.Config{LocalMode: true}, "multiUser")
	if got.Code != http.StatusNotFound {
		t.Fatalf("disabled feature: expected 404, got %d", got.Code)
	}
	var body struct {
		ErrorCode string                 `json:"errorCode"`
		Details   FeatureDisabledDetails `json:"details"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ErrorCode != "feature.disabled" || body.Details.Feature != "multiUser" {
		t.Fatalf("unexpected body: %s", got.Body.String())
	}

	if got := serve(config.Config{}, "no-such-feature"); got.Code != http.StatusNotFound {
		t.Fatalf("unknown feature name must not pass through, got %d", got.Code)
	}
}

func TestFeatureGateRejectsUnknownNameAtRegistration(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Require must panic for a feature name outside the contract")
		}
	}()
	NewFeatureGate(config.NewRuntime(config.Config{})).Require("multiuser")
}
