package llm

import (
	"context"
	"encoding/base64"
	"strings"
)

// openAIResponsesAdapter 实现 OpenAI Responses API（POST /v1/responses）。
type openAIResponsesAdapter struct {
	client *Client
}

func (a *openAIResponsesAdapter) Name() string { return AdapterOpenAIResponses }

func (a *openAIResponsesAdapter) Generate(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error) {
	route.Endpoint = EndpointResponses
	return a.client.generateOpenAICompatible(ctx, route, input)
}

func (a *openAIResponsesAdapter) GenerateStream(ctx context.Context, route RouteConfig, input GenerateInput, onEvent func(GenerateStreamEvent) error) (*GenerateOutput, error) {
	route.Endpoint = EndpointResponses
	return a.client.generateStreamOpenAICompatible(ctx, route, input, onEvent)
}

func (a *openAIResponsesAdapter) ListModels(ctx context.Context, route RouteConfig) ([]ModelItem, error) {
	return a.client.listModelsOpenAICompatible(ctx, route)
}

func buildResponsesRequestBody(
	adapter string,
	model string,
	input GenerateInput,
	messages []Message,
	providerTools []map[string]interface{},
	toolDefinitions []ToolDefinition,
	toolsEnabled bool,
	providerStreamOptions map[string]interface{},
	stream bool,
) map[string]interface{} {
	items := buildResponsesAPIInput(messages)
	payload := map[string]interface{}{
		"model":  strings.TrimSpace(model),
		"input":  items,
		"stream": stream,
	}
	if maxTokens := modelParamInt(input.Options, "max_output_tokens"); maxTokens > 0 {
		payload["max_output_tokens"] = maxTokens
	}
	applyOpenAICompatibleSamplingParams(payload, input.Options, false)
	applyOpenAIResponsesReasoningParams(payload, input.Options)
	applyOpenAIResponsesTextParams(payload, input.Options, adapter == AdapterOpenAIResponses)
	webSearchTools := []map[string]interface{}{}
	if toolsEnabled && modelParamBool(input.Options, "web_search") && adapter == AdapterOpenAIResponses {
		webSearchTools = append(webSearchTools, map[string]interface{}{"type": "web_search"})
	}
	nativeTools := append([]map[string]interface{}{}, providerTools...)
	nativeTools = append(nativeTools, webSearchTools...)
	if retention := normalizePromptCacheRetention(modelParamString(input.Options, "prompt_cache_retention")); retention != "" {
		payload["prompt_cache_retention"] = retention
	}
	appendToolDeclarations(payload, providerTools, webSearchTools, buildOpenAITools(toolDefinitions, false))
	// 有状态会话：提供 previous_response_id 时服务端续接存储的历史，
	// input 仅包含本轮新消息，避免全量重传。
	if prevID := strings.TrimSpace(input.PreviousResponseID); prevID != "" {
		payload["previous_response_id"] = prevID
	}
	if streamOptions := responsesStreamOptions(providerStreamOptions); stream && len(streamOptions) > 0 {
		payload["stream_options"] = streamOptions
	}
	applyProviderOptions(payload, input.Options, responsesProtectedProviderOptionKeys(adapter)...)
	if supportsResponsesIncludeDefaults(adapter) {
		defaultIncludes := responsesDefaultIncludeValues(adapter, stream, nativeTools)
		appendResponseInclude(payload, responseIncludeValues(input.Options, defaultIncludes...)...)
	} else {
		appendResponseInclude(payload, responseIncludeValues(input.Options)...)
	}
	return payload
}

func responsesProtectedProviderOptionKeys(adapter string) []string {
	keys := []string{
		"contents",
		"include",
		"input",
		"messages",
		"model",
		"previous_response_id",
		"reasoning",
		"response_format",
		"stream",
		"stream_options",
		"system",
		"systemInstruction",
		"text",
		"tools",
	}
	if adapter != AdapterOpenAIResponses {
		keys = append(keys, "instructions", "metadata", "prompt")
	}
	return keys
}

func responsesStreamOptions(options map[string]interface{}) map[string]interface{} {
	if len(options) == 0 {
		return nil
	}
	result := map[string]interface{}{}
	if value, ok := options["include_obfuscation"]; ok {
		result["include_obfuscation"] = value
	}
	return result
}

