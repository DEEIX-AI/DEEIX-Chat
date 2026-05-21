package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildOpenAIImageGenerationRequestBody(t *testing.T) {
	payload, err := buildOpenAIImageGenerationRequestBody("gpt-image-1", GenerateInput{
		Messages: []Message{
			{Role: "system", Content: "ignore"},
			{Role: "user", Content: "A clean product render"},
		},
		Options: map[string]interface{}{
			"size":               "1024x1024",
			"quality":            "high",
			"response_format":    "b64_json",
			"output_format":      "webp",
			"output_compression": 80,
			"partial_images":     2,
			"stream":             true,
			"prompt":             "override",
		},
	})
	if err != nil {
		t.Fatalf("build image request body: %v", err)
	}
	if payload["model"] != "gpt-image-1" || payload["prompt"] != "A clean product render" {
		t.Fatalf("unexpected model or prompt: %#v", payload)
	}
	if payload["size"] != "1024x1024" || payload["quality"] != "high" {
		t.Fatalf("expected official image params, got %#v", payload)
	}
	if payload["output_format"] != "webp" || payload["output_compression"] != 80 {
		t.Fatalf("expected output params, got %#v", payload)
	}
	if _, ok := payload["response_format"]; ok {
		t.Fatalf("response_format must not be sent for gpt-image models: %#v", payload)
	}
	if _, ok := payload["stream"]; ok {
		t.Fatalf("stream must not be passed by non-streaming image adapter: %#v", payload)
	}
	if _, ok := payload["partial_images"]; ok {
		t.Fatalf("partial_images must not be passed without upstream image streaming: %#v", payload)
	}
}

func TestBuildOpenAIImageGenerationStreamRequestBody(t *testing.T) {
	payload, err := buildOpenAIImageGenerationStreamRequestBody("gpt-image-1", GenerateInput{
		Messages: []Message{{Role: "user", Content: "A clean product render"}},
		Options: map[string]interface{}{
			"output_format":  "webp",
			"partial_images": 2,
		},
	})
	if err != nil {
		t.Fatalf("build image stream request body: %v", err)
	}
	if payload["stream"] != true || payload["partial_images"] != 2 {
		t.Fatalf("expected stream params, got %#v", payload)
	}
}

func TestBuildOpenAIImageGenerationStreamRequestBodyDefaultsPartialImages(t *testing.T) {
	payload, err := buildOpenAIImageGenerationStreamRequestBody("gpt-image-1", GenerateInput{
		Messages: []Message{{Role: "user", Content: "A clean product render"}},
	})
	if err != nil {
		t.Fatalf("build image stream request body: %v", err)
	}
	if payload["partial_images"] != 1 {
		t.Fatalf("expected default partial_images=1, got %#v", payload)
	}
}

func TestBuildOpenAIImageGenerationRequestBodyDallEParams(t *testing.T) {
	payload, err := buildOpenAIImageGenerationRequestBody("dall-e-3", GenerateInput{
		Messages: []Message{{Role: "user", Content: "A clean product render"}},
		Options: map[string]interface{}{
			"response_format":    "url",
			"style":              "natural",
			"output_format":      "webp",
			"output_compression": 80,
			"background":         "transparent",
			"moderation":         "low",
		},
	})
	if err != nil {
		t.Fatalf("build image request body: %v", err)
	}
	if payload["response_format"] != "url" || payload["style"] != "natural" {
		t.Fatalf("expected DALL-E params, got %#v", payload)
	}
	for _, key := range []string{"output_format", "output_compression", "background", "moderation"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("expected GPT-image-only param %q to be omitted for DALL-E, got %#v", key, payload)
		}
	}
}

func TestParseOpenAIImageGenerationOutput(t *testing.T) {
	output, err := parseOpenAIImageGenerationOutput([]byte(`{
		"created": 1713833628,
		"data": [
			{"url": "https://example.com/a.png", "revised_prompt": "A revised render"},
			{"b64_json": "aGVsbG8="}
		],
		"usage": {
			"input_tokens": 12,
			"output_tokens": 40,
			"input_tokens_details": {"text_tokens": 8, "image_tokens": 4},
			"output_tokens_details": {"image_tokens": 40}
		}
	}`), "webp")
	if err != nil {
		t.Fatalf("parse image output: %v", err)
	}
	if output.Text != "" {
		t.Fatalf("image adapter must not put generated image data into text, got %q", output.Text)
	}
	if len(output.Citations) != 1 || output.Citations[0] != "https://example.com/a.png" {
		t.Fatalf("expected URL citation, got %#v", output.Citations)
	}
	if len(output.GeneratedImages) != 2 {
		t.Fatalf("expected generated image metadata, got %#v", output.GeneratedImages)
	}
	if output.GeneratedImages[0].URL != "https://example.com/a.png" || output.GeneratedImages[0].MIMEType != "image/webp" {
		t.Fatalf("unexpected URL image metadata: %#v", output.GeneratedImages[0])
	}
	if output.GeneratedImages[1].B64JSON != "aGVsbG8=" || output.GeneratedImages[1].MIMEType != "image/webp" {
		t.Fatalf("unexpected b64 image metadata: %#v", output.GeneratedImages[1])
	}
	if output.Usage.InputTokens != 12 || output.Usage.OutputTokens != 40 {
		t.Fatalf("expected parsed upstream image usage, got %#v", output.Usage)
	}
}

