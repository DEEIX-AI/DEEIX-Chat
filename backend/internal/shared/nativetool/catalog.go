package nativetool

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	priceUSD001Nanousd   int64 = 10_000_000
	priceUSD0025Nanousd  int64 = 25_000_000
	priceUSD0005Nanousd  int64 = 5_000_000
	priceUSD00025Nanousd int64 = 2_500_000
)

// Definition 描述后端已适配、可由管理员开启的厂商官方原生工具。
type Definition struct {
	Protocol         string
	Provider         string
	Type             string
	Key              string
	Label            string
	Description      string
	Payload          map[string]interface{}
	DefaultEnabled   bool
	Billable         bool
	BillingUnit      string
	PriceNanousd     int64
	PriceLabel       string
	RiskLevel        string
	UsageAliases     []string
	rawTypeFieldKeys []string
}

// PricingDefinition 描述当前内置的原生工具默认计费项目。
type PricingDefinition struct {
	Provider     string
	ToolKey      string
	PriceNanousd int64
	Unit         string
	PriceLabel   string
	Billable     bool
}

// PricingOverride 描述管理员可覆盖的原生工具计费项。
type PricingOverride struct {
	PriceNanousd int64  `json:"priceNanousd"`
	Unit         string `json:"unit"`
	PriceLabel   string `json:"priceLabel"`
	Billable     bool   `json:"billable"`
}

// UsagePrice 描述可直接折算为按次计费的原生工具价格。
type UsagePrice struct {
	Provider       string
	ServiceName    string
	NanousdPerCall int64
}

var protocolOrder = []string{
	"openai_chat_completions",
	"openai_responses",
	"anthropic_messages",
	"xai_responses",
	"gemini_generate_content",
	"google_image_generation",
}

