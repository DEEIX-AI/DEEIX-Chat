package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIChatCompletionsStreamDoesNotRetryAfterEventEmission(t *testing.T) {
	testChatCompletionsStreamDoesNotRetryAfterEventEmission(t, AdapterOpenAIChatCompletions)
}

func TestOpenRouterChatCompletionsStreamDoesNotRetryAfterEventEmission(t *testing.T) {
	testChatCompletionsStreamDoesNotRetryAfterEventEmission(t, AdapterOpenRouterChat)
}

func testChatCompletionsStreamDoesNotRetryAfterEventEmission(t *testing.T, protocol string) {
	t.Helper()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl_1\",\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"error\":{\"message\":\"unknown field stream_options.include_usage\",\"code\":400}}\n\n"))
	}))
	defer server.Close()

	var deltas strings.Builder
	_, err := newTestClient().GenerateStream(context.Background(), RouteConfig{
		Protocol:      protocol,
		BaseURL:       server.URL,
		UpstreamModel: "gpt-compatible",
	}, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
	}, func(event GenerateStreamEvent) error {
		deltas.WriteString(event.Delta)
		return nil
	})
	if err == nil {
		t.Fatal("expected late stream error")
	}
	if requestCount != 1 {
		t.Fatalf("expected no retry after an observable stream event, got %d requests", requestCount)
	}
	if deltas.String() != "partial" {
		t.Fatalf("expected exactly one visible delta sequence, got %q", deltas.String())
	}
}

func TestOpenAIResponsesStreamDoesNotRetryPromptCacheErrors(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["prompt_cache_key"] != "session-1" {
			t.Fatalf("expected configured prompt cache key, got %#v", payload["prompt_cache_key"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n"))
		_, _ = w.Write([]byte("event: response.error\ndata: {\"type\":\"response.error\",\"error\":{\"message\":\"unknown field prompt_cache_options\",\"code\":400}}\n\n"))
	}))
	defer server.Close()

	var deltas strings.Builder
	_, err := newTestClient().GenerateStream(context.Background(), RouteConfig{
		Protocol:      AdapterOpenAIResponses,
		BaseURL:       server.URL,
		UpstreamModel: "gpt-5.6-relay",
	}, GenerateInput{
		PromptCacheKey: "session-1",
		Messages:       []Message{{Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"prompt_cache_options": map[string]interface{}{"mode": "explicit"},
		},
	}, func(event GenerateStreamEvent) error {
		deltas.WriteString(event.Delta)
		return nil
	})
	if err == nil {
		t.Fatal("expected prompt cache stream error")
	}
	if requestCount != 1 {
		t.Fatalf("expected prompt cache capability failure to stay visible, got %d requests", requestCount)
	}
	if deltas.String() != "partial" {
		t.Fatalf("expected exactly one visible delta sequence, got %q", deltas.String())
	}
}
