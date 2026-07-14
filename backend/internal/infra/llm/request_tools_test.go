package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func mustBuildRequestBody(t *testing.T, protocol string, model string, endpoint string, input GenerateInput, stream bool) map[string]interface{} {
	t.Helper()
	payload, err := buildOpenAIRequestBody(protocol, model, endpoint, input, stream)
	if err != nil {
		t.Fatalf("build request body: %v", err)
	}
	return payload
}

func mustBuildAnthropicRequestBody(t *testing.T, model string, input GenerateInput, stream bool) map[string]interface{} {
	t.Helper()
	payload, err := buildAnthropicRequestBody(model, input, stream)
	if err != nil {
		t.Fatalf("build anthropic request body: %v", err)
	}
	return payload
}

func mustBuildGeminiRequestBody(t *testing.T, input GenerateInput) map[string]interface{} {
	t.Helper()
	payload, err := buildGeminiRequestBody(input)
	if err != nil {
		t.Fatalf("build gemini request body: %v", err)
	}
	return payload
}

func TestBuildChatCompletionsToolMessages(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIChatCompletions, "gpt-5", EndpointChatCompletions, GenerateInput{
		Messages: []Message{
			{
				Role:             "assistant",
				ReasoningContent: "need memory",
				ToolCalls:        []ToolCall{{ToolCallID: "call_1", ToolType: "function", ToolName: "memory.list", ArgumentsJSON: `{"scope":"user"}`}},
			},
			{Role: "tool", ToolResults: []ToolResult{{ToolCallID: "call_1", ToolName: "memory.list", OutputJSON: `{"items":[]}`, Status: "success"}}},
		},
	}, false)

	messages := payload["messages"].([]map[string]interface{})
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %#v", messages)
	}
	if messages[0]["role"] != "assistant" {
		t.Fatalf("expected assistant tool call message, got %#v", messages[0])
	}
	if messages[0]["reasoning_content"] != "need memory" {
		t.Fatalf("expected reasoning_content passback, got %#v", messages[0])
	}
	toolCalls := messages[0]["tool_calls"].([]map[string]interface{})
	if toolCalls[0]["id"] != "call_1" {
		t.Fatalf("expected tool call id, got %#v", toolCalls[0])
	}
	if messages[1]["role"] != "tool" || messages[1]["tool_call_id"] != "call_1" {
		t.Fatalf("expected tool result message, got %#v", messages[1])
	}
}

func TestBuildOpenRouterChatCompletionsToolMessagesUsesReasoningField(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenRouterChat, "openai/gpt-oss-120b:free", EndpointChatCompletions, GenerateInput{
		Messages: []Message{
			{
				Role:             "assistant",
				ReasoningContent: "need live news",
				ToolCalls:        []ToolCall{{ToolCallID: "call_1", ToolType: "function", ToolName: "search_web", ArgumentsJSON: `{"query":"today news China"}`}},
			},
			{Role: "tool", ToolResults: []ToolResult{{ToolCallID: "call_1", ToolName: "search_web", OutputJSON: `{"items":[]}`, Status: "success"}}},
		},
	}, false)

	messages := payload["messages"].([]map[string]interface{})
	assistant := messages[0]
	if assistant["reasoning"] != "need live news" {
		t.Fatalf("expected OpenRouter reasoning passback, got %#v", assistant)
	}
	if _, ok := assistant["reasoning_content"]; ok {
		t.Fatalf("expected no DeepSeek reasoning_content alias for OpenRouter, got %#v", assistant)
	}
	if payload["stream"] != false {
		t.Fatalf("expected chat completions request body, got %#v", payload)
	}
}

func TestParseChatCompletionsOutputSeparatesReasoningContentParts(t *testing.T) {
	result := &GenerateOutput{}
	parseChatCompletionsOutput(AdapterOpenAIChatCompletions, map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "reasoning", "text": "hidden reasoning"},
						map[string]interface{}{"type": "text", "text": "visible answer"},
					},
					"tool_calls": []interface{}{
						map[string]interface{}{
							"id":   "call_1",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "memory.list",
								"arguments": "{}",
							},
						},
					},
				},
			},
		},
	}, result, false)

	if result.Text != "visible answer" {
		t.Fatalf("expected only visible content, got %q", result.Text)
	}
	if result.Reasoning == nil || result.Reasoning.Text != "hidden reasoning" {
		t.Fatalf("expected reasoning content to be separated, got %#v", result.Reasoning)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].ToolName != "memory.list" {
		t.Fatalf("expected tool call to be preserved, got %#v", result.ToolCalls)
	}
}

func TestApplyChatStreamEventSeparatesReasoningContentParts(t *testing.T) {
	result := &GenerateOutput{}
	var visible string
	var reasoning string
	err := applyChatStreamEvent(AdapterOpenAIChatCompletions, map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"delta": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "reasoning", "text": "hidden"},
						map[string]interface{}{"type": "text", "text": "visible"},
					},
				},
			},
		},
	}, result, func(event GenerateStreamEvent) error {
		visible += event.Delta
		if event.Reasoning != nil {
			reasoning += event.Reasoning.Text
		}
		return nil
	}, false)
	if err != nil {
		t.Fatalf("apply chat stream event: %v", err)
	}
	if visible != "visible" || result.Text != "visible" {
		t.Fatalf("expected only visible stream content, visible=%q result=%q", visible, result.Text)
	}
	if reasoning != "hidden" {
		t.Fatalf("expected reasoning stream content, got %q", reasoning)
	}
	if result.Reasoning == nil || result.Reasoning.Text != "hidden" {
		t.Fatalf("expected stream reasoning to be stored for passback, got %#v", result.Reasoning)
	}
}

func TestBuildChatCompletionsCustomToolMessages(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIChatCompletions, "gpt-5", EndpointChatCompletions, GenerateInput{
		Messages: []Message{
			{Role: "assistant", ToolCalls: []ToolCall{{ToolCallID: "call_custom", ToolType: "custom", ToolName: "code_exec", ArgumentsJSON: `print("hi")`}}},
		},
	}, false)

	messages := payload["messages"].([]map[string]interface{})
	toolCalls := messages[0]["tool_calls"].([]map[string]interface{})
	if toolCalls[0]["type"] != "custom" {
		t.Fatalf("expected custom tool call, got %#v", toolCalls[0])
	}
	custom := toolCalls[0]["custom"].(map[string]interface{})
	if custom["name"] != "code_exec" || custom["input"] != `print("hi")` {
		t.Fatalf("expected custom tool payload, got %#v", custom)
	}
}

func TestBuildResponsesToolInputItems(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIResponses, "gpt-5", EndpointResponses, GenerateInput{
		Messages: []Message{
			{Role: "tool", ToolResults: []ToolResult{{ToolCallID: "call_1", ToolName: "memory.list", OutputJSON: `{"items":[]}`, Status: "success"}}},
		},
	}, false)

	items := payload["input"].([]map[string]interface{})
	if len(items) != 1 || items[0]["type"] != "function_call_output" || items[0]["call_id"] != "call_1" {
		t.Fatalf("expected function_call_output item, got %#v", items)
	}
}

func TestBuildOpenRouterResponsesToolHistoryAddsItemIDs(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenRouterResponses, "openai/o4-mini", EndpointResponses, GenerateInput{
		Messages: []Message{
			{Role: "assistant", ToolCalls: []ToolCall{{ToolCallID: "call_123", ToolName: "get_weather", ArgumentsJSON: `{"location":"Boston, MA"}`}}},
			{Role: "tool", ToolResults: []ToolResult{{ToolCallID: "call_123", ToolName: "get_weather", OutputJSON: `{"temperature":"72F"}`, Status: "success"}}},
		},
	}, false)

	items := payload["input"].([]map[string]interface{})
	if len(items) != 2 {
		t.Fatalf("expected two tool history items, got %#v", items)
	}
	if items[0]["type"] != "function_call" || items[0]["id"] == "" || items[0]["call_id"] != "call_123" {
		t.Fatalf("expected function_call item id and call_id, got %#v", items[0])
	}
	if items[1]["type"] != "function_call_output" || items[1]["id"] == "" || items[1]["call_id"] != "call_123" {
		t.Fatalf("expected function_call_output item id and call_id, got %#v", items[1])
	}
}

func TestBuildOpenAIToolsKeepsInputSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"count":{"type":"number"}},"required":["query"]}`)
	payload := mustBuildRequestBody(t, AdapterOpenAIChatCompletions, "gpt-5", EndpointChatCompletions, GenerateInput{
		Messages: []Message{{Role: "user", Content: "search"}},
		Tools: []ToolDefinition{{
			Name:        "bing_search",
			Description: "Search the web",
			InputSchema: schema,
		}},
	}, false)

	tools := payload["tools"].([]map[string]interface{})
	fn := tools[0]["function"].(map[string]interface{})
	parameters := fn["parameters"].(map[string]interface{})
	properties := parameters["properties"].(map[string]interface{})
	query := properties["query"].(map[string]interface{})
	count := properties["count"].(map[string]interface{})
	if fn["name"] != "bing_search" || query["type"] != "string" || count["type"] != "number" {
		t.Fatalf("expected OpenAI tool schema to be preserved, got %#v", tools[0])
	}
}

func TestBuildOpenAIToolsMergesProviderAndMCPTools(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)
	payload := mustBuildRequestBody(t, AdapterOpenAIChatCompletions, "gpt-5", EndpointChatCompletions, GenerateInput{
		Messages: []Message{{Role: "user", Content: "search"}},
		Options: map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"type": "web_search_preview",
				},
			},
		},
		Tools: []ToolDefinition{{
			Name:        "bing_search",
			Description: "Search the web",
			InputSchema: schema,
		}},
	}, false)

	tools := payload["tools"].([]map[string]interface{})
	if len(tools) != 2 {
		t.Fatalf("expected provider tool and MCP tool, got %#v", tools)
	}
	if tools[0]["type"] != "web_search_preview" {
		t.Fatalf("expected provider tool first, got %#v", tools[0])
	}
	fn := tools[1]["function"].(map[string]interface{})
	if fn["name"] != "bing_search" {
		t.Fatalf("expected MCP tool second, got %#v", tools[1])
	}
}

func TestBuildOpenAIResponsesNativeToolOptions(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIResponses, "gpt-5.5", EndpointResponses, GenerateInput{
		Messages: []Message{{Role: "user", Content: "search and run code"}},
		Options: map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{"type": "web_search"},
				map[string]interface{}{
					"type": "shell",
					"environment": map[string]interface{}{
						"type": "container_auto",
					},
				},
				map[string]interface{}{"type": "image_generation"},
				map[string]interface{}{
					"type": "code_interpreter",
					"container": map[string]interface{}{
						"type": "auto",
					},
				},
			},
		},
	}, true)

	tools, ok := payload["tools"].([]map[string]interface{})
	if !ok || len(tools) != 4 {
		t.Fatalf("expected four native OpenAI tools, got %#v", payload["tools"])
	}
	if tools[0]["type"] != "web_search" || tools[1]["type"] != "shell" || tools[2]["type"] != "image_generation" || tools[3]["type"] != "code_interpreter" {
		t.Fatalf("expected OpenAI native tool order to be preserved, got %#v", tools)
	}
	environment, ok := tools[1]["environment"].(map[string]interface{})
	if !ok || environment["type"] != "container_auto" {
		t.Fatalf("expected shell container_auto environment, got %#v", tools[1])
	}
	container, ok := tools[3]["container"].(map[string]interface{})
	if !ok || container["type"] != "auto" {
		t.Fatalf("expected code_interpreter auto container, got %#v", tools[3])
	}
}

func TestBuildRequestBodyDisableToolsRemovesProviderAndMCPTools(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)
	payload := mustBuildRequestBody(t, AdapterOpenAIChatCompletions, "gpt-5", EndpointChatCompletions, GenerateInput{
		Messages: []Message{{Role: "user", Content: "summarize"}},
		Options: map[string]interface{}{
			"tools": []interface{}{map[string]interface{}{"type": "web_search_preview"}},
		},
		Tools: []ToolDefinition{{
			Name:        "bing_search",
			Description: "Search the web",
			InputSchema: schema,
		}},
		DisableTools: true,
	}, false)

	if _, ok := payload["tools"]; ok {
		t.Fatalf("expected tools to be omitted when disabled, got %#v", payload["tools"])
	}
}

func TestBuildResponsesDisableToolsRemovesWebSearchAndMCPTools(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)
	payload := mustBuildRequestBody(t, AdapterOpenAIResponses, "gpt-5", EndpointResponses, GenerateInput{
		Messages: []Message{{Role: "user", Content: "summarize"}},
		Options: map[string]interface{}{
			"web_search": true,
			"tools":      []interface{}{map[string]interface{}{"type": "web_search_preview"}},
		},
		Tools: []ToolDefinition{{
			Name:        "bing_search",
			Description: "Search the web",
			InputSchema: schema,
		}},
		DisableTools: true,
	}, false)

	if _, ok := payload["tools"]; ok {
		t.Fatalf("expected tools to be omitted when disabled, got %#v", payload["tools"])
	}
}

func TestBuildAnthropicToolsMergesProviderAndMCPTools(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)
	payload := mustBuildAnthropicRequestBody(t, "claude-sonnet-4", GenerateInput{
		Messages: []Message{{Role: "user", Content: "search"}},
		Options: map[string]interface{}{
			"web_search": true,
			"tools": []interface{}{
				map[string]interface{}{
					"type": "custom_tool",
					"name": "provider_lookup",
				},
			},
		},
		Tools: []ToolDefinition{{
			Name:        "bing_search",
			Description: "Search the web",
			InputSchema: schema,
		}},
	}, false)

	tools := payload["tools"].([]map[string]interface{})
	if len(tools) != 3 {
		t.Fatalf("expected provider, web search, and MCP tools, got %#v", tools)
	}
	if tools[0]["name"] != "provider_lookup" {
		t.Fatalf("expected provider tool first, got %#v", tools[0])
	}
	if tools[1]["type"] != "web_search_20250305" {
		t.Fatalf("expected Anthropic web search second, got %#v", tools[1])
	}
	if tools[1]["name"] != "web_search" {
		t.Fatalf("expected Anthropic native web search name, got %#v", tools[1])
	}
	if _, ok := tools[1]["allowed_callers"]; ok {
		t.Fatalf("expected legacy Anthropic web search to avoid allowed_callers, got %#v", tools[1])
	}
	if tools[2]["name"] != "bing_search" {
		t.Fatalf("expected MCP tool third, got %#v", tools[2])
	}
}

func TestBuildAnthropicNativeToolsAddsRequiredNames(t *testing.T) {
	payload := mustBuildAnthropicRequestBody(t, "claude-haiku-4-5", GenerateInput{
		Messages: []Message{{Role: "user", Content: "今日天气？"}},
		Options: map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{"type": "web_search_20260209"},
				map[string]interface{}{"type": "web_fetch_20260209"},
				map[string]interface{}{"type": "code_execution_20260120"},
				map[string]interface{}{"type": "advisor_20260301"},
				map[string]interface{}{"type": "tool_search_tool_regex_20251119"},
				map[string]interface{}{"type": "tool_search_tool_bm25_20251119"},
			},
		},
	}, true)

	tools, ok := payload["tools"].([]map[string]interface{})
	if !ok || len(tools) != 6 {
		t.Fatalf("expected normalized Anthropic native tools, got %#v", payload["tools"])
	}
	expectedNames := []string{"web_search", "web_fetch", "code_execution", "advisor", "tool_search_tool_regex", "tool_search_tool_bm25"}
	for index, expected := range expectedNames {
		if tools[index]["name"] != expected {
			t.Fatalf("expected tool %d name %q, got %#v", index, expected, tools[index])
		}
	}
	for _, index := range []int{0, 1} {
		callers, ok := tools[index]["allowed_callers"].([]string)
		if !ok || len(callers) != 1 || callers[0] != "direct" {
			t.Fatalf("expected tool %d to use direct callers, got %#v", index, tools[index])
		}
	}
	if _, ok := tools[2]["allowed_callers"]; ok {
		t.Fatalf("expected code execution tool to avoid allowed_callers, got %#v", tools[2])
	}
}

func TestBuildAnthropicRequestBodyDisableToolsRemovesProviderAndMCPTools(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)
	payload := mustBuildAnthropicRequestBody(t, "claude-sonnet-4", GenerateInput{
		Messages: []Message{{Role: "user", Content: "summarize"}},
		Options: map[string]interface{}{
			"web_search": true,
			"tools": []interface{}{map[string]interface{}{
				"type": "custom_tool",
				"name": "provider_lookup",
			}},
		},
		Tools: []ToolDefinition{{
			Name:        "bing_search",
			Description: "Search the web",
			InputSchema: schema,
		}},
		DisableTools: true,
	}, false)

	if _, ok := payload["tools"]; ok {
		t.Fatalf("expected tools to be omitted when disabled, got %#v", payload["tools"])
	}
}

func TestApplyAnthropicBetaHeadersForNativeTools(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("anthropic-beta", "existing-beta")

	applyAnthropicBetaHeaders(req, map[string]interface{}{
		"tools": []map[string]interface{}{
			{"type": "web_search_20260209"},
			{"type": "web_fetch_20260209"},
			{"type": "code_execution_20260120"},
			{"type": "advisor_20260301"},
			{"type": "mcp_toolset_20251119"},
		},
	}, nil)

	header := req.Header.Get("anthropic-beta")
	if strings.Contains(header, "web-fetch") || strings.Contains(header, "code-execution") {
		t.Fatalf("expected GA native tools to avoid beta headers, got %q", header)
	}
	if !strings.Contains(header, "advisor-2026-03-01") {
		t.Fatalf("expected advisor beta header, got %q", header)
	}
	if !strings.Contains(header, "mcp-client-2025-11-20") {
		t.Fatalf("expected mcp client beta header, got %q", header)
	}
	if !strings.Contains(header, "existing-beta") {
		t.Fatalf("expected existing beta header to be preserved, got %q", header)
	}
}

func TestBuildGeminiToolsMergesProviderAndMCPTools(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)
	payload := mustBuildGeminiRequestBody(t, GenerateInput{
		Messages: []Message{{Role: "user", Content: "search"}},
		Options: map[string]interface{}{
			"web_search": true,
			"tools": []interface{}{
				map[string]interface{}{
					"url_context": map[string]interface{}{},
				},
			},
		},
		Tools: []ToolDefinition{{
			Name:        "bing_search",
			Description: "Search the web",
			InputSchema: schema,
		}},
	})

	tools := payload["tools"].([]map[string]interface{})
	if len(tools) != 3 {
		t.Fatalf("expected provider, web search, and MCP tools, got %#v", tools)
	}
	if _, ok := tools[0]["url_context"]; !ok {
		t.Fatalf("expected provider tool first, got %#v", tools[0])
	}
	if _, ok := tools[1]["google_search"]; !ok {
		t.Fatalf("expected Gemini web search second, got %#v", tools[1])
	}
	declarations := tools[2]["functionDeclarations"].([]map[string]interface{})
	if declarations[0]["name"] != "bing_search" {
		t.Fatalf("expected MCP tool third, got %#v", tools[2])
	}
	toolConfig := payload["toolConfig"].(map[string]interface{})
	if toolConfig["includeServerSideToolInvocations"] != true {
		t.Fatalf("expected Gemini server-side tool invocations to be included when mixed with function declarations, got %#v", toolConfig)
	}
}

func TestBuildGeminiToolsPreservesExplicitToolConfigWhenMixedTools(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)
	payload := mustBuildGeminiRequestBody(t, GenerateInput{
		Messages: []Message{{Role: "user", Content: "search"}},
		Options: map[string]interface{}{
			"web_search": true,
			"toolConfig": map[string]interface{}{
				"functionCallingConfig": map[string]interface{}{"mode": "ANY"},
			},
		},
		Tools: []ToolDefinition{{
			Name:        "search_web",
			Description: "Search the web",
			InputSchema: schema,
		}},
	})

	toolConfig := payload["toolConfig"].(map[string]interface{})
	if toolConfig["includeServerSideToolInvocations"] != true {
		t.Fatalf("expected mixed tools to enable server-side invocations, got %#v", toolConfig)
	}
	if _, ok := toolConfig["functionCallingConfig"].(map[string]interface{}); !ok {
		t.Fatalf("expected explicit functionCallingConfig to be preserved, got %#v", toolConfig)
	}
}

func TestBuildGeminiToolsSanitizesJSONSchemaForFunctionDeclarations(t *testing.T) {
	schema := json.RawMessage(`{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"additionalProperties": false,
		"type": "object",
		"properties": {
			"query": {
				"anyOf": [
					{"type": "string", "default": ""},
					{"type": "array", "items": {"type": "string", "additionalProperties": false}}
				],
				"description": "Search terms"
			},
			"num": {
				"type": "number",
				"default": 30
			}
		},
		"required": ["query"]
	}`)
	payload := mustBuildGeminiRequestBody(t, GenerateInput{
		Messages: []Message{{Role: "user", Content: "search"}},
		Tools: []ToolDefinition{{
			Name:        "search_web",
			Description: "Search the web",
			InputSchema: schema,
		}},
	})

	tools := payload["tools"].([]map[string]interface{})
	declarations := tools[0]["functionDeclarations"].([]map[string]interface{})
	parameters := declarations[0]["parameters"].(map[string]interface{})
	if _, ok := parameters["$schema"]; ok {
		t.Fatalf("expected $schema to be removed for Gemini, got %#v", parameters)
	}
	if _, ok := parameters["additionalProperties"]; ok {
		t.Fatalf("expected additionalProperties to be removed for Gemini, got %#v", parameters)
	}
	properties := parameters["properties"].(map[string]interface{})
	query := properties["query"].(map[string]interface{})
	anyOf := query["anyOf"].([]interface{})
	if _, ok := anyOf[0].(map[string]interface{})["default"]; ok {
		t.Fatalf("expected nested default to be removed for Gemini, got %#v", anyOf[0])
	}
	arraySchema := anyOf[1].(map[string]interface{})
	items := arraySchema["items"].(map[string]interface{})
	if _, ok := items["additionalProperties"]; ok {
		t.Fatalf("expected nested additionalProperties to be removed for Gemini, got %#v", items)
	}
	if parameters["type"] != "object" || len(parameters["required"].([]interface{})) != 1 {
		t.Fatalf("expected supported schema fields to remain, got %#v", parameters)
	}
}

func TestBuildGeminiRequestBodyDisableToolsRemovesProviderAndMCPTools(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`)
	payload := mustBuildGeminiRequestBody(t, GenerateInput{
		Messages: []Message{{Role: "user", Content: "summarize"}},
		Options: map[string]interface{}{
			"web_search": true,
			"tools": []interface{}{
				map[string]interface{}{"url_context": map[string]interface{}{}},
			},
		},
		Tools: []ToolDefinition{{
			Name:        "bing_search",
			Description: "Search the web",
			InputSchema: schema,
		}},
		DisableTools: true,
	})

	if _, ok := payload["tools"]; ok {
		t.Fatalf("expected tools to be omitted when disabled, got %#v", payload["tools"])
	}
}

