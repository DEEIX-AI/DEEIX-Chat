package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

const defaultXAIVideoPollInterval = time.Second

// xAIVideoAdapter 实现 xAI 异步视频生成协议。
type xAIVideoAdapter struct {
	client *Client
}

func (a *xAIVideoAdapter) Name() string { return portllm.AdapterXAIVideo }

func (a *xAIVideoAdapter) Generate(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	route.Protocol = portllm.AdapterXAIVideo
	if input.VideoExtensionSource != nil {
		return nil, fmt.Errorf("xai video generation protocol does not accept an extension source")
	}
	route.Endpoint = portllm.EndpointVideoGenerations
	return a.client.generateXAIVideo(ctx, route, input)
}

func (a *xAIVideoAdapter) GenerateStream(
	context.Context,
	portllm.RouteConfig,
	portllm.GenerateInput,
	func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	return nil, fmt.Errorf("%w: %s", portllm.ErrUnsupportedStream, portllm.AdapterXAIVideo)
}

func (a *xAIVideoAdapter) ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	route.Protocol = portllm.AdapterXAIVideo
	return a.client.listModelsOpenAICompatible(ctx, route)
}

// xAIVideoExtensionsAdapter 实现 xAI 异步视频扩展协议。
type xAIVideoExtensionsAdapter struct {
	client *Client
}

func (a *xAIVideoExtensionsAdapter) Name() string { return portllm.AdapterXAIVideoExtensions }

func (a *xAIVideoExtensionsAdapter) Generate(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	if input.VideoExtensionSource == nil {
		return nil, fmt.Errorf("xai video extension protocol requires an MP4 source")
	}
	route.Protocol = portllm.AdapterXAIVideoExtensions
	route.Endpoint = portllm.EndpointVideoExtensions
	return a.client.generateXAIVideo(ctx, route, input)
}

func (a *xAIVideoExtensionsAdapter) GenerateStream(
	context.Context,
	portllm.RouteConfig,
	portllm.GenerateInput,
	func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	return nil, fmt.Errorf("%w: %s", portllm.ErrUnsupportedStream, portllm.AdapterXAIVideoExtensions)
}

func (a *xAIVideoExtensionsAdapter) ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	route.Protocol = portllm.AdapterXAIVideoExtensions
	return a.client.listModelsOpenAICompatible(ctx, route)
}

// generateXAIVideo 提交视频任务，并在同一请求超时范围内轮询官方结果端点。
func (c *Client) generateXAIVideo(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	requestBody, debugBody, err := buildXAIVideoSubmissionBody(route.UpstreamModel, input)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	requestURL := buildOpenAIRequestURL(route.BaseURL, route.Endpoint)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	req, err := newXAIMediaRequest(requestCtx, http.MethodPost, requestURL, payload, route)
	if err != nil {
		return nil, err
	}
	resp, err := c.doRouteGenerationRequest(route, req)
	if err != nil {
		return nil, err
	}
	body, readErr := readUpstreamBody(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			debug := upstreamDebugSnapshot(req, debugBody, resp, body)
			return nil, acceptedXAIVideoResponseError(readErr, debug)
		}
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseUpstreamError(resp.StatusCode, body, upstreamDebugSnapshot(req, debugBody, resp, body))
	}

	requestID, err := parseXAIVideoRequestID(body)
	if err != nil {
		return nil, acceptedXAIVideoResponseError(err, upstreamDebugSnapshot(req, debugBody, resp, body))
	}
	return c.pollXAIVideoResult(requestCtx, route, requestID, generatedMediaDurationSeconds(requestBody["duration"]))
}

func buildXAIVideoSubmissionBody(model string, input portllm.GenerateInput) (map[string]any, []byte, error) {
	if input.VideoExtensionSource != nil {
		return buildXAIVideoExtensionRequestBody(model, input)
	}
	return buildXAIVideoRequestBody(model, input)
}

