package llm

import (
	"strings"
	"testing"
)

func TestSanitizeUpstreamDebugPayloadRedactsSmallAndMultipleDataURLs(t *testing.T) {
	raw := []byte(`plain data:text/plain,hello data:image/png;base64,eA==`)
	result := SanitizeUpstreamDebugPayload(raw)
	if !strings.Contains(result.Body, "data:text/plain,hello") {
		t.Fatalf("expected non-base64 data URI to remain, got %q", result.Body)
	}
	if strings.Contains(result.Body, "data:image/png;base64,eA==") || !strings.Contains(result.Body, "binary omitted") {
		t.Fatalf("expected second data URI to be redacted, got %q", result.Body)
	}
	if result.RedactedParts != 1 {
		t.Fatalf("redacted parts = %d, want 1", result.RedactedParts)
	}

	knownField := SanitizeUpstreamDebugPayload([]byte(`{"source":{"type":"base64","data":"eA=="},"b64_json":"eA=="}`))
	if strings.Contains(knownField.Body, "eA==") || knownField.RedactedParts != 2 {
		t.Fatalf("expected small known binary fields to be redacted, got %q (%d parts)", knownField.Body, knownField.RedactedParts)
	}

	nested := SanitizeUpstreamDebugPayload([]byte(`{"body":"{\"messages\":[{\"content\":[{\"type\":\"image_url\",\"image_url\":{\"url\":\"data:image/png;base64,eA==\"}}]}]}"}`))
	if strings.Contains(nested.Body, "data:image/png;base64,eA==") || nested.RedactedParts != 1 {
		t.Fatalf("expected nested JSON string binary to be redacted, got %q (%d parts)", nested.Body, nested.RedactedParts)
	}

	ordinary := SanitizeUpstreamDebugPayload([]byte(`{"data":"test"}`))
	if strings.Contains(ordinary.Body, "binary omitted") || ordinary.RedactedParts != 0 {
		t.Fatalf("ordinary data field should remain unchanged, got %q (%d parts)", ordinary.Body, ordinary.RedactedParts)
	}
}
