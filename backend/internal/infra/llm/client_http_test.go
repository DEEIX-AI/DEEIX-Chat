package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"testing"

	"github.com/google/uuid"
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
	req := httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/chat/completions", nil)
	setAdditionalHeadersForInput(req, `{
		"X-Conversation-Id":"${DEEIX_CONVERSATION_ID}",
		"X-Session-Id":"session:${DEEIX_SESSION_ID}",
		"X-Static":"fixed"
	}`, &GenerateInput{
		ConversationPublicID:   "conversation-1",
		ConversationSessionKey: "session-1",
	})

	if got := req.Header.Get("X-Conversation-Id"); got != "conversation-1" {
		t.Fatalf("expected public conversation ID header, got %q", got)
	}
	if got := req.Header.Get("X-Session-Id"); got != "session:session-1" {
		t.Fatalf("expected session header template expansion, got %q", got)
	}
	if got := req.Header.Get("X-Static"); got != "fixed" {
		t.Fatalf("expected static header to remain unchanged, got %q", got)
	}
}

func TestSetAdditionalHeadersExpandsRequestIdentityTemplates(t *testing.T) {
	headersJSON := `{
		"X-Request-Id":"${DEEIX_REQUEST_ID}",
		"X-Client-Request-Id":"${DEEIX_UPSTREAM_REQUEST_ID}"
	}`
	input := &GenerateInput{
		RequestID:            "request-1",
		ConversationPublicID: "conversation-1",
	}

	firstRequest := httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/chat/completions", nil)
	setAdditionalHeadersForInput(firstRequest, headersJSON, input)
	if got := firstRequest.Header.Get("X-Request-Id"); got != "request-1" {
		t.Fatalf("expected DEEIX request ID header, got %q", got)
	}
	firstUpstreamRequestID := firstRequest.Header.Get("X-Client-Request-Id")
	if _, err := uuid.Parse(firstUpstreamRequestID); err != nil {
		t.Fatalf("expected valid upstream request UUID, got %q: %v", firstUpstreamRequestID, err)
	}

	secondRequest := httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/chat/completions", nil)
	setAdditionalHeadersForInput(secondRequest, headersJSON, input)
	secondUpstreamRequestID := secondRequest.Header.Get("X-Client-Request-Id")
	if firstUpstreamRequestID == secondUpstreamRequestID {
		t.Fatalf("expected a unique ID for each upstream request, got %q", secondUpstreamRequestID)
	}
}

func TestSetAdditionalHeadersOmitsDynamicTemplatesForAuxiliaryTasks(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "https://relay.example.com/v1/chat/completions", nil)
	setAdditionalHeadersForInput(req, `{
		"X-Request-Id":"${DEEIX_REQUEST_ID}",
		"X-Client-Request-Id":"${DEEIX_UPSTREAM_REQUEST_ID}",
		"X-Static":"fixed"
	}`, &GenerateInput{RequestID: "request-1"})

	if got := req.Header.Get("X-Request-Id"); got != "" {
		t.Fatalf("expected request ID header to be omitted outside conversation generation, got %q", got)
	}
	if got := req.Header.Get("X-Client-Request-Id"); got != "" {
		t.Fatalf("expected upstream request ID header to be omitted outside conversation generation, got %q", got)
	}
	if got := req.Header.Get("X-Static"); got != "fixed" {
		t.Fatalf("expected static header to remain available, got %q", got)
	}
}

func TestSetAdditionalHeadersOmitsDynamicTemplatesWithoutContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://relay.example.com/v1/models", nil)
	setAdditionalHeaders(req, `{
		"X-Conversation-Id":"${DEEIX_CONVERSATION_ID}",
		"X-Prefixed-Conversation-Id":"conversation:${DEEIX_CONVERSATION_ID}",
		"X-Request-Id":"${DEEIX_REQUEST_ID}",
		"X-Client-Request-Id":"${DEEIX_UPSTREAM_REQUEST_ID}",
		"X-Static":"fixed"
	}`)

	if got := req.Header.Get("X-Conversation-Id"); got != "" {
		t.Fatalf("expected dynamic header to be omitted without conversation context, got %q", got)
	}
	if got := req.Header.Get("X-Prefixed-Conversation-Id"); got != "" {
		t.Fatalf("expected prefixed dynamic header to be omitted without conversation context, got %q", got)
	}
	if got := req.Header.Get("X-Request-Id"); got != "" {
		t.Fatalf("expected request ID header to be omitted without generation context, got %q", got)
	}
	if got := req.Header.Get("X-Client-Request-Id"); got != "" {
		t.Fatalf("expected upstream request ID header to be omitted without generation context, got %q", got)
	}
	if got := req.Header.Get("X-Static"); got != "fixed" {
		t.Fatalf("expected static header to remain available, got %q", got)
	}
}

