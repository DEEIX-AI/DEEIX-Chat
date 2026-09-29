package conversation

import (
	"context"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"go.opentelemetry.io/otel/trace"
)

type generationContextTestKey struct{}

func TestDetachGenerationContextDropsRequestSpanButKeepsValues(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	spanID, _ := trace.SpanIDFromHex("0123456789abcdef")
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled})
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), generationContextTestKey{}, "request-value"))
	parent = trace.ContextWithSpanContext(parent, spanCtx)

	detached := detachGenerationContext(parent)
	cancel()

	if err := detached.Err(); err != nil {
		t.Fatalf("detached context inherited cancellation: %v", err)
	}
	if got := detached.Value(generationContextTestKey{}); got != "request-value" {
		t.Fatalf("detached value = %#v, want request-value", got)
	}
	if trace.SpanContextFromContext(detached).IsValid() {
		t.Fatal("detached context still carries the request span")
	}
	if got := traceid.FromContext(detached); got != traceID.String() {
		t.Fatalf("trace id = %q, want %q", got, traceID.String())
	}
}
