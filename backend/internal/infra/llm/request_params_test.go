package llm

import "testing"

func TestBuildOpenAIChatCompletionsMinimalStreamRequestIncludesUsage(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIChatCompletions, "gpt-5", EndpointChatCompletions, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
	}, true)

	expectedKeys := map[string]struct{}{
		"model":          {},
		"messages":       {},
		"stream":         {},
		"stream_options": {},
	}
	assertOnlyPayloadKeys(t, payload, expectedKeys)
	streamOptions, ok := payload["stream_options"].(map[string]interface{})
	if !ok || streamOptions["include_usage"] != true {
		t.Fatalf("expected stream usage enabled by default, got %#v", payload["stream_options"])
	}
}

func TestBuildOpenAIResponsesMinimalRequestHasOnlyProtocolDefaults(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIResponses, "gpt-5", EndpointResponses, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
	}, true)

	expectedKeys := map[string]struct{}{
		"model":   {},
		"input":   {},
		"stream":  {},
		"include": {},
	}
	assertOnlyPayloadKeys(t, payload, expectedKeys)
	include, ok := payload["include"].([]string)
	if !ok || len(include) != 1 || include[0] != "reasoning.encrypted_content" {
		t.Fatalf("expected responses encrypted reasoning include only, got %#v", payload["include"])
	}
}

func TestBuildOpenAIResponsesUsesOutputTextForAssistantHistory(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIResponses, "gpt-5", EndpointResponses, GenerateInput{
		Messages: []Message{
			{Role: "system", Content: "follow the rules"},
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
			{
				Role: "assistant",
				Parts: []ContentPart{
					{Kind: ContentPartText, Text: "part answer"},
				},
			},
			{Role: "user", Content: "next"},
		},
	}, true)

	inputItems, ok := payload["input"].([]map[string]interface{})
	if !ok || len(inputItems) != 5 {
		t.Fatalf("expected responses input items, got %#v", payload["input"])
	}
	expectedTypes := []string{"input_text", "input_text", "output_text", "output_text", "input_text"}
	for i, expected := range expectedTypes {
		content, ok := inputItems[i]["content"].([]map[string]interface{})
		if !ok || len(content) != 1 {
			t.Fatalf("expected content item at %d, got %#v", i, inputItems[i]["content"])
		}
		if content[0]["type"] != expected {
			t.Fatalf("expected content type %q at %d, got %#v", expected, i, content[0]["type"])
		}
	}
}

func TestBuildOpenRouterResponsesAddsRequiredHistoryMessageFields(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenRouterResponses, "openai/gpt-oss-120b:free", EndpointResponses, GenerateInput{
		Messages: []Message{
			{Role: "user", Content: "你好"},
			{Role: "assistant", Content: "你好！有什么我可以帮忙的吗？"},
			{Role: "user", Content: "你是谁？"},
		},
	}, true)

	inputItems, ok := payload["input"].([]map[string]interface{})
	if !ok || len(inputItems) != 3 {
		t.Fatalf("expected three openrouter responses input items, got %#v", payload["input"])
	}
	if inputItems[0]["type"] != "message" || inputItems[0]["role"] != "user" {
		t.Fatalf("expected user message item, got %#v", inputItems[0])
	}
	assistant := inputItems[1]
	if assistant["type"] != "message" || assistant["role"] != "assistant" {
		t.Fatalf("expected assistant message item, got %#v", assistant)
	}
	if assistant["id"] == "" || assistant["status"] != "completed" {
		t.Fatalf("expected assistant id/status for openrouter history, got %#v", assistant)
	}
	content := assistant["content"].([]map[string]interface{})
	if content[0]["type"] != "output_text" {
		t.Fatalf("expected output_text content, got %#v", content[0])
	}
	if _, ok := payload["include"]; ok {
		t.Fatalf("expected no openai-only default include for openrouter responses, got %#v", payload["include"])
	}
}

func TestBuildOpenAIResponsesPlaylistHistoryMatchesOfficialContentTypes(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIResponses, "gpt-5.5", EndpointResponses, GenerateInput{
		Messages: []Message{
			{Role: "user", Content: "这是我最近听的歌，你根据文件告诉我不同歌手的占比"},
			{Role: "assistant", Content: "根据你文件里的 11 首歌，我按“歌手参与次数”统计。"},
			{Role: "user", Content: "<ctx>\n<files>\n<file name=\"My_FreeText_Playlist_Missing.csv\">Track name\tArtist name\n\t天后 (Live) - 薛之谦</file>\n</files>\n</ctx>\n\n<q>推荐一些流行的哥</q>"},
		},
		Options: map[string]interface{}{
			"include": []interface{}{"reasoning.encrypted_content"},
		},
	}, true)

	inputItems, ok := payload["input"].([]map[string]interface{})
	if !ok || len(inputItems) != 3 {
		t.Fatalf("expected three responses input items, got %#v", payload["input"])
	}
	expectedRoles := []string{"user", "assistant", "user"}
	expectedTypes := []string{"input_text", "output_text", "input_text"}
	for i := range expectedRoles {
		if inputItems[i]["role"] != expectedRoles[i] {
			t.Fatalf("expected role %q at %d, got %#v", expectedRoles[i], i, inputItems[i])
		}
		content, ok := inputItems[i]["content"].([]map[string]interface{})
		if !ok || len(content) != 1 {
			t.Fatalf("expected single content item at %d, got %#v", i, inputItems[i]["content"])
		}
		if content[0]["type"] != expectedTypes[i] {
			t.Fatalf("expected content type %q at %d, got %#v", expectedTypes[i], i, content[0])
		}
	}
}

