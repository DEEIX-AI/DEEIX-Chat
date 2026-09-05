package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestGeminiInteractionsAdapterDefaults(t *testing.T) {
	if got := portllm.DefaultEndpointForAdapter(portllm.AdapterGeminiInteractions); got != portllm.EndpointInteractions {
		t.Fatalf("expected interactions endpoint, got %q", got)
	}
	if !portllm.IsVideoGenerationAdapter(portllm.AdapterGeminiInteractions) {
		t.Fatal("expected Gemini Interactions to be a video generation adapter")
	}
	if !portllm.SupportsStreamingAdapter(portllm.AdapterGeminiInteractions) {
		t.Fatal("Gemini Interactions should support official streaming")
	}
	if !portllm.IsImageGenerationAdapter(portllm.AdapterGeminiInteractions) {
		t.Fatal("Gemini Interactions should support image generation")
	}
	if !portllm.IsImageEditAdapter(portllm.AdapterGeminiInteractions) {
		t.Fatal("Gemini Interactions should support image editing")
	}
}

func TestBuildGeminiInteractionRequestBody(t *testing.T) {
	payload, err := buildGeminiInteractionRequestBody(portllm.RouteConfig{
		Endpoint:      portllm.EndpointInteractions,
		UpstreamModel: "gemini-omni-flash-preview",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{
			Role: "user",
			Parts: []portllm.ContentPart{
				{Kind: portllm.ContentPartText, Text: "A short cinematic product video"},
				{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("image-bytes")},
			},
		}},
		Options: map[string]any{
			"response_format": map[string]any{"type": "video", "aspect_ratio": "16:9", "delivery": "b64_json"},
			"generation_config": map[string]any{
				"video_config": map[string]any{"task": "IMAGE_TO_VIDEO"},
			},
			"input": "override",
		},
	})
	if err != nil {
		t.Fatalf("build Gemini interaction request body: %v", err)
	}
	if payload["model"] != "gemini-omni-flash-preview" {
		t.Fatalf("unexpected model: %#v", payload)
	}
	responseFormat, ok := payload["response_format"].(map[string]any)
	if !ok || responseFormat["type"] != "video" {
		t.Fatalf("expected video response format, got %#v", payload["response_format"])
	}
	if responseFormat["delivery"] != "uri" {
		t.Fatalf("expected video delivery to use URI downloads, got %#v", payload["response_format"])
	}
	if responseFormat["aspect_ratio"] != "16:9" {
		t.Fatalf("expected response_format aspect ratio, got %#v", payload["response_format"])
	}
	config, ok := payload["generation_config"].(map[string]any)
	if !ok {
		t.Fatalf("expected generation config, got %#v", payload["generation_config"])
	}
	videoConfig, ok := config["video_config"].(map[string]any)
	if !ok || videoConfig["task"] != "image_to_video" {
		t.Fatalf("expected video config task, got %#v", payload["generation_config"])
	}
	input, ok := payload["input"].([]map[string]any)
	if !ok || len(input) != 2 {
		t.Fatalf("expected text and image input parts, got %#v", payload["input"])
	}
	if input[0]["type"] != "text" || input[0]["text"] != "A short cinematic product video" {
		t.Fatalf("unexpected text part: %#v", input[0])
	}
	if input[1]["type"] != "image" || input[1]["mime_type"] != "image/png" || input[1]["data"] == "" {
		t.Fatalf("unexpected image part: %#v", input[1])
	}
}

func TestBuildGeminiInteractionRequestBodyRequiresModel(t *testing.T) {
	_, err := buildGeminiInteractionRequestBody(portllm.RouteConfig{
		Endpoint: portllm.EndpointInteractions,
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "Hello"}},
	})
	if err == nil {
		t.Fatal("expected empty Interactions model to be rejected")
	}
}

