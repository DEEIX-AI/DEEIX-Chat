package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// openAIImageGenerationsAdapter 负责 OpenAI Images API 的图片生成端点。
type openAIImageGenerationsAdapter struct {
	client *Client
}

func (a *openAIImageGenerationsAdapter) Name() string { return AdapterOpenAIImageGenerations }

// Generate 调用 OpenAI 图片生成接口，返回结构化图片结果。
func (a *openAIImageGenerationsAdapter) Generate(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error) {
	route.Endpoint = EndpointImageGenerations
	return a.client.generateOpenAIImageGenerations(ctx, route, input)
}

// GenerateStream 调用 OpenAI 图片生成流式接口，事件只输出图片增量，不进入聊天 token delta 链路。
func (a *openAIImageGenerationsAdapter) GenerateStream(
	ctx context.Context,
	route RouteConfig,
	input GenerateInput,
	onEvent func(GenerateStreamEvent) error,
) (*GenerateOutput, error) {
	route.Endpoint = EndpointImageGenerations
	return a.client.generateOpenAIImageGenerationsStream(ctx, route, input, onEvent)
}

// ListModels 复用 OpenAI 兼容 models 目录，供渠道校验和展示使用。
func (a *openAIImageGenerationsAdapter) ListModels(ctx context.Context, route RouteConfig) ([]ModelItem, error) {
	return a.client.listModelsOpenAICompatible(ctx, route)
}

// generateOpenAIImageGenerations 构造并执行 OpenAI 图片生成请求。
func (c *Client) generateOpenAIImageGenerations(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error) {
	requestURL := buildOpenAIRequestURL(route.BaseURL, EndpointImageGenerations)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	requestBody, err := buildOpenAIImageGenerationRequestBody(route.UpstreamModel, input)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey := strings.TrimSpace(route.APIKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	setOpenRouterAttributionHeaders(req, route)
	setAdditionalHeaders(req, route.HeadersJSON)

	resp, err := c.httpClientForRoute(route).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := readUpstreamBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseUpstreamError(resp.StatusCode, body, upstreamDebugSnapshot(req, payload, resp, body))
	}

	return parseOpenAIImageGenerationOutput(body, modelParamString(input.Options, "output_format"))
}

// generateOpenAIImageGenerationsStream 构造并执行 OpenAI 图片生成流式请求。
func (c *Client) generateOpenAIImageGenerationsStream(
	ctx context.Context,
	route RouteConfig,
	input GenerateInput,
	onEvent func(GenerateStreamEvent) error,
) (*GenerateOutput, error) {
	if !SupportsImageGenerationStream(route.Protocol, route.UpstreamModel) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedStream, AdapterOpenAIImageGenerations)
	}
	requestURL := buildOpenAIRequestURL(route.BaseURL, EndpointImageGenerations)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	requestBody, err := buildOpenAIImageGenerationStreamRequestBody(route.UpstreamModel, input)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	firstByteCtx, firstByteCancel := context.WithCancel(ctx)
	defer firstByteCancel()

	readTimeout := resolveReadTimeout(route.ReadTimeoutMS)
	firstByteTimer := time.AfterFunc(readTimeout, func() {
		firstByteCancel()
	})

	req, err := http.NewRequestWithContext(firstByteCtx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		firstByteTimer.Stop()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if apiKey := strings.TrimSpace(route.APIKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	setOpenRouterAttributionHeaders(req, route)
	setAdditionalHeaders(req, route.HeadersJSON)

	resp, err := c.httpClientForRoute(route).Do(req)
	firstByteTimer.Stop()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := readUpstreamBody(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		return nil, parseUpstreamError(resp.StatusCode, body, upstreamDebugSnapshot(req, payload, resp, body))
	}

	outputFormat := modelParamString(input.Options, "output_format")
	if !isEventStreamContentType(resp.Header.Get("Content-Type")) {
		body, readErr := readUpstreamBody(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		output, parseErr := parseOpenAIImageGenerationOutput(body, outputFormat)
		if parseErr != nil {
			return nil, parseErr
		}
		if output.Usage != (Usage{}) && onEvent != nil {
			if err := onEvent(GenerateStreamEvent{Usage: output.Usage}); err != nil {
				return nil, err
			}
		}
		return output, nil
	}

	result := &GenerateOutput{
		ResponseID:      "",
		Usage:           Usage{},
		ToolCalls:       make([]ToolCall, 0),
		ServerToolCalls: make([]ToolCall, 0),
	}
	idleTimeout := resolveStreamIdleTimeout(route.StreamIdleTimeoutMS)
	idleReader := newIdleTimeoutReader(resp.Body, idleTimeout)
	streamBody := newUpstreamBodyRecorder(idleReader)
	if err = consumeOpenAIImageGenerationStream(streamBody, outputFormat, result, onEvent); err != nil {
		return nil, attachUpstreamDebug(err, upstreamDebugSnapshot(req, payload, resp, streamErrorBody(streamBody, err)))
	}
	return result, nil
}

func isEventStreamContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(contentType)), "text/event-stream")
}

