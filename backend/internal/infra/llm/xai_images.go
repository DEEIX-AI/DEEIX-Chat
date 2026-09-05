package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// xAIImageAdapter 实现 xAI 图片生成协议。
type xAIImageAdapter struct {
	client *Client
}

func (a *xAIImageAdapter) Name() string { return portllm.AdapterXAIImage }

// Generate 调用 xAI 图片生成接口，返回结构化图片结果。
func (a *xAIImageAdapter) Generate(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	route.Protocol = portllm.AdapterXAIImage
	route.Endpoint = portllm.EndpointImageGenerations
	return a.client.generateXAIImage(ctx, route, input)
}

// GenerateStream 当前不伪造图片流式；媒体任务会通过非流式调用落库生成结果。
func (a *xAIImageAdapter) GenerateStream(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	return nil, fmt.Errorf("%w: %s", portllm.ErrUnsupportedStream, portllm.AdapterXAIImage)
}

// ListModels 复用 xAI models 目录，供渠道校验和展示使用。
func (a *xAIImageAdapter) ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	route.Protocol = portllm.AdapterXAIImage
	return a.client.listModelsOpenAICompatible(ctx, route)
}

// xAIImageEditsAdapter 实现 xAI 图片编辑协议。
type xAIImageEditsAdapter struct {
	client *Client
}

func (a *xAIImageEditsAdapter) Name() string { return portllm.AdapterXAIImageEdits }

// Generate 调用 xAI 图片编辑接口，返回结构化图片结果。
func (a *xAIImageEditsAdapter) Generate(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	route.Protocol = portllm.AdapterXAIImageEdits
	route.Endpoint = portllm.EndpointImageEdits
	return a.client.generateXAIImage(ctx, route, input)
}

// GenerateStream 当前不伪造图片流式；媒体任务会通过非流式调用落库生成结果。
func (a *xAIImageEditsAdapter) GenerateStream(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	return nil, fmt.Errorf("%w: %s", portllm.ErrUnsupportedStream, portllm.AdapterXAIImageEdits)
}

// ListModels 复用 xAI models 目录，供渠道校验和展示使用。
func (a *xAIImageEditsAdapter) ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	route.Protocol = portllm.AdapterXAIImageEdits
	return a.client.listModelsOpenAICompatible(ctx, route)
}

// generateXAIImage 构造并执行 xAI Images API 请求。
func (c *Client) generateXAIImage(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	protocol := portllm.NormalizeAdapter(route.Protocol)
	if protocol != portllm.AdapterXAIImage && protocol != portllm.AdapterXAIImageEdits {
		protocol = portllm.AdapterXAIImage
	}
	route.Protocol = protocol
	endpoint := portllm.DefaultEndpointForAdapter(protocol)
	requestURL := buildOpenAIRequestURL(route.BaseURL, endpoint)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	requestBody, debugBody, err := buildXAIImageRequest(route.UpstreamModel, endpoint, input)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	debugPayload := payload
	if len(debugBody) > 0 {
		debugPayload = debugBody
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
	setAdditionalHeaders(req, route.HeadersJSON)

	resp, err := c.doRouteGenerationRequest(route, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := readUpstreamBody(resp.Body)
	if err != nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil, portllm.MarkRequestAccepted(attachUpstreamDebug(err, upstreamDebugSnapshot(req, debugPayload, resp, body)))
		}
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseUpstreamError(resp.StatusCode, body, upstreamDebugSnapshot(req, debugPayload, resp, body))
	}

	debug := upstreamDebugSnapshot(req, debugPayload, resp, body)
	output, err := parseXAIImageOutput(body, protocol)
	if err != nil {
		return nil, portllm.MarkRequestAccepted(attachUpstreamDebug(err, debug))
	}
	output.Debug = debug
	return output, nil
}

// buildXAIImageRequest 根据任务端点构造 xAI 图片生成或编辑请求。
func buildXAIImageRequest(model string, endpoint string, input portllm.GenerateInput) (map[string]any, []byte, error) {
	if endpoint == portllm.EndpointImageEdits {
		return buildXAIImageEditRequestBody(model, input)
	}
	payload, err := buildXAIImageRequestBody(model, input)
	return payload, nil, err
}

// buildXAIImageRequestBody 只允许 xAI 图片生成端点支持的字段进入上游。
func buildXAIImageRequestBody(model string, input portllm.GenerateInput) (map[string]any, error) {
	prompt := buildOpenAIImageGenerationPrompt(input.Messages)
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("image generation prompt required")
	}
	payload := map[string]any{
		"model":  strings.TrimSpace(model),
		"prompt": prompt,
	}
	applyXAIImageParams(payload, input.Options)
	return payload, nil
}

// buildXAIImageEditRequestBody 只允许 xAI 图片编辑端点支持的字段进入上游。
func buildXAIImageEditRequestBody(model string, input portllm.GenerateInput) (map[string]any, []byte, error) {
	prompt := buildOpenAIImageGenerationPrompt(input.Messages)
	if strings.TrimSpace(prompt) == "" {
		return nil, nil, fmt.Errorf("image edit prompt required")
	}
	images := collectImageInputParts(input.Messages)
	if len(images) == 0 {
		return nil, nil, fmt.Errorf("image edit input image required")
	}
	if len(images) > 3 {
		return nil, nil, fmt.Errorf("too many image edit input images")
	}
	imageInputs := make([]map[string]any, 0, len(images))
	for _, image := range images {
		imageInputs = append(imageInputs, xAIImageURLPayload(image))
	}
	payload := map[string]any{
		"model":  strings.TrimSpace(model),
		"prompt": prompt,
	}
	if len(imageInputs) == 1 {
		payload["image"] = imageInputs[0]
	} else {
		payload["images"] = imageInputs
	}
	applyXAIImageParams(payload, input.Options)
	debugBody, _ := json.Marshal(map[string]any{
		"model":       payload["model"],
		"prompt":      payload["prompt"],
		"image_count": len(imageInputs),
	})
	return payload, debugBody, nil
}

