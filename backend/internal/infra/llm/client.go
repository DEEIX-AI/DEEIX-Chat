package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"

	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/outboundhttp"
	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
	"github.com/google/uuid"
)

// 超时默认值。
const (
	defaultConnectTimeoutMS    = 10000  // TCP 建连超时 10s
	defaultReadTimeoutMS       = 120000 // 非流式/首字节超时 120s（含 LLM 推理）
	defaultStreamIdleTimeoutMS = 60000  // 流式 chunk 间隔超时 60s
	maxUpstreamBodyBytes       = 64 * 1024 * 1024
	// maxStreamToolCallIndex 防止异常稀疏的上游索引触发无界切片扩容。
	maxStreamToolCallIndex = 1024
)

const upstreamRequestIDHeaderTemplate = "${DEEIX_UPSTREAM_REQUEST_ID}"

// Client 负责跨厂商共享的 HTTP client、adapter 路由和上游调试能力。
type Client struct {
	httpClients *outboundhttp.Pool
	adapters    map[string]transportAdapter
}

func normalizeConnectTimeoutMS(ms int) int {
	if ms <= 0 {
		return defaultConnectTimeoutMS
	}
	return ms
}

func resolveReadTimeout(ms int) time.Duration {
	if ms <= 0 {
		return time.Duration(defaultReadTimeoutMS) * time.Millisecond
	}
	return time.Duration(ms) * time.Millisecond
}

func resolveStreamIdleTimeout(ms int) time.Duration {
	if ms <= 0 {
		return time.Duration(defaultStreamIdleTimeoutMS) * time.Millisecond
	}
	return time.Duration(ms) * time.Millisecond
}

func modelParamString(params map[string]any, key string) string {
	if params == nil {
		return ""
	}
	value, ok := params[key]
	if !ok {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func modelParamInt(params map[string]any, key string) int {
	value, ok := modelParamIntValue(params, key)
	if !ok {
		return 0
	}
	return value
}

func modelParamIntValue(params map[string]any, key string) (int, bool) {
	if params == nil {
		return 0, false
	}
	value, ok := params[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}

func modelParamFloat(params map[string]any, key string) (float64, bool) {
	if params == nil {
		return 0, false
	}
	value, ok := params[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

func modelParamBool(params map[string]any, key string) bool {
	value, ok := modelParamBoolValue(params, key)
	return ok && value
}

func modelParamBoolValue(params map[string]any, key string) (bool, bool) {
	if params == nil {
		return false, false
	}
	value, ok := params[key]
	if !ok {
		return false, false
	}
	typed, ok := value.(bool)
	return typed, ok
}

func modelParamStringList(params map[string]any, key string) []string {
	if params == nil {
		return nil
	}
	value, ok := params[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil
		}
		return []string{text}
	case []string:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(item); text != "" {
				items = append(items, text)
			}
		}
		return items
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				if normalized := strings.TrimSpace(text); normalized != "" {
					items = append(items, normalized)
				}
			}
		}
		return items
	default:
		return nil
	}
}

func modelParamMap(params map[string]any, key string) map[string]any {
	if params == nil {
		return nil
	}
	value, ok := params[key]
	if !ok {
		return nil
	}
	return asMap(value)
}

func applyProviderOptions(payload map[string]any, options map[string]any, protectedKeys ...string) {
	if len(options) == 0 {
		return
	}
	protected := make(map[string]struct{}, len(protectedKeys))
	for _, key := range protectedKeys {
		protected[key] = struct{}{}
	}
	for key, value := range options {
		if _, ok := protected[key]; ok {
			continue
		}
		if current, ok := payload[key].(map[string]any); ok {
			if incoming, ok := value.(map[string]any); ok {
				for nestedKey, nestedValue := range incoming {
					current[nestedKey] = nestedValue
				}
				continue
			}
		}
		if shouldSkipNormalizedProviderOption(key, value) {
			continue
		}
		payload[key] = value
	}
}

func providerToolsFromOptions(options map[string]any) ([]map[string]any, error) {
	if len(options) == 0 {
		return nil, nil
	}
	raw, ok := options["tools"]
	if !ok || raw == nil {
		return nil, nil
	}
	switch typed := raw.(type) {
	case []map[string]any:
		return append([]map[string]any(nil), typed...), nil
	case []any:
		items := make([]map[string]any, 0, len(typed))
		for index, item := range typed {
			payload, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("model option tools[%d] must be an object", index)
			}
			items = append(items, payload)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("model option tools must be an array")
	}
}

func toolDeclarationsForInput(input portllm.GenerateInput) ([]map[string]any, []portllm.ToolDefinition, bool, error) {
	if input.DisableTools {
		return nil, nil, false, nil
	}
	providerTools, err := providerToolsFromOptions(input.Options)
	if err != nil {
		return nil, nil, false, err
	}
	return providerTools, input.Tools, true, nil
}

func providerStreamOptionsFromOptions(options map[string]any) (map[string]any, error) {
	if len(options) == 0 {
		return nil, nil
	}
	raw, ok := options["stream_options"]
	if !ok || raw == nil {
		return nil, nil
	}
	payload, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("model option stream_options must be an object")
	}
	return payload, nil
}