func TestBuildAnthropicMinimalRequestHasOnlyRequiredDefaults(t *testing.T) {
	payload := mustBuildAnthropicRequestBody(t, "claude-sonnet-4", GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
	}, true)

	expectedKeys := map[string]struct{}{
		"model":         {},
		"max_tokens":    {},
		"messages":      {},
		"stream":        {},
		"cache_control": {},
	}
	assertOnlyPayloadKeys(t, payload, expectedKeys)
	if payload["max_tokens"] != anthropicDefaultMaxTokens {
		t.Fatalf("expected default max_tokens, got %#v", payload["max_tokens"])
	}
	cacheControl, ok := payload["cache_control"].(map[string]interface{})
	if !ok || cacheControl["type"] != "ephemeral" {
		t.Fatalf("expected default cache_control, got %#v", payload["cache_control"])
	}
}

func TestBuildGeminiMinimalRequestHasOnlyProtocolDefaults(t *testing.T) {
	payload := mustBuildGeminiRequestBody(t, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})

	expectedKeys := map[string]struct{}{
		"contents": {},
	}
	assertOnlyPayloadKeys(t, payload, expectedKeys)
}

func assertOnlyPayloadKeys(t *testing.T, payload map[string]interface{}, expected map[string]struct{}) {
	t.Helper()
	if len(payload) != len(expected) {
		t.Fatalf("expected only keys %v, got %#v", mapKeys(expected), payload)
	}
	for key := range expected {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected key %q in payload %#v", key, payload)
		}
	}
}

func mapKeys(items map[string]struct{}) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	return keys
}

func TestBuildOpenAIChatCompletionsRequestBodyPromptCacheRetention(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIChatCompletions, "gpt-5", EndpointChatCompletions, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"max_output_tokens":      2048,
			"prompt_cache_retention": "24h",
			"temperature":            0.2,
			"top_p":                  0.8,
			"frequency_penalty":      0.4,
			"presence_penalty":       0.3,
			"seed":                   1234,
			"stop":                   []interface{}{"END", "STOP"},
			"response_format":        "json",
		},
	}, false)

	if payload["max_completion_tokens"] != 2048 {
		t.Fatalf("expected max_completion_tokens, got %#v", payload["max_completion_tokens"])
	}
	if payload["temperature"] != 0.2 {
		t.Fatalf("expected temperature=0.2, got %#v", payload["temperature"])
	}
	if payload["top_p"] != 0.8 {
		t.Fatalf("expected top_p=0.8, got %#v", payload["top_p"])
	}
	if payload["frequency_penalty"] != 0.4 {
		t.Fatalf("expected frequency_penalty=0.4, got %#v", payload["frequency_penalty"])
	}
	if payload["presence_penalty"] != 0.3 {
		t.Fatalf("expected presence_penalty=0.3, got %#v", payload["presence_penalty"])
	}
	if payload["seed"] != 1234 {
		t.Fatalf("expected seed=1234, got %#v", payload["seed"])
	}
	if payload["prompt_cache_retention"] != "24h" {
		t.Fatalf("expected prompt_cache_retention=24h, got %#v", payload["prompt_cache_retention"])
	}
	stops, ok := payload["stop"].([]string)
	if !ok || len(stops) != 2 || stops[0] != "END" || stops[1] != "STOP" {
		t.Fatalf("expected stop sequence list, got %#v", payload["stop"])
	}
	responseFormat, ok := payload["response_format"].(map[string]string)
	if !ok || responseFormat["type"] != "json_object" {
		t.Fatalf("expected json_object response_format, got %#v", payload["response_format"])
	}
}

func TestBuildOpenAIChatCompletionsProviderOptionsMergeAndProtectSystemFields(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIChatCompletions, "gpt-5", EndpointChatCompletions, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"service_tier": "priority",
			"metadata": map[string]interface{}{
				"tenant": "deeix-chat",
			},
			"model":          "attacker-model",
			"messages":       []interface{}{},
			"stream":         false,
			"stream_options": map[string]interface{}{"include_usage": false},
		},
	}, true)

	if payload["model"] != "gpt-5" {
		t.Fatalf("expected protected model, got %#v", payload["model"])
	}
	if payload["stream"] != true {
		t.Fatalf("expected protected stream=true, got %#v", payload["stream"])
	}
	streamOptions, ok := payload["stream_options"].(map[string]interface{})
	if !ok || streamOptions["include_usage"] != false {
		t.Fatalf("expected provider stream_options, got %#v", payload["stream_options"])
	}
	if payload["service_tier"] != "priority" {
		t.Fatalf("expected provider option service_tier, got %#v", payload["service_tier"])
	}
	metadata, ok := payload["metadata"].(map[string]interface{})
	if !ok || metadata["tenant"] != "deeix-chat" {
		t.Fatalf("expected provider metadata merge, got %#v", payload["metadata"])
	}
}

