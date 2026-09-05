package llm

import (
	"strings"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

const (
	openAIPromptCacheModeExplicit = "explicit"
	openAIPromptCacheTTL30Minutes = "30m"
)

type openAIPromptCacheConfig struct {
	Key      string
	Options  map[string]any
	Explicit bool
}

func resolveOpenAIPromptCacheConfig(adapter string, input portllm.GenerateInput) openAIPromptCacheConfig {
	config := openAIPromptCacheConfig{}
	if input.Ephemeral || !isOpenAITextAdapter(adapter) {
		return config
	}
	config.Key = strings.TrimSpace(input.PromptCacheKey)
	config.Options = normalizedOpenAIPromptCacheOptions(input.Options)
	config.Explicit = strings.EqualFold(strings.TrimSpace(getString(config.Options["mode"])), openAIPromptCacheModeExplicit)
	return config
}

func isOpenAITextAdapter(adapter string) bool {
	adapter = portllm.NormalizeAdapter(adapter)
	return adapter == portllm.AdapterOpenAIResponses || adapter == portllm.AdapterOpenAIChatCompletions
}

func normalizedOpenAIPromptCacheOptions(options map[string]any) map[string]any {
	raw := modelParamMap(options, "prompt_cache_options")
	if len(raw) == 0 || !strings.EqualFold(strings.TrimSpace(getString(raw["mode"])), openAIPromptCacheModeExplicit) {
		return nil
	}
	result := map[string]any{"mode": openAIPromptCacheModeExplicit}
	if rawTTL, exists := raw["ttl"]; exists {
		ttl, ok := rawTTL.(string)
		if !ok || strings.ToLower(strings.TrimSpace(ttl)) != openAIPromptCacheTTL30Minutes {
			return nil
		}
		result["ttl"] = openAIPromptCacheTTL30Minutes
	}
	return result
}

func applyOpenAIPromptCacheRequestFields(payload map[string]any, config openAIPromptCacheConfig) {
	if payload == nil {
		return
	}
	if config.Key != "" {
		payload["prompt_cache_key"] = config.Key
	}
	if len(config.Options) > 0 {
		payload["prompt_cache_options"] = cloneMap(config.Options)
	}
}

func appendOpenAIPromptCacheBreakpoint(block map[string]any, hint *portllm.CacheControl, config *openAIPromptCacheConfig) bool {
	if block == nil || hint == nil || config == nil || !config.Explicit ||
		!openAIContentBlockSupportsPromptCacheBreakpoint(block) {
		return false
	}
	if _, exists := block["prompt_cache_breakpoint"]; exists {
		return false
	}
	block["prompt_cache_breakpoint"] = map[string]any{"mode": openAIPromptCacheModeExplicit}
	return true
}

func openAIContentBlockSupportsPromptCacheBreakpoint(block map[string]any) bool {
	switch strings.TrimSpace(getString(block["type"])) {
	case "input_text", "input_image", "input_file", "text", "image_url", "input_audio", "file", "refusal":
		return true
	default:
		return false
	}
}
