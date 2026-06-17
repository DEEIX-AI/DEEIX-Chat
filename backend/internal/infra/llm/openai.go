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
	"net/url"
	"strings"
	"time"
)

const (
	openRouterDefaultAttributionReferer = "https://deeix.com"
	openRouterDefaultAttributionTitle   = "DEEIX Chat"
	openRouterDefaultCategories         = "general-chat"
)

// generateOpenAICompatible 调用 OpenAI 兼容接口并解析响应（非流式）。
// 超时策略：ReadTimeoutMS 控制整体超时（含 LLM 推理等待）。
func (c *Client) generateOpenAICompatible(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error) {
	endpoint := normalizeEndpoint(route.Endpoint)
	requestURL := buildOpenAIRequestURL(route.BaseURL, endpoint)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	requestBody, err := buildOpenAIRequestBody(route.Protocol, route.UpstreamModel, endpoint, input, false)
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

	debug := upstreamDebugSnapshot(req, payload, resp, body)
	output, err := parseOpenAIGenerateOutput(endpoint, route.Protocol, body, deepSeekTextEncodedToolCallsEnabled(route))
	if err != nil {
		return nil, attachUpstreamDebug(err, debug)
	}
	output.Debug = debug
	return output, nil
}

// generateStreamOpenAICompatible 调用上游流式推理接口并实时回传增量文本。
// 超时策略：
//   - ReadTimeoutMS  控制 TCP 建连 + 等待首字节（含 LLM 推理排队）
//   - StreamIdleTimeoutMS 控制流传输中两个 chunk 之间的最大间隔（防假死）
func (c *Client) generateStreamOpenAICompatible(
	ctx context.Context,
	route RouteConfig,
	input GenerateInput,
	onEvent func(GenerateStreamEvent) error,
) (*GenerateOutput, error) {
	endpoint := normalizeEndpoint(route.Endpoint)
	requestURL := buildOpenAIRequestURL(route.BaseURL, endpoint)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	requestBody, err := buildOpenAIRequestBody(route.Protocol, route.UpstreamModel, endpoint, input, true)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	// 用父 ctx 发起请求，不绑定超时 context，避免 cancel 关闭底层连接。
	// 首字节超时通过独立 timer + 取消专用 context 实现。
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

	result := &GenerateOutput{
		ResponseID:      "",
		Text:            "",
		Usage:           Usage{},
		ToolCalls:       make([]ToolCall, 0),
		ServerToolCalls: make([]ToolCall, 0),
		RawJSON:         "",
	}

	idleTimeout := resolveStreamIdleTimeout(route.StreamIdleTimeoutMS)
	idleReader := newIdleTimeoutReader(resp.Body, idleTimeout)
	streamBody := newUpstreamBodyRecorder(idleReader)
	if err = consumeOpenAIGenerateStream(endpoint, route.Protocol, streamBody, result, onEvent, deepSeekTextEncodedToolCallsEnabled(route)); err != nil {
		return nil, attachUpstreamDebug(err, upstreamDebugSnapshot(req, payload, resp, streamErrorBody(streamBody, err)))
	}
	return result, nil
}

// listModelsOpenAICompatible 调用上游 models 目录接口。
func (c *Client) listModelsOpenAICompatible(ctx context.Context, route RouteConfig) ([]ModelItem, error) {
	requestURL := buildOpenAIModelsURL(route.BaseURL)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}

	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
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
		return nil, parseUpstreamError(resp.StatusCode, body, upstreamDebugSnapshot(req, nil, resp, body))
	}

	return parseOpenAIModelList(body)
}

func buildOpenAIRequestBody(protocol string, model string, endpoint string, input GenerateInput, stream bool) (map[string]interface{}, error) {
	endpoint = normalizeEndpoint(endpoint)
	messages := normalizeMessages(input.Messages)
	adapter := NormalizeAdapter(protocol)
	providerTools, toolDefinitions, toolsEnabled, err := toolDeclarationsForInput(input)
	if err != nil {
		return nil, err
	}
	providerStreamOptions, err := providerStreamOptionsFromOptions(input.Options)
	if err != nil {
		return nil, err
	}

	switch endpoint {
	case EndpointChatCompletions:
		return buildChatCompletionsRequestBody(model, input, messages, providerTools, toolDefinitions, providerStreamOptions, stream), nil
	default:
		return buildResponsesRequestBody(adapter, model, input, messages, providerTools, toolDefinitions, toolsEnabled, providerStreamOptions, stream), nil
	}
}

func buildOpenAITools(tools []ToolDefinition, chatCompletions bool) []map[string]interface{} {
	if len(tools) == 0 {
		return nil
	}
	items := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		schema := decodeToolSchema(tool.InputSchema)
		if chatCompletions {
			items = append(items, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        name,
					"description": strings.TrimSpace(tool.Description),
					"parameters":  schema,
				},
			})
			continue
		}
		items = append(items, map[string]interface{}{
			"type":        "function",
			"name":        name,
			"description": strings.TrimSpace(tool.Description),
			"parameters":  schema,
		})
	}
	return items
}

