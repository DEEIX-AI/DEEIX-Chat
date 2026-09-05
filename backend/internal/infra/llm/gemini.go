package llm

// gemini.go — Google Gemini generateContent / streamGenerateContent 适配器实现。
//
// 协议文档：
//   https://ai.google.dev/api/generate-content#method:-models.generatecontent
//   https://ai.google.dev/api/generate-content#method:-models.streamgeneratecontent
//
// 与 OpenAI/Anthropic 的主要差异：
//   - 模型名嵌入 URL 路径（不在请求体中）
//   - 鉴权：x-goog-api-key 头（非 Authorization/x-api-key）
//   - assistant 角色在 Gemini 中叫 "model"
//   - 图片使用 inlineData.data (base64) + mimeType 格式
//   - system 提示通过顶层 systemInstruction 字段传递
//   - 流式端点为 :streamGenerateContent?alt=sse，每个 SSE data 都是完整的 GenerateContentResponse 片段
//   - token 统计在 usageMetadata（promptTokenCount / candidatesTokenCount / cachedContentTokenCount / thoughtsTokenCount）

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

const (
	// geminiDefaultAPIBase 是 Gemini REST API 的默认 base，用于未配置 BaseURL 的渠道。
	geminiDefaultAPIBase = "https://generativelanguage.googleapis.com"
)

// geminiGenerateContentAdapter 实现 Google Gemini generateContent 协议。
type geminiGenerateContentAdapter struct {
	client *Client
}

func (a *geminiGenerateContentAdapter) Name() string { return portllm.AdapterGoogleGenerateContent }

func (a *geminiGenerateContentAdapter) Generate(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
) (*portllm.GenerateOutput, error) {
	return a.client.generateGemini(ctx, route, input)
}

func (a *geminiGenerateContentAdapter) GenerateStream(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	return a.client.generateGeminiStream(ctx, route, input, onEvent)
}

func (a *geminiGenerateContentAdapter) ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	return a.client.listModelsGemini(ctx, route)
}

// ── URL 构造 ───────────────────────────────────────────────────────────────────

func geminiBaseURL(route portllm.RouteConfig) string {
	base := strings.TrimRight(strings.TrimSpace(route.BaseURL), "/")
	if base == "" {
		return geminiDefaultAPIBase
	}
	return base
}

// buildGeminiGenerateURL 构造非流式端点 URL，模型名嵌入路径。
//
//	{base}/v1beta/models/{model}:generateContent
func buildGeminiGenerateURL(base, model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		m = "gemini-2.0-flash"
	}
	// 支持调用方传入 "models/gemini-xxx" 或直接 "gemini-xxx"
	if !strings.HasPrefix(m, "models/") {
		m = "models/" + m
	}
	return buildGeminiEndpointURL(base, "/"+m+":generateContent")
}

// buildGeminiStreamURL 构造流式端点 URL。
//
//	{base}/v1beta/models/{model}:streamGenerateContent?alt=sse
func buildGeminiStreamURL(base, model string) string {
	return strings.Replace(buildGeminiGenerateURL(base, model), ":generateContent", ":streamGenerateContent?alt=sse", 1)
}

// buildGeminiModelsURL 构造模型列表端点 URL。
func buildGeminiModelsURL(base string) string {
	return buildGeminiEndpointURL(base, "/models")
}

func buildGeminiEndpointURL(baseURL string, endpointPath string) string {
	return buildVersionedEndpointURL(normalizeGeminiEndpointBaseURL(baseURL), "v1beta", endpointPath)
}

// normalizeGeminiEndpointBaseURL 兼容 OpenAI 风格代理常见的 /v1 base，避免拼成 /v1/v1beta。
func normalizeGeminiEndpointBaseURL(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(strings.ToLower(base), "/v1") {
		return strings.TrimRight(base[:len(base)-len("/v1")], "/")
	}
	return base
}

// ── 请求构造 ──────────────────────────────────────────────────────────────────

// buildGeminiRequestBody 构造 GenerateContentRequest。
// system role 消息被提取为顶层 systemInstruction；其余消息映射为 contents。
func buildGeminiRequestBody(input portllm.GenerateInput) (map[string]any, error) {
	messages := normalizeMessages(input.Messages)
	providerTools, toolDefinitions, toolsEnabled, err := toolDeclarationsForInput(input)
	if err != nil {
		return nil, err
	}

	var systemTextParts []string
	contents := make([]map[string]any, 0, len(messages))
	generationConfig := map[string]any{}

	for _, msg := range messages {
		if msg.Role == "system" {
			if text := extractMessageText(msg); text != "" {
				systemTextParts = append(systemTextParts, text)
			}
			continue
		}
		contents = append(contents, map[string]any{
			"role":  toGeminiRole(msg.Role),
			"parts": buildGeminiParts(msg),
		})
	}

	payload := map[string]any{
		"contents": contents,
	}
	applyGeminiRootOptions(payload, input.Options)
	applyGeminiGenerationOptions(generationConfig, input.Options)
	if len(generationConfig) > 0 {
		payload["generationConfig"] = generationConfig
	}
	providerTools = buildGeminiProviderTools(providerTools)
	webSearchTools := []map[string]any{}
	if toolsEnabled && modelParamBool(input.Options, "web_search") {
		webSearchTools = append(webSearchTools, map[string]any{"google_search": map[string]any{}})
	}
	appendToolDeclarations(payload, providerTools, webSearchTools, buildGeminiTools(toolDefinitions))
	applyGeminiToolConfigDefaults(payload, len(providerTools)+len(webSearchTools) > 0)

	if len(systemTextParts) > 0 {
		payload["systemInstruction"] = map[string]any{
			"parts": []map[string]any{
				{"text": strings.Join(systemTextParts, "\n\n")},
			},
		}
	}

	applyProviderOptions(payload, input.Options, geminiProtectedProviderOptionKeys()...)
	return payload, nil
}

func applyGeminiToolConfigDefaults(payload map[string]any, hasServerSideTools bool) {
	if !hasServerSideTools {
		return
	}
	toolConfig := asMap(payload["toolConfig"])
	if len(toolConfig) == 0 {
		toolConfig = map[string]any{}
	}
	if _, ok := toolConfig["includeServerSideToolInvocations"]; !ok {
		toolConfig["includeServerSideToolInvocations"] = true
	}
	payload["toolConfig"] = toolConfig
}