func appendToolDeclarations(payload map[string]any, tools ...[]map[string]any) {
	total := 0
	for _, group := range tools {
		total += len(group)
	}
	if total == 0 {
		return
	}
	merged := make([]map[string]any, 0, total)
	if existing, ok := payload["tools"].([]map[string]any); ok {
		merged = append(merged, existing...)
	}
	for _, group := range tools {
		merged = append(merged, group...)
	}
	if len(merged) > 0 {
		payload["tools"] = merged
	}
}

func appendResponseInclude(payload map[string]any, values ...string) {
	if payload == nil {
		return
	}
	current := make([]string, 0)
	switch existing := payload["include"].(type) {
	case []string:
		current = append(current, existing...)
	case []any:
		for _, item := range existing {
			if text, ok := item.(string); ok {
				current = append(current, text)
			}
		}
	}
	payload["include"] = appendUniqueStrings(current, values...)
}

func responseIncludeValues(options map[string]any, defaults ...string) []string {
	values := make([]string, 0, len(defaults))
	values = append(values, defaults...)
	values = append(values, modelParamStringList(options, "include")...)
	return appendUniqueStrings(nil, values...)
}

func mergeObjectParam(payload map[string]any, key string, values map[string]any) {
	if payload == nil || len(values) == 0 {
		return
	}
	current, _ := payload[key].(map[string]any)
	if current == nil {
		current = map[string]any{}
		payload[key] = current
	}
	for field, value := range values {
		if strings.TrimSpace(field) != "" {
			current[field] = value
		}
	}
}

func normalizedJSONResponseFormat(options map[string]any) (any, bool) {
	if len(options) == 0 {
		return nil, false
	}
	raw, ok := options["response_format"]
	if !ok || raw == nil {
		return nil, false
	}
	if text := strings.TrimSpace(getString(raw)); text != "" {
		switch text {
		case "json":
			return map[string]string{"type": "json_object"}, true
		case "json_object", "text":
			return map[string]string{"type": text}, true
		default:
			return nil, false
		}
	}
	format := asMap(raw)
	if len(format) == 0 {
		return nil, false
	}
	if strings.TrimSpace(getString(format["type"])) == "" {
		return nil, false
	}
	return format, true
}

func shouldSkipNormalizedProviderOption(key string, value any) bool {
	if _, ok := value.(map[string]any); ok {
		return false
	}
	switch key {
	case "temperature",
		"top_p",
		"top_k",
		"frequency_penalty",
		"presence_penalty",
		"seed",
		"stop",
		"reasoning_effort",
		"verbosity",
		"max_output_tokens",
		"max_completion_tokens",
		"response_format",
		"web_search",
		"prompt_cache",
		"prompt_cache_retention",
		"enable_cache",
		"cache_timeout",
		"enable_thinking",
		"thinking_display",
		"effort",
		"budget_tokens",
		"thinking_budget",
		"thinking_level":
		return true
	case "thinking":
		_, ok := value.(bool)
		return ok
	default:
		return false
	}
}