func TestBuildRequestBodyRejectsInvalidProviderToolsOption(t *testing.T) {
	_, err := buildOpenAIRequestBody(AdapterOpenAIChatCompletions, "gpt-5", EndpointChatCompletions, GenerateInput{
		Messages: []Message{{Role: "user", Content: "search"}},
		Options: map[string]interface{}{
			"tools": map[string]interface{}{"type": "web_search_preview"},
		},
	}, false)
	if err == nil || err.Error() != "model option tools must be an array" {
		t.Fatalf("expected invalid tools error, got %v", err)
	}
}

func TestChatStreamToolCallArgumentsAreConcatenatedWithoutDefaultPrefix(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0)}
	chunks := []map[string]interface{}{
		{
			"id": "chatcmpl_1",
			"choices": []interface{}{map[string]interface{}{
				"delta": map[string]interface{}{
					"tool_calls": []interface{}{map[string]interface{}{
						"index": float64(0),
						"id":    "call_1",
						"type":  "function",
						"function": map[string]interface{}{
							"name": "bing_search",
						},
					}},
				},
			}},
		},
		{
			"choices": []interface{}{map[string]interface{}{
				"delta": map[string]interface{}{
					"tool_calls": []interface{}{map[string]interface{}{
						"index": float64(0),
						"function": map[string]interface{}{
							"arguments": "{\"query\":\"IDC",
						},
					}},
				},
			}},
		},
		{
			"choices": []interface{}{map[string]interface{}{
				"delta": map[string]interface{}{
					"tool_calls": []interface{}{map[string]interface{}{
						"index": float64(0),
						"function": map[string]interface{}{
							"arguments": " Flare\"}",
						},
					}},
				},
			}},
		},
	}

	for _, chunk := range chunks {
		if err := applyChatStreamEvent(AdapterOpenAIChatCompletions, chunk, result, nil, false); err != nil {
			t.Fatalf("apply stream event: %v", err)
		}
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %#v", result.ToolCalls)
	}
	if result.ToolCalls[0].ArgumentsJSON != `{"query":"IDC Flare"}` {
		t.Fatalf("unexpected arguments JSON: %q", result.ToolCalls[0].ArgumentsJSON)
	}
}

func TestChatStreamCustomToolCallInputIsConcatenated(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0)}
	chunks := []map[string]interface{}{
		{
			"id": "chatcmpl_1",
			"choices": []interface{}{map[string]interface{}{
				"delta": map[string]interface{}{
					"tool_calls": []interface{}{map[string]interface{}{
						"index": float64(0),
						"id":    "call_custom",
						"type":  "custom",
						"custom": map[string]interface{}{
							"name": "code_exec",
						},
					}},
				},
			}},
		},
		{
			"choices": []interface{}{map[string]interface{}{
				"delta": map[string]interface{}{
					"tool_calls": []interface{}{map[string]interface{}{
						"index": float64(0),
						"custom": map[string]interface{}{
							"input": "print(",
						},
					}},
				},
			}},
		},
		{
			"choices": []interface{}{map[string]interface{}{
				"delta": map[string]interface{}{
					"tool_calls": []interface{}{map[string]interface{}{
						"index": float64(0),
						"custom": map[string]interface{}{
							"input": `"hi")`,
						},
					}},
				},
			}},
		},
	}

	for _, chunk := range chunks {
		if err := applyChatStreamEvent(AdapterOpenAIChatCompletions, chunk, result, nil, false); err != nil {
			t.Fatalf("apply stream event: %v", err)
		}
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected one custom tool call, got %#v", result.ToolCalls)
	}
	if result.ToolCalls[0].ToolType != "custom" || result.ToolCalls[0].ToolName != "code_exec" || result.ToolCalls[0].ArgumentsJSON != `print("hi")` {
		t.Fatalf("unexpected custom tool call: %#v", result.ToolCalls[0])
	}
}

func TestParseChatCompletionsCustomToolCall(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "chatcmpl_1",
		"choices": [{
			"message": {
				"role": "assistant",
				"tool_calls": [{
					"id": "call_custom",
					"type": "custom",
					"custom": {"name": "code_exec", "input": "print(\"hi\")"}
				}]
			}
		}]
	}`)

	result := buildGenerateOutputFromParsed(EndpointChatCompletions, payload)
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].ToolType != "custom" || result.ToolCalls[0].ToolName != "code_exec" || result.ToolCalls[0].ArgumentsJSON != `print("hi")` {
		t.Fatalf("unexpected custom tool call: %#v", result.ToolCalls)
	}
}

func TestParseChatCompletionsDSMLToolCalls(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "chatcmpl_1",
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "<｜DSML｜tool_calls>\n<｜DSML｜invoke name=\"searchGitHub\">\n<｜DSML｜parameter name=\"query\" string=\"true\">默认启用MCP DEEIX</｜DSML｜parameter>\n</｜DSML｜invoke>\n</｜DSML｜tool_calls>"
			}
		}]
	}`)

	result := buildGenerateOutputFromParsedForAdapter(EndpointChatCompletions, AdapterOpenAIChatCompletions, payload, true)
	if result.Text != "" {
		t.Fatalf("expected DSML envelope to be removed from visible text, got %q", result.Text)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected one parsed DSML tool call, got %#v", result.ToolCalls)
	}
	call := result.ToolCalls[0]
	if call.ToolCallID != "dsml_call_1" || call.ToolType != "function" || call.ToolName != "searchGitHub" || call.Status != "requested" {
		t.Fatalf("unexpected DSML tool call: %#v", call)
	}
	if call.ArgumentsJSON != `{"query":"默认启用MCP DEEIX"}` {
		t.Fatalf("unexpected DSML arguments: %q", call.ArgumentsJSON)
	}
}