func geminiProtectedProviderOptionKeys() []string {
	return []string{
		"budget_tokens",
		"cached_content",
		"candidateCount",
		"candidate_count",
		"contents",
		"enableEnhancedCivicAnswers",
		"enable_enhanced_civic_answers",
		"frequencyPenalty",
		"frequency_penalty",
		"generationConfig",
		"generation_config",
		"image_config",
		"imageConfig",
		"includeThoughts",
		"include_thoughts",
		"input",
		"instructions",
		"logprobs",
		"max_completion_tokens",
		"max_output_tokens",
		"maxOutputTokens",
		"media_resolution",
		"mediaResolution",
		"messages",
		"model",
		"output_schema",
		"presencePenalty",
		"presence_penalty",
		"prompt",
		"response_format",
		"response_json_schema",
		"response_logprobs",
		"response_mime_type",
		"response_modalities",
		"response_schema",
		"responseJsonSchema",
		"responseLogprobs",
		"responseMimeType",
		"responseModalities",
		"responseSchema",
		"safety_settings",
		"seed",
		"service_tier",
		"speech_config",
		"speechConfig",
		"stop",
		"stopSequences",
		"stream",
		"stream_options",
		"system",
		"systemInstruction",
		"thinking",
		"thinkingBudget",
		"thinkingConfig",
		"thinkingLevel",
		"thinking_budget",
		"thinking_level",
		"tool_config",
		"toolConfig",
		"tools",
		"topK",
		"topP",
		"top_k",
		"top_p",
	}
}

func applyGeminiRootOptions(payload map[string]any, options map[string]any) {
	if len(options) == 0 {
		return
	}
	copyGeminiOption(payload, options, "toolConfig", "toolConfig", "tool_config")
	copyGeminiOption(payload, options, "safetySettings", "safetySettings", "safety_settings")
	copyGeminiOption(payload, options, "cachedContent", "cachedContent", "cached_content")
	copyGeminiOption(payload, options, "serviceTier", "serviceTier", "service_tier")
	copyGeminiOption(payload, options, "store", "store")
}

func applyGeminiGenerationOptions(generationConfig map[string]any, options map[string]any) {
	if len(options) == 0 {
		return
	}
	for _, raw := range []map[string]any{modelParamMap(options, "generationConfig"), modelParamMap(options, "generation_config")} {
		for key, value := range raw {
			if strings.TrimSpace(key) != "" {
				generationConfig[key] = value
			}
		}
	}
	if maxTokens, ok := firstGeminiIntOption(options, "max_output_tokens", "max_completion_tokens", "maxOutputTokens"); ok && maxTokens > 0 {
		generationConfig["maxOutputTokens"] = maxTokens
	}
	if value, ok := modelParamFloat(options, "temperature"); ok {
		generationConfig["temperature"] = value
	}
	if value, ok := firstGeminiFloatOption(options, "top_p", "topP"); ok {
		generationConfig["topP"] = value
	}
	if topK, ok := firstGeminiIntOption(options, "top_k", "topK"); ok && topK > 0 {
		generationConfig["topK"] = topK
	}
	if stops := firstGeminiStringListOption(options, "stop", "stopSequences"); len(stops) > 0 {
		generationConfig["stopSequences"] = stops
	}
	if candidateCount, ok := firstGeminiIntOption(options, "candidate_count", "candidateCount"); ok && candidateCount > 0 {
		generationConfig["candidateCount"] = candidateCount
	}
	if seed, ok := firstGeminiIntOption(options, "seed"); ok {
		generationConfig["seed"] = seed
	}
	if value, ok := firstGeminiFloatOption(options, "presence_penalty", "presencePenalty"); ok {
		generationConfig["presencePenalty"] = value
	}
	if value, ok := firstGeminiFloatOption(options, "frequency_penalty", "frequencyPenalty"); ok {
		generationConfig["frequencyPenalty"] = value
	}
	if value, ok := firstGeminiBoolOption(options, "response_logprobs", "responseLogprobs"); ok {
		generationConfig["responseLogprobs"] = value
	}
	if logprobs, ok := firstGeminiIntOption(options, "logprobs"); ok {
		generationConfig["logprobs"] = logprobs
	}
	if value := firstGeminiStringListOption(options, "response_modalities", "responseModalities"); len(value) > 0 {
		generationConfig["responseModalities"] = value
	}
	if value, ok := firstGeminiBoolOption(options, "enable_enhanced_civic_answers", "enableEnhancedCivicAnswers"); ok {
		generationConfig["enableEnhancedCivicAnswers"] = value
	}
	copyGeminiOption(generationConfig, options, "speechConfig", "speechConfig", "speech_config")
	copyGeminiOption(generationConfig, options, "imageConfig", "imageConfig", "image_config")
	copyGeminiOption(generationConfig, options, "mediaResolution", "mediaResolution", "media_resolution")
	copyGeminiOption(generationConfig, options, "responseMimeType", "responseMimeType", "response_mime_type")
	applyGeminiResponseFormat(generationConfig, options)
	applyGeminiThinkingConfig(generationConfig, options)
}

func applyGeminiResponseFormat(generationConfig map[string]any, options map[string]any) {
	if len(options) == 0 {
		return
	}
	if format := strings.ToLower(modelParamString(options, "response_format")); format != "" {
		if mimeType := geminiResponseMimeType(format); mimeType != "" {
			generationConfig["responseMimeType"] = mimeType
		}
	}
	if format := modelParamMap(options, "response_format"); len(format) > 0 {
		for key, value := range format {
			switch key {
			case "responseMimeType", "responseSchema", "responseJsonSchema":
				generationConfig[key] = value
			case "response_mime_type":
				generationConfig["responseMimeType"] = value
			case "response_schema":
				generationConfig["responseSchema"] = value
			case "response_json_schema":
				generationConfig["responseJsonSchema"] = value
			}
		}
		if mimeType := geminiResponseMimeType(getString(format["type"])); mimeType != "" {
			generationConfig["responseMimeType"] = mimeType
		}
		if schema := geminiSchemaFromResponseFormat(format); len(schema) > 0 {
			generationConfig["responseMimeType"] = "application/json"
			generationConfig["responseSchema"] = schema
		}
	}
	if schema := firstGeminiMapOption(options, "response_schema", "responseSchema", "output_schema"); len(schema) > 0 {
		generationConfig["responseMimeType"] = "application/json"
		generationConfig["responseSchema"] = schema
	}
	if schema := firstGeminiMapOption(options, "response_json_schema", "responseJsonSchema"); len(schema) > 0 {
		generationConfig["responseMimeType"] = "application/json"
		generationConfig["responseJsonSchema"] = schema
	}
}

