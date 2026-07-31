package conversation

import (
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

const (
	openAIPromptCacheCapabilityKey     = "promptCache"
	openAIPromptCacheEnabledCapability = "enabled"
	openAIPromptCacheOptionKey         = "prompt_cache_options"
)

// configureOpenAIPromptCacheForRoute 把路由能力收敛为上游请求所需的缓存键和选项。
// 官方 OpenAI 默认支持；兼容中转站必须在模型能力 JSON 中显式声明 promptCache.enabled=true。
func configureOpenAIPromptCacheForRoute(
	route *channel.ResolvedRoute,
	sessionID string,
	options map[string]interface{},
) (string, map[string]interface{}) {
	if supportsOpenAIPromptCacheRoute(route) {
		return strings.TrimSpace(sessionID), options
	}
	return "", withoutOpenAIPromptCacheOptions(options)
}

func supportsOpenAIPromptCacheRoute(route *channel.ResolvedRoute) bool {
	if route == nil {
		return false
	}
	switch llm.NormalizeAdapter(route.Protocol) {
	case llm.AdapterOpenAIChatCompletions, llm.AdapterOpenAIResponses:
	default:
		return false
	}

	capabilities := decodeModelCapabilities(route.ModelCapabilitiesJSON)
	if enabled, configured := openAIPromptCacheCapability(capabilities); configured {
		return enabled
	}
	return isOfficialOpenAIBaseURL(route.BaseURL)
}

func openAIPromptCacheCapability(capabilities map[string]interface{}) (bool, bool) {
	promptCache, ok := capabilities[openAIPromptCacheCapabilityKey].(map[string]interface{})
	if !ok {
		return false, false
	}
	enabled, ok := promptCache[openAIPromptCacheEnabledCapability].(bool)
	return enabled, ok
}

func withoutOpenAIPromptCacheOptions(options map[string]interface{}) map[string]interface{} {
	if _, exists := options[openAIPromptCacheOptionKey]; !exists {
		return options
	}
	filtered := cloneModelOptionMap(options)
	delete(filtered, openAIPromptCacheOptionKey)
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

// sanitizeOpenAIPromptCacheOptions 只接受管理员在模型能力中声明并锁定的显式缓存配置。
// 用户请求里的同名 options 不能开启或改变缓存写入策略。
func sanitizeOpenAIPromptCacheOptions(options map[string]interface{}, capabilitiesJSON string) {
	if len(options) == 0 {
		return
	}
	configured := lockedOpenAIPromptCacheOptions(capabilitiesJSON)
	if len(configured) == 0 {
		delete(options, openAIPromptCacheOptionKey)
		return
	}
	options[openAIPromptCacheOptionKey] = configured
}

func usesExplicitOpenAIPromptCache(options map[string]interface{}) bool {
	raw, ok := options[openAIPromptCacheOptionKey].(map[string]interface{})
	if !ok {
		return false
	}
	mode, ok := raw["mode"].(string)
	return ok && strings.EqualFold(strings.TrimSpace(mode), "explicit")
}

func lockedOpenAIPromptCacheOptions(capabilitiesJSON string) map[string]interface{} {
	defaults := modelCapabilityDefaultOptions(capabilitiesJSON)
	raw, ok := defaults[openAIPromptCacheOptionKey].(map[string]interface{})
	if !ok || !modelCapabilityLocksOptionPath(capabilitiesJSON, openAIPromptCacheOptionKey, "mode") {
		return nil
	}
	mode, ok := raw["mode"].(string)
	if !ok || !strings.EqualFold(strings.TrimSpace(mode), "explicit") {
		return nil
	}

	result := map[string]interface{}{"mode": "explicit"}
	if rawTTL, exists := raw["ttl"]; exists {
		ttl, ok := rawTTL.(string)
		if !ok || !strings.EqualFold(strings.TrimSpace(ttl), "30m") ||
			!modelCapabilityLocksOptionPath(capabilitiesJSON, openAIPromptCacheOptionKey, "ttl") {
			return nil
		}
		result["ttl"] = "30m"
	}
	return result
}

func modelCapabilityLocksOptionPath(capabilitiesJSON string, expected ...string) bool {
	for _, path := range modelCapabilityLockedOptionPaths(capabilitiesJSON) {
		if len(path) != len(expected) {
			continue
		}
		matches := true
		for index := range path {
			if path[index] != expected[index] {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}