// nonCachedInputTokens 从提示词侧总量中扣除已按缓存价计费的 token（缓存读取与缓存写入），
// 得到按普通输入价计费的非缓存输入。上游各字段是未对齐的前缀计数，差值可能为负，钳制为 0。
func nonCachedInputTokens(totalInputTokens int64, cachedTokens int64) int64 {
	if totalInputTokens <= 0 {
		return 0
	}
	if cachedTokens <= 0 {
		return totalInputTokens
	}
	remaining := totalInputTokens - cachedTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

func rawUsageJSONFromPath(payload map[string]any, keys ...string) string {
	if len(payload) == 0 || len(keys) == 0 {
		return ""
	}
	var current any = payload
	for _, key := range keys {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = currentMap[key]
		if !ok {
			return ""
		}
	}
	switch value := current.(type) {
	case map[string]any:
		if len(value) == 0 {
			return ""
		}
	case []any:
		if len(value) == 0 {
			return ""
		}
	default:
		return ""
	}
	raw, err := json.Marshal(current)
	if err != nil {
		return ""
	}
	return string(raw)
}

// generatedMediaDurationSeconds 将上游媒体时长统一向上取整为可计费秒数。
func generatedMediaDurationSeconds(values ...any) int64 {
	for _, value := range values {
		var seconds float64
		switch typed := value.(type) {
		case int:
			seconds = float64(typed)
		case int64:
			seconds = float64(typed)
		case float64:
			seconds = typed
		case float32:
			seconds = float64(typed)
		case string:
			text := strings.TrimSpace(strings.ToLower(typed))
			for _, suffix := range []string{"seconds", "second", "secs", "sec", "s"} {
				text = strings.TrimSuffix(text, suffix)
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
			if err != nil {
				continue
			}
			seconds = parsed
		default:
			continue
		}
		if seconds <= 0 {
			continue
		}
		whole := int64(seconds)
		if float64(whole) < seconds {
			whole++
		}
		return whole
	}
	return 0
}

// doGenerationRequest 记录 POST 请求是否已经写入连接。请求写出后若在响应头
// 返回前断线，无法判断上游是否已开始生成，因此按已接受处理，避免跨路由重复生成。
func doGenerationRequest(do func(*http.Request) (*http.Response, error), req *http.Request) (*http.Response, error) {
	if do == nil || req == nil {
		return nil, errors.New("generation request is nil")
	}
	var wroteRequest atomic.Bool
	trace := &httptrace.ClientTrace{
		WroteRequest: func(httptrace.WroteRequestInfo) {
			wroteRequest.Store(true)
		},
	}
	tracedRequest := req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	resp, err := do(tracedRequest)
	if err != nil && wroteRequest.Load() {
		return resp, portllm.MarkRequestAccepted(err)
	}
	return resp, err
}

var errStreamDone = errors.New("llm stream done")

// NewClient 创建带出站安全策略的上游调用客户端。
func NewClient(outboundPolicy security.OutboundPolicy) *Client {
	client := &Client{}
	client.httpClients = outboundhttp.NewPool(outboundPolicy, outboundhttp.DefaultCacheLimit, func(policy security.OutboundPolicy, trustedOrigin string, variant string) (outboundhttp.ManagedClient, error) {
		return newRouteHTTPClient(policy, outboundPolicy, trustedOrigin, variant)
	})
	client.adapters = map[string]transportAdapter{
		portllm.AdapterOpenAIResponses:        &openAIResponsesAdapter{client: client},
		portllm.AdapterOpenRouterChat:         &openRouterChatCompletionsAdapter{client: client},
		portllm.AdapterOpenRouterResponses:    &openRouterResponsesAdapter{client: client},
		portllm.AdapterOpenAIChatCompletions:  &openAIChatCompletionsAdapter{client: client},
		portllm.AdapterOpenAIImageGenerations: &openAIImageGenerationsAdapter{client: client},
		portllm.AdapterOpenAIImageEdits:       &openAIImageEditsAdapter{client: client},
		portllm.AdapterXAIResponses:           &xAIResponsesAdapter{client: client},
		portllm.AdapterXAIImage:               &xAIImageAdapter{client: client},
		portllm.AdapterXAIImageEdits:          &xAIImageEditsAdapter{client: client},
		portllm.AdapterXAIVideo:               &xAIVideoAdapter{client: client},
		portllm.AdapterXAIVideoExtensions:     &xAIVideoExtensionsAdapter{client: client},
		portllm.AdapterAnthropicMessages:      &anthropicMessagesAdapter{client: client},
		portllm.AdapterGoogleGenerateContent:  &geminiGenerateContentAdapter{client: client},
		portllm.AdapterGoogleImageGeneration:  &geminiImageGenerationAdapter{client: client},
		portllm.AdapterGeminiInteractions:     &geminiInteractionsAdapter{client: client},
	}
	return client
}

func (c *Client) adapterFor(route portllm.RouteConfig) (transportAdapter, error) {
	adapterName := portllm.NormalizeAdapter(route.Protocol)
	if !portllm.IsImplementedAdapter(adapterName) {
		return nil, fmt.Errorf("%w: %s", portllm.ErrUnsupportedAdapter, adapterName)
	}
	adapter, ok := c.adapters[adapterName]
	if !ok {
		return nil, fmt.Errorf("%w: %s", portllm.ErrUnsupportedAdapter, adapterName)
	}
	return adapter, nil
}

func newRouteHTTPClient(policy security.OutboundPolicy, redirectPolicy security.OutboundPolicy, trustedOrigin string, variant string) (outboundhttp.ManagedClient, error) {
	connectTimeoutMS, err := strconv.Atoi(variant)
	if err != nil || connectTimeoutMS <= 0 {
		return outboundhttp.ManagedClient{}, fmt.Errorf("invalid LLM connect timeout %q", variant)
	}
	transport := security.NewOutboundHTTPTransport(policy, time.Duration(connectTimeoutMS)*time.Millisecond)
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 20
	transport.IdleConnTimeout = 90 * time.Second
	transport.ForceAttemptHTTP2 = true

	client := &http.Client{
		Timeout:   0,
		Transport: platformtracing.NewHTTPTransport(transport),
	}
	if trustedOrigin != "" {
		client.CheckRedirect = outboundhttp.NewRedirectPolicy(redirectPolicy, trustedOrigin, "model provider")
	}
	return outboundhttp.ManagedClient{Client: client, CloseIdleConnections: transport.CloseIdleConnections}, nil
}

func (c *Client) doRouteRequest(route portllm.RouteConfig, request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, fmt.Errorf("model provider request is nil")
	}
	connectTimeoutMS := normalizeConnectTimeoutMS(route.ConnectTimeoutMS)
	return c.httpClients.Do(request, route.BaseURL, strconv.Itoa(connectTimeoutMS))
}

func (c *Client) doRouteGenerationRequest(route portllm.RouteConfig, request *http.Request) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, fmt.Errorf("model provider request is nil")
	}
	connectTimeoutMS := normalizeConnectTimeoutMS(route.ConnectTimeoutMS)
	return doGenerationRequest(func(tracedRequest *http.Request) (*http.Response, error) {
		return c.httpClients.Do(tracedRequest, route.BaseURL, strconv.Itoa(connectTimeoutMS))
	}, request)
}