func applyOpenAIResponsesReasoningParams(payload map[string]interface{}, options map[string]interface{}) {
	reasoning := map[string]interface{}{}
	if existing := modelParamMap(options, "reasoning"); len(existing) > 0 {
		for key, value := range existing {
			reasoning[key] = value
		}
	}
	if effort := modelParamString(options, "reasoning_effort"); effort != "" {
		reasoning["effort"] = effort
	}
	if summary := modelParamString(options, "reasoning_summary"); summary != "" {
		reasoning["summary"] = summary
	}
	mergeObjectParam(payload, "reasoning", reasoning)
}

func applyOpenAIResponsesTextParams(payload map[string]interface{}, options map[string]interface{}, allowVerbosity bool) {
	text := map[string]interface{}{}
	if existing := modelParamMap(options, "text"); len(existing) > 0 {
		for key, value := range existing {
			text[key] = value
		}
	}
	if format, ok := normalizedJSONResponseFormat(options); ok {
		text["format"] = format
	}
	if verbosity := modelParamString(options, "verbosity"); verbosity != "" && allowVerbosity {
		text["verbosity"] = verbosity
	}
	mergeObjectParam(payload, "text", text)
}

type responsesProtocolExtension struct {
	matchesAdapter                  func(adapter string) bool
	includeDefaults                 func(stream bool, tools []map[string]interface{}) []string
	serverToolIdentifierKeys        func() []string
	serverToolCallID                func(item map[string]interface{}, itemType string) (string, bool)
	isServerToolCallItem            func(item map[string]interface{}) bool
	isServerToolCallType            func(itemType string) bool
	normalizeServerSideToolUsageKey func(value string, original string) (string, bool)
}

func supportsResponsesIncludeDefaults(adapter string) bool {
	return adapter == AdapterOpenAIResponses || len(responsesProtocolExtensionsForAdapter(adapter)) > 0
}

func responsesDefaultIncludeValues(adapter string, stream bool, providerTools []map[string]interface{}) []string {
	values := []string{"reasoning.encrypted_content"}
	if adapter == AdapterOpenAIResponses {
		values = append(values, openAIResponsesDefaultIncludeValues(providerTools)...)
	}
	for _, extension := range responsesProtocolExtensionsForAdapter(adapter) {
		if extension.includeDefaults != nil {
			values = append(values, extension.includeDefaults(stream, providerTools)...)
		}
	}
	return appendUniqueStrings(nil, values...)
}

func openAIResponsesDefaultIncludeValues(tools []map[string]interface{}) []string {
	if !responsesToolsIncludeType(tools, "web_search") {
		return nil
	}
	return []string{"web_search_call.action.sources"}
}

func responsesToolsIncludeType(tools []map[string]interface{}, toolType string) bool {
	expected := strings.TrimSpace(toolType)
	if expected == "" {
		return false
	}
	for _, tool := range tools {
		if strings.TrimSpace(getString(tool["type"])) == expected {
			return true
		}
	}
	return false
}

func buildResponsesAPIInput(messages []Message) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		if len(msg.ToolCalls) > 0 {
			for _, item := range msg.ToolCalls {
				args := strings.TrimSpace(item.ArgumentsJSON)
				if args == "" {
					args = "{}"
				}
				items = append(items, map[string]interface{}{
					"type":      "function_call",
					"call_id":   strings.TrimSpace(item.ToolCallID),
					"name":      strings.TrimSpace(item.ToolName),
					"arguments": args,
				})
			}
			continue
		}
		if len(msg.ToolResults) > 0 {
			for _, item := range msg.ToolResults {
				items = append(items, map[string]interface{}{
					"type":    "function_call_output",
					"call_id": strings.TrimSpace(item.ToolCallID),
					"output":  buildToolResultContent(item),
				})
			}
			continue
		}
		items = append(items, map[string]interface{}{
			"role":    normalizeRole(msg.Role),
			"content": buildResponsesAPIContent(msg),
		})
	}
	return items
}

