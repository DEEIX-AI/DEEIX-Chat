package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
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
	result := &portllm.GenerateOutput{ToolCalls: make([]portllm.ToolCall, 0)}
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
	}`), result, func(event portllm.GenerateStreamEvent) error {
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

func TestParseGeminiResponseCapturesFunctionCallThoughtSignature(t *testing.T) {
	output, err := parseGeminiResponse([]byte(`{
		"candidates": [{
			"content": {
				"parts": [{
					"functionCall": {
						"name": "search_web",
						"args": {"query": "SpaceX stock price"}
					},
					"thoughtSignature": "thought-signature-1"
				}]
			}
		}]
	}`))
	if err != nil {
		t.Fatalf("parse Gemini response: %v", err)
	}
	if len(output.ToolCalls) != 1 {
		t.Fatalf("expected one function call, got %#v", output.ToolCalls)
	}
	if output.ToolCalls[0].ThoughtSignature != "thought-signature-1" {
		t.Fatalf("expected thought signature, got %#v", output.ToolCalls[0])
	}
}

func TestBuildGeminiImageGenerationRequestBody(t *testing.T) {
	payload, err := buildGeminiImageGenerationRequestBody("gemini-3.1-flash-image", portllm.GenerateInput{
		Messages: []portllm.Message{
			{Role: "system", Content: "ignore"},
			{Role: "user", Content: "A clean product render"},
		},
		Options: map[string]any{
			"generationConfig": map[string]any{
				"imageConfig": map[string]any{
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
	contents := payload["contents"].([]map[string]any)
	parts := contents[0]["parts"].([]map[string]any)
	if parts[0]["text"] != "A clean product render" {
		t.Fatalf("expected last user prompt, got %#v", payload)
	}
	config := payload["generationConfig"].(map[string]any)
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
	payload, err := buildGeminiImageGenerationRequestBody("gemini-3-pro-image", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
		Options: map[string]any{
			"generationConfig": map[string]any{
				"responseModalities": "IMAGE",
			},
			"tools": []any{
				map[string]any{"google_search": map[string]any{}},
			},
		},
	})
	if err != nil {
		t.Fatalf("build gemini image request body: %v", err)
	}
	config := payload["generationConfig"].(map[string]any)
	modalities := config["responseModalities"].([]string)
	if len(modalities) != 1 || modalities[0] != "IMAGE" {
		t.Fatalf("expected configured image-only modality, got %#v", modalities)
	}
	tools := payload["tools"].([]map[string]any)
	if len(tools) != 1 || len(asMap(tools[0]["google_search"])) != 0 {
		t.Fatalf("expected google_search tool, got %#v", tools)
	}
}

func TestBuildGeminiImageGenerationRequestBodyPreservesGoogleImageSearchOptions(t *testing.T) {
	payload, err := buildGeminiImageGenerationRequestBody("gemini-3-pro-image", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "Find current product imagery"}},
		Options: map[string]any{
			"tools": []any{
				map[string]any{
					"google_search": map[string]any{
						"searchTypes": map[string]any{
							"webSearch":   map[string]any{},
							"imageSearch": map[string]any{},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build gemini image request body: %v", err)
	}
	tools := payload["tools"].([]map[string]any)
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
	if len(output.ServerToolCalls) != 1 {
		t.Fatalf("expected google_search server-side tool trace, got %#v", output.ServerToolCalls)
	}
	call := output.ServerToolCalls[0]
	if call.ToolName != "google_search" || call.ToolType != "google_search" || call.Status != "completed" {
		t.Fatalf("expected completed google_search trace, got %#v", call)
	}
	if call.ArgumentsJSON == "" || call.OutputJSON == "" {
		t.Fatalf("expected google_search input and output details, got %#v", call)
	}
	outputPayload := mustDecodeObject(t, call.OutputJSON)
	if _, ok := outputPayload["groundingChunks"]; ok {
		t.Fatalf("expected compact Gemini search output, got %#v", outputPayload)
	}
	sources := asSlice(outputPayload["sources"])
	if len(sources) != 1 || asMap(sources[0])["url"] != "https://example.com" {
		t.Fatalf("expected compact Gemini sources, got %#v", outputPayload)
	}
}

func TestParseGeminiResponseCapturesURLContextServerToolTrace(t *testing.T) {
	output, err := parseGeminiResponse([]byte(`{
		"candidates": [{
			"content": {"parts": [{"text": "answer"}]},
			"urlContextMetadata": {
				"urlMetadata": [{
					"retrievedUrl": "https://example.com/page",
					"urlRetrievalStatus": "URL_RETRIEVAL_STATUS_SUCCESS"
				}]
			}
		}]
	}`))
	if err != nil {
		t.Fatalf("parse Gemini response: %v", err)
	}
	if output.ServerSideToolUsage["url_context"] != 1 {
		t.Fatalf("expected url_context server-side usage, got %#v", output.ServerSideToolUsage)
	}
	if len(output.ServerToolCalls) != 1 {
		t.Fatalf("expected url_context server-side tool trace, got %#v", output.ServerToolCalls)
	}
	call := output.ServerToolCalls[0]
	if call.ToolName != "url_context" || call.ToolType != "url_context" || call.Status != "completed" {
		t.Fatalf("expected completed url_context trace, got %#v", call)
	}
	if call.ArgumentsJSON == "" || call.OutputJSON == "" {
		t.Fatalf("expected url_context input and output details, got %#v", call)
	}
	outputPayload := mustDecodeObject(t, call.OutputJSON)
	urls := asSlice(outputPayload["urls"])
	if len(urls) != 1 || asMap(urls[0])["url"] != "https://example.com/page" {
		t.Fatalf("expected compact url_context output, got %#v", outputPayload)
	}
}

func TestParseGeminiResponseCapturesCodeExecutionServerToolTrace(t *testing.T) {
	output, err := parseGeminiResponse([]byte(`{
		"candidates": [{
			"content": {
				"parts": [
					{
						"executableCode": {
							"language": "PYTHON",
							"code": "print(1 + 1)"
						}
					},
					{
						"codeExecutionResult": {
							"outcome": "OUTCOME_OK",
							"output": "2\n"
						}
					},
					{"text": "answer"}
				]
			}
		}]
	}`))
	if err != nil {
		t.Fatalf("parse Gemini response: %v", err)
	}
	if output.ServerSideToolUsage["code_execution"] != 1 {
		t.Fatalf("expected code_execution server-side usage, got %#v", output.ServerSideToolUsage)
	}
	if len(output.ServerToolCalls) != 1 {
		t.Fatalf("expected code_execution server-side tool trace, got %#v", output.ServerToolCalls)
	}
	call := output.ServerToolCalls[0]
	if call.ToolName != "code_execution" || call.ToolType != "code_execution" || call.Status != "completed" {
		t.Fatalf("expected completed code_execution trace, got %#v", call)
	}
	if call.ArgumentsJSON == "" || call.OutputJSON == "" || call.ErrorJSON != "" {
		t.Fatalf("expected code_execution input and successful output, got %#v", call)
	}
}

func TestApplyGeminiStreamChunkEmitsServerToolTrace(t *testing.T) {
	result := &portllm.GenerateOutput{ToolCalls: make([]portllm.ToolCall, 0)}
	events := make([]portllm.ToolCall, 0)
	err := applyGeminiStreamChunk(mustDecodeObject(t, `{
		"responseId": "gemini-stream-tool-1",
		"candidates": [{
			"content": {
				"parts": [
					{"executableCode": {"language": "PYTHON", "code": "print(2)"}},
					{"codeExecutionResult": {"outcome": "OUTCOME_OK", "output": "2\n"}}
				]
			}
		}]
	}`), result, func(event portllm.GenerateStreamEvent) error {
		if event.ServerToolCall != nil {
			events = append(events, *event.ServerToolCall)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("apply gemini stream chunk: %v", err)
	}
	if len(result.ServerToolCalls) != 1 || result.ServerToolCalls[0].ToolName != "code_execution" {
		t.Fatalf("expected stored code_execution trace, got %#v", result.ServerToolCalls)
	}
	if len(events) != 1 || events[0].ToolName != "code_execution" || events[0].OutputJSON == "" {
		t.Fatalf("expected emitted code_execution trace, got %#v", events)
	}
}

func TestApplyGeminiStreamChunkEmitsExplicitServerToolInvocation(t *testing.T) {
	result := &portllm.GenerateOutput{ToolCalls: make([]portllm.ToolCall, 0)}
	events := make([]portllm.ToolCall, 0)
	err := applyGeminiStreamChunk(mustDecodeObject(t, `{
		"responseId": "gemini-stream-tool-2",
		"candidates": [{
			"serverSideToolInvocations": [{
				"id": "gsi_1",
				"name": "google_search",
				"status": "running",
				"input": {"query": "SpaceX stock price"}
			}]
		}]
	}`), result, func(event portllm.GenerateStreamEvent) error {
		if event.ServerToolCall != nil {
			events = append(events, *event.ServerToolCall)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("apply gemini stream chunk: %v", err)
	}
	if len(result.ServerToolCalls) != 1 || result.ServerToolCalls[0].Status != "in_progress" {
		t.Fatalf("expected stored in-progress server tool call, got %#v", result.ServerToolCalls)
	}
	if len(events) != 1 || events[0].ToolName != "google_search" || events[0].Status != "in_progress" {
		t.Fatalf("expected emitted google_search invocation, got %#v", events)
	}
}

func TestApplyGeminiStreamChunkKeepsMultipleCodeExecutionTraces(t *testing.T) {
	result := &portllm.GenerateOutput{ToolCalls: make([]portllm.ToolCall, 0)}
	chunks := []string{
		`{"candidates":[{"content":{"parts":[{"executableCode":{"language":"PYTHON","code":"print(1)"}}]}}]}`,
		`{"candidates":[{"content":{"parts":[{"codeExecutionResult":{"outcome":"OUTCOME_OK","output":"1\n"}}]}}]}`,
		`{"candidates":[{"content":{"parts":[{"executableCode":{"language":"PYTHON","code":"print(2)"}}]}}]}`,
		`{"candidates":[{"content":{"parts":[{"codeExecutionResult":{"outcome":"OUTCOME_OK","output":"2\n"}}]}}]}`,
	}
	for _, chunk := range chunks {
		if err := applyGeminiStreamChunk(mustDecodeObject(t, chunk), result, nil); err != nil {
			t.Fatalf("apply gemini stream chunk: %v", err)
		}
	}
	if len(result.ServerToolCalls) != 2 {
		t.Fatalf("expected two code_execution traces, got %#v", result.ServerToolCalls)
	}
	if result.ServerToolCalls[0].ToolCallID == result.ServerToolCalls[1].ToolCallID {
		t.Fatalf("expected distinct code_execution trace ids, got %#v", result.ServerToolCalls)
	}
	if result.ServerToolCalls[0].OutputJSON == "" || result.ServerToolCalls[1].OutputJSON == "" {
		t.Fatalf("expected both code_execution outputs, got %#v", result.ServerToolCalls)
	}
	if result.ServerSideToolUsage["code_execution"] != 2 {
		t.Fatalf("expected cumulative code_execution usage, got %#v", result.ServerSideToolUsage)
	}
}

func TestBuildGeminiImageGenerationRequestBodyDropsUnsupportedImageSize(t *testing.T) {
	payload, err := buildGeminiImageGenerationRequestBody("gemini-2.5-flash-image", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
		Options: map[string]any{
			"aspectRatio": "1:1",
			"imageSize":   "1K",
			"generationConfig": map[string]any{
				"imageConfig": map[string]any{
					"aspectRatio": "1:1",
					"imageSize":   "1K",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build gemini image request body: %v", err)
	}
	imageConfig := asMap(payload["generationConfig"].(map[string]any)["imageConfig"])
	if imageConfig["aspectRatio"] != "1:1" {
		t.Fatalf("expected supported aspect ratio, got %#v", imageConfig)
	}
	if _, ok := imageConfig["imageSize"]; ok {
		t.Fatalf("expected imageSize to be omitted for Gemini 2.5 image model, got %#v", imageConfig)
	}
}

func TestBuildGeminiImageGenerationRequestBodyIncludesInlineImages(t *testing.T) {
	payload, err := buildGeminiImageGenerationRequestBody("gemini-3.1-flash-image", portllm.GenerateInput{
		Messages: []portllm.Message{
			{
				Role: "user",
				Parts: []portllm.ContentPart{
					{Kind: portllm.ContentPartText, Text: "Replace the background"},
					{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("source")},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build gemini image edit request body: %v", err)
	}

	contents := payload["contents"].([]map[string]any)
	parts := contents[0]["parts"].([]map[string]any)
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
		var requestBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		generationConfig := asMap(requestBody["generationConfig"])
		modalities := generationConfig["responseModalities"].([]any)
		if len(modalities) != 2 || modalities[0] != "TEXT" || modalities[1] != "IMAGE" {
			t.Fatalf("expected text and image modalities, got %#v", modalities)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"responseId\":\"gemini-image-stream-1\",\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"A revised prompt\"}]}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"inlineData\":{\"mimeType\":\"image/png\",\"data\":\"iVBORw0KGgo=\"}}]}}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":3}}\n\n"))
	}))
	defer server.Close()

	var usageEvents []portllm.Usage
	output, err := newTestClient().GenerateStream(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterGoogleImageGeneration,
		BaseURL:       server.URL,
		APIKey:        "gemini-key",
		UpstreamModel: "nano-banana-pro",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
	}, func(event portllm.GenerateStreamEvent) error {
		if event.Usage != (portllm.Usage{}) {
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
	var upstreamErr *portllm.UpstreamError
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("expected upstream error, got %v", err)
	}
	if upstreamErr.Message != "google authentication failed; check API key, upstream base URL, and custom auth headers" {
		t.Fatalf("unexpected unauthorized message: %q", upstreamErr.Message)
	}
}
