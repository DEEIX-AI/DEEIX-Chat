package background

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestWaitDrainsStartedTasks(t *testing.T) {
	release := make(chan struct{})
	Go(zap.NewNop(), "t", func() { <-release })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := Wait(ctx); err == nil {
		t.Fatal("expected timeout while task is running")
	}

	close(release)
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	if err := Wait(ctx2); err != nil {
		t.Fatalf("expected drained, got %v", err)
	}
}
