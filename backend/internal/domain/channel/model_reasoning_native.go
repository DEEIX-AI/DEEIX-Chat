package channel

import (
	"encoding/json"
	"math"
	"strings"
)

// ReasoningNativeSignal 是请求参数中各协议原生思考参数所表达的思考设置。
type ReasoningNativeSignal struct {
	// Format 为该原生参数对应的推理格式。
	Format string
	// Level 为能直接得出的规范档位；只有预算或只声明开启时为空。
	Level string
	// Budget 为原生预算值（HasBudget 为 true 时有效），-1 表示上游动态预算。
	Budget    int
	HasBudget bool
	// formatFlexible 为 true 时（如 Anthropic 关闭思考）格式可让位给目录能力的格式。
	formatFlexible bool
}

// DetectReasoningNativeSignal 从参数中识别协议原生思考参数（含适配器兼容的别名，路径清单与前端
// entities/model NATIVE_REASONING_PARAM_PATHS 保持一致）；没有任何可识别的思考参数时返回 false。
func DetectReasoningNativeSignal(protocolKey string, options map[string]any) (ReasoningNativeSignal, bool) {
	if len(options) == 0 {
		return ReasoningNativeSignal{}, false
	}
	read := func(path string) (any, bool) {
		return readCapabilityPath(options, strings.Split(path, "."))
	}
	text := func(paths ...string) (string, bool) {
		for _, path := range paths {
			if value, ok := read(path); ok {
				if typed, isString := value.(string); isString && strings.TrimSpace(typed) != "" {
					return strings.ToLower(strings.TrimSpace(typed)), true
				}
			}
		}
		return "", false
	}
	integer := func(paths ...string) (int, bool) {
		for _, path := range paths {
			if value, ok := read(path); ok {
				if parsed, isInteger := reasoningOptionInteger(value); isInteger {
					return parsed, true
				}
			}
		}
		return 0, false
	}
	boolean := func(path string) (bool, bool) {
		value, ok := read(path)
		typed, isBool := value.(bool)
		return typed, ok && isBool
	}
	level := func(format string, raw string) string {
		normalized, ok := NormalizeReasoningLevel(raw)
		if !ok || !reasoningFormatAllowsLevel(format, normalized) {
			return ""
		}
		return normalized
	}

	switch protocolKey {
	case ReasoningProtocolOpenAIChat, ReasoningProtocolOpenRouterChat:
		if effort, ok := text("reasoning_effort", "reasoning.effort"); ok {
			return ReasoningNativeSignal{Format: ReasoningFormatOpenAI, Level: level(ReasoningFormatOpenAI, effort)}, true
		}
		if enabled, ok := boolean("enable_thinking"); ok {
			signal := ReasoningNativeSignal{Format: ReasoningFormatQwen}
			if !enabled {
				signal.Level = ReasoningLevelNone
			} else if budget, hasBudget := integer("thinking_budget"); hasBudget {
				signal.Budget, signal.HasBudget = budget, true
			}
			return signal, true
		}
		if thinkingType, ok := text("thinking.type"); ok {
			switch thinkingType {
			case "disabled":
				return ReasoningNativeSignal{Format: ReasoningFormatToggle, Level: ReasoningLevelNone}, true
			case "enabled":
				return ReasoningNativeSignal{Format: ReasoningFormatToggle, Level: ReasoningLevelHigh}, true
			}
		}
	case ReasoningProtocolOpenAIResponses, ReasoningProtocolXAIResponses, ReasoningProtocolOpenRouterResponses:
		if effort, ok := text("reasoning.effort"); ok {
			return ReasoningNativeSignal{Format: ReasoningFormatOpenAI, Level: level(ReasoningFormatOpenAI, effort)}, true
		}
	case ReasoningProtocolAnthropic:
		thinkingType, hasType := text("thinking.type")
		enabled, hasEnabled := boolean("enable_thinking")
		if (hasType && thinkingType == "disabled") || (hasEnabled && !enabled) {
			return ReasoningNativeSignal{Format: ReasoningFormatAnthropicBudget, Level: ReasoningLevelNone, formatFlexible: true}, true
		}
		if effort, ok := text("output_config.effort", "effort"); ok {
			return ReasoningNativeSignal{Format: ReasoningFormatAnthropicEffort, Level: level(ReasoningFormatAnthropicEffort, effort)}, true
		}
		if budget, ok := integer("thinking.budget_tokens", "budget_tokens"); ok {
			return ReasoningNativeSignal{Format: ReasoningFormatAnthropicBudget, Budget: budget, HasBudget: true}, true
		}
		if hasType && thinkingType == "adaptive" {
			return ReasoningNativeSignal{Format: ReasoningFormatAnthropicEffort}, true
		}
		if (hasType && thinkingType == "enabled") || (hasEnabled && enabled) {
			return ReasoningNativeSignal{Format: ReasoningFormatAnthropicBudget}, true
		}
	case ReasoningProtocolGeminiGenerate:
		if thinkingLevel, ok := text("generationConfig.thinkingConfig.thinkingLevel", "thinkingConfig.thinkingLevel", "thinking_level", "thinkingLevel"); ok {
			return ReasoningNativeSignal{Format: ReasoningFormatGeminiLevel, Level: level(ReasoningFormatGeminiLevel, thinkingLevel)}, true
		}
		if budget, ok := integer("generationConfig.thinkingConfig.thinkingBudget", "thinkingConfig.thinkingBudget", "thinking_budget", "thinkingBudget", "budget_tokens"); ok {
			signal := ReasoningNativeSignal{Format: ReasoningFormatGeminiBudget, Budget: budget, HasBudget: true}
			if budget == 0 {
				signal.Level = ReasoningLevelNone
			}
			return signal, true
		}
	case ReasoningProtocolGeminiInteractions:
		if thinkingLevel, ok := text("generation_config.thinking_level"); ok {
			return ReasoningNativeSignal{Format: ReasoningFormatGeminiLevel, Level: level(ReasoningFormatGeminiLevel, thinkingLevel)}, true
		}
	}
	return ReasoningNativeSignal{}, false
}

