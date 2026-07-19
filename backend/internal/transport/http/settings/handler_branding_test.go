package settings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/gin-gonic/gin"
)

func TestGetBrandingReturnsRuntimeConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{runtime: config.NewRuntime(config.Config{
		AppName:                    "Example Backend",
		BrandTitle:                 "Example Chat",
		BrandShortName:             "Example",
		BrandDescription:           "Example description",
		BrandLogoURL:               "https://cdn.example.com/logo.svg",
		BrandFaviconURL:            "https://cdn.example.com/favicon.ico",
		BrandPWAIcon192URL:         "https://cdn.example.com/icon-192.png",
		BrandPWAIcon512URL:         "https://cdn.example.com/icon-512.png",
		BrandPWAMaskableIcon512URL: "https://cdn.example.com/icon-maskable.png",
		BrandAppleTouchIcon180URL:  "https://cdn.example.com/apple-touch-icon.png",
	})}
	router := gin.New()
	router.GET("/branding", handler.GetBranding)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/branding", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("unexpected cache policy: %q", got)
	}
	var body struct {
		Data BrandingResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Title != "Example Chat" || body.Data.LogoURL != "https://cdn.example.com/logo.svg" {
		t.Fatalf("unexpected branding response: %+v", body.Data)
	}
}

func TestGetBrandingManifestUsesBrandingIcons(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{runtime: config.NewRuntime(config.Config{
		AppName:                    "Example Backend",
		BrandTitle:                 "Example Chat",
		BrandShortName:             "Example",
		BrandDescription:           "Example description",
		PublicWebBaseURL:           "https://chat.example.com/deeix",
		BrandPWAIcon192URL:         "/pwa/icon-192.png",
		BrandPWAIcon512URL:         "https://cdn.example.com/icon-512.png",
		BrandPWAMaskableIcon512URL: "https://cdn.example.com/icon-maskable.png",
	})}
	router := gin.New()
	NewModule(handler).RegisterFrontendRoutes(router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/manifest+json; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", got)
	}
	var manifest BrandingManifestResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.Name != "Example Chat" || len(manifest.Icons) != 3 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if manifest.StartURL != "https://chat.example.com/chat" || manifest.Icons[0].Src != "https://chat.example.com/pwa/icon-192.png" {
		t.Fatalf("unexpected public web URLs: %+v", manifest)
	}
	if manifest.Icons[2].Purpose != "maskable" || manifest.Icons[2].Src != "https://cdn.example.com/icon-maskable.png" {
		t.Fatalf("unexpected maskable icon: %+v", manifest.Icons[2])
	}
}