var definitions = []Definition{
	{
		Protocol:       "openai_chat_completions",
		Provider:       "OpenAI",
		Type:           "web_search",
		Key:            "openai.web_search",
		Label:          "Web Search",
		Description:    "OpenAI hosted web search.",
		Payload:        map[string]interface{}{"type": "web_search"},
		DefaultEnabled: true,
		Billable:       true,
		BillingUnit:    "call",
		PriceNanousd:   priceUSD0025Nanousd,
		UsageAliases:   []string{"web_search"},
	},
	{
		Protocol:       "openai_chat_completions",
		Provider:       "OpenAI",
		Type:           "web_search_preview",
		Key:            "openai.web_search_preview",
		Label:          "Web Search Preview",
		Description:    "OpenAI hosted web search preview tool.",
		Payload:        map[string]interface{}{"type": "web_search_preview"},
		DefaultEnabled: true,
		Billable:       true,
		BillingUnit:    "call",
		PriceNanousd:   priceUSD0025Nanousd,
		UsageAliases:   []string{"web_search_preview"},
	},
	{
		Protocol:       "openai_responses",
		Provider:       "OpenAI",
		Type:           "web_search",
		Key:            "openai.web_search",
		Label:          "Web Search",
		Description:    "OpenAI hosted web search.",
		Payload:        map[string]interface{}{"type": "web_search"},
		DefaultEnabled: true,
		Billable:       true,
		BillingUnit:    "call",
		PriceNanousd:   priceUSD0025Nanousd,
		UsageAliases:   []string{"web_search"},
	},
	{
		Protocol:       "openai_responses",
		Provider:       "OpenAI",
		Type:           "web_search_preview",
		Key:            "openai.web_search_preview",
		Label:          "Web Search Preview",
		Description:    "OpenAI hosted web search preview tool.",
		Payload:        map[string]interface{}{"type": "web_search_preview"},
		DefaultEnabled: true,
		Billable:       true,
		BillingUnit:    "call",
		PriceNanousd:   priceUSD0025Nanousd,
		UsageAliases:   []string{"web_search_preview"},
	},
	{
		Protocol:       "openai_responses",
		Provider:       "OpenAI",
		Type:           "shell",
		Key:            "openai.shell",
		Label:          "Shell",
		Description:    "OpenAI hosted shell tool with an automatic container.",
		Payload:        map[string]interface{}{"type": "shell", "environment": map[string]interface{}{"type": "container_auto"}},
		DefaultEnabled: true,
		PriceLabel:     "notMetered",
		RiskLevel:      "high",
		UsageAliases:   []string{"shell"},
	},
	{
		Protocol:       "openai_responses",
		Provider:       "OpenAI",
		Type:           "image_generation",
		Key:            "openai.image_generation",
		Label:          "Image Generation",
		Description:    "OpenAI hosted image generation tool.",
		Payload:        map[string]interface{}{"type": "image_generation"},
		DefaultEnabled: true,
		PriceLabel:     "notMetered",
		UsageAliases:   []string{"image_generation"},
	},
	{
		Protocol:       "openai_responses",
		Provider:       "OpenAI",
		Type:           "code_interpreter",
		Key:            "openai.code_interpreter",
		Label:          "Code Interpreter",
		Description:    "OpenAI hosted code interpreter with an automatic container.",
		Payload:        map[string]interface{}{"type": "code_interpreter", "container": map[string]interface{}{"type": "auto"}},
		DefaultEnabled: true,
		PriceLabel:     "notMetered",
		RiskLevel:      "high",
		UsageAliases:   []string{"code_interpreter"},
	},
	{
		Protocol:       "anthropic_messages",
		Provider:       "Anthropic",
		Type:           "web_search_20250305",
		Key:            "anthropic.web_search_20250305",
		Label:          "Web Search",
		Description:    "Anthropic hosted web search tool.",
		Payload:        map[string]interface{}{"type": "web_search_20250305", "name": "web_search"},
		DefaultEnabled: true,
		Billable:       true,
		BillingUnit:    "search",
		PriceNanousd:   priceUSD001Nanousd,
		UsageAliases:   []string{"web_search"},
	},
	{
		Protocol:       "anthropic_messages",
		Provider:       "Anthropic",
		Type:           "web_search_20260209",
		Key:            "anthropic.web_search_20260209",
		Label:          "Web Search",
		Description:    "Anthropic hosted web search tool.",
		Payload:        map[string]interface{}{"type": "web_search_20260209", "name": "web_search", "allowed_callers": []string{"direct"}},
		DefaultEnabled: true,
		Billable:       true,
		BillingUnit:    "search",
		PriceNanousd:   priceUSD001Nanousd,
		UsageAliases:   []string{"web_search"},
	},
	{
		Protocol:       "anthropic_messages",
		Provider:       "Anthropic",
		Type:           "web_fetch_20250910",
		Key:            "anthropic.web_fetch_20250910",
		Label:          "Web Fetch",
		Description:    "Anthropic hosted web fetch tool.",
		Payload:        map[string]interface{}{"type": "web_fetch_20250910", "name": "web_fetch"},
		DefaultEnabled: true,
		PriceLabel:     "included",
		UsageAliases:   []string{"web_fetch"},
	},
	{
		Protocol:       "anthropic_messages",
		Provider:       "Anthropic",
		Type:           "web_fetch_20260209",
		Key:            "anthropic.web_fetch_20260209",
		Label:          "Web Fetch",
		Description:    "Anthropic hosted web fetch tool.",
		Payload:        map[string]interface{}{"type": "web_fetch_20260209", "name": "web_fetch", "allowed_callers": []string{"direct"}},
		DefaultEnabled: true,
		PriceLabel:     "included",
		UsageAliases:   []string{"web_fetch"},
	},
	{
		Protocol:       "anthropic_messages",
		Provider:       "Anthropic",
		Type:           "code_execution_20250825",
		Key:            "anthropic.code_execution_20250825",
		Label:          "Code Execution",
		Description:    "Anthropic hosted code execution tool.",
		Payload:        map[string]interface{}{"type": "code_execution_20250825", "name": "code_execution"},
		DefaultEnabled: true,
		PriceLabel:     "notMetered",
		RiskLevel:      "high",
		UsageAliases:   []string{"code_execution"},
	},
	{
		Protocol:       "anthropic_messages",
		Provider:       "Anthropic",
		Type:           "code_execution_20260120",
		Key:            "anthropic.code_execution_20260120",
		Label:          "Code Execution",
		Description:    "Anthropic hosted code execution tool.",
		Payload:        map[string]interface{}{"type": "code_execution_20260120", "name": "code_execution"},
		DefaultEnabled: true,
		PriceLabel:     "notMetered",
		RiskLevel:      "high",
		UsageAliases:   []string{"code_execution"},
	},
	{
		Protocol:       "anthropic_messages",
		Provider:       "Anthropic",
		Type:           "advisor_20260301",
		Key:            "anthropic.advisor_20260301",
		Label:          "Advisor",
		Description:    "Anthropic hosted advisor tool.",
		Payload:        map[string]interface{}{"type": "advisor_20260301", "name": "advisor"},
		DefaultEnabled: true,
		PriceLabel:     "notMetered",
		UsageAliases:   []string{"advisor"},
	},
	{
		Protocol:       "anthropic_messages",
		Provider:       "Anthropic",
		Type:           "tool_search_tool_regex_20251119",
		Key:            "anthropic.tool_search_tool_regex_20251119",
		Label:          "Tool Search Regex",
		Description:    "Anthropic hosted regex tool search.",
		Payload:        map[string]interface{}{"type": "tool_search_tool_regex_20251119", "name": "tool_search_tool_regex"},
		DefaultEnabled: true,
		PriceLabel:     "included",
		UsageAliases:   []string{"tool_search_tool_regex"},
	},
	{
		Protocol:       "anthropic_messages",
		Provider:       "Anthropic",
		Type:           "tool_search_tool_bm25_20251119",
		Key:            "anthropic.tool_search_tool_bm25_20251119",
		Label:          "Tool Search BM25",
		Description:    "Anthropic hosted BM25 tool search.",
		Payload:        map[string]interface{}{"type": "tool_search_tool_bm25_20251119", "name": "tool_search_tool_bm25"},
		DefaultEnabled: true,
		PriceLabel:     "included",
		UsageAliases:   []string{"tool_search_tool_bm25"},
	},
	{
		Protocol:       "xai_responses",
		Provider:       "xAI",
		Type:           "web_search",
		Key:            "xai.web_search",
		Label:          "Web Search",
		Description:    "xAI hosted web search.",
		Payload:        map[string]interface{}{"type": "web_search"},
		DefaultEnabled: true,
		Billable:       true,
		BillingUnit:    "call",
		PriceNanousd:   priceUSD0005Nanousd,
		UsageAliases:   []string{"web_search"},
	},
	{
		Protocol:       "xai_responses",
		Provider:       "xAI",
		Type:           "x_search",
		Key:            "xai.x_search",
		Label:          "X Search",
		Description:    "xAI hosted X search.",
		Payload:        map[string]interface{}{"type": "x_search"},
		DefaultEnabled: true,
		Billable:       true,
		BillingUnit:    "call",
		PriceNanousd:   priceUSD0005Nanousd,
		UsageAliases:   []string{"x_search"},
	},
	{
		Protocol:       "xai_responses",
		Provider:       "xAI",
		Type:           "code_interpreter",
		Key:            "xai.code_interpreter",
		Label:          "Code Interpreter",
		Description:    "xAI hosted code interpreter.",
		Payload:        map[string]interface{}{"type": "code_interpreter"},
		DefaultEnabled: true,
		Billable:       true,
		BillingUnit:    "call",
		PriceNanousd:   priceUSD0005Nanousd,
		RiskLevel:      "high",
		UsageAliases:   []string{"code_interpreter", "code_execution"},
	},
	{
		Protocol:         "gemini_generate_content",
		Provider:         "Google",
		Type:             "google_search",
		Key:              "google.google_search",
		Label:            "Google Search",
		Description:      "Google hosted search grounding tool.",
		Payload:          map[string]interface{}{"type": "google_search", "google_search": map[string]interface{}{}},
		DefaultEnabled:   true,
		PriceLabel:       "notMetered",
		UsageAliases:     []string{"google_search"},
		rawTypeFieldKeys: []string{"google_search", "googleSearch"},
	},
	{
		Protocol:         "google_image_generation",
		Provider:         "Google",
		Type:             "google_search",
		Key:              "google.google_search",
		Label:            "Google Search",
		Description:      "Google hosted search grounding tool.",
		Payload:          map[string]interface{}{"type": "google_search", "google_search": map[string]interface{}{}},
		DefaultEnabled:   true,
		PriceLabel:       "notMetered",
		UsageAliases:     []string{"google_search"},
		rawTypeFieldKeys: []string{"google_search", "googleSearch"},
	},
}

