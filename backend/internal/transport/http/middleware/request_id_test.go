package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

func TestRequestIDPrefersOpenTelemetryTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	otelTraceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatal(err)
	}
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{TraceID: otelTraceID, SpanID: spanID, TraceFlags: trace.FlagsSampled})

	router := gin.New()
	router.Use(RequestID())
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Trace-ID", "11111111111111111111111111111111")
	req = req.WithContext(trace.ContextWithSpanContext(req.Context(), spanCtx))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if got := recorder.Header().Get("X-Trace-ID"); got != otelTraceID.String() {
		t.Fatalf("expected OTel trace id, got %q", got)
	}
}

func TestRequestIDRejectsInvalidIncomingTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID())
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Trace-ID", "not-a-trace-id")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if got := recorder.Header().Get("X-Trace-ID"); got == "" || got == "not-a-trace-id" {
		t.Fatalf("expected generated trace id, got %q", got)
	}
}

func TestRequestIDOnlyHonorsTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	trusted, err := TrustedProxyHeaders([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(trusted, RequestID())
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	serve := func(remoteAddr string, incoming string) string {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remoteAddr
		req.Header.Set("X-Request-ID", incoming)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder.Header().Get("X-Request-ID")
	}

	if got := serve("203.0.113.9:4000", "victim-request-id"); got == "victim-request-id" || !validRequestID(got) {
		t.Fatalf("untrusted client must not choose the request id, got %q", got)
	}
	if got := serve("10.1.2.3:4000", "gw-7f3a:01.req_9"); got != "gw-7f3a:01.req_9" {
		t.Fatalf("trusted proxy request id should be kept, got %q", got)
	}
	for _, invalid := range []string{"has space", "line\nbreak", "RequestID: forged", strings.Repeat("a", maxRequestIDLength+1)} {
		if got := serve("10.1.2.3:4000", invalid); got == invalid || !validRequestID(got) {
			t.Errorf("invalid request id %q from trusted proxy must be replaced, got %q", invalid, got)
		}
	}
}
