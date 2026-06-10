package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

const (
	// EndpointResponses 表示 OpenAI Responses API 端点。
	EndpointResponses = "responses"
	// EndpointChatCompletions 表示 OpenAI Chat Completions API 端点。
	EndpointChatCompletions = "chat_completions"
	// EndpointImageGenerations 表示 OpenAI Images API 生成端点。
	EndpointImageGenerations = "image_generations"
	// EndpointImageEdits 表示 OpenAI Images API 编辑端点。
	EndpointImageEdits = "image_edits"
)

// 超时默认值。
const (
	defaultConnectTimeoutMS    = 10000  // TCP 建连超时 10s
	defaultReadTimeoutMS       = 120000 // 非流式/首字节超时 120s（含 LLM 推理）
	defaultStreamIdleTimeoutMS = 60000  // 流式 chunk 间隔超时 60s
	maxUpstreamBodyBytes       = 64 * 1024 * 1024
)

// Client 负责跨厂商共享的 HTTP client、adapter 路由和上游调试能力。
type Client struct {
	baseTransport         *http.Transport
	httpClients           sync.Map
	adapters              map[string]transportAdapter
	env                   string
	ssrfProtectionEnabled bool
}

// RouteConfig 定义渠道路由调用参数。
type RouteConfig struct {
	Protocol            string
	BaseURL             string
	APIKey              string
	HeadersJSON         string
	ConnectTimeoutMS    int // TCP 建连超时（默认 10s）
	ReadTimeoutMS       int // 非流式整体超时 / 流式首字节超时（默认 120s）
	StreamIdleTimeoutMS int // 流式两个 chunk 之间最大间隔（默认 60s）
	Endpoint            string
	UpstreamModel       string
	AttributionReferer  string
	AttributionTitle    string
}

