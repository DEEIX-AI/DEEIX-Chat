package llm

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestUpstreamDebugSnapshotRedactsInlineBinaryPayloads(t *testing.T) {
	base64Data := strings.Repeat("a", 4096)
	requestBody := []byte(`{"model":"vision-model","messages":[{"content":[{"type":"image_url","image_url":{"url":"data:image/webp;base64,` + base64Data + `"}}]}],"inlineData":{"mimeType":"image/png","data":"` + base64Data + `"},"source":{"type":"base64","media_type":"image/jpeg","data":"` + base64Data + `"}}`)
	responseBody := []byte(`{"data":[{"b64_json":"` + base64Data + `"}],"error":{"message":"unsupported image"}}`)
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}

	debug := upstreamDebugSnapshot(req, requestBody, &http.Response{StatusCode: http.StatusBadRequest}, responseBody)
	if debug == nil {
		t.Fatal("expected debug snapshot")
	}
	for _, body := range []string{debug.Request.Body, debug.Response.Body} {
		if strings.Contains(body, base64Data) || strings.Contains(body, ";base64,") {
			t.Fatalf("expected binary payload to be redacted, got %s", body)
		}
		if !json.Valid([]byte(body)) {
			t.Fatalf("expected valid JSON debug body, got %s", body)
		}
	}
	if debug.Request.RedactedParts != 3 || debug.Response.RedactedParts != 1 {
		t.Fatalf("unexpected redaction counts: request=%d response=%d", debug.Request.RedactedParts, debug.Response.RedactedParts)
	}
	if debug.Request.BodyBytes != len(requestBody) || debug.Response.BodyBytes != len(responseBody) {
		t.Fatalf("unexpected original body sizes: request=%d response=%d", debug.Request.BodyBytes, debug.Response.BodyBytes)
	}
}

func TestUpstreamDebugSnapshotBoundsOversizedTextBody(t *testing.T) {
	requestBody := []byte(`{"model":"text-model","input":"` + strings.Repeat("x", maxUpstreamDebugBodyBytes) + `"}`)
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	if err != nil {
		t.Fatal(err)
	}

	debug := upstreamDebugSnapshot(req, requestBody, nil, nil)
	if !debug.Request.BodyTruncated {
		t.Fatal("expected oversized request body to be omitted")
	}
	if len(debug.Request.Body) > maxUpstreamDebugBodyBytes || !json.Valid([]byte(debug.Request.Body)) {
		t.Fatalf("expected bounded valid JSON summary, got %q", debug.Request.Body)
	}
	if strings.Contains(debug.Request.Body, strings.Repeat("x", 1024)) {
		t.Fatal("expected oversized text not to remain in debug body")
	}
}

func TestUpstreamDebugSnapshotPreservesAndSanitizesSSE(t *testing.T) {
	base64Data := strings.Repeat("Y", 4096)
	raw := "event: response.error\n" +
		`data: {"type":"response.error","image":{"b64_json":"` + base64Data + `"},"error":{"message":"bad request"}}` + "\n\n"
	req, err := http.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	if err != nil {
		t.Fatal(err)
	}

	debug := upstreamDebugSnapshot(req, []byte(`{"model":"gpt"}`), &http.Response{StatusCode: http.StatusBadRequest}, []byte(raw))
	if strings.Contains(debug.Response.Body, base64Data) || !strings.Contains(debug.Response.Body, "bad request") {
		t.Fatalf("expected SSE error preserved without binary data, got %q", debug.Response.Body)
	}
	if debug.Response.RedactedParts != 1 {
		t.Fatalf("expected one SSE redaction, got %d", debug.Response.RedactedParts)
	}
}

func TestUpstreamDebugSnapshotRedactsSmallAndMultipleDataURLs(t *testing.T) {
	raw := []byte(`plain data:text/plain,hello data:image/png;base64,eA==`)
	result := sanitizeUpstreamDebugBody(raw)
	if !strings.Contains(result.Body, "data:text/plain,hello") {
		t.Fatalf("expected non-base64 data URI to remain, got %q", result.Body)
	}
	if strings.Contains(result.Body, "data:image/png;base64,eA==") || !strings.Contains(result.Body, "binary omitted") {
		t.Fatalf("expected second data URI to be redacted, got %q", result.Body)
	}
	if result.RedactedParts != 1 {
		t.Fatalf("redacted parts = %d, want 1", result.RedactedParts)
	}

	knownField := sanitizeUpstreamDebugBody([]byte(`{"source":{"type":"base64","data":"eA=="},"b64_json":"eA=="}`))
	if strings.Contains(knownField.Body, "eA==") || knownField.RedactedParts != 2 {
		t.Fatalf("expected small known binary fields to be redacted, got %q (%d parts)", knownField.Body, knownField.RedactedParts)
	}

	nested := sanitizeUpstreamDebugBody([]byte(`{"body":"{\"messages\":[{\"content\":[{\"type\":\"image_url\",\"image_url\":{\"url\":\"data:image/png;base64,eA==\"}}]}]}"}`))
	if strings.Contains(nested.Body, "data:image/png;base64,eA==") || nested.RedactedParts != 1 {
		t.Fatalf("expected nested JSON string binary to be redacted, got %q (%d parts)", nested.Body, nested.RedactedParts)
	}

	ordinary := sanitizeUpstreamDebugBody([]byte(`{"data":"test"}`))
	if strings.Contains(ordinary.Body, "binary omitted") || ordinary.RedactedParts != 0 {
		t.Fatalf("ordinary data field should remain unchanged, got %q (%d parts)", ordinary.Body, ordinary.RedactedParts)
	}
}