func applyOpenAICompatibleSamplingParams(payload map[string]interface{}, params map[string]interface{}, chatCompletions bool) {
	if value, ok := modelParamFloat(params, "temperature"); ok {
		payload["temperature"] = value
	}
	if value, ok := modelParamFloat(params, "top_p"); ok {
		payload["top_p"] = value
	}
	if chatCompletions {
		if value, ok := modelParamFloat(params, "frequency_penalty"); ok {
			payload["frequency_penalty"] = value
		}
		if value, ok := modelParamFloat(params, "presence_penalty"); ok {
			payload["presence_penalty"] = value
		}
		if seed := modelParamInt(params, "seed"); seed > 0 {
			payload["seed"] = seed
		}
		if stops := modelParamStringList(params, "stop"); len(stops) == 1 {
			payload["stop"] = stops[0]
		} else if len(stops) > 1 {
			payload["stop"] = stops
		} else if raw, ok := params["stop"]; ok && raw == nil {
			payload["stop"] = nil
		}
		if format, ok := normalizedChatCompletionResponseFormat(params); ok {
			payload["response_format"] = format
		}
		return
	}
	if format, ok := normalizedJSONResponseFormat(params); ok {
		setOpenAIResponseTextParam(payload, "format", format)
	}
}

func setOpenAIResponseTextParam(payload map[string]interface{}, key string, value interface{}) {
	text, _ := payload["text"].(map[string]interface{})
	if text == nil {
		text = map[string]interface{}{}
		payload["text"] = text
	}
	text[key] = value
}

func normalizePromptCacheRetention(value string) string {
	switch strings.TrimSpace(value) {
	case "in-memory":
		return "in-memory"
	case "in_memory":
		return "in-memory"
	case "24h":
		return "24h"
	default:
		return ""
	}
}

func buildOpenAIRequestURL(baseURL string, endpoint string) string {
	switch endpoint {
	case EndpointChatCompletions:
		return buildVersionedEndpointURL(baseURL, "v1", "/chat/completions")
	case EndpointImageGenerations:
		return buildVersionedEndpointURL(baseURL, "v1", "/images/generations")
	case EndpointImageEdits:
		return buildVersionedEndpointURL(baseURL, "v1", "/images/edits")
	default:
		return buildVersionedEndpointURL(baseURL, "v1", "/responses")
	}
}

func buildOpenAIModelsURL(baseURL string) string {
	return buildVersionedEndpointURL(baseURL, "v1", "/models")
}

func setOpenRouterAttributionHeaders(req *http.Request, route RouteConfig) {
	if req == nil || !isOpenRouterBaseURL(route.BaseURL) {
		return
	}
	if req.Header.Get("HTTP-Referer") == "" && !hasAdditionalHeader(route.HeadersJSON, "HTTP-Referer") {
		referer := strings.TrimRight(strings.TrimSpace(route.AttributionReferer), "/")
		if referer == "" {
			referer = openRouterDefaultAttributionReferer
		}
		req.Header.Set("HTTP-Referer", referer)
	}
	if !hasAdditionalHeader(route.HeadersJSON, "X-Title", "X-OpenRouter-Title") &&
		(req.Header.Get("X-Title") == "" || req.Header.Get("X-OpenRouter-Title") == "") {
		title := strings.TrimSpace(route.AttributionTitle)
		if title == "" {
			title = openRouterDefaultAttributionTitle
		}
		if req.Header.Get("X-Title") == "" {
			req.Header.Set("X-Title", title)
		}
		if req.Header.Get("X-OpenRouter-Title") == "" {
			req.Header.Set("X-OpenRouter-Title", title)
		}
	}
	if req.Header.Get("X-OpenRouter-Categories") == "" && !hasAdditionalHeader(route.HeadersJSON, "X-OpenRouter-Categories") {
		req.Header.Set("X-OpenRouter-Categories", openRouterDefaultCategories)
	}
}

func hasAdditionalHeader(headersJSON string, names ...string) bool {
	value := strings.TrimSpace(headersJSON)
	if value == "" {
		return false
	}
	parsed := make(map[string]interface{})
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return false
	}
	for key := range parsed {
		headerKey := strings.TrimSpace(key)
		for _, name := range names {
			if strings.EqualFold(headerKey, name) {
				return true
			}
		}
	}
	return false
}

func isOpenRouterBaseURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return host == "openrouter.ai" || strings.HasSuffix(host, ".openrouter.ai")
}

