package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHTTPSRedirectRedirectsProductionHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(HTTPSRedirect("production"))
	router.GET("/chat", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://chat.example.com/chat?id=demo", nil))

	if recorder.Code != http.StatusPermanentRedirect {
		t.Fatalf("expected status 308, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Location"); got != "https://chat.example.com/chat?id=demo" {
		t.Fatalf("unexpected redirect location: %q", got)
	}
}

func TestHTTPSRedirectAllowsHTTPSAndHealthChecks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(HTTPSRedirect("prod"))
	router.GET("/chat", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/healthz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	httpsRequest := httptest.NewRequest(http.MethodGet, "https://chat.example.com/chat", nil)
	httpsRequest.TLS = &tls.ConnectionState{}
	httpsRecorder := httptest.NewRecorder()
	router.ServeHTTP(httpsRecorder, httpsRequest)
	if httpsRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected HTTPS request to pass, got %d", httpsRecorder.Code)
	}

	healthRecorder := httptest.NewRecorder()
	router.ServeHTTP(healthRecorder, httptest.NewRequest(http.MethodGet, "http://chat.example.com/healthz", nil))
	if healthRecorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP health check to pass, got %d", healthRecorder.Code)
	}
}

func TestHTTPSRedirectAllowsForwardedHTTPS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(HTTPSRedirect("prod"))
	router.GET("/chat", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "http://chat.example.com/chat", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected forwarded HTTPS request to pass, got %d", recorder.Code)
	}
}