// buildResponsesAPIContent 将消息内容序列化为 Responses API 格式（content 数组）。
func buildResponsesAPIContent(msg Message) []map[string]interface{} {
	textType := responsesTextContentType(msg.Role)
	if len(msg.Parts) == 0 {
		return []map[string]interface{}{
			{"type": textType, "text": msg.Content},
		}
	}
	parts := make([]map[string]interface{}, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		switch part.Kind {
		case ContentPartImage:
			if normalizeRole(msg.Role) == "assistant" {
				continue
			}
			if len(part.Data) == 0 {
				continue
			}
			mime := strings.TrimSpace(part.MimeType)
			if mime == "" {
				mime = "image/jpeg"
			}
			b64 := base64.StdEncoding.EncodeToString(part.Data)
			parts = append(parts, map[string]interface{}{
				"type":      "input_image",
				"image_url": "data:" + mime + ";base64," + b64,
			})
		default: // text, file
			text := part.Text
			if strings.TrimSpace(text) == "" {
				continue
			}
			parts = append(parts, map[string]interface{}{
				"type": textType,
				"text": text,
			})
		}
	}
	if len(parts) == 0 {
		return []map[string]interface{}{
			{"type": textType, "text": msg.Content},
		}
	}
	return parts
}

func responsesTextContentType(role string) string {
	if normalizeRole(role) == "assistant" {
		return "output_text"
	}
	return "input_text"
}

func applyResponsesStreamEvent(
	adapter string,
	eventName string,
	parsed map[string]interface{},
	rawBody string,
	result *GenerateOutput,
	onEvent func(GenerateStreamEvent) error,
) error {
	eventType := strings.TrimSpace(getString(parsed["type"]))
	if eventType == "" {
		eventType = strings.TrimSpace(eventName)
	}

	if call, ok := parseResponsesServerToolStatusEvent(eventType, parsed); ok {
		appendUniqueToolCall(&result.ServerToolCalls, call)
		if onEvent != nil {
			return onEvent(GenerateStreamEvent{
				ServerToolCall: &call,
				ResponseID:     result.ResponseID,
			})
		}
		return nil
	}

	switch eventType {
	case "response.created":
		if responseID := strings.TrimSpace(getStringFromPath(parsed, "response", "id")); responseID != "" {
			result.ResponseID = responseID
		}
		if serviceTier := strings.TrimSpace(getStringFromPath(parsed, "response", "service_tier")); serviceTier != "" {
			result.Usage.ServiceTier = serviceTier
		}
	case "response.output_item.added", "response.output_item.in_progress":
		return mergeResponsesStreamOutputItem(result, asMap(parsed["item"]), onEvent)
	case "response.output_item.done":
		return mergeResponsesStreamOutputItem(result, asMap(parsed["item"]), onEvent)
	case "response.custom_tool_call_input.delta", "response.custom_tool_call_input.done":
		return mergeResponsesCustomToolInputEvent(result, parsed, onEvent)
	case "response.output_text.delta":
		delta := getString(parsed["delta"])
		if delta == "" {
			return nil
		}
		result.Text += delta
		if onEvent != nil {
			return onEvent(GenerateStreamEvent{
				Delta:      delta,
				ResponseID: result.ResponseID,
			})
		}
	case "response.output_text.done":
		text := firstNonEmptyString(getString(parsed["text"]), getString(parsed["delta"]))
		if text != "" && !strings.Contains(result.Text, text) {
			result.Text += text
		}
	case "response.refusal.delta":
		delta := getString(parsed["delta"])
		if delta == "" {
			return nil
		}
		result.Text += delta
		if onEvent != nil {
			return onEvent(GenerateStreamEvent{
				Delta:      delta,
				ResponseID: result.ResponseID,
			})
		}
	case "response.refusal.done":
		text := firstNonEmptyString(getString(parsed["refusal"]), getString(parsed["text"]), getString(parsed["delta"]))
		if text != "" && !strings.Contains(result.Text, text) {
			result.Text += text
		}
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta", "response.thinking.delta":
		reasoning := parseResponsesReasoningDelta(eventType, parsed)
		if reasoning == nil || reasoning.Text == "" {
			return nil
		}
		mergeReasoningDeltaOutput(&result.Reasoning, reasoning)
		if onEvent != nil {
			return onEvent(GenerateStreamEvent{
				Reasoning:  reasoning,
				ResponseID: result.ResponseID,
			})
		}
	case "response.reasoning_summary_text.done", "response.reasoning_text.done", "response.thinking.done":
		reasoning := parseResponsesReasoningDone(eventType, parsed)
		if reasoning == nil || reasoning.Text == "" {
			return nil
		}
		if result.Reasoning == nil || !reasoningOutputContains(result.Reasoning, reasoning.Text) {
			mergeReasoningDeltaOutput(&result.Reasoning, reasoning)
		}
	case "response.completed":
		output := buildGenerateOutputFromParsedForAdapter(EndpointResponses, adapter, asMap(parsed["response"]))
		if result.Reasoning == nil && output.Reasoning != nil && onEvent != nil {
			if text := firstNonEmptyString(output.Reasoning.Text, output.Reasoning.Summary); text != "" {
				if err := onEvent(GenerateStreamEvent{
					Reasoning: &ReasoningDelta{
						EventType:        eventType,
						ItemID:           output.Reasoning.ItemID,
						Status:           output.Reasoning.Status,
						Kind:             "content_text",
						Text:             text,
						EncryptedContent: output.Reasoning.EncryptedContent,
					},
					ResponseID: firstNonEmptyString(result.ResponseID, output.ResponseID),
				}); err != nil {
					return err
				}
			}
		}
		mergeGenerateOutput(result, output)
		if output.Usage != (Usage{}) && onEvent != nil {
			return onEvent(GenerateStreamEvent{
				Usage:      output.Usage,
				ResponseID: result.ResponseID,
			})
		}
	case "response.failed", "response.error":
		return parseResponsesStreamErrorEvent(parsed, rawBody)
	}

	return nil
}

