package llm

import (
	"encoding/json"
	"strconv"
	"strings"
)

// ModelCaps 保存模型的上下文窗口与输出 Token 上限。
type ModelCaps struct {
	ContextWindow   int
	MaxOutputTokens int
}

// autocompactBufferTokens 是预留给系统开销与安全缓冲的 Token 数量。
// 参考 claude-code autoCompact.ts 中的 AUTOCOMPACT_BUFFER_TOKENS = 13_000。
const autocompactBufferTokens = 13_000

// GetModelCaps 返回已知模型的上下文窗口与输出 Token 上限。
// 对未知模型返回保守默认值（128k / 8k）。
func GetModelCaps(modelName string) ModelCaps {
	code := strings.ToLower(strings.TrimSpace(modelName))

	switch {
	// ── Anthropic Claude ──────────────────────────────────────────
	case strings.Contains(code, "claude-opus-4") || strings.Contains(code, "claude-sonnet-4"):
		// Claude 4 系列：1M 上下文，32k 输出
		return ModelCaps{ContextWindow: 1_000_000, MaxOutputTokens: 32_000}
	case strings.Contains(code, "claude-3-7") || strings.Contains(code, "claude-3.7"):
		return ModelCaps{ContextWindow: 200_000, MaxOutputTokens: 16_000}
	case strings.Contains(code, "claude-3-5") || strings.Contains(code, "claude-3.5"):
		return ModelCaps{ContextWindow: 200_000, MaxOutputTokens: 8_192}
	case strings.Contains(code, "claude-3") || strings.Contains(code, "claude"):
		return ModelCaps{ContextWindow: 200_000, MaxOutputTokens: 8_192}

	// ── OpenAI ────────────────────────────────────────────────────
	case strings.Contains(code, "gpt-4.1"):
		return ModelCaps{ContextWindow: 1_047_576, MaxOutputTokens: 32_768}
	case strings.Contains(code, "gpt-4.5"):
		return ModelCaps{ContextWindow: 128_000, MaxOutputTokens: 16_384}
	case strings.Contains(code, "gpt-4o"):
		return ModelCaps{ContextWindow: 128_000, MaxOutputTokens: 16_384}
	case strings.Contains(code, "gpt-4"):
		return ModelCaps{ContextWindow: 128_000, MaxOutputTokens: 4_096}
	case strings.Contains(code, "o4"), strings.Contains(code, "o3"), strings.Contains(code, "o1"):
		return ModelCaps{ContextWindow: 200_000, MaxOutputTokens: 100_000}
	case strings.Contains(code, "gpt-3.5"):
		return ModelCaps{ContextWindow: 16_385, MaxOutputTokens: 4_096}

	// ── Google Gemini ─────────────────────────────────────────────
	case strings.Contains(code, "gemini-2.5") || strings.Contains(code, "gemini-2.0"):
		return ModelCaps{ContextWindow: 1_000_000, MaxOutputTokens: 8_192}
	case strings.Contains(code, "gemini-1.5"):
		return ModelCaps{ContextWindow: 1_000_000, MaxOutputTokens: 8_192}
	case strings.Contains(code, "gemini"):
		return ModelCaps{ContextWindow: 128_000, MaxOutputTokens: 8_192}

	// ── xAI Grok ──────────────────────────────────────────────────
	case strings.Contains(code, "grok-3"):
		return ModelCaps{ContextWindow: 131_072, MaxOutputTokens: 16_384}
	case strings.Contains(code, "grok"):
		return ModelCaps{ContextWindow: 131_072, MaxOutputTokens: 8_192}

	// ── 默认（未知模型保守值）────────────────────────────────────
	default:
		return ModelCaps{ContextWindow: 128_000, MaxOutputTokens: 8_192}
	}
}

// GetModelCapsFromCapabilities 使用平台模型能力配置覆盖模型名推断值。
//
// 支持的能力字段：
// - contextWindow / context_window / contextWindowTokens / context_window_tokens
// - maxOutputTokens / max_output_tokens
func GetModelCapsFromCapabilities(modelName string, capabilitiesJSON string) ModelCaps {
	caps := GetModelCaps(modelName)
	raw := strings.TrimSpace(capabilitiesJSON)
	if raw == "" {
		return caps
	}

	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return caps
	}
	if value, ok := firstPositiveInt(payload, "contextWindow", "context_window", "contextWindowTokens", "context_window_tokens"); ok {
		caps.ContextWindow = value
	}
	if value, ok := firstPositiveInt(payload, "maxOutputTokens", "max_output_tokens"); ok {
		caps.MaxOutputTokens = value
	}
	return caps
}

func firstPositiveInt(payload map[string]interface{}, keys ...string) (int, bool) {
	for _, key := range keys {
		if value, ok := positiveInt(payload[key]); ok {
			return value, true
		}
	}
	return 0, false
}

func positiveInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case float64:
		if v > 0 {
			return int(v), true
		}
	case json.Number:
		if n, err := v.Int64(); err == nil && n > 0 {
			return int(n), true
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

// EffectiveContextBudget 返回上下文组装可用的最大 Token 数。
// = context_window - min(max_output, 20_000) - autocompact_buffer
func EffectiveContextBudget(modelName string) int {
	return effectiveContextBudget(GetModelCaps(modelName))
}

// EffectiveContextBudgetFromCapabilities 返回使用平台模型能力配置后的上下文预算。
func EffectiveContextBudgetFromCapabilities(modelName string, capabilitiesJSON string) int {
	return effectiveContextBudget(GetModelCapsFromCapabilities(modelName, capabilitiesJSON))
}

func effectiveContextBudget(caps ModelCaps) int {
	reserve := caps.MaxOutputTokens
	if reserve > 20_000 {
		reserve = 20_000
	}
	budget := caps.ContextWindow - reserve - autocompactBufferTokens
	if budget < 4_000 {
		budget = 4_000
	}
	return budget
}

// CompactionThreshold 返回触发上下文压缩的 Token 阈值（等于有效上下文预算）。
func CompactionThreshold(modelName string) int64 {
	return int64(EffectiveContextBudget(modelName))
}

// CompactionThresholdFromCapabilities 返回使用平台模型能力配置后的压缩阈值。
func CompactionThresholdFromCapabilities(modelName string, capabilitiesJSON string) int64 {
	return int64(EffectiveContextBudgetFromCapabilities(modelName, capabilitiesJSON))
}