// LevelFor 返回信号在能力中对应的规范档位：优先使用直接给出的档位，其次按预算取最接近的档位；无法得出时为空。
func (s ReasoningNativeSignal) LevelFor(capability *ReasoningCapability) string {
	if s.Level != "" {
		return s.Level
	}
	if s.HasBudget && capability != nil {
		return capability.LevelForBudget(s.Budget)
	}
	return ""
}

// LevelForBudget 按预算取最接近的非 none 档位，距离相同取较低档；预算为 0 且声明了 none 时返回 none。
// 能力没有预算时返回空。
func (c ReasoningCapability) LevelForBudget(budget int) string {
	if budget == 0 && c.HasLevel(ReasoningLevelNone) {
		return ReasoningLevelNone
	}
	if budget < 0 {
		return ""
	}
	best, bestDistance := "", math.MaxInt
	for _, level := range c.Levels {
		value, ok := c.Budgets[level]
		if level == ReasoningLevelNone || !ok {
			continue
		}
		distance := value - budget
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			best, bestDistance = level, distance
		}
	}
	return best
}

// reasoningNativeSignalFromControls 在 defaultOptions 没有思考参数时，从管理员配置的原生思考控件推断格式；
// 档位与默认值交由目录或标准档位决定。档位下拉控件由 inferReasoningCapabilityFromLevelControl 处理。
func reasoningNativeSignalFromControls(protocolKey string, payload map[string]any) (ReasoningNativeSignal, bool) {
	controls, _ := payload["optionControls"].([]any)
	for _, rawControl := range controls {
		control, ok := rawControl.(map[string]any)
		if !ok {
			continue
		}
		path, _ := control["path"].(string)
		switch protocolKey {
		case ReasoningProtocolOpenAIChat, ReasoningProtocolOpenRouterChat:
			switch strings.TrimSpace(path) {
			case "enable_thinking", "thinking_budget":
				return ReasoningNativeSignal{Format: ReasoningFormatQwen}, true
			case "thinking.type":
				return ReasoningNativeSignal{Format: ReasoningFormatToggle}, true
			}
		case ReasoningProtocolAnthropic:
			switch strings.TrimSpace(path) {
			case "thinking.budget_tokens", "budget_tokens", "enable_thinking", "thinking.type":
				return ReasoningNativeSignal{Format: ReasoningFormatAnthropicBudget, formatFlexible: true}, true
			}
		case ReasoningProtocolGeminiGenerate:
			switch strings.TrimSpace(path) {
			case "generationConfig.thinkingConfig.thinkingBudget", "thinkingConfig.thinkingBudget", "thinking_budget", "thinkingBudget":
				return ReasoningNativeSignal{Format: ReasoningFormatGeminiBudget}, true
			}
		}
	}
	return ReasoningNativeSignal{}, false
}

