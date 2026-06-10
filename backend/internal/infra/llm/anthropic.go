package llm

// anthropic.go — Anthropic Messages API 适配器实现。
//
// 协议文档：https://docs.anthropic.com/en/api/messages
//
// 与 OpenAI 格式的主要差异：
//   - 鉴权头：x-api-key（非 Authorization: Bearer）
//   - 必须携带 anthropic-version 头
//   - 端点：POST /v1/messages
//   - 系统提示通过顶层 system 字段传递（不放在 messages 数组中）
//   - max_tokens 为必填字段，默认使用较高上限，模型配置可覆盖
//   - 图片 source 格式不同于 OpenAI（base64 + media_type）
//   - 流式事件类型从 JSON payload 的 type 字段读取，SSE event 行仅作为补充信息

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// anthropicVersion 固定版本头，Anthropic 要求必须携带。
	anthropicVersion = "2023-06-01"
	// anthropicDefaultMaxTokens 是 Anthropic Messages API 必填 max_tokens 的默认值。
	anthropicDefaultMaxTokens = 64000
)

// anthropicMessagesAdapter 实现 Anthropic Messages API（POST /v1/messages）。
type anthropicMessagesAdapter struct {
	client *Client
}

func (a *anthropicMessagesAdapter) Name() string { return AdapterAnthropicMessages }

func (a *anthropicMessagesAdapter) Generate(
	ctx context.Context,
	route RouteConfig,
	input GenerateInput,
) (*GenerateOutput, error) {
	return a.client.generateAnthropic(ctx, route, input, false)
}

func (a *anthropicMessagesAdapter) GenerateStream(
	ctx context.Context,
	route RouteConfig,
	input GenerateInput,
	onEvent func(GenerateStreamEvent) error,
) (*GenerateOutput, error) {
	return a.client.generateAnthropicStream(ctx, route, input, onEvent)
}

func (a *anthropicMessagesAdapter) ListModels(ctx context.Context, route RouteConfig) ([]ModelItem, error) {
	return a.client.listModelsAnthropic(ctx, route)
}

// ── URL 构造 ───────────────────────────────────────────────────────────────────

func buildAnthropicMessagesURL(baseURL string) string {
	return buildVersionedEndpointURL(baseURL, "v1", "/messages")
}

func buildAnthropicModelsURL(baseURL string) string {
	return buildVersionedEndpointURL(baseURL, "v1", "/models")
}

// ── 请求构造 ──────────────────────────────────────────────────────────────────

// buildAnthropicRequestBody 构造 Anthropic Messages API 请求体。
// system role 消息会被提取为顶层 system 字段；其余 user/assistant 消息按顺序放入 messages。
func buildAnthropicRequestBody(model string, input GenerateInput, stream bool) (map[string]interface{}, error) {
	messages := normalizeMessages(input.Messages)
	providerTools, toolDefinitions, toolsEnabled, err := toolDeclarationsForInput(input)
	if err != nil {
		return nil, err
	}

	var systemParts []string
	var systemBlocks []map[string]interface{}
	explicitCacheControl := false
	anthropicMessages := make([]map[string]interface{}, 0, len(messages))
	maxTokens := anthropicMaxTokensFromOptions(input.Options)

	for _, msg := range messages {
		if msg.Role == "system" {
			// system role → 顶层 system 字段（拼接多条）
			if text := extractMessageText(msg); text != "" {
				systemParts = append(systemParts, text)
				block := map[string]interface{}{"type": "text", "text": text}
				if cacheControl := anthropicCacheControlFromHint(msg.CacheControl, input.Options); len(cacheControl) > 0 {
					block["cache_control"] = cacheControl
					explicitCacheControl = true
				}
				systemBlocks = append(systemBlocks, block)
			}
			continue
		}
		anthropicMessages = append(anthropicMessages, map[string]interface{}{
			"role":    normalizeAnthropicRole(msg.Role),
			"content": buildAnthropicContent(msg),
		})
	}

	payload := map[string]interface{}{
		"model":      strings.TrimSpace(model),
		"max_tokens": maxTokens,
		"messages":   anthropicMessages,
		"stream":     stream,
	}
	if value, ok := modelParamFloat(input.Options, "temperature"); ok {
		payload["temperature"] = value
	}
	if value, ok := modelParamFloat(input.Options, "top_p"); ok {
		payload["top_p"] = value
	}
	if topK := modelParamInt(input.Options, "top_k"); topK > 0 {
		payload["top_k"] = topK
	}
	if stops := modelParamStringList(input.Options, "stop"); len(stops) > 0 {
		payload["stop_sequences"] = stops
	}
	if thinking := anthropicThinkingFromOptions(input.Options, maxTokens); len(thinking) > 0 {
		payload["thinking"] = thinking
	}
	if toolChoice := anthropicToolChoiceFromOptions(input.Options); len(toolChoice) > 0 {
		payload["tool_choice"] = toolChoice
	}
	if outputConfig := anthropicOutputConfigFromOptions(input.Options); len(outputConfig) > 0 {
		payload["output_config"] = outputConfig
	}
	if speed := strings.TrimSpace(modelParamString(input.Options, "speed")); speed != "" {
		payload["speed"] = speed
	}
	webSearchTools := []map[string]interface{}{}
	if toolsEnabled && modelParamBool(input.Options, "web_search") {
		webSearchTools = append(webSearchTools, map[string]interface{}{
			"type": "web_search_20250305",
			"name": "web_search",
		})
	}
	appendToolDeclarations(payload, providerTools, webSearchTools, buildAnthropicTools(toolDefinitions))
	if len(systemBlocks) > 0 {
		if explicitCacheControl {
			payload["system"] = systemBlocks
		} else {
			payload["system"] = strings.Join(systemParts, "\n\n")
		}
	}
	if !explicitCacheControl {
		if cacheControl := anthropicCacheControlFromOptions(input.Options); len(cacheControl) > 0 {
			payload["cache_control"] = cacheControl
		}
	}
	applyProviderOptions(payload, input.Options,
		"anthropic-beta", "anthropic_beta", "betas", "cache_control", "contents", "input", "instructions", "max_output_tokens", "max_tokens", "messages", "model", "output_config", "output_format", "prompt", "prompt_cache", "response_format", "speed", "stream", "stream_options", "system", "systemInstruction", "thinking", "tool_choice", "tools",
		"enable_cache", "cache_timeout", "enable_thinking", "thinking_display", "effort",
	)
	normalizeAnthropicNativeTools(payload)
	return payload, nil
}