func geminiResponseMimeType(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json", "json_object", "json_schema", "application/json":
		return "application/json"
	case "text", "text/plain":
		return "text/plain"
	case "enum", "text/x.enum":
		return "text/x.enum"
	default:
		return ""
	}
}

func geminiSchemaFromResponseFormat(format map[string]any) map[string]any {
	if schema := asMap(format["schema"]); len(schema) > 0 {
		return schema
	}
	jsonSchema := asMap(format["json_schema"])
	if schema := asMap(jsonSchema["schema"]); len(schema) > 0 {
		return schema
	}
	if len(jsonSchema) > 0 {
		return jsonSchema
	}
	return nil
}

func applyGeminiThinkingConfig(generationConfig map[string]any, options map[string]any) {
	thinkingConfig := map[string]any{}
	for key, value := range asMap(generationConfig["thinkingConfig"]) {
		if strings.TrimSpace(key) != "" {
			thinkingConfig[key] = value
		}
	}
	mergeGeminiThinkingConfig(thinkingConfig, modelParamMap(options, "thinkingConfig"))
	switch thinking := options["thinking"].(type) {
	case bool:
		thinkingConfig["includeThoughts"] = thinking
	case map[string]any:
		mergeGeminiThinkingConfig(thinkingConfig, thinking)
	}
	if value, ok := firstGeminiBoolOption(options, "include_thoughts", "includeThoughts"); ok {
		thinkingConfig["includeThoughts"] = value
	}
	if budget, ok := firstGeminiIntOption(options, "thinking_budget", "thinkingBudget", "budget_tokens"); ok {
		thinkingConfig["thinkingBudget"] = budget
	}
	if level := firstGeminiStringOption(options, "thinking_level", "thinkingLevel"); level != "" {
		thinkingConfig["thinkingLevel"] = level
	}
	if len(thinkingConfig) > 0 {
		generationConfig["thinkingConfig"] = thinkingConfig
	}
}

func mergeGeminiThinkingConfig(dst map[string]any, raw map[string]any) {
	for key, value := range raw {
		switch key {
		case "includeThoughts", "thinkingBudget", "thinkingLevel":
			dst[key] = value
		case "include_thoughts":
			dst["includeThoughts"] = value
		case "thinking_budget", "budget_tokens":
			dst["thinkingBudget"] = value
		case "thinking_level":
			dst["thinkingLevel"] = value
		case "type":
			switch strings.ToLower(strings.TrimSpace(getString(value))) {
			case "enabled", "auto":
				dst["includeThoughts"] = true
			case "disabled", "none":
				dst["includeThoughts"] = false
			}
		default:
			if strings.TrimSpace(key) != "" {
				dst[key] = value
			}
		}
	}
}

func copyGeminiOption(dst map[string]any, options map[string]any, target string, aliases ...string) {
	for _, key := range aliases {
		if value, ok := options[key]; ok {
			dst[target] = value
			return
		}
	}
}

func firstGeminiMapOption(options map[string]any, aliases ...string) map[string]any {
	for _, key := range aliases {
		if value := modelParamMap(options, key); len(value) > 0 {
			return value
		}
	}
	return nil
}

func firstGeminiIntOption(options map[string]any, aliases ...string) (int, bool) {
	for _, key := range aliases {
		if value, ok := modelParamIntValue(options, key); ok {
			return value, true
		}
	}
	return 0, false
}

func firstGeminiFloatOption(options map[string]any, aliases ...string) (float64, bool) {
	for _, key := range aliases {
		if value, ok := modelParamFloat(options, key); ok {
			return value, true
		}
	}
	return 0, false
}

func firstGeminiBoolOption(options map[string]any, aliases ...string) (bool, bool) {
	for _, key := range aliases {
		value, ok := options[key]
		if !ok {
			continue
		}
		typed, ok := value.(bool)
		if ok {
			return typed, true
		}
	}
	return false, false
}

func firstGeminiStringOption(options map[string]any, aliases ...string) string {
	for _, key := range aliases {
		if value := modelParamString(options, key); value != "" {
			return value
		}
	}
	return ""
}

func firstGeminiStringListOption(options map[string]any, aliases ...string) []string {
	for _, key := range aliases {
		if value := modelParamStringList(options, key); len(value) > 0 {
			return value
		}
	}
	return nil
}

func buildGeminiTools(tools []portllm.ToolDefinition) []map[string]any {
	if len(tools) == 0 {
		return nil
	}
	declarations := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		declarations = append(declarations, map[string]any{
			"name":                 name,
			"description":          strings.TrimSpace(tool.Description),
			"parametersJsonSchema": decodeToolSchema(tool.InputSchema),
		})
	}
	if len(declarations) == 0 {
		return nil
	}
	return []map[string]any{{"functionDeclarations": declarations}}
}

func buildGeminiProviderTools(tools []map[string]any) []map[string]any {
	if len(tools) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		if googleSearch, ok := geminiGoogleSearchToolPayload(tool); ok {
			result = append(result, map[string]any{"google_search": googleSearch})
			continue
		}
		result = append(result, tool)
	}
	return result
}

func geminiGoogleSearchToolPayload(tool map[string]any) (map[string]any, bool) {
	if strings.TrimSpace(getString(tool["type"])) == "google_search" {
		return geminiToolParameterMap(tool["google_search"], tool["googleSearch"]), true
	}
	if _, ok := tool["google_search"]; ok {
		return geminiToolParameterMap(tool["google_search"], tool["googleSearch"]), true
	}
	if _, ok := tool["googleSearch"]; ok {
		return geminiToolParameterMap(tool["googleSearch"]), true
	}
	return nil, false
}

func geminiToolParameterMap(values ...any) map[string]any {
	for _, value := range values {
		if typed, ok := value.(map[string]any); ok {
			return typed
		}
	}
	return map[string]any{}
}

// toGeminiRole 将内部 role 转换为 Gemini role（user / model）。
func toGeminiRole(role string) string {
	switch role {
	case "assistant":
		return "model"
	default:
		return "user"
	}
}

