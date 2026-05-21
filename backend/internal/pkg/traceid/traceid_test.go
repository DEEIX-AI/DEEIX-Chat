package traceid

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestFromContextPrefersOpenTelemetryTraceID(t *testing.T) {
	otelTraceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatal(err)
	}
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{TraceID: otelTraceID, SpanID: trace.SpanID{1}, TraceFlags: trace.FlagsSampled})
	ctx := trace.ContextWithSpanContext(WithTraceID(context.Background(), "custom"), spanCtx)

	if got := FromContext(ctx); got != otelTraceID.String() {
		t.Fatalf("expected otel trace id, got %q", got)
	}
}

func TestFromContextFallsBackToCustomTraceID(t *testing.T) {
	ctx := WithTraceID(context.Background(), "custom")

	if got := FromContext(ctx); got != "custom" {
		t.Fatalf("expected custom trace id, got %q", got)
	}
}

func TestValid(t *testing.T) {
	if !Valid("4bf92f3577b34da6a3ce929d0e0e4736") {
		t.Fatal("expected valid trace id")
	}
	if Valid("00000000000000000000000000000000") {
		t.Fatal("expected zero trace id to be invalid")
	}
	if Valid("not-a-trace-id") {
		t.Fatal("expected malformed trace id to be invalid")
	}
}
