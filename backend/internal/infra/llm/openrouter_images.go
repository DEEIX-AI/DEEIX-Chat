package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// openRouterImageMaxInputReferences 是 OpenRouter Image API 文档给出的 input_references 上限。
const openRouterImageMaxInputReferences = 16

// openRouterImageSizePattern 匹配 size 的显式像素写法（如 2048x2048）。
var openRouterImageSizePattern = regexp.MustCompile(`^[0-9]{2,5}x[0-9]{2,5}$`)

// openRouterImagesAdapter 实现 OpenRouter 统一图片端点（POST /v1/images）。
// 文生图与参考图编辑共用同一端点：编辑输入以 input_references 传入，因此该协议同时
// 服务 image_gen 与 image_edit 两种模型能力。
type openRouterImagesAdapter struct {
	client *Client
}

func (a *openRouterImagesAdapter) Name() string { return portllm.AdapterOpenRouterImages }

// Generate 调用 OpenRouter 图片端点（非流式），返回结构化图片结果。
func (a *openRouterImagesAdapter) Generate(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	route = normalizeOpenRouterImagesRoute(route)
	return a.client.generateOpenRouterImage(ctx, route, input, false, nil)
}

// GenerateStream 调用 OpenRouter 图片端点并以 SSE 接收部分图片；对不支持原生流式的
// 提供商，上游会忽略 stream 返回缓冲 JSON，由共享传输层退化为非流式解析。
func (a *openRouterImagesAdapter) GenerateStream(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	route = normalizeOpenRouterImagesRoute(route)
	return a.client.generateOpenRouterImage(ctx, route, input, true, onEvent)
}

// ListModels 读取 OpenRouter 专用的图片模型目录（GET /v1/images/models），
// 该目录只包含输出图片的模型，比通用 /models 更贴合渠道校验用途。
func (a *openRouterImagesAdapter) ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	route = normalizeOpenRouterImagesRoute(route)
	return a.client.listModelsFromURL(ctx, route, buildOpenRouterImageModelsURL(route.BaseURL))
}

// normalizeOpenRouterImagesRoute 固定 OpenRouter 图片协议与端点，忽略调用方传入的其它端点值。
func normalizeOpenRouterImagesRoute(route portllm.RouteConfig) portllm.RouteConfig {
	route.Protocol = portllm.AdapterOpenRouterImages
	route.Endpoint = portllm.EndpointImages
	return route
}

func buildOpenRouterImageModelsURL(baseURL string) string {
	return buildVersionedEndpointURL(baseURL, "v1", "/images/models")
}

// generateOpenRouterImage 构造 OpenRouter 图片请求并交给共享的 OpenAI Images 形状传输层执行。
func (c *Client) generateOpenRouterImage(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	stream bool,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	requestURL := buildOpenAIRequestURL(route.BaseURL, portllm.EndpointImages)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}
	requestBody, debugBody, err := buildOpenRouterImageRequestBody(route.UpstreamModel, input, stream)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}
	outputFormat := modelParamString(input.Options, "output_format")
	if stream {
		return c.postOpenAIImageJSONStream(ctx, route, requestURL, payload, debugBody, outputFormat, onEvent)
	}
	return c.postOpenAIImageJSON(ctx, route, requestURL, payload, debugBody, outputFormat)
}

// buildOpenRouterImageRequestBody 只允许 OpenRouter Image API 文档列出的字段进入上游。
// 消息中的图片输入转换为 input_references；返回的 debugBody 用引用数量替代图片字节，避免快照泄漏原图。
// Image API 没有掩码输入，与 xAI / Gemini 图片协议一致，input.ImageEditMask 不会被发送。
func buildOpenRouterImageRequestBody(model string, input portllm.GenerateInput, stream bool) (map[string]any, []byte, error) {
	prompt := buildOpenAIImageGenerationPrompt(input.Messages)
	if strings.TrimSpace(prompt) == "" {
		return nil, nil, fmt.Errorf("image generation prompt required")
	}
	references := collectImageInputParts(input.Messages)
	if len(references) > openRouterImageMaxInputReferences {
		return nil, nil, fmt.Errorf("too many image reference inputs")
	}

	payload := map[string]any{
		"model":  strings.TrimSpace(model),
		"prompt": prompt,
	}
	if len(references) > 0 {
		inputReferences := make([]map[string]any, 0, len(references))
		for _, image := range references {
			inputReferences = append(inputReferences, openRouterImageReferencePayload(image))
		}
		payload["input_references"] = inputReferences
	}
	applyOpenRouterImageParams(payload, input.Options)
	if stream {
		payload["stream"] = true
	}

	debugPayload := make(map[string]any, len(payload))
	for key, value := range payload {
		if key == "input_references" {
			debugPayload["input_reference_count"] = len(references)
			continue
		}
		debugPayload[key] = value
	}
	debugBody, err := json.Marshal(debugPayload)
	if err != nil {
		return nil, nil, err
	}
	return payload, debugBody, nil
}

// openRouterImageReferencePayload 将内部图片输入转换为 OpenRouter 要求的 image_url 内容块。
func openRouterImageReferencePayload(image portllm.ContentPart) map[string]any {
	mimeType := strings.TrimSpace(image.MimeType)
	if mimeType == "" {
		mimeType = "image/png"
	}
	return map[string]any{
		"type": "image_url",
		"image_url": map[string]any{
			"url": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(image.Data),
		},
	}
}