// buildGeminiParts 将 Message 转换为 Gemini parts 数组。
func buildGeminiParts(msg portllm.Message) []map[string]any {
	if len(msg.Parts) == 0 && len(msg.ToolCalls) == 0 && len(msg.ToolResults) == 0 {
		return []map[string]any{{"text": msg.Content}}
	}

	parts := make([]map[string]any, 0, len(msg.Parts)+len(msg.ToolCalls)+len(msg.ToolResults)+1)
	if text := strings.TrimSpace(msg.Content); text != "" {
		parts = append(parts, map[string]any{"text": text})
	}
	for _, part := range msg.Parts {
		switch part.Kind {
		case portllm.ContentPartImage:
			if len(part.Data) == 0 {
				continue
			}
			mime := strings.TrimSpace(part.MimeType)
			if mime == "" {
				mime = "image/jpeg"
			}
			parts = append(parts, map[string]any{
				"inlineData": map[string]any{
					"mimeType": mime,
					"data":     base64.StdEncoding.EncodeToString(part.Data),
				},
			})
		default: // text, file
			text := part.Text
			if strings.TrimSpace(text) == "" {
				continue
			}
			parts = append(parts, map[string]any{"text": text})
		}
	}
	for _, item := range msg.ToolCalls {
		args := strings.TrimSpace(item.ArgumentsJSON)
		if args == "" {
			args = "{}"
		}
		arguments := make(map[string]any)
		if err := json.Unmarshal([]byte(args), &arguments); err != nil {
			arguments = map[string]any{"arguments": args}
		}
		parts = append(parts, map[string]any{
			"functionCall": map[string]any{
				"name": strings.TrimSpace(item.ToolName),
				"args": arguments,
			},
		})
		if signature := strings.TrimSpace(item.ThoughtSignature); signature != "" {
			parts[len(parts)-1]["thoughtSignature"] = signature
		}
	}
	for _, item := range msg.ToolResults {
		response := map[string]any{
			"name": strings.TrimSpace(item.ToolName),
			"response": map[string]any{
				"content": buildToolResultContent(item),
			},
		}
		if strings.TrimSpace(item.Error) != "" {
			response["response"].(map[string]any)["error"] = strings.TrimSpace(item.Error)
		}
		parts = append(parts, map[string]any{"functionResponse": response})
	}

	if len(parts) == 0 {
		return []map[string]any{{"text": msg.Content}}
	}
	return parts
}

// ── 公共 HTTP 辅助 ─────────────────────────────────────────────────────────────

func (c *Client) newGeminiRequest(
	ctx context.Context,
	method, requestURL string,
	body io.Reader,
	route portllm.RouteConfig,
	input *portllm.GenerateInput,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// 优先用头部传 key（避免 key 出现在访问日志 URL 中）
	if apiKey := strings.TrimSpace(route.APIKey); apiKey != "" {
		req.Header.Set("x-goog-api-key", apiKey)
	}
	setAdditionalHeadersForInput(req, route.HeadersJSON, input)
	return req, nil
}

// parseGeminiError 从 Gemini 错误响应中提取 error.message。
func parseGeminiError(statusCode int, body []byte, debug *portllm.UpstreamDebugSnapshot) error {
	parsed := make(map[string]any)
	if err := json.Unmarshal(body, &parsed); err == nil {
		if msg := getStringFromPath(parsed, "error", "message"); msg != "" {
			return &portllm.UpstreamError{StatusCode: statusCode, Message: msg, Body: string(body), Debug: debug}
		}
	}
	if statusCode == http.StatusUnauthorized {
		return &portllm.UpstreamError{
			StatusCode: statusCode,
			Message:    "google authentication failed; check API key, upstream base URL, and custom auth headers",
			Body:       string(body),
			Debug:      debug,
		}
	}
	return &portllm.UpstreamError{
		StatusCode: statusCode,
		Message:    fmt.Sprintf("upstream_status_%d", statusCode),
		Body:       string(body),
		Debug:      debug,
	}
}

// ── 非流式调用 ────────────────────────────────────────────────────────────────

func (c *Client) generateGemini(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
) (*portllm.GenerateOutput, error) {
	base := geminiBaseURL(route)
	requestURL := buildGeminiGenerateURL(base, route.UpstreamModel)

	requestBody, err := buildGeminiRequestBody(input)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	req, err := c.newGeminiRequest(requestCtx, http.MethodPost, requestURL, bytes.NewReader(payload), route, &input)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRouteGenerationRequest(route, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := readUpstreamBody(resp.Body)
	if err != nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil, portllm.MarkRequestAccepted(err)
		}
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseGeminiError(resp.StatusCode, body, upstreamDebugSnapshot(req, payload, resp, body))
	}

	debug := upstreamDebugSnapshot(req, payload, resp, body)
	output, err := parseGeminiResponse(body)
	if err != nil {
		return nil, portllm.MarkRequestAccepted(attachUpstreamDebug(err, debug))
	}
	output.Debug = debug
	return output, nil
}

// parseGeminiResponse 解析 GenerateContentResponse（非流式）。
func parseGeminiResponse(body []byte) (*portllm.GenerateOutput, error) {
	parsed := make(map[string]any)
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	text := extractGeminiText(parsed)
	result := &portllm.GenerateOutput{
		ResponseID:          strings.TrimSpace(getString(parsed["responseId"])),
		Text:                text,
		Reasoning:           extractGeminiReasoning(parsed),
		Usage:               parseGeminiUsage(parsed),
		ToolCalls:           parseGeminiFunctionCalls(parsed),
		ServerToolCalls:     parseGeminiServerToolCalls(parsed),
		ServerSideToolUsage: parseGeminiServerSideToolUsage(parsed),
		Citations:           parseGeminiCitations(parsed),
		GeneratedImages:     extractGeminiGeneratedImages(parsed, text),
		RawJSON:             string(body),
	}
	return result, nil
}

// extractGeminiText 从 candidates[0].content.parts 中拼接所有 text。
func extractGeminiText(parsed map[string]any) string {
	candidate := firstMapItem(asSlice(parsed["candidates"]))
	content := asMap(candidate["content"])
	chunks := make([]string, 0)
	for _, raw := range asSlice(content["parts"]) {
		part := asMap(raw)
		if thought, ok := part["thought"].(bool); ok && thought {
			continue
		}
		if text := strings.TrimSpace(getString(part["text"])); text != "" {
			chunks = append(chunks, text)
		}
	}
	return strings.Join(chunks, "")
}