func TestBuildOpenAIChatCompletionsStructuredOutputsAndVerbosity(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIChatCompletions, "gpt-5.1", EndpointChatCompletions, GenerateInput{
		Messages: []Message{{Role: "developer", Content: "Return valid JSON."}, {Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"verbosity": "low",
			"response_format": map[string]interface{}{
				"type":   "json_schema",
				"name":   "answer",
				"schema": map[string]interface{}{"type": "object"},
				"strict": true,
			},
		},
	}, false)

	if payload["verbosity"] != "low" {
		t.Fatalf("expected chat verbosity=low, got %#v", payload["verbosity"])
	}
	messages := payload["messages"].([]map[string]interface{})
	if messages[0]["role"] != "developer" {
		t.Fatalf("expected developer role to be preserved, got %#v", messages[0])
	}
	format, ok := payload["response_format"].(map[string]interface{})
	if !ok || format["type"] != "json_schema" {
		t.Fatalf("expected chat response_format json_schema, got %#v", payload["response_format"])
	}
	jsonSchema, ok := format["json_schema"].(map[string]interface{})
	if !ok || jsonSchema["name"] != "answer" || jsonSchema["strict"] != true {
		t.Fatalf("expected chat json_schema wrapper, got %#v", format["json_schema"])
	}
	if _, ok := format["schema"]; ok {
		t.Fatalf("expected schema under json_schema wrapper, got %#v", format)
	}
}

func TestBuildOpenAIChatCompletionsNativeToolOptions(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIChatCompletions, "mimo-v2.5-pro", EndpointChatCompletions, GenerateInput{
		Messages: []Message{{Role: "user", Content: "please introduce Jun Lei"}},
		Options: map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"type":         "web_search",
					"max_keyword":  float64(3),
					"force_search": true,
					"limit":        float64(1),
					"user_location": map[string]interface{}{
						"type":    "approximate",
						"country": "China",
						"region":  "Hubei",
						"city":    "Wuhan",
					},
				},
			},
			"max_completion_tokens": float64(1024),
			"temperature":           float64(1),
			"top_p":                 float64(0.95),
			"stop":                  nil,
			"frequency_penalty":     float64(0),
			"presence_penalty":      float64(0),
			"thinking": map[string]interface{}{
				"type": "disabled",
			},
		},
	}, true)

	if payload["max_completion_tokens"] != 1024 {
		t.Fatalf("expected max_completion_tokens=1024, got %#v", payload["max_completion_tokens"])
	}
	if payload["stop"] != nil {
		t.Fatalf("expected stop=null, got %#v", payload["stop"])
	}
	streamOptions, ok := payload["stream_options"].(map[string]interface{})
	if !ok || streamOptions["include_usage"] != true {
		t.Fatalf("expected stream usage enabled by default, got %#v", payload["stream_options"])
	}
	thinking, ok := payload["thinking"].(map[string]interface{})
	if !ok || thinking["type"] != "disabled" {
		t.Fatalf("expected native thinking config, got %#v", payload["thinking"])
	}
	tools, ok := payload["tools"].([]map[string]interface{})
	if !ok || len(tools) != 1 || tools[0]["type"] != "web_search" {
		t.Fatalf("expected native web_search tool, got %#v", payload["tools"])
	}
}

func TestBuildOpenAIResponsesRequestBodyWebSearchAndPromptCacheRetention(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIResponses, "gpt-5", EndpointResponses, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"max_output_tokens":      2048,
			"prompt_cache_retention": "in-memory",
			"temperature":            0.3,
			"top_p":                  0.7,
			"verbosity":              "high",
			"response_format":        "json",
			"web_search":             true,
		},
	}, false)

	if payload["max_output_tokens"] != 2048 {
		t.Fatalf("expected max_output_tokens, got %#v", payload["max_output_tokens"])
	}
	if payload["temperature"] != 0.3 {
		t.Fatalf("expected temperature=0.3, got %#v", payload["temperature"])
	}
	if payload["top_p"] != 0.7 {
		t.Fatalf("expected top_p=0.7, got %#v", payload["top_p"])
	}
	text, ok := payload["text"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected text config, got %#v", payload["text"])
	}
	if text["verbosity"] != "high" {
		t.Fatalf("expected verbosity=high, got %#v", text["verbosity"])
	}
	format, ok := text["format"].(map[string]string)
	if !ok || format["type"] != "json_object" {
		t.Fatalf("expected text.format=json_object, got %#v", text["format"])
	}
	tools, ok := payload["tools"].([]map[string]interface{})
	if !ok || len(tools) != 1 || tools[0]["type"] != "web_search" {
		t.Fatalf("expected web_search tool, got %#v", payload["tools"])
	}
	include, ok := payload["include"].([]string)
	if !ok || len(include) != 2 || include[0] != "reasoning.encrypted_content" || include[1] != "web_search_call.action.sources" {
		t.Fatalf("expected web search sources include, got %#v", payload["include"])
	}
	if payload["prompt_cache_retention"] != "in-memory" {
		t.Fatalf("expected prompt_cache_retention=in-memory, got %#v", payload["prompt_cache_retention"])
	}
}

