package conversation

import (
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

const (
	openAIPromptCacheCapabilityKey = "promptCache"
	openAIPromptCacheOptionKey     = "prompt_cache_options"
)

type openAIPromptCacheCapabilityConfig struct {
	Enabled                      bool
	EnabledConfigured            bool
	MessageBreakpoints           bool
	MessageBreakpointsConfigured bool
	Mode                         string
	TTL                          string
}

// configureOpenAIPromptCacheForRoute 把路由能力收敛为上游请求所需的缓存键和选项。
// 官方 OpenAI 默认支持；兼容中转站必须在模型能力 JSON 中显式声明 promptCache.enabled=true。
func configureOpenAIPromptCacheForRoute(
	route *channel.ResolvedRoute,
	sessionID string,
	options map[string]any,
) (string, map[string]any) {
	config, supported := resolveOpenAIPromptCacheRouteConfig(route)
	filtered := withoutOpenAIPromptCacheOptions(options)
	if supported {
		return strings.TrimSpace(sessionID), withOpenAIPromptCacheOptions(filtered, config)
	}
	return "", filtered
}

func configureOpenAIPromptCacheRequestForRoute(
	route *channel.ResolvedRoute,
	sessionID string,
	options map[string]any,
	messages []llm.Message,
) (string, map[string]any, []llm.Message) {
	key, configuredOptions := configureOpenAIPromptCacheForRoute(route, sessionID, options)
	configuredMessages := applyOpenAIPromptCacheMessagePolicy(route, configuredOptions, messages)
	return key, configuredOptions, configuredMessages
}

// applyOpenAIPromptCacheMessagePolicy 为显式 OpenAI 缓存保留累积历史断点。
// 当前 user 只在稳定段（文件 + 原文 + 图片）末尾标记，动态上下文始终不进入缓存边界。
func applyOpenAIPromptCacheMessagePolicy(
	route *channel.ResolvedRoute,
	options map[string]any,
	messages []llm.Message,
) []llm.Message {
	config, supported := resolveOpenAIPromptCacheRouteConfig(route)
	if !supported || !usesExplicitOpenAIPromptCache(options) {
		return messages
	}

	result := cloneLLMMessages(messages)
	for index := range result {
		result[index].CacheControl = nil
		if len(result[index].Parts) == 0 {
			continue
		}
		parts := append([]llm.ContentPart(nil), result[index].Parts...)
		for partIndex := range parts {
			parts[partIndex].CacheControl = nil
		}
		result[index].Parts = parts
	}
	if !config.MessageBreakpoints {
		return result
	}

	leadingSystemIndex := -1
	for index := range result {
		if !strings.EqualFold(strings.TrimSpace(result[index].Role), "system") {
			break
		}
		if openAIPromptCacheMessageNonempty(result[index]) {
			leadingSystemIndex = index
		}
	}
	if leadingSystemIndex >= 0 {
		result[leadingSystemIndex].CacheControl = &llm.CacheControl{Type: "ephemeral"}
	}

	currentUserIndex := -1
	for index := len(result) - 1; index >= 0; index-- {
		if strings.EqualFold(strings.TrimSpace(result[index].Role), "user") {
			currentUserIndex = index
			break
		}
	}
	if currentUserIndex < 0 {
		return result
	}

	for index := 0; index < currentUserIndex; index++ {
		if !strings.EqualFold(strings.TrimSpace(result[index].Role), "user") ||
			!openAIPromptCacheMessageNonempty(result[index]) {
			continue
		}
		result[index].CacheControl = &llm.CacheControl{Type: "ephemeral"}
	}
	// 历史 user 已全部标记，本轮只在可重现部分末尾补一个断点，不回退到助手消息。
	result, _ = markCurrentTurnCacheBoundary(result)
	return result
}

func openAIPromptCacheMessageNonempty(message llm.Message) bool {
	if strings.TrimSpace(message.Content) != "" {
		return true
	}
	for _, part := range message.Parts {
		if part.Kind == llm.ContentPartImage {
			if len(part.Data) > 0 {
				return true
			}
			continue
		}
		if strings.TrimSpace(part.Text) != "" {
			return true
		}
	}
	return false
}

func resolveOpenAIPromptCacheRouteConfig(route *channel.ResolvedRoute) (openAIPromptCacheCapabilityConfig, bool) {
	if route == nil {
		return openAIPromptCacheCapabilityConfig{}, false
	}
	switch llm.NormalizeAdapter(route.Protocol) {
	case llm.AdapterOpenAIChatCompletions, llm.AdapterOpenAIResponses:
	default:
		return openAIPromptCacheCapabilityConfig{}, false
	}

	config := openAIPromptCacheCapability(route.ModelCapabilitiesJSON)
	if !config.MessageBreakpointsConfigured {
		config.MessageBreakpoints = isOfficialOpenAIBaseURL(route.BaseURL)
	}
	if config.EnabledConfigured {
		return config, config.Enabled
	}
	return config, isOfficialOpenAIBaseURL(route.BaseURL)
}

func openAIPromptCacheCapability(capabilitiesJSON string) openAIPromptCacheCapabilityConfig {
	capabilities := decodeModelCapabilities(capabilitiesJSON)
	promptCache, ok := capabilities[openAIPromptCacheCapabilityKey].(map[string]any)
	if !ok {
		promptCache = nil
	}
	config := openAIPromptCacheCapabilityConfig{
		Mode: strings.ToLower(strings.TrimSpace(modelOptionStringValue(promptCache["mode"]))),
		TTL:  strings.ToLower(strings.TrimSpace(modelOptionStringValue(promptCache["ttl"]))),
	}
	config.Enabled, config.EnabledConfigured = promptCache["enabled"].(bool)
	config.MessageBreakpoints, config.MessageBreakpointsConfigured = promptCache["messageBreakpoints"].(bool)
	return config
}

func withoutOpenAIPromptCacheOptions(options map[string]any) map[string]any {
	_, hasOptions := options[openAIPromptCacheOptionKey]
	_, hasRetention := options["prompt_cache_retention"]
	if !hasOptions && !hasRetention {
		return options
	}
	filtered := cloneModelOptionMap(options)
	delete(filtered, openAIPromptCacheOptionKey)
	// 旧版模型选项可能仍包含此键。始终丢弃它，使
	// 服务端绝不将调用方控制的保留策略转发给上游。
	delete(filtered, "prompt_cache_retention")
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func withOpenAIPromptCacheOptions(options map[string]any, config openAIPromptCacheCapabilityConfig) map[string]any {
	result := cloneModelOptionMap(options)
	switch config.Mode {
	case "explicit":
		cacheOptions := map[string]any{"mode": "explicit"}
		if config.TTL == "30m" {
			cacheOptions["ttl"] = "30m"
		}
		if result == nil {
			result = make(map[string]any)
		}
		result[openAIPromptCacheOptionKey] = cacheOptions
	}
	return result
}

func usesExplicitOpenAIPromptCache(options map[string]any) bool {
	raw, ok := options[openAIPromptCacheOptionKey].(map[string]any)
	if !ok {
		return false
	}
	mode, ok := raw["mode"].(string)
	return ok && strings.EqualFold(strings.TrimSpace(mode), "explicit")
}

func usesExplicitOpenAIPromptCacheMessageBreakpoints(
	route *channel.ResolvedRoute,
	options map[string]any,
) bool {
	if !usesExplicitOpenAIPromptCache(options) {
		return false
	}
	config, supported := resolveOpenAIPromptCacheRouteConfig(route)
	return supported && config.MessageBreakpoints
}