func TestParseChatCompletionsDSMLToolCallsDecodesJSONParameters(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "chatcmpl_1",
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "<｜DSML｜tool_calls>\n<｜DSML｜invoke name=\"searchGitHub\">\n<｜DSML｜parameter name=\"query\" string=\"true\">DEEIX</｜DSML｜parameter>\n<｜DSML｜parameter name=\"limit\" string=\"false\">3</｜DSML｜parameter>\n<｜DSML｜parameter name=\"filters\" string=\"false\">{\"language\":\"Go\"}</｜DSML｜parameter>\n</｜DSML｜invoke>\n</｜DSML｜tool_calls>"
			}
		}]
	}`)

	result := buildGenerateOutputFromParsedForAdapter(EndpointChatCompletions, AdapterOpenAIChatCompletions, payload, true)
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected one parsed DSML tool call, got %#v", result.ToolCalls)
	}
	if result.ToolCalls[0].ArgumentsJSON != `{"filters":{"language":"Go"},"limit":3,"query":"DEEIX"}` {
		t.Fatalf("unexpected DSML arguments: %q", result.ToolCalls[0].ArgumentsJSON)
	}
}

func TestParseChatCompletionsDSMLToolCallsDisabledByDefault(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "chatcmpl_1",
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "<｜DSML｜tool_calls><｜DSML｜invoke name=\"searchGitHub\"><｜DSML｜parameter name=\"query\" string=\"true\">DEEIX</｜DSML｜parameter></｜DSML｜invoke></｜DSML｜tool_calls>"
			}
		}]
	}`)

	result := buildGenerateOutputFromParsed(EndpointChatCompletions, payload)
	if !strings.Contains(result.Text, "DSML") {
		t.Fatalf("expected default chat completions path to keep DSML as text, got %q", result.Text)
	}
	if len(result.ToolCalls) != 0 {
		t.Fatalf("expected default chat completions path not to parse DSML tool calls, got %#v", result.ToolCalls)
	}
}

func TestTextEncodedToolCallsOnlyEnabledForDeepSeekChatCompletions(t *testing.T) {
	if !deepSeekTextEncodedToolCallsEnabled(RouteConfig{
		Protocol:      AdapterOpenAIChatCompletions,
		UpstreamModel: "deepseek-v4-flash",
	}) {
		t.Fatalf("expected DeepSeek chat completions route to enable text-encoded tool calls")
	}
	if deepSeekTextEncodedToolCallsEnabled(RouteConfig{
		Protocol:      AdapterOpenAIChatCompletions,
		UpstreamModel: "gpt-5.4",
	}) {
		t.Fatalf("expected non-DeepSeek chat completions route to keep text-encoded tool calls disabled")
	}
	if deepSeekTextEncodedToolCallsEnabled(RouteConfig{
		Protocol:      AdapterOpenAIResponses,
		UpstreamModel: "deepseek-v4-flash",
	}) {
		t.Fatalf("expected non-chat-completions route to keep text-encoded tool calls disabled")
	}
}

func TestConsumeChatStreamDSMLToolCallsAreNotEmittedAsText(t *testing.T) {
	rawStream := strings.Join([]string{
		`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"<｜DSML｜tool_calls>\n<｜DSML｜invoke name=\"searchGitHub\">\n"}}]}`,
		`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"<｜DSML｜parameter name=\"query\" string=\"true\">DEEIX MCP</｜DSML｜parameter>\n</｜DSML｜invoke>\n</｜DSML｜tool_calls>"}}]}`,
		`data: [DONE]`,
		``,
	}, "\n\n")
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0)}
	var deltas []string

	err := consumeOpenAIGenerateStream(EndpointChatCompletions, AdapterOpenAIChatCompletions, strings.NewReader(rawStream), result, func(event GenerateStreamEvent) error {
		if event.Delta != "" {
			deltas = append(deltas, event.Delta)
		}
		return nil
	}, true)
	if err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if len(deltas) != 0 || result.Text != "" {
		t.Fatalf("expected DSML stream to stay out of visible text, deltas=%#v text=%q", deltas, result.Text)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].ToolName != "searchGitHub" || result.ToolCalls[0].ArgumentsJSON != `{"query":"DEEIX MCP"}` {
		t.Fatalf("unexpected DSML stream tool calls: %#v", result.ToolCalls)
	}
}

func TestConsumeChatStreamIncompleteDSMLToolCallsReturnsError(t *testing.T) {
	rawStream := strings.Join([]string{
		`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"<｜DSML｜tool_calls>\n<｜DSML｜invoke name=\"searchGitHub\">\n"}}]}`,
		`data: [DONE]`,
		``,
	}, "\n\n")
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0)}

	err := consumeOpenAIGenerateStream(EndpointChatCompletions, AdapterOpenAIChatCompletions, strings.NewReader(rawStream), result, nil, true)
	if !errors.Is(err, errDeepSeekDSMLToolCallsIncomplete) {
		t.Fatalf("expected incomplete DSML error, got %v", err)
	}
	if result.Text != "" || len(result.ToolCalls) != 0 {
		t.Fatalf("expected incomplete DSML to stay out of output, text=%q toolCalls=%#v", result.Text, result.ToolCalls)
	}
}

func TestParseOpenAIGenerateOutputIncompleteDSMLToolCallsReturnsError(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl_1",
		"choices": [{
			"message": {
				"role": "assistant",
				"content": "<｜DSML｜tool_calls>\n<｜DSML｜invoke name=\"searchGitHub\">"
			}
		}]
	}`)

	_, err := parseOpenAIGenerateOutput(EndpointChatCompletions, AdapterOpenAIChatCompletions, body, true)
	if !errors.Is(err, errDeepSeekDSMLToolCallsIncomplete) {
		t.Fatalf("expected incomplete DSML error, got %v", err)
	}
}

func TestConsumeChatStreamAngleBracketTextStillEmits(t *testing.T) {
	rawStream := strings.Join([]string{
		`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"<"}}]}`,
		`data: {"id":"chatcmpl_1","choices":[{"delta":{"content":"not-dsml> ok"}}]}`,
		`data: [DONE]`,
		``,
	}, "\n\n")
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0)}
	var deltas []string

	err := consumeOpenAIGenerateStream(EndpointChatCompletions, AdapterOpenAIChatCompletions, strings.NewReader(rawStream), result, func(event GenerateStreamEvent) error {
		if event.Delta != "" {
			deltas = append(deltas, event.Delta)
		}
		return nil
	}, true)
	if err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if got := strings.Join(deltas, ""); got != "<not-dsml> ok" || result.Text != got {
		t.Fatalf("expected ordinary angle bracket text to stream, deltas=%#v text=%q", deltas, result.Text)
	}
	if len(result.ToolCalls) != 0 {
		t.Fatalf("expected no tool calls, got %#v", result.ToolCalls)
	}
}

func TestConsumeChatStreamErrorPayloadReturnsUpstreamError(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0)}
	stream := bytes.NewBufferString("data: {\"error\":{\"message\":\"Param Incorrect\",\"code\":400}}\n\n")

	err := consumeOpenAIGenerateStream(EndpointChatCompletions, AdapterOpenAIChatCompletions, stream, result, nil, false)
	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected upstream error, got %T %v", err, err)
	}
	if upstreamErr.StatusCode != 400 || upstreamErr.Message != "Param Incorrect" {
		t.Fatalf("unexpected upstream error: %#v", upstreamErr)
	}
	if upstreamErr.Body != `{"error":{"message":"Param Incorrect","code":400}}` {
		t.Fatalf("expected raw upstream error body, got %q", upstreamErr.Body)
	}
}

func TestProviderSpecificErrorsPreserveDebugSnapshots(t *testing.T) {
	debug := &UpstreamDebugSnapshot{
		Request: UpstreamDebugRequest{
			Method:  "POST",
			Path:    "/v1/messages",
			Headers: map[string]string{"x-api-key": "[redacted]"},
			Body:    `{"model":"claude-sonnet-4"}`,
		},
		Response: UpstreamDebugResponse{
			StatusCode: 400,
			Body:       `{"error":{"message":"invalid request"}}`,
		},
	}

	anthropicErr := parseAnthropicError(400, []byte(`{"error":{"message":"invalid request"}}`), debug)
	var upstreamErr *UpstreamError
	if !errors.As(anthropicErr, &upstreamErr) || upstreamErr.Debug == nil || upstreamErr.Debug.Request.Path != "/v1/messages" {
		t.Fatalf("expected anthropic debug snapshot, got %#v", anthropicErr)
	}

	geminiDebug := &UpstreamDebugSnapshot{
		Request: UpstreamDebugRequest{
			Method:  "POST",
			Path:    "/v1beta/models/gemini-2.0-flash:generateContent",
			Headers: map[string]string{"x-goog-api-key": "[redacted]"},
			Body:    `{"contents":[]}`,
		},
		Response: UpstreamDebugResponse{
			StatusCode: 400,
			Body:       `{"error":{"message":"bad request"}}`,
		},
	}
	geminiErr := parseGeminiError(400, []byte(`{"error":{"message":"bad request"}}`), geminiDebug)
	upstreamErr = nil
	if !errors.As(geminiErr, &upstreamErr) || upstreamErr.Debug == nil || upstreamErr.Debug.Request.Path != "/v1beta/models/gemini-2.0-flash:generateContent" {
		t.Fatalf("expected gemini debug snapshot, got %#v", geminiErr)
	}
}

func TestAttachUpstreamDebugWrapsStreamErrors(t *testing.T) {
	debug := &UpstreamDebugSnapshot{
		Request: UpstreamDebugRequest{
			Method:  "POST",
			Path:    "/v1/responses",
			Headers: map[string]string{"Authorization": "[redacted]"},
			Body:    `{"model":"grok-4.3","input":[{"role":"user","content":[{"type":"input_text","text":"武汉天气"}]}],"tools":[{"type":"x_search"}],"stream":true}`,
		},
		Response: UpstreamDebugResponse{
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "text/event-stream"},
			Body:       `{"type":"response.error","error":{"message":"Argument not supported: metadata","code":"bad_response_status_code"}}`,
		},
	}

	err := attachUpstreamDebug(errors.New("Argument not supported: metadata"), debug)
	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected upstream error, got %T %v", err, err)
	}
	if upstreamErr.Debug == nil || upstreamErr.Debug.Request.Path != "/v1/responses" {
		t.Fatalf("expected upstream request snapshot, got %#v", upstreamErr.Debug)
	}
	if upstreamErr.Debug.Request.Body == "" || upstreamErr.Debug.Response.Body == "" {
		t.Fatalf("expected complete request and response bodies, got %#v", upstreamErr.Debug)
	}
	if upstreamErr.StatusCode != 400 || upstreamErr.Body == "" {
		t.Fatalf("expected response status/body to be preserved, got %#v", upstreamErr)
	}
}

func TestStreamDebugSnapshotPreservesRawSSEBody(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0)}
	rawStream := "event: response.error\ndata: {\"type\":\"response.error\",\"error\":{\"message\":\"Argument not supported: metadata\"}}\n\n"
	recorder := newUpstreamBodyRecorder(bytes.NewBufferString(rawStream))

	err := consumeOpenAIGenerateStream(EndpointResponses, AdapterXAIResponses, recorder, result, nil, false)
	req, reqErr := http.NewRequest(http.MethodPost, "https://api.x.ai/v1/responses", strings.NewReader(`{"model":"grok-4.3"}`))
	if reqErr != nil {
		t.Fatal(reqErr)
	}
	debug := upstreamDebugSnapshot(
		req,
		[]byte(`{"model":"grok-4.3"}`),
		&http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}},
		streamErrorBody(recorder, err),
	)
	if debug.Response.Body != rawStream {
		t.Fatalf("expected raw SSE response body, got %q", debug.Response.Body)
	}
}

func TestResponsesStreamReasoningSummaryDeltaIsEmittedAndStored(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0)}
	rawStream := strings.Join([]string{
		`event: response.reasoning_summary_text.delta`,
		`data: {"type":"response.reasoning_summary_text.delta","item_id":"rs_1","delta":"先梳理问题"}`,
		``,
		`event: response.reasoning_summary_text.delta`,
		`data: {"type":"response.reasoning_summary_text.delta","item_id":"rs_1","delta":"，再给出答案。"}`,
		``,
	}, "\n")

	reasoningText := ""
	err := consumeOpenAIGenerateStream(EndpointResponses, AdapterOpenAIResponses, strings.NewReader(rawStream), result, func(event GenerateStreamEvent) error {
		if event.Reasoning != nil {
			reasoningText += event.Reasoning.Text
		}
		return nil
	}, false)
	if err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if reasoningText != "先梳理问题，再给出答案。" {
		t.Fatalf("expected reasoning deltas to be emitted, got %q", reasoningText)
	}
	if result.Reasoning == nil || result.Reasoning.Summary != reasoningText {
		t.Fatalf("expected stream reasoning summary to be stored, got %#v", result.Reasoning)
	}
}

func TestResponsesCompletedReasoningSummaryIsEmittedWhenNoDeltaArrived(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0)}
	rawStream := strings.Join([]string{
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"resp_1","output":[{"id":"rs_1","type":"reasoning","status":"completed","summary":[{"type":"summary_text","text":"这是完整思考摘要。"}]},{"type":"message","content":[{"type":"output_text","text":"最终答案"}]}],"usage":{"input_tokens":1,"output_tokens":2}}}`,
		``,
	}, "\n")

	reasoningText := ""
	err := consumeOpenAIGenerateStream(EndpointResponses, AdapterOpenAIResponses, strings.NewReader(rawStream), result, func(event GenerateStreamEvent) error {
		if event.Reasoning != nil {
			reasoningText += event.Reasoning.Text
		}
		return nil
	}, false)
	if err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if reasoningText != "这是完整思考摘要。" {
		t.Fatalf("expected completed reasoning summary to be emitted, got %q", reasoningText)
	}
	if result.Text != "最终答案" {
		t.Fatalf("expected completed response text, got %q", result.Text)
	}
	if result.Reasoning == nil || result.Reasoning.Summary != reasoningText {
		t.Fatalf("expected completed reasoning summary to be stored, got %#v", result.Reasoning)
	}
}