func TestBuildGeminiInteractionRequestBodySupportsUniversalOptionsAndTools(t *testing.T) {
	payload, err := buildGeminiInteractionRequestBody(portllm.RouteConfig{
		Endpoint:      portllm.EndpointInteractions,
		UpstreamModel: "gemini-3-flash-preview",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{
			{Role: "user", Content: "Create a short answer and an image."},
			{
				Role:    "assistant",
				Content: "I need the weather first.",
				ToolCalls: []portllm.ToolCall{{
					ToolCallID:    "call_weather",
					ToolName:      "get_weather",
					ArgumentsJSON: `{"location":"Paris"}`,
				}},
			},
			{
				Role: "tool",
				ToolResults: []portllm.ToolResult{{
					ToolCallID: "call_weather",
					ToolName:   "get_weather",
					OutputJSON: `{"temperature":"20C"}`,
				}},
			},
			{Role: "user", Content: "Use that result."},
		},
		Tools: []portllm.ToolDefinition{{
			Name:        "get_weather",
			Description: "Gets weather for a location.",
			InputSchema: json.RawMessage(`{
				"type":"object",
				"properties":{"location":{"type":"string"}},
				"required":["location"]
			}`),
		}},
		Options: map[string]any{
			"response_format": []any{
				map[string]any{
					"type":      "text",
					"mime_type": "application/json",
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"summary": map[string]any{"type": "string"},
						},
					},
				},
				map[string]any{
					"type":         "image",
					"aspect_ratio": "1:1",
					"image_size":   "1K",
					"mime_type":    "image/jpeg",
				},
			},
			"generation_config": map[string]any{
				"temperature":        0.4,
				"top_p":              0.9,
				"max_output_tokens":  512,
				"thinking_level":     "low",
				"thinking_summaries": "auto",
			},
		},
	})
	if err != nil {
		t.Fatalf("build Gemini interaction universal body: %v", err)
	}
	formats, ok := payload["response_format"].([]any)
	if !ok || len(formats) != 2 {
		t.Fatalf("expected response_format array, got %#v", payload["response_format"])
	}
	if _, leaked := asMap(payload["response_format"])["_list"]; leaked {
		t.Fatalf("response_format must not use private _list wrapper: %#v", payload["response_format"])
	}
	textFormat := asMap(formats[0])
	if textFormat["mime_type"] != "application/json" || asMap(textFormat["schema"])["type"] != "object" {
		t.Fatalf("unexpected structured text response_format: %#v", textFormat)
	}
	imageFormat := asMap(formats[1])
	if imageFormat["type"] != "image" || imageFormat["aspect_ratio"] != "1:1" || imageFormat["image_size"] != "1K" || imageFormat["mime_type"] != "image/jpeg" {
		t.Fatalf("unexpected image response_format: %#v", imageFormat)
	}
	config, ok := payload["generation_config"].(map[string]any)
	if !ok || config["temperature"] != 0.4 || config["top_p"] != 0.9 || config["max_output_tokens"] != 512 || config["thinking_level"] != "low" || config["thinking_summaries"] != "auto" {
		t.Fatalf("unexpected generation_config: %#v", payload["generation_config"])
	}
	tools, ok := payload["tools"].([]map[string]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected Interactions tool declaration, got %#v", payload["tools"])
	}
	if tools[0]["type"] != "function" || tools[0]["name"] != "get_weather" {
		t.Fatalf("unexpected tool declaration: %#v", tools[0])
	}
	steps, ok := payload["input"].([]map[string]any)
	if !ok || len(steps) != 5 {
		t.Fatalf("expected conversation steps with tool call/result, got %#v", payload["input"])
	}
	if steps[0]["type"] != "user_input" || steps[1]["type"] != "model_output" || steps[2]["type"] != "function_call" || steps[3]["type"] != "function_result" || steps[4]["type"] != "user_input" {
		t.Fatalf("unexpected Interactions step order: %#v", steps)
	}
	if steps[2]["name"] != "get_weather" || steps[3]["name"] != "get_weather" {
		t.Fatalf("expected function call/result steps, got %#v", steps)
	}
	if steps[3]["call_id"] != "call_weather" {
		t.Fatalf("expected function result call_id, got %#v", steps[3])
	}
	resultContent, ok := steps[3]["result"].([]map[string]any)
	if !ok || len(resultContent) != 1 || resultContent[0]["type"] != "text" || resultContent[0]["text"] != `{"temperature":"20C"}` {
		t.Fatalf("expected function result content blocks, got %#v", steps[3]["result"])
	}
}

func TestBuildGeminiInteractionRequestBodyMergesNativeAndFunctionTools(t *testing.T) {
	input := portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "Research and calculate."}},
		Options: map[string]any{
			"tools": []any{
				map[string]any{"type": "google_search"},
				map[string]any{"type": "code_execution"},
				map[string]any{"type": "url_context"},
			},
		},
		Tools: []portllm.ToolDefinition{{
			Name:        "get_weather",
			Description: "Gets weather.",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
	}
	payload, err := buildGeminiInteractionRequestBody(portllm.RouteConfig{
		Endpoint:      portllm.EndpointInteractions,
		UpstreamModel: "gemini-3.5-flash",
	}, input)
	if err != nil {
		t.Fatalf("build Gemini interaction request body: %v", err)
	}
	tools, ok := payload["tools"].([]map[string]any)
	if !ok || len(tools) != 4 {
		t.Fatalf("expected three native tools and one function, got %#v", payload["tools"])
	}
	wantTypes := []string{"google_search", "code_execution", "url_context", "function"}
	for index, wantType := range wantTypes {
		if tools[index]["type"] != wantType {
			t.Fatalf("tool %d type = %#v, want %q", index, tools[index]["type"], wantType)
		}
	}
	if tools[3]["name"] != "get_weather" {
		t.Fatalf("expected function tool to be preserved, got %#v", tools[3])
	}

	input.DisableTools = true
	disabledPayload, err := buildGeminiInteractionRequestBody(portllm.RouteConfig{
		Endpoint:      portllm.EndpointInteractions,
		UpstreamModel: "gemini-3.5-flash",
	}, input)
	if err != nil {
		t.Fatalf("build disabled-tools Gemini interaction request body: %v", err)
	}
	if _, exists := disabledPayload["tools"]; exists {
		t.Fatalf("expected DisableTools to remove native and function tools, got %#v", disabledPayload["tools"])
	}
}