var pricingDefinitions = []PricingDefinition{
	{Provider: "OpenAI", ToolKey: "openaiWebSearchReasoning", PriceNanousd: priceUSD001Nanousd, Unit: "call", Billable: true},
	{Provider: "OpenAI", ToolKey: "openaiWebSearchStandard", PriceNanousd: priceUSD0025Nanousd, Unit: "call", Billable: true},
	{Provider: "OpenAI", ToolKey: "openaiShell", Unit: "call"},
	{Provider: "OpenAI", ToolKey: "openaiImageGeneration", Unit: "call"},
	{Provider: "OpenAI", ToolKey: "openaiCodeInterpreter", Unit: "call"},
	{Provider: "Anthropic", ToolKey: "anthropicWebSearch", PriceNanousd: priceUSD001Nanousd, Unit: "call", Billable: true},
	{Provider: "Anthropic", ToolKey: "anthropicWebFetch", Unit: "call"},
	{Provider: "Anthropic", ToolKey: "anthropicCodeExecution", Unit: "call"},
	{Provider: "Anthropic", ToolKey: "anthropicAdvisor", Unit: "call"},
	{Provider: "Anthropic", ToolKey: "anthropicToolSearch", Unit: "call"},
	{Provider: "xAI", ToolKey: "xaiWebSearch", PriceNanousd: priceUSD0005Nanousd, Unit: "call", Billable: true},
	{Provider: "xAI", ToolKey: "xaiXSearch", PriceNanousd: priceUSD0005Nanousd, Unit: "call", Billable: true},
	{Provider: "xAI", ToolKey: "xaiCodeExecution", PriceNanousd: priceUSD0005Nanousd, Unit: "call", Billable: true},
	{Provider: "xAI", ToolKey: "xaiAttachmentSearch", PriceNanousd: priceUSD001Nanousd, Unit: "call", Billable: true},
	{Provider: "xAI", ToolKey: "xaiCollectionsSearch", PriceNanousd: priceUSD00025Nanousd, Unit: "call", Billable: true},
	{Provider: "Google", ToolKey: "googleGoogleSearch", Unit: "call"},
}