func parseResponsesServerToolStatusEvent(eventType string, parsed map[string]interface{}) (ToolCall, bool) {
	value := strings.TrimSpace(eventType)
	if !strings.HasPrefix(value, "response.") {
		return ToolCall{}, false
	}
	status := ""
	for _, suffix := range []string{".in_progress", ".searching", ".completed", ".failed", ".error"} {
		if strings.HasSuffix(value, suffix) {
			status = strings.TrimPrefix(suffix, ".")
			value = strings.TrimSuffix(strings.TrimPrefix(value, "response."), suffix)
			break
		}
	}
	if status == "" || !isResponsesServerToolCallType(value) {
		return ToolCall{}, false
	}
	item := cloneMap(asMap(parsed["item"]))
	if len(item) == 0 {
		item = make(map[string]interface{})
	}
	mergeMapValueIfEmpty(item, "type", value)
	mergeMapValueIfEmpty(item, "status", status)
	for _, key := range responseServerToolIdentifierKeys() {
		mergeMapValueIfEmpty(item, key, parsed[key])
	}
	for _, key := range []string{"action", "arguments", "input", "query", "output", "outputs", "results", "search_results", "sources", "citations", "data", "items", "content", "response", "result", "error"} {
		mergeMapValueIfEmpty(item, key, parsed[key])
	}
	return parseResponseServerToolCall(item), true
}

func responseServerToolIdentifierKeys() []string {
	keys := []string{"item_id", "id", "call_id", "tool_call_id"}
	for _, extension := range allResponsesProtocolExtensions() {
		if extension.serverToolIdentifierKeys != nil {
			keys = append(keys, extension.serverToolIdentifierKeys()...)
		}
	}
	return keys
}

func mergeResponsesStreamOutputItem(
	result *GenerateOutput,
	item map[string]interface{},
	onEvent func(GenerateStreamEvent) error,
) error {
	if result == nil || len(item) == 0 {
		return nil
	}
	if !isResponsesServerToolCallItem(item) {
		mergeResponsesOutputItem(result, item, false)
		return nil
	}
	call := parseResponseServerToolCall(item)
	appendUniqueToolCall(&result.ServerToolCalls, call)
	result.Citations = appendUniqueStrings(result.Citations, parseResponseCitations(item)...)
	if onEvent == nil {
		return nil
	}
	return onEvent(GenerateStreamEvent{
		ServerToolCall: &call,
		ResponseID:     result.ResponseID,
	})
}

func mergeResponsesCustomToolInputEvent(
	result *GenerateOutput,
	parsed map[string]interface{},
	onEvent func(GenerateStreamEvent) error,
) error {
	if result == nil {
		return nil
	}
	itemID := firstNonEmptyString(getString(parsed["item_id"]), getString(parsed["call_id"]), getString(parsed["id"]))
	if itemID == "" {
		return nil
	}
	input := firstNonEmptyString(getString(parsed["delta"]), getString(parsed["input"]))
	done := strings.HasSuffix(strings.TrimSpace(getString(parsed["type"])), ".done")
	if call, ok := updateToolCallInput(&result.ServerToolCalls, itemID, input, done); ok {
		if onEvent != nil {
			return onEvent(GenerateStreamEvent{
				ServerToolCall: &call,
				ResponseID:     result.ResponseID,
			})
		}
		return nil
	}
	updateToolCallInput(&result.ToolCalls, itemID, input, done)
	return nil
}