func extractGeminiReasoning(parsed map[string]any) *portllm.ReasoningOutput {
	candidate := firstMapItem(asSlice(parsed["candidates"]))
	content := asMap(candidate["content"])
	result := &portllm.ReasoningOutput{}
	for _, raw := range asSlice(content["parts"]) {
		part := asMap(raw)
		thought, ok := part["thought"].(bool)
		if !ok || !thought {
			continue
		}
		result.Text += extractReasoningDeltaText(part)
		if signature := strings.TrimSpace(getString(part["thoughtSignature"])); signature != "" {
			result.Signature = signature
		}
	}
	if result.Text == "" && result.Signature == "" {
		return nil
	}
	return result
}

// parseGeminiUsage 解析 usageMetadata。
func parseGeminiUsage(parsed map[string]any) portllm.Usage {
	totalInputTokens := getInt64FromPath(parsed, "usageMetadata", "promptTokenCount")
	cacheReadTokens := getInt64FromPath(parsed, "usageMetadata", "cachedContentTokenCount")
	return portllm.Usage{
		InputTokens:     nonCachedInputTokens(totalInputTokens, cacheReadTokens),
		OutputTokens:    getInt64FromPath(parsed, "usageMetadata", "candidatesTokenCount"),
		CacheReadTokens: cacheReadTokens,
		ReasoningTokens: getInt64FromPath(parsed, "usageMetadata", "thoughtsTokenCount"),
		RawUsageJSON:    rawUsageJSONFromPath(parsed, "usageMetadata"),
	}
}

// parseGeminiFunctionCalls 解析 candidates[0].content.parts 中的 functionCall。
func parseGeminiFunctionCalls(parsed map[string]any) []portllm.ToolCall {
	candidate := firstMapItem(asSlice(parsed["candidates"]))
	content := asMap(candidate["content"])

	var result []portllm.ToolCall
	for _, raw := range asSlice(content["parts"]) {
		part := asMap(raw)
		fc := asMap(part["functionCall"])
		if len(fc) == 0 {
			continue
		}
		arguments := normalizeJSONString(fc["args"])
		if arguments == "" {
			arguments = "{}"
		}
		result = append(result, portllm.ToolCall{
			ToolType:         "function",
			ToolName:         strings.TrimSpace(getString(fc["name"])),
			ArgumentsJSON:    arguments,
			ThoughtSignature: strings.TrimSpace(getString(part["thoughtSignature"])),
			Status:           "requested",
		})
	}
	if result == nil {
		return make([]portllm.ToolCall, 0)
	}
	return result
}

func parseGeminiServerToolCalls(parsed map[string]any) []portllm.ToolCall {
	if len(parsed) == 0 {
		return make([]portllm.ToolCall, 0)
	}
	result := make([]portllm.ToolCall, 0)
	for _, call := range parseGeminiExplicitServerToolCalls(parsed, -1) {
		appendUniqueToolCall(&result, call)
	}
	for candidateIndex, rawCandidate := range asSlice(parsed["candidates"]) {
		candidate := asMap(rawCandidate)
		for _, call := range parseGeminiExplicitServerToolCalls(candidate, candidateIndex) {
			appendUniqueToolCall(&result, call)
		}
		if call, ok := parseGeminiGoogleSearchServerToolCall(candidate, candidateIndex); ok {
			appendUniqueToolCall(&result, call)
		}
		if call, ok := parseGeminiURLContextServerToolCall(candidate, candidateIndex); ok {
			appendUniqueToolCall(&result, call)
		}
		for _, call := range parseGeminiCodeExecutionServerToolCalls(candidate, candidateIndex) {
			appendUniqueToolCall(&result, call)
		}
	}
	return result
}

func parseGeminiExplicitServerToolCalls(payload map[string]any, candidateIndex int) []portllm.ToolCall {
	if len(payload) == 0 {
		return nil
	}
	result := make([]portllm.ToolCall, 0)
	keys := []string{
		"serverSideToolInvocations",
		"server_side_tool_invocations",
		"toolInvocations",
		"tool_invocations",
	}
	for _, key := range keys {
		for itemIndex, raw := range asSlice(payload[key]) {
			if call, ok := parseGeminiExplicitServerToolCall(asMap(raw), candidateIndex, itemIndex); ok {
				result = append(result, call)
			}
		}
	}
	content := asMap(payload["content"])
	for partIndex, raw := range asSlice(content["parts"]) {
		part := asMap(raw)
		for _, key := range []string{"serverSideToolInvocation", "server_side_tool_invocation", "toolInvocation", "tool_invocation"} {
			if call, ok := parseGeminiExplicitServerToolCall(asMap(part[key]), candidateIndex, partIndex); ok {
				result = append(result, call)
			}
		}
	}
	return result
}

func parseGeminiExplicitServerToolCall(item map[string]any, candidateIndex int, itemIndex int) (portllm.ToolCall, bool) {
	if len(item) == 0 {
		return portllm.ToolCall{}, false
	}
	name := textutil.FirstNonEmpty(
		getString(item["name"]),
		getString(item["toolName"]),
		getString(item["tool_name"]),
		getString(item["type"]),
	)
	name = normalizeGeminiServerToolName(name)
	if name == "" {
		return portllm.ToolCall{}, false
	}
	status := textutil.FirstNonEmpty(getString(item["status"]), "in_progress")
	return portllm.ToolCall{
		ToolCallID: textutil.FirstNonEmpty(
			getString(item["id"]),
			getString(item["callId"]),
			getString(item["call_id"]),
			geminiServerToolCallID(name, candidateIndex, itemIndex),
		),
		ToolType:      name,
		ToolName:      name,
		ArgumentsJSON: textutil.FirstNonEmpty(normalizeJSONString(item["input"]), normalizeJSONString(item["args"]), normalizeJSONString(item["arguments"])),
		Status:        normalizeGeminiExplicitServerToolStatus(status),
		OutputJSON:    textutil.FirstNonEmpty(normalizeJSONString(item["output"]), normalizeJSONString(item["result"]), normalizeJSONString(item["response"])),
		ErrorJSON:     normalizeJSONString(item["error"]),
	}, true
}

func normalizeGeminiServerToolName(name string) string {
	value := strings.ToLower(strings.TrimSpace(name))
	value = strings.TrimSuffix(value, "_call")
	value = strings.TrimSuffix(value, "_tool")
	switch value {
	case "google_search", "search", "web_search":
		return "google_search"
	case "url_context", "urlcontext":
		return "url_context"
	case "code_execution", "codeexecution":
		return "code_execution"
	default:
		return value
	}
}

func normalizeGeminiExplicitServerToolStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "pending", "running", "in_progress", "searching", "requested":
		return "in_progress"
	case "success", "succeeded", "completed", "done":
		return "completed"
	case "failed", "error":
		return "error"
	default:
		return strings.TrimSpace(status)
	}
}

func parseGeminiGoogleSearchServerToolCall(candidate map[string]any, candidateIndex int) (portllm.ToolCall, bool) {
	grounding := asMap(candidate["groundingMetadata"])
	if !hasGeminiSearchGroundingMetadata(grounding) {
		return portllm.ToolCall{}, false
	}
	input := map[string]any{}
	if queries := asSlice(grounding["webSearchQueries"]); len(queries) > 0 {
		input["queries"] = queries
	}
	output := compactGeminiServerToolPayload(map[string]any{
		"queries":            grounding["webSearchQueries"],
		"sources":            geminiGroundingSources(grounding),
		"support_count":      len(asSlice(grounding["groundingSupports"])),
		"search_entry_point": grounding["searchEntryPoint"],
		"retrieval_metadata": grounding["retrievalMetadata"],
	})
	return portllm.ToolCall{
		ToolCallID:    geminiServerToolCallID("google_search", candidateIndex, 0),
		ToolType:      "google_search",
		ToolName:      "google_search",
		ArgumentsJSON: normalizeJSONString(input),
		Status:        "completed",
		OutputJSON:    normalizeJSONString(output),
	}, true
}

func parseGeminiURLContextServerToolCall(candidate map[string]any, candidateIndex int) (portllm.ToolCall, bool) {
	metadata := asMap(candidate["urlContextMetadata"])
	urlMetadata := asSlice(metadata["urlMetadata"])
	if len(urlMetadata) == 0 {
		return portllm.ToolCall{}, false
	}
	urls := make([]string, 0, len(urlMetadata))
	for _, raw := range urlMetadata {
		item := asMap(raw)
		if uri := textutil.FirstNonEmpty(getString(item["retrievedUrl"]), getString(item["url"]), getString(item["uri"])); uri != "" {
			urls = append(urls, uri)
		}
	}
	input := map[string]any{}
	if len(urls) > 0 {
		input["urls"] = urls
	}
	output := compactGeminiServerToolPayload(map[string]any{
		"urls": geminiURLContextItems(urlMetadata),
	})
	return portllm.ToolCall{
		ToolCallID:    geminiServerToolCallID("url_context", candidateIndex, 0),
		ToolType:      "url_context",
		ToolName:      "url_context",
		ArgumentsJSON: normalizeJSONString(input),
		Status:        "completed",
		OutputJSON:    normalizeJSONString(output),
	}, true
}

func parseGeminiCodeExecutionServerToolCalls(candidate map[string]any, candidateIndex int) []portllm.ToolCall {
	content := asMap(candidate["content"])
	parts := asSlice(content["parts"])
	result := make([]portllm.ToolCall, 0)
	currentIndex := -1
	for _, raw := range parts {
		part := asMap(raw)
		executableCode := asMap(part["executableCode"])
		if len(executableCode) > 0 {
			currentIndex++
			result = append(result, portllm.ToolCall{
				ToolCallID:    geminiServerToolCallID("code_execution", candidateIndex, currentIndex),
				ToolType:      "code_execution",
				ToolName:      "code_execution",
				ArgumentsJSON: normalizeJSONString(executableCode),
				Status:        "requested",
			})
			continue
		}
		executionResult := asMap(part["codeExecutionResult"])
		if len(executionResult) == 0 {
			continue
		}
		if currentIndex < 0 {
			currentIndex = 0
			result = append(result, portllm.ToolCall{
				ToolCallID: geminiServerToolCallID("code_execution", candidateIndex, currentIndex),
				ToolType:   "code_execution",
				ToolName:   "code_execution",
			})
		}
		idx := len(result) - 1
		result[idx].OutputJSON = normalizeJSONString(executionResult)
		result[idx].Status = geminiCodeExecutionStatus(executionResult)
		if result[idx].Status == "error" {
			result[idx].ErrorJSON = normalizeJSONString(executionResult)
		}
	}
	return result
}

func geminiGroundingSources(grounding map[string]any) []map[string]any {
	chunks := asSlice(grounding["groundingChunks"])
	if len(chunks) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(chunks))
	for _, raw := range chunks {
		chunk := asMap(raw)
		for _, key := range []string{"web", "retrievedContext"} {
			source := asMap(chunk[key])
			uri := textutil.FirstNonEmpty(getString(source["uri"]), getString(source["url"]))
			if uri == "" {
				continue
			}
			item := map[string]any{"url": uri}
			if title := strings.TrimSpace(getString(source["title"])); title != "" {
				item["title"] = title
			}
			result = append(result, item)
		}
	}
	return result
}

func geminiURLContextItems(items []any) []map[string]any {
	if len(items) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		item := asMap(raw)
		url := textutil.FirstNonEmpty(getString(item["retrievedUrl"]), getString(item["url"]), getString(item["uri"]))
		if url == "" {
			continue
		}
		next := map[string]any{"url": url}
		if status := strings.TrimSpace(getString(item["urlRetrievalStatus"])); status != "" {
			next["status"] = status
		}
		result = append(result, next)
	}
	return result
}

func compactGeminiServerToolPayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	result := make(map[string]any, len(payload))
	for key, value := range payload {
		if isEmptyGeminiPayloadValue(value) {
			continue
		}
		result[key] = value
	}
	return result
}

func isEmptyGeminiPayloadValue(value any) bool {
	if value == nil {
		return true
	}
	if len(asSlice(value)) > 0 || len(asMap(value)) > 0 {
		return false
	}
	switch typed := value.(type) {
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	case string:
		return strings.TrimSpace(typed) == ""
	default:
		return false
	}
}

func geminiCodeExecutionStatus(result map[string]any) string {
	outcome := strings.ToUpper(strings.TrimSpace(getString(result["outcome"])))
	switch outcome {
	case "", "OUTCOME_OK", "OK":
		return "completed"
	default:
		return "error"
	}
}

func geminiServerToolCallID(tool string, candidateIndex int, itemIndex int) string {
	return fmt.Sprintf("gemini_%s_%d_%d", strings.TrimSpace(tool), candidateIndex, itemIndex)
}