func TestBuildOpenAIResponsesRequestBodyMergesIncludeReasoningAndStructuredText(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIResponses, "gpt-5", EndpointResponses, GenerateInput{
		Messages: []Message{{Role: "user", Content: "return json"}},
		Options: map[string]interface{}{
			"include":           []interface{}{"file_search_call.results"},
			"reasoning_effort":  "medium",
			"reasoning_summary": "auto",
			"response_format": map[string]interface{}{
				"type":   "json_schema",
				"name":   "answer",
				"schema": map[string]interface{}{"type": "object"},
				"strict": true,
			},
		},
	}, true)

	include, ok := payload["include"].([]string)
	if !ok || len(include) != 2 || include[0] != "reasoning.encrypted_content" || include[1] != "file_search_call.results" {
		t.Fatalf("expected default and configured include values, got %#v", payload["include"])
	}
	reasoning, ok := payload["reasoning"].(map[string]interface{})
	if !ok || reasoning["effort"] != "medium" || reasoning["summary"] != "auto" {
		t.Fatalf("expected reasoning effort and summary, got %#v", payload["reasoning"])
	}
	text, ok := payload["text"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected text config, got %#v", payload["text"])
	}
	format, ok := text["format"].(map[string]interface{})
	if !ok || format["type"] != "json_schema" || format["name"] != "answer" || format["strict"] != true {
		t.Fatalf("expected response_format mapped to text.format, got %#v", text["format"])
	}
	if _, ok := payload["response_format"]; ok {
		t.Fatalf("expected responses response_format to stay under text.format, got %#v", payload["response_format"])
	}
}

func TestBuildOpenAIResponsesNestedProviderOptionsMergeAndOfficialFields(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterOpenAIResponses, "gpt-5", EndpointResponses, GenerateInput{
		RequestID:      "req-123",
		ConversationID: 42,
		Messages:       []Message{{Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"verbosity": "low",
			"text": map[string]interface{}{
				"format": map[string]interface{}{"type": "json_schema"},
			},
			"metadata": map[string]interface{}{
				"user_tag": "debug",
			},
			"stream_options": map[string]interface{}{
				"include_usage":       true,
				"include_obfuscation": true,
			},
			"input":        []interface{}{},
			"instructions": "official developer instructions",
			"prompt": map[string]interface{}{
				"id": "pmpt_123",
			},
		},
	}, true)

	text, ok := payload["text"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected text config, got %#v", payload["text"])
	}
	if text["verbosity"] != "low" {
		t.Fatalf("expected normalized verbosity to remain, got %#v", text)
	}
	format, ok := text["format"].(map[string]interface{})
	if !ok || format["type"] != "json_schema" {
		t.Fatalf("expected nested text.format merge, got %#v", text["format"])
	}
	metadata, ok := payload["metadata"].(map[string]interface{})
	if !ok || metadata["user_tag"] != "debug" {
		t.Fatalf("expected official metadata to pass through, got %#v", payload["metadata"])
	}
	if _, ok := payload["input"].([]map[string]interface{}); !ok {
		t.Fatalf("expected protected input messages, got %#v", payload["input"])
	}
	if payload["instructions"] != "official developer instructions" {
		t.Fatalf("expected official instructions to pass through, got %#v", payload["instructions"])
	}
	prompt, ok := payload["prompt"].(map[string]interface{})
	if !ok || prompt["id"] != "pmpt_123" {
		t.Fatalf("expected official prompt to pass through, got %#v", payload["prompt"])
	}
	streamOptions, ok := payload["stream_options"].(map[string]interface{})
	if !ok || streamOptions["include_obfuscation"] != true {
		t.Fatalf("expected official responses stream_options, got %#v", payload["stream_options"])
	}
	if _, ok := streamOptions["include_usage"]; ok {
		t.Fatalf("expected chat-only include_usage to be omitted for responses, got %#v", streamOptions)
	}
}

func TestBuildXAIResponsesOmitsUnsupportedSystemMetadata(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterXAIResponses, "grok-4.3", EndpointResponses, GenerateInput{
		RequestID:      "req-123",
		ConversationID: 42,
		Messages:       []Message{{Role: "user", Content: "上海今天的天气"}},
		Options: map[string]interface{}{
			"metadata":     map[string]interface{}{"user_tag": "debug"},
			"instructions": "custom instructions",
			"prompt":       map[string]interface{}{"id": "pmpt_123"},
			"tools": []interface{}{
				map[string]interface{}{"type": "x_search"},
			},
		},
	}, true)

	if _, ok := payload["metadata"]; ok {
		t.Fatalf("expected xAI responses payload to omit unsupported metadata, got %#v", payload["metadata"])
	}
	if _, ok := payload["instructions"]; ok {
		t.Fatalf("expected xAI responses payload to omit unsupported instructions, got %#v", payload["instructions"])
	}
	if _, ok := payload["prompt"]; ok {
		t.Fatalf("expected xAI responses payload to omit unsupported prompt, got %#v", payload["prompt"])
	}
	include, ok := payload["include"].([]string)
	if !ok || len(include) != 1 || include[0] != "reasoning.encrypted_content" {
		t.Fatalf("expected xAI responses encrypted reasoning include, got %#v", payload["include"])
	}
	tools, ok := payload["tools"].([]map[string]interface{})
	if !ok || len(tools) != 1 || tools[0]["type"] != "x_search" {
		t.Fatalf("expected x_search tool to be preserved, got %#v", payload["tools"])
	}
}