func parseResponsesStreamErrorEvent(parsed map[string]interface{}, rawBody string) error {
	errorPayload := asMap(parsed["error"])
	if len(errorPayload) == 0 {
		errorPayload = asMap(asMap(parsed["response"])["error"])
	}
	statusCode := streamErrorStatusCode(parsed, errorPayload)
	message := firstNonEmptyString(
		getString(errorPayload["message"]),
		getString(errorPayload["msg"]),
		getString(parsed["message"]),
		"responses stream returned an error event",
	)
	return &UpstreamError{
		StatusCode: statusCode,
		Message:    message,
		Body:       rawBody,
	}
}

func parseResponsesOutput(adapter string, parsed map[string]interface{}, result *GenerateOutput) {
	result.Text = getString(parsed["output_text"])
	outputItems := asSlice(parsed["output"])
	textChunks := make([]string, 0, len(outputItems))

	for _, raw := range outputItems {
		item := asMap(raw)
		if chunk := mergeResponsesOutputItem(result, item, true); chunk != "" {
			textChunks = append(textChunks, chunk)
		}
	}

	if result.Text == "" && len(textChunks) > 0 {
		result.Text = strings.Join(textChunks, "")
	}

	mergeResponsesTopLevelToolCalls(result, parsed["tool_calls"])

	result.Usage = parseOpenAICompatibleUsageForAdapter(adapter, parsed)
	result.ServerSideToolUsage = parseServerSideToolUsage(parsed)
	result.Citations = appendUniqueStrings(result.Citations, parseResponseCitations(parsed)...)
}

func mergeResponsesTopLevelToolCalls(result *GenerateOutput, raw interface{}) {
	if result == nil {
		return
	}
	for _, value := range asSlice(raw) {
		item := asMap(value)
		if len(item) == 0 {
			continue
		}
		if isResponsesServerToolCallItem(item) {
			appendUniqueToolCall(&result.ServerToolCalls, parseResponseServerToolCall(item))
			result.Citations = appendUniqueStrings(result.Citations, parseResponseCitations(item)...)
			continue
		}
		itemType := strings.TrimSpace(getString(item["type"]))
		if isResponsesClientToolCallType(itemType) {
			appendUniqueToolCall(&result.ToolCalls, parseResponseToolCall(item))
		}
	}
}

func mergeReasoningDeltaOutput(dst **ReasoningOutput, delta *ReasoningDelta) {
	if delta == nil {
		return
	}
	if *dst == nil {
		*dst = &ReasoningOutput{}
	}
	if strings.TrimSpace(delta.ItemID) != "" {
		(*dst).ItemID = strings.TrimSpace(delta.ItemID)
	}
	if strings.TrimSpace(delta.Status) != "" {
		(*dst).Status = strings.TrimSpace(delta.Status)
	}
	switch delta.Kind {
	case "summary_text":
		(*dst).Summary += delta.Text
	default:
		(*dst).Text += delta.Text
	}
	if strings.TrimSpace(delta.Signature) != "" {
		(*dst).Signature = strings.TrimSpace(delta.Signature)
	}
	if strings.TrimSpace(delta.EncryptedContent) != "" {
		(*dst).EncryptedContent = strings.TrimSpace(delta.EncryptedContent)
	}
}

func parseResponsesReasoningDelta(eventType string, parsed map[string]interface{}) *ReasoningDelta {
	text := extractReasoningDeltaText(parsed["delta"])
	if text == "" {
		return nil
	}

	kind := "content_text"
	if strings.Contains(eventType, "summary") {
		kind = "summary_text"
	}

	return &ReasoningDelta{
		EventType:        eventType,
		ItemID:           firstNonEmptyString(getString(parsed["item_id"]), getStringFromPath(parsed, "item", "id")),
		Status:           firstNonEmptyString(getString(parsed["status"]), getStringFromPath(parsed, "item", "status")),
		Kind:             kind,
		Text:             text,
		EncryptedContent: firstNonEmptyString(getString(parsed["encrypted_content"]), getStringFromPath(parsed, "item", "encrypted_content")),
	}
}

