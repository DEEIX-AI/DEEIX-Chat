package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestBuildOpenRouterImageRequestBody(t *testing.T) {
	payload, debugBody, err := buildOpenRouterImageRequestBody("openai/gpt-image-1", portllm.GenerateInput{
		Messages: []portllm.Message{
			{Role: "system", Content: "ignore"},
			{Role: "user", Content: "a red panda astronaut floating in space"},
		},
		Options: map[string]any{
			"n":                  2,
			"aspect_ratio":       "16:9",
			"resolution":         "2k",
			"quality":            "High",
			"output_format":      "jpg",
			"background":         "opaque",
			"output_compression": 80,
			"seed":               42,
			"user":               "end-user-1",
			"provider": map[string]any{
				"only":            []any{"openai", " "},
				"sort":            "price",
				"allow_fallbacks": false,
				"options":         map[string]any{"openai": map[string]any{"moderation": "low"}},
				"unknown":         "dropped",
			},
			"prompt":          "override",
			"stream":          true,
			"partial_images":  2,
			"response_format": "url",
		},
	}, false)
	if err != nil {
		t.Fatalf("build OpenRouter image request body: %v", err)
	}
	if payload["model"] != "openai/gpt-image-1" || payload["prompt"] != "a red panda astronaut floating in space" {
		t.Fatalf("unexpected model or prompt: %#v", payload)
	}
	if payload["n"] != 2 || payload["aspect_ratio"] != "16:9" || payload["resolution"] != "2K" {
		t.Fatalf("expected normalized dimension params, got %#v", payload)
	}
	if payload["quality"] != "high" || payload["output_format"] != "jpeg" || payload["background"] != "opaque" {
		t.Fatalf("expected normalized rendering params, got %#v", payload)
	}
	if payload["output_compression"] != 80 || payload["seed"] != 42 || payload["user"] != "end-user-1" {
		t.Fatalf("expected passthrough scalar params, got %#v", payload)
	}
	provider := payload["provider"].(map[string]any)
	if only := provider["only"].([]string); len(only) != 1 || only[0] != "openai" {
		t.Fatalf("expected trimmed provider.only, got %#v", provider)
	}
	if provider["sort"] != "price" || provider["allow_fallbacks"] != false {
		t.Fatalf("expected provider routing fields, got %#v", provider)
	}
	if _, ok := provider["unknown"]; ok {
		t.Fatalf("unknown provider fields must be dropped: %#v", provider)
	}
	if _, ok := provider["options"].(map[string]any)["openai"]; !ok {
		t.Fatalf("expected provider.options passthrough, got %#v", provider)
	}
	for _, key := range []string{"stream", "partial_images", "response_format", "input_references"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("unexpected param %q in payload %#v", key, payload)
		}
	}
	if !json.Valid(debugBody) || strings.Contains(string(debugBody), "input_reference_count") {
		t.Fatalf("unexpected debug body: %s", string(debugBody))
	}
}

func TestBuildOpenRouterImageRequestBodyDropsInvalidParams(t *testing.T) {
	payload, _, err := buildOpenRouterImageRequestBody("openai/gpt-image-1", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "a product photo"}},
		Options: map[string]any{
			"n":                  11,
			"aspect_ratio":       "7:5",
			"resolution":         "8K",
			"size":               "huge",
			"quality":            "ultra",
			"output_format":      "svg",
			"background":         "blurred",
			"output_compression": 101,
			"seed":               1.5,
			"provider":           "openai",
		},
	}, false)
	if err != nil {
		t.Fatalf("build OpenRouter image request body: %v", err)
	}
	for _, key := range []string{"n", "aspect_ratio", "resolution", "size", "quality", "output_format", "background", "output_compression", "seed", "provider"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("invalid param %q must be removed: %#v", key, payload)
		}
	}
}

