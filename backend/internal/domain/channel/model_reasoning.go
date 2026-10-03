package channel

import (
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
)

// 推理强度规范档位，按由低到高排列。请求参数 options.reasoning_effort 只接受这些取值，
// 由后端按模型能力翻译成各协议的原生参数。
const (
	ReasoningLevelNone    = "none"
	ReasoningLevelMinimal = "minimal"
	ReasoningLevelLow     = "low"
	ReasoningLevelMedium  = "medium"
	ReasoningLevelHigh    = "high"
	ReasoningLevelXHigh   = "xhigh"
	ReasoningLevelMax     = "max"
)

// 推理能力声明的原生映射格式。
const (
	ReasoningFormatOpenAI          = "openai"
	ReasoningFormatAnthropicEffort = "anthropic_effort"
	ReasoningFormatAnthropicBudget = "anthropic_budget"
	ReasoningFormatGeminiLevel     = "gemini_level"
	ReasoningFormatGeminiBudget    = "gemini_budget"
	ReasoningFormatQwen            = "qwen"
	ReasoningFormatToggle          = "toggle"
)

// 推理能力关心的请求协议键，与模型参数策略使用的协议键保持一致。
const (
	ReasoningProtocolOpenAIChat          = "openai_chat_completions"
	ReasoningProtocolOpenRouterChat      = "openrouter_chat_completions"
	ReasoningProtocolOpenAIResponses     = "openai_responses"
	ReasoningProtocolXAIResponses        = "xai_responses"
	ReasoningProtocolOpenRouterResponses = "openrouter_responses"
	ReasoningProtocolAnthropic           = "anthropic_messages"
	ReasoningProtocolGeminiGenerate      = "gemini_generate_content"
	ReasoningProtocolGeminiInteractions  = "gemini_interactions"
)

// AnthropicMinThinkingBudget 是 Anthropic extended thinking 允许的最小 budget_tokens。
const AnthropicMinThinkingBudget = 1024

var (
	// ErrInvalidReasoningFormat 推理能力的 format 未知或缺失。
	ErrInvalidReasoningFormat = errors.New("invalid reasoning capability format")
	// ErrInvalidReasoningLevels 推理档位为空、重复、非规范值或不被该 format 支持。
	ErrInvalidReasoningLevels = errors.New("invalid reasoning capability levels")
	// ErrInvalidReasoningDefault 默认档位不在声明的档位内。
	ErrInvalidReasoningDefault = errors.New("invalid reasoning capability default")
	// ErrInvalidReasoningBudgets 推理预算缺失、非整数、为负或低于协议下限。
	ErrInvalidReasoningBudgets = errors.New("invalid reasoning capability budgets")
)

var reasoningLevelOrder = []string{
	ReasoningLevelNone,
	ReasoningLevelMinimal,
	ReasoningLevelLow,
	ReasoningLevelMedium,
	ReasoningLevelHigh,
	ReasoningLevelXHigh,
	ReasoningLevelMax,
}

// ReasoningCapability 是归一化后的模型推理强度能力，显式声明与旧配置推断的结果行为一致。
type ReasoningCapability struct {
	Format string
	// Levels 按规范顺序排列且不重复。
	Levels  []string
	Default string
	// Budgets 仅对预算类格式有意义，键为规范档位。
	Budgets map[string]int
	// ControlPath 仅在由旧版 optionControls 推断时非空，表示被规范参数接管的原生参数路径。
	ControlPath string
}

// ReasoningLevelRank 返回规范档位的序号；非规范值返回 -1。
func ReasoningLevelRank(level string) int {
	for index, item := range reasoningLevelOrder {
		if item == level {
			return index
		}
	}
	return -1
}

// NormalizeReasoningLevel 把任意请求值归一化为规范档位；非字符串或非规范值返回 false。
func NormalizeReasoningLevel(value any) (string, bool) {
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	level := strings.ToLower(strings.TrimSpace(text))
	if ReasoningLevelRank(level) < 0 {
		return "", false
	}
	return level, true
}

// HasLevel 判断能力是否声明了指定档位。
func (c ReasoningCapability) HasLevel(level string) bool {
	for _, item := range c.Levels {
		if item == level {
			return true
		}
	}
	return false
}