var usagePricesByKey = map[string]UsagePrice{
	"openaiWebSearchReasoning": {Provider: "openai", ServiceName: "OpenAI Web search", NanousdPerCall: priceUSD001Nanousd},
	"openaiWebSearchStandard":  {Provider: "openai", ServiceName: "OpenAI Web search", NanousdPerCall: priceUSD0025Nanousd},
	"openaiShell":              {Provider: "openai", ServiceName: "OpenAI Shell"},
	"openaiImageGeneration":    {Provider: "openai", ServiceName: "OpenAI Image Generation"},
	"openaiCodeInterpreter":    {Provider: "openai", ServiceName: "OpenAI Code Interpreter"},
	"anthropicWebSearch":       {Provider: "anthropic", ServiceName: "Anthropic Web search", NanousdPerCall: priceUSD001Nanousd},
	"anthropicWebFetch":        {Provider: "anthropic", ServiceName: "Anthropic Web Fetch"},
	"anthropicCodeExecution":   {Provider: "anthropic", ServiceName: "Anthropic Code Execution"},
	"anthropicAdvisor":         {Provider: "anthropic", ServiceName: "Anthropic Advisor"},
	"anthropicToolSearch":      {Provider: "anthropic", ServiceName: "Anthropic Tool Search"},
	"xaiWebSearch":             {Provider: "xai", ServiceName: "xAI Web Search", NanousdPerCall: priceUSD0005Nanousd},
	"xaiXSearch":               {Provider: "xai", ServiceName: "xAI X Search", NanousdPerCall: priceUSD0005Nanousd},
	"xaiCodeExecution":         {Provider: "xai", ServiceName: "xAI Code Execution", NanousdPerCall: priceUSD0005Nanousd},
	"xaiAttachmentSearch":      {Provider: "xai", ServiceName: "xAI File Attachments Search", NanousdPerCall: priceUSD001Nanousd},
	"xaiCollectionsSearch":     {Provider: "xai", ServiceName: "xAI Collections Search / RAG", NanousdPerCall: priceUSD00025Nanousd},
	"googleGoogleSearch":       {Provider: "google", ServiceName: "Google Search grounding"},
}

