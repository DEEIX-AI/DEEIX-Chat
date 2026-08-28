package llm

import "strings"

const (
	openAIPromptCacheModeExplicit = "explicit"
	openAIPromptCacheTTL30Minutes = "30m"
)

type openAIPromptCacheConfig struct {
	Key      string
	Options  map[string]interface{}
	Explicit bool
}

func resolveOpenAIPromptCacheConfig(adapter string, input GenerateInput) openAIPromptCacheConfig {
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
	adapter = NormalizeAdapter(adapter)
	return adapter == AdapterOpenAIResponses || adapter == AdapterOpenAIChatCompletions
}

func normalizedOpenAIPromptCacheOptions(options map[string]interface{}) map[string]interface{} {
	raw := modelParamMap(options, "prompt_cache_options")
	if len(raw) == 0 || !strings.EqualFold(strings.TrimSpace(getString(raw["mode"])), openAIPromptCacheModeExplicit) {
		return nil
	}
	result := map[string]interface{}{"mode": openAIPromptCacheModeExplicit}
	if rawTTL, exists := raw["ttl"]; exists {
		ttl, ok := rawTTL.(string)
		if !ok || strings.ToLower(strings.TrimSpace(ttl)) != openAIPromptCacheTTL30Minutes {
			return nil
		}
		result["ttl"] = openAIPromptCacheTTL30Minutes
	}
	return result
}

func applyOpenAIPromptCacheRequestFields(payload map[string]interface{}, config openAIPromptCacheConfig) {
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

func appendOpenAIPromptCacheBreakpoint(block map[string]interface{}, hint *CacheControl, config *openAIPromptCacheConfig) bool {
	if block == nil || hint == nil || config == nil || !config.Explicit ||
		!openAIContentBlockSupportsPromptCacheBreakpoint(block) {
		return false
	}
	if _, exists := block["prompt_cache_breakpoint"]; exists {
		return false
	}
	block["prompt_cache_breakpoint"] = map[string]interface{}{"mode": openAIPromptCacheModeExplicit}
	return true
}

func openAIContentBlockSupportsPromptCacheBreakpoint(block map[string]interface{}) bool {
	switch strings.TrimSpace(getString(block["type"])) {
	case "input_text", "input_image", "input_file", "text", "image_url", "input_audio", "file", "refusal":
		return true
	default:
		return false
	}
}