// CloseIdleConnections 释放所有模型 origin 客户端的空闲连接。
func (c *Client) CloseIdleConnections() {
	if c != nil && c.httpClients != nil {
		c.httpClients.CloseIdleConnections()
	}
}

// Generate 调用上游适配器并解析响应（非流式）。
func (c *Client) Generate(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	adapter, err := c.adapterFor(route)
	if err != nil {
		return nil, err
	}
	return adapter.Generate(ctx, route, input)
}

// GenerateStream 调用上游适配器并实时回传增量文本。
func (c *Client) GenerateStream(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	adapter, err := c.adapterFor(route)
	if err != nil {
		return nil, err
	}
	return adapter.GenerateStream(ctx, route, input, onEvent)
}

// ListModels 调用上游 models 目录接口。
func (c *Client) ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	adapter, err := c.adapterFor(route)
	if err != nil {
		return nil, err
	}
	items, err := adapter.ListModels(ctx, route)
	if err == nil {
		return items, nil
	}
	if !shouldFallbackToOpenAICompatibleModels(route) {
		return nil, err
	}

	fallbackRoute := route
	fallbackRoute.Protocol = portllm.AdapterOpenAIChatCompletions
	fallbackItems, fallbackErr := c.listModelsOpenAICompatible(ctx, fallbackRoute)
	if fallbackErr != nil {
		return nil, fmt.Errorf("%w; openai-compatible models fallback failed: %w", err, fallbackErr)
	}
	return fallbackItems, nil
}

func shouldFallbackToOpenAICompatibleModels(route portllm.RouteConfig) bool {
	if isOpenRouterBaseURL(route.BaseURL) {
		return false
	}
	switch portllm.NormalizeAdapter(route.Protocol) {
	case portllm.AdapterAnthropicMessages, portllm.AdapterGoogleGenerateContent, portllm.AdapterGoogleImageGeneration:
		return true
	default:
		return false
	}
}

// idleTimeoutReader 包装 io.Reader，在两次 Read 之间超过 idle timeout 时返回错误。
type idleTimeoutReader struct {
	reader  io.Reader
	timeout time.Duration
	timer   *time.Timer
}

func newIdleTimeoutReader(reader io.Reader, timeout time.Duration) *idleTimeoutReader {
	return &idleTimeoutReader{
		reader:  reader,
		timeout: timeout,
		timer:   time.NewTimer(timeout),
	}
}

func (r *idleTimeoutReader) Read(p []byte) (int, error) {
	// 每次成功读取后重置 idle timer
	type readResult struct {
		n   int
		err error
	}
	ch := make(chan readResult, 1)
	go func() {
		n, err := r.reader.Read(p)
		ch <- readResult{n, err}
	}()

	r.timer.Reset(r.timeout)
	select {
	case res := <-ch:
		return res.n, res.err
	case <-r.timer.C:
		return 0, fmt.Errorf("stream idle timeout: no data received for %v", r.timeout)
	}
}

func decodeToolSchema(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	payload := make(map[string]any)
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload) == 0 {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	if strings.TrimSpace(getString(payload["type"])) == "" {
		payload["type"] = "object"
	}
	if _, ok := payload["properties"]; !ok {
		payload["properties"] = map[string]any{}
	}
	return payload
}

func buildToolResultContent(item portllm.ToolResult) string {
	if strings.TrimSpace(item.Error) != "" {
		payload := map[string]any{
			"ok":     false,
			"status": strings.TrimSpace(item.Status),
			"error":  strings.TrimSpace(item.Error),
		}
		raw, err := json.Marshal(payload)
		if err == nil {
			return string(raw)
		}
		return strings.TrimSpace(item.Error)
	}
	output := strings.TrimSpace(item.OutputJSON)
	if output == "" {
		return "{}"
	}
	return output
}