func anthropicCacheControlFromHint(hint *CacheControl, options map[string]interface{}) map[string]interface{} {
	if hint == nil || !anthropicPromptCacheEnabled(options) {
		return nil
	}
	cacheControl := map[string]interface{}{"type": "ephemeral"}
	if value := strings.TrimSpace(hint.Type); value != "" {
		cacheControl["type"] = value
	}
	ttl := strings.TrimSpace(hint.TTL)
	if ttl == "" {
		ttl = anthropicPromptCacheTTL(options)
	}
	if ttl != "" {
		cacheControl["ttl"] = ttl
	}
	return cacheControl
}

func anthropicMaxTokensFromOptions(options map[string]interface{}) int {
	if requestedMaxTokens, ok := modelParamIntValue(options, "max_output_tokens"); ok && requestedMaxTokens >= 0 {
		return requestedMaxTokens
	}
	if requestedMaxTokens, ok := modelParamIntValue(options, "max_tokens"); ok && requestedMaxTokens >= 0 {
		return requestedMaxTokens
	}
	return anthropicDefaultMaxTokens
}

func anthropicPromptCacheEnabled(options map[string]interface{}) bool {
	if options == nil {
		return true
	}
	if enabled, ok := modelParamBoolValue(options, "enable_cache"); ok {
		return enabled
	}
	raw, ok := options["prompt_cache"]
	if !ok || raw == nil {
		return true
	}
	typed, ok := raw.(bool)
	if !ok {
		return true
	}
	return typed
}

func anthropicCacheControlFromOptions(options map[string]interface{}) map[string]interface{} {
	if !anthropicPromptCacheEnabled(options) {
		return nil
	}
	if raw, ok := options["cache_control"]; ok && raw != nil {
		if typed := asMap(raw); len(typed) > 0 {
			cacheControl := cloneMap(typed)
			if strings.TrimSpace(getString(cacheControl["type"])) == "" {
				cacheControl["type"] = "ephemeral"
			}
			return cacheControl
		}
	}
	cacheControl := map[string]interface{}{"type": "ephemeral"}
	if ttl := anthropicPromptCacheTTL(options); ttl != "" {
		cacheControl["ttl"] = ttl
	}
	return cacheControl
}

func anthropicPromptCacheTTL(options map[string]interface{}) string {
	switch strings.TrimSpace(strings.ToLower(modelParamString(options, "cache_timeout"))) {
	case "5m":
		return "5m"
	case "1h":
		return "1h"
	default:
		return ""
	}
}

func anthropicThinkingFromOptions(options map[string]interface{}, maxTokens int) map[string]interface{} {
	result := anthropicBaseThinkingFromOptions(options, maxTokens)
	if enabled, ok := modelParamBoolValue(options, "enable_thinking"); ok {
		if result == nil {
			result = map[string]interface{}{}
		}
		if !enabled {
			result["type"] = "disabled"
			delete(result, "budget_tokens")
		} else if budgetTokens := modelParamInt(options, "budget_tokens"); budgetTokens >= 1024 {
			result["type"] = "enabled"
			result["budget_tokens"] = budgetTokens
		} else {
			result["type"] = "adaptive"
		}
	}
	if display := strings.TrimSpace(modelParamString(options, "thinking_display")); display != "" {
		if result == nil {
			result = map[string]interface{}{}
		}
		result["display"] = display
	}
	if len(result) == 0 {
		return nil
	}
	if strings.TrimSpace(getString(result["type"])) == "" {
		result["type"] = "enabled"
	}
	return result
}

func anthropicBaseThinkingFromOptions(options map[string]interface{}, maxTokens int) map[string]interface{} {
	if thinking := modelParamMap(options, "thinking"); len(thinking) > 0 {
		result := make(map[string]interface{}, len(thinking))
		for key, value := range thinking {
			result[key] = value
		}
		return result
	}
	if value := modelParamString(options, "thinking"); value != "" {
		return map[string]interface{}{"type": value}
	}
	if raw, ok := options["thinking"].(bool); ok {
		if !raw {
			return map[string]interface{}{"type": "disabled"}
		}
		budgetTokens := modelParamInt(options, "budget_tokens")
		if budgetTokens < 1024 {
			budgetTokens = 1024
			if maxTokens > 2048 {
				budgetTokens = min(maxTokens/4, maxTokens-1)
			}
		}
		return map[string]interface{}{"type": "enabled", "budget_tokens": budgetTokens}
	}
	if budgetTokens := modelParamInt(options, "budget_tokens"); budgetTokens >= 1024 {
		return map[string]interface{}{
			"type":          "enabled",
			"budget_tokens": budgetTokens,
		}
	}
	return nil
}

func anthropicToolChoiceFromOptions(options map[string]interface{}) map[string]interface{} {
	raw, ok := options["tool_choice"]
	if !ok || raw == nil {
		return nil
	}
	if payload := asMap(raw); len(payload) > 0 {
		return payload
	}
	value := strings.TrimSpace(getString(raw))
	if value == "" {
		return nil
	}
	switch value {
	case "auto", "any", "none":
		return map[string]interface{}{"type": value}
	default:
		return map[string]interface{}{"type": "tool", "name": value}
	}
}