func parseResponsesReasoningDone(eventType string, parsed map[string]interface{}) *ReasoningDelta {
	text := firstNonEmptyString(
		extractReasoningDeltaText(parsed["text"]),
		extractReasoningDeltaText(parsed["summary"]),
		extractReasoningDeltaText(parsed["delta"]),
	)
	if text == "" {
		return nil
	}

	kind := "content_text"
	if strings.Contains(eventType, "summary") {
		kind = "summary_text"
	}

	return &ReasoningDelta{
		EventType:        eventType,
		ItemID:           firstNonEmptyString(getString(parsed["item_id"]), getStringFromPath(parsed, "item", "id")),
		Status:           firstNonEmptyString(getString(parsed["status"]), getStringFromPath(parsed, "item", "status")),
		Kind:             kind,
		Text:             text,
		EncryptedContent: firstNonEmptyString(getString(parsed["encrypted_content"]), getStringFromPath(parsed, "item", "encrypted_content")),
	}
}

func reasoningOutputContains(output *ReasoningOutput, text string) bool {
	if output == nil || strings.TrimSpace(text) == "" {
		return false
	}
	return strings.Contains(output.Text, text) || strings.Contains(output.Summary, text)
}

func parseReasoningOutputItem(item map[string]interface{}) *ReasoningOutput {
	if len(item) == 0 {
		return nil
	}

	summaryParts := make([]string, 0)
	for _, raw := range asSlice(item["summary"]) {
		if text := extractReasoningDeltaText(raw); text != "" {
			summaryParts = append(summaryParts, text)
		}
	}

	contentParts := make([]string, 0)
	for _, raw := range asSlice(item["content"]) {
		if text := extractReasoningDeltaText(raw); text != "" {
			contentParts = append(contentParts, text)
		}
	}

	text := strings.Join(contentParts, "")
	if text == "" {
		text = extractReasoningDeltaText(item["text"])
	}

	result := &ReasoningOutput{
		ItemID:           getString(item["id"]),
		Status:           getString(item["status"]),
		Summary:          strings.Join(summaryParts, ""),
		Text:             text,
		EncryptedContent: getString(item["encrypted_content"]),
	}
	if strings.TrimSpace(result.ItemID) == "" &&
		strings.TrimSpace(result.Status) == "" &&
		strings.TrimSpace(result.Summary) == "" &&
		strings.TrimSpace(result.Text) == "" &&
		strings.TrimSpace(result.EncryptedContent) == "" {
		return nil
	}
	return result
}

func mergeReasoningOutput(dst **ReasoningOutput, src *ReasoningOutput) {
	if src == nil {
		return
	}
	if *dst == nil {
		value := *src
		*dst = &value
		return
	}
	if strings.TrimSpace(src.ItemID) != "" {
		(*dst).ItemID = strings.TrimSpace(src.ItemID)
	}
	if strings.TrimSpace(src.Status) != "" {
		(*dst).Status = strings.TrimSpace(src.Status)
	}
	if strings.TrimSpace(src.Summary) != "" {
		(*dst).Summary = strings.TrimSpace(src.Summary)
	}
	if strings.TrimSpace(src.Text) != "" {
		(*dst).Text = strings.TrimSpace(src.Text)
	}
	if strings.TrimSpace(src.Signature) != "" {
		(*dst).Signature = strings.TrimSpace(src.Signature)
	}
	if strings.TrimSpace(src.EncryptedContent) != "" {
		(*dst).EncryptedContent = strings.TrimSpace(src.EncryptedContent)
	}
}

func mergeResponsesOutputItem(result *GenerateOutput, item map[string]interface{}, collectText bool) string {
	if result == nil || len(item) == 0 {
		return ""
	}
	itemType := strings.TrimSpace(getString(item["type"]))
	switch {
	case itemType == "reasoning":
		mergeReasoningOutput(&result.Reasoning, parseReasoningOutputItem(item))
	case isResponsesServerToolCallItem(item):
		appendUniqueToolCall(&result.ServerToolCalls, parseResponseServerToolCall(item))
		result.Citations = appendUniqueStrings(result.Citations, parseResponseCitations(item)...)
	case isResponsesClientToolCallType(itemType):
		appendUniqueToolCall(&result.ToolCalls, parseResponseToolCall(item))
	default:
		result.Citations = appendUniqueStrings(result.Citations, parseResponseCitations(item)...)
		if collectText {
			return extractOutputTextChunk(item)
		}
	}
	return ""
}