func buildXAIVideoExtensionRequestBody(model string, input portllm.GenerateInput) (map[string]any, []byte, error) {
	prompt := strings.TrimSpace(buildOpenAIImageGenerationPrompt(input.Messages))
	source := input.VideoExtensionSource
	if prompt == "" {
		return nil, nil, fmt.Errorf("video extension prompt required")
	}
	if source == nil || source.Kind != portllm.ContentPartVideo || strings.ToLower(strings.TrimSpace(source.MimeType)) != "video/mp4" || len(source.Data) == 0 {
		return nil, nil, fmt.Errorf("video extension source must be a non-empty MP4 video")
	}
	payload := map[string]any{
		"model":  strings.TrimSpace(model),
		"prompt": prompt,
		"video": map[string]any{
			"url": "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(source.Data),
		},
	}
	applyXAIVideoExtensionParams(payload, input.Options)
	debugPayload := map[string]any{
		"model":        payload["model"],
		"prompt":       payload["prompt"],
		"video_source": "data:video/mp4;base64,[REDACTED]",
	}
	if duration, ok := payload["duration"]; ok {
		debugPayload["duration"] = duration
	}
	debugBody, _ := json.Marshal(debugPayload)
	return payload, debugBody, nil
}

func newXAIMediaRequest(ctx context.Context, method string, requestURL string, payload []byte, route portllm.RouteConfig) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	if len(payload) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey := strings.TrimSpace(route.APIKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	setAdditionalHeaders(req, route.HeadersJSON)
	return req, nil
}

func buildXAIVideoRequestBody(model string, input portllm.GenerateInput) (map[string]any, []byte, error) {
	prompt := buildOpenAIImageGenerationPrompt(input.Messages)
	images := collectImageInputParts(input.Messages)
	if strings.TrimSpace(prompt) == "" && len(images) == 0 {
		return nil, nil, fmt.Errorf("video generation prompt or input image required")
	}
	if len(images) > 1 {
		return nil, nil, fmt.Errorf("too many video generation input images")
	}

	payload := map[string]any{
		"model": strings.TrimSpace(model),
	}
	if strings.TrimSpace(prompt) != "" {
		payload["prompt"] = strings.TrimSpace(prompt)
	}
	if len(images) == 1 {
		payload["image"] = xAIVideoImagePayload(images[0])
	}
	applyXAIVideoParams(payload, input.Options)

	debugPayload := make(map[string]any, len(payload))
	for key, value := range payload {
		if key != "image" {
			debugPayload[key] = value
		}
	}
	debugPayload["image_count"] = len(images)
	debugBody, _ := json.Marshal(debugPayload)
	return payload, debugBody, nil
}

func xAIVideoImagePayload(image portllm.ContentPart) map[string]any {
	mimeType := strings.ToLower(strings.TrimSpace(image.MimeType))
	switch mimeType {
	case "image/png", "image/webp", "image/jpeg":
	default:
		mimeType = "image/jpeg"
	}
	return map[string]any{
		"url": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(image.Data),
	}
}

func applyXAIVideoParams(payload map[string]any, options map[string]any) {
	normalized := maps.Clone(options)
	portllm.SanitizeXAIVideoOptions(normalized)
	for _, key := range []string{"aspect_ratio", "duration", "resolution"} {
		if value, ok := normalized[key]; ok {
			payload[key] = value
		}
	}
}

func applyXAIVideoExtensionParams(payload map[string]any, options map[string]any) {
	normalized := maps.Clone(options)
	portllm.SanitizeXAIVideoExtensionOptions(normalized)
	if duration, ok := normalized["duration"]; ok {
		payload["duration"] = duration
	}
}

func parseXAIVideoRequestID(body []byte) (string, error) {
	parsed := make(map[string]any)
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	requestID := strings.TrimSpace(getString(parsed["request_id"]))
	if requestID == "" {
		return "", fmt.Errorf("xAI video response missing request_id")
	}
	return requestID, nil
}

