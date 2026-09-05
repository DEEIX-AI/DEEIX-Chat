package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
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
	// EndpointVideoGenerations 表示异步视频生成端点。
	EndpointVideoGenerations = "video_generations"
	// EndpointVideoExtensions 表示 xAI 异步视频扩展端点。
	EndpointVideoExtensions = "video_extensions"
	// EndpointInteractions 表示 Gemini Interactions API 端点。
	EndpointInteractions = "interactions"
)

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

// ContentPart 类型常量。
const (
	ContentPartText  = "text"  // 纯文本
	ContentPartImage = "image" // 图片（原始字节，序列化时 base64 编码）
	ContentPartVideo = "video" // 视频（原始字节，仅供支持视频输入的 adapter 使用）
	ContentPartFile  = "file"  // 文件提取文本（前端解析后注入）
)

// ContentPart 表示多模态消息中的一个内容片段。
type ContentPart struct {
	Kind         string        // text | image | video | file
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
	RequestID              string
	ConversationID         uint
	ConversationPublicID   string
	ConversationSessionKey string
	// PromptCacheKey 是 OpenAI prompt_cache_key 的服务端受控值，用户 Options 不得覆盖。
	PromptCacheKey string
	Messages       []Message
	// Instructions 承载可映射到上游原生指令字段的系统/开发者指令。
	// 不支持原生指令字段的 adapter 应继续通过 messages 承载系统提示。
	Instructions string
	Tools        []ToolDefinition
	// DisableTools 表示本轮调用必须只生成文本，adapter 不再声明 MCP 或厂商原生工具。
	DisableTools bool
	// Options 承载本次调用的自由 JSON 参数。系统字段（model/messages/input/stream）
	// 由 adapter 固定构造；Options 只表达采样、推理、工具、缓存和厂商原生扩展。
	Options map[string]any
	// PreviousResponseID 供 OpenAI Responses API 实现有状态会话。
	// 非空时：仅在 input 中发送本轮新消息，服务端从存储状态续接历史。
	// 空串时：退回全量发送模式，适用于所有 adapter。
	PreviousResponseID string
	// ResponsesBackground 表示官方 OpenAI Responses 请求使用 background mode。
	// 这是服务端能力开关，不从用户 Options 透传，避免改变未显式启用模型的数据保留语义。
	ResponsesBackground bool
	// Ephemeral 表示调用方要求无状态推理。支持该语义的 adapter 必须显式关闭
	// provider 侧响应存储，并忽略 background、previous response 与提示缓存状态。
	// 该字段只约束上游请求，不替代调用方自身的持久化边界。
	Ephemeral bool
	// ImageEditMask 仅供图片编辑 adapter 使用，表示透明区域掩码。
	ImageEditMask *ContentPart
	// VideoExtensionSource 仅供视频扩展 adapter 使用，表示待扩展的源视频。
	VideoExtensionSource *ContentPart
}

// ToolDefinition 是模型可调用工具的统一声明。
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// Usage 记录上游返回 token 使用量。InputTokens 是扣除缓存读取后的非缓存输入。
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

// HasObservedInput 表示上游是否上报了输入侧用量。提示词全部命中缓存时非缓存输入
// 合法为 0，因此判断"是否观测到输入"必须连同缓存读写一起看，不能只看 InputTokens。
func (u Usage) HasObservedInput() bool {
	return u.InputTokens > 0 || u.CacheReadTokens > 0 || u.CacheWriteTokens > 0
}

// HasObservedOutput 表示上游是否上报了输出侧用量。部分上游把思考 token 与可见输出分开上报，
// 因此任一侧大于 0 都算已观测；两者同时为 0 才需要按已产出文本预估。
func (u Usage) HasObservedOutput() bool {
	return u.OutputTokens > 0 || u.ReasoningTokens > 0
}

// MergeRawUsageJSON 合并两段上游原始 usage JSON，去重并保持数组语义。
func MergeRawUsageJSON(left string, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" {
		return right
	}
	if right == "" || right == left {
		return left
	}
	items := make([]any, 0, 2)
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

func appendRawUsageJSON(items []any, raw string) []any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return items
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return items
	}
	switch value := decoded.(type) {
	case []any:
		return append(items, value...)
	case map[string]any:
		return append(items, value)
	default:
		return items
	}
}

// ToolCall 记录上游返回的工具调用请求。
type ToolCall struct {
	ToolCallID       string
	ToolType         string
	ToolName         string
	ArgumentsJSON    string
	ThoughtSignature string
	Status           string
	OutputJSON       string
	ErrorJSON        string
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
	GeneratedVideos     []GeneratedVideo
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

// GeneratedVideo 表示视频生成接口返回的一个视频结果。
type GeneratedVideo struct {
	URL             string
	B64JSON         string
	MIMEType        string
	FileName        string
	DurationSeconds int64
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
	GeneratedImageIndex   int64
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
	Cause      error
}

func (e *UpstreamError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Sprintf("upstream request failed: status=%d", e.StatusCode)
	}
	return fmt.Sprintf("upstream request failed: status=%d message=%s", e.StatusCode, e.Message)
}

func (e *UpstreamError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// AcceptedRequestError 表示上游已接受请求，或请求已写出但结果未知。
// 生成请求不具备跨 Provider 幂等性，此类错误不得自动切换路由重试。
type AcceptedRequestError struct {
	cause error
}

func (e *AcceptedRequestError) Error() string {
	if e == nil || e.cause == nil {
		return "upstream request failed after acceptance"
	}
	return e.cause.Error()
}

func (e *AcceptedRequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// MarkRequestAccepted 标记错误发生时请求已被上游接受，或可能已被接受。
func MarkRequestAccepted(err error) error {
	if err == nil || RequestWasAccepted(err) {
		return err
	}
	return &AcceptedRequestError{cause: err}
}

// RequestWasAccepted 判断错误是否发生在请求已被或可能被上游接受之后。
func RequestWasAccepted(err error) bool {
	var acceptedErr *AcceptedRequestError
	return errors.As(err, &acceptedErr)
}

// UpstreamDebugSnapshot 记录上游请求与响应的调试快照。
// 对外返回前必须先经过 application 层脱敏，避免泄漏源站、密钥或上游响应头。
type UpstreamDebugSnapshot struct {
	Request  UpstreamDebugRequest  `json:"request"`
	Response UpstreamDebugResponse `json:"response"`
}

// UpstreamDebugRequest 表示上游请求侧的调试信息。
type UpstreamDebugRequest struct {
	Method        string            `json:"method"`
	Path          string            `json:"path"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          string            `json:"body"`
	BodyBytes     int               `json:"bodyBytes,omitempty"`
	BodyTruncated bool              `json:"bodyTruncated,omitempty"`
	RedactedParts int               `json:"redactedParts,omitempty"`
}

// UpstreamDebugResponse 表示上游响应侧的调试信息。
type UpstreamDebugResponse struct {
	StatusCode    int               `json:"statusCode"`
	Headers       map[string]string `json:"headers,omitempty"`
	Body          string            `json:"body"`
	BodyBytes     int               `json:"bodyBytes,omitempty"`
	BodyTruncated bool              `json:"bodyTruncated,omitempty"`
	RedactedParts int               `json:"redactedParts,omitempty"`
}

// NormalizeToolName 将任意工具名规范化为上游 API 接受的合法标识符。
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
