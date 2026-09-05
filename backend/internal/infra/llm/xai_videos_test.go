package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestBuildXAIVideoRequestBody(t *testing.T) {
	payload, debugBody, err := buildXAIVideoRequestBody("grok-imagine-video", portllm.GenerateInput{
		Messages: []portllm.Message{{
			Role: "user",
			Parts: []portllm.ContentPart{
				{Kind: portllm.ContentPartText, Text: "Animate the scene"},
				{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("source")},
			},
		}},
		Options: map[string]any{
			"aspect_ratio": "16:9",
			"duration":     6,
			"resolution":   "720P",
			"prompt":       "must not override messages",
			"output":       "must not pass through",
		},
	})
	if err != nil {
		t.Fatalf("build xAI video request body: %v", err)
	}
	if payload["model"] != "grok-imagine-video" || payload["prompt"] != "Animate the scene" {
		t.Fatalf("unexpected model or prompt: %#v", payload)
	}
	if payload["aspect_ratio"] != "16:9" || payload["duration"] != 6 || payload["resolution"] != "720p" {
		t.Fatalf("expected documented xAI video params, got %#v", payload)
	}
	image := asMap(payload["image"])
	if !strings.HasPrefix(getString(image["url"]), "data:image/png;base64,c291cmNl") {
		t.Fatalf("expected data URL image input, got %#v", image)
	}
	for _, key := range []string{"output"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("unexpected xAI video param %q in %#v", key, payload)
		}
	}
	if strings.Contains(string(debugBody), "c291cmNl") || !strings.Contains(string(debugBody), `"image_count":1`) {
		t.Fatalf("debug body must summarize without source bytes: %s", debugBody)
	}
}

func TestBuildXAIVideoRequestBodyRejectsMultipleImages(t *testing.T) {
	_, _, err := buildXAIVideoRequestBody("grok-imagine-video", portllm.GenerateInput{
		Messages: []portllm.Message{{
			Role: "user",
			Parts: []portllm.ContentPart{
				{Kind: portllm.ContentPartText, Text: "Animate the scene"},
				{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("one")},
				{Kind: portllm.ContentPartImage, MimeType: "image/png", Data: []byte("two")},
			},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("expected multiple image validation error, got %v", err)
	}
}

func TestBuildXAIVideoRequestBodyDropsUnsupportedParams(t *testing.T) {
	payload, _, err := buildXAIVideoRequestBody("grok-imagine-video", portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "Animate the scene"}},
		Options: map[string]any{
			"aspect_ratio": "21:9",
			"duration":     8.5,
			"resolution":   "4k",
		},
	})
	if err != nil {
		t.Fatalf("build xAI video request body: %v", err)
	}
	for _, key := range []string{"aspect_ratio", "duration", "resolution"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("unsupported xAI video param %q must be removed: %#v", key, payload)
		}
	}
}

func TestBuildXAIVideoExtensionRequestBody(t *testing.T) {
	payload, debugBody, err := buildXAIVideoExtensionRequestBody("grok-imagine-video", portllm.GenerateInput{
		Messages:             []portllm.Message{{Role: "user", Content: "Continue the camera movement"}},
		VideoExtensionSource: &portllm.ContentPart{Kind: portllm.ContentPartVideo, MimeType: "video/mp4", Data: []byte("source-video")},
		Options:              map[string]any{"duration": 8, "aspect_ratio": "16:9", "resolution": "1080p"},
	})
	if err != nil {
		t.Fatalf("build xAI video extension request: %v", err)
	}
	if payload["duration"] != 8 || payload["prompt"] != "Continue the camera movement" {
		t.Fatalf("unexpected video extension payload: %#v", payload)
	}
	if _, ok := payload["aspect_ratio"]; ok {
		t.Fatalf("video extension must not forward generation-only options: %#v", payload)
	}
	video := asMap(payload["video"])
	if !strings.HasPrefix(getString(video["url"]), "data:video/mp4;base64,") {
		t.Fatalf("expected MP4 data URL, got %#v", video)
	}
	if strings.Contains(string(debugBody), "c291cmNlLXZpZGVv") {
		t.Fatalf("debug body must redact source video: %s", debugBody)
	}
}