// Definitions 返回全部官方原生工具定义。
func Definitions() []Definition {
	result := make([]Definition, 0, len(definitions))
	for _, definition := range definitions {
		result = append(result, cloneDefinition(definition))
	}
	return result
}

// DefinitionsByProtocol 返回指定协议下的官方原生工具定义。
func DefinitionsByProtocol(protocol string) []Definition {
	protocol = strings.TrimSpace(protocol)
	result := make([]Definition, 0)
	for _, definition := range definitions {
		if definition.Protocol == protocol {
			result = append(result, cloneDefinition(definition))
		}
	}
	return result
}

// Protocols 返回有原生工具定义的协议主键。
func Protocols() []string {
	return append([]string(nil), protocolOrder...)
}

// Find 返回指定协议和类型的官方原生工具定义。
func Find(protocol string, toolType string) (Definition, bool) {
	protocol = strings.TrimSpace(protocol)
	toolType = strings.TrimSpace(toolType)
	for _, definition := range definitions {
		if definition.Protocol == protocol && definition.Type == toolType {
			return cloneDefinition(definition), true
		}
	}
	return Definition{}, false
}

// FindByKey 返回指定官方原生工具 key 的第一个目录定义。
func FindByKey(key string) (Definition, bool) {
	key = strings.TrimSpace(key)
	for _, definition := range definitions {
		if definition.Key == key {
			return cloneDefinition(definition), true
		}
	}
	return Definition{}, false
}

var nativeToolDeniedPayloadKeys = map[string]struct{}{
	"model":                {},
	"messages":             {},
	"input":                {},
	"instructions":         {},
	"prompt":               {},
	"system":               {},
	"systemInstruction":    {},
	"headers":              {},
	"api_key":              {},
	"apiKey":               {},
	"base_url":             {},
	"baseURL":              {},
	"stream":               {},
	"previous_response_id": {},
}

// PayloadFromOption 识别用户 options.tools 项中的官方原生工具，并返回可发送给上游的规范化 payload。
func PayloadFromOption(protocol string, raw map[string]interface{}) (Definition, map[string]interface{}, bool) {
	toolType := strings.TrimSpace(stringValue(raw["type"]))
	if toolType == "" {
		toolType = inferToolTypeFromRawKeys(protocol, raw)
	}
	if toolType == "" {
		return Definition{}, nil, false
	}
	definition, ok := Find(protocol, toolType)
	if !ok {
		return Definition{}, nil, false
	}
	return definition, buildPayload(definition, raw), true
}

// PayloadFromKey 识别指定官方原生工具 key，并返回可发送给上游的规范化 payload。
func PayloadFromKey(key string, raw map[string]interface{}) (Definition, map[string]interface{}, bool) {
	key = strings.TrimSpace(key)
	for _, definition := range definitions {
		if definition.Key != key {
			continue
		}
		matched, payload, ok := PayloadFromOption(definition.Protocol, raw)
		if !ok {
			continue
		}
		return matched, payload, true
	}
	return Definition{}, nil, false
}

func buildPayload(definition Definition, raw map[string]interface{}) map[string]interface{} {
	payload := cloneMap(raw)
	for key := range nativeToolDeniedPayloadKeys {
		delete(payload, key)
	}
	for _, key := range definition.rawTypeFieldKeys {
		if _, canonical := definition.Payload[key]; !canonical {
			delete(payload, key)
		}
	}
	mergePayload(payload, definition.Payload)
	return payload
}

