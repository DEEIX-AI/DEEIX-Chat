package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestAccessLogSkipsHealthz(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zap.InfoLevel)
	router := gin.New()
	router.Use(AccessLog(zap.New(core)))
	router.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/api/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if logs.Len() != 0 {
		t.Fatalf("expected /healthz to skip access log, got %d entries", logs.Len())
	}

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/ping", nil))
	if logs.Len() != 1 {
		t.Fatalf("expected regular route to emit access log, got %d entries", logs.Len())
	}
	entry := logs.All()[0]
	if len(entry.Context) != 0 {
		t.Fatalf("expected access log context fields to be empty, got %#v", entry.Context)
	}
	if entry.Message == "" || entry.Message == "http_request" {
		t.Fatalf("expected request details in message, got %q", entry.Message)
	}
}