// buildOpenAIImageGenerationRequestBody 只允许图片端点支持的请求字段进入上游。
func buildOpenAIImageGenerationRequestBody(model string, input GenerateInput) (map[string]interface{}, error) {
	prompt := buildOpenAIImageGenerationPrompt(input.Messages)
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("image generation prompt required")
	}
	payload := map[string]interface{}{
		"model":  strings.TrimSpace(model),
		"prompt": prompt,
	}
	applyOpenAIImageGenerationParams(payload, strings.TrimSpace(model), input.Options)
	return payload, nil
}

// buildOpenAIImageGenerationStreamRequestBody 只在流式 adapter 内写入 stream / partial_images。
func buildOpenAIImageGenerationStreamRequestBody(model string, input GenerateInput) (map[string]interface{}, error) {
	payload, err := buildOpenAIImageGenerationRequestBody(model, input)
	if err != nil {
		return nil, err
	}
	if !openAIImageGenerationModelSupportsStream(model) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedStream, AdapterOpenAIImageGenerations)
	}
	payload["stream"] = true
	applyOpenAIImageGenerationStreamParams(payload, input.Options)
	return payload, nil
}

// applyOpenAIImageGenerationParams 从 options 中提取官方 Images API 参数。
func applyOpenAIImageGenerationParams(payload map[string]interface{}, model string, options map[string]interface{}) {
	if len(options) == 0 {
		return
	}
	for _, key := range []string{"quality", "size", "user"} {
		if value := modelParamString(options, key); value != "" {
			payload[key] = value
		}
	}
	if value := modelParamInt(options, "n"); value > 0 {
		payload["n"] = value
	}
	if openAIImageGenerationModelSupportsGPTImageParams(model) {
		for _, key := range []string{"background", "moderation", "output_format"} {
			if value := modelParamString(options, key); value != "" {
				payload[key] = value
			}
		}
		if value := modelParamInt(options, "output_compression"); value > 0 {
			payload["output_compression"] = value
		}
	}
	if openAIImageGenerationModelSupportsResponseFormat(model) {
		if value := modelParamString(options, "response_format"); value != "" {
			payload["response_format"] = value
		}
	}
	if openAIImageGenerationModelSupportsStyle(model) {
		if value := modelParamString(options, "style"); value != "" {
			payload["style"] = value
		}
	}
}

// applyOpenAIImageGenerationStreamParams 只处理流式图片端点支持的增量参数。
func applyOpenAIImageGenerationStreamParams(payload map[string]interface{}, options map[string]interface{}) {
	value, ok := modelParamIntValue(options, "partial_images")
	if ok {
		if value > 0 {
			payload["partial_images"] = value
		}
		return
	}
	payload["partial_images"] = 1
}

func openAIImageGenerationModelSupportsStream(model string) bool {
	return openAIImageGenerationModelSupportsGPTImageParams(model)
}

func openAIImageGenerationModelSupportsGPTImageParams(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-image-")
}

func openAIImageGenerationModelSupportsResponseFormat(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "dall-e-2", "dall-e-3":
		return true
	default:
		return false
	}
}

func openAIImageGenerationModelSupportsStyle(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), "dall-e-3")
}

// buildOpenAIImageGenerationPrompt 使用最后一条用户文本作为图片提示词。
func buildOpenAIImageGenerationPrompt(messages []Message) string {
	for index := len(messages) - 1; index >= 0; index-- {
		msg := messages[index]
		if normalizeRole(msg.Role) != "user" {
			continue
		}
		if text := strings.TrimSpace(messagePromptText(msg)); text != "" {
			return text
		}
	}
	for index := len(messages) - 1; index >= 0; index-- {
		if text := strings.TrimSpace(messagePromptText(messages[index])); text != "" {
			return text
		}
	}
	return ""
}

