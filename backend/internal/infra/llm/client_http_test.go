package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPClientForRouteReusesClientByConnectTimeout(t *testing.T) {
	client := NewClient()

	defaultClient := client.httpClientForRoute(RouteConfig{})
	explicitDefaultClient := client.httpClientForRoute(RouteConfig{ConnectTimeoutMS: defaultConnectTimeoutMS})
	customClient := client.httpClientForRoute(RouteConfig{ConnectTimeoutMS: 2500})
	customClientAgain := client.httpClientForRoute(RouteConfig{ConnectTimeoutMS: 2500})

	if defaultClient == nil {
		t.Fatal("expected default route client")
	}
	if defaultClient != explicitDefaultClient {
		t.Fatal("expected default and explicit default connect timeout to reuse the same client")
	}
	if customClient == nil {
		t.Fatal("expected custom route client")
	}
	if customClient != customClientAgain {
		t.Fatal("expected identical custom connect timeout to reuse the same client")
	}
	if defaultClient == customClient {
		t.Fatal("expected different connect timeouts to use separate clients")
	}
}

func TestReadUpstreamBodyRejectsOversizedBody(t *testing.T) {
	_, err := readUpstreamBody(io.MultiReader(
		&repeatingReader{remaining: maxUpstreamBodyBytes},
		strings.NewReader("x"),
	))
	if err == nil {
		t.Fatal("expected oversized upstream body to fail")
	}
}

func TestListModelsFallsBackToOpenAICompatibleModels(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Header.Get("Authorization")+"|"+r.Header.Get("x-api-key"))
		if r.URL.Path != "/v1/models" {
			t.Fatalf("expected /v1/models, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{"message": "bearer required"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"id": "claude-3-7-sonnet-20250219", "object": "model", "owned_by": "clewdr"},
			},
		})
	}))
	defer server.Close()

	items, err := NewClient().ListModels(context.Background(), RouteConfig{
		Protocol: AdapterAnthropicMessages,
		BaseURL:  server.URL,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got %v", err)
	}
	if len(items) != 1 || items[0].ID != "claude-3-7-sonnet-20250219" || items[0].OwnedBy != "clewdr" {
		t.Fatalf("unexpected fallback models: %#v", items)
	}
	if len(calls) != 2 {
		t.Fatalf("expected primary and fallback calls, got %d: %#v", len(calls), calls)
	}
	if calls[0] != "|test-key" {
		t.Fatalf("expected primary anthropic auth header, got %q", calls[0])
	}
	if calls[1] != "Bearer test-key|" {
		t.Fatalf("expected fallback bearer auth header, got %q", calls[1])
	}
}

type repeatingReader struct {
	remaining int
}

func (r *repeatingReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	if len(p) > r.remaining {
		p = p[:r.remaining]
	}
	for i := range p {
		p[i] = 'a'
	}
	r.remaining -= len(p)
	return len(p), nil
}

func TestListModelsFallsBackToOpenAICompatibleModelsForGemini(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/v1beta/models" {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{"message": "no gemini models endpoint"},
			})
			return
		}
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected fallback path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("expected fallback bearer auth header, got %q", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"id": "gemini-openai-compatible", "object": "model", "owned_by": "proxy"},
			},
		})
	}))
	defer server.Close()

	items, err := NewClient().ListModels(context.Background(), RouteConfig{
		Protocol: AdapterGoogleGenerateContent,
		BaseURL:  server.URL,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("expected gemini fallback to succeed, got %v", err)
	}
	if len(items) != 1 || items[0].ID != "gemini-openai-compatible" {
		t.Fatalf("unexpected fallback models: %#v", items)
	}
	if len(paths) != 2 || paths[0] != "/v1beta/models" || paths[1] != "/v1/models" {
		t.Fatalf("expected primary gemini list then openai-compatible fallback, got %#v", paths)
	}
}

