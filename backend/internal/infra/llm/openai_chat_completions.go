package llm

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// openAIChatCompletionsAdapter 实现 OpenAI Chat Completions API（POST /v1/chat/completions）。
type openAIChatCompletionsAdapter struct {
	client *Client
}

func (a *openAIChatCompletionsAdapter) Name() string { return portllm.AdapterOpenAIChatCompletions }

func (a *openAIChatCompletionsAdapter) Generate(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	route.Endpoint = portllm.EndpointChatCompletions
	return a.client.generateOpenAICompatible(ctx, route, input)
}

func (a *openAIChatCompletionsAdapter) GenerateStream(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput, onEvent func(portllm.GenerateStreamEvent) error) (*portllm.GenerateOutput, error) {
	route.Endpoint = portllm.EndpointChatCompletions
	return a.client.generateChatCompletionsStreamWithAutoUsageFallback(ctx, route, input, onEvent)
}

func (a *openAIChatCompletionsAdapter) ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	return a.client.listModelsOpenAICompatible(ctx, route)
}

func buildChatCompletionsRequestBody(
	adapter string,
	model string,
	input portllm.GenerateInput,
	messages []portllm.Message,
	providerTools []map[string]any,
	toolDefinitions []portllm.ToolDefinition,
	providerStreamOptions map[string]any,
	stream bool,
) map[string]any {
	promptCache := resolveOpenAIPromptCacheConfig(adapter, input)
	items := make([]map[string]any, 0, len(messages))
	for _, item := range messages {
		items = append(items, buildChatCompletionsMessages(adapter, item, &promptCache)...)
	}
	payload := map[string]any{
		"model":    strings.TrimSpace(model),
		"messages": items,
		"stream":   stream,
	}
	if streamOptions := chatCompletionsStreamOptions(providerStreamOptions, stream); len(streamOptions) > 0 {
		payload["stream_options"] = streamOptions
	}
	if effort := modelParamString(input.Options, "reasoning_effort"); effort != "" {
		payload["reasoning_effort"] = effort
	}
	if _, ok := input.Options["thinking"]; ok {
		thinkingType := "disabled"
		if modelParamBool(input.Options, "thinking") {
			thinkingType = "enabled"
		}
		payload["thinking"] = map[string]any{
			"type": thinkingType,
		}
	}
	if maxTokens := modelParamInt(input.Options, "max_completion_tokens"); maxTokens > 0 {
		payload["max_completion_tokens"] = maxTokens
	} else if maxTokens := modelParamInt(input.Options, "max_output_tokens"); maxTokens > 0 {
		payload["max_completion_tokens"] = maxTokens
	}
	applyOpenAICompatibleSamplingParams(payload, input.Options, true)
	if verbosity := modelParamString(input.Options, "verbosity"); verbosity != "" {
		payload["verbosity"] = verbosity
	}
	applyOpenAIPromptCacheRequestFields(payload, promptCache)
	appendToolDeclarations(payload, providerTools, buildOpenAITools(toolDefinitions, true))
	applyProviderOptions(payload, input.Options,
		"contents", "input", "instructions", "messages", "model", "prompt", "prompt_cache_key", "prompt_cache_options", "prompt_cache_retention", "response_format", "stream", "stream_options", "system", "systemInstruction", "tools",
	)
	return payload
}

func chatCompletionsStreamOptions(options map[string]any, stream bool) map[string]any {
	if !stream {
		return nil
	}
	result := map[string]any{"include_usage": true}
	for key, value := range options {
		result[key] = value
	}
	return result
}

func normalizedChatCompletionResponseFormat(options map[string]any) (any, bool) {
	format, ok := normalizedJSONResponseFormat(options)
	if !ok {
		return nil, false
	}
	payload := asMap(format)
	if len(payload) == 0 || strings.TrimSpace(getString(payload["type"])) != "json_schema" {
		return format, true
	}
	if _, ok := payload["json_schema"]; ok {
		return payload, true
	}
	jsonSchema := map[string]any{}
	for _, key := range []string{"name", "description", "schema", "strict"} {
		if value, ok := payload[key]; ok {
			jsonSchema[key] = value
		}
	}
	if len(jsonSchema) == 0 {
		return payload, true
	}
	return map[string]any{
		"type":        "json_schema",
		"json_schema": jsonSchema,
	}, true
}

