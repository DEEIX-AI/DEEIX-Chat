package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// geminiImageGenerationAdapter 实现 Google Gemini 图片生成/编辑协议。
type geminiImageGenerationAdapter struct {
	client *Client
}

func (a *geminiImageGenerationAdapter) Name() string { return portllm.AdapterGoogleImageGeneration }

// Generate 调用 Gemini generateContent 图片能力，返回结构化图片结果。
func (a *geminiImageGenerationAdapter) Generate(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	return a.client.generateGeminiImageGeneration(ctx, route, input)
}

// GenerateStream 调用 Gemini streamGenerateContent；最终图片仍由媒体任务落库。
func (a *geminiImageGenerationAdapter) GenerateStream(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	return a.client.generateGeminiImageGenerationStream(ctx, route, input, onEvent)
}

// ListModels 复用 Gemini models 目录，供渠道校验和展示使用。
func (a *geminiImageGenerationAdapter) ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	return a.client.listModelsGemini(ctx, route)
}

// generateGeminiImageGeneration 调用 generateContent，并强制请求图片模态输出。
func (c *Client) generateGeminiImageGeneration(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	base := geminiBaseURL(route)
	model := strings.TrimSpace(route.UpstreamModel)
	requestURL := buildGeminiGenerateURL(base, model)

	requestBody, err := buildGeminiImageGenerationRequestBody(model, input)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	req, err := c.newGeminiRequest(requestCtx, http.MethodPost, requestURL, bytes.NewReader(payload), route, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRouteRequest(route, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := readUpstreamBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseGeminiError(resp.StatusCode, body, upstreamDebugSnapshot(req, payload, resp, body))
	}

	return parseGeminiImageGenerationOutput(body)
}

// generateGeminiImageGenerationStream 调用 Gemini 图片 SSE 输出并复用 GenerateContentResponse 解析。
func (c *Client) generateGeminiImageGenerationStream(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	base := geminiBaseURL(route)
	model := strings.TrimSpace(route.UpstreamModel)
	requestURL := buildGeminiStreamURL(base, model)

	requestBody, err := buildGeminiImageGenerationRequestBody(model, input)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	firstByteCtx, firstByteCancel := context.WithCancel(ctx)
	defer firstByteCancel()

	firstByteTimer := time.AfterFunc(resolveReadTimeout(route.ReadTimeoutMS), firstByteCancel)
	defer firstByteTimer.Stop()

	req, err := c.newGeminiRequest(firstByteCtx, http.MethodPost, requestURL, bytes.NewReader(payload), route, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.doRouteRequest(route, req)
	firstByteTimer.Stop()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := readUpstreamBody(resp.Body)
		return nil, parseGeminiError(resp.StatusCode, body, upstreamDebugSnapshot(req, payload, resp, body))
	}

	result := &portllm.GenerateOutput{
		ToolCalls: make([]portllm.ToolCall, 0),
	}
	idleReader := newIdleTimeoutReader(resp.Body, resolveStreamIdleTimeout(route.StreamIdleTimeoutMS))
	streamBody := newUpstreamBodyRecorder(idleReader)
	if err = consumeGeminiStream(streamBody, result, onEvent); err != nil {
		return nil, attachUpstreamDebug(err, upstreamDebugSnapshot(req, payload, resp, streamErrorBody(streamBody, err)))
	}
	for i := range result.GeneratedImages {
		result.GeneratedImages[i].RevisedPrompt = strings.TrimSpace(result.Text)
	}
	return result, nil
}

// buildGeminiImageGenerationRequestBody 构造 Gemini 图片生成/编辑请求字段。
func buildGeminiImageGenerationRequestBody(model string, input portllm.GenerateInput) (map[string]any, error) {
	prompt := buildOpenAIImageGenerationPrompt(input.Messages)
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("image generation prompt required")
	}
	providerTools, _, toolsEnabled, err := toolDeclarationsForInput(input)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"contents": []map[string]any{
			{
				"role":  "user",
				"parts": buildGeminiImageGenerationParts(prompt, collectImageInputParts(input.Messages)),
			},
		},
		"generationConfig": buildGeminiImageGenerationConfig(model, input.Options),
	}
	if toolsEnabled {
		tools := buildGeminiProviderTools(providerTools)
		if len(tools) > 0 {
			payload["tools"] = tools
		}
	}
	return payload, nil
}

func buildGeminiImageGenerationConfig(model string, options map[string]any) map[string]any {
	generationConfig := map[string]any{
		"responseModalities": []string{"TEXT", "IMAGE"},
	}
	if len(options) == 0 {
		return generationConfig
	}
	rawConfig := modelParamMap(options, "generationConfig")
	if modalities := geminiImageResponseModalities(rawConfig["responseModalities"]); len(modalities) > 0 {
		generationConfig["responseModalities"] = modalities
	}
	if imageConfig := buildGeminiImageConfig(model, modelParamMap(rawConfig, "imageConfig")); len(imageConfig) > 0 {
		generationConfig["imageConfig"] = imageConfig
	}
	return generationConfig
}