func (c *Client) pollXAIVideoResult(ctx context.Context, route portllm.RouteConfig, requestID string, requestedDurationSeconds int64) (*portllm.GenerateOutput, error) {
	requestURL := buildXAIVideoResultURL(route.BaseURL, requestID)
	if requestURL == "" {
		return nil, portllm.MarkRequestAccepted(fmt.Errorf("invalid xAI video result url"))
	}

	for {
		req, err := newXAIMediaRequest(ctx, http.MethodGet, requestURL, nil, route)
		if err != nil {
			return nil, portllm.MarkRequestAccepted(err)
		}
		resp, err := c.doRouteRequest(route, req)
		if err != nil {
			return nil, portllm.MarkRequestAccepted(err)
		}
		body, readErr := readUpstreamBody(resp.Body)
		_ = resp.Body.Close()
		debug := upstreamDebugSnapshot(req, nil, resp, body)
		if readErr != nil {
			return nil, acceptedXAIVideoResponseError(readErr, debug)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			return nil, portllm.MarkRequestAccepted(parseUpstreamError(resp.StatusCode, body, debug))
		}

		output, pending, err := parseXAIVideoResult(body, requestID, requestedDurationSeconds)
		if err != nil {
			return nil, acceptedXAIVideoResponseError(err, debug)
		}
		if !pending {
			output.Debug = debug
			return output, nil
		}
		if err := waitXAIVideoPoll(ctx, xAIVideoPollDelay(resp.Header.Get("Retry-After"))); err != nil {
			return nil, portllm.MarkRequestAccepted(err)
		}
	}
}

func buildXAIVideoResultURL(baseURL string, requestID string) string {
	id := strings.TrimSpace(requestID)
	if id == "" {
		return ""
	}
	return buildVersionedEndpointURL(baseURL, "v1", "/videos/"+url.PathEscape(id))
}

func parseXAIVideoResult(body []byte, requestID string, requestedDurationSeconds int64) (*portllm.GenerateOutput, bool, error) {
	parsed := make(map[string]any)
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, false, err
	}
	status := strings.ToLower(strings.TrimSpace(getString(parsed["status"])))
	switch status {
	case "pending":
		return nil, true, nil
	case "failed":
		errorPayload := asMap(parsed["error"])
		code := strings.TrimSpace(getString(errorPayload["code"]))
		message := strings.TrimSpace(getString(errorPayload["message"]))
		if message == "" {
			message = "xAI video generation failed"
		}
		if code != "" {
			message = code + ": " + message
		}
		return nil, false, fmt.Errorf("xAI video generation failed: %s", message)
	case "done":
	default:
		return nil, false, fmt.Errorf("unexpected xAI video status %q", status)
	}

	videoPayload := asMap(parsed["video"])
	if respectsModeration, ok := videoPayload["respect_moderation"].(bool); ok && !respectsModeration {
		return nil, false, fmt.Errorf("xAI video result was blocked by content moderation")
	}
	videoURL := strings.TrimSpace(getString(videoPayload["url"]))
	fileOutput := asMap(videoPayload["file_output"])
	if videoURL == "" {
		videoURL = strings.TrimSpace(getString(fileOutput["public_url"]))
	}
	if videoURL == "" {
		return nil, false, fmt.Errorf("xAI video result missing downloadable URL")
	}
	durationSeconds := generatedMediaDurationSeconds(
		videoPayload["duration_seconds"],
		videoPayload["duration"],
		fileOutput["duration_seconds"],
		parsed["duration_seconds"],
		parsed["duration"],
		requestedDurationSeconds,
	)
	result := &portllm.GenerateOutput{
		ResponseID:      strings.TrimSpace(requestID),
		ToolCalls:       make([]portllm.ToolCall, 0),
		ServerToolCalls: make([]portllm.ToolCall, 0),
		GeneratedVideos: []portllm.GeneratedVideo{{
			URL:             videoURL,
			MIMEType:        "video/mp4",
			FileName:        strings.TrimSpace(getString(fileOutput["filename"])),
			DurationSeconds: durationSeconds,
		}},
		RawJSON: string(body),
	}
	result.Usage.RawUsageJSON = rawUsageJSONFromPath(parsed, "usage")
	return result, false, nil
}

func acceptedXAIVideoResponseError(err error, debug *portllm.UpstreamDebugSnapshot) error {
	if err == nil {
		return nil
	}
	body := ""
	if debug != nil {
		body = debug.Response.Body
	}
	return portllm.MarkRequestAccepted(&portllm.UpstreamError{
		StatusCode: http.StatusBadGateway,
		Message:    strings.TrimSpace(err.Error()),
		Body:       body,
		Debug:      debug,
	})
}

func xAIVideoPollDelay(retryAfter string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter))
	if err != nil || seconds <= 0 {
		return defaultXAIVideoPollInterval
	}
	delay := time.Duration(seconds) * time.Second
	if delay > 10*time.Second {
		return 10 * time.Second
	}
	return delay
}

func waitXAIVideoPoll(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