func normalizeMessages(messages []portllm.Message) []portllm.Message {
	normalized := make([]portllm.Message, 0, len(messages))
	for _, item := range messages {
		// 跳过空消息（Parts 非空或 Content 非空才保留）
		if len(item.Parts) == 0 && strings.TrimSpace(item.Content) == "" && len(item.ToolCalls) == 0 && len(item.ToolResults) == 0 {
			continue
		}
		normalized = append(normalized, portllm.Message{
			Role:             normalizeRole(item.Role),
			Content:          item.Content,
			Parts:            item.Parts,
			ReasoningContent: item.ReasoningContent,
			ToolCalls:        item.ToolCalls,
			ToolResults:      item.ToolResults,
			CacheControl:     item.CacheControl,
		})
	}
	if len(normalized) == 0 {
		normalized = append(normalized, portllm.Message{Role: "user", Content: ""})
	}
	return normalized
}

func setAdditionalHeaders(req *http.Request, headersJSON string) {
	setAdditionalHeadersForInput(req, headersJSON, nil)
}

func setAdditionalHeadersForInput(req *http.Request, headersJSON string, input *portllm.GenerateInput) {
	if req == nil {
		return
	}
	value := strings.TrimSpace(headersJSON)
	if value == "" {
		return
	}
	parsed := make(map[string]any)
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return
	}
	dynamicHeaderInput := input
	if input == nil || (strings.TrimSpace(input.ConversationPublicID) == "" && strings.TrimSpace(input.ConversationSessionKey) == "") {
		dynamicHeaderInput = nil
	}
	upstreamRequestID := ""
	if dynamicHeaderInput != nil && strings.Contains(value, upstreamRequestIDHeaderTemplate) {
		upstreamRequestID = uuid.NewString()
	}
	for key, rawValue := range parsed {
		headerKey := strings.TrimSpace(key)
		if headerKey == "" {
			continue
		}
		headerValue, ok := expandAdditionalHeaderValue(stringify(rawValue), dynamicHeaderInput, upstreamRequestID)
		if !ok || strings.TrimSpace(headerValue) == "" {
			continue
		}
		req.Header.Set(headerKey, headerValue)
	}
}

func expandAdditionalHeaderValue(value string, input *portllm.GenerateInput, upstreamRequestID string) (string, bool) {
	replacements := []struct {
		template string
		value    string
	}{
		{template: "${DEEIX_CONVERSATION_ID}"},
		{template: "${DEEIX_SESSION_ID}"},
		{template: "${DEEIX_REQUEST_ID}"},
		{template: upstreamRequestIDHeaderTemplate},
	}
	if input != nil {
		replacements[0].value = strings.TrimSpace(input.ConversationPublicID)
		replacements[1].value = strings.TrimSpace(input.ConversationSessionKey)
		replacements[2].value = strings.TrimSpace(input.RequestID)
		replacements[3].value = strings.TrimSpace(upstreamRequestID)
	}

	expanded := value
	for _, replacement := range replacements {
		if !strings.Contains(expanded, replacement.template) {
			continue
		}
		if replacement.value == "" {
			return "", false
		}
		expanded = strings.ReplaceAll(expanded, replacement.template, replacement.value)
	}
	return expanded, true
}

func readUpstreamBody(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxUpstreamBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxUpstreamBodyBytes {
		return nil, fmt.Errorf("upstream response body exceeds %d bytes", maxUpstreamBodyBytes)
	}
	return body, nil
}

type upstreamBodyRecorder struct {
	reader io.Reader
	buffer bytes.Buffer
}

func newUpstreamBodyRecorder(reader io.Reader) *upstreamBodyRecorder {
	return &upstreamBodyRecorder{reader: reader}
}

func (r *upstreamBodyRecorder) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		_, _ = r.buffer.Write(p[:n])
	}
	return n, err
}

func (r *upstreamBodyRecorder) Bytes() []byte {
	return r.buffer.Bytes()
}

func streamErrorBody(recorder *upstreamBodyRecorder, err error) []byte {
	if recorder != nil && recorder.buffer.Len() > 0 {
		return recorder.Bytes()
	}
	return upstreamErrorBody(err)
}

func parseUpstreamError(statusCode int, body []byte, debug *portllm.UpstreamDebugSnapshot) error {
	message := fmt.Sprintf("upstream_status_%d", statusCode)
	parsed := make(map[string]any)
	if err := json.Unmarshal(body, &parsed); err == nil {
		if m := getStringFromPath(parsed, "error", "message"); m != "" {
			message = m
		} else if m := getString(parsed["message"]); m != "" {
			message = m
		}
	}
	return &portllm.UpstreamError{
		StatusCode: statusCode,
		Message:    message,
		Body:       string(body),
		Debug:      debug,
	}
}