func mergePayload(dst map[string]interface{}, src map[string]interface{}) {
	for key, value := range src {
		srcMap, srcIsMap := value.(map[string]interface{})
		dstMap, dstIsMap := dst[key].(map[string]interface{})
		if srcIsMap && dstIsMap {
			mergePayload(dstMap, srcMap)
			continue
		}
		dst[key] = cloneValue(value)
	}
}

// PricingDefinitions 返回内置默认原生工具计费展示目录。
func PricingDefinitions() []PricingDefinition {
	result := make([]PricingDefinition, len(pricingDefinitions))
	copy(result, pricingDefinitions)
	return result
}

// PricingDefinitionsWithOverrides 返回应用管理员覆盖后的原生工具计费展示目录。
func PricingDefinitionsWithOverrides(raw string) []PricingDefinition {
	items := PricingDefinitions()
	overrides, err := ParsePricingOverridesJSON(raw)
	if err != nil {
		return items
	}
	for index, item := range items {
		override, ok := overrides[item.ToolKey]
		if !ok {
			continue
		}
		items[index].PriceNanousd = override.PriceNanousd
		items[index].Unit = override.Unit
		items[index].PriceLabel = override.PriceLabel
		items[index].Billable = override.Billable
	}
	return items
}

// DefaultPricingJSON 返回原生工具默认计费配置 JSON。
func DefaultPricingJSON() string {
	raw, err := json.MarshalIndent(PricingOverridesFromDefinitions(PricingDefinitions()), "", "  ")
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// PricingOverridesFromDefinitions 将计费展示项转换为可保存的覆盖配置。
func PricingOverridesFromDefinitions(items []PricingDefinition) map[string]PricingOverride {
	result := make(map[string]PricingOverride, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.ToolKey)
		if key == "" {
			continue
		}
		result[key] = PricingOverride{
			PriceNanousd: item.PriceNanousd,
			Unit:         strings.TrimSpace(item.Unit),
			PriceLabel:   strings.TrimSpace(item.PriceLabel),
			Billable:     item.Billable,
		}
	}
	return result
}

// PricingOverridesJSON 将覆盖配置规范化为稳定 JSON。
func PricingOverridesJSON(overrides map[string]PricingOverride) (string, error) {
	normalized, err := normalizePricingOverrides(overrides)
	if err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ParsePricingOverridesJSON 解析并校验管理员原生工具计费覆盖配置。
func ParsePricingOverridesJSON(raw string) (map[string]PricingOverride, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return map[string]PricingOverride{}, nil
	}
	var parsed map[string]PricingOverride
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return nil, fmt.Errorf("native tool pricing must be a JSON object: %w", err)
	}
	return normalizePricingOverrides(parsed)
}

// PricingOverridesUseDefaults 判断配置是否等同于内置默认价格。
func PricingOverridesUseDefaults(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return true
	}
	overrides, err := ParsePricingOverridesJSON(value)
	if err != nil {
		return true
	}
	defaults := PricingOverridesFromDefinitions(PricingDefinitions())
	for key, override := range overrides {
		defaultOverride, ok := defaults[key]
		if !ok || !pricingOverrideEqual(override, defaultOverride) {
			return false
		}
	}
	return true
}

// UsagePricingKey 返回后端 usage key 对应的默认计费项主键。
func UsagePricingKey(protocol string, toolName string) (string, bool) {
	protocol = strings.TrimSpace(protocol)
	tool := strings.TrimSpace(toolName)
	switch protocol {
	case "anthropic_messages":
		switch tool {
		case "web_search":
			return "anthropicWebSearch", true
		case "web_fetch":
			return "anthropicWebFetch", true
		case "code_execution":
			return "anthropicCodeExecution", true
		case "advisor":
			return "anthropicAdvisor", true
		case "tool_search_tool_regex", "tool_search_tool_bm25":
			return "anthropicToolSearch", true
		}
	case "openai_responses", "openai_chat_completions":
		switch tool {
		case "web_search", "web_search_preview":
			return "openaiWebSearchStandard", true
		case "shell":
			return "openaiShell", true
		case "image_generation":
			return "openaiImageGeneration", true
		case "code_interpreter":
			return "openaiCodeInterpreter", true
		}
	case "xai_responses":
		switch tool {
		case "web_search":
			return "xaiWebSearch", true
		case "x_search":
			return "xaiXSearch", true
		case "code_interpreter", "code_execution":
			return "xaiCodeExecution", true
		case "attachment_search", "file_attachment_search":
			return "xaiAttachmentSearch", true
		case "file_search", "collection_search", "collections_search":
			return "xaiCollectionsSearch", true
		}
	case "gemini_generate_content", "google_image_generation":
		if tool == "google_search" {
			return "googleGoogleSearch", true
		}
	}
	return "", false
}

