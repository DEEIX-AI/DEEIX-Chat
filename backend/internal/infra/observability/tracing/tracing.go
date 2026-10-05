package tracing

import (
	"context"
	"errors"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/DEEIX-AI/DEEIX-Chat/backend"

// ErrExporterUnavailable 表示当前二进制以 -tags nootlp 构建、不含 OTLP exporter。Init 此时只安装
// 传播器并返回空操作的释放函数，调用方应把它当作「链路追踪已关闭」的提示而不是启动失败。
var ErrExporterUnavailable = errors.New("tracing: OTLP exporter not compiled into this binary")

// ShutdownFunc 释放一次 tracing 初始化所创建的资源。
type ShutdownFunc func(context.Context) error

type Config struct {
	ServiceName  string
	Enabled      *bool
	Endpoint     string
	Headers      string
	Insecure     bool
	Protocol     string
	SamplingRate float64
}

// Init 为当前单实例进程安装 OpenTelemetry provider，并返回由 App 持有的资源释放函数。
func Init(ctx context.Context, cfg Config) (ShutdownFunc, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	if !enabled(cfg) {
		return func(context.Context) error { return nil }, nil
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, errors.New("otel exporter endpoint is required when tracing is enabled")
	}
	serviceName := cfg.ServiceName
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "deeix-chat"
	}

	exporter, err := newExporter(ctx, cfg)
	if errors.Is(err, ErrExporterUnavailable) {
		return func(context.Context) error { return nil }, err
	}
	if err != nil {
		return nil, err
	}
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.HostName(hostname()),
			semconv.K8SNamespaceName(os.Getenv("KUBERNETES_NAMESPACE")),
			semconv.K8SPodName(os.Getenv("KUBERNETES_POD_NAME")),
			semconv.K8SPodUID(os.Getenv("KUBERNETES_POD_UID")),
		),
	)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(samplingRate(cfg.SamplingRate)))),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}

func Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer(instrumentationName).Start(ctx, name, opts...)
}

func RecordError(span trace.Span, err error) {
	if err == nil || span == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func enabled(cfg Config) bool {
	if cfg.Enabled != nil {
		return *cfg.Enabled
	}
	return strings.TrimSpace(cfg.Endpoint) != ""
}

func parseHeaders(value string) map[string]string {
	parts := strings.Split(value, ",")
	headers := make(map[string]string, len(parts))
	for _, part := range parts {
		key, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		headers[key] = strings.TrimSpace(val)
	}
	return headers
}

func samplingRate(rate float64) float64 {
	if rate < 0 {
		return 0
	}
	if rate > 1 {
		return 1
	}
	return rate
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}
