package llm

import (
	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// 数据契约定义在 ports/llm，此处保留同名引用供传输层实现使用。

// 端点标识。
const (
	EndpointResponses        = portllm.EndpointResponses
	EndpointChatCompletions  = portllm.EndpointChatCompletions
	EndpointImageGenerations = portllm.EndpointImageGenerations
	EndpointImageEdits       = portllm.EndpointImageEdits
	EndpointVideoGenerations = portllm.EndpointVideoGenerations
	EndpointVideoExtensions  = portllm.EndpointVideoExtensions
	EndpointInteractions     = portllm.EndpointInteractions
)

// 协议标识。
const (
	AdapterOpenAIResponses        = portllm.AdapterOpenAIResponses
	AdapterOpenRouterChat         = portllm.AdapterOpenRouterChat
	AdapterOpenRouterResponses    = portllm.AdapterOpenRouterResponses
	AdapterOpenAIChatCompletions  = portllm.AdapterOpenAIChatCompletions
	AdapterOpenAIImageGenerations = portllm.AdapterOpenAIImageGenerations
	AdapterOpenAIImageEdits       = portllm.AdapterOpenAIImageEdits
	AdapterAnthropicMessages      = portllm.AdapterAnthropicMessages
	AdapterGoogleGenerateContent  = portllm.AdapterGoogleGenerateContent
	AdapterGoogleImageGeneration  = portllm.AdapterGoogleImageGeneration
	AdapterGeminiInteractions     = portllm.AdapterGeminiInteractions
	AdapterXAIResponses           = portllm.AdapterXAIResponses
	AdapterXAIImage               = portllm.AdapterXAIImage
	AdapterXAIImageEdits          = portllm.AdapterXAIImageEdits
	AdapterXAIVideo               = portllm.AdapterXAIVideo
	AdapterXAIVideoExtensions     = portllm.AdapterXAIVideoExtensions
)

// 消息内容片段类型。
const (
	ContentPartText  = portllm.ContentPartText
	ContentPartImage = portllm.ContentPartImage
	ContentPartVideo = portllm.ContentPartVideo
	ContentPartFile  = portllm.ContentPartFile
)

var (
	// ErrUnsupportedAdapter 表示协议没有可用适配器实现。
	ErrUnsupportedAdapter = portllm.ErrUnsupportedAdapter
	// ErrUnsupportedStream 表示协议存在但不支持真实流式输出。
	ErrUnsupportedStream = portllm.ErrUnsupportedStream
)

// 请求/响应数据模型。
type (
	RouteConfig           = portllm.RouteConfig
	ContentPart           = portllm.ContentPart
	CacheControl          = portllm.CacheControl
	Message               = portllm.Message
	GenerateInput         = portllm.GenerateInput
	ToolDefinition        = portllm.ToolDefinition
	Usage                 = portllm.Usage
	ToolCall              = portllm.ToolCall
	ToolResult            = portllm.ToolResult
	ReasoningOutput       = portllm.ReasoningOutput
	GenerateOutput        = portllm.GenerateOutput
	GeneratedImage        = portllm.GeneratedImage
	GeneratedVideo        = portllm.GeneratedVideo
	ReasoningDelta        = portllm.ReasoningDelta
	GenerateStreamEvent   = portllm.GenerateStreamEvent
	ModelItem             = portllm.ModelItem
	UpstreamError         = portllm.UpstreamError
	AcceptedRequestError  = portllm.AcceptedRequestError
	UpstreamDebugSnapshot = portllm.UpstreamDebugSnapshot
	UpstreamDebugRequest  = portllm.UpstreamDebugRequest
	UpstreamDebugResponse = portllm.UpstreamDebugResponse
)

// NormalizeAdapter 规范化协议名。
func NormalizeAdapter(raw string) string { return portllm.NormalizeAdapter(raw) }

// IsKnownAdapter 返回协议是否为已知值（含未实现的）。
func IsKnownAdapter(raw string) bool { return portllm.IsKnownAdapter(raw) }

// IsImplementedAdapter 返回协议是否已有可用的传输层实现。
func IsImplementedAdapter(raw string) bool { return portllm.IsImplementedAdapter(raw) }

// SupportsStreamingAdapter 返回协议是否有真实的上游流式传输。
func SupportsStreamingAdapter(raw string) bool { return portllm.SupportsStreamingAdapter(raw) }

// SupportsImageGenerationStream 返回图片媒体协议和模型是否支持真实上游流式。
func SupportsImageGenerationStream(protocol string, model string) bool {
	return portllm.SupportsImageGenerationStream(protocol, model)
}

// IsImageGenerationAdapter 返回协议是否属于独立图片生成链路。
func IsImageGenerationAdapter(raw string) bool { return portllm.IsImageGenerationAdapter(raw) }

// IsImageEditAdapter 返回协议是否属于独立图片编辑链路。
func IsImageEditAdapter(raw string) bool { return portllm.IsImageEditAdapter(raw) }

// IsVideoGenerationAdapter 返回协议是否属于独立视频生成链路。
func IsVideoGenerationAdapter(raw string) bool { return portllm.IsVideoGenerationAdapter(raw) }

// DefaultEndpointForAdapter 返回协议对应的固定端点标识。
func DefaultEndpointForAdapter(adapter string) string {
	return portllm.DefaultEndpointForAdapter(adapter)
}

// SupportsPreviousResponseID 返回协议是否支持 previous_response_id 有状态续接。
func SupportsPreviousResponseID(adapter string) bool {
	return portllm.SupportsPreviousResponseID(adapter)
}

// MergeRawUsageJSON 合并两段上游原始 usage JSON。
func MergeRawUsageJSON(left string, right string) string {
	return portllm.MergeRawUsageJSON(left, right)
}

// NormalizeToolName 将任意工具名规范化为上游 API 接受的合法标识符。
func NormalizeToolName(name string) string { return portllm.NormalizeToolName(name) }

// MarkRequestAccepted 标记错误发生时请求已被上游接受，或可能已被接受。
func MarkRequestAccepted(err error) error { return portllm.MarkRequestAccepted(err) }

// RequestWasAccepted 判断错误是否发生在请求已被或可能被上游接受之后。
func RequestWasAccepted(err error) bool { return portllm.RequestWasAccepted(err) }

// SanitizeUpstreamDebugBody 对调试体做脱敏与限长。
func SanitizeUpstreamDebugBody(raw string) string { return portllm.SanitizeUpstreamDebugBody(raw) }

// SanitizeXAIVideoOptions 将 xAI 视频协议参数收敛为实际会上送的规范值。
func SanitizeXAIVideoOptions(options map[string]interface{}) {
	portllm.SanitizeXAIVideoOptions(options)
}

// SanitizeXAIVideoExtensionOptions 仅保留 xAI 视频扩展支持的时长参数。
func SanitizeXAIVideoExtensionOptions(options map[string]interface{}) {
	portllm.SanitizeXAIVideoExtensionOptions(options)
}
