package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
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

// openAIImageEditsAdapter 负责 OpenAI Images API 的图片编辑端点。
type openAIImageEditsAdapter struct {
	client *Client
}

func (a *openAIImageEditsAdapter) Name() string { return AdapterOpenAIImageEdits }

// Generate 调用 OpenAI 图片编辑接口，返回结构化图片结果。
func (a *openAIImageEditsAdapter) Generate(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error) {
	route.Endpoint = EndpointImageEdits
	return a.client.generateOpenAIImageEdits(ctx, route, input)
}

// GenerateStream 调用 OpenAI 图片编辑流式接口，事件只输出图片增量，不进入聊天 token delta 链路。
func (a *openAIImageEditsAdapter) GenerateStream(
	ctx context.Context,
	route RouteConfig,
	input GenerateInput,
	onEvent func(GenerateStreamEvent) error,
) (*GenerateOutput, error) {
	route.Endpoint = EndpointImageEdits
	return a.client.generateOpenAIImageEditsStream(ctx, route, input, onEvent)
}

// ListModels 复用 OpenAI 兼容 models 目录，供渠道校验和展示使用。
func (a *openAIImageEditsAdapter) ListModels(ctx context.Context, route RouteConfig) ([]ModelItem, error) {
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

	return parseOpenAIImageOutput(body, modelParamString(input.Options, "output_format"))
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
		output, parseErr := parseOpenAIImageOutput(body, outputFormat)
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
	if err = consumeOpenAIImageStream(streamBody, outputFormat, result, onEvent); err != nil {
		return nil, attachUpstreamDebug(err, upstreamDebugSnapshot(req, payload, resp, streamErrorBody(streamBody, err)))
	}
	return result, nil
}

func isEventStreamContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(contentType)), "text/event-stream")
}

// generateOpenAIImageEdits 构造并执行 OpenAI 图片编辑请求。
func (c *Client) generateOpenAIImageEdits(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error) {
	requestURL := buildOpenAIRequestURL(route.BaseURL, EndpointImageEdits)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	payload, contentType, debugBody, err := buildOpenAIImageEditMultipartRequest(route.UpstreamModel, input, false)
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
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
		return nil, parseUpstreamError(resp.StatusCode, body, upstreamDebugSnapshot(req, debugBody, resp, body))
	}

	return parseOpenAIImageOutput(body, modelParamString(input.Options, "output_format"))
}

// generateOpenAIImageEditsStream 构造并执行 OpenAI 图片编辑流式请求。
func (c *Client) generateOpenAIImageEditsStream(
	ctx context.Context,
	route RouteConfig,
	input GenerateInput,
	onEvent func(GenerateStreamEvent) error,
) (*GenerateOutput, error) {
	if !SupportsImageGenerationStream(route.Protocol, route.UpstreamModel) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedStream, AdapterOpenAIImageEdits)
	}
	requestURL := buildOpenAIRequestURL(route.BaseURL, EndpointImageEdits)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	payload, contentType, debugBody, err := buildOpenAIImageEditMultipartRequest(route.UpstreamModel, input, true)
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
	req.Header.Set("Content-Type", contentType)
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
		return nil, parseUpstreamError(resp.StatusCode, body, upstreamDebugSnapshot(req, debugBody, resp, body))
	}

	outputFormat := modelParamString(input.Options, "output_format")
	if !isEventStreamContentType(resp.Header.Get("Content-Type")) {
		body, readErr := readUpstreamBody(resp.Body)
		if readErr != nil {
			return nil, readErr
		}
		output, parseErr := parseOpenAIImageOutput(body, outputFormat)
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
	if err = consumeOpenAIImageStream(streamBody, outputFormat, result, onEvent); err != nil {
		return nil, attachUpstreamDebug(err, upstreamDebugSnapshot(req, debugBody, resp, streamErrorBody(streamBody, err)))
	}
	return result, nil
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

// buildOpenAIImageEditMultipartRequest 构造 OpenAI Images Edits 官方 multipart 请求体。
func buildOpenAIImageEditMultipartRequest(model string, input GenerateInput, stream bool) ([]byte, string, []byte, error) {
	prompt := buildOpenAIImageGenerationPrompt(input.Messages)
	if strings.TrimSpace(prompt) == "" {
		return nil, "", nil, fmt.Errorf("image edit prompt required")
	}
	images := collectImageInputParts(input.Messages)
	if len(images) == 0 {
		return nil, "", nil, fmt.Errorf("image edit input image required")
	}
	if stream && !openAIImageEditModelSupportsStream(model) {
		return nil, "", nil, fmt.Errorf("%w: %s", ErrUnsupportedStream, AdapterOpenAIImageEdits)
	}

	formFields := map[string]string{
		"model":  strings.TrimSpace(model),
		"prompt": prompt,
	}
	applyOpenAIImageEditParams(formFields, strings.TrimSpace(model), input.Options)
	if stream {
		formFields["stream"] = "true"
		applyOpenAIImageEditStreamParams(formFields, input.Options)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range formFields {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", nil, err
		}
	}
	for index, image := range images {
		fileName := image.FileName
		if strings.TrimSpace(fileName) == "" {
			fileName = fmt.Sprintf("image-%02d%s", index+1, openAIImageFileExtension(image.MimeType))
		}
		if err := writeOpenAIMultipartFile(writer, "image[]", fileName, image.MimeType, image.Data); err != nil {
			return nil, "", nil, err
		}
	}
	if input.ImageEditMask != nil && len(input.ImageEditMask.Data) > 0 {
		mask := *input.ImageEditMask
		fileName := mask.FileName
		if strings.TrimSpace(fileName) == "" {
			fileName = "mask" + openAIImageFileExtension(mask.MimeType)
		}
		if err := writeOpenAIMultipartFile(writer, "mask", fileName, mask.MimeType, mask.Data); err != nil {
			return nil, "", nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", nil, err
	}
	debugBody := buildOpenAIImageEditDebugBody(formFields, len(images), input.ImageEditMask != nil && len(input.ImageEditMask.Data) > 0)
	return body.Bytes(), writer.FormDataContentType(), debugBody, nil
}

func applyOpenAIImageEditParams(fields map[string]string, model string, options map[string]interface{}) {
	if openAIImageGenerationModelSupportsResponseFormat(model) {
		fields["response_format"] = defaultImageResponseFormat(options)
	}
	for _, key := range []string{"quality", "size", "user"} {
		if value := modelParamString(options, key); value != "" {
			fields[key] = value
		}
	}
	if value := modelParamInt(options, "n"); value > 0 {
		fields["n"] = fmt.Sprintf("%d", value)
	}
	if openAIImageEditModelSupportsGPTImageParams(model) {
		for _, key := range []string{"background", "input_fidelity", "output_format"} {
			if value := modelParamString(options, key); value != "" {
				fields[key] = value
			}
		}
		if value := modelParamInt(options, "output_compression"); value > 0 {
			fields["output_compression"] = fmt.Sprintf("%d", value)
		}
	}
}

func applyOpenAIImageEditStreamParams(fields map[string]string, options map[string]interface{}) {
	value, ok := modelParamIntValue(options, "partial_images")
	if !ok {
		return
	}
	if value > 0 {
		fields["partial_images"] = fmt.Sprintf("%d", value)
	}
}

func writeOpenAIMultipartFile(writer *multipart.Writer, fieldName string, fileName string, mimeType string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("multipart file %s is empty", fieldName)
	}
	normalizedMIME := strings.TrimSpace(mimeType)
	if normalizedMIME == "" {
		normalizedMIME = "image/png"
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeMultipartQuote(fieldName), escapeMultipartQuote(fileName)))
	header.Set("Content-Type", normalizedMIME)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = part.Write(data)
	return err
}