func parseGeminiCitations(parsed map[string]any) []string {
	if len(parsed) == 0 {
		return nil
	}
	citations := make([]string, 0)
	for _, rawCandidate := range asSlice(parsed["candidates"]) {
		candidate := asMap(rawCandidate)
		grounding := asMap(candidate["groundingMetadata"])
		for _, raw := range asSlice(grounding["groundingChunks"]) {
			chunk := asMap(raw)
			for _, key := range []string{"web", "retrievedContext"} {
				source := asMap(chunk[key])
				if uri := textutil.FirstNonEmpty(getString(source["uri"]), getString(source["url"])); uri != "" {
					citations = append(citations, uri)
				}
			}
		}
		urlContext := asMap(candidate["urlContextMetadata"])
		for _, raw := range asSlice(urlContext["urlMetadata"]) {
			item := asMap(raw)
			if uri := textutil.FirstNonEmpty(getString(item["retrievedUrl"]), getString(item["url"]), getString(item["uri"])); uri != "" {
				citations = append(citations, uri)
			}
		}
	}
	return appendUniqueStrings(nil, citations...)
}

func parseGeminiServerSideToolUsage(parsed map[string]any) map[string]int64 {
	if len(parsed) == 0 {
		return nil
	}
	result := make(map[string]int64)
	for _, rawCandidate := range asSlice(parsed["candidates"]) {
		candidate := asMap(rawCandidate)
		if hasGeminiSearchGroundingMetadata(asMap(candidate["groundingMetadata"])) {
			result["google_search"] = 1
		}
		if len(asSlice(asMap(candidate["urlContextMetadata"])["urlMetadata"])) > 0 {
			result["url_context"] = 1
		}
		codeExecutionCount := int64(len(parseGeminiCodeExecutionServerToolCalls(candidate, 0)))
		if codeExecutionCount > 0 {
			result["code_execution"] += codeExecutionCount
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func hasGeminiSearchGroundingMetadata(grounding map[string]any) bool {
	if len(grounding) == 0 {
		return false
	}
	if len(asSlice(grounding["groundingChunks"])) > 0 || len(asSlice(grounding["groundingSupports"])) > 0 {
		return true
	}
	if len(asSlice(grounding["webSearchQueries"])) > 0 {
		return true
	}
	if len(asMap(grounding["searchEntryPoint"])) > 0 || len(asMap(grounding["retrievalMetadata"])) > 0 {
		return true
	}
	return false
}

// ── 流式调用 ──────────────────────────────────────────────────────────────────

func (c *Client) generateGeminiStream(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	base := geminiBaseURL(route)
	requestURL := buildGeminiStreamURL(base, route.UpstreamModel)

	requestBody, err := buildGeminiRequestBody(input)
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

	req, err := c.newGeminiRequest(firstByteCtx, http.MethodPost, requestURL, bytes.NewReader(payload), route, &input)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.doRouteGenerationRequest(route, req)
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
		return nil, portllm.MarkRequestAccepted(attachUpstreamDebug(err, upstreamDebugSnapshot(req, payload, resp, streamErrorBody(streamBody, err))))
	}
	return result, nil
}

// consumeGeminiStream 解析 Gemini SSE 流。
//
// Gemini 流式格式：每个 SSE data 都是一个完整的 GenerateContentResponse 片段。
// 最后一个事件通常携带完整的 usageMetadata。
//
//	data: {"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"}}]}
//	data: {"candidates":[{"content":{"parts":[{"text":" world"}],"role":"model"}}],"usageMetadata":{...}}
func consumeGeminiStream(
	reader io.Reader,
	result *portllm.GenerateOutput,
	onEvent func(portllm.GenerateStreamEvent) error,
) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxUpstreamBodyBytes)

	var dataLines []string

	flush := func() error {
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		data = strings.TrimSpace(data)
		if data == "" || data == "[DONE]" {
			return nil
		}
		parsed := make(map[string]any)
		if err := json.Unmarshal([]byte(data), &parsed); err != nil {
			return nil // 单个异常事件不应中断后续流式输出。
		}
		if err := parseStreamUpstreamError(parsed, data); err != nil {
			return err
		}
		return applyGeminiStreamChunk(parsed, result, onEvent)
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line[len("data:"):], " ")
			dataLines = append(dataLines, data)
		}
		// Gemini 仅使用 data 行，其他 SSE 前缀不参与内容解析。
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

// applyGeminiStreamChunk 将单个 GenerateContentResponse 片段合并到 result。
func applyGeminiStreamChunk(
	parsed map[string]any,
	result *portllm.GenerateOutput,
	onEvent func(portllm.GenerateStreamEvent) error,
) error {
	if responseID := strings.TrimSpace(getString(parsed["responseId"])); responseID != "" {
		result.ResponseID = responseID
	}
	result.Citations = appendUniqueStrings(result.Citations, parseGeminiCitations(parsed)...)
	result.ServerSideToolUsage = mergeGeminiServerSideToolUsage(result.ServerSideToolUsage, parseGeminiServerSideToolUsage(parsed))
	for _, call := range parseGeminiServerToolCalls(parsed) {
		merged := appendGeminiServerToolCall(result, call)
		if onEvent != nil {
			if err := onEvent(portllm.GenerateStreamEvent{
				ServerToolCall: &merged,
				ResponseID:     result.ResponseID,
			}); err != nil {
				return err
			}
		}
	}
	applyGeminiStreamServerToolUsage(result)

	// 提取文本增量
	candidate := firstMapItem(asSlice(parsed["candidates"]))
	content := asMap(candidate["content"])
	for _, raw := range asSlice(content["parts"]) {
		part := asMap(raw)
		if thought, ok := part["thought"].(bool); ok && thought {
			think := extractReasoningDeltaText(part)
			if think == "" {
				continue
			}
			reasoning := &portllm.ReasoningDelta{
				EventType: "google.generate_content",
				Kind:      "content_text",
				Text:      think,
				Signature: strings.TrimSpace(getString(part["thoughtSignature"])),
			}
			mergeReasoningDeltaOutput(&result.Reasoning, reasoning)
			if onEvent != nil {
				if err := onEvent(portllm.GenerateStreamEvent{
					Reasoning:  reasoning,
					ResponseID: result.ResponseID,
				}); err != nil {
					return err
				}
			}
			continue
		}
		text := getString(part["text"])
		if text == "" {
			continue
		}
		result.Text += text
		if onEvent != nil {
			if err := onEvent(portllm.GenerateStreamEvent{
				Delta:      text,
				ResponseID: result.ResponseID,
			}); err != nil {
				return err
			}
		}
	}
	result.GeneratedImages = append(result.GeneratedImages, extractGeminiGeneratedImages(parsed, result.Text)...)

	// 函数调用（流式最后一帧可能包含）
	for _, raw := range asSlice(content["parts"]) {
		part := asMap(raw)
		fc := asMap(part["functionCall"])
		if len(fc) == 0 {
			continue
		}
		arguments := normalizeJSONString(fc["args"])
		if arguments == "" {
			arguments = "{}"
		}
		result.ToolCalls = append(result.ToolCalls, portllm.ToolCall{
			ToolType:         "function",
			ToolName:         strings.TrimSpace(getString(fc["name"])),
			ArgumentsJSON:    arguments,
			ThoughtSignature: strings.TrimSpace(getString(part["thoughtSignature"])),
			Status:           "requested",
		})
	}

	// usageMetadata（最后一帧携带完整统计）
	if usage := parseGeminiUsage(parsed); usage != (portllm.Usage{}) {
		result.Usage = usage
		if onEvent != nil {
			return onEvent(portllm.GenerateStreamEvent{
				Usage:      usage,
				ResponseID: result.ResponseID,
			})
		}
	}

	return nil
}