func TestResponsesStreamDoneEventsAreMergedWithoutDuplicateText(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0)}
	rawStream := strings.Join([]string{
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"Hello"}`,
		``,
		`event: response.output_text.done`,
		`data: {"type":"response.output_text.done","text":"Hello"}`,
		``,
		`event: response.reasoning_summary_text.done`,
		`data: {"type":"response.reasoning_summary_text.done","item_id":"rs_1","text":"简要思考"}`,
		``,
	}, "\n")

	if err := consumeOpenAIGenerateStream(EndpointResponses, AdapterOpenAIResponses, strings.NewReader(rawStream), result, nil, false); err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if result.Text != "Hello" {
		t.Fatalf("expected output_text.done not to duplicate text, got %q", result.Text)
	}
	if result.Reasoning == nil || result.Reasoning.Summary != "简要思考" {
		t.Fatalf("expected reasoning done text to be stored, got %#v", result.Reasoning)
	}
}

func TestParseResponsesSeparatesFunctionCallsFromServerSideToolCalls(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "resp_1",
		"output": [
			{"type":"x_search_call","id":"xsc_1","status":"completed","action":{"query":"latest xAI news"},"results":[{"url":"https://x.ai/news","title":"xAI"}]},
			{"type":"function_call","call_id":"call_1","name":"memory.save","arguments":"{\"text\":\"remember\"}"},
			{"type":"message","content":[{"type":"output_text","text":"done"}]}
		],
		"usage": {
			"prompt_tokens": 11,
			"completion_tokens": 13,
			"prompt_tokens_details": {"cached_tokens": 5},
			"completion_tokens_details": {"reasoning_tokens": 7},
			"server_side_tool_usage": {"x_search": 1}
		}
	}`)

	result := buildGenerateOutputFromParsed(EndpointResponses, payload)
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].ToolName != "memory.save" {
		t.Fatalf("expected only function_call to require local execution, got %#v", result.ToolCalls)
	}
	if len(result.ServerToolCalls) != 1 || result.ServerToolCalls[0].ToolType != "x_search_call" || result.ServerToolCalls[0].ToolName != "x_search" {
		t.Fatalf("expected x_search_call as server-side tool trace, got %#v", result.ServerToolCalls)
	}
	if result.Text != "done" {
		t.Fatalf("expected text output, got %q", result.Text)
	}
	if result.Usage.InputTokens != 6 || result.Usage.OutputTokens != 6 || result.Usage.CacheReadTokens != 5 || result.Usage.ReasoningTokens != 7 {
		t.Fatalf("unexpected usage fallback parse: %+v", result.Usage)
	}
	if result.ServerSideToolUsage["x_search"] != 1 {
		t.Fatalf("expected server side tool usage, got %#v", result.ServerSideToolUsage)
	}
}

func TestParseResponsesCapturesServerSideToolCallOutputItems(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "resp_1",
		"output": [
			{"type":"x_search_call","id":"xsc_1","status":"completed","action":{"query":"latest xAI news","sources":[{"url":"https://x.ai/source"}]}},
			{"type":"x_search_call_output","call_id":"xsc_1","outputs":[{"url":"https://x.ai/news","title":"xAI News"}]}
		]
	}`)

	result := buildGenerateOutputFromParsed(EndpointResponses, payload)
	if len(result.ServerToolCalls) != 1 {
		t.Fatalf("expected merged server-side tool call, got %#v", result.ServerToolCalls)
	}
	call := result.ServerToolCalls[0]
	if call.ToolCallID != "xsc_1" || call.ToolType != "x_search_call_output" || call.ToolName != "x_search" {
		t.Fatalf("unexpected server-side tool call identity: %#v", call)
	}
	if call.OutputJSON == "" || !strings.Contains(call.OutputJSON, "https://x.ai/news") {
		t.Fatalf("expected tool output item to be captured, got %#v", call.OutputJSON)
	}
	if len(result.Citations) != 2 || result.Citations[0] != "https://x.ai/source" || result.Citations[1] != "https://x.ai/news" {
		t.Fatalf("expected citations from action sources and output, got %#v", result.Citations)
	}
}

func TestParseResponsesCapturesSearchActionSourcesAsToolOutput(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "resp_1",
		"output": [
			{"type":"web_search_call","id":"wsc_1","status":"completed","action":{"type":"search","query":"today news","sources":[{"type":"url","url":"https://example.com/a"},{"type":"url","url":"https://example.com/b"}]}}
		]
	}`)

	result := buildGenerateOutputFromParsed(EndpointResponses, payload)
	if len(result.ServerToolCalls) != 1 {
		t.Fatalf("expected server-side web search call, got %#v", result.ServerToolCalls)
	}
	call := result.ServerToolCalls[0]
	if !strings.Contains(call.ArgumentsJSON, "today news") || strings.Contains(call.ArgumentsJSON, "https://example.com/a") {
		t.Fatalf("expected request action without sources, got %q", call.ArgumentsJSON)
	}
	if !strings.Contains(call.OutputJSON, "https://example.com/a") || !strings.Contains(call.OutputJSON, "today news") {
		t.Fatalf("expected action sources as tool output, got %q", call.OutputJSON)
	}
	if len(result.Citations) != 2 || result.Citations[0] != "https://example.com/a" || result.Citations[1] != "https://example.com/b" {
		t.Fatalf("expected citations from action sources, got %#v", result.Citations)
	}
}

