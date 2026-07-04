package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiInteractionsAdapterDefaults(t *testing.T) {
	if got := DefaultEndpointForAdapter(AdapterGeminiInteractions); got != EndpointInteractions {
		t.Fatalf("expected interactions endpoint, got %q", got)
	}
	if !IsVideoGenerationAdapter(AdapterGeminiInteractions) {
		t.Fatal("expected Gemini Interactions to be a video generation adapter")
	}
	if !SupportsStreamingAdapter(AdapterGeminiInteractions) {
		t.Fatal("Gemini Interactions should support official streaming")
	}
	if !IsImageGenerationAdapter(AdapterGeminiInteractions) {
		t.Fatal("Gemini Interactions should support image generation")
	}
	if !IsImageEditAdapter(AdapterGeminiInteractions) {
		t.Fatal("Gemini Interactions should support image editing")
	}
}

func TestBuildGeminiInteractionRequestBody(t *testing.T) {
	payload, err := buildGeminiInteractionRequestBody(RouteConfig{
		Endpoint:      EndpointInteractions,
		UpstreamModel: "gemini-omni-flash-preview",
	}, GenerateInput{
		Messages: []Message{{
			Role: "user",
			Parts: []ContentPart{
				{Kind: ContentPartText, Text: "A short cinematic product video"},
				{Kind: ContentPartImage, MimeType: "image/png", Data: []byte("image-bytes")},
			},
		}},
		Options: map[string]interface{}{
			"response_format": map[string]interface{}{"type": "video", "aspect_ratio": "16:9", "delivery": "b64_json"},
			"generation_config": map[string]interface{}{
				"video_config": map[string]interface{}{"task": "IMAGE_TO_VIDEO"},
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
	responseFormat, ok := payload["response_format"].(map[string]interface{})
	if !ok || responseFormat["type"] != "video" {
		t.Fatalf("expected video response format, got %#v", payload["response_format"])
	}
	if responseFormat["delivery"] != "uri" {
		t.Fatalf("expected video delivery to use URI downloads, got %#v", payload["response_format"])
	}
	if responseFormat["aspect_ratio"] != "16:9" {
		t.Fatalf("expected response_format aspect ratio, got %#v", payload["response_format"])
	}
	config, ok := payload["generation_config"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected generation config, got %#v", payload["generation_config"])
	}
	videoConfig, ok := config["video_config"].(map[string]interface{})
	if !ok || videoConfig["task"] != "image_to_video" {
		t.Fatalf("expected video config task, got %#v", payload["generation_config"])
	}
	input, ok := payload["input"].([]map[string]interface{})
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
	_, err := buildGeminiInteractionRequestBody(RouteConfig{
		Endpoint: EndpointInteractions,
	}, GenerateInput{
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err == nil {
		t.Fatal("expected empty Interactions model to be rejected")
	}
}

func TestBuildGeminiInteractionRequestBodySupportsUniversalOptionsAndTools(t *testing.T) {
	payload, err := buildGeminiInteractionRequestBody(RouteConfig{
		Endpoint:      EndpointInteractions,
		UpstreamModel: "gemini-3-flash-preview",
	}, GenerateInput{
		Messages: []Message{
			{Role: "user", Content: "Create a short answer and an image."},
			{
				Role:    "assistant",
				Content: "I need the weather first.",
				ToolCalls: []ToolCall{{
					ToolCallID:    "call_weather",
					ToolName:      "get_weather",
					ArgumentsJSON: `{"location":"Paris"}`,
				}},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{{
					ToolCallID: "call_weather",
					ToolName:   "get_weather",
					OutputJSON: `{"temperature":"20C"}`,
				}},
			},
			{Role: "user", Content: "Use that result."},
		},
		Tools: []ToolDefinition{{
			Name:        "get_weather",
			Description: "Gets weather for a location.",
			InputSchema: json.RawMessage(`{
				"type":"object",
				"properties":{"location":{"type":"string"}},
				"required":["location"]
			}`),
		}},
		Options: map[string]interface{}{
			"response_format": []interface{}{
				map[string]interface{}{"type": "text"},
				map[string]interface{}{
					"type":         "image",
					"aspect_ratio": "1:1",
					"image_size":   "1K",
					"mime_type":    "image/png",
				},
			},
			"temperature":       0.4,
			"top_p":             0.9,
			"max_output_tokens": 512,
			"thinking_level":    "low",
			"generation_config": map[string]interface{}{
				"thinkingLevel": "high",
			},
		},
	})
	if err != nil {
		t.Fatalf("build Gemini interaction universal body: %v", err)
	}
	formats, ok := payload["response_format"].([]interface{})
	if !ok || len(formats) != 2 {
		t.Fatalf("expected response_format array, got %#v", payload["response_format"])
	}
	if _, leaked := asMap(payload["response_format"])["_list"]; leaked {
		t.Fatalf("response_format must not use private _list wrapper: %#v", payload["response_format"])
	}
	imageFormat := asMap(formats[1])
	if imageFormat["type"] != "image" || imageFormat["aspect_ratio"] != "1:1" || imageFormat["image_size"] != "1K" || imageFormat["mime_type"] != "image/png" {
		t.Fatalf("unexpected image response_format: %#v", imageFormat)
	}
	config, ok := payload["generation_config"].(map[string]interface{})
	if !ok || config["temperature"] != 0.4 || config["top_p"] != 0.9 || config["max_output_tokens"] != 512 || config["thinking_level"] != "low" {
		t.Fatalf("unexpected generation_config: %#v", payload["generation_config"])
	}
	tools, ok := payload["tools"].([]map[string]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("expected Interactions tool declaration, got %#v", payload["tools"])
	}
	if tools[0]["type"] != "function" || tools[0]["name"] != "get_weather" {
		t.Fatalf("unexpected tool declaration: %#v", tools[0])
	}
	steps, ok := payload["input"].([]map[string]interface{})
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
	resultContent, ok := steps[3]["result"].([]map[string]interface{})
	if !ok || len(resultContent) != 1 || resultContent[0]["type"] != "text" || resultContent[0]["text"] != `{"temperature":"20C"}` {
		t.Fatalf("expected function result content blocks, got %#v", steps[3]["result"])
	}
}

func TestBuildGeminiInteractionRequestBodyAcceptsTypedResponseFormatList(t *testing.T) {
	payload, err := buildGeminiInteractionRequestBody(RouteConfig{
		Endpoint:      EndpointInteractions,
		UpstreamModel: "gemini-3-flash-preview",
	}, GenerateInput{
		Messages: []Message{{Role: "user", Content: "Return text and image."}},
		Options: map[string]interface{}{
			"response_format": []map[string]interface{}{
				{"type": "text"},
				{"type": "image", "image_size": "2K"},
			},
		},
	})
	if err != nil {
		t.Fatalf("build Gemini interaction typed response format body: %v", err)
	}
	formats, ok := payload["response_format"].([]interface{})
	if !ok || len(formats) != 2 {
		t.Fatalf("expected response_format array, got %#v", payload["response_format"])
	}
	imageFormat := asMap(formats[1])
	if imageFormat["type"] != "image" || imageFormat["image_size"] != "2K" {
		t.Fatalf("unexpected typed response_format image entry: %#v", imageFormat)
	}
}

func TestBuildGeminiInteractionRequestBodyNormalizesVideoOptions(t *testing.T) {
	payload, err := buildGeminiInteractionRequestBody(RouteConfig{
		Endpoint:      EndpointInteractions,
		UpstreamModel: "gemini-omni-flash-preview",
	}, GenerateInput{
		Messages: []Message{{Role: "user", Content: "Edit this video."}},
		Options: map[string]interface{}{
			"response_format": map[string]interface{}{"type": "video", "aspect_ratio": "1:1"},
			"generation_config": map[string]interface{}{
				"video_config": map[string]interface{}{"task": "edit"},
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
	payload, err := buildGeminiInteractionRequestBody(RouteConfig{
		Endpoint:      EndpointInteractions,
		UpstreamModel: "gemini-3.5-flash",
	}, GenerateInput{
		Instructions: "Be concise.",
		Messages: []Message{
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
	steps, ok := payload["input"].([]map[string]interface{})
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
		"output": [
			{"type": "video", "fileData": {"fileUri": "https://example.com/video.mp4", "mimeType": "video/mp4"}},
			{"type": "video", "file_data": {"file_uri": "https://example.com/video.mp4", "mime_type": "video/mp4"}},
			{"type": "video", "inlineData": {"data": "` + inline + `", "mimeType": "video/webm"}}
		],
		"usageMetadata": {"promptTokenCount": 3, "candidatesTokenCount": 5}
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

func TestParseGeminiInteractionOutputExtractsTextAndImages(t *testing.T) {
	inline := base64.StdEncoding.EncodeToString([]byte("png"))
	inputInline := base64.StdEncoding.EncodeToString([]byte("source"))
	body := []byte(`{
		"id": "interaction-2",
		"steps": [
			{"type": "user_input", "content": [
				{"type": "image", "inlineData": {"data": "` + inputInline + `", "mimeType": "image/png"}}
			]},
			{"type": "model_output", "content": [
				{"type": "text", "text": "A revised prompt"},
				{"type": "image", "inlineData": {"data": "` + inline + `", "mimeType": "image/png"}},
				{"type": "image", "fileData": {"fileUri": "https://example.com/image.png", "mimeType": "image/png"}}
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
			{"type": "model_output", "content": "Let me check."},
			{"type": "function_call", "id": "call_weather", "name": "get_weather", "arguments": {"location": "Paris"}},
			{"type": "model_output", "content": [
				{"type": "function_call", "id": "call_weather", "name": "get_weather", "arguments": {"location": "Paris"}}
			]}
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
		t.Fatalf("expected deduped function call, got %#v", output.ToolCalls)
	}
	call := output.ToolCalls[0]
	if call.ToolCallID != "call_weather" || call.ToolType != "function" || call.ToolName != "get_weather" || call.Status != "requested" {
		t.Fatalf("unexpected function call: %#v", call)
	}
	if call.ArgumentsJSON != `{"location":"Paris"}` {
		t.Fatalf("unexpected arguments: %s", call.ArgumentsJSON)
	}
}

func TestGenerateGeminiInteractionPostsInteractionsRequest(t *testing.T) {
	var capturedPath string
	var capturedPayload map[string]interface{}
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
		_, _ = w.Write([]byte(`{"id":"interaction-1","output":[{"fileData":{"fileUri":"https://example.com/video.mp4","mimeType":"video/mp4"}}]}`))
	}))
	defer server.Close()

	output, err := NewClient().generateGeminiInteraction(context.Background(), RouteConfig{
		BaseURL:       server.URL,
		APIKey:        "test-key",
		UpstreamModel: "gemini-omni-flash-preview",
	}, GenerateInput{
		Messages: []Message{{Role: "user", Content: "Make a short video"}},
		Options:  map[string]interface{}{"response_format": map[string]interface{}{"type": "video"}},
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
	var capturedPayload map[string]interface{}
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
		_, _ = w.Write([]byte(`data: {"type":"interaction.created","interaction":{"id":"interaction-stream-1"}}

data: {"type":"step.delta","interaction_id":"interaction-stream-1","delta":{"type":"text","text":"Hello"}}

data: {"type":"step.delta","interaction_id":"interaction-stream-1","delta":{"type":"text","text":" world"}}

data: {"type":"interaction.completed","usage_metadata":{"prompt_token_count":4,"candidates_token_count":2},"interaction":{"id":"interaction-stream-1","output":[{"type":"text","text":"Hello world"}]}}

data: {"type":"done"}

`))
	}))
	defer server.Close()

	var deltas []string
	var usageEvents []Usage
	output, err := NewClient().GenerateStream(context.Background(), RouteConfig{
		Protocol:      AdapterGeminiInteractions,
		BaseURL:       server.URL,
		APIKey:        "test-key",
		UpstreamModel: "gemini-3.5-flash",
	}, GenerateInput{
		Messages: []Message{{Role: "user", Content: "How does AI work?"}},
	}, func(event GenerateStreamEvent) error {
		if event.Delta != "" {
			deltas = append(deltas, event.Delta)
		}
		if event.Usage != (Usage{}) {
			usageEvents = append(usageEvents, event.Usage)
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
	if len(usageEvents) != 1 || output.Usage.InputTokens != 4 || output.Usage.OutputTokens != 2 {
		t.Fatalf("unexpected stream usage events=%#v output=%#v", usageEvents, output.Usage)
	}
}

func TestNewGeminiRequestUsesOnlyGoogleAPIKeyForOfficialHost(t *testing.T) {
	req, err := NewClient().newGeminiRequest(context.Background(), http.MethodPost, "https://generativelanguage.googleapis.com/v1beta/interactions", nil, RouteConfig{
		APIKey: "test-key",
	})
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