func buildChatCompletionsMessages(adapter string, msg portllm.Message, promptCache *openAIPromptCacheConfig) []map[string]any {
	result := make([]map[string]any, 0, 1+len(msg.ToolResults))
	if len(msg.ToolResults) > 0 {
		for _, item := range msg.ToolResults {
			result = append(result, map[string]any{
				"role":         "tool",
				"tool_call_id": strings.TrimSpace(item.ToolCallID),
				"content":      buildToolResultContent(item),
			})
		}
		return result
	}

	payload := map[string]any{
		"role":    normalizeRole(msg.Role),
		"content": buildChatCompletionsContent(msg, promptCache),
	}
	if reasoningContent := strings.TrimSpace(msg.ReasoningContent); reasoningContent != "" && normalizeRole(msg.Role) == "assistant" {
		if portllm.NormalizeAdapter(adapter) == portllm.AdapterOpenRouterChat {
			payload["reasoning"] = reasoningContent
		} else {
			payload["reasoning_content"] = reasoningContent
		}
	}
	if len(msg.ToolCalls) > 0 {
		payload["tool_calls"] = buildChatCompletionsToolCalls(msg.ToolCalls)
		if strings.TrimSpace(msg.Content) == "" && len(msg.Parts) == 0 {
			payload["content"] = ""
		}
	}
	result = append(result, payload)
	return result
}

func buildChatCompletionsToolCalls(toolCalls []portllm.ToolCall) []map[string]any {
	items := make([]map[string]any, 0, len(toolCalls))
	for _, item := range toolCalls {
		toolType := strings.TrimSpace(item.ToolType)
		if toolType == "" {
			toolType = "function"
		}
		args := strings.TrimSpace(item.ArgumentsJSON)
		if args == "" {
			args = "{}"
		}
		if toolType == "custom" {
			items = append(items, map[string]any{
				"id":   strings.TrimSpace(item.ToolCallID),
				"type": toolType,
				"custom": map[string]any{
					"name":  strings.TrimSpace(item.ToolName),
					"input": args,
				},
			})
			continue
		}
		items = append(items, map[string]any{
			"id":   strings.TrimSpace(item.ToolCallID),
			"type": toolType,
			"function": map[string]any{
				"name":      strings.TrimSpace(item.ToolName),
				"arguments": args,
			},
		})
	}
	return items
}

// buildChatCompletionsContent 将消息内容序列化为 Chat Completions API 格式。
// 多模态或显式缓存消息返回 parts 数组；其余纯文本消息保持字符串结构。
func buildChatCompletionsContent(msg portllm.Message, promptCache *openAIPromptCacheConfig) any {
	if len(msg.Parts) == 0 {
		if msg.CacheControl != nil && promptCache != nil && promptCache.Explicit {
			block := map[string]any{"type": "text", "text": msg.Content}
			appendOpenAIPromptCacheBreakpoint(block, msg.CacheControl, promptCache)
			return []map[string]any{block}
		}
		return msg.Content
	}
	parts := make([]map[string]any, 0, len(msg.Parts))
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
			b64 := base64.StdEncoding.EncodeToString(part.Data)
			block := map[string]any{
				"type": "image_url",
				"image_url": map[string]string{
					"url": "data:" + mime + ";base64," + b64,
				},
			}
			appendOpenAIPromptCacheBreakpoint(block, part.CacheControl, promptCache)
			parts = append(parts, block)
		default: // text, file — treated as plain text
			text := part.Text
			if strings.TrimSpace(text) == "" {
				continue
			}
			block := map[string]any{
				"type": "text",
				"text": text,
			}
			appendOpenAIPromptCacheBreakpoint(block, part.CacheControl, promptCache)
			parts = append(parts, block)
		}
	}
	if len(parts) == 0 {
		return msg.Content
	}
	if msg.CacheControl != nil {
		for index := len(parts) - 1; index >= 0; index-- {
			if appendOpenAIPromptCacheBreakpoint(parts[index], msg.CacheControl, promptCache) {
				break
			}
		}
	}
	return parts
}