func TestBuildXAIResponsesWebSearchRequestHasNoExtraIncludes(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterXAIResponses, "grok-4.3", EndpointResponses, GenerateInput{
		Messages: []Message{{Role: "user", Content: "今日新闻？"}},
		Options: map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{"type": "web_search"},
			},
		},
	}, true)

	include, ok := payload["include"].([]string)
	if !ok {
		t.Fatalf("expected include list, got %#v", payload["include"])
	}
	expectedInclude := []string{
		"reasoning.encrypted_content",
		"web_search_call.action.sources",
	}
	if len(include) != len(expectedInclude) {
		t.Fatalf("expected include %v, got %#v", expectedInclude, include)
	}
	for index, value := range expectedInclude {
		if include[index] != value {
			t.Fatalf("expected include[%d]=%q, got %#v", index, value, include)
		}
	}
	for _, key := range []string{"metadata", "reasoning", "text", "stream_options", "prompt_cache_retention", "temperature", "top_p", "max_output_tokens"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("expected no auto-added %s, got %#v", key, payload[key])
		}
	}
}

func TestBuildXAIResponsesIncludesSupportedNativeToolOutputs(t *testing.T) {
	payload := mustBuildRequestBody(t, AdapterXAIResponses, "grok-4.3", EndpointResponses, GenerateInput{
		Messages: []Message{{Role: "user", Content: "搜索新闻"}},
		Options: map[string]interface{}{
			"include": []interface{}{"custom.include"},
			"tools": []interface{}{
				map[string]interface{}{"type": "x_search"},
				map[string]interface{}{"type": "web_search"},
				map[string]interface{}{"type": "code_interpreter"},
			},
		},
	}, true)

	include, ok := payload["include"].([]string)
	if !ok {
		t.Fatalf("expected include list, got %#v", payload["include"])
	}
	tools, ok := payload["tools"].([]map[string]interface{})
	if !ok || len(tools) != 3 || tools[0]["type"] != "x_search" || tools[1]["type"] != "web_search" || tools[2]["type"] != "code_interpreter" {
		t.Fatalf("expected xAI native tools to be preserved, got %#v", payload["tools"])
	}
	expected := []string{
		"reasoning.encrypted_content",
		"web_search_call.action.sources",
		"code_interpreter_call.outputs",
		"custom.include",
	}
	if len(include) != len(expected) {
		t.Fatalf("expected include values %v, got %#v", expected, include)
	}
	for index, value := range expected {
		if include[index] != value {
			t.Fatalf("expected include[%d]=%q, got %#v", index, value, include)
		}
	}
}

func TestBuildAnthropicRequestBodyWebSearchAndPromptCache(t *testing.T) {
	longSystem := make([]byte, 4096)
	for i := range longSystem {
		longSystem[i] = 'a'
	}
	payload := mustBuildAnthropicRequestBody(t, "claude-sonnet-4", GenerateInput{
		Messages: []Message{
			{Role: "system", Content: string(longSystem)},
			{Role: "user", Content: "hello"},
		},
		Options: map[string]interface{}{
			"max_output_tokens": 4096,
			"prompt_cache":      true,
			"temperature":       0.5,
			"top_p":             0.9,
			"top_k":             64,
			"stop":              "END",
			"web_search":        true,
		},
	}, false)

	if payload["max_tokens"] != 4096 {
		t.Fatalf("expected max_tokens=4096, got %#v", payload["max_tokens"])
	}
	if payload["temperature"] != 0.5 {
		t.Fatalf("expected temperature=0.5, got %#v", payload["temperature"])
	}
	if payload["top_p"] != 0.9 {
		t.Fatalf("expected top_p=0.9, got %#v", payload["top_p"])
	}
	if payload["top_k"] != 64 {
		t.Fatalf("expected top_k=64, got %#v", payload["top_k"])
	}
	stops, ok := payload["stop_sequences"].([]string)
	if !ok || len(stops) != 1 || stops[0] != "END" {
		t.Fatalf("expected stop_sequences, got %#v", payload["stop_sequences"])
	}
	tools, ok := payload["tools"].([]map[string]interface{})
	if !ok || len(tools) != 1 || tools[0]["type"] != "web_search_20250305" {
		t.Fatalf("expected anthropic web search tool, got %#v", payload["tools"])
	}
	if tools[0]["name"] != "web_search" {
		t.Fatalf("expected anthropic web search tool name, got %#v", tools[0])
	}
	if _, ok := tools[0]["allowed_callers"]; ok {
		t.Fatalf("expected legacy anthropic web search to avoid direct caller override, got %#v", tools[0])
	}
	system, ok := payload["system"].(string)
	if !ok || len(system) != len(longSystem) {
		t.Fatalf("expected plain system string, got %#v", payload["system"])
	}
	cacheControl, ok := payload["cache_control"].(map[string]interface{})
	if !ok || cacheControl["type"] != "ephemeral" {
		t.Fatalf("expected top-level cache_control, got %#v", payload["cache_control"])
	}
}

func TestBuildAnthropicRequestBodyPromptCacheDisabled(t *testing.T) {
	longSystem := make([]byte, 4096)
	for i := range longSystem {
		longSystem[i] = 'a'
	}
	payload := mustBuildAnthropicRequestBody(t, "claude-sonnet-4", GenerateInput{
		Messages: []Message{
			{Role: "system", Content: string(longSystem)},
			{Role: "user", Content: "hello"},
		},
		Options: map[string]interface{}{
			"prompt_cache": false,
		},
	}, false)

	system, ok := payload["system"].(string)
	if !ok || len(system) != len(longSystem) {
		t.Fatalf("expected plain system string when prompt cache disabled, got %#v", payload["system"])
	}
	if _, ok := payload["cache_control"]; ok {
		t.Fatalf("expected cache_control omitted when prompt cache disabled, got %#v", payload["cache_control"])
	}
}