func anthropicOutputConfigFromOptions(options map[string]interface{}) map[string]interface{} {
	var result map[string]interface{}
	if outputConfig := modelParamMap(options, "output_config"); len(outputConfig) > 0 {
		result = make(map[string]interface{}, len(outputConfig))
		for key, value := range outputConfig {
			result[key] = value
		}
	}
	format := modelParamMap(options, "output_format")
	if len(format) == 0 {
		if normalizedFormat, ok := normalizedJSONResponseFormat(options); ok {
			format = asMap(normalizedFormat)
		}
	}
	if len(format) > 0 {
		if result == nil {
			result = map[string]interface{}{}
		}
		if _, exists := result["format"]; !exists {
			result["format"] = format
		}
	}
	if effort := strings.TrimSpace(modelParamString(options, "effort")); effort != "" {
		if result == nil {
			result = map[string]interface{}{}
		}
		if strings.TrimSpace(getString(result["effort"])) == "" {
			result["effort"] = effort
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func buildAnthropicTools(tools []ToolDefinition) []map[string]interface{} {
	if len(tools) == 0 {
		return nil
	}
	items := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		items = append(items, map[string]interface{}{
			"name":         name,
			"description":  strings.TrimSpace(tool.Description),
			"input_schema": decodeToolSchema(tool.InputSchema),
		})
	}
	return items
}

func normalizeAnthropicNativeTools(payload map[string]interface{}) {
	if payload == nil {
		return
	}
	tools := anthropicToolPayloads(payload["tools"])
	if len(tools) == 0 {
		return
	}
	changed := false
	for index, tool := range tools {
		toolType := strings.TrimSpace(getString(tool["type"]))
		if toolType == "" {
			continue
		}
		next := cloneMap(tool)
		hasChange := false
		if strings.TrimSpace(getString(next["name"])) == "" {
			name := anthropicNativeToolName(toolType)
			if name != "" {
				next["name"] = name
				hasChange = true
			}
		}
		if _, ok := next["allowed_callers"]; !ok {
			if callers := anthropicNativeToolAllowedCallers(toolType); len(callers) > 0 {
				next["allowed_callers"] = callers
				hasChange = true
			}
		}
		if !hasChange {
			continue
		}
		tools[index] = next
		changed = true
	}
	if changed {
		payload["tools"] = tools
	}
}

func anthropicNativeToolName(toolType string) string {
	switch strings.TrimSpace(toolType) {
	case "web_search_20250305", "web_search_20260209":
		return "web_search"
	case "web_fetch_20250910", "web_fetch_20260209":
		return "web_fetch"
	case "code_execution_20250825", "code_execution_20260120":
		return "code_execution"
	case "advisor_20260301":
		return "advisor"
	case "tool_search_tool_regex_20251119":
		return "tool_search_tool_regex"
	case "tool_search_tool_bm25_20251119":
		return "tool_search_tool_bm25"
	default:
		return ""
	}
}

func anthropicNativeToolAllowedCallers(toolType string) []string {
	switch strings.TrimSpace(toolType) {
	case "web_search_20260209", "web_fetch_20260209":
		return []string{"direct"}
	default:
		return nil
	}
}

type anthropicToolClassifier struct {
	nativeServerToolNames map[string]struct{}
	clientToolNames       map[string]struct{}
}

func newAnthropicToolClassifier(payload map[string]interface{}, clientTools []ToolDefinition) anthropicToolClassifier {
	classifier := anthropicToolClassifier{}
	for _, tool := range anthropicToolPayloads(payload["tools"]) {
		toolType := strings.TrimSpace(getString(tool["type"]))
		if !isAnthropicNativeToolType(toolType) {
			continue
		}
		toolName := strings.TrimSpace(getString(tool["name"]))
		if toolName == "" {
			toolName = anthropicNativeToolName(toolType)
		}
		if toolName == "" {
			continue
		}
		if classifier.nativeServerToolNames == nil {
			classifier.nativeServerToolNames = map[string]struct{}{}
		}
		classifier.nativeServerToolNames[toolName] = struct{}{}
	}
	for _, tool := range clientTools {
		toolName := strings.TrimSpace(tool.Name)
		if toolName == "" {
			continue
		}
		if classifier.clientToolNames == nil {
			classifier.clientToolNames = map[string]struct{}{}
		}
		classifier.clientToolNames[toolName] = struct{}{}
	}
	return classifier
}

func (c anthropicToolClassifier) isUnsupportedNativeClientToolUse(toolName string) bool {
	name := strings.TrimSpace(toolName)
	if name == "" || len(c.nativeServerToolNames) == 0 {
		return false
	}
	if _, ok := c.nativeServerToolNames[name]; !ok {
		return false
	}
	_, clientToolDeclared := c.clientToolNames[name]
	return !clientToolDeclared
}

func isAnthropicNativeToolType(toolType string) bool {
	switch strings.TrimSpace(toolType) {
	case "web_search_20250305",
		"web_search_20260209",
		"web_fetch_20250910",
		"web_fetch_20260209",
		"code_execution_20250825",
		"code_execution_20260120",
		"advisor_20260301",
		"tool_search_tool_regex_20251119",
		"tool_search_tool_bm25_20251119":
		return true
	default:
		return false
	}
}

func isAnthropicServerToolResultBlockType(blockType string) bool {
	value := strings.TrimSpace(blockType)
	return value != "" && strings.HasSuffix(value, "_tool_result")
}

func anthropicUnsupportedNativeToolError(toolName string, rawBody string) error {
	return &UpstreamError{
		StatusCode: http.StatusBadGateway,
		Message:    anthropicUnsupportedNativeToolMessage(toolName),
		Body:       rawBody,
	}
}

func anthropicUnsupportedNativeToolMessage(toolName string) string {
	name := strings.TrimSpace(toolName)
	if name == "" {
		name = "native_tool"
	}
	return fmt.Sprintf("Anthropic native tool %q must be executed server-side, but the selected upstream returned it as a client-side tool call. Disable this Claude native tool for the channel or use an MCP tool instead.", name)
}

// normalizeAnthropicRole 将内部 role 映射到 Anthropic 的 user/assistant。
// Anthropic Messages API 只接受这两种角色（system 已被提升到顶层）。
func normalizeAnthropicRole(role string) string {
	switch role {
	case "assistant":
		return "assistant"
	default:
		return "user"
	}
}

// extractMessageText 从纯文本消息中提取文字（用于 system 提升）。
func extractMessageText(msg Message) string {
	if len(msg.Parts) == 0 {
		return msg.Content
	}
	chunks := make([]string, 0, len(msg.Parts))
	for _, p := range msg.Parts {
		if p.Kind == ContentPartText || p.Kind == ContentPartFile {
			chunks = append(chunks, p.Text)
		}
	}
	return strings.Join(chunks, "\n")
}

// buildAnthropicContent 将 Message 转换为 Anthropic content 数组。
// 纯文本消息可简化为字符串（Anthropic 支持简化形式）；多模态保持数组。
func buildAnthropicContent(msg Message) interface{} {
	if len(msg.Parts) == 0 && len(msg.ToolCalls) == 0 && len(msg.ToolResults) == 0 {
		// 简化形式：纯文本直接用字符串
		return msg.Content
	}

	blocks := make([]map[string]interface{}, 0, len(msg.Parts)+len(msg.ToolCalls)+len(msg.ToolResults)+1)
	if text := strings.TrimSpace(msg.Content); text != "" {
		blocks = append(blocks, map[string]interface{}{
			"type": "text",
			"text": text,
		})
	}
	for _, part := range msg.Parts {
		switch part.Kind {
		case ContentPartImage:
			if len(part.Data) == 0 {
				continue
			}
			mime := strings.TrimSpace(part.MimeType)
			if mime == "" {
				mime = "image/jpeg"
			}
			blocks = append(blocks, map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type":       "base64",
					"media_type": mime,
					"data":       base64.StdEncoding.EncodeToString(part.Data),
				},
			})
		default: // text, file
			text := part.Text
			if strings.TrimSpace(text) == "" {
				continue
			}
			blocks = append(blocks, map[string]interface{}{
				"type": "text",
				"text": text,
			})
		}
	}
	for _, item := range msg.ToolCalls {
		args := strings.TrimSpace(item.ArgumentsJSON)
		if args == "" {
			args = "{}"
		}
		input := make(map[string]interface{})
		if err := json.Unmarshal([]byte(args), &input); err != nil {
			input = map[string]interface{}{"arguments": args}
		}
		blocks = append(blocks, map[string]interface{}{
			"type":  "tool_use",
			"id":    strings.TrimSpace(item.ToolCallID),
			"name":  strings.TrimSpace(item.ToolName),
			"input": input,
		})
	}
	for _, item := range msg.ToolResults {
		block := map[string]interface{}{
			"type":        "tool_result",
			"tool_use_id": strings.TrimSpace(item.ToolCallID),
			"content":     buildToolResultContent(item),
		}
		if strings.TrimSpace(item.Error) != "" {
			block["is_error"] = true
		}
		blocks = append(blocks, block)
	}

	if len(blocks) == 0 {
		return msg.Content
	}
	return blocks
}