// ResolveLevel 把请求档位收敛到声明档位：非规范值回退默认档位；未声明的档位按规范顺序取最近的
// 声明档位，距离相同取较低档；请求 none 但未声明时自然落到最低声明档位。
func (c ReasoningCapability) ResolveLevel(requested string) string {
	if len(c.Levels) == 0 {
		return ""
	}
	level, ok := NormalizeReasoningLevel(requested)
	if !ok {
		return c.Default
	}
	if c.HasLevel(level) {
		return level
	}
	target := ReasoningLevelRank(level)
	best := c.Levels[0]
	bestDistance := math.MaxInt
	for _, item := range c.Levels {
		distance := ReasoningLevelRank(item) - target
		if distance < 0 {
			distance = -distance
		}
		// Levels 按规范顺序遍历，严格小于即可保证距离相同时取较低档。
		if distance < bestDistance {
			best = item
			bestDistance = distance
		}
	}
	return best
}

// ReasoningFormatSupportsProtocol 判断推理格式能否翻译到指定协议；不兼容时请求链路不做翻译。
func ReasoningFormatSupportsProtocol(format string, protocolKey string) bool {
	switch format {
	case ReasoningFormatOpenAI:
		switch protocolKey {
		case ReasoningProtocolOpenAIChat, ReasoningProtocolOpenRouterChat,
			ReasoningProtocolOpenAIResponses, ReasoningProtocolXAIResponses, ReasoningProtocolOpenRouterResponses:
			return true
		}
	case ReasoningFormatAnthropicEffort, ReasoningFormatAnthropicBudget:
		return protocolKey == ReasoningProtocolAnthropic
	case ReasoningFormatGeminiLevel:
		return protocolKey == ReasoningProtocolGeminiGenerate || protocolKey == ReasoningProtocolGeminiInteractions
	case ReasoningFormatGeminiBudget:
		return protocolKey == ReasoningProtocolGeminiGenerate
	case ReasoningFormatQwen, ReasoningFormatToggle:
		return protocolKey == ReasoningProtocolOpenAIChat || protocolKey == ReasoningProtocolOpenRouterChat
	}
	return false
}

// ReasoningNativeLevelPath 返回格式在协议上以档位字符串表达的原生参数路径，用于兼容旧客户端直接
// 传原生参数；预算类与开关类格式没有档位路径，返回空。
func ReasoningNativeLevelPath(format string, protocolKey string) string {
	if !ReasoningFormatSupportsProtocol(format, protocolKey) {
		return ""
	}
	switch format {
	case ReasoningFormatOpenAI, ReasoningFormatAnthropicEffort, ReasoningFormatGeminiLevel:
		return legacyReasoningControlPath(protocolKey)
	}
	return ""
}

// legacyReasoningControlPath 返回各协议旧版推理强度下拉控件的原生路径。
func legacyReasoningControlPath(protocolKey string) string {
	switch protocolKey {
	case ReasoningProtocolOpenAIChat, ReasoningProtocolOpenRouterChat:
		return "reasoning_effort"
	case ReasoningProtocolOpenAIResponses, ReasoningProtocolXAIResponses, ReasoningProtocolOpenRouterResponses:
		return "reasoning.effort"
	case ReasoningProtocolAnthropic:
		return "output_config.effort"
	case ReasoningProtocolGeminiGenerate:
		return "generationConfig.thinkingConfig.thinkingLevel"
	case ReasoningProtocolGeminiInteractions:
		return "generation_config.thinking_level"
	}
	return ""
}

func legacyReasoningFormat(protocolKey string) string {
	switch protocolKey {
	case ReasoningProtocolOpenAIChat, ReasoningProtocolOpenRouterChat,
		ReasoningProtocolOpenAIResponses, ReasoningProtocolXAIResponses, ReasoningProtocolOpenRouterResponses:
		return ReasoningFormatOpenAI
	case ReasoningProtocolAnthropic:
		return ReasoningFormatAnthropicEffort
	case ReasoningProtocolGeminiGenerate, ReasoningProtocolGeminiInteractions:
		return ReasoningFormatGeminiLevel
	}
	return ""
}