func TestParseResponsesCapturesTopLevelServerSideToolCalls(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "resp_1",
		"tool_calls": [
			{"type":"web_search_call","id":"wsc_1","status":"completed","action":{"query":"weather"},"results":[{"url":"https://example.com/weather"}]},
			{"type":"x_search_call","id":"xsc_1","status":"in_progress","action":{"query":"xAI"}},
			{"type":"code_interpreter_call","id":"cic_1","status":"completed","input":{"code":"print(1)"},"outputs":[{"type":"logs","logs":"1"}]}
		],
		"output": [
			{"type":"message","content":[{"type":"output_text","text":"done"}]}
		]
	}`)

	result := buildGenerateOutputFromParsed(EndpointResponses, payload)
	if len(result.ServerToolCalls) != 3 {
		t.Fatalf("expected top-level server-side tool calls, got %#v", result.ServerToolCalls)
	}
	if result.ServerToolCalls[0].ToolCallID != "wsc_1" || result.ServerToolCalls[0].OutputJSON == "" {
		t.Fatalf("expected web search call output to be captured, got %#v", result.ServerToolCalls[0])
	}
	if result.ServerToolCalls[1].ToolCallID != "xsc_1" || result.ServerToolCalls[1].Status != "in_progress" || result.ServerToolCalls[1].ArgumentsJSON == "" {
		t.Fatalf("expected x search call status and action to be captured, got %#v", result.ServerToolCalls[1])
	}
	if result.ServerToolCalls[2].ToolCallID != "cic_1" || result.ServerToolCalls[2].OutputJSON == "" {
		t.Fatalf("expected code interpreter output to be captured, got %#v", result.ServerToolCalls[2])
	}
	if len(result.Citations) != 1 || result.Citations[0] != "https://example.com/weather" {
		t.Fatalf("expected citations from top-level server-side tool calls, got %#v", result.Citations)
	}
}

func TestParseResponsesCapturesOpenAINativeShellAndImageTools(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "resp_1",
		"output": [
			{"type":"shell_call","id":"sh_1","status":"completed","input":{"cmd":"ls -la"},"output":{"stdout":"total 0"}},
			{"type":"image_generation_call","id":"img_1","status":"completed","result":{"type":"image","url":"https://example.com/image.png"}}
		],
		"usage": {
			"server_side_tool_usage_details": {"shell_calls":1,"image_generation_calls":1}
		}
	}`)

	result := buildGenerateOutputFromParsed(EndpointResponses, payload)
	if len(result.ServerToolCalls) != 2 {
		t.Fatalf("expected shell and image generation traces, got %#v", result.ServerToolCalls)
	}
	if result.ServerToolCalls[0].ToolName != "shell" || !strings.Contains(result.ServerToolCalls[0].ArgumentsJSON, "ls -la") || !strings.Contains(result.ServerToolCalls[0].OutputJSON, "total 0") {
		t.Fatalf("expected shell input/output to be captured, got %#v", result.ServerToolCalls[0])
	}
	if result.ServerToolCalls[1].ToolName != "image_generation" || !strings.Contains(result.ServerToolCalls[1].OutputJSON, "https://example.com/image.png") {
		t.Fatalf("expected image generation output to be captured, got %#v", result.ServerToolCalls[1])
	}
	if result.ServerSideToolUsage["shell"] != 1 || result.ServerSideToolUsage["image_generation"] != 1 {
		t.Fatalf("expected native tool usage to be normalized, got %#v", result.ServerSideToolUsage)
	}
	if len(result.Citations) != 1 || result.Citations[0] != "https://example.com/image.png" {
		t.Fatalf("expected image URL citation to be collected, got %#v", result.Citations)
	}
}

func TestParseResponsesCapturesGeneratedImageWithoutBase64Trace(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "resp_1",
		"output": [
			{
				"type":"image_generation_call",
				"id":"img_1",
				"status":"completed",
				"output_format":"png",
				"revised_prompt":"A dog running",
				"result":"ZmluYWw="
			}
		]
	}`)

	result := buildGenerateOutputFromParsed(EndpointResponses, payload)
	if len(result.GeneratedImages) != 1 {
		t.Fatalf("expected one generated image, got %#v", result.GeneratedImages)
	}
	image := result.GeneratedImages[0]
	if image.B64JSON != "ZmluYWw=" || image.MIMEType != "image/png" || image.RevisedPrompt != "A dog running" {
		t.Fatalf("unexpected generated image: %#v", image)
	}
	if len(result.ServerToolCalls) != 1 {
		t.Fatalf("expected one image tool trace, got %#v", result.ServerToolCalls)
	}
	outputJSON := result.ServerToolCalls[0].OutputJSON
	if strings.Contains(outputJSON, "ZmluYWw=") {
		t.Fatalf("expected image base64 to be excluded from tool trace, got %q", outputJSON)
	}
	if !strings.Contains(outputJSON, `"image_generated":true`) {
		t.Fatalf("expected generated image metadata in tool trace, got %q", outputJSON)
	}
}

func TestResponsesStreamAcceptsLargePartialImageAndKeepsOnlyFinalImage(t *testing.T) {
	partial := strings.Repeat("A", 2*1024*1024)
	partialPayload, err := json.Marshal(map[string]interface{}{
		"type":                "response.image_generation_call.partial_image",
		"item_id":             "img_1",
		"output_format":       "png",
		"output_index":        0,
		"partial_image_index": 3,
		"partial_image_b64":   partial,
	})
	if err != nil {
		t.Fatalf("marshal partial image event: %v", err)
	}
	completedPayload, err := json.Marshal(map[string]interface{}{
		"type": "response.completed",
		"response": map[string]interface{}{
			"id": "resp_1",
			"output": []interface{}{
				map[string]interface{}{
					"type":          "image_generation_call",
					"id":            "img_1",
					"status":        "completed",
					"output_format": "png",
					"result":        "ZmluYWw=",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal completed event: %v", err)
	}
	rawStream := strings.Join([]string{
		"event: response.image_generation_call.partial_image",
		"data: " + string(partialPayload),
		"",
		"event: response.completed",
		"data: " + string(completedPayload),
		"",
	}, "\n")

	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0), ServerToolCalls: make([]ToolCall, 0)}
	partialEvents := 0
	err = consumeOpenAIGenerateStream(EndpointResponses, AdapterOpenAIResponses, strings.NewReader(rawStream), result, func(event GenerateStreamEvent) error {
		if event.GeneratedImage == nil {
			return nil
		}
		partialEvents++
		if !event.GeneratedImagePartial || event.GeneratedImageIndex != 3 {
			t.Fatalf("unexpected partial image event metadata: %#v", event)
		}
		if event.GeneratedImage.B64JSON != partial || event.GeneratedImage.MIMEType != "image/png" {
			t.Fatalf("unexpected partial image event: %#v", event.GeneratedImage)
		}
		return nil
	}, false)
	if err != nil {
		t.Fatalf("consume large image stream: %v", err)
	}
	if partialEvents != 1 {
		t.Fatalf("expected one partial image event, got %d", partialEvents)
	}
	if len(result.GeneratedImages) != 1 || result.GeneratedImages[0].B64JSON != "ZmluYWw=" {
		t.Fatalf("expected only the final image to be persisted, got %#v", result.GeneratedImages)
	}
	if len(result.ServerToolCalls) != 1 || strings.Contains(result.ServerToolCalls[0].OutputJSON, "ZmluYWw=") {
		t.Fatalf("expected sanitized final image tool trace, got %#v", result.ServerToolCalls)
	}
}

func TestParseResponsesTreatsXSearchCustomCallsAsServerSide(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "resp_1",
		"output": [
			{"type":"custom_tool_call","id":"ctc_1","call_id":"xs_0","name":"x_keyword_search","status":"completed","input":"{\"query\":\"since:2026-05-11 news\",\"limit\":\"5\",\"mode\":\"Latest\"}"},
			{"type":"custom_tool_call","id":"ctc_2","call_id":"xs_1","name":"x_semantic_search","status":"completed","input":"{\"query\":\"today's news\"}"}
		],
		"usage": {
			"server_side_tool_usage_details": {"web_search_calls":0,"x_search_calls":2,"code_interpreter_calls":0}
		}
	}`)

	result := buildGenerateOutputFromParsed(EndpointResponses, payload)
	if len(result.ToolCalls) != 0 {
		t.Fatalf("expected x_search native custom calls not to require local execution, got %#v", result.ToolCalls)
	}
	if len(result.ServerToolCalls) != 2 {
		t.Fatalf("expected x_search native custom calls as server-side tools, got %#v", result.ServerToolCalls)
	}
	if result.ServerToolCalls[0].ToolName != "x_keyword_search" || !strings.Contains(result.ServerToolCalls[0].ArgumentsJSON, "since:2026-05-11") {
		t.Fatalf("expected x_keyword_search input, got %#v", result.ServerToolCalls[0])
	}
	if result.ServerSideToolUsage["x_search"] != 2 {
		t.Fatalf("expected normalized x_search usage details, got %#v", result.ServerSideToolUsage)
	}
}