func upstreamDebugSnapshot(req *http.Request, requestBody []byte, resp *http.Response, responseBody []byte) *portllm.UpstreamDebugSnapshot {
	if req == nil {
		return nil
	}
	path := ""
	if req.URL != nil {
		path = req.URL.EscapedPath()
		if req.URL.RawQuery != "" {
			path += "?" + req.URL.RawQuery
		}
	}
	requestDebugBody := portllm.SanitizeUpstreamDebugPayload(requestBody)
	responseDebugBody := portllm.SanitizeUpstreamDebugPayload(responseBody)
	return &portllm.UpstreamDebugSnapshot{
		Request: portllm.UpstreamDebugRequest{
			Method:        req.Method,
			Path:          path,
			Headers:       redactHeaders(req.Header),
			Body:          requestDebugBody.Body,
			BodyBytes:     requestDebugBody.OriginalBytes,
			BodyTruncated: requestDebugBody.Truncated,
			RedactedParts: requestDebugBody.RedactedParts,
		},
		Response: portllm.UpstreamDebugResponse{
			StatusCode:    responseStatusCode(resp),
			Headers:       responseHeaders(resp),
			Body:          responseDebugBody.Body,
			BodyBytes:     responseDebugBody.OriginalBytes,
			BodyTruncated: responseDebugBody.Truncated,
			RedactedParts: responseDebugBody.RedactedParts,
		},
	}
}

func responseStatusCode(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

func responseHeaders(resp *http.Response) map[string]string {
	if resp == nil {
		return map[string]string{}
	}
	return redactHeaders(resp.Header)
}

func redactHeaders(headers http.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		headerKey := strings.TrimSpace(key)
		if headerKey == "" {
			continue
		}
		if isSecretHeader(headerKey) {
			result[headerKey] = "[redacted]"
			continue
		}
		result[headerKey] = strings.Join(values, ", ")
	}
	return result
}

func isSecretHeader(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	return normalized == "authorization" ||
		normalized == "proxy-authorization" ||
		normalized == "cookie" ||
		normalized == "set-cookie" ||
		normalized == "x-api-key" ||
		normalized == "api-key" ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "key")
}

func attachUpstreamDebug(err error, debug *portllm.UpstreamDebugSnapshot) error {
	if err == nil || debug == nil {
		return err
	}
	var upstreamErr *portllm.UpstreamError
	if errors.As(err, &upstreamErr) {
		if upstreamErr.Debug == nil {
			upstreamErr.Debug = debug
		}
		if upstreamErr.StatusCode == 0 {
			upstreamErr.StatusCode = responseStatusCodeFromDebug(debug)
		}
		if strings.TrimSpace(upstreamErr.Body) == "" {
			upstreamErr.Body = debug.Response.Body
		}
		return err
	}
	statusCode := responseStatusCodeFromDebug(debug)
	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = fmt.Sprintf("upstream_status_%d", statusCode)
	}
	return &portllm.UpstreamError{
		StatusCode: statusCode,
		Message:    message,
		Body:       debug.Response.Body,
		Debug:      debug,
		Cause:      err,
	}
}

func responseStatusCodeFromDebug(debug *portllm.UpstreamDebugSnapshot) int {
	if debug != nil && debug.Response.StatusCode >= 100 && debug.Response.StatusCode <= 599 {
		return debug.Response.StatusCode
	}
	return http.StatusBadGateway
}

func upstreamErrorBody(err error) []byte {
	var upstreamErr *portllm.UpstreamError
	if errors.As(err, &upstreamErr) && strings.TrimSpace(upstreamErr.Body) != "" {
		return []byte(upstreamErr.Body)
	}
	return nil
}

func parseStreamUpstreamError(parsed map[string]any, rawBody string) error {
	errorPayload := asMap(parsed["error"])
	if len(errorPayload) == 0 {
		errorPayload = asMap(asMap(parsed["response"])["error"])
	}
	if len(errorPayload) == 0 {
		return nil
	}
	statusCode := streamErrorStatusCode(parsed, errorPayload)
	message := textutil.FirstNonEmpty(
		getString(errorPayload["message"]),
		getString(errorPayload["msg"]),
		getString(parsed["message"]),
		http.StatusText(statusCode),
	)
	return &portllm.UpstreamError{
		StatusCode: statusCode,
		Message:    message,
		Body:       rawBody,
	}
}

func streamErrorStatusCode(parsed map[string]any, errorPayload map[string]any) int {
	for _, raw := range []any{
		errorPayload["status"],
		errorPayload["status_code"],
		errorPayload["code"],
		parsed["status"],
		parsed["status_code"],
	} {
		statusCode := toHTTPStatusCode(raw)
		if statusCode > 0 {
			return statusCode
		}
	}
	return http.StatusBadGateway
}

