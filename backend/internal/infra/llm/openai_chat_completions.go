package llm

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
)

// openAIChatCompletionsAdapter 实现 OpenAI Chat Completions API（POST /v1/chat/completions）。
type openAIChatCompletionsAdapter struct {
	client *Client
}

func (a *openAIChatCompletionsAdapter) Name() string { return AdapterOpenAIChatCompletions }

func (a *openAIChatCompletionsAdapter) Generate(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error) {
	route.Endpoint = EndpointChatCompletions
	return a.client.generateOpenAICompatible(ctx, route, input)
}

func (a *openAIChatCompletionsAdapter) GenerateStream(ctx context.Context, route RouteConfig, input GenerateInput, onEvent func(GenerateStreamEvent) error) (*GenerateOutput, error) {
	route.Endpoint = EndpointChatCompletions
	output, err := a.client.generateStreamOpenAICompatible(ctx, route, input, onEvent)
	if err == nil || !shouldRetryChatCompletionsWithoutAutoStreamUsage(input.Options, err) {
		return output, err
	}
	retryInput := input
	retryInput.Options = disableChatCompletionsAutoStreamUsage(input.Options)
	return a.client.generateStreamOpenAICompatible(ctx, route, retryInput, onEvent)
}

func (a *openAIChatCompletionsAdapter) ListModels(ctx context.Context, route RouteConfig) ([]ModelItem, error) {
	return a.client.listModelsOpenAICompatible(ctx, route)
}

func buildChatCompletionsRequestBody(
	model string,
	input GenerateInput,
	messages []Message,
	providerTools []map[string]interface{},
	toolDefinitions []ToolDefinition,
	providerStreamOptions map[string]interface{},
	stream bool,
) map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(messages))
	for _, item := range messages {
		items = append(items, buildChatCompletionsMessages(item)...)
	}
	payload := map[string]interface{}{
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
		payload["thinking"] = map[string]interface{}{
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
	if retention := normalizePromptCacheRetention(modelParamString(input.Options, "prompt_cache_retention")); retention != "" {
		payload["prompt_cache_retention"] = retention
	}
	appendToolDeclarations(payload, providerTools, buildOpenAITools(toolDefinitions, true))
	applyProviderOptions(payload, input.Options,
		"contents", "input", "instructions", "messages", "model", "prompt", "response_format", "stream", "stream_options", "system", "systemInstruction", "tools",
	)
	return payload
}

func chatCompletionsStreamOptions(options map[string]interface{}, stream bool) map[string]interface{} {
	if !stream {
		return nil
	}
	result := map[string]interface{}{"include_usage": true}
	for key, value := range options {
		result[key] = value
	}
	return result
}

func shouldRetryChatCompletionsWithoutAutoStreamUsage(options map[string]interface{}, err error) bool {
	if chatCompletionsStreamUsageExplicit(options) {
		return false
	}
	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) {
		return false
	}
	if upstreamErr.StatusCode != 400 && upstreamErr.StatusCode != 422 {
		return false
	}
	detail := strings.ToLower(strings.TrimSpace(upstreamErr.Message + " " + upstreamErr.Body))
	return strings.Contains(detail, "stream_options") || strings.Contains(detail, "include_usage")
}

func chatCompletionsStreamUsageExplicit(options map[string]interface{}) bool {
	streamOptions, ok := options["stream_options"].(map[string]interface{})
	if !ok {
		return false
	}
	_, ok = streamOptions["include_usage"]
	return ok
}

func disableChatCompletionsAutoStreamUsage(options map[string]interface{}) map[string]interface{} {
	result := cloneMap(options)
	streamOptions := cloneMap(asMap(result["stream_options"]))
	streamOptions["include_usage"] = false
	result["stream_options"] = streamOptions
	return result
}

func normalizedChatCompletionResponseFormat(options map[string]interface{}) (interface{}, bool) {
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
	jsonSchema := map[string]interface{}{}
	for _, key := range []string{"name", "description", "schema", "strict"} {
		if value, ok := payload[key]; ok {
			jsonSchema[key] = value
		}
	}
	if len(jsonSchema) == 0 {
		return payload, true
	}
	return map[string]interface{}{
		"type":        "json_schema",
		"json_schema": jsonSchema,
	}, true
}

