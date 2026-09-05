package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestBuildXAIImageRequestBody(t *testing.T) {
	payload, err := buildXAIImageRequestBody("grok-imagine-image-quality", portllm.GenerateInput{
		Messages: []portllm.Message{
			{Role: "system", Content: "ignore"},
			{Role: "user", Content: "A clean product render"},
		},
		Options: map[string]any{
			"aspect_ratio":    "16:9",
			"n":               2,
			"resolution":      "2k",
			"response_format": "b64_json",
			"prompt":          "override",
			"stream":          true,
			"quality":         "high",
		},
	})
	if err != nil {
		t.Fatalf("build xAI image request body: %v", err)
	}
	if payload["model"] != "grok-imagine-image-quality" || payload["prompt"] != "A clean product render" {
		t.Fatalf("unexpected model or prompt: %#v", payload)
	}
	if payload["aspect_ratio"] != "16:9" || payload["n"] != 2 || payload["resolution"] != "2k" || payload["response_format"] != "b64_json" {
		t.Fatalf("expected xAI image params, got %#v", payload)
	}
	for _, key := range []string{"stream", "quality"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("unexpected xAI image param %q in payload %#v", key, payload)
		}
	}
}

func TestBuildXAIImageRequestBodyDropsUnsupportedParams(t *testing.T) {
	payload, err := buildXAIImageRequestBody("grok-imagine-image-quality", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
		Options: map[string]any{
			"aspect_ratio": "21:9",
			"n":            2.5,
			"resolution":   "4k",
		},
	})
	if err != nil {
		t.Fatalf("build xAI image request body: %v", err)
	}
	for _, key := range []string{"aspect_ratio", "n", "resolution"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("unsupported xAI image param %q must be removed: %#v", key, payload)
		}
	}
}

func TestBuildXAIImageRequestBodyPreservesOfficialDefaultResponseFormat(t *testing.T) {
	payload, err := buildXAIImageRequestBody("grok-imagine-image-quality", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
	})
	if err != nil {
		t.Fatalf("build xAI image request body: %v", err)
	}
	if _, ok := payload["response_format"]; ok {
		t.Fatalf("expected xAI to apply its documented URL default, got %#v", payload)
	}
}

func TestBuildXAIImageEditRequestBody(t *testing.T) {
	payload, debugBody, err := buildXAIImageEditRequestBody("grok-imagine-image-quality", portllm.GenerateInput{
		Messages: []portllm.Message{
			{
				Role: "user",
				Parts: []portllm.ContentPart{
					{Kind: portllm.ContentPartText, Text: "Render this as a pencil sketch"},
					{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("source")},
				},
			},
		},
		Options: map[string]any{
			"aspect_ratio": "1:1",
			"resolution":   "2k",
		},
	})
	if err != nil {
		t.Fatalf("build xAI image edit request body: %v", err)
	}
	if payload["model"] != "grok-imagine-image-quality" || payload["prompt"] != "Render this as a pencil sketch" {
		t.Fatalf("unexpected model or prompt: %#v", payload)
	}
	if payload["aspect_ratio"] != "1:1" || payload["resolution"] != "2k" {
		t.Fatalf("expected xAI edit params, got %#v", payload)
	}
	if _, ok := payload["response_format"]; ok {
		t.Fatalf("expected xAI to apply its documented URL default, got %#v", payload)
	}
	image := payload["image"].(map[string]any)
	if image["type"] != "image_url" {
		t.Fatalf("expected image_url type, got %#v", image)
	}
	if url := image["url"].(string); !strings.HasPrefix(url, "data:image/png;base64,c291cmNl") {
		t.Fatalf("expected base64 data uri, got %q", url)
	}
	if strings.Contains(string(debugBody), "c291cmNl") {
		t.Fatalf("debug body must not include source image bytes: %s", string(debugBody))
	}
}

