// Package llm 定义 LLM 上游调用端口的数据契约：
// 协议与端点词汇表、请求/响应数据模型、以及跨层复用的纯函数。
package llm

import (
	"errors"
	"strings"
)

// 已支持的协议常量。每个协议固定对应一个 HTTP 端点，任务能力由模型类别和路由规则约束。
const (
	AdapterOpenAIResponses        = "openai_responses"            // POST /v1/responses
	AdapterOpenRouterChat         = "openrouter_chat_completions" // POST /v1/chat/completions（OpenRouter）
	AdapterOpenRouterResponses    = "openrouter_responses"        // POST /v1/responses（OpenRouter Responses Beta）
	AdapterOpenAIChatCompletions  = "openai_chat_completions"     // POST /v1/chat/completions
	AdapterOpenAIImageGenerations = "openai_image_generations"    // POST /v1/images/generations
	AdapterOpenAIImageEdits       = "openai_image_edits"          // POST /v1/images/edits
	AdapterAnthropicMessages      = "anthropic_messages"          // POST /v1/messages
	AdapterGoogleGenerateContent  = "google_generate_content"     // POST /v1beta/models/{model}:generateContent
	AdapterGoogleImageGeneration  = "google_image_generation"     // POST /v1beta/models/{model}:generateContent
	AdapterGeminiInteractions     = "gemini_interactions"         // POST /v1beta/interactions
	AdapterXAIResponses           = "xai_responses"               // POST /v1/responses（OpenAI 兼容）
	AdapterXAIImage               = "xai_image"                   // POST /v1/images/generations
	AdapterXAIImageEdits          = "xai_image_edits"             // POST /v1/images/edits
	AdapterXAIVideo               = "xai_video"                   // POST /v1/videos/generations + GET /v1/videos/{request_id}
	AdapterXAIVideoExtensions     = "xai_video_extensions"        // POST /v1/videos/extensions + GET /v1/videos/{request_id}
)

var (
	// ErrUnsupportedAdapter 表示协议没有可用适配器实现。
	ErrUnsupportedAdapter = errors.New("unsupported llm adapter")
	// ErrUnsupportedStream 表示协议存在但不支持真实流式输出。
	ErrUnsupportedStream = errors.New("unsupported llm stream")
)

// NormalizeAdapter 规范化协议名；空值按历史默认使用 openai_responses，未知值保留给校验层处理。
func NormalizeAdapter(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return AdapterOpenAIResponses
	}
	return value
}

// IsKnownAdapter 返回协议是否为已知值（含未实现的）。
func IsKnownAdapter(raw string) bool {
	switch NormalizeAdapter(raw) {
	case AdapterOpenAIResponses,
		AdapterOpenRouterChat,
		AdapterOpenRouterResponses,
		AdapterOpenAIChatCompletions,
		AdapterOpenAIImageGenerations,
		AdapterOpenAIImageEdits,
		AdapterAnthropicMessages,
		AdapterGoogleGenerateContent,
		AdapterGoogleImageGeneration,
		AdapterGeminiInteractions,
		AdapterXAIResponses,
		AdapterXAIImage,
		AdapterXAIImageEdits,
		AdapterXAIVideo,
		AdapterXAIVideoExtensions:
		return true
	default:
		return false
	}
}

// IsImplementedAdapter 返回协议是否已有可用的传输层实现。
func IsImplementedAdapter(raw string) bool {
	switch NormalizeAdapter(raw) {
	case AdapterOpenAIResponses, AdapterOpenRouterChat, AdapterOpenRouterResponses, AdapterOpenAIChatCompletions, AdapterOpenAIImageGenerations, AdapterOpenAIImageEdits, AdapterXAIResponses,
		AdapterAnthropicMessages, AdapterGoogleGenerateContent, AdapterGoogleImageGeneration, AdapterGeminiInteractions, AdapterXAIImage, AdapterXAIImageEdits, AdapterXAIVideo, AdapterXAIVideoExtensions:
		return true
	default:
		return false
	}
}

