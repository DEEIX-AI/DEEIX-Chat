package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTrustedProxyHeadersScopesConfigurationToMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	trusted, err := TrustedProxyHeaders([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	untrusted, err := TrustedProxyHeaders([]string{"192.0.2.0/24"})
	if err != nil {
		t.Fatal(err)
	}

	assertCountry := func(handler gin.HandlerFunc, remoteAddress string, want string) {
		t.Helper()
		engine := gin.New()
		engine.Use(handler)
		engine.GET("/", func(c *gin.Context) {
			c.String(http.StatusOK, ResolveSessionAuditContext(c).CountryCode)
		})
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = remoteAddress
		request.Header.Set("CF-IPCountry", "CN")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, request)
		if recorder.Body.String() != want {
			t.Fatalf("country = %q, want %q", recorder.Body.String(), want)
		}
	}

	assertCountry(trusted, "10.1.2.3:1234", "CN")
	assertCountry(untrusted, "10.1.2.3:1234", "")
}

func TestTrustedProxyHeadersRejectsInvalidPrefix(t *testing.T) {
	if _, err := TrustedProxyHeaders([]string{"not-an-address"}); err == nil {
		t.Fatal("expected invalid trusted proxy configuration to fail")
	}
}