func parseResponseToolCall(item map[string]interface{}) ToolCall {
	arguments := normalizeJSONString(item["arguments"])
	if arguments == "" {
		arguments = normalizeJSONString(item["input"])
	}
	if arguments == "" {
		arguments = "{}"
	}

	toolCallID := strings.TrimSpace(getString(item["call_id"]))
	if toolCallID == "" {
		toolCallID = strings.TrimSpace(getString(item["id"]))
	}

	toolName := strings.TrimSpace(getString(item["name"]))
	if toolName == "" {
		toolName = strings.TrimSpace(getStringFromPath(item, "function", "name"))
	}

	toolType := strings.TrimSpace(getString(item["type"]))
	if toolType == "" {
		toolType = "function"
	}

	status := strings.TrimSpace(getString(item["status"]))
	if status == "" {
		status = "requested"
	}

	return ToolCall{
		ToolCallID:    toolCallID,
		ToolType:      toolType,
		ToolName:      toolName,
		ArgumentsJSON: arguments,
		Status:        status,
	}
}

func parseResponseServerToolCall(item map[string]interface{}) ToolCall {
	itemType := strings.TrimSpace(getString(item["type"]))
	if itemType == "" {
		itemType = "server_tool_call"
	}
	toolCallID := responseServerToolCallID(item, itemType)
	toolName := firstNonEmptyString(getString(item["name"]), responseServerToolNameFromType(itemType))
	status := firstNonEmptyString(getString(item["status"]), "completed")
	actionInputJSON, actionOutputJSON := splitResponsesServerToolAction(item["action"])
	inputJSON := firstNonEmptyString(
		actionInputJSON,
		normalizeJSONString(item["arguments"]),
		normalizeJSONString(item["input"]),
		normalizeJSONString(item["query"]),
	)
	outputJSON := firstNonEmptyString(
		normalizeJSONString(item["output"]),
		normalizeJSONString(item["outputs"]),
		normalizeJSONString(item["results"]),
		normalizeJSONString(item["search_results"]),
		normalizeJSONString(item["sources"]),
		normalizeJSONString(item["citations"]),
		normalizeJSONString(item["data"]),
		normalizeJSONString(item["items"]),
		normalizeJSONString(item["content"]),
		normalizeJSONString(item["response"]),
		normalizeJSONString(item["result"]),
		actionOutputJSON,
	)
	errorJSON := normalizeJSONString(item["error"])
	return ToolCall{
		ToolCallID:    toolCallID,
		ToolType:      itemType,
		ToolName:      toolName,
		ArgumentsJSON: inputJSON,
		Status:        status,
		OutputJSON:    outputJSON,
		ErrorJSON:     errorJSON,
	}
}

func responseServerToolCallID(item map[string]interface{}, itemType string) string {
	for _, extension := range allResponsesProtocolExtensions() {
		if extension.serverToolCallID == nil {
			continue
		}
		if toolCallID, ok := extension.serverToolCallID(item, itemType); ok {
			return toolCallID
		}
	}
	return firstNonEmptyString(
		getString(item["item_id"]),
		getString(item["call_id"]),
		getString(item["id"]),
		getString(item["tool_call_id"]),
	)
}

func responseServerToolNameFromType(itemType string) string {
	value := strings.TrimSpace(itemType)
	for _, suffix := range []string{"_call_output", "_call"} {
		value = strings.TrimSuffix(value, suffix)
	}
	return value
}

func splitResponsesServerToolAction(raw interface{}) (string, string) {
	action := asMap(raw)
	if len(action) == 0 {
		return normalizeJSONString(raw), ""
	}
	input := cloneMap(action)
	delete(input, "sources")
	output := make(map[string]interface{})
	if query := strings.TrimSpace(getString(action["query"])); query != "" {
		output["query"] = query
	}
	if actionType := strings.TrimSpace(getString(action["type"])); actionType != "" {
		output["type"] = actionType
	}
	if sources := asSlice(action["sources"]); len(sources) > 0 {
		output["sources"] = sources
	}
	outputJSON := ""
	if len(output) > 0 && len(asSlice(output["sources"])) > 0 {
		outputJSON = normalizeJSONString(output)
	}
	return normalizeJSONString(input), outputJSON
}

func isResponsesServerToolCallItem(item map[string]interface{}) bool {
	itemType := strings.TrimSpace(getString(item["type"]))
	if isResponsesServerToolCallType(itemType) {
		return true
	}
	for _, extension := range allResponsesProtocolExtensions() {
		if extension.isServerToolCallItem != nil && extension.isServerToolCallItem(item) {
			return true
		}
	}
	return false
}

