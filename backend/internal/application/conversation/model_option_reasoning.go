package conversation

import (
	"encoding/json"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
)

// reasoningEffortOptionKey 是请求 options 中的规范推理档位键，由后端按模型推理能力翻译成原生参数。
const reasoningEffortOptionKey = "reasoning_effort"

// anthropicDefaultMaxTokens 与 Anthropic 适配器未显式设置 max_tokens 时的默认值保持一致，
// 用于把 budget_tokens 钳制在 max_tokens 之内。
const anthropicDefaultMaxTokens = 64000

// resolveChatModelOptions 是消息发送与临时对话共用的参数管线。管理员在模型能力 JSON 中声明能力，用户只通过
// cfg.Controls 选择控件取值，因此这里不再应用参数白名单/黑名单，只保留系统字段的硬拒绝。步骤：
//
//  1. 管理员 defaultOptions 作为基础参数。
//  2. 依次应用每个控件的取值（用户取值无效、未提交或控件被锁定时用默认值）：选项对应的参数片段
//     按协议深度合并，数值控件写入声明的路径。
//  3. options 原样合并：普通用户只剩原生工具选择（由传输层收敛），管理员可提交高级 JSON。
//  4. 重新写入管理员锁定的默认参数。
//  5. 思考强度：锁定 > 用户选择的档位（auto 表示不下发）> 参数中已有的原生思考参数（原样透传）
//     > models.dev 目录（不下发）> 管理员默认档（翻译为协议原生参数）。
//  6. 原生工具按模型配置收敛，删除系统字段，按协议规范取值。
//
// 返回最终参数与本次生效的推理档位（无法得出时为空）。媒体生成链路不走这里。
func resolveChatModelOptions(options map[string]any, protocol string, cfg modelOptionPolicyConfig) (map[string]any, string) {
	protocolKey := modelOptionPolicyProtocolKey(protocol)
	capabilitiesJSON := cfg.ModelCapabilitiesJSON
	defaults := modelCapabilityDefaultOptions(capabilitiesJSON)
	result := cloneModelOptionMap(defaults)
	if result == nil {
		result = make(map[string]any)
	}

	capability, source := domainchannel.ResolveReasoningCapability(protocolKey, capabilitiesJSON, cfg.CatalogReasoning)
	reasoningSelection, reasoningLocked, reasoningLevelFromPatch, reasoningPatched := "", false, "", false
	for _, control := range domainchannel.ResolveModelControls(capabilitiesJSON, []string{protocolKey}, capability, source) {
		value, requested := control.Default, false
		// 锁定或隐藏的控件不接受用户取值，固定使用管理员默认值。
		if !control.Locked && !control.Hidden {
			if raw, ok := cfg.Controls[control.ID]; ok {
				if normalized, valid := control.NormalizeValue(raw); valid {
					value, requested = normalized, true
				}
			}
		}
		if control.Kind == domainchannel.ModelControlKindReasoning && !modelControlUsesPatches(control) {
			// 思考强度档位交给推理能力翻译；未选择且未锁定时不在这里决定。
			if control.Locked {
				reasoningSelection, reasoningLocked = value, true
			} else if requested {
				reasoningSelection = value
			}
			continue
		}
		if control.Kind == domainchannel.ModelControlKindReasoning {
			// 管理员为思考强度写了参数片段：由片段决定原生参数，不再按推理能力翻译。
			reasoningPatched = true
			if _, canonical := domainchannel.NormalizeReasoningLevel(value); canonical {
				reasoningLevelFromPatch = strings.ToLower(value)
			}
		}
		applyModelControlValue(result, protocolKey, control, value)
	}

	rawOptions := cloneModelOptionMap(options)
	if reasoningSelection == "" {
		// 兼容旧客户端与「以指定档位重新生成」：options.reasoning_effort 视为思考强度选择。
		if text, ok := rawOptions[reasoningEffortOptionKey].(string); ok {
			reasoningSelection = strings.ToLower(strings.TrimSpace(text))
		}
	}
	// 规范键只在 OpenAI 兼容 Chat 协议上同时是原生参数；其他协议或已有推理能力时由翻译接管。
	if capability != nil || reasoningPatched || !isReasoningEffortNativeChatProtocol(protocolKey) {
		delete(rawOptions, reasoningEffortOptionKey)
	}
	mergeModelOptionMap(result, rawOptions)
	for _, path := range modelCapabilityLockedOptionPaths(capabilitiesJSON) {
		if value, ok := readModelOptionPath(defaults, path); ok {
			writeModelOptionPath(result, path, cloneModelOptionValue(value))
		}
	}

	level := reasoningLevelFromPatch
	if capability != nil && !reasoningPatched {
		result, level = resolveChatReasoning(result, protocolKey, *capability, source, reasoningSelection, reasoningLocked)
	}

	nativeTools := nativeProviderToolsFromOption(protocolKey, result["tools"], capabilitiesJSON)
	delete(result, "tools")
	for _, path := range hardDeniedModelOptionPaths {
		deleteModelOptionPath(result, path)
	}
	sanitizeModelOptionValues(result, protocolKey)
	if len(nativeTools) > 0 {
		result["tools"] = nativeTools
	}
	if len(result) == 0 {
		return nil, level
	}
	return result, level
}