func TestBuildXAIVideoExtensionRequestBodyRejectsInvalidSourceAndDuration(t *testing.T) {
	payload, _, err := buildXAIVideoExtensionRequestBody("grok-imagine-video", portllm.GenerateInput{
		Messages:             []portllm.Message{{Role: "user", Content: "Continue"}},
		VideoExtensionSource: &portllm.ContentPart{Kind: portllm.ContentPartVideo, MimeType: "video/webm", Data: []byte("source")},
	})
	if err == nil || payload != nil {
		t.Fatalf("expected invalid source error, got payload=%#v err=%v", payload, err)
	}
	options := map[string]any{"duration": 11, "resolution": "720p"}
	portllm.SanitizeXAIVideoExtensionOptions(options)
	if len(options) != 0 {
		t.Fatalf("unsupported extension options must be removed: %#v", options)
	}
}

func TestXAIVideoPollDelayClampsUnsafeValues(t *testing.T) {
	tests := map[string]time.Duration{
		"":   time.Second,
		"0":  time.Second,
		"-1": time.Second,
		"2":  2 * time.Second,
		"99": 10 * time.Second,
	}
	for retryAfter, expected := range tests {
		if got := xAIVideoPollDelay(retryAfter); got != expected {
			t.Fatalf("retry-after %q: expected %s, got %s", retryAfter, expected, got)
		}
	}
}

func TestGenerateXAIVideoSubmitsAndPolls(t *testing.T) {
	postCount := 0
	pollCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer xai-key" {
			t.Fatalf("unexpected auth header %q", got)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos/generations":
			postCount++
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			if payload["model"] != "grok-imagine-video" || payload["duration"] != float64(8) {
				t.Fatalf("unexpected request body: %#v", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"request_id":"video_req_1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/video_req_1":
			pollCount++
			w.Header().Set("Content-Type", "application/json")
			if pollCount == 1 {
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"status":"pending","progress":50}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"status":"done",
				"video":{
					"url":"https://example.com/generated.mp4",
					"duration_seconds":6,
					"respect_moderation":true,
					"file_output":{"filename":"generated.mp4"}
				},
				"usage":{"cost_in_usd_ticks":27}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	output, err := newTestClient().Generate(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterXAIVideo,
		BaseURL:       server.URL + "/v1",
		APIKey:        "xai-key",
		ReadTimeoutMS: 5000,
		UpstreamModel: "grok-imagine-video",
	}, portllm.GenerateInput{
		Messages: []portllm.Message{{Role: "user", Content: "A cinematic orbit"}},
		Options:  map[string]any{"duration": 8},
	})
	if err != nil {
		t.Fatalf("generate xAI video: %v", err)
	}
	if postCount != 1 || pollCount != 2 {
		t.Fatalf("expected one submission and two polls, got post=%d poll=%d", postCount, pollCount)
	}
	if output.ResponseID != "video_req_1" || len(output.GeneratedVideos) != 1 {
		t.Fatalf("unexpected xAI video output: %#v", output)
	}
	video := output.GeneratedVideos[0]
	if video.URL != "https://example.com/generated.mp4" || video.MIMEType != "video/mp4" || video.FileName != "generated.mp4" || video.DurationSeconds != 6 {
		t.Fatalf("unexpected generated video: %#v", video)
	}
	if !strings.Contains(output.Usage.RawUsageJSON, `"cost_in_usd_ticks":27`) {
		t.Fatalf("expected raw usage JSON, got %#v", output.Usage)
	}
	if output.Debug == nil || output.Debug.Request.Path != "/v1/videos/video_req_1" {
		t.Fatalf("expected final poll debug snapshot, got %#v", output.Debug)
	}
}

func TestGenerateXAIVideoMarksFailedTaskAsAccepted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"request_id":"video_req_failed"}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"status":"failed",
			"error":{"code":"content_policy_violation","message":"request rejected"}
		}`))
	}))
	defer server.Close()

	_, err := newTestClient().Generate(context.Background(), portllm.RouteConfig{
		Protocol:      portllm.AdapterXAIVideo,
		BaseURL:       server.URL + "/v1",
		ReadTimeoutMS: 5000,
		UpstreamModel: "grok-imagine-video",
	}, portllm.GenerateInput{Messages: []portllm.Message{{Role: "user", Content: "Animate this"}}})
	if err == nil || !portllm.RequestWasAccepted(err) {
		t.Fatalf("expected accepted request error, got %v", err)
	}
	if !strings.Contains(err.Error(), "content_policy_violation: request rejected") {
		t.Fatalf("expected upstream failure details, got %v", err)
	}
}

func TestParseXAIVideoResultRejectsModeratedOutput(t *testing.T) {
	_, _, err := parseXAIVideoResult([]byte(`{
		"status":"done",
		"video":{"url":"https://example.com/blocked.mp4","respect_moderation":false}
	}`), "video_req_1", 6)
	if err == nil || !strings.Contains(err.Error(), "content moderation") {
		t.Fatalf("expected moderation error, got %v", err)
	}
}