func isResponsesServerToolCallType(itemType string) bool {
	switch strings.TrimSpace(itemType) {
	case "web_search_call",
		"web_search_call_output",
		"file_search_call",
		"file_search_call_output",
		"code_interpreter_call",
		"code_interpreter_call_output",
		"code_execution_call",
		"code_execution_call_output",
		"collections_search_call",
		"collections_search_call_output",
		"attachment_search_call",
		"attachment_search_call_output",
		"computer_call",
		"computer_call_output",
		"mcp_call",
		"mcp_call_output",
		"image_generation_call",
		"image_generation_call_output",
		"shell_call",
		"shell_call_output",
		"local_shell_call",
		"local_shell_call_output":
		return true
	default:
		for _, extension := range allResponsesProtocolExtensions() {
			if extension.isServerToolCallType != nil && extension.isServerToolCallType(itemType) {
				return true
			}
		}
		return false
	}
}

func isResponsesClientToolCallType(itemType string) bool {
	value := strings.TrimSpace(itemType)
	if value == "" {
		return false
	}
	if value == "function_call" || value == "tool_call" || value == "custom_tool_call" {
		return true
	}
	return strings.HasSuffix(value, "_tool_call") && !isResponsesServerToolCallType(value)
}

func parseResponseCitations(parsed map[string]interface{}) []string {
	if len(parsed) == 0 {
		return nil
	}
	result := make([]string, 0)
	for _, key := range []string{"citations", "sources", "urls"} {
		for _, raw := range asSlice(parsed[key]) {
			if text := firstNonEmptyString(getString(raw), getStringFromPath(asMap(raw), "url"), getStringFromPath(asMap(raw), "uri")); text != "" {
				result = append(result, text)
			}
		}
	}
	collectResponseCitationURLs(parsed, &result)
	return appendUniqueStrings(nil, result...)
}

func collectResponseCitationURLs(raw interface{}, result *[]string) {
	if result == nil {
		return
	}
	switch value := raw.(type) {
	case []interface{}:
		for _, item := range value {
			collectResponseCitationURLs(item, result)
		}
	case map[string]interface{}:
		itemType := strings.TrimSpace(getString(value["type"]))
		switch itemType {
		case "url_citation", "file_citation":
			if text := firstNonEmptyString(
				getString(value["url"]),
				getString(value["uri"]),
				getStringFromPath(value, "url_citation", "url"),
				getStringFromPath(value, "file_citation", "url"),
			); text != "" {
				*result = append(*result, text)
			}
		}
		if text := firstNonEmptyString(getString(value["url"]), getString(value["uri"])); text != "" {
			*result = append(*result, text)
		}
		for _, key := range []string{"action", "content", "annotations", "output", "outputs", "results", "search_results", "sources", "citations", "url_citation", "file_citation", "data", "items", "response", "result"} {
			collectResponseCitationURLs(value[key], result)
		}
	}
}

func parseServerSideToolUsage(parsed map[string]interface{}) map[string]int64 {
	usage := asMap(parsed["usage"])
	if len(usage) == 0 {
		return nil
	}
	raw := asMap(usage["server_side_tool_usage"])
	if len(raw) == 0 {
		raw = asMap(usage["tool_usage"])
	}
	if len(raw) == 0 {
		raw = asMap(usage["server_side_tool_usage_details"])
	}
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string]int64, len(raw))
	for key, value := range raw {
		if normalized := normalizeServerSideToolUsageKey(key); normalized != "" {
			result[normalized] = toInt64(value)
		}
	}
	return result
}

func normalizeServerSideToolUsageKey(key string) string {
	value := strings.TrimSpace(key)
	value = strings.TrimSuffix(value, "_calls")
	value = strings.TrimSuffix(value, "_call")
	switch value {
	case "web_search", "code_interpreter", "image_generation", "shell", "file_search", "mcp", "document_search":
		return value
	default:
		for _, extension := range allResponsesProtocolExtensions() {
			if extension.normalizeServerSideToolUsageKey == nil {
				continue
			}
			if normalized, ok := extension.normalizeServerSideToolUsageKey(value, key); ok {
				return normalized
			}
		}
		return key
	}
}

func extractOutputTextChunk(item map[string]interface{}) string {
	if chunk := extractContentText(item["content"]); chunk != "" {
		return chunk
	}
	if chunk := getString(item["text"]); chunk != "" {
		return chunk
	}
	return ""
}