func toHTTPStatusCode(raw any) int {
	statusCode := toInt64(raw)
	if statusCode >= 100 && statusCode <= 599 {
		return int(statusCode)
	}
	value := strings.TrimSpace(getString(raw))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 100 || parsed > 599 {
		return 0
	}
	return parsed
}

func appendUniqueToolCall(items *[]portllm.ToolCall, item portllm.ToolCall) {
	if items == nil {
		return
	}
	for idx, existing := range *items {
		if shouldMergeToolCall(existing, item) {
			(*items)[idx] = mergeToolCall(existing, item)
			return
		}
	}
	*items = append(*items, item)
}

func shouldMergeToolCall(existing portllm.ToolCall, incoming portllm.ToolCall) bool {
	existingID := strings.TrimSpace(existing.ToolCallID)
	incomingID := strings.TrimSpace(incoming.ToolCallID)
	if existingID != "" && incomingID != "" {
		return existingID == incomingID
	}
	if existingID == "" && incomingID == "" {
		return sameToolCallKind(existing, incoming)
	}
	if existingID == "" && incomingID != "" {
		return sameToolCallKind(existing, incoming)
	}
	return false
}

func sameToolCallKind(left portllm.ToolCall, right portllm.ToolCall) bool {
	leftName := strings.TrimSpace(left.ToolName)
	rightName := strings.TrimSpace(right.ToolName)
	if leftName != "" && rightName != "" && leftName == rightName {
		return true
	}
	leftType := strings.TrimSpace(left.ToolType)
	rightType := strings.TrimSpace(right.ToolType)
	return leftType != "" && rightType != "" && leftType == rightType
}

func mergeToolCall(existing portllm.ToolCall, incoming portllm.ToolCall) portllm.ToolCall {
	merged := existing
	if value := strings.TrimSpace(incoming.ToolCallID); value != "" {
		merged.ToolCallID = value
	}
	if value := strings.TrimSpace(incoming.ToolType); value != "" {
		merged.ToolType = value
	}
	if value := strings.TrimSpace(incoming.ToolName); value != "" {
		merged.ToolName = value
	}
	if value := strings.TrimSpace(incoming.ArgumentsJSON); value != "" {
		merged.ArgumentsJSON = value
	}
	if value := strings.TrimSpace(incoming.ThoughtSignature); value != "" {
		merged.ThoughtSignature = value
	}
	if value := strings.TrimSpace(incoming.OutputJSON); value != "" {
		merged.OutputJSON = value
	}
	if value := strings.TrimSpace(incoming.ErrorJSON); value != "" {
		merged.ErrorJSON = value
	}
	merged.Status = mergeToolCallStatus(existing.Status, incoming.Status)
	return merged
}

func mergeToolCallStatus(existing string, incoming string) string {
	current := strings.TrimSpace(existing)
	next := strings.TrimSpace(incoming)
	if next == "" {
		return current
	}
	if current == "" {
		return next
	}
	if toolCallStatusRank(next) >= toolCallStatusRank(current) {
		return next
	}
	return current
}

func toolCallStatusRank(status string) int {
	switch strings.TrimSpace(status) {
	case "failed", "error":
		return 4
	case "completed", "success":
		return 3
	case "in_progress", "queued", "searching", "streaming", "requested":
		return 2
	default:
		return 1
	}
}

func updateToolCallInput(items *[]portllm.ToolCall, itemID string, input string, done bool) (portllm.ToolCall, bool) {
	if items == nil || strings.TrimSpace(itemID) == "" {
		return portllm.ToolCall{}, false
	}
	for index, item := range *items {
		if strings.TrimSpace(item.ToolCallID) != itemID {
			continue
		}
		if done {
			if strings.TrimSpace(input) != "" {
				item.ArgumentsJSON = strings.TrimSpace(input)
			}
			if strings.TrimSpace(item.Status) == "" || strings.TrimSpace(item.Status) == "in_progress" {
				item.Status = "completed"
			}
		} else if strings.TrimSpace(input) != "" {
			item.ArgumentsJSON += input
			if strings.TrimSpace(item.Status) == "" {
				item.Status = "in_progress"
			}
		}
		(*items)[index] = item
		return item, true
	}
	return portllm.ToolCall{}, false
}