func TestBuildGeminiInteractionToolsPreservesJSONSchemaReferences(t *testing.T) {
	payload, err := buildGeminiInteractionRequestBody(portllm.RouteConfig{
		Endpoint:      portllm.EndpointInteractions,
		UpstreamModel: "gemini-3-flash-preview",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "Run the workflow."}},
		Tools: []portllm.ToolDefinition{{
			Name:        "run_workflow",
			Description: "Runs a workflow.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"headers": {"type": "object"},
					"actions": {"type": "array", "items": {"$ref": "#/properties/headers"}},
					"parser": {"anyOf": [{"$ref": "#/$defs/parser"}, {"type": "null"}]}
				},
				"$defs": {"parser": {"type": "object"}},
				"required": ["actions"]
			}`),
		}},
	})
	if err != nil {
		t.Fatalf("build Gemini interaction request body: %v", err)
	}

	tools, ok := payload["tools"].([]map[string]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("expected one Interactions tool, got %#v", payload["tools"])
	}
	parameters, ok := tools[0]["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("expected native JSON Schema parameters, got %#v", tools[0])
	}
	properties := asMap(parameters["properties"])
	if asMap(asMap(properties["actions"])["items"])["$ref"] != "#/properties/headers" {
		t.Fatalf("expected array item reference to be preserved, got %#v", properties["actions"])
	}
	anyOf := asSlice(asMap(properties["parser"])["anyOf"])
	if len(anyOf) != 2 || asMap(anyOf[0])["$ref"] != "#/$defs/parser" {
		t.Fatalf("expected anyOf reference to be preserved, got %#v", anyOf)
	}
	if asMap(asMap(parameters["$defs"])["parser"])["type"] != "object" {
		t.Fatalf("expected JSON Schema definitions to be preserved, got %#v", parameters["$defs"])
	}
}

func TestBuildGeminiInteractionRequestBodyAcceptsTypedResponseFormatList(t *testing.T) {
	payload, err := buildGeminiInteractionRequestBody(portllm.RouteConfig{
		Endpoint:      portllm.EndpointInteractions,
		UpstreamModel: "gemini-3-flash-preview",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "Return text and image."}},
		Options: map[string]any{
			"response_format": []map[string]any{
				{"type": "text"},
				{"type": "image", "image_size": "2K"},
			},
		},
	})
	if err != nil {
		t.Fatalf("build Gemini interaction typed response format body: %v", err)
	}
	formats, ok := payload["response_format"].([]any)
	if !ok || len(formats) != 2 {
		t.Fatalf("expected response_format array, got %#v", payload["response_format"])
	}
	imageFormat := asMap(formats[1])
	if imageFormat["type"] != "image" || imageFormat["image_size"] != "2K" {
		t.Fatalf("unexpected typed response_format image entry: %#v", imageFormat)
	}
}

func TestBuildGeminiInteractionRequestBodyNormalizesVideoOptions(t *testing.T) {
	payload, err := buildGeminiInteractionRequestBody(portllm.RouteConfig{
		Endpoint:      portllm.EndpointInteractions,
		UpstreamModel: "gemini-omni-flash-preview",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "Edit this video."}},
		Options: map[string]any{
			"response_format": map[string]any{"type": "video", "aspect_ratio": "1:1"},
			"generation_config": map[string]any{
				"video_config": map[string]any{"task": "edit"},
			},
		},
	})
	if err != nil {
		t.Fatalf("build Gemini interaction video body: %v", err)
	}
	responseFormat := asMap(payload["response_format"])
	if _, ok := responseFormat["aspect_ratio"]; ok {
		t.Fatalf("unsupported video aspect ratio should be dropped, got %#v", responseFormat)
	}
	videoConfig := asMap(asMap(payload["generation_config"])["video_config"])
	if videoConfig["task"] != "edit" {
		t.Fatalf("expected video edit task to normalize to edit, got %#v", videoConfig)
	}
}

func TestBuildGeminiInteractionRequestBodyUsesConversationSteps(t *testing.T) {
	payload, err := buildGeminiInteractionRequestBody(portllm.RouteConfig{
		Endpoint:      portllm.EndpointInteractions,
		UpstreamModel: "gemini-3.5-flash",
	}, portllm.GenerateInput{
		Instructions: "Be concise.",
		Messages: []portllm.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
			{Role: "user", Content: "Reply with OK."},
		},
		PreviousResponseID: "interaction-prev",
	})
	if err != nil {
		t.Fatalf("build Gemini interaction chat body: %v", err)
	}
	if _, ok := payload["response_format"]; ok {
		t.Fatalf("chat interaction should not force media response_format, got %#v", payload["response_format"])
	}
	if payload["system_instruction"] != "Be concise." || payload["previous_interaction_id"] != "interaction-prev" {
		t.Fatalf("expected instruction and previous interaction id, got %#v", payload)
	}
	steps, ok := payload["input"].([]map[string]any)
	if !ok || len(steps) != 3 {
		t.Fatalf("expected conversation steps, got %#v", payload["input"])
	}
	if steps[0]["type"] != "user_input" || steps[1]["type"] != "model_output" || steps[2]["content"] != "Reply with OK." {
		t.Fatalf("unexpected steps: %#v", steps)
	}
}

func TestParseGeminiInteractionOutputExtractsVideoURIAndInlineData(t *testing.T) {
	inline := base64.StdEncoding.EncodeToString([]byte("video"))
	body := []byte(`{
		"id": "interaction-1",
		"steps": [{
			"type": "model_output",
			"content": [
				{"type": "video", "uri": "https://example.com/video.mp4", "mime_type": "video/mp4"},
				{"type": "video", "uri": "https://example.com/video.mp4", "mime_type": "video/mp4"},
				{"type": "video", "data": "` + inline + `", "mime_type": "video/webm"}
			]
		}],
		"usage": {"total_input_tokens": 3, "total_output_tokens": 5}
	}`)
	output, err := parseGeminiInteractionOutput(body)
	if err != nil {
		t.Fatalf("parse Gemini interaction output: %v", err)
	}
	if output.ResponseID != "interaction-1" {
		t.Fatalf("unexpected response id: %q", output.ResponseID)
	}
	if got := len(output.GeneratedVideos); got != 2 {
		t.Fatalf("expected duplicate URI to be deduped, got %d videos: %#v", got, output.GeneratedVideos)
	}
	if output.GeneratedVideos[0].URL != "https://example.com/video.mp4" || output.GeneratedVideos[0].MIMEType != "video/mp4" {
		t.Fatalf("unexpected URI video: %#v", output.GeneratedVideos[0])
	}
	if output.GeneratedVideos[1].B64JSON != inline || output.GeneratedVideos[1].MIMEType != "video/webm" {
		t.Fatalf("unexpected inline video: %#v", output.GeneratedVideos[1])
	}
	if output.Usage.InputTokens != 3 || output.Usage.OutputTokens != 5 {
		t.Fatalf("unexpected usage: %#v", output.Usage)
	}
}

func TestParseGeminiInteractionOutputExtractsReasoningAndOfficialUsage(t *testing.T) {
	body := []byte(`{
		"id": "interaction-reasoning",
		"service_tier": "priority",
		"steps": [{
			"type": "thought",
			"summary": [
				{"type": "text", "text": "Check the inputs."},
				{"type": "text", "text": " Then answer."}
			],
			"signature": "thought-signature"
		}],
		"usage": {
			"total_input_tokens": 10,
			"total_cached_tokens": 4,
			"total_output_tokens": 6,
			"total_thought_tokens": 3,
			"total_tool_use_tokens": 2
		}
	}`)
	output, err := parseGeminiInteractionOutput(body)
	if err != nil {
		t.Fatalf("parse Gemini interaction output: %v", err)
	}
	if output.Reasoning == nil || output.Reasoning.Summary != "Check the inputs.\n\nThen answer." || output.Reasoning.Signature != "thought-signature" {
		t.Fatalf("unexpected reasoning: %#v", output.Reasoning)
	}
	if output.Usage.InputTokens != 6 || output.Usage.CacheReadTokens != 4 || output.Usage.OutputTokens != 6 || output.Usage.ReasoningTokens != 3 || output.Usage.ServiceTier != "priority" {
		t.Fatalf("unexpected usage: %#v", output.Usage)
	}
	if !strings.Contains(output.Usage.RawUsageJSON, `"total_tool_use_tokens":2`) {
		t.Fatalf("expected raw usage to preserve tool-use tokens, got %q", output.Usage.RawUsageJSON)
	}
}

func TestParseGeminiInteractionOutputExtractsTextAndImages(t *testing.T) {
	inline := base64.StdEncoding.EncodeToString([]byte("png"))
	inputInline := base64.StdEncoding.EncodeToString([]byte("source"))
	body := []byte(`{
		"id": "interaction-2",
		"steps": [
			{"type": "user_input", "content": [
				{"type": "image", "data": "` + inputInline + `", "mime_type": "image/png"}
			]},
			{"type": "model_output", "content": [
				{"type": "text", "text": "A revised prompt"},
				{"type": "image", "data": "` + inline + `", "mime_type": "image/png"},
				{"type": "image", "uri": "https://example.com/image.png", "mime_type": "image/png"}
			]}
		]
	}`)
	output, err := parseGeminiInteractionOutput(body)
	if err != nil {
		t.Fatalf("parse Gemini interaction output: %v", err)
	}
	if output.Text != "A revised prompt" {
		t.Fatalf("expected text from steps, got %q", output.Text)
	}
	if len(output.GeneratedImages) != 2 {
		t.Fatalf("expected generated images, got %#v", output.GeneratedImages)
	}
	if output.GeneratedImages[0].B64JSON != inline || output.GeneratedImages[0].RevisedPrompt != "A revised prompt" {
		t.Fatalf("unexpected inline image: %#v", output.GeneratedImages[0])
	}
	if output.GeneratedImages[0].B64JSON == inputInline {
		t.Fatalf("user input image must not be treated as generated output: %#v", output.GeneratedImages)
	}
	if output.GeneratedImages[1].URL != "https://example.com/image.png" {
		t.Fatalf("unexpected URI image: %#v", output.GeneratedImages[1])
	}
}

func TestParseGeminiInteractionOutputExtractsFunctionCalls(t *testing.T) {
	body := []byte(`{
		"id": "interaction-tools",
		"steps": [
			{"type": "model_output", "content": [{"type": "text", "text": "Let me check."}]},
			{"type": "function_call", "id": "call_weather", "name": "get_weather", "arguments": {"location": "Paris"}}
		]
	}`)
	output, err := parseGeminiInteractionOutput(body)
	if err != nil {
		t.Fatalf("parse Gemini interaction output: %v", err)
	}
	if output.Text != "Let me check." {
		t.Fatalf("expected text from model_output only, got %q", output.Text)
	}
	if len(output.ToolCalls) != 1 {
		t.Fatalf("expected function call, got %#v", output.ToolCalls)
	}
	call := output.ToolCalls[0]
	if call.ToolCallID != "call_weather" || call.ToolType != "function" || call.ToolName != "get_weather" || call.Status != "requested" {
		t.Fatalf("unexpected function call: %#v", call)
	}
	if call.ArgumentsJSON != `{"location":"Paris"}` {
		t.Fatalf("unexpected arguments: %s", call.ArgumentsJSON)
	}
}

func TestParseGeminiInteractionOutputExtractsNativeToolTracesAndUsage(t *testing.T) {
	body := []byte(`{
		"id": "interaction-native-tools",
		"steps": [
			{"type": "thought", "signature": "thought_sig_1", "summary": [{"type": "text", "text": "I should verify the result."}]},
			{"type": "google_search_call", "arguments": {"query": "Paris 2024 men's 100m winner"}, "id": "search_call_1"},
			{"type": "google_search_result", "call_id": "search_call_1", "result": [{"title": "Olympics", "url": "https://example.com/olympics", "snippet": "Noah Lyles won."}]},
			{"type": "code_execution_call", "arguments": {"code": "print(sum(range(1, 11)))", "language": "python"}, "id": "code_call_1"},
			{"type": "code_execution_result", "call_id": "code_call_1", "result": "55\n"},
			{"type": "url_context_call", "arguments": {"urls": ["https://example.com"]}, "id": "url_call_1"},
			{"type": "url_context_result", "call_id": "url_call_1", "result": [{"title": "Example Domain", "url": "https://example.com", "snippet": "Example content."}]}
		],
		"usage": {
			"total_input_tokens": 20,
			"total_cached_tokens": 5,
			"total_output_tokens": 8,
			"total_thought_tokens": 3
		},
		"service_tier": "standard"
	}`)
	output, err := parseGeminiInteractionOutput(body)
	if err != nil {
		t.Fatalf("parse Gemini interaction output: %v", err)
	}
	if len(output.ServerToolCalls) != 3 {
		t.Fatalf("expected three native tool traces, got %#v", output.ServerToolCalls)
	}
	for _, call := range output.ServerToolCalls {
		if call.Status != "completed" || call.ArgumentsJSON == "" || call.OutputJSON == "" {
			t.Fatalf("expected completed native tool trace, got %#v", call)
		}
		if output.ServerSideToolUsage[call.ToolName] != 1 {
			t.Fatalf("expected one %s invocation, got %#v", call.ToolName, output.ServerSideToolUsage)
		}
	}
	if output.Usage.InputTokens != 15 || output.Usage.CacheReadTokens != 5 || output.Usage.OutputTokens != 8 || output.Usage.ReasoningTokens != 3 {
		t.Fatalf("unexpected Interactions usage: %#v", output.Usage)
	}
	if output.Usage.ServiceTier != "standard" {
		t.Fatalf("expected service tier, got %#v", output.Usage)
	}
	if output.Reasoning == nil || output.Reasoning.Summary != "I should verify the result." || output.Reasoning.Signature != "thought_sig_1" {
		t.Fatalf("expected Interactions thought summary, got %#v", output.Reasoning)
	}
}

func TestApplyGeminiInteractionStreamEventMergesNativeToolDeltas(t *testing.T) {
	result := &portllm.GenerateOutput{}
	streamState := newGeminiInteractionStreamState()
	events := make([]portllm.ToolCall, 0)
	onEvent := func(event portllm.GenerateStreamEvent) error {
		if event.ServerToolCall != nil {
			events = append(events, *event.ServerToolCall)
		}
		return nil
	}
	chunks := []string{
		`{"event_type":"step.start","index":2,"step":{"type":"google_search_call","arguments":{"queries":["latest Gemini news"]},"id":"search_call_1"}}`,
		`{"event_type":"step.delta","index":2,"delta":{"type":"google_search_call","arguments":{"queries":["latest Gemini news"]}}}`,
		`{"event_type":"step.start","index":3,"step":{"type":"google_search_result","call_id":"search_call_1"}}`,
		`{"event_type":"step.delta","index":3,"delta":{"type":"google_search_result","result":[{"title":"Gemini","url":"https://example.com/gemini"}]}}`,
		`{"event_type":"interaction.completed","interaction":{"id":"interaction-tools-stream","steps":[{"type":"google_search_call","arguments":{"query":"latest Gemini news"},"id":"search_call_1"},{"type":"google_search_result","call_id":"search_call_1","result":[{"title":"Gemini","url":"https://example.com/gemini"}]}]}}`,
	}
	for _, chunk := range chunks {
		if err := applyGeminiInteractionStreamEvent(mustDecodeObject(t, chunk), result, streamState, onEvent); err != nil {
			t.Fatalf("apply Gemini interaction stream event: %v", err)
		}
	}
	if len(result.ServerToolCalls) != 1 {
		t.Fatalf("expected one merged native tool trace, got %#v", result.ServerToolCalls)
	}
	call := result.ServerToolCalls[0]
	if call.ToolCallID != "search_call_1" || call.Status != "completed" || call.ArgumentsJSON == "" || call.OutputJSON == "" {
		t.Fatalf("unexpected merged native tool trace: %#v", call)
	}
	if result.ServerSideToolUsage["google_search"] != 1 {
		t.Fatalf("expected one streamed search invocation, got %#v", result.ServerSideToolUsage)
	}
	if len(events) != 4 || events[len(events)-1].Status != "completed" {
		t.Fatalf("unexpected native tool events: %#v", events)
	}
	for _, event := range events {
		if event.ToolCallID != "search_call_1" {
			t.Fatalf("expected a stable streamed tool-call id, got %#v", events)
		}
	}
}

func TestApplyGeminiInteractionStreamEventPreservesInt64StepIndex(t *testing.T) {
	result := &portllm.GenerateOutput{}
	streamState := newGeminiInteractionStreamState()
	chunks := []string{
		`{"event_type":"step.start","index":"9223372036854775807","step":{"type":"google_search_call","arguments":{"queries":["Gemini"]}}}`,
		`{"event_type":"step.delta","index":"9223372036854775807","delta":{"type":"google_search_call","arguments":{"queries":["Gemini"]}}}`,
	}
	for _, chunk := range chunks {
		if err := applyGeminiInteractionStreamEvent(mustDecodeObject(t, chunk), result, streamState, nil); err != nil {
			t.Fatalf("apply Gemini interaction stream event: %v", err)
		}
	}
	if len(result.ServerToolCalls) != 1 {
		t.Fatalf("expected one merged native tool trace, got %#v", result.ServerToolCalls)
	}
	if got := result.ServerToolCalls[0].ToolCallID; got != "gemini_interaction_google_search_9223372036854775807" {
		t.Fatalf("expected full int64 index in stable tool-call id, got %q", got)
	}
}

func TestApplyGeminiInteractionStreamEventCapturesThoughtSummaryAndMetadataUsage(t *testing.T) {
	result := &portllm.GenerateOutput{}
	streamState := newGeminiInteractionStreamState()
	reasoningEvents := make([]portllm.ReasoningDelta, 0)
	usageEvents := make([]portllm.Usage, 0)
	onEvent := func(event portllm.GenerateStreamEvent) error {
		if event.Reasoning != nil {
			reasoningEvents = append(reasoningEvents, *event.Reasoning)
		}
		if event.Usage != (portllm.Usage{}) {
			usageEvents = append(usageEvents, event.Usage)
		}
		return nil
	}
	chunks := []string{
		`{"event_type":"step.start","index":0,"step":{"type":"thought"}}`,
		`{"event_type":"step.delta","index":0,"delta":{"type":"thought_summary","content":{"type":"text","text":"I should search first."}}}`,
		`{"event_type":"step.delta","index":0,"delta":{"type":"thought_signature","signature":"thought_sig_stream"}}`,
		`{"event_type":"step.delta","index":1,"delta":{"type":"text","text":"Answer"},"metadata":{"total_usage":{"total_input_tokens":12,"total_cached_tokens":2,"total_output_tokens":4,"total_thought_tokens":3,"total_tool_use_tokens":1}}}`,
		`{"event_type":"interaction.completed","interaction":{"id":"interaction-thought-stream","model":"gemini-3.6-flash","status":"completed"}}`,
	}
	for _, chunk := range chunks {
		if err := applyGeminiInteractionStreamEvent(mustDecodeObject(t, chunk), result, streamState, onEvent); err != nil {
			t.Fatalf("apply Gemini interaction stream event: %v", err)
		}
	}
	if result.Reasoning == nil || result.Reasoning.Summary != "I should search first." || result.Reasoning.Signature != "thought_sig_stream" {
		t.Fatalf("unexpected streamed thought summary: %#v", result.Reasoning)
	}
	if len(reasoningEvents) != 2 || reasoningEvents[0].Kind != "summary_text" {
		t.Fatalf("unexpected streamed reasoning events: %#v", reasoningEvents)
	}
	if result.Usage.InputTokens != 10 || result.Usage.CacheReadTokens != 2 || result.Usage.OutputTokens != 4 || result.Usage.ReasoningTokens != 3 {
		t.Fatalf("unexpected metadata usage: %#v", result.Usage)
	}
	if len(usageEvents) != 1 || usageEvents[0].InputTokens != 10 {
		t.Fatalf("unexpected usage events: %#v", usageEvents)
	}
	if result.ResponseID != "interaction-thought-stream" || result.Text != "Answer" {
		t.Fatalf("unexpected final stream result: %#v", result)
	}
}

func TestParseGeminiInteractionOutputPreservesNativeToolFailure(t *testing.T) {
	output, err := parseGeminiInteractionOutput([]byte(`{
		"id": "interaction-native-tool-error",
		"steps": [
			{"type": "code_execution_call", "arguments": {"code": "raise RuntimeError()"}, "id": "code_call_error"},
			{"type": "code_execution_result", "call_id": "code_call_error", "result": "RuntimeError", "is_error": true}
		]
	}`))
	if err != nil {
		t.Fatalf("parse Gemini interaction output: %v", err)
	}
	if len(output.ServerToolCalls) != 1 {
		t.Fatalf("expected one failed native tool trace, got %#v", output.ServerToolCalls)
	}
	call := output.ServerToolCalls[0]
	if call.Status != "error" || call.ErrorJSON != `"RuntimeError"` || call.OutputJSON != `"RuntimeError"` {
		t.Fatalf("unexpected failed native tool trace: %#v", call)
	}
	if output.ServerSideToolUsage["code_execution"] != 1 {
		t.Fatalf("expected one code execution invocation, got %#v", output.ServerSideToolUsage)
	}
}

func TestGenerateGeminiInteractionPostsInteractionsRequest(t *testing.T) {
	var capturedPath string
	var capturedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		if !strings.HasSuffix(capturedPath, "/v1beta/interactions") {
			t.Fatalf("unexpected request path: %s", capturedPath)
		}
		if got := r.Header.Get("X-goog-api-key"); got != "test-key" {
			t.Fatalf("expected Gemini API key header, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("expected Gemini interactions request to avoid bearer auth, got %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedPayload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"interaction-1","steps":[{"type":"model_output","content":[{"type":"video","uri":"https://example.com/video.mp4","mime_type":"video/mp4"}]}]}`))
	}))
	defer server.Close()

	output, err := newTestClient().generateGeminiInteraction(context.Background(), portllm.RouteConfig{
		BaseURL:       server.URL,
		APIKey:        "test-key",
		UpstreamModel: "gemini-omni-flash-preview",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "Make a short video"}},
		Options:  map[string]any{"response_format": map[string]any{"type": "video"}},
	})
	if err != nil {
		t.Fatalf("generate Gemini interaction: %v", err)
	}
	if capturedPayload["model"] != "gemini-omni-flash-preview" || capturedPayload["input"] != "Make a short video" {
		t.Fatalf("unexpected request payload: %#v", capturedPayload)
	}
	if len(output.GeneratedVideos) != 1 || output.GeneratedVideos[0].URL != "https://example.com/video.mp4" {
		t.Fatalf("unexpected generated videos: %#v", output.GeneratedVideos)
	}
}