func TestParseOpenAIImageGenerationOutputDoesNotInventUsage(t *testing.T) {
	output, err := parseOpenAIImageGenerationOutput([]byte(`{
		"data": [{"b64_json": "aGVsbG8="}]
	}`), "png")
	if err != nil {
		t.Fatalf("parse image output: %v", err)
	}
	if output.Usage != (Usage{}) {
		t.Fatalf("expected missing upstream usage to remain empty, got %#v", output.Usage)
	}
}

func TestOpenAIImageGenerationStream(t *testing.T) {
	var requestPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("expected event stream accept header, got %q", r.Header.Get("Accept"))
		}
		if err := json.NewDecoder(r.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: image_generation.partial_image\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"image_generation.partial_image\",\"partial_image_index\":1,\"b64_json\":\"cGFydGlhbA==\"}\n\n"))
		_, _ = w.Write([]byte("event: image_generation.completed\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"image_generation.completed\",\"id\":\"img_1\",\"b64_json\":\"ZmluYWw=\",\"revised_prompt\":\"A final render\",\"usage\":{\"input_tokens\":12,\"output_tokens\":40}}\n\n"))
	}))
	defer server.Close()

	client := NewClient()
	var partials []GenerateStreamEvent
	output, err := client.GenerateStream(context.Background(), RouteConfig{
		Protocol:      AdapterOpenAIImageGenerations,
		BaseURL:       server.URL,
		UpstreamModel: "gpt-image-1",
	}, GenerateInput{
		Messages: []Message{{Role: "user", Content: "A clean product render"}},
		Options: map[string]interface{}{
			"output_format":  "webp",
			"partial_images": 2,
		},
	}, func(event GenerateStreamEvent) error {
		if event.GeneratedImage != nil {
			partials = append(partials, event)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("generate image stream: %v", err)
	}
	if requestPayload["stream"] != true || requestPayload["partial_images"] != float64(2) {
		t.Fatalf("expected stream request payload, got %#v", requestPayload)
	}
	if len(partials) != 1 || partials[0].GeneratedImage == nil || partials[0].GeneratedImage.B64JSON != "cGFydGlhbA==" {
		t.Fatalf("expected partial image event, got %#v", partials)
	}
	if partials[0].GeneratedImageIndex != 1 || !partials[0].GeneratedImagePartial {
		t.Fatalf("unexpected partial metadata: %#v", partials[0])
	}
	if len(output.GeneratedImages) != 1 || output.GeneratedImages[0].B64JSON != "ZmluYWw=" {
		t.Fatalf("expected final generated image, got %#v", output.GeneratedImages)
	}
	if output.GeneratedImages[0].MIMEType != "image/webp" || output.GeneratedImages[0].RevisedPrompt != "A final render" {
		t.Fatalf("unexpected final image metadata: %#v", output.GeneratedImages[0])
	}
	if output.Usage.InputTokens != 12 || output.Usage.OutputTokens != 40 {
		t.Fatalf("expected upstream stream usage, got %#v", output.Usage)
	}
}

func TestOpenAIImageGenerationStreamFallsBackToJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/generations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "img_json_1",
			"data": [{"b64_json": "ZmluYWw="}],
			"usage": {"input_tokens": 12, "output_tokens": 40}
		}`))
	}))
	defer server.Close()

	client := NewClient()
	var usageEvents []Usage
	output, err := client.GenerateStream(context.Background(), RouteConfig{
		Protocol:      AdapterOpenAIImageGenerations,
		BaseURL:       server.URL,
		UpstreamModel: "gpt-image-1",
	}, GenerateInput{
		Messages: []Message{{Role: "user", Content: "A clean product render"}},
	}, func(event GenerateStreamEvent) error {
		if event.Usage != (Usage{}) {
			usageEvents = append(usageEvents, event.Usage)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("generate image stream json fallback: %v", err)
	}
	if len(output.GeneratedImages) != 1 || output.GeneratedImages[0].B64JSON != "ZmluYWw=" {
		t.Fatalf("expected json fallback image, got %#v", output.GeneratedImages)
	}
	if output.ResponseID != "img_json_1" {
		t.Fatalf("expected response id from json fallback, got %q", output.ResponseID)
	}
	if output.Usage.InputTokens != 12 || output.Usage.OutputTokens != 40 {
		t.Fatalf("expected parsed json fallback usage, got %#v", output.Usage)
	}
	if len(usageEvents) != 1 || usageEvents[0].InputTokens != 12 || usageEvents[0].OutputTokens != 40 {
		t.Fatalf("expected usage event from json fallback, got %#v", usageEvents)
	}
}
