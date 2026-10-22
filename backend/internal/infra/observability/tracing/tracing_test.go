package tracing

import (
	"testing"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

func TestSemconvMatchesSDKResourceSchema(t *testing.T) {
	if got, want := resource.Default().SchemaURL(), semconv.SchemaURL; got != want {
		t.Fatalf("tracing.go imports semconv %s but otel/sdk builds resources as %s; bump the import", want, got)
	}
}

func TestEnabledUsesEndpointWhenFlagUnset(t *testing.T) {
	if !enabled(Config{Endpoint: "127.0.0.1:4317"}) {
		t.Fatal("expected endpoint to enable tracing when enabled flag is unset")
	}
}

func TestEnabledFlagOverridesEndpoint(t *testing.T) {
	disabled := false
	if enabled(Config{Enabled: &disabled, Endpoint: "127.0.0.1:4317"}) {
		t.Fatal("expected explicit disabled flag to override endpoint")
	}
}

func TestInitRequiresEndpointWhenExplicitlyEnabled(t *testing.T) {
	enabled := true
	if _, err := Init(t.Context(), Config{Enabled: &enabled}); err == nil {
		t.Fatal("expected endpoint validation error")
	}
}

func TestParseHeaders(t *testing.T) {
	headers := parseHeaders("authorization=Bearer token, x-tenant = deeix-chat, invalid")
	if headers["authorization"] != "Bearer token" {
		t.Fatalf("unexpected authorization header: %q", headers["authorization"])
	}
	if headers["x-tenant"] != "deeix-chat" {
		t.Fatalf("unexpected tenant header: %q", headers["x-tenant"])
	}
	if _, ok := headers["invalid"]; ok {
		t.Fatal("expected malformed header item to be ignored")
	}
}
