package channel

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

var ErrInvalidModelCapsOverride = errors.New("invalid model capability limits")

const (
	defaultContextWindow    = 128_000
	defaultMaxOutputTokens  = 8_192
	autocompactBufferTokens = 13_000
)

// ModelCaps 保存模型的上下文窗口与输出 Token 上限。
type ModelCaps struct {
	ContextWindow   int
	MaxOutputTokens int
}

// ModelCapsSource 描述模型能力值的来源，用于运行日志与决策诊断。
type ModelCapsSource string

const (
	ModelCapsSourceCapabilities ModelCapsSource = "capabilities_override"
	ModelCapsSourceCatalog      ModelCapsSource = "builtin_catalog"
	ModelCapsSourceFallback     ModelCapsSource = "fallback"
)

// ResolvedModelCaps 是包含来源信息的模型能力解析结果。
type ResolvedModelCaps struct {
	ModelCaps
	Source ModelCapsSource
}

type modelCapsRule struct {
	patterns []string
	caps     ModelCaps
}

// modelCapsCatalog 仅保存能够稳定按模型族识别的保守值。
// 精确型号优先于通用型号；动态目录（例如 OpenRouter）只作为管理端建议，
// 不在请求链路上改变运行时能力。
var modelCapsCatalog = []modelCapsRule{
	{patterns: []string{"claude-sonnet-3.7", "claude-3.7"}, caps: ModelCaps{200_000, 16_000}},
	{patterns: []string{"claude-sonnet-3.5", "claude-haiku-3.5", "claude-3.5"}, caps: ModelCaps{200_000, 8_192}},
	{patterns: []string{"gpt-4.1"}, caps: ModelCaps{1_047_576, 32_768}},
	{patterns: []string{"gpt-4.5"}, caps: ModelCaps{128_000, 16_384}},
	{patterns: []string{"gpt-4o"}, caps: ModelCaps{128_000, 16_384}},
	{patterns: []string{"o1", "o3", "o4"}, caps: ModelCaps{200_000, 100_000}},
	{patterns: []string{"gpt-3.5"}, caps: ModelCaps{16_385, 4_096}},
	{patterns: []string{"gemini-2.0", "gemini-2.5"}, caps: ModelCaps{1_000_000, 8_192}},
	{patterns: []string{"gemini-1.5"}, caps: ModelCaps{1_000_000, 8_192}},
	{patterns: []string{"grok-3"}, caps: ModelCaps{131_072, 16_384}},
}

// ResolveModelCapsWithFallback 返回模型能力及其来源，并允许调用方配置未知模型的回退窗口。
// 显式能力配置和内置目录始终优先于回退值。
func ResolveModelCapsWithFallback(modelName string, fallbackContextWindow int) ResolvedModelCaps {
	code := strings.ToLower(strings.TrimSpace(modelName))
	for _, rule := range modelCapsCatalog {
		for _, pattern := range rule.patterns {
			if containsModelPattern(code, pattern) {
				return ResolvedModelCaps{
					ModelCaps: rule.caps,
					Source:    ModelCapsSourceCatalog,
				}
			}
		}
	}
	fallbackContextWindow = normalizeFallbackContextWindow(fallbackContextWindow)
	return ResolvedModelCaps{
		ModelCaps: ModelCaps{ContextWindow: fallbackContextWindow, MaxOutputTokens: defaultMaxOutputTokens},
		Source:    ModelCapsSourceFallback,
	}
}

func normalizeFallbackContextWindow(value int) int {
	if value < 4_096 || value > 16_000_000 {
		return defaultContextWindow
	}
	return value
}

// containsModelPattern tolerates cosmetic separators while preserving model
// identity and numeric versions. For example, gpt-4.1 and gpt_4_1 match, but
// gpt-4.10 does not; claude-sonnet-4.5 and 4.6 remain distinct.
func containsModelPattern(code string, pattern string) bool {
	if code == "" || pattern == "" {
		return false
	}
	patternSignature := modelIdentifierSignature(pattern)
	if len(patternSignature) == 0 {
		return false
	}
	for _, segment := range strings.FieldsFunc(strings.ToLower(code), func(value rune) bool {
		return value == '/' || value == ':'
	}) {
		segmentSignature := modelIdentifierSignature(segment)
		if len(segmentSignature) < len(patternSignature) {
			continue
		}
		matched := true
		for index := range patternSignature {
			if segmentSignature[index] != patternSignature[index] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func modelIdentifierSignature(value string) []string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return nil
	}
	result := make([]string, 0, 6)
	letters := strings.Builder{}
	digits := strings.Builder{}
	flushLetters := func() {
		if letters.Len() == 0 {
			return
		}
		result = append(result, letters.String())
		letters.Reset()
	}
	flushDigits := func() {
		if digits.Len() == 0 {
			return
		}
		result = append(result, digits.String())
		digits.Reset()
	}
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
			flushDigits()
			letters.WriteRune(character)
		case character >= '0' && character <= '9':
			flushLetters()
			digits.WriteRune(character)
		default:
			// Cosmetic separators may join words, but they delimit numeric
			// version components so 4.1 never collapses into 41.
			flushDigits()
		}
	}
	flushLetters()
	flushDigits()
	return result
}