// resolveChatReasoning 按优先级决定思考参数，返回参数与生效档位。
func resolveChatReasoning(
	options map[string]any,
	protocolKey string,
	capability domainchannel.ReasoningCapability,
	source string,
	selection string,
	locked bool,
) (map[string]any, string) {
	switch {
	case locked:
		level := capability.ResolveLevel(selection)
		return applyReasoningEffortTranslation(options, protocolKey, capability, level), level
	case selection == domainchannel.ModelControlReasoningAuto:
		return options, ""
	case selection != "":
		level := capability.ResolveLevel(selection)
		return applyReasoningEffortTranslation(options, protocolKey, capability, level), level
	}
	if signal, ok := domainchannel.DetectReasoningNativeSignal(protocolKey, options); ok {
		level := signal.LevelFor(&capability)
		if !capability.HasLevel(level) {
			level = ""
		}
		return options, level
	}
	if source == domainchannel.ReasoningSourceCatalog {
		return options, ""
	}
	return applyReasoningEffortTranslation(options, protocolKey, capability, capability.Default), capability.Default
}

func isReasoningEffortNativeChatProtocol(protocolKey string) bool {
	return protocolKey == domainchannel.ReasoningProtocolOpenAIChat || protocolKey == domainchannel.ReasoningProtocolOpenRouterChat
}

// modelControlUsesPatches 判断控件是否由管理员参数片段驱动；自动生成的思考强度控件没有片段。
func modelControlUsesPatches(control domainchannel.ModelControl) bool {
	for _, option := range control.Options {
		if option.Patch != nil || len(option.Patches) > 0 {
			return true
		}
	}
	return false
}

// applyModelControlValue 把控件取值写入参数；取值为空（无默认值且用户未选择）时不修改。
func applyModelControlValue(options map[string]any, protocolKey string, control domainchannel.ModelControl, value string) {
	if value == "" {
		return
	}
	switch control.Type {
	case domainchannel.ModelControlTypeNumber:
		if number, ok := control.NumberValue(value); ok {
			var typed any = number
			if control.Integer || number == float64(int64(number)) {
				typed = int64(number)
			}
			writeModelOptionPath(options, splitModelOptionPath(control.Path), typed)
		}
	default:
		if option, ok := control.Option(value); ok {
			applyModelControlPatch(options, option.PatchFor(protocolKey))
		}
	}
}

