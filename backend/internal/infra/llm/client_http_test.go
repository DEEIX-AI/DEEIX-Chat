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

func TestListModelsFallbackOnlyAppliesToAnthropic(t *testing.T) {
	called := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"message": "no gemini key"},
		})
	}))
	defer server.Close()

	_, err := NewClient().ListModels(context.Background(), RouteConfig{
		Protocol: AdapterGoogleGenerateContent,
		BaseURL:  server.URL,
		APIKey:   "test-key",
	})
	if err == nil {
		t.Fatal("expected gemini list models to fail")
	}
	if called != 1 {
		t.Fatalf("expected no openai-compatible fallback for gemini, got %d calls", called)
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