// ResolveReasoningCapability 是请求链路与模型目录共用的唯一解析入口，返回模型在指定协议上生效的
// 推理能力及其来源。优先级：显式声明 > 管理员原生思考参数推断（档位控件、defaultOptions、原生思考控件）>
// models.dev 目录（catalog 为调用方按同一协议预先推导的能力，可为 nil）> 无。显式声明存在时即使无效或与
// 协议不兼容也不再回退，返回 nil 与来源 explicit，保持管理员配置的最终决定权；defaultOptions 中的原生
// 思考参数会覆盖声明的默认档位。
func ResolveReasoningCapability(protocolKey string, capabilitiesJSON string, catalog *ReasoningCapability) (*ReasoningCapability, string) {
	payload, _ := parseCapabilities(capabilitiesJSON)
	if raw, declared := explicitReasoningPayload(payload); declared {
		capability, err := parseReasoningCapability(raw)
		if err != nil || !ReasoningFormatSupportsProtocol(capability.Format, protocolKey) {
			return nil, ReasoningSourceExplicit
		}
		applyNativeDefault(capability, []string{protocolKey}, payload)
		return capability, ReasoningSourceExplicit
	}
	if capability := inferReasoningCapability(protocolKey, payload, catalog); capability != nil {
		return capability, ReasoningSourceInferred
	}
	if catalog != nil && ReasoningFormatSupportsProtocol(catalog.Format, protocolKey) {
		return catalog.clone(), ReasoningSourceCatalog
	}
	return nil, ""
}

// ResolveReasoningCapabilityForProtocols 为协议无关的模型目录解析推理能力，优先级与
// ResolveReasoningCapability 一致：显式声明只要与任一可用协议兼容（或尚无路由）即生效；否则按协议
// 顺序取第一个可推断的旧配置；最后使用目录能力（须与任一可用协议兼容）。
func ResolveReasoningCapabilityForProtocols(protocolKeys []string, capabilitiesJSON string, catalog *ReasoningCapability) (*ReasoningCapability, string) {
	payload, _ := parseCapabilities(capabilitiesJSON)
	if raw, declared := explicitReasoningPayload(payload); declared {
		capability, err := parseReasoningCapability(raw)
		if err != nil {
			return nil, ReasoningSourceExplicit
		}
		if len(protocolKeys) == 0 || reasoningFormatSupportsAnyProtocol(capability.Format, protocolKeys) {
			applyNativeDefault(capability, protocolKeys, payload)
			return capability, ReasoningSourceExplicit
		}
		return nil, ReasoningSourceExplicit
	}
	for _, protocolKey := range protocolKeys {
		protocolCatalog := catalog
		if protocolCatalog != nil && !ReasoningFormatSupportsProtocol(protocolCatalog.Format, protocolKey) {
			protocolCatalog = nil
		}
		if capability := inferReasoningCapability(protocolKey, payload, protocolCatalog); capability != nil {
			return capability, ReasoningSourceInferred
		}
	}
	if catalog != nil && reasoningFormatSupportsAnyProtocol(catalog.Format, protocolKeys) {
		return catalog.clone(), ReasoningSourceCatalog
	}
	return nil, ""
}

func reasoningFormatSupportsAnyProtocol(format string, protocolKeys []string) bool {
	for _, protocolKey := range protocolKeys {
		if ReasoningFormatSupportsProtocol(format, protocolKey) {
			return true
		}
	}
	return false
}

// clone 返回不与调用方共享切片与映射的副本，目录能力可能被多次复用。
func (c *ReasoningCapability) clone() *ReasoningCapability {
	if c == nil {
		return nil
	}
	copied := *c
	copied.Levels = append([]string(nil), c.Levels...)
	if c.Budgets != nil {
		copied.Budgets = make(map[string]int, len(c.Budgets))
		for level, budget := range c.Budgets {
			copied.Budgets[level] = budget
		}
	}
	return &copied
}

// validateReasoningCapabilityOverride 校验能力 JSON 中显式声明的 reasoning 字段；未声明时不干涉。
func validateReasoningCapabilityOverride(payload map[string]any) error {
	raw, declared := explicitReasoningPayload(payload)
	if !declared {
		return nil
	}
	_, err := parseReasoningCapability(raw)
	return err
}

func explicitReasoningPayload(payload map[string]any) (any, bool) {
	raw, exists := payload["reasoning"]
	if !exists || raw == nil {
		return nil, false
	}
	return raw, true
}