func TestParseResponsesCapturesMixedWebAndXSearchToolShapes(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "resp_1",
		"output": [
			{"id":"ws_1","type":"web_search_call","status":"completed","action":{"type":"search","query":"today news","sources":[{"type":"url","url":"https://example.com/news"}]}},
			{"id":"ws_2","type":"web_search_call","status":"completed","action":{"type":"open_page","url":"https://example.com/news"}},
			{"id":"ctc_1","call_id":"xs_0","type":"custom_tool_call","name":"x_keyword_search","status":"completed","input":"{\"query\":\"today news\",\"limit\":\"5\",\"mode\":\"Latest\"}"}
		],
		"usage": {
			"server_side_tool_usage_details": {"web_search_calls":2,"x_search_calls":1}
		}
	}`)

	result := buildGenerateOutputFromParsed(EndpointResponses, payload)
	if len(result.ToolCalls) != 0 {
		t.Fatalf("expected no local tool calls for native search tools, got %#v", result.ToolCalls)
	}
	if len(result.ServerToolCalls) != 3 {
		t.Fatalf("expected web search, open page, and x search traces, got %#v", result.ServerToolCalls)
	}
	if result.ServerToolCalls[0].ToolName != "web_search" || !strings.Contains(result.ServerToolCalls[0].OutputJSON, "https://example.com/news") {
		t.Fatalf("expected web search sources as output, got %#v", result.ServerToolCalls[0])
	}
	if result.ServerToolCalls[1].ToolName != "web_search" || !strings.Contains(result.ServerToolCalls[1].ArgumentsJSON, "open_page") {
		t.Fatalf("expected open_page action as web search request, got %#v", result.ServerToolCalls[1])
	}
	if result.ServerToolCalls[2].ToolName != "x_keyword_search" || result.ServerToolCalls[2].ToolCallID != "ctc_1" {
		t.Fatalf("expected xAI x_search custom item id to be preserved, got %#v", result.ServerToolCalls[2])
	}
	if result.ServerSideToolUsage["web_search"] != 2 || result.ServerSideToolUsage["x_search"] != 1 {
		t.Fatalf("expected mixed native tool usage to be normalized, got %#v", result.ServerSideToolUsage)
	}
}

func TestParseXAIResponsesNormalizesBillableNativeToolUsage(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "resp_1",
		"usage": {
			"server_side_tool_usage_details": {
				"web_search_calls": 1,
				"x_search_calls": 2,
				"code_interpreter_calls": 3,
				"file_attachment_search_calls": 4,
				"collection_search_calls": 5
			}
		}
	}`)

	result := buildGenerateOutputFromParsed(EndpointResponses, payload)
	if result.ServerSideToolUsage["web_search"] != 1 ||
		result.ServerSideToolUsage["x_search"] != 2 ||
		result.ServerSideToolUsage["code_interpreter"] != 3 ||
		result.ServerSideToolUsage["attachment_search"] != 4 ||
		result.ServerSideToolUsage["collections_search"] != 5 {
		t.Fatalf("expected xAI billable native tool usage to be normalized, got %#v", result.ServerSideToolUsage)
	}
}

func TestParseResponsesKeepsClientToolCallsGeneric(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "resp_1",
		"output": [
			{"type":"custom_tool_call","call_id":"ctc_1","name":"browser.open","input":{"url":"https://example.com"}},
			{"type":"project_tool_call","call_id":"ptc_1","name":"workspace.lookup","input":{"query":"readme"}}
		],
		"tool_calls": [
			{"type":"custom_tool_call","call_id":"ctc_2","name":"memory.save","input":{"text":"remember"}}
		]
	}`)

	result := buildGenerateOutputFromParsed(EndpointResponses, payload)
	if len(result.ServerToolCalls) != 0 {
		t.Fatalf("expected no server-side tool calls for client tools, got %#v", result.ServerToolCalls)
	}
	if len(result.ToolCalls) != 3 {
		t.Fatalf("expected generic client tool calls, got %#v", result.ToolCalls)
	}
	if result.ToolCalls[0].ToolName != "browser.open" || result.ToolCalls[0].ArgumentsJSON == "" {
		t.Fatalf("expected custom tool call input to be captured, got %#v", result.ToolCalls[0])
	}
	if result.ToolCalls[1].ToolType != "project_tool_call" || result.ToolCalls[1].ToolName != "workspace.lookup" {
		t.Fatalf("expected unknown client tool call type to stay generic, got %#v", result.ToolCalls[1])
	}
	if result.ToolCalls[2].ToolCallID != "ctc_2" || result.ToolCalls[2].ToolName != "memory.save" {
		t.Fatalf("expected top-level client tool call to be captured, got %#v", result.ToolCalls[2])
	}
}

func TestParseResponsesCitationsFromOutputAnnotations(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "resp_1",
		"output": [
			{"type":"message","content":[{"type":"output_text","text":"source","annotations":[{"type":"url_citation","url":"https://example.com/a"}]}]},
			{"type":"web_search_call","id":"wsc_1","status":"completed","results":[{"url":"https://example.com/b"}]}
		]
	}`)

	result := buildGenerateOutputFromParsed(EndpointResponses, payload)
	if len(result.Citations) != 2 || result.Citations[0] != "https://example.com/a" || result.Citations[1] != "https://example.com/b" {
		t.Fatalf("expected citations from annotations and server tool results, got %#v", result.Citations)
	}
}

func TestResponsesOutputItemDoneCapturesServerSideToolCall(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0), ServerToolCalls: make([]ToolCall, 0)}
	rawStream := strings.Join([]string{
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","item":{"type":"web_search_call","id":"wsc_1","status":"completed","action":{"query":"weather"},"sources":[{"url":"https://example.com/weather"}]}}`,
		``,
	}, "\n")

	err := consumeOpenAIGenerateStream(EndpointResponses, AdapterOpenAIResponses, strings.NewReader(rawStream), result, nil, false)
	if err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if len(result.ToolCalls) != 0 {
		t.Fatalf("expected no local tool calls, got %#v", result.ToolCalls)
	}
	if len(result.ServerToolCalls) != 1 || result.ServerToolCalls[0].ToolType != "web_search_call" {
		t.Fatalf("expected server-side web search call, got %#v", result.ServerToolCalls)
	}
	if len(result.Citations) != 1 || result.Citations[0] != "https://example.com/weather" {
		t.Fatalf("expected citations from server-side tool, got %#v", result.Citations)
	}
}

func TestResponsesStreamEmitsServerSideToolStatusEvents(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0), ServerToolCalls: make([]ToolCall, 0)}
	rawStream := strings.Join([]string{
		`event: response.web_search_call.in_progress`,
		`data: {"type":"response.web_search_call.in_progress","item_id":"wsc_1"}`,
		``,
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","item":{"type":"web_search_call","id":"wsc_1","status":"completed","action":{"query":"weather"},"sources":[{"url":"https://example.com/weather"}]}}`,
		``,
	}, "\n")

	statuses := make([]string, 0)
	err := consumeOpenAIGenerateStream(EndpointResponses, AdapterOpenAIResponses, strings.NewReader(rawStream), result, func(event GenerateStreamEvent) error {
		if event.ServerToolCall != nil {
			statuses = append(statuses, event.ServerToolCall.Status)
		}
		return nil
	}, false)
	if err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if len(statuses) != 2 || statuses[0] != "in_progress" || statuses[1] != "completed" {
		t.Fatalf("expected streamed server tool statuses, got %#v", statuses)
	}
	if len(result.ServerToolCalls) != 1 || result.ServerToolCalls[0].Status != "completed" {
		t.Fatalf("expected final merged server tool call, got %#v", result.ServerToolCalls)
	}
}

func TestResponsesServerToolFinalItemReplacesStreamingPlaceholder(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0), ServerToolCalls: make([]ToolCall, 0)}
	rawStream := strings.Join([]string{
		`event: response.web_search_call.searching`,
		`data: {"type":"response.web_search_call.searching","action":{"type":"search","query":""}}`,
		``,
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","item":{"type":"web_search_call","id":"wsc_1","status":"completed","action":{"type":"search","query":"今日新闻","sources":[{"url":"https://example.com/news"}]}}}`,
		``,
	}, "\n")

	if err := consumeOpenAIGenerateStream(EndpointResponses, AdapterOpenAIResponses, strings.NewReader(rawStream), result, nil, false); err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if len(result.ServerToolCalls) != 1 {
		t.Fatalf("expected final item to replace streaming placeholder, got %#v", result.ServerToolCalls)
	}
	call := result.ServerToolCalls[0]
	if call.ToolCallID != "wsc_1" || call.Status != "completed" || !strings.Contains(call.ArgumentsJSON, "今日新闻") {
		t.Fatalf("unexpected merged server tool call: %#v", call)
	}
}

func TestResponsesStreamStatusEventCapturesNestedServerToolItem(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0), ServerToolCalls: make([]ToolCall, 0)}
	rawStream := strings.Join([]string{
		`event: response.web_search_call.searching`,
		`data: {"type":"response.web_search_call.searching","item":{"id":"wsc_1","action":{"query":"weather"},"sources":[{"url":"https://example.com/source"}]}}`,
		``,
	}, "\n")

	var streamed *ToolCall
	err := consumeOpenAIGenerateStream(EndpointResponses, AdapterOpenAIResponses, strings.NewReader(rawStream), result, func(event GenerateStreamEvent) error {
		if event.ServerToolCall != nil {
			value := *event.ServerToolCall
			streamed = &value
		}
		return nil
	}, false)
	if err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if streamed == nil || streamed.ToolCallID != "wsc_1" || streamed.Status != "searching" || streamed.ArgumentsJSON == "" || streamed.OutputJSON == "" {
		t.Fatalf("expected nested status event payload to be captured, got %#v", streamed)
	}
	if len(result.ServerToolCalls) != 1 || result.ServerToolCalls[0].ToolCallID != "wsc_1" {
		t.Fatalf("expected merged server tool call, got %#v", result.ServerToolCalls)
	}
}

func TestResponsesStreamCapturesXSearchCustomToolInput(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0), ServerToolCalls: make([]ToolCall, 0)}
	rawStream := strings.Join([]string{
		`event: response.output_item.added`,
		`data: {"type":"response.output_item.added","item":{"id":"ctc_1","call_id":"xs_0","input":"","name":"x_keyword_search","type":"custom_tool_call","status":"in_progress"},"output_index":1}`,
		``,
		`event: response.custom_tool_call_input.delta`,
		`data: {"type":"response.custom_tool_call_input.delta","item_id":"ctc_1","delta":"{\"query\":\"news\""}`,
		``,
		`event: response.custom_tool_call_input.done`,
		`data: {"type":"response.custom_tool_call_input.done","item_id":"ctc_1","input":"{\"query\":\"news\",\"limit\":\"5\"}"}`,
		``,
		`event: response.output_item.done`,
		`data: {"type":"response.output_item.done","item":{"id":"ctc_1","call_id":"xs_0","input":"{\"query\":\"news\",\"limit\":\"5\"}","name":"x_keyword_search","type":"custom_tool_call","status":"completed"},"output_index":1}`,
		``,
	}, "\n")

	events := make([]ToolCall, 0)
	err := consumeOpenAIGenerateStream(EndpointResponses, AdapterXAIResponses, strings.NewReader(rawStream), result, func(event GenerateStreamEvent) error {
		if event.ServerToolCall != nil {
			events = append(events, *event.ServerToolCall)
		}
		return nil
	}, false)
	if err != nil {
		t.Fatalf("consume stream: %v", err)
	}
	if len(result.ToolCalls) != 0 {
		t.Fatalf("expected no local tool calls, got %#v", result.ToolCalls)
	}
	if len(result.ServerToolCalls) != 1 || result.ServerToolCalls[0].ToolName != "x_keyword_search" || !strings.Contains(result.ServerToolCalls[0].ArgumentsJSON, `"limit":"5"`) {
		t.Fatalf("expected streamed x_search custom input, got %#v", result.ServerToolCalls)
	}
	if len(events) < 2 {
		t.Fatalf("expected streamed server tool events, got %#v", events)
	}
	foundInputDone := false
	for _, event := range events {
		if event.ToolCallID == "ctc_1" && event.ToolName == "x_keyword_search" && strings.Contains(event.ArgumentsJSON, `"limit":"5"`) {
			foundInputDone = true
		}
	}
	if !foundInputDone {
		t.Fatalf("expected custom tool input done event to update the existing server tool call, got %#v", events)
	}
}