func appendUniqueStrings(items []string, values ...string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	for _, item := range values {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func cloneInt64Map(value map[string]int64) map[string]int64 {
	if len(value) == 0 {
		return nil
	}
	cloned := make(map[string]int64, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}

func extractReasoningDeltaText(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text := extractReasoningDeltaText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	case map[string]any:
		for _, key := range []string{"text", "delta", "thinking", "summary", "content"} {
			if text := extractReasoningDeltaText(value[key]); text != "" {
				return text
			}
		}
	}
	return ""
}

func extractContentText(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case []any:
		chunks := make([]string, 0, len(value))
		for _, item := range value {
			if text := extractContentText(item); text != "" {
				chunks = append(chunks, text)
			}
		}
		return strings.Join(chunks, "")
	case map[string]any:
		if text := getString(value["text"]); text != "" {
			return text
		}
		if text := getString(value["output_text"]); text != "" {
			return text
		}
		if text := getString(value["input_text"]); text != "" {
			return text
		}
		if nested := extractContentText(value["content"]); nested != "" {
			return nested
		}
	}
	return ""
}

func getStringFromPath(payload map[string]any, keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	current := any(payload)
	for _, key := range keys {
		node := asMap(current)
		if len(node) == 0 {
			return ""
		}
		current = node[key]
	}
	return strings.TrimSpace(getString(current))
}

func getInt64FromPath(payload map[string]any, keys ...string) int64 {
	if len(keys) == 0 {
		return 0
	}
	current := any(payload)
	for _, key := range keys {
		node := asMap(current)
		if len(node) == 0 {
			return 0
		}
		current = node[key]
	}
	return toInt64(current)
}

func firstMapItem(items []any) map[string]any {
	for _, item := range items {
		payload := asMap(item)
		if len(payload) > 0 {
			return payload
		}
	}
	return map[string]any{}
}

func normalizeEndpoint(raw string) string {
	switch strings.TrimSpace(raw) {
	case portllm.EndpointChatCompletions:
		return portllm.EndpointChatCompletions
	case portllm.EndpointImageGenerations:
		return portllm.EndpointImageGenerations
	case portllm.EndpointImageEdits:
		return portllm.EndpointImageEdits
	case portllm.EndpointVideoGenerations:
		return portllm.EndpointVideoGenerations
	case portllm.EndpointVideoExtensions:
		return portllm.EndpointVideoExtensions
	case portllm.EndpointInteractions:
		return portllm.EndpointInteractions
	default:
		return portllm.EndpointResponses
	}
}

func normalizeRole(raw string) string {
	switch strings.TrimSpace(raw) {
	case "developer", "system", "user", "assistant", "tool", "function":
		return strings.TrimSpace(raw)
	default:
		return "user"
	}
}

func asMap(raw any) map[string]any {
	if payload, ok := raw.(map[string]any); ok {
		return payload
	}
	return map[string]any{}
}

func cloneMap(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	payload := make(map[string]any, len(raw))
	for key, value := range raw {
		payload[key] = value
	}
	return payload
}

func mergeMapValueIfEmpty(payload map[string]any, key string, value any) {
	if payload == nil || value == nil {
		return
	}
	if existing, ok := payload[key]; ok && normalizeJSONString(existing) != "" {
		return
	}
	if normalizeJSONString(value) == "" {
		return
	}
	payload[key] = value
}

func asSlice(raw any) []any {
	if payload, ok := raw.([]any); ok {
		return payload
	}
	return []any{}
}

func getString(raw any) string {
	switch value := raw.(type) {
	case string:
		return value
	case json.Number:
		return value.String()
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case uint64:
		return strconv.FormatUint(value, 10)
	case bool:
		if value {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func toInt64(raw any) int64 {
	switch value := raw.(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case int32:
		return int64(value)
	case uint:
		return int64(value)
	case uint64:
		if value > uint64(^uint64(0)>>1) {
			return 0
		}
		return int64(value)
	case float64:
		return int64(value)
	case float32:
		return int64(value)
	case json.Number:
		parsed, err := value.Int64()
		if err == nil {
			return parsed
		}
		f, ferr := value.Float64()
		if ferr != nil {
			return 0
		}
		return int64(f)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func streamToolCallIndex(raw any, fallback int) int {
	if fallback < 0 || fallback > maxStreamToolCallIndex {
		fallback = 0
	}
	index, err := strconv.Atoi(strings.TrimSpace(getString(raw)))
	if err != nil || index < 0 || index > maxStreamToolCallIndex {
		return fallback
	}
	return index
}

func normalizeJSONString(raw any) string {
	if raw == nil {
		return ""
	}
	if text, ok := raw.(string); ok {
		value := strings.TrimSpace(text)
		if value == "" {
			return ""
		}
		if json.Valid([]byte(value)) {
			return value
		}
		return value
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func stringify(raw any) string {
	if raw == nil {
		return ""
	}
	if text, ok := raw.(string); ok {
		return text
	}
	if numeric, ok := raw.(json.Number); ok {
		return numeric.String()
	}
	return fmt.Sprintf("%v", raw)
}

func firstNonZero(values ...int64) int64 {
	for _, item := range values {
		if item > 0 {
			return item
		}
	}
	return 0
}