// standardReasoningLevels 是既无控件选项也无目录数据时各格式使用的保守档位。
func standardReasoningLevels(format string) []string {
	switch format {
	case ReasoningFormatGeminiLevel:
		return []string{ReasoningLevelLow, ReasoningLevelHigh}
	case ReasoningFormatQwen, ReasoningFormatToggle:
		if format == ReasoningFormatToggle {
			return []string{ReasoningLevelNone, ReasoningLevelHigh}
		}
		return []string{ReasoningLevelNone, ReasoningLevelLow, ReasoningLevelMedium, ReasoningLevelHigh}
	}
	return []string{ReasoningLevelLow, ReasoningLevelMedium, ReasoningLevelHigh}
}

// capabilityFromNativeSignal 由管理员原生思考参数构造能力：格式取自原生参数（可让位的信号优先采用目录格式）；
// 档位取目录（同格式时）或标准档位，并补入管理员给出的档位；默认档位取管理员的值；预算类格式中管理员给出的
// 预算原样用于对应档位。结果须通过显式声明的同一套校验，失败时返回 nil。
func capabilityFromNativeSignal(protocolKey string, signal ReasoningNativeSignal, catalog *ReasoningCapability) *ReasoningCapability {
	format := signal.Format
	if signal.formatFlexible && catalog != nil && ReasoningFormatSupportsProtocol(catalog.Format, protocolKey) {
		format = catalog.Format
	}
	var levels []string
	budgets := map[string]int{}
	if catalog != nil && catalog.Format == format {
		levels = append([]string(nil), catalog.Levels...)
		for level, budget := range catalog.Budgets {
			budgets[level] = budget
		}
	} else {
		levels = standardReasoningLevels(format)
		if reasoningFormatUsesBudgets(format) {
			for _, target := range reasoningCatalogBudgetTargets {
				budgets[target.level] = target.budget
			}
		}
	}
	probe := &ReasoningCapability{Format: format, Levels: levels, Budgets: budgets}
	defaultLevel := signal.LevelFor(probe)
	if defaultLevel != "" && !reasoningFormatAllowsLevel(format, defaultLevel) {
		defaultLevel = ""
	}
	if defaultLevel != "" && !probe.HasLevel(defaultLevel) {
		levels = append(levels, defaultLevel)
		sortReasoningLevels(levels)
	}
	if signal.HasBudget && defaultLevel != "" && defaultLevel != ReasoningLevelNone && reasoningFormatUsesBudgets(format) {
		budget := signal.Budget
		if format == ReasoningFormatAnthropicBudget && budget < AnthropicMinThinkingBudget {
			budget = AnthropicMinThinkingBudget
		}
		if budget >= 0 {
			budgets[defaultLevel] = budget
		}
	}
	if (format == ReasoningFormatQwen || format == ReasoningFormatGeminiBudget) && containsReasoningLevel(levels, ReasoningLevelNone) {
		budgets[ReasoningLevelNone] = 0
	}
	if defaultLevel == "" {
		defaultLevel = catalogReasoningDefault(levels)
	}
	payload := map[string]any{
		"format":  format,
		"levels":  reasoningLevelsPayload(levels),
		"default": defaultLevel,
	}
	if reasoningFormatUsesBudgets(format) {
		budgetPayload := make(map[string]any, len(budgets))
		for level, budget := range budgets {
			if containsReasoningLevel(levels, level) {
				budgetPayload[level] = float64(budget)
			}
		}
		payload["budgets"] = budgetPayload
	}
	capability, err := parseReasoningCapability(payload)
	if err != nil || !ReasoningFormatSupportsProtocol(capability.Format, protocolKey) {
		return nil
	}
	return capability
}

// applyNativeDefault 让 defaultOptions 中的管理员原生思考参数决定默认档位（管理员配置 > 声明的默认档）。
func applyNativeDefault(capability *ReasoningCapability, protocolKeys []string, payload map[string]any) {
	defaults, _ := payload["defaultOptions"].(map[string]any)
	for _, protocolKey := range protocolKeys {
		signal, ok := DetectReasoningNativeSignal(protocolKey, defaults)
		if !ok {
			continue
		}
		if level := signal.LevelFor(capability); level != "" && capability.HasLevel(level) {
			capability.Default = level
		}
		return
	}
}

func containsReasoningLevel(levels []string, level string) bool {
	for _, item := range levels {
		if item == level {
			return true
		}
	}
	return false
}

func reasoningOptionInteger(value any) (int, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil || parsed < -1 || parsed > math.MaxInt32 {
			return 0, false
		}
		return int(parsed), true
	case float64:
		if typed < -1 || typed > math.MaxInt32 || typed != math.Trunc(typed) {
			return 0, false
		}
		return int(typed), true
	case int:
		return typed, typed >= -1
	case int64:
		return int(typed), typed >= -1 && typed <= math.MaxInt32
	}
	return 0, false
}