// xAIImageURLPayload 将内部图片输入转换为 xAI 文档要求的 image_url 对象。
func xAIImageURLPayload(image portllm.ContentPart) map[string]any {
	mimeType := strings.TrimSpace(image.MimeType)
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	return map[string]any{
		"url":  "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(image.Data),
		"type": "image_url",
	}
}

// applyXAIImageParams 从 options 中提取 xAI 图片生成官方参数。
func applyXAIImageParams(payload map[string]any, options map[string]any) {
	if value := xAIImageResponseFormat(options); value != "" {
		payload["response_format"] = value
	}
	if value := strings.ToLower(modelParamString(options, "aspect_ratio")); isXAIImageAspectRatio(value) {
		payload["aspect_ratio"] = value
	}
	if value := strings.ToLower(modelParamString(options, "resolution")); isXAIImageResolution(value) {
		payload["resolution"] = value
	}
	if value, ok := portllm.IntegerOption(options, "n"); ok && value >= 1 && value <= 10 {
		payload["n"] = value
	}
}

func isXAIImageAspectRatio(value string) bool {
	switch value {
	case "1:1", "3:4", "4:3", "9:16", "16:9", "2:3", "3:2", "9:19.5", "19.5:9", "9:20", "20:9", "1:2", "2:1", "auto":
		return true
	default:
		return false
	}
}

func isXAIImageResolution(value string) bool {
	return value == "1k" || value == "2k"
}

func xAIImageResponseFormat(options map[string]any) string {
	switch strings.ToLower(modelParamString(options, "response_format")) {
	case "url":
		return "url"
	case "b64_json":
		return "b64_json"
	default:
		return ""
	}
}

// parseXAIImageOutput 解析 xAI 图片响应；图片字节只进入 GeneratedImages。
func parseXAIImageOutput(body []byte, protocol string) (*portllm.GenerateOutput, error) {
	parsed := make(map[string]any)
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	result := &portllm.GenerateOutput{
		ResponseID:      strings.TrimSpace(getString(parsed["id"])),
		ToolCalls:       make([]portllm.ToolCall, 0),
		ServerToolCalls: make([]portllm.ToolCall, 0),
		RawJSON:         string(body),
	}
	if usage := parseOpenAICompatibleUsageForAdapter(protocol, parsed); usage != (portllm.Usage{}) {
		result.Usage = usage
	}
	data := asSlice(parsed["data"])
	citations := make([]string, 0, len(data))
	for _, item := range data {
		if image, ok := parseXAIImagePayload(asMap(item)); ok {
			if url := strings.TrimSpace(image.URL); url != "" {
				citations = append(citations, url)
			}
			result.GeneratedImages = append(result.GeneratedImages, image)
		}
	}
	if len(data) == 0 {
		if image, ok := parseXAIImagePayload(parsed); ok {
			if url := strings.TrimSpace(image.URL); url != "" {
				citations = append(citations, url)
			}
			result.GeneratedImages = append(result.GeneratedImages, image)
		}
	}
	result.Citations = appendUniqueStrings(result.Citations, citations...)
	return result, nil
}

func parseXAIImagePayload(payload map[string]any) (portllm.GeneratedImage, bool) {
	if len(payload) == 0 {
		return portllm.GeneratedImage{}, false
	}
	mimeType := xAIImageMIMEType(payload)
	revisedPrompt := strings.TrimSpace(getString(payload["revised_prompt"]))
	if revisedPrompt == "" {
		revisedPrompt = strings.TrimSpace(getString(payload["revisedPrompt"]))
	}
	if url := strings.TrimSpace(getString(payload["url"])); url != "" {
		return portllm.GeneratedImage{
			URL:           url,
			MIMEType:      mimeType,
			RevisedPrompt: revisedPrompt,
		}, true
	}
	if b64 := strings.TrimSpace(getString(payload["b64_json"])); b64 != "" {
		return portllm.GeneratedImage{
			B64JSON:       b64,
			MIMEType:      mimeType,
			RevisedPrompt: revisedPrompt,
		}, true
	}
	if publicURL := strings.TrimSpace(getString(asMap(payload["file_output"])["public_url"])); publicURL != "" {
		return portllm.GeneratedImage{
			URL:           publicURL,
			MIMEType:      mimeType,
			RevisedPrompt: revisedPrompt,
		}, true
	}
	return portllm.GeneratedImage{}, false
}

// xAIImageMIMEType 优先采用官方响应中的 MIME；旧代理未返回时回退为 JPEG。
func xAIImageMIMEType(payload map[string]any) string {
	if mimeType := strings.ToLower(strings.TrimSpace(getString(payload["mime_type"]))); strings.HasPrefix(mimeType, "image/") {
		return mimeType
	}
	return "image/jpeg"
}