func buildOpenAIImageEditDebugBody(fields map[string]string, imageCount int, hasMask bool) []byte {
	payload := make(map[string]interface{}, len(fields)+2)
	for key, value := range fields {
		payload[key] = value
	}
	payload["image_count"] = imageCount
	payload["mask"] = hasMask
	raw, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{"multipart":true}`)
	}
	return raw
}

func escapeMultipartQuote(value string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(strings.TrimSpace(value))
}

// applyOpenAIImageGenerationParams 从 options 中提取官方 Images API 参数。
func applyOpenAIImageGenerationParams(payload map[string]interface{}, model string, options map[string]interface{}) {
	if openAIImageGenerationModelSupportsResponseFormat(model) {
		payload["response_format"] = defaultImageResponseFormat(options)
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
	if openAIImageGenerationModelSupportsStyle(model) {
		if value := modelParamString(options, "style"); value != "" {
			payload["style"] = value
		}
	}
}

func defaultImageResponseFormat(options map[string]interface{}) string {
	if value := modelParamString(options, "response_format"); value != "" {
		return value
	}
	return "b64_json"
}

// applyOpenAIImageGenerationStreamParams 只处理流式图片端点支持的增量参数。
func applyOpenAIImageGenerationStreamParams(payload map[string]interface{}, options map[string]interface{}) {
	value, ok := modelParamIntValue(options, "partial_images")
	if !ok {
		return
	}
	if value > 0 {
		payload["partial_images"] = value
	}
}

func openAIImageGenerationModelSupportsStream(model string) bool {
	return openAIImageGenerationModelSupportsGPTImageParams(model)
}

func openAIImageGenerationModelSupportsGPTImageParams(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-image-")
}

func openAIImageEditModelSupportsStream(model string) bool {
	return openAIImageEditModelSupportsGPTImageParams(model)
}

func openAIImageEditModelSupportsGPTImageParams(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(normalized, "gpt-image-") || normalized == "chatgpt-image-latest"
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

// parseOpenAIImageOutput 解析 OpenAI 图片响应。
// 图片字节只放入 GeneratedImages，避免把 data URL 写入普通文本链路。
func parseOpenAIImageOutput(body []byte, outputFormat string) (*GenerateOutput, error) {
	parsed := make(map[string]interface{})
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	result := &GenerateOutput{
		ToolCalls:       make([]ToolCall, 0),
		ServerToolCalls: make([]ToolCall, 0),
		RawJSON:         string(body),
	}
	applyOpenAIImageCompletedPayload(parsed, outputFormat, result)
	return result, nil
}

func applyOpenAIImageCompletedPayload(parsed map[string]interface{}, outputFormat string, result *GenerateOutput) {
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
			applyOpenAIImageCompletedPayload(response, outputFormat, result)
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

func consumeOpenAIImageStream(
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
			return emitOpenAIImagePartial(parsed, outputFormat, onEvent)
		case strings.Contains(eventType, "completed") || strings.Contains(eventType, "final"):
			applyOpenAIImageCompletedPayload(parsed, outputFormat, result)
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
		applyOpenAIImageCompletedPayload(parsed, outputFormat, result)
		if result.Usage != (Usage{}) && onEvent != nil {
			if err := onEvent(GenerateStreamEvent{Usage: result.Usage}); err != nil {
				return err
			}
		}
	}
	return nil
}

func emitOpenAIImagePartial(
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
	index := firstNonZero(
		getInt64FromPath(parsed, "partial_image_index"),
		getInt64FromPath(parsed, "index"),
	)
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

func openAIImageFileExtension(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}