// applyModelControlPatch 把参数片段深度合并进参数：对象逐层合并，其他值整体替换，null 删除该键。
func applyModelControlPatch(options map[string]any, patch map[string]any) {
	for key, value := range patch {
		if value == nil {
			delete(options, key)
			continue
		}
		if patchMap, ok := value.(map[string]any); ok {
			target, isMap := options[key].(map[string]any)
			if !isMap || target == nil {
				target = make(map[string]any, len(patchMap))
				options[key] = target
			}
			applyModelControlPatch(target, patchMap)
			continue
		}
		options[key] = modelControlPatchValue(value)
	}
}

// modelControlPatchValue 复制片段中的值，并把能力 JSON 解析得到的 json.Number 还原为整数或浮点数，
// 与用户参数与 defaultOptions 的数值表示一致。
func modelControlPatchValue(value any) any {
	switch typed := value.(type) {
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer
		}
		if number, err := typed.Float64(); err == nil {
			return number
		}
		return typed.String()
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = modelControlPatchValue(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = modelControlPatchValue(item)
		}
		return result
	}
	return value
}

// applyReasoningEffortTranslation 删除规范键与冲突的原生推理键，再写入该格式在协议上的原生形式。
func applyReasoningEffortTranslation(
	options map[string]any,
	protocolKey string,
	capability domainchannel.ReasoningCapability,
	level string,
) map[string]any {
	if options == nil {
		options = make(map[string]any)
	}
	isChat := protocolKey == domainchannel.ReasoningProtocolOpenAIChat || protocolKey == domainchannel.ReasoningProtocolOpenRouterChat
	if capability.Format != domainchannel.ReasoningFormatOpenAI || !isChat {
		delete(options, reasoningEffortOptionKey)
	}
	switch capability.Format {
	case domainchannel.ReasoningFormatOpenAI:
		if isChat {
			options[reasoningEffortOptionKey] = level
			// OpenRouter Chat 同时接受统一 reasoning 对象，两者并存时以规范档位为准。
			deleteModelOptionPathPruned(options, "reasoning.effort")
			return options
		}
		writeModelOptionPath(options, []string{"reasoning", "effort"}, level)
	case domainchannel.ReasoningFormatAnthropicEffort:
		translateAnthropicEffort(options, level)
	case domainchannel.ReasoningFormatAnthropicBudget:
		translateAnthropicBudget(options, capability, level)
	case domainchannel.ReasoningFormatGeminiLevel:
		if protocolKey == domainchannel.ReasoningProtocolGeminiInteractions {
			writeModelOptionPath(options, []string{"generation_config", "thinking_level"}, level)
			return options
		}
		deleteGeminiThinkingAliases(options)
		writeModelOptionPath(options, []string{"generationConfig", "thinkingConfig", "thinkingLevel"}, level)
	case domainchannel.ReasoningFormatGeminiBudget:
		deleteGeminiThinkingAliases(options)
		budget := 0
		if level != domainchannel.ReasoningLevelNone {
			budget = capability.Budgets[level]
		}
		writeModelOptionPath(options, []string{"generationConfig", "thinkingConfig", "thinkingBudget"}, budget)
	case domainchannel.ReasoningFormatQwen:
		if level == domainchannel.ReasoningLevelNone {
			options["enable_thinking"] = false
			delete(options, "thinking_budget")
			return options
		}
		options["enable_thinking"] = true
		if budget, ok := capability.Budgets[level]; ok {
			options["thinking_budget"] = budget
		} else {
			delete(options, "thinking_budget")
		}
	case domainchannel.ReasoningFormatToggle:
		thinkingType := "enabled"
		if level == domainchannel.ReasoningLevelNone {
			thinkingType = "disabled"
		}
		options["thinking"] = map[string]any{"type": thinkingType}
	}
	return options
}

// deleteAnthropicThinkingAliases 删除会在 Anthropic 适配器里覆盖 thinking 的兼容别名。
func deleteAnthropicThinkingAliases(options map[string]any) {
	delete(options, "enable_thinking")
	delete(options, "budget_tokens")
}

