//go:build !nootlp

package tracing

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// newExporter 按配置的协议创建 OTLP span exporter。
func newExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	switch cfg.Protocol {
	case "grpc":
		return otlptracegrpc.New(ctx, grpcExporterOptions(cfg)...)
	case "http":
		return otlptracehttp.New(ctx, httpExporterOptions(cfg)...)
	default:
		return nil, fmt.Errorf("unsupported otel exporter protocol %q", cfg.Protocol)
	}
}

func grpcExporterOptions(cfg Config) []otlptracegrpc.Option {
	options := make([]otlptracegrpc.Option, 0, 3)
	if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
		if strings.Contains(endpoint, "://") {
			options = append(options, otlptracegrpc.WithEndpointURL(endpoint))
		} else {
			options = append(options, otlptracegrpc.WithEndpoint(endpoint))
		}
	}
	if headers := parseHeaders(cfg.Headers); len(headers) > 0 {
		options = append(options, otlptracegrpc.WithHeaders(headers))
	}
	if cfg.Insecure {
		options = append(options, otlptracegrpc.WithInsecure())
	}
	return options
}

func httpExporterOptions(cfg Config) []otlptracehttp.Option {
	options := make([]otlptracehttp.Option, 0, 3)
	if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
		if strings.Contains(endpoint, "://") {
			options = append(options, otlptracehttp.WithEndpointURL(endpoint))
		} else {
			options = append(options, otlptracehttp.WithEndpoint(endpoint))
		}
	}
	if headers := parseHeaders(cfg.Headers); len(headers) > 0 {
		options = append(options, otlptracehttp.WithHeaders(headers))
	}
	if cfg.Insecure {
		options = append(options, otlptracehttp.WithInsecure())
	}
	return options
}
