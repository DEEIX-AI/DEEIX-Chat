package channel

import (
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestSanitizeModelProbeDebugRedactsSensitiveData(t *testing.T) {
	route := llm.RouteConfig{
		APIKey:  "sk-test-secret",
		BaseURL: "https://secret.example.com/v1",
	}
	debug := &llm.UpstreamDebugSnapshot{
		Request: llm.UpstreamDebugRequest{
			Method: "POST",
			Path:   "/v1/responses?key=sk-test-secret",
			Headers: map[string]string{
				"Accept":        "application/json",
				"Authorization": "[redacted]",
				"Content-Type":  "application/json",
				"X-Client-IP":   "127.0.0.1",
			},
			Body: `{"api_key":"sk-test-secret","base_url":"https://secret.example.com/v1","host":"secret.example.com"}`,
		},
		Response: llm.UpstreamDebugResponse{
			StatusCode: 401,
			Headers: map[string]string{
				"Content-Type": "application/json",
				"Server":       "secret-edge",
				"Set-Cookie":   "sid=value",
			},
			Body: `{"error":"sk-test-secret secret.example.com"}`,
		},
	}

	got := sanitizeModelProbeDebug(debug, route)
	if got == nil {
		t.Fatal("expected sanitized debug")
	}

	combined := got.Request.Path + got.Request.Body + got.Response.Body
	if strings.Contains(combined, "sk-test-secret") || strings.Contains(combined, "secret.example.com") {
		t.Fatalf("sensitive data was not redacted: %s", combined)
	}
	if got.Request.Headers["Content-Type"] != "application/json" || got.Request.Headers["Accept"] != "application/json" {
		t.Fatalf("expected safe request headers to be retained: %#v", got.Request.Headers)
	}
	if _, ok := got.Request.Headers["Authorization"]; ok {
		t.Fatalf("authorization header must not be returned: %#v", got.Request.Headers)
	}
	if _, ok := got.Request.Headers["X-Client-IP"]; ok {
		t.Fatalf("client ip header must not be returned: %#v", got.Request.Headers)
	}
	if got.Response.Headers["Content-Type"] != "application/json" {
		t.Fatalf("expected response content type to be retained: %#v", got.Response.Headers)
	}
	if _, ok := got.Response.Headers["Server"]; ok {
		t.Fatalf("server header must not be returned: %#v", got.Response.Headers)
	}
	if _, ok := got.Response.Headers["Set-Cookie"]; ok {
		t.Fatalf("set-cookie header must not be returned: %#v", got.Response.Headers)
	}
}

func TestClassifyModelProbeErrorTreatsSuccessfulHTTPParseFailureAsIncompatibleResponse(t *testing.T) {
	code, message, statusCode := classifyModelProbeError(&llm.UpstreamError{
		StatusCode: 200,
		Message:    "invalid response: missing output",
	})

	if code != "response_incompatible" {
		t.Fatalf("expected response_incompatible, got %s", code)
	}
	if statusCode != 200 {
		t.Fatalf("expected status 200, got %d", statusCode)
	}
	if message != "upstream response format is incompatible" {
		t.Fatalf("message = %q, want stable public text", message)
	}
}

func TestFailedModelProbeDoesNotExposeUpstreamMessage(t *testing.T) {
	const secret = "provider response contains customer token"
	row := repository.ChannelUpstreamRouteRow{Protocol: "openai_chat_completions", PlatformModelName: "demo"}
	result := (&Service{}).failedModelProbeFromError(row, llm.RouteConfig{APIKey: "secret"}, &llm.UpstreamError{
		StatusCode: 502,
		Message:    secret,
	}, 12)
	if strings.Contains(result.ErrorMessage, secret) {
		t.Fatalf("upstream message leaked into probe result: %q", result.ErrorMessage)
	}
	if result.ErrorMessage != "upstream service is unavailable" {
		t.Fatalf("error message = %q, want stable public text", result.ErrorMessage)
	}
}

func TestFilterModelProbeRowsUsesRouteProtocolWhenTaskIsEmpty(t *testing.T) {
	rows := []repository.ChannelUpstreamRouteRow{
		{ModelKindsJSON: `["image_gen","image_edit"]`, Protocol: "openai_image_generations"},
		{ModelKindsJSON: `["image_gen","image_edit"]`, Protocol: "openai_image_edits"},
		{ModelKindsJSON: `["image_gen","image_edit"]`, Protocol: "openai_responses"},
	}

	got := filterModelProbeRows(rows, "")
	if len(got) != 2 {
		t.Fatalf("expected two image routes, got %d", len(got))
	}
	if got[0].Protocol != "openai_image_generations" || got[1].Protocol != "openai_image_edits" {
		t.Fatalf("unexpected filtered route order: %#v", got)
	}
}

func TestLightweightModelProbeSupportsOpenRouterResponses(t *testing.T) {
	if !isLightweightModelProbeProtocol(llm.AdapterOpenRouterResponses) {
		t.Fatal("expected OpenRouter Responses to support lightweight model probes")
	}
}

func TestSummarizeModelProbeResultsCountsUnsupportedAsFailed(t *testing.T) {
	got := summarizeModelProbeResults([]ModelProbeResult{
		{Success: true, Status: modelProbeStatusSuccess},
		{Status: modelProbeStatusFailed},
		{Status: modelProbeStatusUnsupported},
	})

	if got.TotalCount != 3 || got.SuccessCount != 1 || got.FailedCount != 2 || got.UnsupportedCount != 1 {
		t.Fatalf("unexpected probe summary: %#v", got)
	}
}