func translateAnthropicEffort(options map[string]any, level string) {
	deleteAnthropicThinkingAliases(options)
	// 顶层 effort 是 output_config.effort 的兼容别名，由规范档位接管。
	delete(options, "effort")
	if level == domainchannel.ReasoningLevelNone {
		options["thinking"] = map[string]any{"type": "disabled"}
		delete(options, "thinking_display")
		deleteModelOptionPathPruned(options, "output_config.effort")
		return
	}
	writeModelOptionPath(options, []string{"output_config", "effort"}, level)
	thinking, ok := options["thinking"].(map[string]any)
	if !ok {
		options["thinking"] = map[string]any{"type": "adaptive"}
		return
	}
	thinkingType := strings.TrimSpace(modelOptionStringValue(thinking["type"]))
	if thinkingType == "" || thinkingType == "disabled" {
		thinking["type"] = "adaptive"
		delete(thinking, "budget_tokens")
	}
}

func translateAnthropicBudget(options map[string]any, capability domainchannel.ReasoningCapability, level string) {
	deleteAnthropicThinkingAliases(options)
	if level == domainchannel.ReasoningLevelNone {
		options["thinking"] = map[string]any{"type": "disabled"}
		delete(options, "thinking_display")
		return
	}
	thinking := map[string]any{
		"type":          "enabled",
		"budget_tokens": clampAnthropicThinkingBudget(capability.Budgets[level], anthropicRequestMaxTokens(options)),
	}
	if existing, ok := options["thinking"].(map[string]any); ok {
		if display, exists := existing["display"]; exists {
			thinking["display"] = display
		}
	}
	options["thinking"] = thinking
}

// anthropicRequestMaxTokens 复刻 Anthropic 适配器读取 max_tokens 的优先级。
func anthropicRequestMaxTokens(options map[string]any) int {
	for _, key := range []string{"max_output_tokens", "max_tokens"} {
		if value, ok := modelParamIntFromOption(options[key]); ok && value > 0 {
			return value
		}
	}
	return anthropicDefaultMaxTokens
}

// clampAnthropicThinkingBudget 把预算钳制到 [1024, max_tokens-1]；max_tokens 过小时以协议下限为准，
// 让上游返回明确的参数错误而不是静默关闭思考。
func clampAnthropicThinkingBudget(budget int, maxTokens int) int {
	if upper := maxTokens - 1; budget > upper {
		budget = upper
	}
	if budget < domainchannel.AnthropicMinThinkingBudget {
		budget = domainchannel.AnthropicMinThinkingBudget
	}
	return budget
}

// deleteGeminiThinkingAliases 删除 Gemini 适配器会合并进 thinkingConfig 的档位/预算别名，
// thinkingLevel 与 thinkingBudget 在上游互斥，统一由规范档位写入其一。
func deleteGeminiThinkingAliases(options map[string]any) {
	for _, key := range []string{"thinking_level", "thinkingLevel", "thinking_budget", "thinkingBudget", "budget_tokens"} {
		delete(options, key)
	}
	for _, parent := range []string{"thinkingConfig", "generationConfig.thinkingConfig", "generation_config.thinkingConfig"} {
		for _, key := range []string{"thinkingLevel", "thinkingBudget", "thinking_level", "thinking_budget", "budget_tokens"} {
			deleteModelOptionPathPruned(options, parent+"."+key)
		}
	}
}

// deleteModelOptionPathPruned 删除点分路径上的值，并清理因此变空的父级对象，避免下发空对象。
func deleteModelOptionPathPruned(options map[string]any, dotted string) {
	path := splitModelOptionPath(dotted)
	if len(path) == 0 {
		return
	}
	pruneModelOptionPath(options, path)
}

func pruneModelOptionPath(current map[string]any, path []string) {
	if len(path) == 1 {
		delete(current, path[0])
		return
	}
	child, ok := current[path[0]].(map[string]any)
	if !ok {
		return
	}
	pruneModelOptionPath(child, path[1:])
	if len(child) == 0 {
		delete(current, path[0])
	}
}