func TestBuildOpenRouterImageRequestBodyProviderSortForms(t *testing.T) {
	cases := map[string]struct {
		sort any
		want any
	}{
		"strategy string":       {sort: "Exacto", want: "exacto"},
		"unknown strategy":      {sort: "cheapest", want: nil},
		"config object":         {sort: map[string]any{"by": "latency", "partition": "none"}, want: map[string]any{"by": "latency", "partition": "none"}},
		"config object invalid": {sort: map[string]any{"by": "random", "partition": "half"}, want: nil},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			payload, _, err := buildOpenRouterImageRequestBody("openai/gpt-image-1", portllm.GenerateInput{
				Messages: []portllm.Message{{Role: "user", Content: "a product photo"}},
				Options:  map[string]any{"provider": map[string]any{"sort": testCase.sort}},
			}, false)
			if err != nil {
				t.Fatalf("build OpenRouter image request body: %v", err)
			}
			provider, _ := payload["provider"].(map[string]any)
			got := provider["sort"]
			if testCase.want == nil {
				if got != nil {
					t.Fatalf("expected sort to be dropped, got %#v", got)
				}
				return
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(testCase.want)
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("sort = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestBuildOpenRouterImageRequestBodySizeAcceptsTierAndPixels(t *testing.T) {
	for raw, expected := range map[string]string{"4k": "4K", "2048x2048": "2048x2048", "1024X768": "1024x768"} {
		payload, _, err := buildOpenRouterImageRequestBody("openai/gpt-image-1", portllm.GenerateInput{
			Messages: []portllm.Message{{Role: "user", Content: "a product photo"}},
			Options:  map[string]any{"size": raw},
		}, false)
		if err != nil {
			t.Fatalf("build OpenRouter image request body: %v", err)
		}
		if payload["size"] != expected {
			t.Fatalf("size %q: expected %q, got %#v", raw, expected, payload["size"])
		}
	}
}

func TestBuildOpenRouterImageRequestBodyWithReferences(t *testing.T) {
	payload, debugBody, err := buildOpenRouterImageRequestBody("openai/gpt-image-1", portllm.GenerateInput{
		Messages: []portllm.Message{{
			Role: "user",
			Parts: []portllm.ContentPart{
				{Kind: portllm.ContentPartText, Text: "make this scene look like a watercolor painting"},
				{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("source")},
				{Kind: portllm.ContentPartImage, Data: []byte("second")},
			},
		}},
	}, true)
	if err != nil {
		t.Fatalf("build OpenRouter image edit request body: %v", err)
	}
	if payload["prompt"] != "make this scene look like a watercolor painting" || payload["stream"] != true {
		t.Fatalf("unexpected prompt or stream flag: %#v", payload)
	}
	references := payload["input_references"].([]map[string]any)
	if len(references) != 2 {
		t.Fatalf("expected two input references, got %#v", references)
	}
	first := references[0]
	if first["type"] != "image_url" {
		t.Fatalf("expected image_url content part, got %#v", first)
	}
	if url := first["image_url"].(map[string]any)["url"].(string); !strings.HasPrefix(url, "data:image/png;base64,c291cmNl") {
		t.Fatalf("expected base64 data url, got %q", url)
	}
	if url := references[1]["image_url"].(map[string]any)["url"].(string); !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("expected png fallback mime for untyped reference, got %q", url)
	}
	debug := string(debugBody)
	if strings.Contains(debug, "c291cmNl") || !strings.Contains(debug, `"input_reference_count":2`) {
		t.Fatalf("debug body must replace reference bytes with a count: %s", debug)
	}
}

func TestBuildOpenRouterImageRequestBodyRejectsTooManyReferences(t *testing.T) {
	parts := []portllm.ContentPart{{Kind: portllm.ContentPartText, Text: "combine"}}
	for range openRouterImageMaxInputReferences + 1 {
		parts = append(parts, portllm.ContentPart{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("x")})
	}
	_, _, err := buildOpenRouterImageRequestBody("openai/gpt-image-1", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Parts: parts}},
	}, false)
	if err == nil {
		t.Fatal("expected too many references error")
	}
}