// ResolveModelCapsFromCapabilitiesWithFallback 将平台模型能力配置覆盖在目录和回退值之上。
func ResolveModelCapsFromCapabilitiesWithFallback(modelName string, capabilitiesJSON string, fallbackContextWindow int) ResolvedModelCaps {
	resolved := ResolveModelCapsWithFallback(modelName, fallbackContextWindow)
	payload, ok := parseCapabilities(capabilitiesJSON)
	if !ok {
		return resolved
	}
	overridden := false
	if value, found := firstPositiveInt(payload, "contextWindow", "context_window", "contextWindowTokens", "context_window_tokens"); found {
		resolved.ContextWindow = value
		overridden = true
	}
	if value, found := firstPositiveInt(payload, "maxOutputTokens", "max_output_tokens"); found {
		resolved.MaxOutputTokens = value
		overridden = true
	}
	if overridden {
		resolved.Source = ModelCapsSourceCapabilities
	}
	return resolved
}

// ValidateModelCapsOverrides 校验显式能力覆盖，避免无效或异常大数值进入请求预算计算。
// 未配置相关字段时不干涉其他能力配置。
func ValidateModelCapsOverrides(capabilitiesJSON string) error {
	payload, ok := parseCapabilities(capabilitiesJSON)
	if !ok {
		return nil
	}
	contextWindow, hasContext := firstPresentInt(payload, "contextWindow", "context_window", "contextWindowTokens", "context_window_tokens")
	if hasContext && (contextWindow < 4_096 || contextWindow > 16_000_000) {
		return ErrInvalidModelCapsOverride
	}
	maxOutput, hasOutput := firstPresentInt(payload, "maxOutputTokens", "max_output_tokens")
	if hasOutput && (maxOutput < 1 || maxOutput > 1_000_000) {
		return ErrInvalidModelCapsOverride
	}
	if hasContext && hasOutput && maxOutput >= contextWindow {
		return ErrInvalidModelCapsOverride
	}
	return nil
}

func firstPresentInt(payload map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, exists := payload[key]
		if !exists {
			continue
		}
		parsed, ok := positiveInt(value)
		if !ok {
			return 0, true
		}
		return parsed, true
	}
	return 0, false
}

func parseCapabilities(raw string) (map[string]any, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	payload := map[string]any{}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, false
	}
	return payload, true
}

func firstPositiveInt(payload map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		if value, ok := positiveInt(payload[key]); ok {
			return value, true
		}
	}
	return 0, false
}

func positiveInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		if v > 0 && v <= float64(^uint(0)>>1) {
			return int(v), true
		}
	case json.Number:
		if n, err := v.Int64(); err == nil && n > 0 && int64(int(n)) == n {
			return int(n), true
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

func EffectiveContextBudgetFromCapabilitiesWithFallback(modelName string, capabilitiesJSON string, fallbackContextWindow int) int {
	return effectiveContextBudget(ResolveModelCapsFromCapabilitiesWithFallback(modelName, capabilitiesJSON, fallbackContextWindow).ModelCaps)
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

// CompactionThresholdFromCapabilitiesWithFallback 按有效输入预算的比例计算主动压缩阈值。
// 0 表示关闭按 Token 触发；超过 100 的值按 100 处理，避免越过硬预算。
func CompactionThresholdFromCapabilitiesWithFallback(modelName string, capabilitiesJSON string, fallbackContextWindow int, triggerPercent int) int64 {
	if triggerPercent <= 0 {
		return 0
	}
	if triggerPercent > 100 {
		triggerPercent = 100
	}
	budget := int64(EffectiveContextBudgetFromCapabilitiesWithFallback(modelName, capabilitiesJSON, fallbackContextWindow))
	threshold := budget * int64(triggerPercent) / 100
	if threshold < 4_000 {
		return 4_000
	}
	return threshold
}