func TestProviderRequestBuildersExpandConversationIdentityHeaders(t *testing.T) {
	client := NewClient()
	input := &GenerateInput{
		ConversationPublicID:   "conversation-1",
		ConversationSessionKey: "session-1",
	}
	route := RouteConfig{HeadersJSON: `{
		"X-Conversation-Id":"${DEEIX_CONVERSATION_ID}",
		"X-Session-Id":"${DEEIX_SESSION_ID}"
	}`}

	requests := make([]*http.Request, 0, 2)
	anthropicRequest, err := client.newAnthropicRequest(t.Context(), http.MethodPost, "https://api.anthropic.com/v1/messages", nil, route, input)
	if err != nil {
		t.Fatalf("new anthropic request: %v", err)
	}
	requests = append(requests, anthropicRequest)

	geminiRequest, err := client.newGeminiRequest(t.Context(), http.MethodPost, "https://generativelanguage.googleapis.com/v1beta/models/test:generateContent", nil, route, input)
	if err != nil {
		t.Fatalf("new gemini request: %v", err)
	}
	requests = append(requests, geminiRequest)

	for _, req := range requests {
		if got := req.Header.Get("X-Conversation-Id"); got != "conversation-1" {
			t.Fatalf("expected %s request to include conversation ID, got %q", req.URL.Host, got)
		}
		if got := req.Header.Get("X-Session-Id"); got != "session-1" {
			t.Fatalf("expected %s request to include session ID, got %q", req.URL.Host, got)
		}
	}
}

func TestOpenAIGenerationExpandsConversationIdentityHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Conversation-Id"); got != "conversation-1" {
			t.Fatalf("expected conversation identity header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"ok"}}],
			"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
		}`))
	}))
	defer server.Close()

	output, err := NewClient().Generate(t.Context(), RouteConfig{
		Protocol:      AdapterOpenAIChatCompletions,
		BaseURL:       server.URL,
		UpstreamModel: "test-model",
		HeadersJSON:   `{"X-Conversation-Id":"${DEEIX_CONVERSATION_ID}"}`,
	}, GenerateInput{
		ConversationPublicID: "conversation-1",
		Messages:             []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if output.Text != "ok" {
		t.Fatalf("expected generated text, got %q", output.Text)
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

func TestGenerateMarksSuccessfulHTTPParseFailureAsAccepted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"invalid"`))
	}))
	defer server.Close()

	_, err := NewClient().Generate(t.Context(), RouteConfig{
		Protocol:      AdapterOpenAIChatCompletions,
		BaseURL:       server.URL,
		UpstreamModel: "test-model",
	}, GenerateInput{Messages: []Message{{Role: "user", Content: "hello"}}})
	if err == nil || !RequestWasAccepted(err) {
		t.Fatalf("expected accepted parse error, got %v", err)
	}
}

func TestGenerateMarksConnectionDropAfterRequestWriteAsAccepted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
			return
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()

	_, err := NewClient().Generate(t.Context(), RouteConfig{
		Protocol:      AdapterOpenAIChatCompletions,
		BaseURL:       server.URL,
		UpstreamModel: "test-model",
	}, GenerateInput{Messages: []Message{{Role: "user", Content: "hello"}}})
	if err == nil || !RequestWasAccepted(err) {
		t.Fatalf("expected post-write connection drop to be treated as accepted, got %v", err)
	}
}

func TestGenerateStreamMarksSuccessfulHTTPStreamFailureAsAccepted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {invalid}\n\n"))
	}))
	defer server.Close()

	_, err := NewClient().GenerateStream(t.Context(), RouteConfig{
		Protocol:      AdapterOpenAIChatCompletions,
		BaseURL:       server.URL,
		UpstreamModel: "test-model",
	}, GenerateInput{Messages: []Message{{Role: "user", Content: "hello"}}}, nil)
	if err == nil || !RequestWasAccepted(err) {
		t.Fatalf("expected accepted stream error, got %v", err)
	}
}

func TestGenerateStreamPreservesBackgroundResponseIDBeforeAcceptedFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_background\"}}\n\ndata: {invalid}\n\n"))
	}))
	defer server.Close()

	responseID := ""
	_, err := NewClient().GenerateStream(t.Context(), RouteConfig{
		Protocol:      AdapterOpenAIResponses,
		BaseURL:       server.URL,
		UpstreamModel: "test-model",
	}, GenerateInput{
		Messages:            []Message{{Role: "user", Content: "hello"}},
		ResponsesBackground: true,
	}, func(event GenerateStreamEvent) error {
		if event.ResponseID != "" {
			responseID = event.ResponseID
		}
		return nil
	})
	if err == nil || !RequestWasAccepted(err) {
		t.Fatalf("expected accepted background stream error, got %v", err)
	}
	if responseID != "resp_background" {
		t.Fatalf("response ID = %q, want resp_background", responseID)
	}
}

func TestDoGenerationRequestMarksPostWriteFailureAsAccepted(t *testing.T) {
	errAfterWrite := errors.New("connection closed before response headers")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		trace := httptrace.ContextClientTrace(req.Context())
		if trace == nil || trace.WroteRequest == nil {
			t.Fatal("expected request write trace")
		}
		trace.WroteRequest(httptrace.WroteRequestInfo{})
		return nil, errAfterWrite
	})}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.com/v1/chat/completions", strings.NewReader(`{"model":"test"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	_, err = doGenerationRequest(client, req)
	if !errors.Is(err, errAfterWrite) || !RequestWasAccepted(err) {
		t.Fatalf("expected ambiguous post-write failure, got %v", err)
	}
}

func TestDoGenerationRequestKeepsPreWriteFailureRetryable(t *testing.T) {
	errBeforeWrite := errors.New("dial failed")
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errBeforeWrite
	})}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.com/v1/chat/completions", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	_, err = doGenerationRequest(client, req)
	if !errors.Is(err, errBeforeWrite) || RequestWasAccepted(err) {
		t.Fatalf("expected retryable pre-write failure, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
