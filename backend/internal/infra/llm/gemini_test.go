package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseGeminiResponseReasoningAndCitations(t *testing.T) {
	result, err := parseGeminiResponse([]byte(`{
		"responseId": "gemini-response-1",
		"candidates": [
			{
				"content": {
					"parts": [
						{"text": "internal reasoning", "thought": true, "thoughtSignature": "sig-1"},
						{"text": "final answer"}
					]
				},
				"groundingMetadata": {
					"groundingChunks": [
						{"web": {"uri": "https://example.com/a", "title": "A"}},
						{"retrievedContext": {"uri": "https://example.com/b"}}
					]
				},
				"urlContextMetadata": {
					"urlMetadata": [
						{"retrievedUrl": "https://example.com/c"}
					]
				}
			}
		]
	}`))
	if err != nil {
		t.Fatalf("parse gemini response: %v", err)
	}
	if result.Text != "final answer" {
		t.Fatalf("expected final answer without thought text, got %#v", result.Text)
	}
	if result.Reasoning == nil || result.Reasoning.Text != "internal reasoning" || result.Reasoning.Signature != "sig-1" {
		t.Fatalf("expected Gemini reasoning output, got %#v", result.Reasoning)
	}
	if len(result.Citations) != 3 || result.Citations[0] != "https://example.com/a" || result.Citations[2] != "https://example.com/c" {
		t.Fatalf("expected Gemini citations, got %#v", result.Citations)
	}
}

func TestApplyGeminiStreamChunkStoresReasoningAndCitations(t *testing.T) {
	result := &GenerateOutput{ToolCalls: make([]ToolCall, 0)}
	var reasoningText string
	err := applyGeminiStreamChunk(mustDecodeObject(t, `{
		"responseId": "gemini-stream-1",
		"candidates": [
			{
				"content": {
					"parts": [
						{"text": "think", "thought": true, "thoughtSignature": "sig-stream"},
						{"text": "answer"}
					]
				},
				"groundingMetadata": {
					"groundingChunks": [
						{"web": {"uri": "https://example.com/source"}}
					]
				}
			}
		]
	}`), result, func(event GenerateStreamEvent) error {
		if event.Reasoning != nil {
			reasoningText += event.Reasoning.Text
		}
		return nil
	})
	if err != nil {
		t.Fatalf("apply gemini stream chunk: %v", err)
	}
	if result.ResponseID != "gemini-stream-1" || result.Text != "answer" {
		t.Fatalf("expected response id and answer text, got %#v", result)
	}
	if reasoningText != "think" || result.Reasoning == nil || result.Reasoning.Text != "think" || result.Reasoning.Signature != "sig-stream" {
		t.Fatalf("expected stored and emitted reasoning, got text=%q result=%#v", reasoningText, result.Reasoning)
	}
	if len(result.Citations) != 1 || result.Citations[0] != "https://example.com/source" {
		t.Fatalf("expected stream citations, got %#v", result.Citations)
	}
}

func TestBuildGeminiImageGenerationRequestBody(t *testing.T) {
	payload, err := buildGeminiImageGenerationRequestBody("gemini-3.1-flash-image", GenerateInput{
		Messages: []Message{
			{Role: "system", Content: "ignore"},
			{Role: "user", Content: "A clean product render"},
		},
		Options: map[string]interface{}{
			"generationConfig": map[string]interface{}{
				"imageConfig": map[string]interface{}{
					"aspectRatio": "16:9",
					"imageSize":   "2K",
				},
			},
			"prompt": "override",
			"stream": true,
		},
	})
	if err != nil {
		t.Fatalf("build gemini image request body: %v", err)
	}
	contents := payload["contents"].([]map[string]interface{})
	parts := contents[0]["parts"].([]map[string]interface{})
	if parts[0]["text"] != "A clean product render" {
		t.Fatalf("expected last user prompt, got %#v", payload)
	}
	config := payload["generationConfig"].(map[string]interface{})
	modalities := config["responseModalities"].([]string)
	if len(modalities) != 2 || modalities[0] != "TEXT" || modalities[1] != "IMAGE" {
		t.Fatalf("expected default image response modalities, got %#v", config["responseModalities"])
	}
	imageConfig := asMap(config["imageConfig"])
	if imageConfig["aspectRatio"] != "16:9" || imageConfig["imageSize"] != "2K" {
		t.Fatalf("expected image config, got %#v", config)
	}
	if _, ok := config["responseFormat"]; ok {
		t.Fatalf("Gemini image requests must use generationConfig.imageConfig, got %#v", config["responseFormat"])
	}
	if _, ok := payload["stream"]; ok {
		t.Fatalf("stream must not be passed to Gemini image generation: %#v", payload)
	}
}