func TestListModelsDoesNotFallbackForOpenRouterBaseURL(t *testing.T) {
	if shouldFallbackToOpenAICompatibleModels(RouteConfig{
		Protocol: AdapterGoogleGenerateContent,
		BaseURL:  "https://openrouter.ai/api/v1",
	}) {
		t.Fatal("expected openrouter base URL to keep its own models directory")
	}
}

func TestSetOpenRouterAttributionHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", nil)
	setOpenRouterAttributionHeaders(req, RouteConfig{
		BaseURL:            "https://openrouter.ai/api/v1",
		AttributionReferer: "https://app.example.com/",
		AttributionTitle:   "Example App",
	})

	if got := req.Header.Get("HTTP-Referer"); got != "https://app.example.com" {
		t.Fatalf("expected referer header, got %q", got)
	}
	if got := req.Header.Get("X-Title"); got != "Example App" {
		t.Fatalf("expected x-title header, got %q", got)
	}
	if got := req.Header.Get("X-OpenRouter-Title"); got != "Example App" {
		t.Fatalf("expected x-openrouter-title header, got %q", got)
	}
	if got := req.Header.Get("X-OpenRouter-Categories"); got != "general-chat" {
		t.Fatalf("expected x-openrouter-categories header, got %q", got)
	}
}

func TestSetOpenRouterAttributionHeadersSkipsNonOpenRouterBaseURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://api.example.com/v1/chat/completions", nil)
	setOpenRouterAttributionHeaders(req, RouteConfig{
		BaseURL:            "https://api.example.com/v1",
		AttributionReferer: "https://app.example.com",
		AttributionTitle:   "Example App",
	})

	if req.Header.Get("HTTP-Referer") != "" ||
		req.Header.Get("X-Title") != "" ||
		req.Header.Get("X-OpenRouter-Title") != "" ||
		req.Header.Get("X-OpenRouter-Categories") != "" {
		t.Fatalf("expected no openrouter attribution headers, got %#v", req.Header)
	}
}

func TestSetOpenRouterAttributionHeadersRespectsConfiguredHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", nil)
	setOpenRouterAttributionHeaders(req, RouteConfig{
		BaseURL:            "https://openrouter.ai/api/v1",
		HeadersJSON:        `{"HTTP-Referer":"https://custom.example.com","X-Title":"Custom App"}`,
		AttributionReferer: "https://app.example.com",
		AttributionTitle:   "Example App",
	})
	setAdditionalHeaders(req, `{"HTTP-Referer":"https://custom.example.com","X-Title":"Custom App"}`)

	if got := req.Header.Get("HTTP-Referer"); got != "https://custom.example.com" {
		t.Fatalf("expected configured referer header, got %q", got)
	}
	if got := req.Header.Get("X-Title"); got != "Custom App" {
		t.Fatalf("expected configured x-title header, got %q", got)
	}
	if got := req.Header.Get("X-OpenRouter-Title"); got != "" {
		t.Fatalf("expected no default x-openrouter-title when title is configured, got %q", got)
	}
	if got := req.Header.Get("X-OpenRouter-Categories"); got != "general-chat" {
		t.Fatalf("expected default x-openrouter-categories header, got %q", got)
	}
}

func TestSetAdditionalHeadersExpandsConversationIdentityTemplates(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/responses", nil)
	setAdditionalHeadersForInput(req, `{
		"session-id":"${DEEIX_SESSION_ID}",
		"thread-id":"${DEEIX_THREAD_ID}",
		"X-Conversation-Id":"${DEEIX_CONVERSATION_ID}",
		"x-client-request-id":"${DEEIX_REQUEST_ID}",
		"X-Static":"fixed"
	}`, GenerateInput{
		RequestID: "request-1",
		SessionID: "session-1",
		ThreadID:  "thread-1",
	})

	if req.Header.Get("session-id") != "session-1" || req.Header.Get("thread-id") != "thread-1" {
		t.Fatalf("expected Codex-style session headers, got %#v", req.Header)
	}
	if req.Header.Get("X-Conversation-Id") != "session-1" {
		t.Fatalf("expected conversation header to use the opaque session id, got %q", req.Header.Get("X-Conversation-Id"))
	}
	if req.Header.Get("x-client-request-id") != "request-1" || req.Header.Get("X-Static") != "fixed" {
		t.Fatalf("expected request and static headers to be preserved, got %#v", req.Header)
	}
}

