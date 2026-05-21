package middleware

import (
	"net/http"
	"net/http/httptest"
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