func TestGenerateGeminiInteractionStreamPostsStreamRequest(t *testing.T) {
	var capturedPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1beta/interactions") {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-goog-api-key"); got != "test-key" {
			t.Fatalf("expected Gemini API key header, got %q", got)
		}
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("expected event-stream accept header, got %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&capturedPayload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"event_type":"interaction.created","interaction":{"id":"interaction-stream-1","service_tier":"standard"}}

data: {"event_type":"step.start","index":0,"step":{"type":"thought","summary":[{"type":"text","text":"I should check"}],"signature":""}}

data: {"event_type":"step.delta","index":0,"delta":{"type":"thought_summary","content":{"type":"text","text":" the weather."}}}

data: {"event_type":"step.delta","index":0,"delta":{"type":"thought_signature","signature":"stream-signature"}}

data: {"event_type":"step.start","index":1,"step":{"type":"function_call","id":"call_weather","name":"get_weather","arguments":{"stale":true}}}

data: {"event_type":"step.delta","index":1,"delta":{"type":"arguments_delta","arguments":"{\"location\":\""}}

data: {"event_type":"step.delta","index":1,"delta":{"type":"arguments_delta","arguments":"Paris\"}"}}

data: {"event_type":"step.stop","index":1,"status":"waiting"}

data: {"event_type":"step.start","index":2,"step":{"type":"model_output","content":[{"type":"text","text":"Hello"}]}}

data: {"event_type":"step.delta","index":2,"delta":{"type":"text","text":" world"},"metadata":{"total_usage":{"total_input_tokens":4,"total_cached_tokens":1,"total_output_tokens":2,"total_thought_tokens":3,"total_tool_use_tokens":1}}}

data: {"event_type":"step.start","index":3,"step":{"type":"google_search_call","id":"search_call_1","arguments":{}}}

data: {"event_type":"step.delta","index":3,"delta":{"type":"google_search_call","arguments":{"queries":["Gemini streaming"]},"signature":"search-signature"}}

data: {"event_type":"step.start","index":4,"step":{"type":"google_search_result","call_id":"search_call_1"}}

data: {"event_type":"step.delta","index":4,"delta":{"type":"google_search_result","result":[{"title":"Gemini","url":"https://ai.google.dev/gemini-api/docs/streaming"}],"signature":"result-signature"}}

data: {"event_type":"step.start","index":5,"step":{"type":"model_output"}}

data: {"event_type":"step.delta","index":5,"delta":{"type":"image","mime_type":"image/jpeg","data":"aW1hZ2U="}}

data: {"event_type":"step.start","index":6,"step":{"type":"model_output"}}

data: {"event_type":"step.delta","index":6,"delta":{"type":"video","mime_type":"video/mp4","uri":"https://example.com/video.mp4"}}

data: {"event_type":"interaction.completed","interaction":{"id":"interaction-stream-1","status":"completed","usage":{"total_input_tokens":4,"total_cached_tokens":1,"total_output_tokens":2,"total_thought_tokens":3,"total_tool_use_tokens":1}}}

data: [DONE]

`))
	}))
	defer server.Close()

	var deltas []string
	var reasoningDeltas []portllm.ReasoningDelta
	var usageEvents []portllm.Usage
	var serverToolEvents []portllm.ToolCall
	var imageEvents []portllm.GenerateStreamEvent
	output, err := newTestClient().GenerateStream(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterGeminiInteractions,
		BaseURL:       server.URL,
		APIKey:        "test-key",
		UpstreamModel: "gemini-3.5-flash",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "How does AI work?"}},
	}, func(event portllm.GenerateStreamEvent) error {
		if event.Delta != "" {
			deltas = append(deltas, event.Delta)
		}
		if event.Reasoning != nil {
			reasoningDeltas = append(reasoningDeltas, *event.Reasoning)
		}
		if event.Usage != (portllm.Usage{}) {
			usageEvents = append(usageEvents, event.Usage)
		}
		if event.ServerToolCall != nil {
			serverToolEvents = append(serverToolEvents, *event.ServerToolCall)
		}
		if event.GeneratedImage != nil {
			imageEvents = append(imageEvents, event)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("generate Gemini interaction stream: %v", err)
	}
	if capturedPayload["model"] != "gemini-3.5-flash" || capturedPayload["input"] != "How does AI work?" || capturedPayload["stream"] != true {
		t.Fatalf("unexpected stream request payload: %#v", capturedPayload)
	}
	if output.ResponseID != "interaction-stream-1" || output.Text != "Hello world" {
		t.Fatalf("unexpected stream output: %#v", output)
	}
	if strings.Join(deltas, "") != "Hello world" {
		t.Fatalf("unexpected stream deltas: %#v", deltas)
	}
	if output.Reasoning == nil || output.Reasoning.Summary != "I should check the weather." || output.Reasoning.Signature != "stream-signature" {
		t.Fatalf("unexpected stream reasoning: %#v", output.Reasoning)
	}
	if len(reasoningDeltas) != 3 || reasoningDeltas[0].Kind != "summary_text" || reasoningDeltas[0].Text != "I should check" || reasoningDeltas[1].Text != " the weather." || reasoningDeltas[2].Signature != "stream-signature" {
		t.Fatalf("unexpected reasoning deltas: %#v", reasoningDeltas)
	}
	if len(output.ToolCalls) != 1 || output.ToolCalls[0].ToolCallID != "call_weather" || output.ToolCalls[0].ToolName != "get_weather" || output.ToolCalls[0].ArgumentsJSON != `{"location":"Paris"}` {
		t.Fatalf("unexpected stream tool calls: %#v", output.ToolCalls)
	}
	if len(output.ServerToolCalls) != 1 || output.ServerToolCalls[0].ToolCallID != "search_call_1" || output.ServerToolCalls[0].ToolName != "google_search" || output.ServerToolCalls[0].Status != "completed" {
		t.Fatalf("unexpected stream server tool calls: %#v", output.ServerToolCalls)
	}
	if output.ServerToolCalls[0].ArgumentsJSON != `{"queries":["Gemini streaming"]}` || !strings.Contains(output.ServerToolCalls[0].OutputJSON, `"https://ai.google.dev/gemini-api/docs/streaming"`) {
		t.Fatalf("unexpected stream server tool payload: %#v", output.ServerToolCalls[0])
	}
	if len(serverToolEvents) != 4 || serverToolEvents[len(serverToolEvents)-1].Status != "completed" || output.ServerSideToolUsage["google_search"] != 1 {
		t.Fatalf("unexpected server tool events=%#v usage=%#v", serverToolEvents, output.ServerSideToolUsage)
	}
	if len(output.Citations) != 1 || output.Citations[0] != "https://ai.google.dev/gemini-api/docs/streaming" {
		t.Fatalf("unexpected citations: %#v", output.Citations)
	}
	if len(output.GeneratedImages) != 1 || output.GeneratedImages[0].B64JSON != "aW1hZ2U=" || output.GeneratedImages[0].MIMEType != "image/jpeg" || output.GeneratedImages[0].RevisedPrompt != "Hello world" {
		t.Fatalf("unexpected streamed images: %#v", output.GeneratedImages)
	}
	if len(imageEvents) != 1 || !imageEvents[0].GeneratedImagePartial || imageEvents[0].GeneratedImageIndex != 0 {
		t.Fatalf("unexpected streamed image events: %#v", imageEvents)
	}
	if len(output.GeneratedVideos) != 1 || output.GeneratedVideos[0].URL != "https://example.com/video.mp4" || output.GeneratedVideos[0].MIMEType != "video/mp4" {
		t.Fatalf("unexpected streamed videos: %#v", output.GeneratedVideos)
	}
	if len(usageEvents) != 2 || output.Usage.InputTokens != 3 || output.Usage.CacheReadTokens != 1 || output.Usage.OutputTokens != 2 || output.Usage.ReasoningTokens != 3 || output.Usage.ServiceTier != "standard" {
		t.Fatalf("unexpected stream usage events=%#v output=%#v", usageEvents, output.Usage)
	}
}

func TestParseGeminiInteractionUsageReadsOfficialStepShapes(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
	}{
		{
			name: "accumulated step usage",
			payload: map[string]any{
				"usage": map[string]any{
					"total_input_tokens":  float64(8),
					"total_cached_tokens": float64(3),
					"total_output_tokens": float64(5),
				},
			},
		},
		{
			name: "delta metadata total usage",
			payload: map[string]any{
				"metadata": map[string]any{
					"total_usage": map[string]any{
						"total_input_tokens":   float64(8),
						"total_cached_tokens":  float64(3),
						"total_output_tokens":  float64(5),
						"total_thought_tokens": float64(2),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := parseGeminiInteractionUsage(tt.payload)
			if usage.InputTokens != 5 || usage.CacheReadTokens != 3 || usage.OutputTokens != 5 {
				t.Fatalf("unexpected usage: %#v", usage)
			}
		})
	}
}

func TestParseGeminiInteractionOutputExtractsOfficialServerToolSteps(t *testing.T) {
	body := []byte(`{
		"id":"interaction-native-tools",
		"steps":[
			{"type":"code_execution_call","id":"code_call_1","arguments":{"code":"print(42)","language":"python"}},
			{"type":"code_execution_result","call_id":"code_call_1","result":"42\n"},
			{"type":"url_context_call","id":"url_call_1","arguments":{"urls":["https://example.com"]}},
			{"type":"url_context_result","call_id":"url_call_1","result":[{"url":"https://example.com","status":"success"}]}
		]
	}`)
	output, err := parseGeminiInteractionOutput(body)
	if err != nil {
		t.Fatalf("parse Gemini interaction output: %v", err)
	}
	if len(output.ServerToolCalls) != 2 {
		t.Fatalf("unexpected server tool calls: %#v", output.ServerToolCalls)
	}
	if output.ServerToolCalls[0].ToolName != "code_execution" || output.ServerToolCalls[0].Status != "completed" || output.ServerToolCalls[0].ArgumentsJSON != `{"code":"print(42)","language":"python"}` || output.ServerToolCalls[0].OutputJSON != `"42\n"` {
		t.Fatalf("unexpected code execution call: %#v", output.ServerToolCalls[0])
	}
	if output.ServerToolCalls[1].ToolName != "url_context" || output.ServerToolCalls[1].Status != "completed" || !strings.Contains(output.ServerToolCalls[1].OutputJSON, `"https://example.com"`) {
		t.Fatalf("unexpected URL context call: %#v", output.ServerToolCalls[1])
	}
	if output.ServerSideToolUsage["code_execution"] != 1 || output.ServerSideToolUsage["url_context"] != 1 {
		t.Fatalf("unexpected server-side tool usage: %#v", output.ServerSideToolUsage)
	}
	if len(output.Citations) != 1 || output.Citations[0] != "https://example.com" {
		t.Fatalf("unexpected citations: %#v", output.Citations)
	}
}

func TestGeminiInteractionStreamToolCallKeepsStepStartArgumentsWithoutDeltas(t *testing.T) {
	result := &portllm.GenerateOutput{}
	state := &geminiInteractionStreamState{}
	updateGeminiInteractionStreamToolCall(result, state, map[string]any{
		"index": float64(0),
		"step": map[string]any{
			"type":      "function_call",
			"id":        "call-1",
			"name":      "lookup",
			"arguments": map[string]any{"query": "weather"},
		},
	}, "step.start")
	updateGeminiInteractionStreamToolCall(result, state, map[string]any{
		"index": float64(0),
	}, "step.stop")
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].ArgumentsJSON != `{"query":"weather"}` {
		t.Fatalf("unexpected tool calls: %#v", result.ToolCalls)
	}
}

func TestNewGeminiRequestUsesOnlyGoogleAPIKeyForOfficialHost(t *testing.T) {
	req, err := newTestClient().newGeminiRequest(context.Background(), http.MethodPost, "https://generativelanguage.googleapis.com/v1beta/interactions", nil, portllm.RouteConfig{
		APIKey: "test-key",
	}, nil)
	if err != nil {
		t.Fatalf("build Gemini request: %v", err)
	}
	if got := req.Header.Get("X-goog-api-key"); got != "test-key" {
		t.Fatalf("expected Google API key header, got %q", got)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("expected official Gemini host to avoid bearer fallback, got %q", got)
	}
}