func buildChatCompletionsMessages(msg Message) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, 1+len(msg.ToolResults))
	if len(msg.ToolResults) > 0 {
		for _, item := range msg.ToolResults {
			result = append(result, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": strings.TrimSpace(item.ToolCallID),
				"content":      buildToolResultContent(item),
			})
		}
		return result
	}

	payload := map[string]interface{}{
		"role":    normalizeRole(msg.Role),
		"content": buildChatCompletionsContent(msg),
	}
	if reasoningContent := strings.TrimSpace(msg.ReasoningContent); reasoningContent != "" && normalizeRole(msg.Role) == "assistant" {
		payload["reasoning_content"] = reasoningContent
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

func buildChatCompletionsToolCalls(toolCalls []ToolCall) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(toolCalls))
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
			items = append(items, map[string]interface{}{
				"id":   strings.TrimSpace(item.ToolCallID),
				"type": toolType,
				"custom": map[string]interface{}{
					"name":  strings.TrimSpace(item.ToolName),
					"input": args,
				},
			})
			continue
		}
		items = append(items, map[string]interface{}{
			"id":   strings.TrimSpace(item.ToolCallID),
			"type": toolType,
			"function": map[string]interface{}{
				"name":      strings.TrimSpace(item.ToolName),
				"arguments": args,
			},
		})
	}
	return items
}

// buildChatCompletionsContent 将消息内容序列化为 Chat Completions API 格式。
// 多模态消息返回 parts 数组；纯文本消息保持字符串结构，避免无意义包装。
func buildChatCompletionsContent(msg Message) interface{} {
	if len(msg.Parts) == 0 {
		return msg.Content
	}
	parts := make([]map[string]interface{}, 0, len(msg.Parts))
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
			b64 := base64.StdEncoding.EncodeToString(part.Data)
			parts = append(parts, map[string]interface{}{
				"type": "image_url",
				"image_url": map[string]string{
					"url": "data:" + mime + ";base64," + b64,
				},
			})
		default: // text, file — treated as plain text
			text := part.Text
			if strings.TrimSpace(text) == "" {
				continue
			}
			parts = append(parts, map[string]interface{}{
				"type": "text",
				"text": text,
			})
		}
	}
	if len(parts) == 0 {
		return msg.Content
	}
	return parts
}

