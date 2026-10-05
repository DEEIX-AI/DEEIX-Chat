//go:build nootlp

package tracing

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// newExporter 在 -tags nootlp 构建（桌面端本地模式）中不可用：本地模式不导出链路，
// OTLP exporter 及其 gRPC / protobuf 依赖不编入二进制。
func newExporter(context.Context, Config) (sdktrace.SpanExporter, error) {
	return nil, ErrExporterUnavailable
}