// applyOpenRouterImageParams 从 options 中提取 OpenRouter Image API 的官方参数，
// 枚举值按文档规范化，越界或未知取值直接丢弃而不是透传给上游。
func applyOpenRouterImageParams(payload map[string]any, options map[string]any) {
	if value, ok := portllm.IntegerOption(options, "n"); ok && value >= 1 && value <= 10 {
		payload["n"] = value
	}
	if value := normalizeOpenRouterImageAspectRatio(modelParamString(options, "aspect_ratio")); value != "" {
		payload["aspect_ratio"] = value
	}
	if value := normalizeOpenRouterImageResolution(modelParamString(options, "resolution")); value != "" {
		payload["resolution"] = value
	}
	if value := normalizeOpenRouterImageSize(modelParamString(options, "size")); value != "" {
		payload["size"] = value
	}
	if value := strings.ToLower(modelParamString(options, "quality")); isOpenRouterImageQuality(value) {
		payload["quality"] = value
	}
	if value := normalizeOpenRouterImageOutputFormat(modelParamString(options, "output_format")); value != "" {
		payload["output_format"] = value
	}
	if value := strings.ToLower(modelParamString(options, "background")); isOpenRouterImageBackground(value) {
		payload["background"] = value
	}
	if value, ok := portllm.IntegerOption(options, "output_compression"); ok && value >= 0 && value <= 100 {
		payload["output_compression"] = value
	}
	if value, ok := portllm.IntegerOption(options, "seed"); ok {
		payload["seed"] = value
	}
	if value := modelParamString(options, "user"); value != "" {
		payload["user"] = value
	}
	if provider := openRouterImageProviderPreferences(options); len(provider) > 0 {
		payload["provider"] = provider
	}
}

func normalizeOpenRouterImageAspectRatio(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "1:1", "1:2", "1:4", "1:8", "2:1", "2:3", "3:2", "3:4", "4:1", "4:3", "4:5", "5:4", "8:1",
		"9:16", "16:9", "9:19.5", "19.5:9", "9:20", "20:9", "9:21", "21:9", "auto":
		return value
	default:
		return ""
	}
}

func normalizeOpenRouterImageResolution(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "512":
		return "512"
	case "1K":
		return "1K"
	case "2K":
		return "2K"
	case "4K":
		return "4K"
	default:
		return ""
	}
}

// normalizeOpenRouterImageSize 接受分辨率档位或显式像素（如 2048x2048）。
func normalizeOpenRouterImageSize(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if tier := normalizeOpenRouterImageResolution(trimmed); tier != "" {
		return tier
	}
	lowered := strings.ToLower(trimmed)
	if openRouterImageSizePattern.MatchString(lowered) {
		return lowered
	}
	return ""
}

func isOpenRouterImageQuality(value string) bool {
	switch value {
	case "auto", "low", "medium", "high", "xhigh", "max":
		return true
	default:
		return false
	}
}

// normalizeOpenRouterImageOutputFormat 只放行媒体落库链路能持久化的栅格格式；
// 文档中的 svg（矢量化模型）无法通过 detectGeneratedImageMIME 校验，请求时直接丢弃以免生成后入库失败。
func normalizeOpenRouterImageOutputFormat(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "png":
		return "png"
	case "jpeg", "jpg":
		return "jpeg"
	case "webp":
		return "webp"
	default:
		return ""
	}
}

func isOpenRouterImageBackground(value string) bool {
	switch value {
	case "auto", "transparent", "opaque":
		return true
	default:
		return false
	}
}

// isOpenRouterProviderSortStrategy 对应 OpenAPI ProviderSort 枚举。
func isOpenRouterProviderSortStrategy(value string) bool {
	switch value {
	case "price", "throughput", "latency", "exacto":
		return true
	default:
		return false
	}
}

// openRouterImageProviderSort 同时接受文档中的两种写法：策略字符串（ProviderSort）或 {by, partition} 对象（ProviderSortConfig）。
func openRouterImageProviderSort(raw any) any {
	switch value := raw.(type) {
	case string:
		strategy := strings.ToLower(strings.TrimSpace(value))
		if isOpenRouterProviderSortStrategy(strategy) {
			return strategy
		}
		return nil
	case map[string]any:
		config := make(map[string]any, 2)
		if by := strings.ToLower(strings.TrimSpace(getString(value["by"]))); isOpenRouterProviderSortStrategy(by) {
			config["by"] = by
		}
		if partition := strings.ToLower(strings.TrimSpace(getString(value["partition"]))); partition == "model" || partition == "none" {
			config["partition"] = partition
		}
		if len(config) == 0 {
			return nil
		}
		return config
	default:
		return nil
	}
}

// openRouterImageProviderPreferences 只保留 Image API 文档支持的 provider 路由字段。
func openRouterImageProviderPreferences(options map[string]any) map[string]any {
	source := asMap(options["provider"])
	if len(source) == 0 {
		return nil
	}
	provider := make(map[string]any, len(source))
	for _, key := range []string{"only", "order", "ignore"} {
		if values := stringSliceOption(source[key]); len(values) > 0 {
			provider[key] = values
		}
	}
	if sort := openRouterImageProviderSort(source["sort"]); sort != nil {
		provider["sort"] = sort
	}
	if allowFallbacks, ok := source["allow_fallbacks"].(bool); ok {
		provider["allow_fallbacks"] = allowFallbacks
	}
	if passthrough := asMap(source["options"]); len(passthrough) > 0 {
		provider["options"] = passthrough
	}
	return provider
}

// stringSliceOption 把 options 中的字符串数组规范化为去空白、去空项的切片。
func stringSliceOption(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(getString(item)); text != "" {
			result = append(result, text)
		}
	}
	return result
}