func TestBuildAnthropicRequestBodyFastMode(t *testing.T) {
	payload := mustBuildAnthropicRequestBody(t, "claude-opus-4-6", GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"speed": "fast",
		},
	}, false)

	if payload["speed"] != "fast" {
		t.Fatalf("expected speed=fast, got %#v", payload["speed"])
	}
}

func TestBuildAnthropicRequestBodyPromptCacheEnabledByDefault(t *testing.T) {
	payload := mustBuildAnthropicRequestBody(t, "claude-sonnet-4", GenerateInput{
		Messages: []Message{
			{Role: "system", Content: "short system"},
			{Role: "user", Content: "hello"},
		},
	}, false)

	cacheControl, ok := payload["cache_control"].(map[string]interface{})
	if !ok || cacheControl["type"] != "ephemeral" {
		t.Fatalf("expected default top-level cache_control, got %#v", payload["cache_control"])
	}
}

func TestBuildAnthropicRequestBodySystemBlockCacheControl(t *testing.T) {
	payload := mustBuildAnthropicRequestBody(t, "claude-sonnet-4", GenerateInput{
		Messages: []Message{
			{Role: "system", Content: "stable file context", CacheControl: &CacheControl{Type: "ephemeral"}},
			{Role: "user", Content: "<ctx><rag>dynamic</rag></ctx><q>hello</q>"},
		},
		Options: map[string]interface{}{
			"cache_timeout": "1h",
		},
	}, false)

	system, ok := payload["system"].([]map[string]interface{})
	if !ok || len(system) != 1 {
		t.Fatalf("expected system blocks, got %#v", payload["system"])
	}
	cacheControl, ok := system[0]["cache_control"].(map[string]interface{})
	if !ok || cacheControl["type"] != "ephemeral" || cacheControl["ttl"] != "1h" {
		t.Fatalf("expected system block cache_control ttl=1h, got %#v", system[0])
	}
	if _, ok := payload["cache_control"]; ok {
		t.Fatalf("expected top-level cache_control omitted with explicit block hint, got %#v", payload["cache_control"])
	}
}

func TestBuildAnthropicRequestBodyOpenWebUIAliases(t *testing.T) {
	longSystem := make([]byte, 4096)
	for i := range longSystem {
		longSystem[i] = 'a'
	}
	payload := mustBuildAnthropicRequestBody(t, "claude-sonnet-4-6", GenerateInput{
		Messages: []Message{
			{Role: "system", Content: string(longSystem)},
			{Role: "user", Content: "hello"},
		},
		Options: map[string]interface{}{
			"max_tokens":       64000,
			"enable_thinking":  true,
			"thinking_display": "omitted",
			"effort":           "xhigh",
			"enable_cache":     true,
			"cache_timeout":    "1h",
		},
	}, false)

	if payload["max_tokens"] != 64000 {
		t.Fatalf("expected max_tokens=64000, got %#v", payload["max_tokens"])
	}
	thinking, ok := payload["thinking"].(map[string]interface{})
	if !ok || thinking["type"] != "adaptive" || thinking["display"] != "omitted" {
		t.Fatalf("expected alias thinking config, got %#v", payload["thinking"])
	}
	outputConfig, ok := payload["output_config"].(map[string]interface{})
	if !ok || outputConfig["effort"] != "xhigh" {
		t.Fatalf("expected output_config.effort from alias, got %#v", payload["output_config"])
	}
	system, ok := payload["system"].(string)
	if !ok || len(system) != len(longSystem) {
		t.Fatalf("expected plain system string, got %#v", payload["system"])
	}
	cacheControl, ok := payload["cache_control"].(map[string]interface{})
	if !ok || cacheControl["type"] != "ephemeral" || cacheControl["ttl"] != "1h" {
		t.Fatalf("expected top-level cache_control ttl=1h, got %#v", payload["cache_control"])
	}
}

func TestBuildAnthropicRequestBodyAliasOverridesOfficialThinking(t *testing.T) {
	payload := mustBuildAnthropicRequestBody(t, "claude-sonnet-4-6", GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"thinking":         map[string]interface{}{"type": "disabled", "display": "summarized"},
			"enable_thinking":  true,
			"thinking_display": "omitted",
			"output_config":    map[string]interface{}{"effort": "high"},
			"effort":           "max",
		},
	}, false)

	thinking, ok := payload["thinking"].(map[string]interface{})
	if !ok || thinking["type"] != "adaptive" || thinking["display"] != "omitted" {
		t.Fatalf("expected alias to override thinking type/display, got %#v", payload["thinking"])
	}
	outputConfig, ok := payload["output_config"].(map[string]interface{})
	if !ok || outputConfig["effort"] != "high" {
		t.Fatalf("expected explicit output_config.effort to win, got %#v", payload["output_config"])
	}
}