// ── HTTP 请求辅助 ──────────────────────────────────────────────────────────────

func (c *Client) newAnthropicRequest(
	ctx context.Context,
	method, url string,
	body io.Reader,
	route RouteConfig,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", anthropicVersion)
	if apiKey := strings.TrimSpace(route.APIKey); apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}
	setAdditionalHeaders(req, route.HeadersJSON)
	return req, nil
}

// parseAnthropicError 从 Anthropic 错误响应中提取 error.message。
// 格式：{"type":"error","error":{"type":"...","message":"..."}}
func parseAnthropicError(statusCode int, body []byte, debug *UpstreamDebugSnapshot) error {
	parsed := make(map[string]interface{})
	if err := json.Unmarshal(body, &parsed); err == nil {
		if msg := getStringFromPath(parsed, "error", "message"); msg != "" {
			return &UpstreamError{StatusCode: statusCode, Message: msg, Body: string(body), Debug: debug}
		}
	}
	return &UpstreamError{
		StatusCode: statusCode,
		Message:    fmt.Sprintf("upstream_status_%d", statusCode),
		Body:       string(body),
		Debug:      debug,
	}
}

func applyAnthropicBetaHeaders(req *http.Request, payload map[string]interface{}, options map[string]interface{}) {
	if req == nil || len(payload) == 0 {
		return
	}
	betas := make([]string, 0, 2)
	betas = appendUniqueStrings(betas, modelParamStringList(options, "betas")...)
	betas = appendUniqueStrings(betas, modelParamStringList(options, "anthropic_beta")...)
	betas = appendUniqueStrings(betas, modelParamStringList(options, "anthropic-beta")...)
	if strings.EqualFold(strings.TrimSpace(getString(payload["speed"])), "fast") {
		betas = appendUniqueStrings(betas, "fast-mode-2026-02-01")
	}
	for _, tool := range anthropicToolPayloads(payload["tools"]) {
		switch strings.TrimSpace(getString(tool["type"])) {
		case "advisor_20260301":
			betas = appendUniqueStrings(betas, "advisor-2026-03-01")
		case "mcp_toolset_20251119":
			betas = appendUniqueStrings(betas, "mcp-client-2025-11-20")
		}
	}
	if len(betas) == 0 {
		return
	}
	existing := strings.Split(req.Header.Get("anthropic-beta"), ",")
	for _, item := range existing {
		betas = appendUniqueStrings(betas, strings.TrimSpace(item))
	}
	req.Header.Set("anthropic-beta", strings.Join(betas, ","))
}

func anthropicToolPayloads(raw interface{}) []map[string]interface{} {
	switch typed := raw.(type) {
	case []map[string]interface{}:
		return append([]map[string]interface{}(nil), typed...)
	case []interface{}:
		items := make([]map[string]interface{}, 0, len(typed))
		for _, item := range typed {
			if payload := asMap(item); len(payload) > 0 {
				items = append(items, payload)
			}
		}
		return items
	default:
		return nil
	}
}

// ── 非流式调用 ────────────────────────────────────────────────────────────────

func (c *Client) generateAnthropic(
	ctx context.Context,
	route RouteConfig,
	input GenerateInput,
	_ bool, // stream 参数保留，始终传 false
) (*GenerateOutput, error) {
	requestURL := buildAnthropicMessagesURL(route.BaseURL)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	requestBody, err := buildAnthropicRequestBody(route.UpstreamModel, input, false)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	req, err := c.newAnthropicRequest(requestCtx, http.MethodPost, requestURL, bytes.NewReader(payload), route)
	if err != nil {
		return nil, err
	}
	applyAnthropicBetaHeaders(req, requestBody, input.Options)

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
		return nil, parseAnthropicError(resp.StatusCode, body, upstreamDebugSnapshot(req, payload, resp, body))
	}

	debug := upstreamDebugSnapshot(req, payload, resp, body)
	output, err := parseAnthropicResponse(body, newAnthropicToolClassifier(requestBody, input.Tools))
	if err != nil {
		return nil, attachUpstreamDebug(err, debug)
	}
	output.Debug = debug
	return output, nil
}

// parseAnthropicResponse 解析 Anthropic 非流式响应。
//
// 响应格式：
//
//	{
//	  "id": "msg_...",
//	  "content": [{"type":"text","text":"..."}],
//	  "usage": {
//	    "input_tokens": 10,
//	    "output_tokens": 100,
//	    "cache_creation_input_tokens": 0,
//	    "cache_read_input_tokens": 0
//	  }
//	}
func parseAnthropicResponse(body []byte, classifier anthropicToolClassifier) (*GenerateOutput, error) {
	parsed := make(map[string]interface{})
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	toolCalls, err := parseAnthropicToolUse(parsed, classifier, string(body))
	if err != nil {
		return nil, err
	}

	result := &GenerateOutput{
		ResponseID:          strings.TrimSpace(getString(parsed["id"])),
		Text:                extractAnthropicText(parsed),
		Reasoning:           extractAnthropicReasoning(parsed),
		Usage:               parseAnthropicUsage(parsed),
		ToolCalls:           toolCalls,
		ServerToolCalls:     parseAnthropicServerToolUse(parsed),
		ServerSideToolUsage: parseAnthropicServerSideToolUsage(parsed),
		RawJSON:             string(body),
	}
	return result, nil
}

