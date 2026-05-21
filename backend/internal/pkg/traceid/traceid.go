package traceid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

type contextKey struct{}

// Generate 生成 32 字符 hex trace_id，对齐 OpenTelemetry W3C Trace Context 128-bit。
func Generate() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// WithTraceID 将 trace_id 注入 context。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, contextKey{}, traceID)
}

// Valid 判断字符串是否为有效的 OpenTelemetry TraceID。
func Valid(value string) bool {
	traceID, err := trace.TraceIDFromHex(strings.TrimSpace(value))
	return err == nil && traceID.IsValid()
}

// FromContext 从 context 提取 trace_id。
func FromContext(ctx context.Context) string {
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		return spanCtx.TraceID().String()
	}
	if v, ok := ctx.Value(contextKey{}).(string); ok {
		return v
	}
	return ""
}