// UsagePriceByKey 返回可计费原生工具项的按次价格。
func UsagePriceByKey(key string) (UsagePrice, bool) {
	price, ok := usagePricesByKey[strings.TrimSpace(key)]
	return price, ok
}

// UsagePriceByKeyWithOverrides 返回应用管理员覆盖后的可计费原生工具按次价格。
func UsagePriceByKeyWithOverrides(key string, overrides map[string]PricingOverride) (UsagePrice, bool) {
	normalizedKey := strings.TrimSpace(key)
	price, ok := usagePricesByKey[normalizedKey]
	if !ok {
		return UsagePrice{}, false
	}
	override, hasOverride := overrides[normalizedKey]
	if !hasOverride {
		return price, true
	}
	if override.PriceNanousd <= 0 {
		return UsagePrice{}, false
	}
	price.NanousdPerCall = override.PriceNanousd
	return price, true
}

func normalizePricingOverrides(overrides map[string]PricingOverride) (map[string]PricingOverride, error) {
	defaults := PricingOverridesFromDefinitions(PricingDefinitions())
	result := make(map[string]PricingOverride, len(overrides))
	for key, override := range overrides {
		key = strings.TrimSpace(key)
		defaultOverride, ok := defaults[key]
		if !ok {
			return nil, fmt.Errorf("native tool pricing contains unsupported tool key: %s", key)
		}
		if override.PriceNanousd < 0 {
			return nil, fmt.Errorf("native tool pricing %s priceNanousd must be >= 0", key)
		}
		override.Unit = strings.TrimSpace(defaultOverride.Unit)
		if override.Unit == "" {
			override.Unit = "call"
		}
		override.PriceLabel = ""
		override.Billable = override.PriceNanousd > 0
		if len([]rune(override.Unit)) > 32 {
			return nil, fmt.Errorf("native tool pricing %s unit length must be <= 32", key)
		}
		if len([]rune(override.PriceLabel)) > 64 {
			return nil, fmt.Errorf("native tool pricing %s priceLabel length must be <= 64", key)
		}
		result[key] = override
	}
	return result, nil
}

func pricingOverrideEqual(left PricingOverride, right PricingOverride) bool {
	return left.PriceNanousd == right.PriceNanousd &&
		strings.TrimSpace(left.Unit) == strings.TrimSpace(right.Unit) &&
		strings.TrimSpace(left.PriceLabel) == strings.TrimSpace(right.PriceLabel) &&
		left.Billable == right.Billable
}

func inferToolTypeFromRawKeys(protocol string, raw map[string]interface{}) string {
	for _, definition := range definitions {
		if definition.Protocol != protocol {
			continue
		}
		for _, key := range definition.rawTypeFieldKeys {
			if _, ok := raw[key]; ok {
				return definition.Type
			}
		}
	}
	return ""
}

func cloneDefinition(definition Definition) Definition {
	definition.Payload = cloneMap(definition.Payload)
	definition.UsageAliases = append([]string(nil), definition.UsageAliases...)
	definition.rawTypeFieldKeys = append([]string(nil), definition.rawTypeFieldKeys...)
	return definition
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = cloneValue(value)
	}
	return dst
}

func cloneValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return cloneMap(typed)
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		items := make([]interface{}, len(typed))
		for index, item := range typed {
			items[index] = cloneValue(item)
		}
		return items
	default:
		return typed
	}
}

func stringValue(value interface{}) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}