// extractAnthropicText 从 content 数组中提取所有 text block 的文字。
func extractAnthropicText(parsed map[string]interface{}) string {
	chunks := make([]string, 0)
	for _, raw := range asSlice(parsed["content"]) {
		block := asMap(raw)
		if getString(block["type"]) == "text" {
			if text := strings.TrimSpace(getString(block["text"])); text != "" {
				chunks = append(chunks, text)
			}
		}
	}
	return strings.Join(chunks, "")
}

func extractAnthropicReasoning(parsed map[string]interface{}) *ReasoningOutput {
	result := &ReasoningOutput{}
	for _, raw := range asSlice(parsed["content"]) {
		block := asMap(raw)
		blockType := strings.TrimSpace(getString(block["type"]))
		switch {
		case strings.Contains(blockType, "thinking") || strings.Contains(blockType, "reason"):
			result.Text += extractReasoningDeltaText(block)
			if signature := strings.TrimSpace(getString(block["signature"])); signature != "" {
				result.Signature = signature
			}
		}
	}
	if strings.TrimSpace(result.Text) == "" && strings.TrimSpace(result.Signature) == "" {
		return nil
	}
	return result
}

// parseAnthropicUsage 解析 Anthropic usage 字段。
func parseAnthropicUsage(parsed map[string]interface{}) Usage {
	cacheReadInputTokens := getInt64FromPath(parsed, "usage", "cache_read_input_tokens")
	cacheCreation1hInputTokens := getInt64FromPath(parsed, "usage", "cache_creation", "ephemeral_1h_input_tokens")
	cacheCreation5mInputTokens := getInt64FromPath(parsed, "usage", "cache_creation", "ephemeral_5m_input_tokens")
	cacheCreationByTTL := cacheCreation1hInputTokens + cacheCreation5mInputTokens
	cacheCreationInputTokens := cacheCreationByTTL
	if cacheCreationInputTokens <= 0 {
		cacheCreationInputTokens = getInt64FromPath(parsed, "usage", "cache_creation_input_tokens")
	}
	return Usage{
		InputTokens:        getInt64FromPath(parsed, "usage", "input_tokens"),
		OutputTokens:       getInt64FromPath(parsed, "usage", "output_tokens"),
		CacheReadTokens:    cacheReadInputTokens,
		CacheWriteTokens:   cacheCreationInputTokens,
		CacheWrite5mTokens: cacheCreation5mInputTokens,
		CacheWrite1hTokens: cacheCreation1hInputTokens,
		Speed:              strings.TrimSpace(getStringFromPath(parsed, "usage", "speed")),
		RawUsageJSON:       rawUsageJSONFromPath(parsed, "usage"),
	}
}

func parseAnthropicServerSideToolUsage(parsed map[string]interface{}) map[string]int64 {
	usage := asMap(parsed["usage"])
	if len(usage) == 0 {
		return nil
	}
	raw := asMap(usage["server_tool_use"])
	if len(raw) == 0 {
		raw = asMap(usage["server_side_tool_usage"])
	}
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string]int64, len(raw))
	for key, value := range raw {
		normalized := normalizeAnthropicServerSideToolUsageKey(key)
		count := toInt64(value)
		if normalized == "" || count <= 0 {
			continue
		}
		result[normalized] += count
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeAnthropicServerSideToolUsageKey(key string) string {
	value := strings.TrimSpace(key)
	value = strings.TrimSuffix(value, "_requests")
	value = strings.TrimSuffix(value, "_request")
	value = strings.TrimSuffix(value, "_calls")
	value = strings.TrimSuffix(value, "_call")
	return strings.TrimSpace(value)
}

// parseAnthropicToolUse 解析需要本地执行的 Anthropic tool_use content block。
func parseAnthropicToolUse(parsed map[string]interface{}, classifier anthropicToolClassifier, rawBody string) ([]ToolCall, error) {
	var result []ToolCall
	for _, raw := range asSlice(parsed["content"]) {
		block := asMap(raw)
		if getString(block["type"]) != "tool_use" {
			continue
		}
		toolName := strings.TrimSpace(getString(block["name"]))
		if classifier.isUnsupportedNativeClientToolUse(toolName) {
			return nil, anthropicUnsupportedNativeToolError(toolName, rawBody)
		}
		arguments := normalizeJSONString(block["input"])
		if arguments == "" {
			arguments = "{}"
		}
		result = append(result, ToolCall{
			ToolCallID:    strings.TrimSpace(getString(block["id"])),
			ToolType:      "function",
			ToolName:      toolName,
			ArgumentsJSON: arguments,
			Status:        "requested",
		})
	}
	if result == nil {
		return make([]ToolCall, 0), nil
	}
	return result, nil
}

// parseAnthropicServerToolUse 解析 Anthropic server-side tool trace。
func parseAnthropicServerToolUse(parsed map[string]interface{}) []ToolCall {
	items := make([]ToolCall, 0)
	indexByID := map[string]int{}
	for _, raw := range asSlice(parsed["content"]) {
		block := asMap(raw)
		blockType := strings.TrimSpace(getString(block["type"]))
		switch {
		case blockType == "server_tool_use":
			arguments := normalizeJSONString(block["input"])
			if arguments == "" {
				arguments = "{}"
			}
			toolCall := ToolCall{
				ToolCallID:    strings.TrimSpace(getString(block["id"])),
				ToolType:      blockType,
				ToolName:      strings.TrimSpace(getString(block["name"])),
				ArgumentsJSON: arguments,
				Status:        "completed",
			}
			if toolCall.ToolCallID != "" {
				indexByID[toolCall.ToolCallID] = len(items)
			}
			items = append(items, toolCall)
		case isAnthropicServerToolResultBlockType(blockType):
			toolUseID := strings.TrimSpace(getString(block["tool_use_id"]))
			if toolUseID == "" {
				continue
			}
			index, ok := indexByID[toolUseID]
			if !ok {
				continue
			}
			output := anthropicServerToolResultOutputJSON(block)
			if output == "" {
				output = "{}"
			}
			items[index].OutputJSON = output
			items[index].Status = "completed"
		}
	}
	if items == nil {
		return make([]ToolCall, 0)
	}
	return items
}

// ── 流式调用 ──────────────────────────────────────────────────────────────────

func (c *Client) generateAnthropicStream(
	ctx context.Context,
	route RouteConfig,
	input GenerateInput,
	onEvent func(GenerateStreamEvent) error,
) (*GenerateOutput, error) {
	requestURL := buildAnthropicMessagesURL(route.BaseURL)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	requestBody, err := buildAnthropicRequestBody(route.UpstreamModel, input, true)
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

	req, err := c.newAnthropicRequest(firstByteCtx, http.MethodPost, requestURL, bytes.NewReader(payload), route)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	applyAnthropicBetaHeaders(req, requestBody, input.Options)

	resp, err := c.httpClientForRoute(route).Do(req)
	firstByteTimer.Stop()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := readUpstreamBody(resp.Body)
		return nil, parseAnthropicError(resp.StatusCode, body, upstreamDebugSnapshot(req, payload, resp, body))
	}

	result := &GenerateOutput{
		ToolCalls:       make([]ToolCall, 0),
		ServerToolCalls: make([]ToolCall, 0),
	}

	idleReader := newIdleTimeoutReader(resp.Body, resolveStreamIdleTimeout(route.StreamIdleTimeoutMS))
	streamBody := newUpstreamBodyRecorder(idleReader)
	if err = consumeAnthropicStream(streamBody, result, onEvent, newAnthropicToolClassifier(requestBody, input.Tools)); err != nil {
		return nil, attachUpstreamDebug(err, upstreamDebugSnapshot(req, payload, resp, streamErrorBody(streamBody, err)))
	}
	compactAnthropicStreamToolCalls(result)
	return result, nil
}