// SupportsStreamingAdapter 返回协议是否有真实的上游流式传输。
func SupportsStreamingAdapter(raw string) bool {
	switch NormalizeAdapter(raw) {
	case AdapterOpenAIResponses,
		AdapterOpenRouterChat,
		AdapterOpenRouterResponses,
		AdapterOpenAIChatCompletions,
		AdapterOpenAIImageGenerations,
		AdapterOpenAIImageEdits,
		AdapterAnthropicMessages,
		AdapterGoogleGenerateContent,
		AdapterGoogleImageGeneration,
		AdapterGeminiInteractions,
		AdapterXAIResponses:
		return true
	default:
		return false
	}
}

// SupportsImageGenerationStream 返回图片媒体协议和模型是否支持真实上游流式。
func SupportsImageGenerationStream(protocol string, model string) bool {
	switch NormalizeAdapter(protocol) {
	case AdapterOpenAIImageGenerations:
		return openAIImageGenerationModelSupportsStream(model)
	case AdapterGoogleImageGeneration:
		return true
	case AdapterGeminiInteractions:
		return true
	case AdapterOpenAIImageEdits:
		return openAIImageEditModelSupportsStream(model)
	default:
		return false
	}
}

func openAIImageGenerationModelSupportsStream(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-image-")
}

func openAIImageEditModelSupportsStream(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(normalized, "gpt-image-") || normalized == "chatgpt-image-latest"
}

// IsImageGenerationAdapter 返回协议是否属于独立图片生成链路。
func IsImageGenerationAdapter(raw string) bool {
	switch NormalizeAdapter(raw) {
	case AdapterOpenAIImageGenerations, AdapterGoogleImageGeneration, AdapterGeminiInteractions, AdapterXAIImage:
		return true
	default:
		return false
	}
}

// IsImageEditAdapter 返回协议是否属于独立图片编辑链路。
func IsImageEditAdapter(raw string) bool {
	switch NormalizeAdapter(raw) {
	case AdapterOpenAIImageEdits, AdapterGoogleImageGeneration, AdapterGeminiInteractions, AdapterXAIImageEdits:
		return true
	default:
		return false
	}
}

// IsVideoGenerationAdapter 返回协议是否属于独立视频生成链路。
func IsVideoGenerationAdapter(raw string) bool {
	switch NormalizeAdapter(raw) {
	case AdapterGeminiInteractions, AdapterXAIVideo, AdapterXAIVideoExtensions:
		return true
	default:
		return false
	}
}

// DefaultEndpointForAdapter 返回协议对应的固定端点标识。
func DefaultEndpointForAdapter(adapter string) string {
	switch NormalizeAdapter(adapter) {
	case AdapterOpenAIChatCompletions, AdapterOpenRouterChat:
		return EndpointChatCompletions
	case AdapterOpenAIImageGenerations, AdapterGoogleImageGeneration, AdapterXAIImage:
		return EndpointImageGenerations
	case AdapterOpenAIImageEdits, AdapterXAIImageEdits:
		return EndpointImageEdits
	case AdapterXAIVideo:
		return EndpointVideoGenerations
	case AdapterXAIVideoExtensions:
		return EndpointVideoExtensions
	case AdapterGeminiInteractions:
		return EndpointInteractions
	default:
		// openai_responses、openrouter_responses、xai_responses 及所有未知值均使用 Responses 端点。
		return EndpointResponses
	}
}

// SupportsPreviousResponseID 返回协议是否明确支持 previous_response_id 有状态续接。
// 兼容/逆向实现即使复用 Responses 形状，也不一定支持该字段；默认保持关闭。
func SupportsPreviousResponseID(adapter string) bool {
	return NormalizeAdapter(adapter) == AdapterOpenAIResponses
}