func TestBuildAnthropicToolBlocks(t *testing.T) {
	content := buildAnthropicContent(Message{
		Role:        "tool",
		ToolResults: []ToolResult{{ToolCallID: "toolu_1", ToolName: "memory.list", OutputJSON: `{"items":[]}`, Status: "success"}},
	})

	blocks := content.([]map[string]interface{})
	if len(blocks) != 1 || blocks[0]["type"] != "tool_result" || blocks[0]["tool_use_id"] != "toolu_1" {
		t.Fatalf("expected anthropic tool_result block, got %#v", blocks)
	}
}

func TestAnthropicStreamToolUseInputJSONDeltaIsCaptured(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0)}
	rawStream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":10}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_1","name":"memory.save","input":{}}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"text\":\"hello"}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":" world\"}"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":1}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	if err := consumeAnthropicStream(strings.NewReader(rawStream), result, nil, anthropicToolClassifier{}); err != nil {
		t.Fatalf("consume anthropic stream: %v", err)
	}
	compactAnthropicStreamToolCalls(result)
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %#v", result.ToolCalls)
	}
	if result.ToolCalls[0].ToolCallID != "toolu_1" || result.ToolCalls[0].ToolName != "memory.save" || result.ToolCalls[0].ArgumentsJSON != `{"text":"hello world"}` {
		t.Fatalf("unexpected anthropic stream tool call: %#v", result.ToolCalls[0])
	}
}

func TestAnthropicStreamRejectsNativeToolUseReturnedAsClientTool(t *testing.T) {
	payload := mustBuildAnthropicRequestBody(t, "claude-sonnet-4-6", GenerateInput{
		Messages: []Message{{Role: "user", Content: "北京今天天气"}},
		Options: map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{"type": "web_search_20260209"},
			},
		},
	}, true)
	classifier := newAnthropicToolClassifier(payload, nil)
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0), ServerToolCalls: make([]ToolCall, 0)}
	events := make([]ToolCall, 0)
	rawStream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":10}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"web_search","input":{"query":"北京天气"}}}`,
		``,
	}, "\n")

	err := consumeAnthropicStream(strings.NewReader(rawStream), result, func(event GenerateStreamEvent) error {
		if event.ServerToolCall != nil {
			events = append(events, *event.ServerToolCall)
		}
		return nil
	}, classifier)
	if err == nil {
		t.Fatalf("expected native tool mismatch error")
	}
	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected upstream error, got %T %v", err, err)
	}
	if !strings.Contains(upstreamErr.Message, "client-side tool call") {
		t.Fatalf("expected clear native tool error, got %q", upstreamErr.Message)
	}
	if len(result.ToolCalls) != 0 {
		t.Fatalf("expected no local tool calls, got %#v", result.ToolCalls)
	}
	if len(result.ServerToolCalls) != 1 || result.ServerToolCalls[0].Status != "error" || result.ServerToolCalls[0].ToolName != "web_search" {
		t.Fatalf("expected failed server-side tool trace, got %#v", result.ServerToolCalls)
	}
	if len(events) != 1 || events[0].Status != "error" {
		t.Fatalf("expected one error trace event, got %#v", events)
	}
}

func TestAnthropicStreamKeepsDeclaredClientToolWithNativeNameLocal(t *testing.T) {
	payload := mustBuildAnthropicRequestBody(t, "claude-sonnet-4-6", GenerateInput{
		Messages: []Message{{Role: "user", Content: "search"}},
		Options: map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{"type": "web_search_20260209"},
			},
		},
		Tools: []ToolDefinition{{Name: "web_search", Description: "Local search", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	}, true)
	classifier := newAnthropicToolClassifier(payload, []ToolDefinition{{Name: "web_search"}})
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0), ServerToolCalls: make([]ToolCall, 0)}
	rawStream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":10}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"web_search","input":{}}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"query\":\"weather\"}"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	if err := consumeAnthropicStream(strings.NewReader(rawStream), result, nil, classifier); err != nil {
		t.Fatalf("consume anthropic stream: %v", err)
	}
	compactAnthropicStreamToolCalls(result)
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].ToolName != "web_search" {
		t.Fatalf("expected declared client tool call, got %#v", result.ToolCalls)
	}
	if len(result.ServerToolCalls) != 0 {
		t.Fatalf("expected no server-side tool calls, got %#v", result.ServerToolCalls)
	}
}

func TestAnthropicStreamServerToolUseIsNotLocalToolCall(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0), ServerToolCalls: make([]ToolCall, 0)}
	rawStream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":10}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"server_tool_use","id":"srv_1","name":"web_search","input":{}}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"query\":\"weather\"}"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	if err := consumeAnthropicStream(strings.NewReader(rawStream), result, nil, anthropicToolClassifier{}); err != nil {
		t.Fatalf("consume anthropic stream: %v", err)
	}
	compactAnthropicStreamToolCalls(result)
	if len(result.ToolCalls) != 0 {
		t.Fatalf("expected no local tool calls, got %#v", result.ToolCalls)
	}
	if len(result.ServerToolCalls) != 1 || result.ServerToolCalls[0].ToolName != "web_search" || result.ServerToolCalls[0].Status != "completed" {
		t.Fatalf("expected completed server-side tool call, got %#v", result.ServerToolCalls)
	}
}

func TestAnthropicStreamMergesServerToolResultIntoToolCall(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0), ServerToolCalls: make([]ToolCall, 0)}
	events := make([]ToolCall, 0)
	rawStream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":10}}}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"server_tool_use","id":"srv_1","name":"web_search","input":{}}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"query\":\"北京天气今天 2026\"}"}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`event: content_block_start`,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"web_search_tool_result","tool_use_id":"srv_1","content":[{"type":"web_search_result","title":"北京天气","url":"https://example.com/weather","page_age":"2026-05-24","encrypted_content":"redacted"}]}}`,
		``,
		`event: content_block_stop`,
		`data: {"type":"content_block_stop","index":1}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","usage":{"output_tokens":20,"server_tool_use":{"web_search_requests":1}}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	if err := consumeAnthropicStream(strings.NewReader(rawStream), result, func(event GenerateStreamEvent) error {
		if event.ServerToolCall != nil {
			events = append(events, *event.ServerToolCall)
		}
		return nil
	}, anthropicToolClassifier{}); err != nil {
		t.Fatalf("consume anthropic stream: %v", err)
	}
	compactAnthropicStreamToolCalls(result)
	if len(result.ToolCalls) != 0 {
		t.Fatalf("expected no local tool calls, got %#v", result.ToolCalls)
	}
	if len(result.ServerToolCalls) != 1 {
		t.Fatalf("expected one server-side tool call, got %#v", result.ServerToolCalls)
	}
	call := result.ServerToolCalls[0]
	if call.ToolName != "web_search" || call.Status != "completed" {
		t.Fatalf("expected completed web search call, got %#v", call)
	}
	if !strings.Contains(call.OutputJSON, "https://example.com/weather") || !strings.Contains(call.OutputJSON, "北京天气") {
		t.Fatalf("expected search result output to be captured, got %q", call.OutputJSON)
	}
	if strings.Contains(call.OutputJSON, "encrypted_content") || strings.Contains(call.OutputJSON, "redacted") {
		t.Fatalf("expected opaque search result fields to be removed, got %q", call.OutputJSON)
	}
	if result.ServerSideToolUsage["web_search"] != 1 {
		t.Fatalf("expected anthropic server-side tool usage, got %#v", result.ServerSideToolUsage)
	}
	if len(events) < 2 || events[len(events)-1].OutputJSON == "" {
		t.Fatalf("expected streaming result event with output, got %#v", events)
	}
}

func TestBuildGeminiToolParts(t *testing.T) {
	parts := buildGeminiParts(Message{
		Role:        "tool",
		ToolResults: []ToolResult{{ToolCallID: "call_1", ToolName: "memory.list", OutputJSON: `{"items":[]}`, Status: "success"}},
	})

	response := parts[0]["functionResponse"].(map[string]interface{})
	if response["name"] != "memory.list" {
		t.Fatalf("expected gemini functionResponse name, got %#v", response)
	}
}

func TestBuildGeminiToolCallPartsPreserveThoughtSignature(t *testing.T) {
	parts := buildGeminiParts(Message{
		Role: "assistant",
		ToolCalls: []ToolCall{{
			ToolName:         "search_web",
			ArgumentsJSON:    `{"query":"SpaceX stock price"}`,
			ThoughtSignature: "thought-signature-1",
		}},
	})

	if parts[0]["thoughtSignature"] != "thought-signature-1" {
		t.Fatalf("expected thoughtSignature on Gemini functionCall part, got %#v", parts[0])
	}
}