func TestBuildGeminiImageGenerationRequestBodySupportsResponseModalitiesAndTools(t *testing.T) {
	payload, err := buildGeminiImageGenerationRequestBody("gemini-3-pro-image", GenerateInput{
		Messages: []Message{{Role: "user", Content: "A clean product render"}},
		Options: map[string]interface{}{
			"generationConfig": map[string]interface{}{
				"responseModalities": "IMAGE",
			},
			"tools": []interface{}{
				map[string]interface{}{"google_search": map[string]interface{}{}},
			},
		},
	})
	if err != nil {
		t.Fatalf("build gemini image request body: %v", err)
	}
	config := payload["generationConfig"].(map[string]interface{})
	modalities := config["responseModalities"].([]string)
	if len(modalities) != 1 || modalities[0] != "IMAGE" {
		t.Fatalf("expected configured image-only modality, got %#v", modalities)
	}
	tools := payload["tools"].([]map[string]interface{})
	if len(tools) != 1 || len(asMap(tools[0]["google_search"])) != 0 {
		t.Fatalf("expected google_search tool, got %#v", tools)
	}
}

func TestBuildGeminiImageGenerationRequestBodyPreservesGoogleImageSearchOptions(t *testing.T) {
	payload, err := buildGeminiImageGenerationRequestBody("gemini-3-pro-image", GenerateInput{
		Messages: []Message{{Role: "user", Content: "Find current product imagery"}},
		Options: map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"google_search": map[string]interface{}{
						"searchTypes": map[string]interface{}{
							"webSearch":   map[string]interface{}{},
							"imageSearch": map[string]interface{}{},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build gemini image request body: %v", err)
	}
	tools := payload["tools"].([]map[string]interface{})
	googleSearch := asMap(tools[0]["google_search"])
	searchTypes := asMap(googleSearch["searchTypes"])
	if _, ok := searchTypes["webSearch"]; !ok {
		t.Fatalf("expected webSearch to pass, got %#v", tools)
	}
	if _, ok := searchTypes["imageSearch"]; !ok {
		t.Fatalf("expected imageSearch to pass, got %#v", tools)
	}
}

func TestParseGeminiResponseCapturesGoogleSearchUsage(t *testing.T) {
	output, err := parseGeminiResponse([]byte(`{
		"candidates": [{
			"content": {"parts": [{"text": "answer"}]},
			"groundingMetadata": {
				"webSearchQueries": ["latest docs"],
				"groundingChunks": [{"web": {"uri": "https://example.com"}}]
			}
		}]
	}`))
	if err != nil {
		t.Fatalf("parse Gemini response: %v", err)
	}
	if output.ServerSideToolUsage["google_search"] != 1 {
		t.Fatalf("expected google_search server-side usage, got %#v", output.ServerSideToolUsage)
	}
}

func TestBuildGeminiImageGenerationRequestBodyDropsUnsupportedImageSize(t *testing.T) {
	payload, err := buildGeminiImageGenerationRequestBody("gemini-2.5-flash-image", GenerateInput{
		Messages: []Message{{Role: "user", Content: "A clean product render"}},
		Options: map[string]interface{}{
			"aspectRatio": "1:1",
			"imageSize":   "1K",
			"generationConfig": map[string]interface{}{
				"imageConfig": map[string]interface{}{
					"aspectRatio": "1:1",
					"imageSize":   "1K",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build gemini image request body: %v", err)
	}
	imageConfig := asMap(payload["generationConfig"].(map[string]interface{})["imageConfig"])
	if imageConfig["aspectRatio"] != "1:1" {
		t.Fatalf("expected supported aspect ratio, got %#v", imageConfig)
	}
	if _, ok := imageConfig["imageSize"]; ok {
		t.Fatalf("expected imageSize to be omitted for Gemini 2.5 image model, got %#v", imageConfig)
	}
}

func TestBuildGeminiImageGenerationRequestBodyIncludesInlineImages(t *testing.T) {
	payload, err := buildGeminiImageGenerationRequestBody("gemini-3.1-flash-image", GenerateInput{
		Messages: []Message{
			{
				Role: "user",
				Parts: []ContentPart{
					{Kind: ContentPartText, Text: "Replace the background"},
					{Kind: ContentPartImage, MimeType: "image/png", Data: []byte("source")},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build gemini image edit request body: %v", err)
	}

	contents := payload["contents"].([]map[string]interface{})
	parts := contents[0]["parts"].([]map[string]interface{})
	if len(parts) != 2 {
		t.Fatalf("expected text and image parts, got %#v", parts)
	}
	if parts[0]["text"] != "Replace the background" {
		t.Fatalf("expected prompt text part, got %#v", parts[0])
	}
	inlineData := asMap(parts[1]["inline_data"])
	if inlineData["mime_type"] != "image/png" || inlineData["data"] != "c291cmNl" {
		t.Fatalf("expected inline image data, got %#v", inlineData)
	}
}

func TestParseGeminiImageGenerationOutput(t *testing.T) {
	output, err := parseGeminiImageGenerationOutput([]byte(`{
		"responseId": "gemini-image-1",
		"candidates": [
			{
				"content": {
					"parts": [
						{"text": "A revised prompt"},
						{"thought": true, "inlineData": {"mimeType": "image/png", "data": "thought-image"}},
						{"inlineData": {"mimeType": "image/png", "data": "iVBORw0KGgo="}}
					]
				}
			},
			{
				"content": {
					"parts": [
						{"inline_data": {"mime_type": "image/webp", "data": "UklGRg=="}}
					]
				}
			}
		],
		"usageMetadata": {
			"promptTokenCount": 15,
			"candidatesTokenCount": 7,
			"cachedContentTokenCount": 3
		}
	}`))
	if err != nil {
		t.Fatalf("parse gemini image output: %v", err)
	}
	if output.ResponseID != "gemini-image-1" {
		t.Fatalf("expected response id, got %q", output.ResponseID)
	}
	if len(output.GeneratedImages) != 2 {
		t.Fatalf("expected generated images, got %#v", output.GeneratedImages)
	}
	if output.GeneratedImages[0].B64JSON != "iVBORw0KGgo=" || output.GeneratedImages[0].MIMEType != "image/png" {
		t.Fatalf("unexpected first image: %#v", output.GeneratedImages[0])
	}
	if output.GeneratedImages[1].B64JSON != "UklGRg==" || output.GeneratedImages[1].MIMEType != "image/webp" {
		t.Fatalf("unexpected second image: %#v", output.GeneratedImages[1])
	}
	if output.GeneratedImages[0].RevisedPrompt != "A revised prompt" {
		t.Fatalf("expected revised prompt, got %#v", output.GeneratedImages[0])
	}
	if output.Usage.InputTokens != 12 || output.Usage.OutputTokens != 7 || output.Usage.CacheReadTokens != 3 {
		t.Fatalf("expected parsed usage, got %#v", output.Usage)
	}
}

func TestGeminiImageGenerationStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/nano-banana-pro:streamGenerateContent" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "gemini-key" {
			t.Fatalf("expected gemini API key header, got %q", r.Header.Get("x-goog-api-key"))
		}
		var requestBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		generationConfig := asMap(requestBody["generationConfig"])
		modalities := generationConfig["responseModalities"].([]interface{})
		if len(modalities) != 2 || modalities[0] != "TEXT" || modalities[1] != "IMAGE" {
			t.Fatalf("expected text and image modalities, got %#v", modalities)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"responseId\":\"gemini-image-stream-1\",\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"A revised prompt\"}]}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"inlineData\":{\"mimeType\":\"image/png\",\"data\":\"iVBORw0KGgo=\"}}]}}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":3}}\n\n"))
	}))
	defer server.Close()

	var usageEvents []Usage
	output, err := NewClient().GenerateStream(context.Background(), RouteConfig{
		Protocol:      AdapterGoogleImageGeneration,
		BaseURL:       server.URL,
		APIKey:        "gemini-key",
		UpstreamModel: "nano-banana-pro",
	}, GenerateInput{
		Messages: []Message{{Role: "user", Content: "A clean product render"}},
	}, func(event GenerateStreamEvent) error {
		if event.Usage != (Usage{}) {
			usageEvents = append(usageEvents, event.Usage)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("gemini image stream: %v", err)
	}
	if output.ResponseID != "gemini-image-stream-1" {
		t.Fatalf("expected response id, got %q", output.ResponseID)
	}
	if output.Text != "A revised prompt" {
		t.Fatalf("expected revised prompt text, got %q", output.Text)
	}
	if len(output.GeneratedImages) != 1 || output.GeneratedImages[0].B64JSON != "iVBORw0KGgo=" {
		t.Fatalf("expected streamed image, got %#v", output.GeneratedImages)
	}
	if output.GeneratedImages[0].MIMEType != "image/png" || output.GeneratedImages[0].RevisedPrompt != "A revised prompt" {
		t.Fatalf("unexpected streamed image metadata: %#v", output.GeneratedImages[0])
	}
	if len(usageEvents) != 1 || usageEvents[0].InputTokens != 10 || usageEvents[0].OutputTokens != 3 {
		t.Fatalf("expected usage event, got %#v", usageEvents)
	}
}

func TestParseGeminiErrorProvidesUnauthorizedFallback(t *testing.T) {
	err := parseGeminiError(http.StatusUnauthorized, nil, nil)
	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected upstream error, got %v", err)
	}
	if upstreamErr.Message != "google authentication failed; check API key, upstream base URL, and custom auth headers" {
		t.Fatalf("unexpected unauthorized message: %q", upstreamErr.Message)
	}
}