func TestGenerateOpenRouterImageUsesUnifiedImagesEndpoint(t *testing.T) {
	var requestPath string
	var requestBody map[string]any
	var headers http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		headers = r.Header.Clone()
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"created": 1748372400,
			"data": [
				{"b64_json": "aGVsbG8=", "media_type": "image/webp"},
				{"b64_json": "anBn", "media_type": "image/jpeg"}
			],
			"usage": {"prompt_tokens": 16, "completion_tokens": 272, "total_tokens": 288, "cost": 0.011}
		}`))
	}))
	defer server.Close()

	output, err := newTestClient().Generate(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterOpenRouterImages,
		Endpoint:      portllm.EndpointImageEdits,
		BaseURL:       server.URL + "/api/v1",
		APIKey:        "or-key",
		UpstreamModel: "bytedance-seed/seedream-4.5",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "a red panda astronaut"}},
		Options:  map[string]any{"resolution": "2K", "output_format": "png"},
	})
	if err != nil {
		t.Fatalf("generate OpenRouter image: %v", err)
	}
	if requestPath != "/api/v1/images" {
		t.Fatalf("expected unified images endpoint, got %q", requestPath)
	}
	if headers.Get("Authorization") != "Bearer or-key" {
		t.Fatalf("unexpected auth header %q", headers.Get("Authorization"))
	}
	if requestBody["model"] != "bytedance-seed/seedream-4.5" || requestBody["resolution"] != "2K" {
		t.Fatalf("unexpected request body: %#v", requestBody)
	}
	if _, ok := requestBody["stream"]; ok {
		t.Fatalf("non-stream generate must not send stream flag: %#v", requestBody)
	}
	if len(output.GeneratedImages) != 2 {
		t.Fatalf("expected two generated images, got %#v", output.GeneratedImages)
	}
	if output.GeneratedImages[0].B64JSON != "aGVsbG8=" || output.GeneratedImages[0].MIMEType != "image/webp" {
		t.Fatalf("declared media_type must win over output_format: %#v", output.GeneratedImages[0])
	}
	if output.GeneratedImages[1].MIMEType != "image/jpeg" {
		t.Fatalf("expected per-item media type, got %#v", output.GeneratedImages[1])
	}
	if output.Usage.InputTokens != 16 || output.Usage.OutputTokens != 272 {
		t.Fatalf("expected upstream usage, got %#v", output.Usage)
	}
	if !strings.Contains(output.Usage.RawUsageJSON, `"cost"`) {
		t.Fatalf("raw usage must retain OpenRouter cost for auditing, got %q", output.Usage.RawUsageJSON)
	}
}

func TestGenerateOpenRouterImageSetsAttributionHeadersForOpenRouterHost(t *testing.T) {
	var headers http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"created":1,"data":[{"b64_json":"aGVsbG8="}]}`))
	}))
	defer server.Close()

	// 归因头只在真实 OpenRouter 域名上附加，本地测试服务器不应带上。
	_, err := newTestClient().Generate(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterOpenRouterImages,
		BaseURL:       server.URL,
		UpstreamModel: "openai/gpt-image-1",
	}, portllm.GenerateInput{Messages: []portllm.Message{{Role: "user", Content: "x"}}})
	if err != nil {
		t.Fatalf("generate OpenRouter image: %v", err)
	}
	if headers.Get("HTTP-Referer") != "" {
		t.Fatalf("attribution headers must be scoped to openrouter.ai, got %q", headers.Get("HTTP-Referer"))
	}

	req, err := http.NewRequest(http.MethodPost, "https://openrouter.ai/api/v1/images", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	setOpenRouterAttributionHeaders(req, portllm.RouteConfig{BaseURL: "https://openrouter.ai/api/v1"})
	if req.Header.Get("HTTP-Referer") == "" || req.Header.Get("X-Title") == "" {
		t.Fatalf("expected attribution headers for openrouter.ai, got %#v", req.Header)
	}
}

func TestOpenRouterImageStreamEmitsPartialAndCompletedEvents(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/images" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Fatalf("expected event stream accept header, got %q", r.Header.Get("Accept"))
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"image_generation.partial_image\",\"partial_image_index\":0,\"b64_json\":\"cGFydGlhbA==\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"image_generation.text_chunk\",\"phase\":\"reasoning\",\"text\":\"thinking\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"image_generation.completed\",\"b64_json\":\"ZmluYWw=\",\"media_type\":\"image/png\",\"created\":1748372400,\"usage\":{\"prompt_tokens\":16,\"completion_tokens\":272,\"total_tokens\":288,\"cost\":0.011}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	var partials []portllm.GenerateStreamEvent
	var usageEvents []portllm.Usage
	output, err := newTestClient().GenerateStream(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterOpenRouterImages,
		BaseURL:       server.URL + "/api/v1",
		UpstreamModel: "openai/gpt-image-1",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "a detailed landscape"}},
		Options:  map[string]any{"output_format": "webp"},
	}, func(event portllm.GenerateStreamEvent) error {
		if event.GeneratedImage != nil {
			partials = append(partials, event)
		}
		if event.Usage != (portllm.Usage{}) {
			usageEvents = append(usageEvents, event.Usage)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("generate OpenRouter image stream: %v", err)
	}
	if requestBody["stream"] != true {
		t.Fatalf("expected stream request payload, got %#v", requestBody)
	}
	if len(partials) != 1 || partials[0].GeneratedImage.B64JSON != "cGFydGlhbA==" || !partials[0].GeneratedImagePartial {
		t.Fatalf("expected one partial image event, got %#v", partials)
	}
	if len(output.GeneratedImages) != 1 || output.GeneratedImages[0].B64JSON != "ZmluYWw=" {
		t.Fatalf("expected final generated image, got %#v", output.GeneratedImages)
	}
	if output.GeneratedImages[0].MIMEType != "image/png" {
		t.Fatalf("completed event media_type must override output_format, got %#v", output.GeneratedImages[0])
	}
	if output.Usage.InputTokens != 16 || output.Usage.OutputTokens != 272 || len(usageEvents) != 1 {
		t.Fatalf("expected stream usage, got %#v / %#v", output.Usage, usageEvents)
	}
	if output.Text != "" {
		t.Fatalf("text chunks are intermediate phases and must not leak into text, got %q", output.Text)
	}
}