func applyChatStreamEvent(
	adapter string,
	parsed map[string]any,
	result *portllm.GenerateOutput,
	visibleTextBuffer *string,
	onEvent func(portllm.GenerateStreamEvent) error,
	allowTextEncodedToolCalls bool,
) error {
	if responseID := strings.TrimSpace(getString(parsed["id"])); responseID != "" {
		result.ResponseID = responseID
	}

	delta := extractChatStreamDelta(parsed)
	if delta != "" {
		if allowTextEncodedToolCalls {
			if err := bufferChatVisibleDelta(result, visibleTextBuffer, delta, onEvent); err != nil {
				return err
			}
		} else if err := emitChatVisibleDelta(result, delta, onEvent); err != nil {
			return err
		}
	}
	if reasoning := extractChatStreamReasoningDelta(parsed); reasoning != nil && reasoning.Text != "" {
		mergeReasoningDeltaOutput(&result.Reasoning, reasoning)
		if onEvent != nil {
			if err := onEvent(portllm.GenerateStreamEvent{
				Reasoning:  reasoning,
				ResponseID: result.ResponseID,
			}); err != nil {
				return err
			}
		}
	}
	mergeChatStreamToolCalls(parsed, result)
	if serviceTier := strings.TrimSpace(getString(parsed["service_tier"])); serviceTier != "" {
		result.Usage.ServiceTier = serviceTier
	}

	if usage := parseChatStreamUsage(adapter, parsed); usage != (portllm.Usage{}) {
		if usage.ServiceTier == "" {
			usage.ServiceTier = result.Usage.ServiceTier
		}
		result.Usage = usage
		if onEvent != nil {
			if err := onEvent(portllm.GenerateStreamEvent{
				Usage:      usage,
				ResponseID: result.ResponseID,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func mergeChatStreamToolCalls(parsed map[string]any, result *portllm.GenerateOutput) {
	choice := firstMapItem(asSlice(parsed["choices"]))
	delta := asMap(choice["delta"])
	items := asSlice(delta["tool_calls"])
	if len(items) == 0 {
		return
	}
	for fallbackIndex, raw := range items {
		payload := asMap(raw)
		index := streamToolCallIndex(payload["index"], fallbackIndex)
		for len(result.ToolCalls) <= index {
			result.ToolCalls = append(result.ToolCalls, portllm.ToolCall{Status: "requested"})
		}
		current := result.ToolCalls[index]
		if id := strings.TrimSpace(getString(payload["id"])); id != "" {
			current.ToolCallID = id
		}
		if toolType := strings.TrimSpace(getString(payload["type"])); toolType != "" {
			current.ToolType = toolType
		} else if strings.TrimSpace(current.ToolType) == "" {
			current.ToolType = "function"
		}
		function := asMap(payload["function"])
		if name := strings.TrimSpace(getString(function["name"])); name != "" {
			current.ToolName = name
		}
		if argumentsDelta := getString(function["arguments"]); argumentsDelta != "" {
			current.ArgumentsJSON += argumentsDelta
		}
		custom := asMap(payload["custom"])
		if name := strings.TrimSpace(getString(custom["name"])); name != "" {
			current.ToolName = name
		}
		if inputDelta := getString(custom["input"]); inputDelta != "" {
			current.ArgumentsJSON += inputDelta
		}
		result.ToolCalls[index] = current
	}
}

func parseChatCompletionsOutput(adapter string, parsed map[string]any, result *portllm.GenerateOutput, allowTextEncodedToolCalls bool) {
	choice := firstMapItem(asSlice(parsed["choices"]))
	message := asMap(choice["message"])
	result.Text = extractChatVisibleContentText(message["content"])
	result.Reasoning = parseChatReasoningOutput(message)

	result.Usage = parseOpenAICompatibleUsageForAdapter(adapter, parsed)

	toolCalls := parseChatToolCalls(message["tool_calls"])
	if len(toolCalls) > 0 {
		result.ToolCalls = append(result.ToolCalls, toolCalls...)
	}
	if allowTextEncodedToolCalls {
		applyTextEncodedToolCalls(result)
	}
}

func parseChatReasoningOutput(message map[string]any) *portllm.ReasoningOutput {
	if len(message) == 0 {
		return nil
	}
	text := textutil.FirstNonEmpty(
		extractReasoningDeltaText(message["reasoning"]),
		extractReasoningDeltaText(message["reasoning_content"]),
		extractChatReasoningContentText(message["content"]),
	)
	if text == "" {
		return nil
	}
	return &portllm.ReasoningOutput{
		Text: text,
	}
}

func extractChatStreamDelta(parsed map[string]any) string {
	choice := firstMapItem(asSlice(parsed["choices"]))
	delta := asMap(choice["delta"])
	return extractChatVisibleContentText(delta["content"])
}

func extractChatStreamReasoningDelta(parsed map[string]any) *portllm.ReasoningDelta {
	choice := firstMapItem(asSlice(parsed["choices"]))
	delta := asMap(choice["delta"])
	if think := extractReasoningDeltaText(delta["reasoning"]); think != "" {
		return &portllm.ReasoningDelta{
			EventType: "chat.completion.chunk",
			Kind:      "content_text",
			Text:      think,
		}
	}
	if think := extractReasoningDeltaText(delta["reasoning_content"]); think != "" {
		return &portllm.ReasoningDelta{
			EventType: "chat.completion.chunk",
			Kind:      "content_text",
			Text:      think,
		}
	}
	for _, raw := range asSlice(delta["content"]) {
		item := asMap(raw)
		itemType := strings.ToLower(strings.TrimSpace(getString(item["type"])))
		if strings.Contains(itemType, "reason") || strings.Contains(itemType, "think") {
			if think := extractReasoningDeltaText(item); think != "" {
				kind := "content_text"
				if strings.Contains(itemType, "summary") {
					kind = "summary_text"
				}
				return &portllm.ReasoningDelta{
					EventType: "chat.completion.chunk",
					Kind:      kind,
					Text:      think,
				}
			}
		}
	}
	return nil
}

func extractChatVisibleContentText(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case []any:
		chunks := make([]string, 0, len(value))
		for _, item := range value {
			if text := extractChatVisibleContentText(item); text != "" {
				chunks = append(chunks, text)
			}
		}
		return strings.Join(chunks, "")
	case map[string]any:
		if isChatReasoningContentType(value["type"]) {
			return ""
		}
		if text := getString(value["text"]); text != "" {
			return text
		}
		if text := getString(value["output_text"]); text != "" {
			return text
		}
		if text := getString(value["input_text"]); text != "" {
			return text
		}
		return extractChatVisibleContentText(value["content"])
	default:
		return ""
	}
}

func bufferChatVisibleDelta(result *portllm.GenerateOutput, buffer *string, delta string, onEvent func(portllm.GenerateStreamEvent) error) error {
	if result == nil || buffer == nil || delta == "" {
		return nil
	}
	*buffer += delta
	return flushChatVisibleBuffer(result, buffer, onEvent, false)
}

// flushChatVisibleBuffer 在 DeepSeek DSML 模式下延迟释放可见文本，确保完整工具调用不会作为普通文本输出。
func flushChatVisibleBuffer(result *portllm.GenerateOutput, buffer *string, onEvent func(portllm.GenerateStreamEvent) error, final bool) error {
	if result == nil || buffer == nil || *buffer == "" {
		return nil
	}
	if cleanText, toolCalls, ok := parseDSMLToolCalls(*buffer); ok {
		*buffer = ""
		result.ToolCalls = append(result.ToolCalls, toolCalls...)
		if cleanText == "" {
			return nil
		}
		return emitChatVisibleDelta(result, cleanText, onEvent)
	}
	if !final && maybeDSMLToolCallsPrefix(*buffer) {
		return nil
	}
	if final && maybeDSMLToolCallsPrefix(*buffer) {
		return errDeepSeekDSMLToolCallsIncomplete
	}
	text := *buffer
	*buffer = ""
	return emitChatVisibleDelta(result, text, onEvent)
}

// emitChatVisibleDelta 统一写入可见文本并发送流式增量事件。
func emitChatVisibleDelta(result *portllm.GenerateOutput, delta string, onEvent func(portllm.GenerateStreamEvent) error) error {
	if delta == "" {
		return nil
	}
	result.Text += delta
	if onEvent == nil {
		return nil
	}
	return onEvent(portllm.GenerateStreamEvent{
		Delta:      delta,
		ResponseID: result.ResponseID,
	})
}

// maybeDSMLToolCallsPrefix 只识别 DeepSeek DSML tool_calls 的起始片段，用于流式等待更多 chunk。
func maybeDSMLToolCallsPrefix(text string) bool {
	value := strings.ToLower(strings.TrimLeft(strings.TrimSpace(text), "\ufeff"))
	if value == "" {
		return false
	}
	targets := []string{
		"<｜dsml｜tool_calls",
		"<｜｜dsml｜｜tool_calls",
		"<||dsml||tool_calls",
		"<|dsml|tool_calls",
	}
	for _, target := range targets {
		if strings.HasPrefix(target, value) || strings.HasPrefix(value, target) {
			return true
		}
	}
	return false
}

func extractChatReasoningContentText(raw any) string {
	switch value := raw.(type) {
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text := extractChatReasoningContentText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	case map[string]any:
		if isChatReasoningContentType(value["type"]) {
			return extractReasoningDeltaText(value)
		}
		return extractChatReasoningContentText(value["content"])
	default:
		return ""
	}
}

func isChatReasoningContentType(raw any) bool {
	itemType := strings.ToLower(strings.TrimSpace(getString(raw)))
	return strings.Contains(itemType, "reason") || strings.Contains(itemType, "think")
}

func parseChatStreamUsage(adapter string, parsed map[string]any) portllm.Usage {
	if len(asMap(parsed["usage"])) == 0 {
		return portllm.Usage{}
	}
	return parseOpenAICompatibleUsageForAdapter(adapter, parsed)
}

func parseOpenAICompatibleUsageForAdapter(adapter string, parsed map[string]any) portllm.Usage {
	totalInputTokens := firstNonZero(
		getInt64FromPath(parsed, "usage", "input_tokens"),
		getInt64FromPath(parsed, "usage", "prompt_tokens"),
	)
	outputTokens := firstNonZero(
		getInt64FromPath(parsed, "usage", "output_tokens"),
		getInt64FromPath(parsed, "usage", "completion_tokens"),
	)
	reasoningTokens := firstNonZero(
		getInt64FromPath(parsed, "usage", "output_tokens_details", "reasoning_tokens"),
		getInt64FromPath(parsed, "usage", "completion_tokens_details", "reasoning_tokens"),
		getInt64FromPath(parsed, "usage", "reasoning_tokens"),
	)
	// OpenAI reports output/completion tokens as the billable output total,
	// while xAI reports reasoning separately from completion/output tokens.
	visibleTokens := outputTokens
	if openAICompatibleOutputIncludesReasoning(adapter) {
		visibleTokens = visibleOutputTokens(outputTokens, reasoningTokens)
	}
	cacheReadTokens := firstNonZero(
		getInt64FromPath(parsed, "usage", "input_tokens_details", "cached_tokens"),
		getInt64FromPath(parsed, "usage", "prompt_tokens_details", "cached_tokens"),
		getInt64FromPath(parsed, "usage", "input_tokens_details", "cache_read_tokens"),
		getInt64FromPath(parsed, "usage", "prompt_tokens_details", "cache_read_tokens"),
		getInt64FromPath(parsed, "usage", "cache_read_input_tokens"),
		getInt64FromPath(parsed, "usage", "cache_read_tokens"),
	)
	cacheWriteTokens := firstNonZero(
		getInt64FromPath(parsed, "usage", "input_tokens_details", "cache_write_tokens"),
		getInt64FromPath(parsed, "usage", "prompt_tokens_details", "cache_write_tokens"),
		getInt64FromPath(parsed, "usage", "input_tokens_details", "cache_creation_tokens"),
		getInt64FromPath(parsed, "usage", "prompt_tokens_details", "cache_creation_tokens"),
		getInt64FromPath(parsed, "usage", "input_tokens_details", "cached_creation_tokens"),
		getInt64FromPath(parsed, "usage", "prompt_tokens_details", "cached_creation_tokens"),
		getInt64FromPath(parsed, "usage", "input_tokens_details", "cache_creation_input_tokens"),
		getInt64FromPath(parsed, "usage", "prompt_tokens_details", "cache_creation_input_tokens"),
		getInt64FromPath(parsed, "usage", "cache_write_input_tokens"),
		getInt64FromPath(parsed, "usage", "cache_write_tokens"),
		getInt64FromPath(parsed, "usage", "cache_creation_input_tokens"),
		getInt64FromPath(parsed, "usage", "cache_creation", "input_tokens"),
		getInt64FromPath(parsed, "usage", "cache_creation", "ephemeral_1h_input_tokens")+
			getInt64FromPath(parsed, "usage", "cache_creation", "ephemeral_5m_input_tokens"),
	)
	// OpenAI 兼容 usage 的 prompt_tokens/input_tokens 是提示词侧总量，缓存读取与缓存写入都是它的子集
	// （OpenAI 原生、new-api、OpenRouter 均如此）。非缓存输入必须同时扣除两者，否则缓存写入的 token
	// 会先按输入价、再按写入价重复计费。
	return portllm.Usage{
		InputTokens:      nonCachedInputTokens(totalInputTokens, cacheReadTokens+cacheWriteTokens),
		OutputTokens:     visibleTokens,
		CacheReadTokens:  cacheReadTokens,
		CacheWriteTokens: cacheWriteTokens,
		ReasoningTokens:  reasoningTokens,
		ServiceTier:      strings.TrimSpace(getString(parsed["service_tier"])),
		RawUsageJSON:     rawUsageJSONFromPath(parsed, "usage"),
	}
}

func openAICompatibleOutputIncludesReasoning(adapter string) bool {
	switch portllm.NormalizeAdapter(adapter) {
	case portllm.AdapterXAIResponses, portllm.AdapterXAIImage, portllm.AdapterXAIImageEdits:
		return false
	default:
		return true
	}
}

func visibleOutputTokens(outputTokens int64, reasoningTokens int64) int64 {
	if outputTokens <= 0 {
		return 0
	}
	if reasoningTokens <= 0 {
		return outputTokens
	}
	if outputTokens <= reasoningTokens {
		return 0
	}
	return outputTokens - reasoningTokens
}

func parseChatToolCalls(raw any) []portllm.ToolCall {
	items := asSlice(raw)
	result := make([]portllm.ToolCall, 0, len(items))
	for _, item := range items {
		payload := asMap(item)
		function := asMap(payload["function"])
		toolType := strings.TrimSpace(getString(payload["type"]))
		if toolType == "" {
			toolType = "function"
		}
		toolName := strings.TrimSpace(getString(function["name"]))
		arguments := normalizeJSONString(function["arguments"])
		if toolType == "custom" {
			custom := asMap(payload["custom"])
			toolName = strings.TrimSpace(getString(custom["name"]))
			arguments = normalizeJSONString(custom["input"])
		}
		if arguments == "" {
			arguments = "{}"
		}
		result = append(result, portllm.ToolCall{
			ToolCallID:    strings.TrimSpace(getString(payload["id"])),
			ToolType:      toolType,
			ToolName:      toolName,
			ArgumentsJSON: arguments,
			Status:        "requested",
		})
	}
	return result
}