// messagePromptText 抽取消息中的可读文本，文件文本只作为提示词补充。
func messagePromptText(msg Message) string {
	if len(msg.Parts) == 0 {
		return msg.Content
	}
	parts := make([]string, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		switch part.Kind {
		case ContentPartText, ContentPartFile:
			if text := strings.TrimSpace(part.Text); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// parseOpenAIImageGenerationOutput 解析 OpenAI 图片响应。
// 图片字节只放入 GeneratedImages，避免把 data URL 写入普通文本链路。
func parseOpenAIImageGenerationOutput(body []byte, outputFormat string) (*GenerateOutput, error) {
	parsed := make(map[string]interface{})
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	result := &GenerateOutput{
		ToolCalls:       make([]ToolCall, 0),
		ServerToolCalls: make([]ToolCall, 0),
		RawJSON:         string(body),
	}
	applyOpenAIImageGenerationCompletedPayload(parsed, outputFormat, result)
	return result, nil
}

func applyOpenAIImageGenerationCompletedPayload(parsed map[string]interface{}, outputFormat string, result *GenerateOutput) {
	if result == nil {
		return
	}
	if result.ResponseID == "" {
		result.ResponseID = strings.TrimSpace(getString(parsed["id"]))
	}
	if result.ResponseID == "" {
		result.ResponseID = strings.TrimSpace(getStringFromPath(parsed, "response", "id"))
	}
	if usage := parseOpenAICompatibleUsageForAdapter(AdapterOpenAIImageGenerations, parsed); usage != (Usage{}) {
		result.Usage = usage
	}
	data, _ := parsed["data"].([]interface{})
	citations := make([]string, 0, len(data))
	for _, item := range data {
		if image, ok := parseOpenAIImagePayload(asMap(item), outputFormat); ok {
			if url := strings.TrimSpace(image.URL); url != "" {
				citations = append(citations, url)
			}
			result.GeneratedImages = append(result.GeneratedImages, image)
		}
	}
	if len(data) == 0 {
		if response := asMap(parsed["response"]); len(response) > 0 {
			applyOpenAIImageGenerationCompletedPayload(response, outputFormat, result)
			return
		}
	}
	if len(data) == 0 {
		if image, ok := parseOpenAIImagePayload(parsed, outputFormat); ok {
			if url := strings.TrimSpace(image.URL); url != "" {
				citations = append(citations, url)
			}
			result.GeneratedImages = append(result.GeneratedImages, image)
		}
	}
	result.Citations = appendUniqueStrings(result.Citations, citations...)
}

func parseOpenAIImagePayload(payload map[string]interface{}, outputFormat string) (GeneratedImage, bool) {
	if len(payload) == 0 {
		return GeneratedImage{}, false
	}
	if url := strings.TrimSpace(getString(payload["url"])); url != "" {
		return GeneratedImage{
			URL:           url,
			MIMEType:      openAIImageMIMEType(outputFormat),
			RevisedPrompt: strings.TrimSpace(getString(payload["revised_prompt"])),
		}, true
	}
	if b64 := strings.TrimSpace(getString(payload["b64_json"])); b64 != "" {
		return GeneratedImage{
			B64JSON:       b64,
			MIMEType:      openAIImageMIMEType(outputFormat),
			RevisedPrompt: strings.TrimSpace(getString(payload["revised_prompt"])),
		}, true
	}
	if nested := asMap(payload["image"]); len(nested) > 0 {
		image, ok := parseOpenAIImagePayload(nested, outputFormat)
		if !ok {
			return GeneratedImage{}, false
		}
		if image.RevisedPrompt == "" {
			image.RevisedPrompt = strings.TrimSpace(getString(payload["revised_prompt"]))
		}
		return image, true
	}
	return GeneratedImage{}, false
}

func consumeOpenAIImageGenerationStream(
	reader io.Reader,
	outputFormat string,
	result *GenerateOutput,
	onEvent func(GenerateStreamEvent) error,
) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 256*1024), 64*1024*1024)

	var eventName string
	dataLines := make([]string, 0, 4)
	rawLines := make([]string, 0, 16)

	dispatch := func() error {
		if len(dataLines) == 0 && strings.TrimSpace(eventName) == "" {
			return nil
		}
		currentEvent := strings.TrimSpace(eventName)
		payloadText := strings.Join(dataLines, "\n")
		eventName = ""
		dataLines = dataLines[:0]
		if strings.TrimSpace(payloadText) == "" {
			return nil
		}
		if strings.TrimSpace(payloadText) == "[DONE]" {
			return errStreamDone
		}
		if result != nil {
			if result.RawJSON != "" {
				result.RawJSON += "\n"
			}
			result.RawJSON += payloadText
		}

		parsed := make(map[string]interface{})
		if err := json.Unmarshal([]byte(payloadText), &parsed); err != nil {
			return err
		}
		if err := parseStreamUpstreamError(parsed, payloadText); err != nil {
			return err
		}

		eventType := strings.TrimSpace(getString(parsed["type"]))
		if eventType == "" {
			eventType = currentEvent
		}
		if eventType == "" {
			eventType = "image_generation.completed"
		}
		if responseID := strings.TrimSpace(getString(parsed["id"])); responseID != "" && result != nil && result.ResponseID == "" {
			result.ResponseID = responseID
		}
		usage := parseOpenAICompatibleUsageForAdapter(AdapterOpenAIImageGenerations, parsed)
		if usage == (Usage{}) {
			usage = parseOpenAICompatibleUsageForAdapter(AdapterOpenAIImageGenerations, asMap(parsed["response"]))
		}
		if usage != (Usage{}) {
			if result != nil {
				result.Usage = usage
			}
			if onEvent != nil {
				if err := onEvent(GenerateStreamEvent{Usage: usage}); err != nil {
					return err
				}
			}
		}

		switch {
		case strings.Contains(eventType, "partial_image"):
			return emitOpenAIImageGenerationPartial(parsed, outputFormat, onEvent)
		case strings.Contains(eventType, "completed") || strings.Contains(eventType, "final"):
			applyOpenAIImageGenerationCompletedPayload(parsed, outputFormat, result)
		}
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			if err := dispatch(); err != nil {
				if errors.Is(err, errStreamDone) {
					return nil
				}
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(line[len("event:"):])
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data := line[len("data:"):]
			data = strings.TrimPrefix(data, " ")
			dataLines = append(dataLines, data)
			continue
		}
		if !strings.HasPrefix(line, ":") {
			rawLines = append(rawLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	if err := dispatch(); err != nil && !errors.Is(err, errStreamDone) {
		return err
	}
	if len(rawLines) > 0 && result != nil && len(result.GeneratedImages) == 0 {
		parsed := make(map[string]interface{})
		payloadText := strings.TrimSpace(strings.Join(rawLines, "\n"))
		if payloadText == "" {
			return nil
		}
		if err := json.Unmarshal([]byte(payloadText), &parsed); err != nil {
			return err
		}
		if err := parseStreamUpstreamError(parsed, payloadText); err != nil {
			return err
		}
		if result.RawJSON != "" {
			result.RawJSON += "\n"
		}
		result.RawJSON += payloadText
		applyOpenAIImageGenerationCompletedPayload(parsed, outputFormat, result)
		if result.Usage != (Usage{}) && onEvent != nil {
			if err := onEvent(GenerateStreamEvent{Usage: result.Usage}); err != nil {
				return err
			}
		}
	}
	return nil
}

func emitOpenAIImageGenerationPartial(
	parsed map[string]interface{},
	outputFormat string,
	onEvent func(GenerateStreamEvent) error,
) error {
	if onEvent == nil {
		return nil
	}
	image, ok := parseOpenAIImagePayload(parsed, outputFormat)
	if !ok {
		return nil
	}
	index := int(firstNonZero(
		getInt64FromPath(parsed, "partial_image_index"),
		getInt64FromPath(parsed, "index"),
	))
	return onEvent(GenerateStreamEvent{
		GeneratedImage:        &image,
		GeneratedImageIndex:   index,
		GeneratedImagePartial: true,
		ResponseID:            strings.TrimSpace(getString(parsed["id"])),
	})
}

// openAIImageMIMEType 根据 output_format 推断上游图片 MIME。
func openAIImageMIMEType(outputFormat string) string {
	switch strings.TrimSpace(strings.ToLower(outputFormat)) {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return "image/png"
	}
}
