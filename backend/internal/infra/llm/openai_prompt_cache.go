package llm

import (
	"errors"
	"strings"
)

const (
	openAIPromptCacheModeExplicit = "explicit"
	openAIPromptCacheTTL30Minutes = "30m"
	openAIMaxCacheBreakpoints     = 4
)

type openAIPromptCacheConfig struct {
	Key             string
	Options         map[string]interface{}
	Explicit        bool
	BreakpointsLeft int
}

func resolveOpenAIPromptCacheConfig(adapter string, input GenerateInput) openAIPromptCacheConfig {
	config := openAIPromptCacheConfig{BreakpointsLeft: openAIMaxCacheBreakpoints}
	if input.DisablePromptCache || !isOpenAITextAdapter(adapter) {
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

func hasOpenAIPromptCacheHint(messages []Message) bool {
	for _, message := range messages {
		if message.CacheControl != nil {
			return true
		}
		for _, part := range message.Parts {
			if part.CacheControl != nil {
				return true
			}
		}
	}
	return false
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
	if block == nil || hint == nil || config == nil || !config.Explicit || config.BreakpointsLeft <= 0 ||
		!openAIContentBlockSupportsPromptCacheBreakpoint(block) {
		return false
	}
	if _, exists := block["prompt_cache_breakpoint"]; exists {
		return false
	}
	block["prompt_cache_breakpoint"] = map[string]interface{}{"mode": openAIPromptCacheModeExplicit}
	config.BreakpointsLeft--
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

func shouldRetryWithoutOpenAIPromptCache(input GenerateInput, err error) bool {
	if input.DisablePromptCache || !openAIPromptCacheRequested(input) {
		return false
	}
	var upstreamErr *UpstreamError
	if !errors.As(err, &upstreamErr) || (upstreamErr.StatusCode != 400 && upstreamErr.StatusCode != 422) {
		return false
	}
	detail := strings.ToLower(strings.TrimSpace(upstreamErr.Message + " " + upstreamErr.Body))
	return strings.Contains(detail, "prompt_cache_key") ||
		strings.Contains(detail, "prompt_cache_options") ||
		strings.Contains(detail, "prompt_cache_breakpoint")
}

func openAIPromptCacheRequested(input GenerateInput) bool {
	return strings.TrimSpace(input.PromptCacheKey) != "" ||
		len(modelParamMap(input.Options, "prompt_cache_options")) > 0 ||
		hasOpenAIPromptCacheHint(input.Messages)
}

func disableOpenAIPromptCache(input GenerateInput) GenerateInput {
	input.DisablePromptCache = true
	input.PromptCacheKey = ""
	input.Options = cloneMap(input.Options)
	delete(input.Options, "prompt_cache_options")
	return input
}