func TestBuildXAIImageEditRequestBodyAllowsUpToThreeImages(t *testing.T) {
	payload, _, err := buildXAIImageEditRequestBody("grok-imagine-image-quality", portllm.GenerateInput{
		Messages: []portllm.Message{
			{
				Role: "user",
				Parts: []portllm.ContentPart{
					{Kind: portllm.ContentPartText, Text: "Combine these"},
					{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("one")},
					{Kind: portllm.ContentPartImage, MimeType: "image/jpeg", Data: []byte("two")},
					{Kind: portllm.ContentPartImage, MimeType: "image/webp", Data: []byte("three")},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build xAI multi-image edit request body: %v", err)
	}
	if _, ok := payload["image"]; ok {
		t.Fatalf("multi-reference edit must not send the singular image field: %#v", payload)
	}
	images := payload["images"].([]map[string]any)
	if len(images) != 3 {
		t.Fatalf("expected three ordered image inputs, got %#v", images)
	}
}

func TestGenerateXAIImageUsesImageEndpoint(t *testing.T) {
	var requestPath string
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if got := r.Header.Get("Authorization"); got != "Bearer xai-key" {
			t.Fatalf("unexpected auth header %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "img_xai_1",
			"data": [
				{"url": "https://example.com/generated.jpg"}
			],
			"usage": {"input_tokens": 11, "output_tokens": 1}
		}`))
	}))
	defer server.Close()

	output, err := newTestClient().Generate(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterXAIImage,
		BaseURL:       server.URL + "/v1",
		APIKey:        "xai-key",
		UpstreamModel: "grok-imagine-image-quality",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
		Options: map[string]any{
			"aspect_ratio": "16:9",
		},
	})
	if err != nil {
		t.Fatalf("generate xAI image: %v", err)
	}
	if requestPath != "/v1/images/generations" {
		t.Fatalf("expected xAI images endpoint, got %q", requestPath)
	}
	if requestBody["model"] != "grok-imagine-image-quality" || requestBody["aspect_ratio"] != "16:9" {
		t.Fatalf("unexpected request body: %#v", requestBody)
	}
	if output.ResponseID != "img_xai_1" || len(output.GeneratedImages) != 1 || output.GeneratedImages[0].URL != "https://example.com/generated.jpg" {
		t.Fatalf("unexpected generated image output: %#v", output)
	}
	if output.Usage.InputTokens != 11 || output.Usage.OutputTokens != 1 {
		t.Fatalf("expected upstream usage, got %#v", output.Usage)
	}
}

func TestGenerateXAIImageGenerationAdapterKeepsGenerationEndpoint(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://example.com/generated.jpg"}]}`))
	}))
	defer server.Close()

	_, err := newTestClient().Generate(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterXAIImage,
		Endpoint:      portllm.EndpointImageEdits,
		BaseURL:       server.URL + "/v1",
		UpstreamModel: "grok-imagine-image-quality",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
	})
	if err != nil {
		t.Fatalf("generate xAI image: %v", err)
	}
	if requestPath != "/v1/images/generations" {
		t.Fatalf("expected xAI image generation endpoint, got %q", requestPath)
	}
}

func TestGenerateXAIImageEditUsesImageEditsEndpoint(t *testing.T) {
	var requestPath string
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if got := r.Header.Get("Authorization"); got != "Bearer xai-key" {
			t.Fatalf("unexpected auth header %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "img_xai_edit_1",
			"data": [
				{"b64_json": "ZWRpdGVk", "revised_prompt": "Edited prompt"}
			],
			"usage": {"input_tokens": 13, "output_tokens": 2}
		}`))
	}))
	defer server.Close()

	output, err := newTestClient().Generate(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterXAIImageEdits,
		BaseURL:       server.URL + "/v1",
		APIKey:        "xai-key",
		UpstreamModel: "grok-imagine-image-quality",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{
			Role: "user",
			Parts: []portllm.ContentPart{
				{Kind: portllm.ContentPartText, Text: "Render this as a pencil sketch"},
				{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("source")},
			},
		}},
		Options: map[string]any{
			"response_format": "b64_json",
		},
	})
	if err != nil {
		t.Fatalf("generate xAI image edit: %v", err)
	}
	if requestPath != "/v1/images/edits" {
		t.Fatalf("expected xAI image edits endpoint, got %q", requestPath)
	}
	image := asMap(requestBody["image"])
	if image["type"] != "image_url" || !strings.HasPrefix(getString(image["url"]), "data:image/png;base64,c291cmNl") {
		t.Fatalf("unexpected edit image payload: %#v", requestBody)
	}
	if output.ResponseID != "img_xai_edit_1" || len(output.GeneratedImages) != 1 || output.GeneratedImages[0].B64JSON != "ZWRpdGVk" {
		t.Fatalf("unexpected edited image output: %#v", output)
	}
	if output.Usage.InputTokens != 13 || output.Usage.OutputTokens != 2 {
		t.Fatalf("expected upstream usage, got %#v", output.Usage)
	}
}

func TestParseXAIImageOutput(t *testing.T) {
	output, err := parseXAIImageOutput([]byte(`{
		"id": "img_xai_1",
		"data": [
			{"url": "https://example.com/a.jpg", "mime_type": "image/webp"},
			{"b64_json": "aGVsbG8=", "mime_type": "image/png", "revised_prompt": "A revised render"}
		]
	}`), portllm.AdapterXAIImage)
	if err != nil {
		t.Fatalf("parse xAI image output: %v", err)
	}
	if output.Text != "" {
		t.Fatalf("xAI image adapter must not write image data into text, got %q", output.Text)
	}
	if len(output.GeneratedImages) != 2 {
		t.Fatalf("expected two generated images, got %#v", output.GeneratedImages)
	}
	if output.GeneratedImages[0].URL != "https://example.com/a.jpg" || output.GeneratedImages[0].MIMEType != "image/webp" {
		t.Fatalf("unexpected URL image: %#v", output.GeneratedImages[0])
	}
	if output.GeneratedImages[1].B64JSON != "aGVsbG8=" || output.GeneratedImages[1].MIMEType != "image/png" {
		t.Fatalf("unexpected base64 image: %#v", output.GeneratedImages[1])
	}
	if len(output.Citations) != 1 || output.Citations[0] != "https://example.com/a.jpg" {
		t.Fatalf("expected URL citation, got %#v", output.Citations)
	}
}