func resolveConnectTimeout(ms int) time.Duration {
	return time.Duration(normalizeConnectTimeoutMS(ms)) * time.Millisecond
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

// ContentPart 类型常量。
const (
	ContentPartText  = "text"  // 纯文本
	ContentPartImage = "image" // 图片（原始字节，序列化时 base64 编码）
	ContentPartFile  = "file"  // 文件提取文本（前端解析后注入）
)

// ContentPart 表示多模态消息中的一个内容片段。
type ContentPart struct {
	Kind         string        // text | image | file
	Text         string        // Kind=text 或 Kind=file 时的文本内容
	MimeType     string        // Kind=image 时的 MIME 类型（如 "image/jpeg"）
	Data         []byte        // Kind=image 时的原始字节（发送时 base64 编码）
	FileName     string        // Kind=file 时的文件显示名
	CacheControl *CacheControl // 支持块级缓存的 adapter 可读取该提示
}

// CacheControl 表示可被支持方言渲染为 prompt cache breakpoint 的提示。
type CacheControl struct {
	Type string
	TTL  string
}

// Message 定义发送给上游的消息结构。
// Parts 非空时覆盖 Content 用于多模态内容。
type Message struct {
	Role             string
	Content          string        // 纯文本消息内容（Parts 为空时使用）
	Parts            []ContentPart // 多模态内容片段（设置后优先于 Content）
	ReasoningContent string        // OpenAI-compatible thinking mode 的 reasoning_content 回灌字段
	ToolCalls        []ToolCall    // assistant 请求执行的工具调用
	ToolResults      []ToolResult  // 工具执行结果，用于回灌下一轮模型调用
	CacheControl     *CacheControl // 支持块级缓存的 adapter 可读取该提示
}

// GenerateInput 定义上游推理请求入参。
type GenerateInput struct {
	RequestID      string
	ConversationID uint
	Messages       []Message
	Tools          []ToolDefinition
	// DisableTools 表示本轮调用必须只生成文本，adapter 不再声明 MCP 或厂商原生工具。
	DisableTools bool
	// Options 承载本次调用的自由 JSON 参数。系统字段（model/messages/input/stream）
	// 由 adapter 固定构造；Options 只表达采样、推理、工具、缓存和厂商原生扩展。
	Options map[string]interface{}
	// PreviousResponseID 供 OpenAI/xAI Responses API 实现有状态会话。
	// 非空时：仅在 input 中发送本轮新消息，服务端从存储状态续接历史。
	// 空串时：退回全量发送模式，适用于所有 adapter。
	PreviousResponseID string
	// ImageEditMask 仅供图片编辑 adapter 使用，表示透明区域掩码。
	ImageEditMask *ContentPart
}

// ToolDefinition 是模型可调用工具的统一声明。
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

func modelParamString(params map[string]interface{}, key string) string {
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

func modelParamInt(params map[string]interface{}, key string) int {
	value, ok := modelParamIntValue(params, key)
	if !ok {
		return 0
	}
	return value
}

func modelParamIntValue(params map[string]interface{}, key string) (int, bool) {
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

func modelParamFloat(params map[string]interface{}, key string) (float64, bool) {
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

func modelParamBool(params map[string]interface{}, key string) bool {
	value, ok := modelParamBoolValue(params, key)
	return ok && value
}

func modelParamBoolValue(params map[string]interface{}, key string) (bool, bool) {
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

func modelParamStringList(params map[string]interface{}, key string) []string {
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
	case []interface{}:
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

func modelParamMap(params map[string]interface{}, key string) map[string]interface{} {
	if params == nil {
		return nil
	}
	value, ok := params[key]
	if !ok {
		return nil
	}
	return asMap(value)
}

func applyProviderOptions(payload map[string]interface{}, options map[string]interface{}, protectedKeys ...string) {
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
		if current, ok := payload[key].(map[string]interface{}); ok {
			if incoming, ok := value.(map[string]interface{}); ok {
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

func providerToolsFromOptions(options map[string]interface{}) ([]map[string]interface{}, error) {
	if len(options) == 0 {
		return nil, nil
	}
	raw, ok := options["tools"]
	if !ok || raw == nil {
		return nil, nil
	}
	switch typed := raw.(type) {
	case []map[string]interface{}:
		return append([]map[string]interface{}(nil), typed...), nil
	case []interface{}:
		items := make([]map[string]interface{}, 0, len(typed))
		for index, item := range typed {
			payload, ok := item.(map[string]interface{})
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

func toolDeclarationsForInput(input GenerateInput) ([]map[string]interface{}, []ToolDefinition, bool, error) {
	if input.DisableTools {
		return nil, nil, false, nil
	}
	providerTools, err := providerToolsFromOptions(input.Options)
	if err != nil {
		return nil, nil, false, err
	}
	return providerTools, input.Tools, true, nil
}

func providerStreamOptionsFromOptions(options map[string]interface{}) (map[string]interface{}, error) {
	if len(options) == 0 {
		return nil, nil
	}
	raw, ok := options["stream_options"]
	if !ok || raw == nil {
		return nil, nil
	}
	payload, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("model option stream_options must be an object")
	}
	return payload, nil
}

func appendToolDeclarations(payload map[string]interface{}, tools ...[]map[string]interface{}) {
	total := 0
	for _, group := range tools {
		total += len(group)
	}
	if total == 0 {
		return
	}
	merged := make([]map[string]interface{}, 0, total)
	if existing, ok := payload["tools"].([]map[string]interface{}); ok {
		merged = append(merged, existing...)
	}
	for _, group := range tools {
		merged = append(merged, group...)
	}
	if len(merged) > 0 {
		payload["tools"] = merged
	}
}

func appendResponseInclude(payload map[string]interface{}, values ...string) {
	if payload == nil {
		return
	}
	current := make([]string, 0)
	switch existing := payload["include"].(type) {
	case []string:
		current = append(current, existing...)
	case []interface{}:
		for _, item := range existing {
			if text, ok := item.(string); ok {
				current = append(current, text)
			}
		}
	}
	payload["include"] = appendUniqueStrings(current, values...)
}

func responseIncludeValues(options map[string]interface{}, defaults ...string) []string {
	values := make([]string, 0, len(defaults))
	values = append(values, defaults...)
	values = append(values, modelParamStringList(options, "include")...)
	return appendUniqueStrings(nil, values...)
}

func mergeObjectParam(payload map[string]interface{}, key string, values map[string]interface{}) {
	if payload == nil || len(values) == 0 {
		return
	}
	current, _ := payload[key].(map[string]interface{})
	if current == nil {
		current = map[string]interface{}{}
		payload[key] = current
	}
	for field, value := range values {
		if strings.TrimSpace(field) != "" {
			current[field] = value
		}
	}
}

func normalizedJSONResponseFormat(options map[string]interface{}) (interface{}, bool) {
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

func shouldSkipNormalizedProviderOption(key string, value interface{}) bool {
	if _, ok := value.(map[string]interface{}); ok {
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

// Usage 记录上游返回 token 使用量。
type Usage struct {
	InputTokens        int64
	OutputTokens       int64
	CacheReadTokens    int64
	CacheWriteTokens   int64
	CacheWrite5mTokens int64
	CacheWrite1hTokens int64
	ReasoningTokens    int64
	Speed              string
	ServiceTier        string
	RawUsageJSON       string
}

func nonCachedInputTokens(totalInputTokens int64, cacheReadTokens int64) int64 {
	if totalInputTokens <= 0 {
		return 0
	}
	if cacheReadTokens <= 0 {
		return totalInputTokens
	}
	remaining := totalInputTokens - cacheReadTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

func rawUsageJSONFromPath(payload map[string]interface{}, keys ...string) string {
	if len(payload) == 0 || len(keys) == 0 {
		return ""
	}
	var current interface{} = payload
	for _, key := range keys {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current, ok = currentMap[key]
		if !ok {
			return ""
		}
	}
	switch value := current.(type) {
	case map[string]interface{}:
		if len(value) == 0 {
			return ""
		}
	case []interface{}:
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

func MergeRawUsageJSON(left string, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" {
		return right
	}
	if right == "" || right == left {
		return left
	}
	items := make([]interface{}, 0, 2)
	items = appendRawUsageJSON(items, left)
	items = appendRawUsageJSON(items, right)
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		raw, err := json.Marshal(items[0])
		if err != nil {
			return ""
		}
		return string(raw)
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return ""
	}
	return string(raw)
}

func appendRawUsageJSON(items []interface{}, raw string) []interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return items
	}
	var decoded interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return items
	}
	switch value := decoded.(type) {
	case []interface{}:
		return append(items, value...)
	case map[string]interface{}:
		return append(items, value)
	default:
		return items
	}
}

// ToolCall 记录上游返回的工具调用请求。
type ToolCall struct {
	ToolCallID    string
	ToolType      string
	ToolName      string
	ArgumentsJSON string
	Status        string
	OutputJSON    string
	ErrorJSON     string
}

// ToolResult 记录工具执行结果，由各 adapter 序列化为对应 SDK/API 所需格式。
type ToolResult struct {
	ToolCallID string
	ToolName   string
	OutputJSON string
	Status     string
	Error      string
}

// ReasoningOutput 定义结构化 reasoning 输出。
type ReasoningOutput struct {
	ItemID           string
	Status           string
	Summary          string
	Text             string
	Signature        string
	EncryptedContent string
}

// GenerateOutput 定义上游推理结果。
type GenerateOutput struct {
	ResponseID          string
	Text                string
	Reasoning           *ReasoningOutput
	Usage               Usage
	ToolCalls           []ToolCall
	ServerToolCalls     []ToolCall
	ServerSideToolUsage map[string]int64
	Citations           []string
	GeneratedImages     []GeneratedImage
	RawJSON             string
	Debug               *UpstreamDebugSnapshot `json:"-"`
}

// GeneratedImage 表示图片生成/编辑接口返回的一张图片。
type GeneratedImage struct {
	URL           string
	B64JSON       string
	MIMEType      string
	RevisedPrompt string
}

// ReasoningDelta 定义流式 reasoning 增量。
type ReasoningDelta struct {
	EventType        string
	ItemID           string
	Status           string
	Kind             string
	Text             string
	Signature        string
	EncryptedContent string
}

// GenerateStreamEvent 定义上游流式增量片段。
type GenerateStreamEvent struct {
	Delta                 string
	Reasoning             *ReasoningDelta
	Usage                 Usage
	ServerToolCall        *ToolCall
	ResponseID            string
	GeneratedImage        *GeneratedImage
	GeneratedImageIndex   int
	GeneratedImagePartial bool
}

// ModelItem 定义上游模型目录项。
type ModelItem struct {
	ID      string
	OwnedBy string
}

// UpstreamError 是上游 HTTP 调用错误。
type UpstreamError struct {
	StatusCode int
	Message    string
	Body       string
	Debug      *UpstreamDebugSnapshot
}

var errStreamDone = errors.New("llm stream done")

// UpstreamDebugSnapshot 记录上游请求与响应的调试快照。
// 对外返回前必须先经过 application 层脱敏，避免泄漏源站、密钥或上游响应头。
type UpstreamDebugSnapshot struct {
	Request  UpstreamDebugRequest  `json:"request"`
	Response UpstreamDebugResponse `json:"response"`
}

// UpstreamDebugRequest 表示上游请求侧的调试信息。
type UpstreamDebugRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body"`
}

// UpstreamDebugResponse 表示上游响应侧的调试信息。
type UpstreamDebugResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body"`
}

func (e *UpstreamError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Sprintf("upstream request failed: status=%d", e.StatusCode)
	}
	return fmt.Sprintf("upstream request failed: status=%d message=%s", e.StatusCode, e.Message)
}

// NewClient 创建上游调用客户端。
func NewClient() *Client {
	return NewClientWithEnv("", false)
}

// NewClientWithEnv 创建带运行环境的上游调用客户端。
func NewClientWithEnv(env string, ssrfProtectionEnabled bool) *Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	client := &Client{
		baseTransport:         transport,
		env:                   strings.TrimSpace(env),
		ssrfProtectionEnabled: ssrfProtectionEnabled,
	}
	client.adapters = map[string]transportAdapter{
		AdapterOpenAIResponses:        &openAIResponsesAdapter{client: client},
		AdapterOpenRouterResponses:    &openRouterResponsesAdapter{client: client},
		AdapterOpenAIChatCompletions:  &openAIChatCompletionsAdapter{client: client},
		AdapterOpenAIImageGenerations: &openAIImageGenerationsAdapter{client: client},
		AdapterOpenAIImageEdits:       &openAIImageEditsAdapter{client: client},
		AdapterXAIResponses:           &xAIResponsesAdapter{client: client},
		AdapterXAIImage:               &xAIImageAdapter{client: client},
		AdapterXAIImageEdits:          &xAIImageEditsAdapter{client: client},
		AdapterAnthropicMessages:      &anthropicMessagesAdapter{client: client},
		AdapterGoogleGenerateContent:  &geminiGenerateContentAdapter{client: client},
		AdapterGoogleImageGeneration:  &geminiImageGenerationAdapter{client: client},
	}
	return client
}

func (c *Client) adapterFor(route RouteConfig) (transportAdapter, error) {
	adapterName := NormalizeAdapter(route.Protocol)
	if !IsImplementedAdapter(adapterName) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAdapter, adapterName)
	}
	adapter, ok := c.adapters[adapterName]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAdapter, adapterName)
	}
	return adapter, nil
}

func (c *Client) httpClientForRoute(route RouteConfig) *http.Client {
	connectTimeoutMS := normalizeConnectTimeoutMS(route.ConnectTimeoutMS)
	if value, ok := c.httpClients.Load(connectTimeoutMS); ok {
		if client, castOK := value.(*http.Client); castOK {
			return client
		}
	}

	created := c.newHTTPClient(connectTimeoutMS)
	actual, _ := c.httpClients.LoadOrStore(connectTimeoutMS, created)
	if client, ok := actual.(*http.Client); ok {
		return client
	}
	return created
}

func (c *Client) newHTTPClient(connectTimeoutMS int) *http.Client {
	transport := c.baseTransport.Clone()
	transport.DialContext = security.NewOutboundDialContext(c.env, c.ssrfProtectionEnabled, time.Duration(connectTimeoutMS)*time.Millisecond, 30*time.Second)

	return &http.Client{
		Timeout:   0,
		Transport: platformtracing.NewHTTPTransport(transport),
	}
}

// Generate 调用上游适配器并解析响应（非流式）。
func (c *Client) Generate(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error) {
	adapter, err := c.adapterFor(route)
	if err != nil {
		return nil, err
	}
	return adapter.Generate(ctx, route, input)
}

// GenerateStream 调用上游适配器并实时回传增量文本。
func (c *Client) GenerateStream(
	ctx context.Context,
	route RouteConfig,
	input GenerateInput,
	onEvent func(GenerateStreamEvent) error,
) (*GenerateOutput, error) {
	adapter, err := c.adapterFor(route)
	if err != nil {
		return nil, err
	}
	return adapter.GenerateStream(ctx, route, input, onEvent)
}

// ListModels 调用上游 models 目录接口。
func (c *Client) ListModels(ctx context.Context, route RouteConfig) ([]ModelItem, error) {
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
	fallbackRoute.Protocol = AdapterOpenAIChatCompletions
	fallbackItems, fallbackErr := c.listModelsOpenAICompatible(ctx, fallbackRoute)
	if fallbackErr != nil {
		return nil, fmt.Errorf("%w; openai-compatible models fallback failed: %v", err, fallbackErr)
	}
	return fallbackItems, nil
}

func shouldFallbackToOpenAICompatibleModels(route RouteConfig) bool {
	if isOpenRouterBaseURL(route.BaseURL) {
		return false
	}
	switch NormalizeAdapter(route.Protocol) {
	case AdapterAnthropicMessages, AdapterGoogleGenerateContent, AdapterGoogleImageGeneration:
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
	done    chan struct{}
	err     error
}

func newIdleTimeoutReader(reader io.Reader, timeout time.Duration) *idleTimeoutReader {
	return &idleTimeoutReader{
		reader:  reader,
		timeout: timeout,
		timer:   time.NewTimer(timeout),
		done:    make(chan struct{}),
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

func decodeToolSchema(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	}
	payload := make(map[string]interface{})
	if err := json.Unmarshal(raw, &payload); err != nil || len(payload) == 0 {
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	}
	if strings.TrimSpace(getString(payload["type"])) == "" {
		payload["type"] = "object"
	}
	if _, ok := payload["properties"]; !ok {
		payload["properties"] = map[string]interface{}{}
	}
	return payload
}

func NormalizeToolName(name string) string {
	value := strings.TrimSpace(name)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range value {
		valid := r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r)
		if valid {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	normalized := strings.Trim(builder.String(), "_-")
	if normalized == "" {
		return ""
	}
	first := []rune(normalized)[0]
	if !unicode.IsLetter(first) && first != '_' {
		normalized = "tool_" + normalized
	}
	if len(normalized) > 64 {
		normalized = normalized[:64]
	}
	return normalized
}

func buildToolResultContent(item ToolResult) string {
	if strings.TrimSpace(item.Error) != "" {
		payload := map[string]interface{}{
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

func normalizeMessages(messages []Message) []Message {
	normalized := make([]Message, 0, len(messages))
	for _, item := range messages {
		// 跳过空消息（Parts 非空或 Content 非空才保留）
		if len(item.Parts) == 0 && strings.TrimSpace(item.Content) == "" && len(item.ToolCalls) == 0 && len(item.ToolResults) == 0 {
			continue
		}
		normalized = append(normalized, Message{
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
		normalized = append(normalized, Message{Role: "user", Content: ""})
	}
	return normalized
}

func setAdditionalHeaders(req *http.Request, headersJSON string) {
	value := strings.TrimSpace(headersJSON)
	if value == "" {
		return
	}
	parsed := make(map[string]interface{})
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return
	}
	for key, rawValue := range parsed {
		headerKey := strings.TrimSpace(key)
		if headerKey == "" {
			continue
		}
		req.Header.Set(headerKey, stringify(rawValue))
	}
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

func parseUpstreamError(statusCode int, body []byte, debug *UpstreamDebugSnapshot) error {
	message := fmt.Sprintf("upstream_status_%d", statusCode)
	parsed := make(map[string]interface{})
	if err := json.Unmarshal(body, &parsed); err == nil {
		if m := getStringFromPath(parsed, "error", "message"); m != "" {
			message = m
		} else if m := getString(parsed["message"]); m != "" {
			message = m
		}
	}
	return &UpstreamError{
		StatusCode: statusCode,
		Message:    message,
		Body:       string(body),
		Debug:      debug,
	}
}

func upstreamDebugSnapshot(req *http.Request, requestBody []byte, resp *http.Response, responseBody []byte) *UpstreamDebugSnapshot {
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
	return &UpstreamDebugSnapshot{
		Request: UpstreamDebugRequest{
			Method:  req.Method,
			Path:    path,
			Headers: redactHeaders(req.Header),
			Body:    string(requestBody),
		},
		Response: UpstreamDebugResponse{
			StatusCode: responseStatusCode(resp),
			Headers:    responseHeaders(resp),
			Body:       string(responseBody),
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

func attachUpstreamDebug(err error, debug *UpstreamDebugSnapshot) error {
	if err == nil || debug == nil {
		return err
	}
	var upstreamErr *UpstreamError
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
	return &UpstreamError{
		StatusCode: statusCode,
		Message:    message,
		Body:       debug.Response.Body,
		Debug:      debug,
	}
}

func responseStatusCodeFromDebug(debug *UpstreamDebugSnapshot) int {
	if debug != nil && debug.Response.StatusCode >= 100 && debug.Response.StatusCode <= 599 {
		return debug.Response.StatusCode
	}
	return http.StatusBadGateway
}

func upstreamErrorBody(err error) []byte {
	var upstreamErr *UpstreamError
	if errors.As(err, &upstreamErr) && strings.TrimSpace(upstreamErr.Body) != "" {
		return []byte(upstreamErr.Body)
	}
	return nil
}

func parseStreamUpstreamError(parsed map[string]interface{}, rawBody string) error {
	errorPayload := asMap(parsed["error"])
	if len(errorPayload) == 0 {
		errorPayload = asMap(asMap(parsed["response"])["error"])
	}
	if len(errorPayload) == 0 {
		return nil
	}
	statusCode := streamErrorStatusCode(parsed, errorPayload)
	message := firstNonEmptyString(
		getString(errorPayload["message"]),
		getString(errorPayload["msg"]),
		getString(parsed["message"]),
		http.StatusText(statusCode),
	)
	return &UpstreamError{
		StatusCode: statusCode,
		Message:    message,
		Body:       rawBody,
	}
}

func streamErrorStatusCode(parsed map[string]interface{}, errorPayload map[string]interface{}) int {
	for _, raw := range []interface{}{
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

func toHTTPStatusCode(raw interface{}) int {
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

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func appendUniqueToolCall(items *[]ToolCall, item ToolCall) {
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

func shouldMergeToolCall(existing ToolCall, incoming ToolCall) bool {
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

func sameToolCallKind(left ToolCall, right ToolCall) bool {
	leftName := strings.TrimSpace(left.ToolName)
	rightName := strings.TrimSpace(right.ToolName)
	if leftName != "" && rightName != "" && leftName == rightName {
		return true
	}
	leftType := strings.TrimSpace(left.ToolType)
	rightType := strings.TrimSpace(right.ToolType)
	return leftType != "" && rightType != "" && leftType == rightType
}

func mergeToolCall(existing ToolCall, incoming ToolCall) ToolCall {
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

func updateToolCallInput(items *[]ToolCall, itemID string, input string, done bool) (ToolCall, bool) {
	if items == nil || strings.TrimSpace(itemID) == "" {
		return ToolCall{}, false
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
	return ToolCall{}, false
}

func appendUniqueStrings(items []string, values ...string) []string {
	seen := make(map[string]struct{}, len(items)+len(values))
	result := make([]string, 0, len(items)+len(values))
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

func extractReasoningDeltaText(raw interface{}) string {
	switch value := raw.(type) {
	case string:
		return value
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if text := extractReasoningDeltaText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	case map[string]interface{}:
		for _, key := range []string{"text", "delta", "thinking", "summary", "content"} {
			if text := extractReasoningDeltaText(value[key]); text != "" {
				return text
			}
		}
	}
	return ""
}

func extractContentText(raw interface{}) string {
	switch value := raw.(type) {
	case string:
		return value
	case []interface{}:
		chunks := make([]string, 0, len(value))
		for _, item := range value {
			if text := extractContentText(item); text != "" {
				chunks = append(chunks, text)
			}
		}
		return strings.Join(chunks, "")
	case map[string]interface{}:
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

func getStringFromPath(payload map[string]interface{}, keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	current := interface{}(payload)
	for _, key := range keys {
		node := asMap(current)
		if len(node) == 0 {
			return ""
		}
		current = node[key]
	}
	return strings.TrimSpace(getString(current))
}

func getInt64FromPath(payload map[string]interface{}, keys ...string) int64 {
	if len(keys) == 0 {
		return 0
	}
	current := interface{}(payload)
	for _, key := range keys {
		node := asMap(current)
		if len(node) == 0 {
			return 0
		}
		current = node[key]
	}
	return toInt64(current)
}

func firstMapItem(items []interface{}) map[string]interface{} {
	for _, item := range items {
		payload := asMap(item)
		if len(payload) > 0 {
			return payload
		}
	}
	return map[string]interface{}{}
}

func normalizeEndpoint(raw string) string {
	switch strings.TrimSpace(raw) {
	case EndpointChatCompletions:
		return EndpointChatCompletions
	case EndpointImageGenerations:
		return EndpointImageGenerations
	case EndpointImageEdits:
		return EndpointImageEdits
	default:
		return EndpointResponses
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

func asMap(raw interface{}) map[string]interface{} {
	if payload, ok := raw.(map[string]interface{}); ok {
		return payload
	}
	return map[string]interface{}{}
}

func cloneMap(raw map[string]interface{}) map[string]interface{} {
	if len(raw) == 0 {
		return map[string]interface{}{}
	}
	payload := make(map[string]interface{}, len(raw))
	for key, value := range raw {
		payload[key] = value
	}
	return payload
}

func mergeMapValueIfEmpty(payload map[string]interface{}, key string, value interface{}) {
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

func asSlice(raw interface{}) []interface{} {
	if payload, ok := raw.([]interface{}); ok {
		return payload
	}
	return []interface{}{}
}

func getString(raw interface{}) string {
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

func toInt64(raw interface{}) int64 {
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

func normalizeJSONString(raw interface{}) string {
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

func stringify(raw interface{}) string {
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
