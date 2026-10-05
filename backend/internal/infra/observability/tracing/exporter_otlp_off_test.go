//go:build nootlp

package tracing

import (
	"context"
	"errors"
	"testing"
)

// 不含 OTLP exporter 的构建（桌面端 sidecar）遇到已配置的链路追踪时只关闭追踪，不阻止启动。
func TestInitWithoutExporterDisablesTracing(t *testing.T) {
	enabled := true
	shutdown, err := Init(context.Background(), Config{Enabled: &enabled, Endpoint: "collector:4317", Protocol: "grpc"})
	if !errors.Is(err, ErrExporterUnavailable) {
		t.Fatalf("expected ErrExporterUnavailable, got %v", err)
	}
	if shutdown == nil || shutdown(context.Background()) != nil {
		t.Fatal("expected a no-op shutdown function")
	}
}