func buildGeminiImageConfig(model string, raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	imageConfig := map[string]any{}
	if aspectRatio := geminiImageAspectRatio(getString(raw["aspectRatio"])); aspectRatio != "" {
		imageConfig["aspectRatio"] = aspectRatio
	}
	if imageSize := geminiImageSize(getString(raw["imageSize"]), model); imageSize != "" {
		imageConfig["imageSize"] = imageSize
	}
	if len(imageConfig) == 0 {
		return nil
	}
	return imageConfig
}

func geminiImageResponseModalities(raw any) []string {
	switch value := raw.(type) {
	case string:
		return geminiImageResponseModalitiesList([]any{value})
	case []string:
		items := make([]any, 0, len(value))
		for _, item := range value {
			items = append(items, item)
		}
		return geminiImageResponseModalitiesList(items)
	case []any:
		return geminiImageResponseModalitiesList(value)
	default:
		return nil
	}
}

func geminiImageResponseModalitiesList(raw []any) []string {
	result := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, item := range raw {
		switch strings.ToLower(strings.TrimSpace(getString(item))) {
		case "text":
			if _, ok := seen["TEXT"]; !ok {
				result = append(result, "TEXT")
				seen["TEXT"] = struct{}{}
			}
		case "image":
			if _, ok := seen["IMAGE"]; !ok {
				result = append(result, "IMAGE")
				seen["IMAGE"] = struct{}{}
			}
		}
	}
	return result
}

func geminiImageAspectRatio(value string) string {
	normalized := strings.TrimSpace(value)
	switch normalized {
	case "1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9", "1:4", "4:1", "1:8", "8:1":
		return normalized
	default:
		return ""
	}
}

func geminiImageSize(value string, model string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" || geminiImageModelDisallowsImageSize(model) {
		return ""
	}
	switch normalized {
	case "512":
		if strings.Contains(strings.ToLower(strings.TrimSpace(model)), "3-pro-image") {
			return ""
		}
		return normalized
	case "1K", "2K", "4K":
		return normalized
	default:
		return ""
	}
}

func geminiImageModelDisallowsImageSize(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(normalized, "gemini-2.5-flash-image")
}

// buildGeminiImageGenerationParts 按 Google GenerateContent 格式组合文本提示词和编辑输入图。
func buildGeminiImageGenerationParts(prompt string, images []portllm.ContentPart) []map[string]any {
	parts := []map[string]any{
		{"text": strings.TrimSpace(prompt)},
	}
	for _, image := range images {
		if len(image.Data) == 0 {
			continue
		}
		mimeType := strings.TrimSpace(image.MimeType)
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		parts = append(parts, map[string]any{
			"inline_data": map[string]any{
				"mime_type": mimeType,
				"data":      base64.StdEncoding.EncodeToString(image.Data),
			},
		})
	}
	return parts
}

// parseGeminiImageGenerationOutput 抽取 Gemini inlineData 图片，文本片段只作为 revised prompt。
func parseGeminiImageGenerationOutput(body []byte) (*portllm.GenerateOutput, error) {
	parsed := make(map[string]any)
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	revisedPrompt := extractGeminiText(parsed)
	result := &portllm.GenerateOutput{
		ResponseID:      strings.TrimSpace(getString(parsed["responseId"])),
		Text:            revisedPrompt,
		Usage:           parseGeminiUsage(parsed),
		ToolCalls:       make([]portllm.ToolCall, 0),
		ServerToolCalls: make([]portllm.ToolCall, 0),
		GeneratedImages: extractGeminiGeneratedImages(parsed, revisedPrompt),
		RawJSON:         string(body),
	}
	return result, nil
}

// extractGeminiGeneratedImages 扫描所有候选内容，避免只读取第一个候选导致丢图。
func extractGeminiGeneratedImages(parsed map[string]any, revisedPrompt string) []portllm.GeneratedImage {
	images := make([]portllm.GeneratedImage, 0)
	for _, rawCandidate := range asSlice(parsed["candidates"]) {
		candidate := asMap(rawCandidate)
		content := asMap(candidate["content"])
		for _, rawPart := range asSlice(content["parts"]) {
			part := asMap(rawPart)
			if isGeminiThoughtPart(part) {
				continue
			}
			inlineData := asMap(part["inlineData"])
			if len(inlineData) == 0 {
				// Google 文档的 REST 与 SDK 示例同时出现 camelCase / snake_case，适配器边界统一收敛。
				inlineData = asMap(part["inline_data"])
			}
			if len(inlineData) == 0 {
				continue
			}
			b64 := strings.TrimSpace(getString(inlineData["data"]))
			if b64 == "" {
				continue
			}
			mimeType := strings.TrimSpace(getString(inlineData["mimeType"]))
			if mimeType == "" {
				mimeType = strings.TrimSpace(getString(inlineData["mime_type"]))
			}
			if mimeType == "" {
				mimeType = "image/png"
			}
			images = append(images, portllm.GeneratedImage{
				B64JSON:       b64,
				MIMEType:      mimeType,
				RevisedPrompt: revisedPrompt,
			})
		}
	}
	return images
}

func isGeminiThoughtPart(part map[string]any) bool {
	thought, ok := part["thought"].(bool)
	return ok && thought
}
