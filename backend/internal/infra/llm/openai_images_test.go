package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestBuildOpenAIImageGenerationRequestBody(t *testing.T) {
	payload, err := buildOpenAIImageGenerationRequestBody("gpt-image-1", portllm.GenerateInput{
		Messages: []portllm.Message{
			{Role: "system", Content: "ignore"},
			{Role: "user", Content: "A clean product render"},
		},
		Options: map[string]any{
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
	payload, err := buildOpenAIImageGenerationStreamRequestBody("gpt-image-1", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
		Options: map[string]any{
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

func TestBuildOpenAIImageGenerationStreamRequestBodyOmitsDefaultPartialImages(t *testing.T) {
	payload, err := buildOpenAIImageGenerationStreamRequestBody("gpt-image-1", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
	})
	if err != nil {
		t.Fatalf("build image stream request body: %v", err)
	}
	if payload["stream"] != true {
		t.Fatalf("expected stream request payload, got %#v", payload)
	}
	if _, ok := payload["partial_images"]; ok {
		t.Fatalf("partial_images must only be sent when explicitly configured, got %#v", payload)
	}
}

func TestBuildOpenAIImageGenerationStreamRequestBodyOmitsZeroPartialImages(t *testing.T) {
	payload, err := buildOpenAIImageGenerationStreamRequestBody("gpt-image-1", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
		Options:  map[string]any{"partial_images": 0},
	})
	if err != nil {
		t.Fatalf("build image stream request body: %v", err)
	}
	if _, ok := payload["partial_images"]; ok {
		t.Fatalf("partial_images must only be sent when configured as a positive value, got %#v", payload)
	}
}

func TestBuildOpenAIImageGenerationRequestBodyDallEParams(t *testing.T) {
	payload, err := buildOpenAIImageGenerationRequestBody("dall-e-3", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
		Options: map[string]any{
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

func TestBuildOpenAIImageGenerationRequestBodyDefaultsDallEToBase64(t *testing.T) {
	payload, err := buildOpenAIImageGenerationRequestBody("dall-e-3", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
	})
	if err != nil {
		t.Fatalf("build image request body: %v", err)
	}
	if payload["response_format"] != "b64_json" {
		t.Fatalf("expected DALL-E image generation to default to base64, got %#v", payload)
	}
}

func TestBuildOpenAIImageEditMultipartRequest(t *testing.T) {
	body, contentType, debugBody, err := buildOpenAIImageEditMultipartRequest("gpt-image-1", portllm.GenerateInput{
		Messages: []portllm.Message{{
			Role: "user",
			Parts: []portllm.ContentPart{
				{Kind: portllm.ContentPartText, Text: "Replace the background"},
				{Kind: portllm.ContentPartImage, MimeType: "image/png", FileName: "source.png", Data: []byte("source")},
			},
		}},
		ImageEditMask: &portllm.ContentPart{Kind: portllm.ContentPartImage, MimeType: "image/png", FileName: "mask.png", Data: []byte("mask")},
		Options: map[string]any{
			"size":               "1024x1024",
			"quality":            "high",
			"output_format":      "webp",
			"output_compression": 80,
			"input_fidelity":     "high",
			"partial_images":     2,
			"stream":             true,
			"prompt":             "override",
		},
	}, false)
	if err != nil {
		t.Fatalf("build image edit multipart request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if err = req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("parse multipart body: %v", err)
	}
	form := req.MultipartForm
	if form.Value["model"][0] != "gpt-image-1" || form.Value["prompt"][0] != "Replace the background" {
		t.Fatalf("unexpected edit form fields: %#v", form.Value)
	}
	if form.Value["output_format"][0] != "webp" || form.Value["input_fidelity"][0] != "high" {
		t.Fatalf("expected edit params, got %#v", form.Value)
	}
	if _, ok := form.Value["stream"]; ok {
		t.Fatalf("stream must not be passed by non-streaming edit adapter: %#v", form.Value)
	}
	if _, ok := form.Value["partial_images"]; ok {
		t.Fatalf("partial_images must not be passed without upstream image edit streaming: %#v", form.Value)
	}
	if len(form.File["image[]"]) != 1 || len(form.File["mask"]) != 1 {
		t.Fatalf("expected image[] and mask files, got %#v", form.File)
	}
	var debug map[string]any
	if err = json.Unmarshal(debugBody, &debug); err != nil {
		t.Fatalf("parse debug body: %v", err)
	}
	if debug["image_count"] != float64(1) || debug["mask"] != true {
		t.Fatalf("expected sanitized debug body, got %#v", debug)
	}
}

func TestBuildOpenAIImageEditMultipartRequestDefaultsDallEToBase64(t *testing.T) {
	body, contentType, _, err := buildOpenAIImageEditMultipartRequest("dall-e-2", portllm.GenerateInput{
		Messages: []portllm.Message{{
			Role: "user",
			Parts: []portllm.ContentPart{
				{Kind: portllm.ContentPartText, Text: "Replace the background"},
				{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("source")},
			},
		}},
	}, false)
	if err != nil {
		t.Fatalf("build image edit multipart request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if err = req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("parse multipart body: %v", err)
	}
	if req.MultipartForm.Value["response_format"][0] != "b64_json" {
		t.Fatalf("expected DALL-E image edit to default to base64, got %#v", req.MultipartForm.Value)
	}
}

func TestBuildOpenAIImageEditStreamMultipartRequest(t *testing.T) {
	body, contentType, _, err := buildOpenAIImageEditMultipartRequest("gpt-image-1", portllm.GenerateInput{
		Messages: []portllm.Message{{
			Role: "user",
			Parts: []portllm.ContentPart{
				{Kind: portllm.ContentPartText, Text: "Replace the background"},
				{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("source")},
			},
		}},
		Options: map[string]any{"partial_images": 2},
	}, true)
	if err != nil {
		t.Fatalf("build image edit stream multipart request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if err = req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("parse multipart body: %v", err)
	}
	if req.MultipartForm.Value["stream"][0] != "true" || req.MultipartForm.Value["partial_images"][0] != "2" {
		t.Fatalf("expected edit stream fields, got %#v", req.MultipartForm.Value)
	}
}

func TestBuildOpenAIImageEditStreamMultipartRequestOmitsZeroPartialImages(t *testing.T) {
	body, contentType, _, err := buildOpenAIImageEditMultipartRequest("gpt-image-1", portllm.GenerateInput{
		Messages: []portllm.Message{{
			Role: "user",
			Parts: []portllm.ContentPart{
				{Kind: portllm.ContentPartText, Text: "Replace the background"},
				{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("source")},
			},
		}},
		Options: map[string]any{"partial_images": 0},
	}, true)
	if err != nil {
		t.Fatalf("build image edit stream multipart request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if err = req.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("parse multipart body: %v", err)
	}
	if _, ok := req.MultipartForm.Value["partial_images"]; ok {
		t.Fatalf("partial_images must only be sent when configured as a positive value, got %#v", req.MultipartForm.Value)
	}
}

func TestParseOpenAIImageGenerationOutput(t *testing.T) {
	output, err := parseOpenAIImageOutput([]byte(`{
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
	output, err := parseOpenAIImageOutput([]byte(`{
		"data": [{"b64_json": "aGVsbG8="}]
	}`), "png")
	if err != nil {
		t.Fatalf("parse image output: %v", err)
	}
	if output.Usage != (portllm.Usage{}) {
		t.Fatalf("expected missing upstream usage to remain empty, got %#v", output.Usage)
	}
}

// 兼容网关可能在同一条目里同时返回 url 与 b64_json：两者都要保留，下游才能直接用内联字节，
// 不必再下载 url。JSON、SSE 与流式请求收到的普通 JSON 走同一套解析。
func TestOpenAIImageResultKeepsURLAndBase64(t *testing.T) {
	for _, test := range []struct {
		name    string
		payload string
		wantURL bool
		wantB64 bool
	}{
		{"both", `{"url":"https://images.example.test/result.png","b64_json":"aW1hZ2U="}`, true, true},
		{"url only", `{"url":"https://images.example.test/result.png"}`, true, false},
		{"base64 only", `{"b64_json":"aW1hZ2U="}`, false, true},
		{"blank base64", `{"url":"https://images.example.test/result.png","b64_json":" \t"}`, true, false},
		// 无效的 Base64 原样保留，由下游解码时报错，不在解析阶段悄悄换成下载。
		{"invalid base64 is retained", `{"url":"https://images.example.test/result.png","b64_json":"invalid!"}`, true, true},
	} {
		for _, format := range []string{"json", "sse", "stream json"} {
			t.Run(test.name+"/"+format, func(t *testing.T) {
				body := `{"response":{"data":[{"image":` + test.payload + `,"revised_prompt":"revised"}]}}`
				var output *portllm.GenerateOutput
				var err error
				if format == "json" {
					output, err = parseOpenAIImageOutput([]byte(body), "webp")
				} else {
					output = &portllm.GenerateOutput{}
					if format == "sse" {
						body = "event: image_generation.completed\ndata: " + body + "\n\n"
					}
					err = consumeOpenAIImageStream(strings.NewReader(body), "webp", output, nil)
				}
				if err != nil || len(output.GeneratedImages) != 1 {
					t.Fatalf("parse result: err=%v", err)
				}
				image := output.GeneratedImages[0]
				if (image.URL != "") != test.wantURL || (image.B64JSON != "") != test.wantB64 {
					t.Fatalf("unexpected image sources: URL=%t base64=%t", image.URL != "", image.B64JSON != "")
				}
				if image.MIMEType != "image/webp" || image.RevisedPrompt != "revised" {
					t.Fatal("image metadata was not preserved")
				}
			})
		}
	}
}

// Responses 的内嵌生图共用同一个解析函数，同样两者都保留。
func TestResponsesImageResultKeepsURLAndBase64(t *testing.T) {
	image, ok := parseResponsesGeneratedImage(map[string]any{
		"type": "image_generation_call",
		"result": map[string]any{
			"url":      "https://images.example.test/result.png",
			"b64_json": "aW1hZ2U=",
		},
	})
	if !ok || image.URL == "" || image.B64JSON == "" {
		t.Fatalf("expected both image sources, got URL=%t base64=%t", image.URL != "", image.B64JSON != "")
	}
}

// 每个图片适配器在非流式、SSE 与流式请求收到普通 JSON 时，最终图与流式中间图都带上内联字节，
// 且不改动请求的 response_format 与 stream。
func TestImageAdaptersKeepInlineImageBytes(t *testing.T) {
	for _, protocol := range []string{
		portllm.AdapterOpenAIImageGenerations, portllm.AdapterOpenAIImageEdits,
		portllm.AdapterOpenRouterImages, portllm.AdapterXAIImage, portllm.AdapterXAIImageEdits,
	} {
		for _, stream := range []bool{false, true} {
			if stream && !portllm.SupportsImageGenerationStream(protocol, "gpt-image-1") {
				continue
			}
			for _, jsonFallback := range []bool{false, true} {
				if !stream && jsonFallback {
					continue
				}
				t.Run(fmt.Sprintf("%s/stream=%t/json=%t", protocol, stream, jsonFallback), func(t *testing.T) {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
							if err := r.ParseMultipartForm(1 << 20); err != nil {
								t.Errorf("parse multipart: %v", err)
								return
							}
							defer r.MultipartForm.RemoveAll() //nolint:errcheck
							if r.FormValue("response_format") != "" {
								t.Error("request response_format changed")
							}
							if (r.FormValue("stream") == "true") != stream {
								t.Error("request stream changed")
							}
						} else {
							var payload map[string]any
							if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
								t.Errorf("decode request: %v", err)
								return
							}
							if _, ok := payload["response_format"]; ok {
								t.Error("request response_format changed")
							}
							if (payload["stream"] == true) != stream {
								t.Error("request stream changed")
							}
						}
						entry := `{"url":"https://images.example.test/unavailable","b64_json":"aW1hZ2U="}`
						if stream && !jsonFallback {
							w.Header().Set("Content-Type", "text/event-stream")
							_, _ = fmt.Fprintf(w, "event: image_generation.partial_image\ndata: %s\n\nevent: image_generation.completed\ndata: %s\n\n", entry, entry)
						} else {
							w.Header().Set("Content-Type", "application/json")
							_, _ = fmt.Fprintf(w, `{"data":[%s]}`, entry)
						}
					}))
					defer server.Close()
					client := newTestClient()
					defer client.CloseIdleConnections()
					input := portllm.GenerateInput{Messages: []portllm.Message{{Role: "user", Parts: []portllm.ContentPart{
						{Kind: portllm.ContentPartText, Text: "draw"},
						{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("source")},
					}}}}
					route := portllm.RouteConfig{Protocol: protocol, BaseURL: server.URL, UpstreamModel: "gpt-image-1"}
					var output *portllm.GenerateOutput
					var err error
					partials := 0
					if stream {
						output, err = client.GenerateStream(t.Context(), route, input, func(event portllm.GenerateStreamEvent) error {
							if event.GeneratedImage != nil {
								partials++
								if event.GeneratedImage.B64JSON == "" {
									t.Error("partial image dropped its inline bytes")
								}
							}
							return nil
						})
					} else {
						output, err = client.Generate(t.Context(), route, input)
					}
					if err != nil || len(output.GeneratedImages) != 1 {
						t.Fatalf("generate: err=%v", err)
					}
					if image := output.GeneratedImages[0]; image.B64JSON == "" || image.URL == "" {
						t.Fatalf("expected both image sources, got URL=%t base64=%t", image.URL != "", image.B64JSON != "")
					}
					if stream && !jsonFallback && partials != 1 {
						t.Fatalf("partial image count = %d", partials)
					}
				})
			}
		}
	}
}

func TestOpenAIImageGenerationStream(t *testing.T) {
	var requestPayload map[string]any
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

	client := newTestClient()
	var partials []portllm.GenerateStreamEvent
	output, err := client.GenerateStream(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterOpenAIImageGenerations,
		BaseURL:       server.URL,
		UpstreamModel: "gpt-image-1",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
		Options: map[string]any{
			"output_format":  "webp",
			"partial_images": 2,
		},
	}, func(event portllm.GenerateStreamEvent) error {
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

	client := newTestClient()
	var usageEvents []portllm.Usage
	output, err := client.GenerateStream(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterOpenAIImageGenerations,
		BaseURL:       server.URL,
		UpstreamModel: "gpt-image-1",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A clean product render"}},
	}, func(event portllm.GenerateStreamEvent) error {
		if event.Usage != (portllm.Usage{}) {
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

func TestOpenAIImageEditStream(t *testing.T) {
	var requestValues map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("expected event stream accept header, got %q", r.Header.Get("Accept"))
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse multipart request: %v", err)
		}
		requestValues = r.MultipartForm.Value
		if len(r.MultipartForm.File["image[]"]) != 1 {
			t.Fatalf("expected one edit image, got %#v", r.MultipartForm.File)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: image_edit.partial_image\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"image_edit.partial_image\",\"partial_image_index\":1,\"b64_json\":\"cGFydGlhbA==\"}\n\n"))
		_, _ = w.Write([]byte("event: image_edit.completed\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"image_edit.completed\",\"id\":\"img_edit_1\",\"b64_json\":\"ZmluYWw=\",\"usage\":{\"input_tokens\":22,\"output_tokens\":44}}\n\n"))
	}))
	defer server.Close()

	client := newTestClient()
	var partials []portllm.GenerateStreamEvent
	output, err := client.GenerateStream(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterOpenAIImageEdits,
		BaseURL:       server.URL,
		UpstreamModel: "gpt-image-1",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{
			Role: "user",
			Parts: []portllm.ContentPart{
				{Kind: portllm.ContentPartText, Text: "Replace the background"},
				{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("source")},
			},
		}},
		Options: map[string]any{
			"output_format":  "webp",
			"partial_images": 2,
		},
	}, func(event portllm.GenerateStreamEvent) error {
		if event.GeneratedImage != nil {
			partials = append(partials, event)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("generate image edit stream: %v", err)
	}
	if requestValues["stream"][0] != "true" || requestValues["partial_images"][0] != "2" {
		t.Fatalf("expected stream request values, got %#v", requestValues)
	}
	if len(partials) != 1 || partials[0].GeneratedImage == nil || partials[0].GeneratedImage.B64JSON != "cGFydGlhbA==" {
		t.Fatalf("expected partial edit image event, got %#v", partials)
	}
	if len(output.GeneratedImages) != 1 || output.GeneratedImages[0].B64JSON != "ZmluYWw=" {
		t.Fatalf("expected final edit image, got %#v", output.GeneratedImages)
	}
	if output.ResponseID != "img_edit_1" {
		t.Fatalf("expected edit response id, got %q", output.ResponseID)
	}
	if output.Usage.InputTokens != 22 || output.Usage.OutputTokens != 44 {
		t.Fatalf("expected edit usage, got %#v", output.Usage)
	}
}