// consumeAnthropicStream 解析 Anthropic SSE 流事件并回填 result。
//
// Anthropic SSE 格式：
//
//	event: message_start
//	data: {"type":"message_start","message":{"id":"msg_...","usage":{...}}}
//
//	event: content_block_delta
//	data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}
//
//	event: message_delta
//	data: {"type":"message_delta","usage":{"output_tokens":100}}
//
//	event: message_stop
//	data: {"type":"message_stop"}
func consumeAnthropicStream(
	reader io.Reader,
	result *GenerateOutput,
	onEvent func(GenerateStreamEvent) error,
	classifier anthropicToolClassifier,
) error {
	// 使用 Anthropic 自身的 SSE dispatch，保持事件语义独立于 OpenAI-family 解析。
	// Anthropic 专用事件处理函数。
	// Anthropic 的事件名称在 JSON payload 中，独立扫描可以保留该协议的解析边界。
	type sseEvent struct {
		name string
		data string
	}

	dispatch := func(ev sseEvent) error {
		if strings.TrimSpace(ev.data) == "" {
			return nil
		}
		parsed := make(map[string]interface{})
		if err := json.Unmarshal([]byte(ev.data), &parsed); err != nil {
			return nil // 单个异常事件不应中断后续流式输出。
		}
		return applyAnthropicStreamEvent(parsed, ev.data, result, onEvent, classifier)
	}

	var (
		eventName string
		dataLines []string
	)

	flush := func() error {
		ev := sseEvent{name: eventName, data: strings.Join(dataLines, "\n")}
		eventName = ""
		dataLines = dataLines[:0]
		return dispatch(ev)
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			if err := flush(); err != nil {
				if isAnthropicStreamDone(err) {
					return nil
				}
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(line[len("event:"):])
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line[len("data:"):], " ")
			dataLines = append(dataLines, data)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if err := flush(); err != nil && !isAnthropicStreamDone(err) {
		return err
	}
	return nil
}

var errAnthropicStreamDone = fmt.Errorf("anthropic stream done")

func isAnthropicStreamDone(err error) bool {
	return err == errAnthropicStreamDone
}

// applyAnthropicStreamEvent 处理单个 Anthropic 流式事件 payload。
func applyAnthropicStreamEvent(
	parsed map[string]interface{},
	rawBody string,
	result *GenerateOutput,
	onEvent func(GenerateStreamEvent) error,
	classifier anthropicToolClassifier,
) error {
	eventType := strings.TrimSpace(getString(parsed["type"]))

	switch eventType {
	case "message_start":
		msg := asMap(parsed["message"])
		if id := strings.TrimSpace(getString(msg["id"])); id != "" {
			result.ResponseID = id
		}
		// message_start 中的 usage 包含 input_tokens 和缓存统计
		result.Usage = parseAnthropicUsage(msg)
		if serverSideToolUsage := parseAnthropicServerSideToolUsage(msg); len(serverSideToolUsage) > 0 {
			result.ServerSideToolUsage = serverSideToolUsage
		}
		if result.Usage != (Usage{}) && onEvent != nil {
			return onEvent(GenerateStreamEvent{
				Usage:      result.Usage,
				ResponseID: result.ResponseID,
			})
		}

	case "content_block_start":
		block := asMap(parsed["content_block"])
		index := anthropicContentBlockIndex(parsed)
		switch strings.TrimSpace(getString(block["type"])) {
		case "tool_use":
			toolName := strings.TrimSpace(getString(block["name"]))
			if classifier.isUnsupportedNativeClientToolUse(toolName) {
				message := anthropicUnsupportedNativeToolMessage(toolName)
				toolCall := upsertAnthropicStreamServerToolCall(result, index, ToolCall{
					ToolCallID:    strings.TrimSpace(getString(block["id"])),
					ToolType:      "native_tool_use",
					ToolName:      toolName,
					ArgumentsJSON: normalizeJSONString(block["input"]),
					Status:        "error",
					ErrorJSON:     message,
				})
				if onEvent != nil {
					if err := onEvent(GenerateStreamEvent{
						ServerToolCall: &toolCall,
						ResponseID:     result.ResponseID,
					}); err != nil {
						return err
					}
				}
				return anthropicUnsupportedNativeToolError(toolName, rawBody)
			}
			_ = upsertAnthropicStreamToolCall(result, index, ToolCall{
				ToolCallID:    strings.TrimSpace(getString(block["id"])),
				ToolType:      "function",
				ToolName:      toolName,
				ArgumentsJSON: normalizeJSONString(block["input"]),
				Status:        "requested",
			})
		case "server_tool_use":
			toolCall := upsertAnthropicStreamServerToolCall(result, index, ToolCall{
				ToolCallID:    strings.TrimSpace(getString(block["id"])),
				ToolType:      strings.TrimSpace(getString(block["type"])),
				ToolName:      strings.TrimSpace(getString(block["name"])),
				ArgumentsJSON: normalizeJSONString(block["input"]),
				Status:        "in_progress",
			})
			if onEvent != nil {
				return onEvent(GenerateStreamEvent{
					ServerToolCall: &toolCall,
					ResponseID:     result.ResponseID,
				})
			}
		default:
			if isAnthropicServerToolResultBlockType(strings.TrimSpace(getString(block["type"]))) {
				toolCall, ok := mergeAnthropicStreamServerToolResult(result, block)
				if !ok {
					return nil
				}
				if onEvent != nil {
					return onEvent(GenerateStreamEvent{
						ServerToolCall: &toolCall,
						ResponseID:     result.ResponseID,
					})
				}
			}
		}

	case "content_block_delta":
		delta := asMap(parsed["delta"])
		deltaType := strings.TrimSpace(getString(delta["type"]))
		if deltaType == "text_delta" {
			text := getString(delta["text"])
			if text == "" {
				return nil
			}
			result.Text += text
			if onEvent != nil {
				return onEvent(GenerateStreamEvent{
					Delta:      text,
					ResponseID: result.ResponseID,
				})
			}
		}
		if deltaType == "input_json_delta" {
			partial := getString(delta["partial_json"])
			if partial == "" {
				return nil
			}
			index := anthropicContentBlockIndex(parsed)
			if toolCall, ok := appendAnthropicStreamServerToolInput(result, index, partial); ok {
				if onEvent != nil {
					return onEvent(GenerateStreamEvent{
						ServerToolCall: &toolCall,
						ResponseID:     result.ResponseID,
					})
				}
				return nil
			}
			toolCall := appendAnthropicStreamToolInput(result, index, partial)
			if onEvent != nil {
				return onEvent(GenerateStreamEvent{
					ServerToolCall: &toolCall,
					ResponseID:     result.ResponseID,
				})
			}
			return nil
		}
		if deltaType == "signature_delta" {
			signature := strings.TrimSpace(getString(delta["signature"]))
			if signature == "" {
				return nil
			}
			mergeReasoningDeltaOutput(&result.Reasoning, &ReasoningDelta{
				EventType: "anthropic.content_block_delta",
				ItemID:    fmt.Sprintf("%v", parsed["index"]),
				Status:    deltaType,
				Kind:      "signature",
				Signature: signature,
			})
			if onEvent != nil {
				return onEvent(GenerateStreamEvent{
					Reasoning: &ReasoningDelta{
						EventType: "anthropic.content_block_delta",
						ItemID:    fmt.Sprintf("%v", parsed["index"]),
						Status:    deltaType,
						Kind:      "signature",
						Signature: signature,
					},
					ResponseID: result.ResponseID,
				})
			}
		}
		if strings.Contains(deltaType, "thinking") || strings.Contains(deltaType, "reason") {
			think := extractReasoningDeltaText(delta)
			if think == "" {
				return nil
			}
			reasoning := &ReasoningDelta{
				EventType: "anthropic.content_block_delta",
				ItemID:    fmt.Sprintf("%v", parsed["index"]),
				Status:    deltaType,
				Kind:      "content_text",
				Text:      think,
			}
			mergeReasoningDeltaOutput(&result.Reasoning, reasoning)
			if onEvent != nil {
				return onEvent(GenerateStreamEvent{
					Reasoning:  reasoning,
					ResponseID: result.ResponseID,
				})
			}
		}

	case "content_block_stop":
		index := anthropicContentBlockIndex(parsed)
		_, _ = markAnthropicStreamToolCallComplete(result, index)

	case "message_delta":
		// message_delta 包含最终 output_tokens
		if serverSideToolUsage := parseAnthropicServerSideToolUsage(parsed); len(serverSideToolUsage) > 0 {
			result.ServerSideToolUsage = serverSideToolUsage
		}
		deltaUsage := asMap(parsed["usage"])
		if out := toInt64(deltaUsage["output_tokens"]); out > 0 {
			result.Usage.OutputTokens = out
			result.Usage.RawUsageJSON = MergeRawUsageJSON(result.Usage.RawUsageJSON, rawUsageJSONFromPath(parsed, "usage"))
			if onEvent != nil {
				return onEvent(GenerateStreamEvent{
					Usage:      result.Usage,
					ResponseID: result.ResponseID,
				})
			}
		}

	case "message_stop":
		return errAnthropicStreamDone

	case "error":
		errBlock := asMap(parsed["error"])
		msg := strings.TrimSpace(getString(errBlock["message"]))
		if msg == "" {
			msg = "anthropic stream error"
		}
		return &UpstreamError{
			StatusCode: http.StatusBadGateway,
			Message:    msg,
			Body:       rawBody,
		}
	}

	return nil
}

func anthropicContentBlockIndex(parsed map[string]interface{}) int {
	index := int(toInt64(parsed["index"]))
	if index < 0 {
		return 0
	}
	return index
}

func upsertAnthropicStreamToolCall(result *GenerateOutput, index int, item ToolCall) ToolCall {
	if result == nil {
		return item
	}
	if result.ToolCalls == nil {
		result.ToolCalls = make([]ToolCall, 0)
	}
	for len(result.ToolCalls) <= index {
		result.ToolCalls = append(result.ToolCalls, ToolCall{Status: "requested"})
	}
	current := result.ToolCalls[index]
	if strings.TrimSpace(item.ToolCallID) != "" {
		current.ToolCallID = item.ToolCallID
	}
	if strings.TrimSpace(item.ToolType) != "" {
		current.ToolType = item.ToolType
	}
	if strings.TrimSpace(item.ToolName) != "" {
		current.ToolName = item.ToolName
	}
	if strings.TrimSpace(item.ArgumentsJSON) != "" && item.ArgumentsJSON != "{}" {
		current.ArgumentsJSON = item.ArgumentsJSON
	}
	if strings.TrimSpace(item.Status) != "" {
		current.Status = item.Status
	}
	result.ToolCalls[index] = current
	return current
}

func appendAnthropicStreamToolInput(result *GenerateOutput, index int, partial string) ToolCall {
	if result == nil || partial == "" {
		return ToolCall{}
	}
	upsertAnthropicStreamToolCall(result, index, ToolCall{ToolType: "function", Status: "requested"})
	result.ToolCalls[index].ArgumentsJSON += partial
	return result.ToolCalls[index]
}

func markAnthropicStreamToolCallComplete(result *GenerateOutput, index int) (ToolCall, bool) {
	if result == nil || index < 0 || index >= len(result.ToolCalls) {
		return ToolCall{}, false
	}
	if strings.TrimSpace(result.ToolCalls[index].ToolName) == "" && strings.TrimSpace(result.ToolCalls[index].ToolCallID) == "" {
		return ToolCall{}, false
	}
	if strings.TrimSpace(result.ToolCalls[index].ArgumentsJSON) == "" {
		result.ToolCalls[index].ArgumentsJSON = "{}"
	}
	if strings.TrimSpace(result.ToolCalls[index].Status) == "" {
		result.ToolCalls[index].Status = "requested"
	}
	return result.ToolCalls[index], true
}

func upsertAnthropicStreamServerToolCall(result *GenerateOutput, index int, item ToolCall) ToolCall {
	if result == nil {
		return item
	}
	if result.ServerToolCalls == nil {
		result.ServerToolCalls = make([]ToolCall, 0)
	}
	for len(result.ServerToolCalls) <= index {
		result.ServerToolCalls = append(result.ServerToolCalls, ToolCall{Status: "in_progress"})
	}
	current := result.ServerToolCalls[index]
	if strings.TrimSpace(item.ToolCallID) != "" {
		current.ToolCallID = item.ToolCallID
	}
	if strings.TrimSpace(item.ToolType) != "" {
		current.ToolType = item.ToolType
	}
	if strings.TrimSpace(item.ToolName) != "" {
		current.ToolName = item.ToolName
	}
	if strings.TrimSpace(item.ArgumentsJSON) != "" && item.ArgumentsJSON != "{}" {
		current.ArgumentsJSON = item.ArgumentsJSON
	}
	if strings.TrimSpace(item.Status) != "" {
		current.Status = item.Status
	}
	result.ServerToolCalls[index] = current
	return current
}

func appendAnthropicStreamServerToolInput(result *GenerateOutput, index int, partial string) (ToolCall, bool) {
	if result == nil || partial == "" || index < 0 || index >= len(result.ServerToolCalls) {
		return ToolCall{}, false
	}
	item := result.ServerToolCalls[index]
	if strings.TrimSpace(item.ToolCallID) == "" && strings.TrimSpace(item.ToolName) == "" && strings.TrimSpace(item.ToolType) == "" {
		return ToolCall{}, false
	}
	item.ArgumentsJSON += partial
	if strings.TrimSpace(item.Status) == "" {
		item.Status = "in_progress"
	}
	result.ServerToolCalls[index] = item
	return item, true
}

func markAnthropicStreamServerToolCallComplete(result *GenerateOutput, index int) (ToolCall, bool) {
	if result == nil || index < 0 || index >= len(result.ServerToolCalls) {
		return ToolCall{}, false
	}
	item := result.ServerToolCalls[index]
	if strings.TrimSpace(item.ToolCallID) == "" && strings.TrimSpace(item.ToolName) == "" && strings.TrimSpace(item.ToolType) == "" {
		return ToolCall{}, false
	}
	if strings.TrimSpace(item.ArgumentsJSON) == "" {
		item.ArgumentsJSON = "{}"
	}
	if strings.TrimSpace(item.Status) == "" || strings.TrimSpace(item.Status) == "in_progress" {
		item.Status = "completed"
	}
	result.ServerToolCalls[index] = item
	return item, true
}

func mergeAnthropicStreamServerToolResult(result *GenerateOutput, block map[string]interface{}) (ToolCall, bool) {
	if result == nil {
		return ToolCall{}, false
	}
	toolUseID := strings.TrimSpace(getString(block["tool_use_id"]))
	if toolUseID == "" {
		return ToolCall{}, false
	}
	for index := range result.ServerToolCalls {
		item := result.ServerToolCalls[index]
		if strings.TrimSpace(item.ToolCallID) != toolUseID {
			continue
		}
		item.OutputJSON = anthropicServerToolResultOutputJSON(block)
		if item.OutputJSON == "" {
			item.OutputJSON = "{}"
		}
		item.Status = "completed"
		result.ServerToolCalls[index] = item
		return item, true
	}
	return ToolCall{}, false
}

func anthropicServerToolResultOutputJSON(block map[string]interface{}) string {
	return normalizeJSONString(sanitizeAnthropicServerToolResultValue(block))
}

func sanitizeAnthropicServerToolResultValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			if isAnthropicOpaqueToolResultKey(key) {
				continue
			}
			result[key] = sanitizeAnthropicServerToolResultValue(item)
		}
		return result
	case []interface{}:
		items := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			items = append(items, sanitizeAnthropicServerToolResultValue(item))
		}
		return items
	default:
		return value
	}
}

func isAnthropicOpaqueToolResultKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "encrypted_content", "encryptedContent":
		return true
	default:
		return false
	}
}

func compactAnthropicStreamToolCalls(result *GenerateOutput) {
	if result == nil || len(result.ToolCalls) == 0 {
		compactAnthropicStreamServerToolCalls(result)
		return
	}
	items := make([]ToolCall, 0, len(result.ToolCalls))
	for _, item := range result.ToolCalls {
		if strings.TrimSpace(item.ToolCallID) == "" && strings.TrimSpace(item.ToolName) == "" {
			continue
		}
		if strings.TrimSpace(item.ArgumentsJSON) == "" {
			item.ArgumentsJSON = "{}"
		}
		if strings.TrimSpace(item.ToolType) == "" {
			item.ToolType = "function"
		}
		if strings.TrimSpace(item.Status) == "" {
			item.Status = "requested"
		}
		items = append(items, item)
	}
	result.ToolCalls = items
	compactAnthropicStreamServerToolCalls(result)
}

func compactAnthropicStreamServerToolCalls(result *GenerateOutput) {
	if result == nil || len(result.ServerToolCalls) == 0 {
		return
	}
	items := make([]ToolCall, 0, len(result.ServerToolCalls))
	for _, item := range result.ServerToolCalls {
		if strings.TrimSpace(item.ToolCallID) == "" && strings.TrimSpace(item.ToolName) == "" && strings.TrimSpace(item.ToolType) == "" {
			continue
		}
		if strings.TrimSpace(item.ArgumentsJSON) == "" {
			item.ArgumentsJSON = "{}"
		}
		if strings.TrimSpace(item.Status) == "" || strings.TrimSpace(item.Status) == "in_progress" {
			item.Status = "completed"
		}
		items = append(items, item)
	}
	result.ServerToolCalls = items
}

// ── Models 目录 ───────────────────────────────────────────────────────────────

// listModelsAnthropic 调用 Anthropic GET /v1/models 接口。
//
// 响应格式：
//
//	{"data":[{"id":"claude-opus-4-5","display_name":"...","type":"model"},...]}
func (c *Client) listModelsAnthropic(ctx context.Context, route RouteConfig) ([]ModelItem, error) {
	requestURL := buildAnthropicModelsURL(route.BaseURL)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	req, err := c.newAnthropicRequest(requestCtx, http.MethodGet, requestURL, nil, route)
	if err != nil {
		return nil, err
	}

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
		return nil, parseAnthropicError(resp.StatusCode, body, upstreamDebugSnapshot(req, nil, resp, body))
	}

	parsed := struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}{}
	if err = json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	results := make([]ModelItem, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		modelID := strings.TrimSpace(item.ID)
		if modelID == "" {
			continue
		}
		results = append(results, ModelItem{
			ID:      modelID,
			OwnedBy: "anthropic",
		})
	}
	return results, nil
}