// parseReasoningCapability 校验并归一化显式推理能力声明。
func parseReasoningCapability(raw any) (*ReasoningCapability, error) {
	payload, ok := raw.(map[string]any)
	if !ok {
		return nil, ErrInvalidReasoningFormat
	}
	format, _ := payload["format"].(string)
	format = strings.TrimSpace(format)
	if !isKnownReasoningFormat(format) {
		return nil, ErrInvalidReasoningFormat
	}
	levels, err := parseReasoningLevels(format, payload["levels"])
	if err != nil {
		return nil, err
	}
	capability := &ReasoningCapability{Format: format, Levels: levels}
	defaultLevel, ok := NormalizeReasoningLevel(payload["default"])
	if !ok || !capability.HasLevel(defaultLevel) {
		return nil, ErrInvalidReasoningDefault
	}
	capability.Default = defaultLevel
	budgets, err := parseReasoningBudgets(format, levels, payload["budgets"])
	if err != nil {
		return nil, err
	}
	capability.Budgets = budgets
	return capability, nil
}

func isKnownReasoningFormat(format string) bool {
	switch format {
	case ReasoningFormatOpenAI, ReasoningFormatAnthropicEffort, ReasoningFormatAnthropicBudget,
		ReasoningFormatGeminiLevel, ReasoningFormatGeminiBudget, ReasoningFormatQwen, ReasoningFormatToggle:
		return true
	}
	return false
}

// reasoningFormatAllowsLevel 返回格式可声明的档位：anthropic_effort 没有 minimal，
// gemini_level 只有 minimal..high，其余格式允许全部规范档位。
func reasoningFormatAllowsLevel(format string, level string) bool {
	switch format {
	case ReasoningFormatAnthropicEffort:
		return level != ReasoningLevelMinimal
	case ReasoningFormatGeminiLevel:
		switch level {
		case ReasoningLevelMinimal, ReasoningLevelLow, ReasoningLevelMedium, ReasoningLevelHigh:
			return true
		}
		return false
	}
	return ReasoningLevelRank(level) >= 0
}

func parseReasoningLevels(format string, raw any) ([]string, error) {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		return nil, ErrInvalidReasoningLevels
	}
	seen := make(map[string]struct{}, len(items))
	levels := make([]string, 0, len(items))
	for _, item := range items {
		level, ok := NormalizeReasoningLevel(item)
		if !ok || !reasoningFormatAllowsLevel(format, level) {
			return nil, ErrInvalidReasoningLevels
		}
		if _, duplicated := seen[level]; duplicated {
			return nil, ErrInvalidReasoningLevels
		}
		seen[level] = struct{}{}
		levels = append(levels, level)
	}
	sortReasoningLevels(levels)
	if format == ReasoningFormatToggle && (len(levels) != 2 || levels[0] != ReasoningLevelNone) {
		return nil, ErrInvalidReasoningLevels
	}
	return levels, nil
}

func sortReasoningLevels(levels []string) {
	sort.SliceStable(levels, func(left, right int) bool {
		return ReasoningLevelRank(levels[left]) < ReasoningLevelRank(levels[right])
	})
}

func reasoningFormatUsesBudgets(format string) bool {
	switch format {
	case ReasoningFormatAnthropicBudget, ReasoningFormatGeminiBudget, ReasoningFormatQwen:
		return true
	}
	return false
}

// parseReasoningBudgets 解析档位预算：预算类格式要求每个非 none 档位都有预算，qwen 可选，
// 其余格式忽略该字段。
func parseReasoningBudgets(format string, levels []string, raw any) (map[string]int, error) {
	if !reasoningFormatUsesBudgets(format) {
		return nil, nil
	}
	budgets := map[string]int{}
	if raw != nil {
		payload, ok := raw.(map[string]any)
		if !ok {
			return nil, ErrInvalidReasoningBudgets
		}
		for key, value := range payload {
			level, ok := NormalizeReasoningLevel(key)
			if !ok {
				return nil, ErrInvalidReasoningBudgets
			}
			budget, ok := nonNegativeInteger(value)
			if !ok {
				return nil, ErrInvalidReasoningBudgets
			}
			budgets[level] = budget
		}
	}
	if format == ReasoningFormatQwen {
		return budgets, nil
	}
	for _, level := range levels {
		if level == ReasoningLevelNone {
			continue
		}
		budget, ok := budgets[level]
		if !ok {
			return nil, ErrInvalidReasoningBudgets
		}
		if format == ReasoningFormatAnthropicBudget && budget < AnthropicMinThinkingBudget {
			return nil, ErrInvalidReasoningBudgets
		}
	}
	return budgets, nil
}