func TestBuildAnthropicRequestBodyStructuredOutputThinkingAndToolChoice(t *testing.T) {
	payload := mustBuildAnthropicRequestBody(t, "claude-sonnet-4-5", GenerateInput{
		Messages: []Message{{Role: "user", Content: "return json"}},
		Options: map[string]interface{}{
			"max_output_tokens": 4096,
			"thinking": map[string]interface{}{
				"type":          "enabled",
				"budget_tokens": float64(2048),
			},
			"tool_choice":     "memory.save",
			"response_format": map[string]interface{}{"type": "json_object"},
		},
	}, false)

	thinking, ok := payload["thinking"].(map[string]interface{})
	if !ok || thinking["type"] != "enabled" || thinking["budget_tokens"] != float64(2048) {
		t.Fatalf("expected Anthropic thinking object, got %#v", payload["thinking"])
	}
	toolChoice, ok := payload["tool_choice"].(map[string]interface{})
	if !ok || toolChoice["type"] != "tool" || toolChoice["name"] != "memory.save" {
		t.Fatalf("expected Anthropic named tool_choice, got %#v", payload["tool_choice"])
	}
	outputConfig, ok := payload["output_config"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected output_config, got %#v", payload["output_config"])
	}
	format, ok := outputConfig["format"].(map[string]interface{})
	if !ok || format["type"] != "json_object" {
		t.Fatalf("expected response_format mapped to output_config.format, got %#v", payload["output_config"])
	}
	if _, ok := payload["response_format"]; ok {
		t.Fatalf("expected response_format not to be sent as top-level Anthropic param, got %#v", payload["response_format"])
	}
}

func TestBuildGeminiRequestBodyWebSearch(t *testing.T) {
	payload := mustBuildGeminiRequestBody(t, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"max_output_tokens": 2048,
			"temperature":       0.4,
			"top_p":             0.6,
			"top_k":             32,
			"stop":              []string{"END"},
			"response_format":   "json",
			"web_search":        true,
		},
	})

	generationConfig, ok := payload["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected generationConfig, got %#v", payload["generationConfig"])
	}
	if generationConfig["maxOutputTokens"] != 2048 {
		t.Fatalf("expected maxOutputTokens=2048, got %#v", generationConfig["maxOutputTokens"])
	}
	if generationConfig["temperature"] != 0.4 {
		t.Fatalf("expected temperature=0.4, got %#v", generationConfig["temperature"])
	}
	if generationConfig["topP"] != 0.6 {
		t.Fatalf("expected topP=0.6, got %#v", generationConfig["topP"])
	}
	if generationConfig["topK"] != 32 {
		t.Fatalf("expected topK=32, got %#v", generationConfig["topK"])
	}
	stops, ok := generationConfig["stopSequences"].([]string)
	if !ok || len(stops) != 1 || stops[0] != "END" {
		t.Fatalf("expected stopSequences, got %#v", generationConfig["stopSequences"])
	}
	if generationConfig["responseMimeType"] != "application/json" {
		t.Fatalf("expected responseMimeType=application/json, got %#v", generationConfig["responseMimeType"])
	}
	tools, ok := payload["tools"].([]map[string]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("expected gemini tools, got %#v", payload["tools"])
	}
	if _, ok := tools[0]["google_search"]; !ok {
		t.Fatalf("expected google_search tool, got %#v", tools[0])
	}
}

func TestBuildGeminiRequestBodyStructuredOutputAndGenerationConfig(t *testing.T) {
	payload := mustBuildGeminiRequestBody(t, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"max_completion_tokens": 1024,
			"candidate_count":       2,
			"presence_penalty":      0.3,
			"frequency_penalty":     0.4,
			"seed":                  42,
			"response_logprobs":     true,
			"logprobs":              3,
			"response_modalities":   []interface{}{"TEXT"},
			"response_format": map[string]interface{}{
				"type": "json_schema",
				"name": "answer",
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"answer": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"answer"},
				},
			},
		},
	})

	generationConfig, ok := payload["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected generationConfig, got %#v", payload["generationConfig"])
	}
	if generationConfig["maxOutputTokens"] != 1024 || generationConfig["candidateCount"] != 2 {
		t.Fatalf("expected maxOutputTokens/candidateCount, got %#v", generationConfig)
	}
	if generationConfig["presencePenalty"] != 0.3 || generationConfig["frequencyPenalty"] != 0.4 {
		t.Fatalf("expected penalty params, got %#v", generationConfig)
	}
	if generationConfig["seed"] != 42 || generationConfig["responseLogprobs"] != true || generationConfig["logprobs"] != 3 {
		t.Fatalf("expected seed/logprobs params, got %#v", generationConfig)
	}
	modalities, ok := generationConfig["responseModalities"].([]string)
	if !ok || len(modalities) != 1 || modalities[0] != "TEXT" {
		t.Fatalf("expected responseModalities, got %#v", generationConfig["responseModalities"])
	}
	if generationConfig["responseMimeType"] != "application/json" {
		t.Fatalf("expected responseMimeType=application/json, got %#v", generationConfig["responseMimeType"])
	}
	schema, ok := generationConfig["responseSchema"].(map[string]interface{})
	if !ok || schema["type"] != "object" {
		t.Fatalf("expected responseSchema object, got %#v", generationConfig["responseSchema"])
	}
	if _, ok := payload["response_format"]; ok {
		t.Fatalf("expected response_format not to leak to Gemini root, got %#v", payload["response_format"])
	}
	for _, key := range []string{"candidate_count", "presence_penalty", "frequency_penalty", "response_logprobs", "response_modalities", "logprobs"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("expected normalized Gemini option %q not to leak to root, got %#v", key, payload[key])
		}
	}
}