func TestOpenAIResponsesRetriesWithoutPromptCacheFieldsWhenRelayRejectsThem(t *testing.T) {
	requests := make([]map[string]interface{}, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requests = append(requests, payload)
		if len(requests) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{"message": "unknown field prompt_cache_options"},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "resp_1",
			"output": []map[string]interface{}{
				{"type": "message", "content": []map[string]interface{}{{"type": "output_text", "text": "ok"}}},
			},
			"usage": map[string]interface{}{"input_tokens": 3, "output_tokens": 2},
		})
	}))
	defer server.Close()

	output, err := NewClient().Generate(context.Background(), RouteConfig{
		Protocol:      AdapterOpenAIResponses,
		BaseURL:       server.URL,
		UpstreamModel: "gpt-5.6-relay",
	}, GenerateInput{
		PromptCacheKey: "session-retry",
		Messages: []Message{
			{Role: "system", Content: "stable", CacheControl: &CacheControl{Type: "ephemeral"}},
			{Role: "user", Content: "hello"},
		},
		Options: map[string]interface{}{
			"prompt_cache_options": map[string]interface{}{"mode": "explicit"},
		},
	})
	if err != nil {
		t.Fatalf("generate with compatibility retry: %v", err)
	}
	if output.Text != "ok" || len(requests) != 2 {
		t.Fatalf("expected successful one-shot compatibility retry, output=%#v requests=%d", output, len(requests))
	}
	if _, ok := requests[0]["prompt_cache_key"]; !ok {
		t.Fatalf("expected first request to include prompt cache fields, got %#v", requests[0])
	}
	if _, ok := requests[1]["prompt_cache_key"]; ok {
		t.Fatalf("expected retry to remove prompt_cache_key, got %#v", requests[1])
	}
	if _, ok := requests[1]["prompt_cache_options"]; ok {
		t.Fatalf("expected retry to remove prompt_cache_options, got %#v", requests[1])
	}
	for _, item := range requests[1]["input"].([]interface{}) {
		content := asMap(item)["content"].([]interface{})
		for _, block := range content {
			if _, ok := asMap(block)["prompt_cache_breakpoint"]; ok {
				t.Fatalf("expected retry to remove prompt cache breakpoints, got %#v", requests[1])
			}
		}
	}
}

func TestOpenAIChatCompletionsStreamRetriesWhenAutoUsageOptionIsRejected(t *testing.T) {
	var includeUsageValues []interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("expected chat completions path, got %s", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		streamOptions := asMap(payload["stream_options"])
		includeUsageValues = append(includeUsageValues, streamOptions["include_usage"])
		if len(includeUsageValues) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{"message": "unknown field stream_options.include_usage"},
			})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl_1\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl_1\",\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	output, err := NewClient().GenerateStream(context.Background(), RouteConfig{
		Protocol:      AdapterOpenAIChatCompletions,
		BaseURL:       server.URL,
		UpstreamModel: "gpt-compatible",
	}, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
	}, nil)
	if err != nil {
		t.Fatalf("generate stream: %v", err)
	}
	if output.Text != "ok" || output.Usage.InputTokens != 3 || output.Usage.OutputTokens != 2 {
		t.Fatalf("expected retried stream output and usage, got output=%#v usage=%#v", output.Text, output.Usage)
	}
	if len(includeUsageValues) != 2 || includeUsageValues[0] != true || includeUsageValues[1] != false {
		t.Fatalf("expected retry to disable only auto stream usage, got %#v", includeUsageValues)
	}
}