func TestOpenRouterImageStreamFallsBackToBufferedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"created":1748372400,"data":[{"b64_json":"ZmluYWw=","media_type":"image/jpeg"}],"usage":{"prompt_tokens":0,"completion_tokens":4175,"total_tokens":4175,"cost":0.04}}`))
	}))
	defer server.Close()

	var usageEvents []portllm.Usage
	output, err := newTestClient().GenerateStream(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterOpenRouterImages,
		BaseURL:       server.URL + "/api/v1",
		UpstreamModel: "bytedance-seed/seedream-4.5",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "a landscape photo"}},
	}, func(event portllm.GenerateStreamEvent) error {
		if event.Usage != (portllm.Usage{}) {
			usageEvents = append(usageEvents, event.Usage)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("generate OpenRouter image stream fallback: %v", err)
	}
	if len(output.GeneratedImages) != 1 || output.GeneratedImages[0].MIMEType != "image/jpeg" {
		t.Fatalf("expected buffered image with declared media type, got %#v", output.GeneratedImages)
	}
	if len(usageEvents) != 1 || usageEvents[0].OutputTokens != 4175 {
		t.Fatalf("expected usage event from buffered fallback, got %#v", usageEvents)
	}
}

func TestOpenRouterImageStreamSurfacesErrorEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"error\",\"error\":{\"message\":\"The upstream provider returned an error\",\"code\":\"upstream_error\",\"type\":\"provider_error\"}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	_, err := newTestClient().GenerateStream(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterOpenRouterImages,
		BaseURL:       server.URL + "/api/v1",
		UpstreamModel: "openai/gpt-image-1",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "x"}},
	}, func(portllm.GenerateStreamEvent) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "upstream provider returned an error") {
		t.Fatalf("expected stream error event to surface, got %v", err)
	}
}

func TestOpenRouterImageGenerateRejectsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(`{"error":{"code":402,"message":"Insufficient credits. Add more using https://openrouter.ai/credits"}}`))
	}))
	defer server.Close()

	_, err := newTestClient().Generate(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterOpenRouterImages,
		BaseURL:       server.URL + "/api/v1",
		UpstreamModel: "openai/gpt-image-1",
	}, portllm.GenerateInput{Messages: []portllm.Message{{Role: "user", Content: "x"}}})
	var upstreamErr *portllm.UpstreamError
	if err == nil || !errors.As(err, &upstreamErr) || upstreamErr.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("expected 402 upstream error, got %v", err)
	}
}

func TestOpenRouterImagesListModelsUsesImageCatalog(t *testing.T) {
	var requestPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"bytedance-seed/seedream-4.5","name":"Seedream 4.5","architecture":{"output_modalities":["image"]}}]}`))
	}))
	defer server.Close()

	items, err := newTestClient().ListModels(context.Background(), portllm.RouteConfig{
		Protocol: portllm.AdapterOpenRouterImages,
		BaseURL:  server.URL + "/api/v1",
	})
	if err != nil {
		t.Fatalf("list OpenRouter image models: %v", err)
	}
	if requestPath != "/api/v1/images/models" {
		t.Fatalf("expected image model catalog path, got %q", requestPath)
	}
	if len(items) != 1 || items[0].ID != "bytedance-seed/seedream-4.5" {
		t.Fatalf("unexpected model items: %#v", items)
	}
}

func TestOpenRouterImagesAdapterContract(t *testing.T) {
	if !portllm.IsImplementedAdapter(portllm.AdapterOpenRouterImages) {
		t.Fatal("openrouter_images must be an implemented adapter")
	}
	if !portllm.IsImageGenerationAdapter(portllm.AdapterOpenRouterImages) || !portllm.IsImageEditAdapter(portllm.AdapterOpenRouterImages) {
		t.Fatal("openrouter_images must serve both image generation and reference-image edits")
	}
	if !portllm.SupportsStreamingAdapter(portllm.AdapterOpenRouterImages) || !portllm.SupportsImageGenerationStream(portllm.AdapterOpenRouterImages, "bytedance-seed/seedream-4.5") {
		t.Fatal("openrouter_images must always route through the stream entry (buffered fallback is handled upstream)")
	}
	if got := portllm.DefaultEndpointForAdapter(portllm.AdapterOpenRouterImages); got != portllm.EndpointImages {
		t.Fatalf("expected images endpoint, got %q", got)
	}
	if got := buildOpenAIRequestURL("https://openrouter.ai/api/v1", portllm.EndpointImages); got != "https://openrouter.ai/api/v1/images" {
		t.Fatalf("unexpected images url %q", got)
	}
	if got := buildOpenAIRequestURL("https://openrouter.ai/api", portllm.EndpointImages); got != "https://openrouter.ai/api/v1/images" {
		t.Fatalf("expected version segment to be appended, got %q", got)
	}
}