func nonNegativeInteger(value any) (int, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil || parsed < 0 || parsed > math.MaxInt32 {
			return 0, false
		}
		return int(parsed), true
	case float64:
		if typed < 0 || typed > math.MaxInt32 || typed != math.Trunc(typed) {
			return 0, false
		}
		return int(typed), true
	}
	return 0, false
}

// inferReasoningCapability 把管理员配置的原生思考参数映射为统一能力，依次尝试：
// (1) 协议档位路径上的 select 控件（档位取控件选项）；(2) defaultOptions 中的原生思考参数；
// (3) 原生思考控件（开关、预算）。(2)(3) 的档位取目录（同格式）或标准档位，默认档位取管理员的值。
func inferReasoningCapability(protocolKey string, payload map[string]any, catalog *ReasoningCapability) *ReasoningCapability {
	if capability := inferReasoningCapabilityFromLevelControl(protocolKey, payload); capability != nil {
		return capability
	}
	if catalog != nil && !ReasoningFormatSupportsProtocol(catalog.Format, protocolKey) {
		catalog = nil
	}
	defaults, _ := payload["defaultOptions"].(map[string]any)
	if signal, ok := DetectReasoningNativeSignal(protocolKey, defaults); ok {
		if capability := capabilityFromNativeSignal(protocolKey, signal, catalog); capability != nil {
			capability.ControlPath = nativeSignalControlPath(protocolKey, signal)
			return capability
		}
	}
	if signal, ok := reasoningNativeSignalFromControls(protocolKey, payload); ok {
		return capabilityFromNativeSignal(protocolKey, signal, catalog)
	}
	return nil
}

// nativeSignalControlPath 在原生参数就是协议档位路径时返回该路径，供前端隐藏重复控件。
func nativeSignalControlPath(protocolKey string, signal ReasoningNativeSignal) string {
	if signal.Level == "" || signal.Format != legacyReasoningFormat(protocolKey) {
		return ""
	}
	return legacyReasoningControlPath(protocolKey)
}

// inferReasoningCapabilityFromLevelControl 取协议已知档位路径上的 select 控件，只保留规范档位；
// 默认值取 defaultOptions 同路径值，其次取不高于 medium 的最高档，再次取首档。
func inferReasoningCapabilityFromLevelControl(protocolKey string, payload map[string]any) *ReasoningCapability {
	path := legacyReasoningControlPath(protocolKey)
	format := legacyReasoningFormat(protocolKey)
	if path == "" || format == "" {
		return nil
	}
	controls, _ := payload["optionControls"].([]any)
	for _, rawControl := range controls {
		control, ok := rawControl.(map[string]any)
		if !ok {
			continue
		}
		controlPath, _ := control["path"].(string)
		controlType, _ := control["type"].(string)
		if strings.TrimSpace(controlPath) != path || !strings.EqualFold(strings.TrimSpace(controlType), "select") {
			continue
		}
		levels := inferredReasoningLevels(format, control["options"])
		if len(levels) == 0 {
			return nil
		}
		capability := &ReasoningCapability{Format: format, Levels: levels, ControlPath: path}
		capability.Default = inferredReasoningDefault(capability, payload["defaultOptions"], path)
		return capability
	}
	return nil
}

func inferredReasoningLevels(format string, raw any) []string {
	items, _ := raw.([]any)
	seen := make(map[string]struct{}, len(items))
	levels := make([]string, 0, len(items))
	for _, item := range items {
		level, ok := NormalizeReasoningLevel(item)
		if !ok || !reasoningFormatAllowsLevel(format, level) {
			continue
		}
		if _, duplicated := seen[level]; duplicated {
			continue
		}
		seen[level] = struct{}{}
		levels = append(levels, level)
	}
	sortReasoningLevels(levels)
	return levels
}

func inferredReasoningDefault(capability *ReasoningCapability, defaults any, path string) string {
	if value, ok := readCapabilityPath(defaults, strings.Split(path, ".")); ok {
		if level, ok := NormalizeReasoningLevel(value); ok && capability.HasLevel(level) {
			return level
		}
	}
	mediumRank := ReasoningLevelRank(ReasoningLevelMedium)
	fallback := ""
	for _, level := range capability.Levels {
		if ReasoningLevelRank(level) <= mediumRank {
			fallback = level
		}
	}
	if fallback != "" {
		return fallback
	}
	return capability.Levels[0]
}

func readCapabilityPath(value any, path []string) (any, bool) {
	current := value
	for _, segment := range path {
		payload, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = payload[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