func TestBuildGeminiRequestBodyMapsNativeGenerationConfigAliases(t *testing.T) {
	payload := mustBuildGeminiRequestBody(t, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"maxOutputTokens":  2048,
			"topP":             0.8,
			"topK":             64,
			"stopSequences":    []interface{}{"DONE"},
			"responseMimeType": "text/plain",
			"thinkingConfig": map[string]interface{}{
				"includeThoughts": true,
				"thinkingBudget":  256,
			},
		},
	})

	generationConfig, ok := payload["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected generationConfig, got %#v", payload["generationConfig"])
	}
	if generationConfig["maxOutputTokens"] != 2048 || generationConfig["topP"] != 0.8 || generationConfig["topK"] != 64 {
		t.Fatalf("expected native generationConfig aliases mapped, got %#v", generationConfig)
	}
	stops, ok := generationConfig["stopSequences"].([]string)
	if !ok || len(stops) != 1 || stops[0] != "DONE" {
		t.Fatalf("expected stopSequences mapped, got %#v", generationConfig["stopSequences"])
	}
	if generationConfig["responseMimeType"] != "text/plain" {
		t.Fatalf("expected responseMimeType mapped, got %#v", generationConfig["responseMimeType"])
	}
	thinkingConfig, ok := generationConfig["thinkingConfig"].(map[string]interface{})
	if !ok || thinkingConfig["includeThoughts"] != true || thinkingConfig["thinkingBudget"] != 256 {
		t.Fatalf("expected thinkingConfig mapped, got %#v", generationConfig["thinkingConfig"])
	}
	for _, key := range []string{"maxOutputTokens", "topP", "topK", "stopSequences", "responseMimeType", "thinkingConfig"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("expected native Gemini option %q not to leak to root, got %#v", key, payload[key])
		}
	}
}

func TestBuildGeminiRequestBodyRootAliasesAndThinkingConfig(t *testing.T) {
	payload := mustBuildGeminiRequestBody(t, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"tool_config": map[string]interface{}{
				"functionCallingConfig": map[string]interface{}{"mode": "ANY"},
			},
			"safety_settings": []interface{}{
				map[string]interface{}{"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_ONLY_HIGH"},
			},
			"cached_content": "cachedContents/abc",
			"service_tier":   "flex",
			"thinking": map[string]interface{}{
				"include_thoughts": true,
				"thinking_budget":  512,
			},
			"thinking_level": "low",
		},
	})

	if _, ok := payload["tool_config"]; ok {
		t.Fatalf("expected tool_config not to leak, got %#v", payload["tool_config"])
	}
	toolConfig, ok := payload["toolConfig"].(map[string]interface{})
	if !ok || len(toolConfig) == 0 {
		t.Fatalf("expected toolConfig, got %#v", payload["toolConfig"])
	}
	if payload["cachedContent"] != "cachedContents/abc" || payload["serviceTier"] != "flex" {
		t.Fatalf("expected cachedContent/serviceTier, got %#v", payload)
	}
	if _, ok := payload["safetySettings"].([]interface{}); !ok {
		t.Fatalf("expected safetySettings passthrough, got %#v", payload["safetySettings"])
	}
	generationConfig := payload["generationConfig"].(map[string]interface{})
	thinkingConfig, ok := generationConfig["thinkingConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected thinkingConfig, got %#v", generationConfig["thinkingConfig"])
	}
	if thinkingConfig["includeThoughts"] != true || thinkingConfig["thinkingBudget"] != 512 || thinkingConfig["thinkingLevel"] != "low" {
		t.Fatalf("expected normalized thinkingConfig, got %#v", thinkingConfig)
	}
}

func TestBuildGeminiRequestBodyAllowsNestedGenerationConfig(t *testing.T) {
	payload := mustBuildGeminiRequestBody(t, GenerateInput{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Options: map[string]interface{}{
			"generationConfig": map[string]interface{}{
				"candidateCount": float64(2),
			},
			"contents":          []interface{}{},
			"model":             "attacker-model",
			"prompt":            "override prompt",
			"systemInstruction": map[string]interface{}{"parts": []map[string]string{{"text": "override"}}},
		},
	})

	generationConfig, ok := payload["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected generationConfig, got %#v", payload["generationConfig"])
	}
	if generationConfig["candidateCount"] != float64(2) {
		t.Fatalf("expected candidateCount merge, got %#v", generationConfig["candidateCount"])
	}
	contents, ok := payload["contents"].([]map[string]interface{})
	if !ok || len(contents) != 1 {
		t.Fatalf("expected protected contents, got %#v", payload["contents"])
	}
	if _, ok := payload["model"]; ok {
		t.Fatalf("expected protected model to be omitted, got %#v", payload["model"])
	}
	if _, ok := payload["prompt"]; ok {
		t.Fatalf("expected protected prompt to be omitted, got %#v", payload["prompt"])
	}
	if _, ok := payload["systemInstruction"]; ok {
		t.Fatalf("expected protected systemInstruction to be omitted, got %#v", payload["systemInstruction"])
	}
}
