package channel

// ReasoningTemplate 是管理端自定义推理强度时的编辑模板：管理员在 Levels 中勾选档位并指定默认档，
// 前端按 Format 与 Budgets 生成显式 reasoning 声明，保存时仍走 ValidateModelCapsOverrides 的同一套校验。
type ReasoningTemplate struct {
	Format string
	// Levels 为该格式可声明的规范档位，按由低到高排列；toggle 格式固定为 none 与 high。
	Levels []string
	// Budgets 仅预算类格式非空，覆盖 Levels 中的每个档位。
	Budgets map[string]int
}

// reasoningTemplateBudgets 是预算类格式在既无生效能力也无目录预算时的建议预算；low / medium / high 与目录
// 推导使用的目标预算一致，其余档位按相同梯度外推。
var reasoningTemplateBudgets = map[string]int{
	ReasoningLevelNone:    0,
	ReasoningLevelMinimal: 1024,
	ReasoningLevelLow:     2048,
	ReasoningLevelMedium:  8192,
	ReasoningLevelHigh:    24576,
	ReasoningLevelXHigh:   32768,
	ReasoningLevelMax:     65536,
}

// CatalogReasoningBudgetBounds 返回目录条目 budget_tokens 选项声明的预算区间；未声明的一端为 0。
func CatalogReasoningBudgetBounds(entry *ModelCatalogEntry) (lower int, upper int) {
	if entry == nil {
		return 0, 0
	}
	option := entry.option(ReasoningCatalogOptionBudget)
	if option == nil {
		return 0, 0
	}
	if option.Min != nil && *option.Min > 0 {
		lower = *option.Min
	}
	if option.Max != nil && *option.Max > 0 {
		upper = *option.Max
	}
	return lower, upper
}

// ReasoningCustomizationTemplate 返回自定义推理强度的编辑模板。格式依次取当前生效能力、目录建议与首个可翻译
// 协议的档位格式（openai / anthropic_effort / gemini_level）；三者都没有（如尚未配置路由）时返回 nil。
// 预算依次取建议预算（钳制到目录声明的 [budgetMin, budgetMax]，0 表示未声明）、目录预算与生效能力的预算
// （同格式时），Anthropic 预算不低于协议下限。
func ReasoningCustomizationTemplate(protocolKeys []string, effective *ReasoningCapability, catalog *ReasoningCapability, budgetMin int, budgetMax int) *ReasoningTemplate {
	format := ""
	switch {
	case effective != nil && (len(protocolKeys) == 0 || reasoningFormatSupportsAnyProtocol(effective.Format, protocolKeys)):
		format = effective.Format
	case catalog != nil && reasoningFormatSupportsAnyProtocol(catalog.Format, protocolKeys):
		format = catalog.Format
	default:
		for _, protocolKey := range protocolKeys {
			if format = legacyReasoningFormat(protocolKey); format != "" {
				break
			}
		}
	}
	if format == "" {
		return nil
	}

	template := &ReasoningTemplate{Format: format}
	if format == ReasoningFormatToggle {
		template.Levels = []string{ReasoningLevelNone, ReasoningLevelHigh}
	} else {
		for _, level := range reasoningLevelOrder {
			// thinkingBudget=0 关闭思考只对目录允许零预算的 Gemini 模型有效（如 2.5 Pro 的下限为 128）。
			if level == ReasoningLevelNone && format == ReasoningFormatGeminiBudget && budgetMin > 0 {
				continue
			}
			if reasoningFormatAllowsLevel(format, level) {
				template.Levels = append(template.Levels, level)
			}
		}
	}
	if !reasoningFormatUsesBudgets(format) {
		return template
	}
	template.Budgets = make(map[string]int, len(template.Levels))
	for _, level := range template.Levels {
		budget := reasoningTemplateBudgets[level]
		if level != ReasoningLevelNone {
			if budgetMin > 0 && budget < budgetMin {
				budget = budgetMin
			}
			if budgetMax > 0 && budget > budgetMax {
				budget = budgetMax
			}
			if format == ReasoningFormatAnthropicBudget && budget < AnthropicMinThinkingBudget {
				budget = AnthropicMinThinkingBudget
			}
		}
		template.Budgets[level] = budget
	}
	for _, source := range []*ReasoningCapability{catalog, effective} {
		if source == nil || source.Format != format {
			continue
		}
		for level, budget := range source.Budgets {
			if _, ok := template.Budgets[level]; ok {
				template.Budgets[level] = budget
			}
		}
	}
	return template
}