func applyChatStreamEvent(
	adapter string,
	parsed map[string]interface{},
	result *GenerateOutput,
	onEvent func(GenerateStreamEvent) error,
) error {
	if responseID := strings.TrimSpace(getString(parsed["id"])); responseID != "" {
		result.ResponseID = responseID
	}

	delta := extractChatStreamDelta(parsed)
	if delta != "" {
		result.Text += delta
		if onEvent != nil {
			if err := onEvent(GenerateStreamEvent{
				Delta:      delta,
				ResponseID: result.ResponseID,
			}); err != nil {
				return err
			}
		}
	}
	if reasoning := extractChatStreamReasoningDelta(parsed); reasoning != nil && reasoning.Text != "" {
		mergeReasoningDeltaOutput(&result.Reasoning, reasoning)
		if onEvent != nil {
			if err := onEvent(GenerateStreamEvent{
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

	if usage := parseChatStreamUsage(adapter, parsed); usage != (Usage{}) {
		if usage.ServiceTier == "" {
			usage.ServiceTier = result.Usage.ServiceTier
		}
		result.Usage = usage
		if onEvent != nil {
			if err := onEvent(GenerateStreamEvent{
				Usage:      usage,
				ResponseID: result.ResponseID,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func mergeChatStreamToolCalls(parsed map[string]interface{}, result *GenerateOutput) {
	choice := firstMapItem(asSlice(parsed["choices"]))
	delta := asMap(choice["delta"])
	items := asSlice(delta["tool_calls"])
	if len(items) == 0 {
		return
	}
	for fallbackIndex, raw := range items {
		payload := asMap(raw)
		index := int(toInt64(payload["index"]))
		if index < 0 {
			index = fallbackIndex
		}
		for len(result.ToolCalls) <= index {
			result.ToolCalls = append(result.ToolCalls, ToolCall{Status: "requested"})
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

func parseChatCompletionsOutput(adapter string, parsed map[string]interface{}, result *GenerateOutput) {
	choice := firstMapItem(asSlice(parsed["choices"]))
	message := asMap(choice["message"])
	result.Text = extractChatVisibleContentText(message["content"])
	result.Reasoning = parseChatReasoningOutput(message)

	result.Usage = parseOpenAICompatibleUsageForAdapter(adapter, parsed)

	toolCalls := parseChatToolCalls(message["tool_calls"])
	if len(toolCalls) > 0 {
		result.ToolCalls = append(result.ToolCalls, toolCalls...)
	}
}

func parseChatReasoningOutput(message map[string]interface{}) *ReasoningOutput {
	if len(message) == 0 {
		return nil
	}
	text := firstNonEmptyString(
		extractReasoningDeltaText(message["reasoning"]),
		extractReasoningDeltaText(message["reasoning_content"]),
		extractChatReasoningContentText(message["content"]),
	)
	if text == "" {
		return nil
	}
	return &ReasoningOutput{
		Text: text,
	}
}

func extractChatStreamDelta(parsed map[string]interface{}) string {
	choice := firstMapItem(asSlice(parsed["choices"]))
	delta := asMap(choice["delta"])
	return extractChatVisibleContentText(delta["content"])
}

func extractChatStreamReasoningDelta(parsed map[string]interface{}) *ReasoningDelta {
	choice := firstMapItem(asSlice(parsed["choices"]))
	delta := asMap(choice["delta"])
	if think := extractReasoningDeltaText(delta["reasoning"]); think != "" {
		return &ReasoningDelta{
			EventType: "chat.completion.chunk",
			Kind:      "content_text",
			Text:      think,
		}
	}
	if think := extractReasoningDeltaText(delta["reasoning_content"]); think != "" {
		return &ReasoningDelta{
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
				return &ReasoningDelta{
					EventType: "chat.completion.chunk",
					Kind:      kind,
					Text:      think,
				}
			}
		}
	}
	return nil
}

func extractChatVisibleContentText(raw interface{}) string {
	switch value := raw.(type) {
	case string:
		return value
	case []interface{}:
		chunks := make([]string, 0, len(value))
		for _, item := range value {
			if text := extractChatVisibleContentText(item); text != "" {
				chunks = append(chunks, text)
			}
		}
		return strings.Join(chunks, "")
	case map[string]interface{}:
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

func extractChatReasoningContentText(raw interface{}) string {
	switch value := raw.(type) {
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text := extractChatReasoningContentText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	case map[string]interface{}:
		if isChatReasoningContentType(value["type"]) {
			return extractReasoningDeltaText(value)
		}
		return extractChatReasoningContentText(value["content"])
	default:
		return ""
	}
}

func isChatReasoningContentType(raw interface{}) bool {
	itemType := strings.ToLower(strings.TrimSpace(getString(raw)))
	return strings.Contains(itemType, "reason") || strings.Contains(itemType, "think")
}

func parseChatStreamUsage(adapter string, parsed map[string]interface{}) Usage {
	if len(asMap(parsed["usage"])) == 0 {
		return Usage{}
	}
	return parseOpenAICompatibleUsageForAdapter(adapter, parsed)
}

func parseOpenAICompatibleUsage(parsed map[string]interface{}) Usage {
	return parseOpenAICompatibleUsageForAdapter(AdapterOpenAIResponses, parsed)
}

func parseOpenAICompatibleUsageForAdapter(adapter string, parsed map[string]interface{}) Usage {
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
	return Usage{
		InputTokens:     nonCachedInputTokens(totalInputTokens, cacheReadTokens),
		OutputTokens:    visibleTokens,
		CacheReadTokens: cacheReadTokens,
		CacheWriteTokens: firstNonZero(
			getInt64FromPath(parsed, "usage", "input_tokens_details", "cache_write_tokens"),
			getInt64FromPath(parsed, "usage", "prompt_tokens_details", "cache_write_tokens"),
			getInt64FromPath(parsed, "usage", "input_tokens_details", "cache_creation_tokens"),
			getInt64FromPath(parsed, "usage", "prompt_tokens_details", "cache_creation_tokens"),
			getInt64FromPath(parsed, "usage", "input_tokens_details", "cache_creation_input_tokens"),
			getInt64FromPath(parsed, "usage", "prompt_tokens_details", "cache_creation_input_tokens"),
			getInt64FromPath(parsed, "usage", "cache_write_input_tokens"),
			getInt64FromPath(parsed, "usage", "cache_write_tokens"),
			getInt64FromPath(parsed, "usage", "cache_creation_input_tokens"),
			getInt64FromPath(parsed, "usage", "cache_creation", "input_tokens"),
			getInt64FromPath(parsed, "usage", "cache_creation", "ephemeral_1h_input_tokens")+
				getInt64FromPath(parsed, "usage", "cache_creation", "ephemeral_5m_input_tokens"),
		),
		ReasoningTokens: reasoningTokens,
		ServiceTier:     strings.TrimSpace(getString(parsed["service_tier"])),
		RawUsageJSON:    rawUsageJSONFromPath(parsed, "usage"),
	}
}

func openAICompatibleOutputIncludesReasoning(adapter string) bool {
	switch NormalizeAdapter(adapter) {
	case AdapterXAIResponses, AdapterXAIImage, AdapterXAIImageEdits:
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

func parseChatToolCalls(raw interface{}) []ToolCall {
	items := asSlice(raw)
	result := make([]ToolCall, 0, len(items))
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
		result = append(result, ToolCall{
			ToolCallID:    strings.TrimSpace(getString(payload["id"])),
			ToolType:      toolType,
			ToolName:      toolName,
			ArgumentsJSON: arguments,
			Status:        "requested",
		})
	}
	return result
}