func consumeOpenAIGenerateStream(
	endpoint string,
	adapter string,
	reader io.Reader,
	result *GenerateOutput,
	onEvent func(GenerateStreamEvent) error,
	allowTextEncodedToolCalls bool,
) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var eventName string
	dataLines := make([]string, 0, 4)

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
			if normalizeEndpoint(endpoint) == EndpointChatCompletions && allowTextEncodedToolCalls {
				if err := flushChatVisibleBuffer(result, onEvent, true); err != nil {
					return err
				}
			}
			return errStreamDone
		}

		parsed := make(map[string]interface{})
		if err := json.Unmarshal([]byte(payloadText), &parsed); err != nil {
			return err
		}
		if err := parseStreamUpstreamError(parsed, payloadText); err != nil {
			return err
		}

		switch normalizeEndpoint(endpoint) {
		case EndpointChatCompletions:
			return applyChatStreamEvent(adapter, parsed, result, onEvent, allowTextEncodedToolCalls)
		default:
			return applyResponsesStreamEvent(adapter, currentEvent, parsed, payloadText, result, onEvent)
		}
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
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	if err := dispatch(); err != nil && !errors.Is(err, errStreamDone) {
		return err
	}
	if normalizeEndpoint(endpoint) == EndpointChatCompletions && allowTextEncodedToolCalls {
		if err := flushChatVisibleBuffer(result, onEvent, true); err != nil {
			return err
		}
	}
	return nil
}

func parseOpenAIGenerateOutput(endpoint string, adapter string, body []byte, allowTextEncodedToolCalls bool) (*GenerateOutput, error) {
	parsed := make(map[string]interface{})
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	result := buildGenerateOutputFromParsedForAdapter(endpoint, adapter, parsed, allowTextEncodedToolCalls)
	if normalizeEndpoint(endpoint) == EndpointChatCompletions && allowTextEncodedToolCalls && maybeDSMLToolCallsPrefix(result.Text) {
		return nil, errDeepSeekDSMLToolCallsIncomplete
	}
	result.RawJSON = string(body)
	return result, nil
}

func buildGenerateOutputFromParsed(endpoint string, parsed map[string]interface{}) *GenerateOutput {
	return buildGenerateOutputFromParsedForAdapter(endpoint, AdapterOpenAIResponses, parsed, false)
}

func buildGenerateOutputFromParsedForAdapter(endpoint string, adapter string, parsed map[string]interface{}, allowTextEncodedToolCalls bool) *GenerateOutput {
	result := &GenerateOutput{
		ResponseID:      strings.TrimSpace(getString(parsed["id"])),
		Text:            "",
		Usage:           Usage{},
		ToolCalls:       make([]ToolCall, 0),
		ServerToolCalls: make([]ToolCall, 0),
		RawJSON:         "",
	}

	switch normalizeEndpoint(endpoint) {
	case EndpointChatCompletions:
		parseChatCompletionsOutput(adapter, parsed, result, allowTextEncodedToolCalls)
	default:
		parseResponsesOutput(adapter, parsed, result)
	}

	if result.ResponseID == "" {
		result.ResponseID = strings.TrimSpace(getStringFromPath(parsed, "response", "id"))
	}
	if result.Text == "" {
		result.Text = getString(parsed["text"])
	}
	return result
}

func parseOpenAIModelList(body []byte) ([]ModelItem, error) {
	parsed := struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}{}
	if err := json.Unmarshal(body, &parsed); err != nil {
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
			OwnedBy: strings.TrimSpace(item.OwnedBy),
		})
	}
	return results, nil
}

func mergeGenerateOutput(dst *GenerateOutput, src *GenerateOutput) {
	if dst == nil || src == nil {
		return
	}
	if dst.ResponseID == "" {
		dst.ResponseID = strings.TrimSpace(src.ResponseID)
	}
	if dst.Text == "" {
		dst.Text = src.Text
	}
	if src.Usage != (Usage{}) {
		usage := src.Usage
		if usage.ServiceTier == "" {
			usage.ServiceTier = dst.Usage.ServiceTier
		}
		dst.Usage = usage
	}
	if src.Reasoning != nil {
		mergeReasoningOutput(&dst.Reasoning, src.Reasoning)
	}
	if len(src.ToolCalls) > 0 {
		dst.ToolCalls = append(dst.ToolCalls[:0], src.ToolCalls...)
	}
	if len(src.ServerToolCalls) > 0 {
		dst.ServerToolCalls = append(dst.ServerToolCalls[:0], src.ServerToolCalls...)
	}
	if len(src.ServerSideToolUsage) > 0 {
		dst.ServerSideToolUsage = cloneInt64Map(src.ServerSideToolUsage)
	}
	if len(src.Citations) > 0 {
		dst.Citations = append(dst.Citations[:0], src.Citations...)
	}
	if strings.TrimSpace(src.RawJSON) != "" {
		dst.RawJSON = src.RawJSON
	}
}
