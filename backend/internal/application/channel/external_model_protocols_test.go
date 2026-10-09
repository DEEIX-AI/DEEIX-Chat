package channel

import (
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestSuggestExternalModelProtocols(t *testing.T) {
	cases := []struct {
		provider, model, want string
	}{
		{llm.AdapterOpenAIChatCompletions, "gpt-4o", llm.AdapterOpenAIChatCompletions},
		{llm.AdapterOpenAIResponses, "gpt-image-1", llm.AdapterOpenAIImageGenerations + "," + llm.AdapterOpenAIImageEdits},
		{llm.AdapterOpenAIChatCompletions, "dall-e-3", llm.AdapterOpenAIImageGenerations},
		{llm.AdapterXAIResponses, "grok-4", llm.AdapterXAIResponses},
		{llm.AdapterXAIResponses, "grok-imagine-image", llm.AdapterXAIImage + "," + llm.AdapterXAIImageEdits},
		{llm.AdapterXAIResponses, "grok-imagine-video", llm.AdapterXAIVideo + "," + llm.AdapterXAIVideoExtensions},
		{llm.AdapterGoogleGenerateContent, "gemini-3-pro-image", llm.AdapterGoogleImageGeneration},
		{llm.AdapterOpenRouterResponses, "gemini-2.5-flash-image", llm.AdapterOpenRouterImages},
		// 中转站的厂商前缀不影响识别。
		{llm.AdapterOpenRouterChat, "google/gemini-2.5-flash-image", llm.AdapterOpenRouterImages},
		{llm.AdapterOpenAIChatCompletions, "openai/gpt-image-1", llm.AdapterOpenAIImageGenerations + "," + llm.AdapterOpenAIImageEdits},
		// OpenAI 兼容的中转站上，xAI 媒体模型按模型名走 xAI 协议，对话模型仍用服务协议。
		{llm.AdapterOpenAIChatCompletions, "grok-imagine-image-2.0", llm.AdapterXAIImage + "," + llm.AdapterXAIImageEdits},
		{llm.AdapterOpenAIChatCompletions, "grok-imagine-video", llm.AdapterXAIVideo + "," + llm.AdapterXAIVideoExtensions},
		{llm.AdapterOpenAIChatCompletions, "grok-4.7", llm.AdapterOpenAIChatCompletions},
		// 没有实现的协议（OpenAI 视频）回退到服务协议，由用户自行修改。
		{llm.AdapterOpenAIChatCompletions, "sora", llm.AdapterOpenAIChatCompletions},
		{llm.AdapterAnthropicMessages, "gpt-image-1", llm.AdapterAnthropicMessages},
	}
	for _, check := range cases {
		if got := strings.Join(SuggestExternalModelProtocols(check.provider, check.model), ","); got != check.want {
			t.Errorf("%s on %s = %q, want %q", check.model, check.provider, got, check.want)
		}
	}
}

func TestExternalModelKindsFollowTheProtocols(t *testing.T) {
	cases := []struct {
		model     string
		protocols []string
		want      string
	}{
		{"gpt-4o", []string{llm.AdapterOpenAIChatCompletions}, "chat"},
		{"gpt-image-1", []string{llm.AdapterOpenAIImageGenerations, llm.AdapterOpenAIImageEdits}, "image_gen,image_edit"},
		{"relay-draw", []string{llm.AdapterOpenAIImageGenerations}, "image_gen"},
		{"relay-video", []string{llm.AdapterXAIVideo, llm.AdapterXAIVideoExtensions}, "video_gen,video_extension"},
		{"gemini-3-pro", []string{llm.AdapterGeminiInteractions}, "chat"},
	}
	for _, check := range cases {
		if got := strings.Join(externalModelKinds(check.model, check.protocols), ","); got != check.want {
			t.Errorf("%s %v kinds = %q, want %q", check.model, check.protocols, got, check.want)
		}
	}
}
