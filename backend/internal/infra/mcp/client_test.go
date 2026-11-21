package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

type recordedRequest struct {
	Method  string
	Params  map[string]any
	Headers http.Header
}

type rpcRequest struct {
	ID     any            `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

// fakeServer 记录请求并按 legacy 或 modern 规范作答；handle 返回 tools/* 的 result。
type fakeServer struct {
	t        *testing.T
	legacy   bool
	mu       sync.Mutex
	requests []recordedRequest
	handle   func(req rpcRequest, headers http.Header) (any, int, any)
}

func (f *fakeServer) methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	methods := make([]string, 0, len(f.requests))
	for _, item := range f.requests {
		methods = append(methods, item.Method)
	}
	return methods
}

func (f *fakeServer) request(method string) recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	for index := len(f.requests) - 1; index >= 0; index-- {
		if f.requests[index].Method == method {
			return f.requests[index]
		}
	}
	return recordedRequest{}
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{Method: req.Method, Params: req.Params, Headers: r.Header.Clone()})
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	writeError := func(status int, code int, message string, data any) {
		w.WriteHeader(status)
		payload := map[string]any{"code": code, "message": message}
		if data != nil {
			payload["data"] = data
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": payload})
	}
	if f.legacy {
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "session_1")
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{},
			}})
			return
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if r.Header.Get("Mcp-Session-Id") != "session_1" {
			writeError(http.StatusBadRequest, -32000, "Bad Request: Server not initialized", nil)
			return
		}
	}
	result, status, rpcErr := f.handle(req, r.Header)
	if rpcErr != nil {
		errMap := rpcErr.(map[string]any)
		writeError(status, errMap["code"].(int), errMap["message"].(string), errMap["data"])
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}

func newFakeServer(t *testing.T, legacy bool, handle func(req rpcRequest, headers http.Header) (any, int, any)) (*fakeServer, *httptest.Server) {
	fake := &fakeServer{t: t, legacy: legacy, handle: handle}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	return fake, server
}

func textResult(text string) map[string]any {
	return map[string]any{"content": []map[string]string{{"type": "text", "text": text}}}
}

func TestClientFallsBackToLegacyHandshakeAndCachesEra(t *testing.T) {
	fake, server := newFakeServer(t, true, func(req rpcRequest, headers http.Header) (any, int, any) {
		if req.Params["name"] != "memory.list" {
			t.Errorf("expected tool name, got %#v", req.Params["name"])
		}
		return textResult("ok"), 0, nil
	})
	client := NewClient(security.NewStrictOutboundPolicy(true))
	cfg := CallConfig{BaseURL: server.URL + "/mcp"}
	input := CallInput{ToolName: "memory.list", ArgumentsJSON: `{"scope":"user"}`}

	output, err := client.CallTool(context.Background(), cfg, input)
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if output != `{"content":[{"text":"ok","type":"text"}]}` {
		t.Fatalf("unexpected output: %s", output)
	}
	want := []string{"tools/call", "initialize", "notifications/initialized", "tools/call"}
	if got := fake.methods(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("methods = %v, want %v", got, want)
	}
	initialize := fake.request("initialize")
	if initialize.Params["protocolVersion"] != legacyProtocolVersion || initialize.Headers.Get("MCP-Protocol-Version") != "" {
		t.Fatalf("unexpected initialize request: %#v", initialize)
	}
	legacyCall := fake.request("tools/call")
	if legacyCall.Headers.Get("Mcp-Session-Id") != "session_1" || legacyCall.Headers.Get("MCP-Protocol-Version") != "2025-06-18" {
		t.Fatalf("legacy call must carry session and negotiated version, got %v", legacyCall.Headers)
	}

	if _, err = client.CallTool(context.Background(), cfg, input); err != nil {
		t.Fatalf("second call: %v", err)
	}
	want = append(want, "initialize", "notifications/initialized", "tools/call")
	if got := fake.methods(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("cached legacy era must skip the modern probe: methods = %v", got)
	}
}

func TestClientSendsNegotiatedProtocolVersionAfterInitialize(t *testing.T) {
	fake, server := newFakeServer(t, true, func(req rpcRequest, headers http.Header) (any, int, any) {
		return map[string]any{"tools": []any{}}, 0, nil
	})
	client := NewClient(security.NewStrictOutboundPolicy(true))
	if _, err := client.ListTools(context.Background(), CallConfig{BaseURL: server.URL}); err != nil {
		t.Fatalf("list tools: %v", err)
	}
	for _, method := range []string{"notifications/initialized", "tools/list"} {
		if got := fake.request(method).Headers.Get("MCP-Protocol-Version"); got != "2025-06-18" {
			t.Fatalf("%s MCP-Protocol-Version = %q, want negotiated 2025-06-18", method, got)
		}
	}
}

func TestClientModernRequestCarriesMetadataAndHeaders(t *testing.T) {
	fake, server := newFakeServer(t, false, func(req rpcRequest, headers http.Header) (any, int, any) {
		return map[string]any{"resultType": "complete", "content": []map[string]string{{"type": "text", "text": "done"}}}, 0, nil
	})
	client := NewClient(security.NewStrictOutboundPolicy(true))
	schema := json.RawMessage(`{"type":"object","properties":{
		"region":{"type":"string","x-mcp-header":"Region"},
		"greeting":{"type":"string","x-mcp-header":"Greeting"},
		"options":{"type":"object","properties":{"dryRun":{"type":"boolean","x-mcp-header":"Dry-Run"}}},
		"limit":{"type":"integer","x-mcp-header":"Limit"},
		"query":{"type":"string"}
	}}`)
	cfg := CallConfig{BaseURL: server.URL, Headers: map[string]string{"MCP-Protocol-Version": "1999-01-01", "X-Custom": "kept"}, InputSchema: schema}
	_, err := client.CallTool(context.Background(), cfg, CallInput{
		ToolName:      "execute_sql",
		ArgumentsJSON: `{"region":"us-west1","greeting":"Hello, 世界","options":{"dryRun":true},"limit":null,"query":"SELECT 1"}`,
		UserID:        7,
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if got := fake.methods(); len(got) != 1 || got[0] != "tools/call" {
		t.Fatalf("modern server must not see a handshake, methods = %v", got)
	}
	request := fake.request("tools/call")
	headers := request.Headers
	checks := map[string]string{
		"MCP-Protocol-Version": modernProtocolVersion,
		"Mcp-Method":           "tools/call",
		"Mcp-Name":             "execute_sql",
		"Mcp-Param-Region":     "us-west1",
		"Mcp-Param-Greeting":   "=?base64?SGVsbG8sIOS4lueVjA==?=",
		"Mcp-Param-Dry-Run":    "true",
		"X-Custom":             "kept",
	}
	for name, want := range checks {
		if got := headers.Get(name); got != want {
			t.Errorf("header %s = %q, want %q", name, got, want)
		}
	}
	if headers.Get("Mcp-Param-Limit") != "" || headers.Get("Mcp-Session-Id") != "" {
		t.Errorf("null parameters and sessions must not produce headers: %v", headers)
	}
	meta, _ := request.Params["_meta"].(map[string]any)
	if meta[metaProtocolVersion] != modernProtocolVersion || meta[metaClientInfo] == nil || meta[metaClientCapabilities] == nil || meta["user_id"] != float64(7) {
		t.Fatalf("unexpected _meta: %#v", meta)
	}
}

func TestClientModernProtocolErrorsDoNotFallBack(t *testing.T) {
	cases := map[string]struct {
		status int
		err    map[string]any
	}{
		"unsupported version": {http.StatusBadRequest, map[string]any{"code": errorCodeUnsupportedProtocolVersion, "message": "Unsupported protocol version", "data": map[string]any{"supported": []string{"2027-01-01"}}}},
		"method not found":    {http.StatusNotFound, map[string]any{"code": errorCodeMethodNotFound, "message": "Method not found"}},
		"header mismatch":     {http.StatusBadRequest, map[string]any{"code": errorCodeHeaderMismatch, "message": "Header mismatch"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fake, server := newFakeServer(t, false, func(req rpcRequest, headers http.Header) (any, int, any) {
				return nil, tc.status, tc.err
			})
			client := NewClient(security.NewStrictOutboundPolicy(true))
			if _, err := client.ListTools(context.Background(), CallConfig{BaseURL: server.URL}); err == nil {
				t.Fatal("expected modern protocol error")
			}
			if got := fake.methods(); len(got) != 1 || got[0] != "tools/list" {
				t.Fatalf("modern errors must not trigger the legacy handshake, methods = %v", got)
			}
		})
	}
}

func TestClientFallsBackWhenServerOnlySupportsLegacyVersions(t *testing.T) {
	fake, server := newFakeServer(t, false, func(req rpcRequest, headers http.Header) (any, int, any) {
		if headers.Get("Mcp-Session-Id") == "" && req.Method != "initialize" && req.Method != "notifications/initialized" {
			return nil, http.StatusBadRequest, map[string]any{"code": errorCodeUnsupportedProtocolVersion, "message": "Unsupported protocol version", "data": map[string]any{"supported": []string{"2025-11-25"}}}
		}
		return map[string]any{"tools": []any{}}, 0, nil
	})
	fake.legacy = true
	client := NewClient(security.NewStrictOutboundPolicy(true))
	if _, err := client.ListTools(context.Background(), CallConfig{BaseURL: server.URL}); err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if got := fake.methods(); len(got) != 4 || got[1] != "initialize" {
		t.Fatalf("expected handshake fallback, methods = %v", got)
	}
}

func TestClientListToolsFollowsCursorAndDropsInvalidHeaderTools(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		fake, server := newFakeServer(t, legacy, func(req rpcRequest, headers http.Header) (any, int, any) {
			if req.Params["cursor"] == "page-2" {
				return map[string]any{"tools": []map[string]any{{"name": "second"}}}, 0, nil
			}
			return map[string]any{
				"nextCursor": "page-2",
				"tools": []map[string]any{
					{"name": "first"},
					{"name": "broken", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{
						"ratio": map[string]any{"type": "number", "x-mcp-header": "Ratio"},
					}}},
				},
			}, 0, nil
		})
		client := NewClient(security.NewStrictOutboundPolicy(true))
		tools, err := client.ListTools(context.Background(), CallConfig{BaseURL: server.URL})
		if err != nil {
			t.Fatalf("legacy=%v list tools: %v", legacy, err)
		}
		if len(tools) != 2 || tools[0].Name != "first" || tools[1].Name != "second" {
			t.Fatalf("legacy=%v unexpected tools: %#v", legacy, tools)
		}
		if legacy && strings.Count(strings.Join(fake.methods(), ","), "initialize,") != 1 {
			t.Fatalf("legacy pagination must reuse the session, methods = %v", fake.methods())
		}
	}
}

func TestClientRejectsInputRequiredResult(t *testing.T) {
	_, server := newFakeServer(t, false, func(req rpcRequest, headers http.Header) (any, int, any) {
		return map[string]any{"resultType": "input_required", "inputRequests": map[string]any{}}, 0, nil
	})
	client := NewClient(security.NewStrictOutboundPolicy(true))
	_, err := client.CallTool(context.Background(), CallConfig{BaseURL: server.URL}, CallInput{ToolName: "ask"})
	if err == nil || !strings.Contains(err.Error(), "input_required") {
		t.Fatalf("expected input_required error, got %v", err)
	}
}

func TestClientCallToolRejectsInvalidArgumentsJSON(t *testing.T) {
	client := NewClient(security.NewStrictOutboundPolicy(true))
	_, err := client.CallTool(context.Background(), CallConfig{BaseURL: "https://example.com/mcp"}, CallInput{
		ToolName:      "memory.list",
		ArgumentsJSON: `{"scope":`,
	})
	if err == nil || err.Error() != "mcp tool arguments must be a valid JSON object" {
		t.Fatalf("expected invalid arguments error, got %v", err)
	}
}

func TestClientCallToolTreatsMCPResultErrorAsExecutionError(t *testing.T) {
	_, server := newFakeServer(t, true, func(req rpcRequest, headers http.Header) (any, int, any) {
		return map[string]any{"isError": true, "content": []map[string]string{{"type": "text", "text": "search backend unavailable"}}}, 0, nil
	})
	client := NewClient(security.OutboundPolicy{})
	_, err := client.CallTool(context.Background(), CallConfig{BaseURL: server.URL}, CallInput{ToolName: "search", ArgumentsJSON: `{}`})
	if err == nil || err.Error() != "mcp tool error: search backend unavailable" {
		t.Fatalf("expected MCP tool error, got %v", err)
	}
}

func TestClientCallToolTreatsWrappedMCPProtocolErrorAsExecutionError(t *testing.T) {
	_, server := newFakeServer(t, false, func(req rpcRequest, headers http.Header) (any, int, any) {
		return textResult("MCP error -32602: Input validation error: Invalid arguments for tool bing_search"), 0, nil
	})
	client := NewClient(security.OutboundPolicy{})
	_, err := client.CallTool(context.Background(), CallConfig{BaseURL: server.URL}, CallInput{ToolName: "bing_search", ArgumentsJSON: `{}`})
	if err == nil || err.Error() != "mcp tool error: MCP error -32602: Input validation error: Invalid arguments for tool bing_search" {
		t.Fatalf("expected wrapped MCP protocol error, got %v", err)
	}
}

func TestClientListToolsParsesSSEJSONRPC(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{\"progress\":1}}\n\n" +
			"event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":3,\"result\":{\"tools\":[{\"name\":\"memory.list\",\"description\":\"List memories\"}]}}\n\n"))
	}))
	defer server.Close()

	client := NewClient(security.OutboundPolicy{})
	tools, err := client.ListTools(context.Background(), CallConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "memory.list" || tools[0].Description != "List memories" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}

func TestClientListToolsDoesNotExposeHTTPResponseBody(t *testing.T) {
	const secret = "provider-internal-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(secret))
	}))
	defer server.Close()

	client := NewClient(security.OutboundPolicy{})
	_, err := client.ListTools(context.Background(), CallConfig{BaseURL: server.URL})
	if err == nil {
		t.Fatal("expected list tools to fail")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("provider response body leaked into error: %v", err)
	}
	if !strings.Contains(err.Error(), "status=502") {
		t.Fatalf("error = %v, want status-only diagnostic", err)
	}
}

func TestParseRPCResponseParsesLargeSingleLineSSEData(t *testing.T) {
	toolDescription := strings.Repeat("x", 70*1024)
	payload := `event: message
data: {"jsonrpc":"2.0","id":3,"result":{"tools":[{"name":"memory.list","description":"` + toolDescription + `"}]}}

`

	result, err := parseRPCResponse("text/event-stream", []byte(payload))
	if err != nil {
		t.Fatalf("parse rpc response: %v", err)
	}

	var parsed struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(parsed.Tools) != 1 || parsed.Tools[0].Name != "memory.list" {
		t.Fatalf("unexpected tools: %#v", parsed.Tools)
	}
	if parsed.Tools[0].Description != toolDescription {
		t.Fatalf("unexpected tool description length: got %d want %d", len(parsed.Tools[0].Description), len(toolDescription))
	}
}

func TestParseHeaderParamsValidation(t *testing.T) {
	invalid := map[string]string{
		"number type":       `{"type":"object","properties":{"ratio":{"type":"number","x-mcp-header":"Ratio"}}}`,
		"inside items":      `{"type":"object","properties":{"list":{"type":"array","items":{"type":"string","x-mcp-header":"Item"}}}}`,
		"inside oneOf":      `{"type":"object","oneOf":[{"properties":{"a":{"type":"string","x-mcp-header":"A"}}}]}`,
		"on root":           `{"type":"object","x-mcp-header":"Root"}`,
		"duplicate name":    `{"type":"object","properties":{"a":{"type":"string","x-mcp-header":"Name"},"b":{"type":"string","x-mcp-header":"name"}}}`,
		"invalid token":     `{"type":"object","properties":{"a":{"type":"string","x-mcp-header":"Bad Name"}}}`,
		"empty header name": `{"type":"object","properties":{"a":{"type":"string","x-mcp-header":""}}}`,
	}
	for name, schema := range invalid {
		if _, err := parseHeaderParams(json.RawMessage(schema)); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
	params, err := parseHeaderParams(json.RawMessage(`{"type":"object","properties":{"outer":{"type":"object","properties":{"id":{"type":"integer","x-mcp-header":"Id"}}}}}`))
	if err != nil || len(params) != 1 || strings.Join(params[0].Path, ".") != "outer.id" {
		t.Fatalf("expected nested header param, got %#v %v", params, err)
	}
	headers := buildParamHeaders(params, map[string]any{"outer": map[string]any{"id": int64(42)}})
	if headers["Id"] != "42" {
		t.Fatalf("expected integer header value, got %#v", headers)
	}
}

func TestEncodeHeaderValue(t *testing.T) {
	cases := map[string]string{
		"us-west1":           "us-west1",
		"Hello, 世界":          "=?base64?SGVsbG8sIOS4lueVjA==?=",
		" padded ":           "=?base64?IHBhZGRlZCA=?=",
		"line1\nline2":       "=?base64?bGluZTEKbGluZTI=?=",
		"=?base64?literal?=": "=?base64?PT9iYXNlNjQ/bGl0ZXJhbD89?=",
	}
	for input, want := range cases {
		if got := encodeHeaderValue(input); got != want {
			t.Errorf("encodeHeaderValue(%q) = %q, want %q", input, got, want)
		}
	}
}
