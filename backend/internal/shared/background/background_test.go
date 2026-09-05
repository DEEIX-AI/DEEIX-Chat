package background

import (
	"context"
	"testing"
	"time"
)

type contextKey string

func TestDetachPreservesValuesWithoutParentCancellation(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.WithValue(context.Background(), contextKey("request"), "request-123"))
	detached := Detach(parent)
	cancelParent()

	if got := detached.Value(contextKey("request")); got != "request-123" {
		t.Fatalf("detached value = %#v, want request-123", got)
	}
	if err := detached.Err(); err != nil {
		t.Fatalf("detached context inherited parent cancellation: %v", err)
	}
	if _, ok := detached.Deadline(); ok {
		t.Fatal("detached context inherited parent deadline")
	}
}

func TestWithTimeoutDetachesParentAndAddsIndependentDeadline(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.WithValue(context.Background(), contextKey("trace"), "trace-123"))
	ctx, cancel := WithTimeout(parent, time.Hour)
	defer cancel()
	cancelParent()

	if got := ctx.Value(contextKey("trace")); got != "trace-123" {
		t.Fatalf("timeout context value = %#v, want trace-123", got)
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("timeout context inherited parent cancellation: %v", err)
	}
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("timeout context has no deadline")
	}
}