func appendGeminiServerToolCall(result *portllm.GenerateOutput, call portllm.ToolCall) portllm.ToolCall {
	if result == nil {
		return call
	}
	call = normalizeGeminiStreamServerToolCallID(result.ServerToolCalls, call)
	appendUniqueToolCall(&result.ServerToolCalls, call)
	for _, item := range result.ServerToolCalls {
		if shouldMergeToolCall(item, call) {
			return item
		}
	}
	return call
}

func normalizeGeminiStreamServerToolCallID(existing []portllm.ToolCall, call portllm.ToolCall) portllm.ToolCall {
	if strings.TrimSpace(call.ToolName) != "code_execution" && strings.TrimSpace(call.ToolType) != "code_execution" {
		return call
	}
	hasInput := strings.TrimSpace(call.ArgumentsJSON) != ""
	hasOutput := strings.TrimSpace(call.OutputJSON) != "" || strings.TrimSpace(call.ErrorJSON) != ""
	if hasInput && !hasOutput {
		if item, ok := latestGeminiPendingCodeExecution(existing); ok && strings.TrimSpace(item.OutputJSON) == "" && strings.TrimSpace(item.ErrorJSON) == "" {
			call.ToolCallID = item.ToolCallID
			return call
		}
		call.ToolCallID = geminiServerToolCallID("code_execution", 0, countGeminiCodeExecutionCalls(existing))
		return call
	}
	if !hasInput && hasOutput {
		if item, ok := latestGeminiPendingCodeExecution(existing); ok {
			call.ToolCallID = item.ToolCallID
			return call
		}
		call.ToolCallID = geminiServerToolCallID("code_execution", 0, countGeminiCodeExecutionCalls(existing))
	}
	return call
}

func latestGeminiPendingCodeExecution(items []portllm.ToolCall) (portllm.ToolCall, bool) {
	for idx := len(items) - 1; idx >= 0; idx-- {
		item := items[idx]
		if strings.TrimSpace(item.ToolName) != "code_execution" && strings.TrimSpace(item.ToolType) != "code_execution" {
			continue
		}
		if strings.TrimSpace(item.OutputJSON) == "" && strings.TrimSpace(item.ErrorJSON) == "" {
			return item, true
		}
	}
	return portllm.ToolCall{}, false
}

func countGeminiCodeExecutionCalls(items []portllm.ToolCall) int {
	count := 0
	for _, item := range items {
		if strings.TrimSpace(item.ToolName) == "code_execution" || strings.TrimSpace(item.ToolType) == "code_execution" {
			count++
		}
	}
	return count
}

func applyGeminiStreamServerToolUsage(result *portllm.GenerateOutput) {
	if result == nil {
		return
	}
	codeExecutionCount := int64(countGeminiCodeExecutionCalls(result.ServerToolCalls))
	if codeExecutionCount <= 0 {
		return
	}
	if result.ServerSideToolUsage == nil {
		result.ServerSideToolUsage = make(map[string]int64)
	}
	if codeExecutionCount > result.ServerSideToolUsage["code_execution"] {
		result.ServerSideToolUsage["code_execution"] = codeExecutionCount
	}
}

func mergeGeminiServerSideToolUsage(current map[string]int64, next map[string]int64) map[string]int64 {
	if len(next) == 0 {
		return current
	}
	if current == nil {
		current = make(map[string]int64, len(next))
	}
	for key, count := range next {
		if count > current[key] {
			current[key] = count
		}
	}
	return current
}

// ── Models 目录 ───────────────────────────────────────────────────────────────

// listModelsGemini 调用 GET /v1beta/models。
//
// 响应：{"models":[{"name":"models/gemini-2.0-flash","displayName":"..."},...]}
func (c *Client) listModelsGemini(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	base := geminiBaseURL(route)
	baseRequestURL := buildGeminiModelsURL(base)

	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	results := make([]portllm.ModelItem, 0)
	pageToken := ""
	seenPageTokens := make(map[string]struct{})
	for {
		pageURL, err := url.Parse(baseRequestURL)
		if err != nil {
			return nil, err
		}
		query := pageURL.Query()
		query.Set("pageSize", "1000")
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		pageURL.RawQuery = query.Encode()

		req, err := c.newGeminiRequest(requestCtx, http.MethodGet, pageURL.String(), nil, route, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.doRouteRequest(route, req)
		if err != nil {
			return nil, err
		}
		body, readErr := readUpstreamBody(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, parseGeminiError(resp.StatusCode, body, upstreamDebugSnapshot(req, nil, resp, body))
		}

		parsed := struct {
			Models []struct {
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}{}
		if err = json.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}
		for _, item := range parsed.Models {
			id := strings.TrimPrefix(strings.TrimSpace(item.Name), "models/")
			if id == "" {
				continue
			}
			results = append(results, portllm.ModelItem{ID: id, OwnedBy: "google"})
		}
		nextPageToken := strings.TrimSpace(parsed.NextPageToken)
		if nextPageToken == "" {
			return results, nil
		}
		if _, exists := seenPageTokens[nextPageToken]; exists {
			return nil, fmt.Errorf("repeated gemini models pagination token")
		}
		seenPageTokens[nextPageToken] = struct{}{}
		pageToken = nextPageToken
	}
}
