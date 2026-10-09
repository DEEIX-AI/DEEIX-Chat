package channel

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
)

// 推理能力来源，与管理端模型 DTO 的 reasoningSource 取值一致。
const (
	ReasoningSourceExplicit = "explicit"
	ReasoningSourceInferred = "inferred"
	ReasoningSourceCatalog  = "catalog"
)

// models.dev reasoning_options 条目类型。
const (
	ReasoningCatalogOptionEffort = "effort"
	ReasoningCatalogOptionBudget = "budget_tokens"
	ReasoningCatalogOptionToggle = "toggle"
)

// 目录条目无法转换为推理能力的原因，与批量导入结果的 skipped.reason 取值一致。
const (
	ReasoningCatalogReasonNotReasoning        = "not_reasoning"
	ReasoningCatalogReasonUnsupportedProtocol = "unsupported_protocol"
	ReasoningCatalogReasonInvalid             = "invalid"
)

// 预算类格式按 low / medium / high 生成档位时使用的目标预算，随后钳制到目录声明的区间。
var reasoningCatalogBudgetTargets = []struct {
	level  string
	budget int
}{
	{ReasoningLevelLow, 2048},
	{ReasoningLevelMedium, 8192},
	{ReasoningLevelHigh, 24576},
}

// modelCatalogVendorProviders 把平台技术厂商 key 映射到 models.dev 的第一方 provider id；
// 首项同时作为该厂商在 canonical_model_id 中使用的前缀。未登记的厂商直接以自身 key 作为 provider id。
var modelCatalogVendorProviders = map[string][]string{
	"openai":     {"openai"},
	"anthropic":  {"anthropic"},
	"google":     {"google"},
	"meta":       {"meta", "llama"},
	"microsoft":  {"microsoft"},
	"amazon":     {"amazon", "amazon-bedrock"},
	"nvidia":     {"nvidia"},
	"deepseek":   {"deepseek"},
	"moonshot":   {"moonshotai", "moonshotai-cn"},
	"zhipu":      {"zhipuai", "zai"},
	"minimax":    {"minimax", "minimax-cn"},
	"bytedance":  {"bytedance-seed", "volcengine"},
	"tencent":    {"tencent", "tencent-tokenhub"},
	"longcat":    {"meituan", "longcat"},
	"mistral":    {"mistral"},
	"alibaba":    {"alibaba", "alibaba-cn"},
	"qwen":       {"alibaba", "alibaba-cn"},
	"xai":        {"xai"},
	"xiaomi":     {"xiaomi"},
	"stepfun":    {"stepfun", "stepfun-ai"},
	"openrouter": {"openrouter"},
	"copilot":    {"github-copilot"},
}

// modelCatalogAggregatorProviders 是映射表中转售多家模型的 provider，不视为第一方。
var modelCatalogAggregatorProviders = map[string]struct{}{
	"openrouter":     {},
	"github-copilot": {},
	"amazon-bedrock": {},
}

var modelCatalogFirstPartyProviders = func() map[string]struct{} {
	result := make(map[string]struct{})
	for _, providers := range modelCatalogVendorProviders {
		for _, provider := range providers {
			if _, aggregator := modelCatalogAggregatorProviders[provider]; !aggregator {
				result[provider] = struct{}{}
			}
		}
	}
	return result
}()

// ReasoningCatalogOption 是目录条目中的一个 reasoning_options 项；Min/Max 仅对 budget_tokens 有意义。
type ReasoningCatalogOption struct {
	Type   string
	Values []string
	Min    *int
	Max    *int
}

// ModelCatalogEntry 是 models.dev 中单个 provider 下单个模型的精简目录信息：推理能力、输入/输出模态与上下文窗口。
// Reasoning 为 true 时 Options 非空；InputModalities 可独立存在（纯文本模型也会登记）。
type ModelCatalogEntry struct {
	Provider    string
	ModelID     string
	CanonicalID string
	Reasoning   bool
	Options     []ReasoningCatalogOption
	// InputModalities 为 models.dev modalities.input 的归一化结果，见 NormalizeInputModalities。
	InputModalities []string
	// OutputModalities 为 models.dev modalities.output 的归一化结果，仅用于展示。
	OutputModalities []string
	// ContextWindow 为 models.dev limit.context（Token）；未声明或超出平台允许范围时为 0。
	ContextWindow int
}

// SplitCatalogEntries 把目录条目分成推理索引与输入模态索引各自需要的部分。
// 两类查询分别建索引：推理匹配只在有推理选项的条目中进行，与只含推理条目的旧目录行为一致；
// 输入模态匹配则覆盖所有声明了模态的模型（包括纯文本模型，用于判断「不支持图片」）。
func SplitCatalogEntries(entries []ModelCatalogEntry) (reasoning []ModelCatalogEntry, modalities []ModelCatalogEntry) {
	for _, entry := range entries {
		if entry.Reasoning && len(entry.Options) > 0 {
			reasoning = append(reasoning, entry)
		}
		if len(entry.InputModalities) > 0 {
			modalities = append(modalities, entry)
		}
	}
	return reasoning, modalities
}

// ModelCatalog 是按 provider/model、模型 id 与 canonical id 建立的只读内存索引，支持请求链路 O(1) 查找。
type ModelCatalog struct {
	entries         []ModelCatalogEntry
	byProviderModel map[string]int
	byModelID       map[string][]int
	byCanonical     map[string][]int
	byCanonicalName map[string][]int
}

// NewModelCatalog 构建目录索引；provider 与模型 id 忽略大小写，重复条目保留首个。
func NewModelCatalog(entries []ModelCatalogEntry) *ModelCatalog {
	catalog := &ModelCatalog{
		entries:         make([]ModelCatalogEntry, 0, len(entries)),
		byProviderModel: make(map[string]int, len(entries)),
		byModelID:       make(map[string][]int),
		byCanonical:     make(map[string][]int),
		byCanonicalName: make(map[string][]int),
	}
	for _, entry := range entries {
		provider := normalizeModelCatalogKey(entry.Provider)
		modelID := normalizeModelCatalogKey(entry.ModelID)
		if provider == "" || modelID == "" {
			continue
		}
		key := provider + "\x00" + modelID
		if _, exists := catalog.byProviderModel[key]; exists {
			continue
		}
		index := len(catalog.entries)
		catalog.entries = append(catalog.entries, entry)
		catalog.byProviderModel[key] = index
		catalog.byModelID[modelID] = append(catalog.byModelID[modelID], index)
		if canonical := normalizeModelCatalogKey(entry.CanonicalID); canonical != "" {
			catalog.byCanonical[canonical] = append(catalog.byCanonical[canonical], index)
			if slash := strings.LastIndex(canonical, "/"); slash >= 0 && slash < len(canonical)-1 {
				name := canonical[slash+1:]
				catalog.byCanonicalName[name] = append(catalog.byCanonicalName[name], index)
			}
		}
	}
	for _, index := range []map[string][]int{catalog.byModelID, catalog.byCanonical, catalog.byCanonicalName} {
		for key := range index {
			catalog.sortCandidates(index[key])
		}
	}
	return catalog
}

// Len 返回索引中的模型条目数。
func (c *ModelCatalog) Len() int {
	if c == nil {
		return 0
	}
	return len(c.entries)
}

// Entries 返回索引条目副本，用于序列化快照。
func (c *ModelCatalog) Entries() []ModelCatalogEntry {
	if c == nil {
		return nil
	}
	return append([]ModelCatalogEntry(nil), c.entries...)
}

// sortCandidates 让同一查找键下的候选按「第一方优先，再按 provider id 字母序」确定性排序。
func (c *ModelCatalog) sortCandidates(indexes []int) {
	sort.SliceStable(indexes, func(left, right int) bool {
		a, b := c.entries[indexes[left]], c.entries[indexes[right]]
		aFirst, bFirst := modelCatalogIsFirstParty(a), modelCatalogIsFirstParty(b)
		if aFirst != bFirst {
			return aFirst
		}
		aProvider, bProvider := normalizeModelCatalogKey(a.Provider), normalizeModelCatalogKey(b.Provider)
		if aProvider != bProvider {
			return aProvider < bProvider
		}
		return normalizeModelCatalogKey(a.ModelID) < normalizeModelCatalogKey(b.ModelID)
	})
}

// Match 按固定顺序为平台模型查找目录条目，不做模糊匹配：
// (1) 厂商对应 provider 下 id 相同；(2) 任一第一方 provider 下 id 相同；
// (3) canonical_model_id 为「厂商前缀/名称」或以「/名称」结尾；(4) OpenRouter 路由查 openrouter provider。
func (c *ModelCatalog) Match(name string, vendor string, protocolKeys []string) *ModelCatalogEntry {
	if c.Len() == 0 {
		return nil
	}
	fullName := normalizeModelCatalogKey(name)
	if fullName == "" {
		return nil
	}
	bareName, namePrefix := fullName, ""
	if slash := strings.Index(fullName, "/"); slash >= 0 {
		namePrefix = fullName[:slash]
		bareName = fullName[strings.LastIndex(fullName, "/")+1:]
		if bareName == "" {
			return nil
		}
	}
	providers, canonicalPrefixes := modelCatalogVendorTargets(normalizeModelCatalogKey(vendor), namePrefix)

	for _, provider := range providers {
		for _, modelID := range uniqueModelCatalogKeys(bareName, fullName) {
			if entry := c.lookup(provider, modelID); entry != nil {
				return entry
			}
		}
	}
	for _, index := range c.byModelID[bareName] {
		if modelCatalogIsFirstParty(c.entries[index]) {
			return &c.entries[index]
		}
	}
	for _, prefix := range canonicalPrefixes {
		if indexes := c.byCanonical[prefix+"/"+bareName]; len(indexes) > 0 {
			return &c.entries[indexes[0]]
		}
	}
	if indexes := c.byCanonicalName[bareName]; len(indexes) > 0 {
		return &c.entries[indexes[0]]
	}
	if modelCatalogHasOpenRouterProtocol(protocolKeys) {
		candidates := []string{fullName}
		for _, prefix := range canonicalPrefixes {
			candidates = append(candidates, prefix+"/"+bareName)
		}
		for _, modelID := range candidates {
			if entry := c.lookup("openrouter", modelID); entry != nil {
				return entry
			}
		}
	}
	return nil
}

// MatchExternal 为用户自带 Key 的外部模型查找目录条目：先按 Match 的固定顺序，
// 未命中时再按 OpenRouter 的 id 查 openrouter provider（中转站普遍沿用「厂商/模型」与点号版本，
// 如 anthropic/claude-sonnet-4.5），不受协议限制。只用于输入模态与上下文窗口，
// 推理能力仍走 Match，避免把 OpenRouter 的推理参数形态套到其它协议上。
func (c *ModelCatalog) MatchExternal(name string, vendor string, protocolKeys []string) *ModelCatalogEntry {
	if entry := c.Match(name, vendor, protocolKeys); entry != nil {
		return entry
	}
	if c.Len() == 0 {
		return nil
	}
	fullName := normalizeModelCatalogKey(name)
	if fullName == "" {
		return nil
	}
	bareName, namePrefix := fullName, ""
	if slash := strings.Index(fullName, "/"); slash >= 0 {
		namePrefix = fullName[:slash]
		bareName = fullName[strings.LastIndex(fullName, "/")+1:]
		if bareName == "" {
			return nil
		}
	}
	_, canonicalPrefixes := modelCatalogVendorTargets(normalizeModelCatalogKey(vendor), namePrefix)
	candidates := []string{fullName}
	for _, prefix := range canonicalPrefixes {
		candidates = append(candidates, prefix+"/"+bareName)
	}
	for _, modelID := range uniqueModelCatalogKeys(candidates...) {
		if entry := c.lookup("openrouter", modelID); entry != nil {
			return entry
		}
	}
	return nil
}

func (c *ModelCatalog) lookup(provider string, modelID string) *ModelCatalogEntry {
	index, ok := c.byProviderModel[provider+"\x00"+modelID]
	if !ok {
		return nil
	}
	return &c.entries[index]
}

// modelCatalogVendorTargets 返回按优先级排列的 provider 列表与 canonical 前缀列表：先平台厂商，再名称前缀。
func modelCatalogVendorTargets(vendor string, namePrefix string) ([]string, []string) {
	var providers, prefixes []string
	seenProviders := map[string]struct{}{}
	seenPrefixes := map[string]struct{}{}
	for _, key := range []string{vendor, namePrefix} {
		if key == "" || key == "unknown" {
			continue
		}
		mapped, ok := modelCatalogVendorProviders[key]
		if !ok {
			mapped = []string{key}
		}
		for _, provider := range mapped {
			if _, seen := seenProviders[provider]; !seen {
				seenProviders[provider] = struct{}{}
				providers = append(providers, provider)
			}
		}
		if _, seen := seenPrefixes[mapped[0]]; !seen {
			seenPrefixes[mapped[0]] = struct{}{}
			prefixes = append(prefixes, mapped[0])
		}
	}
	return providers, prefixes
}

func uniqueModelCatalogKeys(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		duplicated := false
		for _, existing := range result {
			if existing == value {
				duplicated = true
				break
			}
		}
		if !duplicated {
			result = append(result, value)
		}
	}
	return result
}

// modelCatalogIsFirstParty 判断条目是否来自模型原厂：provider 等于 canonical 前缀，或属于已登记的厂商 provider。
func modelCatalogIsFirstParty(entry ModelCatalogEntry) bool {
	provider := normalizeModelCatalogKey(entry.Provider)
	canonical := normalizeModelCatalogKey(entry.CanonicalID)
	if slash := strings.Index(canonical, "/"); slash > 0 && canonical[:slash] == provider {
		return true
	}
	_, ok := modelCatalogFirstPartyProviders[provider]
	return ok
}

func modelCatalogHasOpenRouterProtocol(protocolKeys []string) bool {
	for _, protocolKey := range protocolKeys {
		if strings.HasPrefix(protocolKey, "openrouter_") {
			return true
		}
	}
	return false
}

func normalizeModelCatalogKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// CatalogReasoningCapability 把目录条目转换为指定协议上的推理能力；失败时返回 nil 与原因。
// 优先使用 effort，其次 budget_tokens，最后 toggle；结果必须通过显式声明的同一套校验。
func CatalogReasoningCapability(entry *ModelCatalogEntry, protocolKey string, vendor string) (*ReasoningCapability, string) {
	if entry == nil || !entry.Reasoning || len(entry.Options) == 0 {
		return nil, ReasoningCatalogReasonNotReasoning
	}
	qwen := reasoningCatalogIsQwen(entry, vendor)
	var payload map[string]any
	if option := entry.option(ReasoningCatalogOptionEffort); option != nil {
		payload = catalogEffortPayload(option.Values, protocolKey)
	}
	if payload == nil {
		if option := entry.option(ReasoningCatalogOptionBudget); option != nil {
			payload = catalogBudgetPayload(*option, protocolKey, qwen)
		}
	}
	if payload == nil && entry.option(ReasoningCatalogOptionToggle) != nil && isReasoningOpenAICompatibleChat(protocolKey) {
		format := ReasoningFormatToggle
		if qwen {
			format = ReasoningFormatQwen
		}
		payload = map[string]any{
			"format":  format,
			"levels":  []any{ReasoningLevelNone, ReasoningLevelHigh},
			"default": ReasoningLevelHigh,
		}
	}
	if payload == nil {
		return nil, ReasoningCatalogReasonUnsupportedProtocol
	}
	capability, err := parseReasoningCapability(payload)
	if err != nil || !ReasoningFormatSupportsProtocol(capability.Format, protocolKey) {
		return nil, ReasoningCatalogReasonInvalid
	}
	return capability, ""
}

// CatalogReasoningCapabilityForProtocols 在模型的全部可用协议上转换目录条目，取对最多协议有效的能力；
// 票数相同时取协议顺序中最先出现者。全部失败时按 invalid > not_reasoning > unsupported_protocol 返回原因。
func CatalogReasoningCapabilityForProtocols(entry *ModelCatalogEntry, protocolKeys []string, vendor string) (*ReasoningCapability, string) {
	if len(protocolKeys) == 0 {
		if entry == nil || !entry.Reasoning || len(entry.Options) == 0 {
			return nil, ReasoningCatalogReasonNotReasoning
		}
		return nil, ReasoningCatalogReasonUnsupportedProtocol
	}
	type candidate struct {
		capability *ReasoningCapability
		votes      int
	}
	var candidates []candidate
	reason := ""
	for _, protocolKey := range protocolKeys {
		capability, failure := CatalogReasoningCapability(entry, protocolKey, vendor)
		if capability == nil {
			reason = worseReasoningCatalogReason(reason, failure)
			continue
		}
		matched := false
		for index := range candidates {
			if reflect.DeepEqual(candidates[index].capability, capability) {
				candidates[index].votes++
				matched = true
				break
			}
		}
		if !matched {
			candidates = append(candidates, candidate{capability: capability, votes: 1})
		}
	}
	if len(candidates) == 0 {
		return nil, reason
	}
	best := candidates[0]
	for _, item := range candidates[1:] {
		if item.votes > best.votes {
			best = item
		}
	}
	return best.capability, ""
}

func worseReasoningCatalogReason(current string, next string) string {
	rank := map[string]int{
		ReasoningCatalogReasonUnsupportedProtocol: 1,
		ReasoningCatalogReasonNotReasoning:        2,
		ReasoningCatalogReasonInvalid:             3,
	}
	if rank[next] > rank[current] {
		return next
	}
	return current
}

func (e *ModelCatalogEntry) option(optionType string) *ReasoningCatalogOption {
	for index := range e.Options {
		if e.Options[index].Type == optionType {
			return &e.Options[index]
		}
	}
	return nil
}

// reasoningCatalogIsQwen 判断条目是否属于通义千问系列。平台厂商已知时以厂商为准：阿里云托管的其他厂商模型
// （如 alibaba-cn 下的 kimi）不应按 enable_thinking 下发。厂商未知时才按 canonical 前缀、provider 或模型 id 推测。
func reasoningCatalogIsQwen(entry *ModelCatalogEntry, vendor string) bool {
	switch normalizeModelCatalogKey(vendor) {
	case "alibaba", "qwen":
		return true
	case "", "unknown":
	default:
		return false
	}
	if canonical := normalizeModelCatalogKey(entry.CanonicalID); canonical != "" {
		return strings.HasPrefix(canonical, "alibaba/") || strings.HasPrefix(canonical, "qwen/") || strings.Contains(canonical, "/qwen")
	}
	return strings.HasPrefix(normalizeModelCatalogKey(entry.Provider), "alibaba") ||
		strings.Contains(normalizeModelCatalogKey(entry.ModelID), "qwen")
}

func isReasoningOpenAICompatibleChat(protocolKey string) bool {
	return protocolKey == ReasoningProtocolOpenAIChat || protocolKey == ReasoningProtocolOpenRouterChat
}

func isReasoningOpenAIProtocol(protocolKey string) bool {
	switch protocolKey {
	case ReasoningProtocolOpenAIChat, ReasoningProtocolOpenRouterChat, ReasoningProtocolOpenAIResponses,
		ReasoningProtocolXAIResponses, ReasoningProtocolOpenRouterResponses:
		return true
	}
	return false
}

// catalogEffortPayload 把 effort 取值映射为协议对应的档位格式；过滤后没有档位时返回 nil，交由后续选项处理。
func catalogEffortPayload(values []string, protocolKey string) map[string]any {
	var format string
	switch {
	case isReasoningOpenAIProtocol(protocolKey):
		format = ReasoningFormatOpenAI
	case protocolKey == ReasoningProtocolAnthropic:
		format = ReasoningFormatAnthropicEffort
	case protocolKey == ReasoningProtocolGeminiGenerate || protocolKey == ReasoningProtocolGeminiInteractions:
		format = ReasoningFormatGeminiLevel
	default:
		return nil
	}
	levels := make([]string, 0, len(values))
	for _, value := range values {
		if level, ok := NormalizeReasoningLevel(value); ok && reasoningFormatAllowsLevel(format, level) {
			levels = append(levels, level)
		}
	}
	levels = uniqueModelCatalogKeys(levels...)
	if len(levels) == 0 {
		return nil
	}
	sortReasoningLevels(levels)
	return map[string]any{
		"format":  format,
		"levels":  reasoningLevelsPayload(levels),
		"default": catalogReasoningDefault(levels),
	}
}

// catalogBudgetPayload 生成预算类能力：目标预算钳制到 [min, max]（Anthropic 下限 1024），钳制后预算相同的档位
// 只保留较高档；gemini_budget 在 min 缺省或为 0 时、qwen 始终额外提供 none（预算 0）。
func catalogBudgetPayload(option ReasoningCatalogOption, protocolKey string, qwen bool) map[string]any {
	var format string
	switch {
	case protocolKey == ReasoningProtocolAnthropic:
		format = ReasoningFormatAnthropicBudget
	case protocolKey == ReasoningProtocolGeminiGenerate:
		format = ReasoningFormatGeminiBudget
	case qwen && isReasoningOpenAICompatibleChat(protocolKey):
		format = ReasoningFormatQwen
	default:
		return nil
	}
	lower := 0
	if option.Min != nil && *option.Min > 0 {
		lower = *option.Min
	}
	if format == ReasoningFormatAnthropicBudget && lower < AnthropicMinThinkingBudget {
		lower = AnthropicMinThinkingBudget
	}
	upper := 0
	if option.Max != nil {
		upper = *option.Max
		if upper < lower || upper <= 0 {
			return nil
		}
	}
	levels := make([]string, 0, len(reasoningCatalogBudgetTargets)+1)
	budgets := map[string]any{}
	if format == ReasoningFormatQwen || (format == ReasoningFormatGeminiBudget && (option.Min == nil || *option.Min == 0)) {
		levels = append(levels, ReasoningLevelNone)
		budgets[ReasoningLevelNone] = float64(0)
	}
	previousLevel, previousBudget := "", -1
	for _, target := range reasoningCatalogBudgetTargets {
		budget := target.budget
		if budget < lower {
			budget = lower
		}
		if upper > 0 && budget > upper {
			budget = upper
		}
		if budget == previousBudget {
			// 钳制后与低一档预算相同，丢弃较低的重复档位。
			levels = levels[:len(levels)-1]
			delete(budgets, previousLevel)
		}
		levels = append(levels, target.level)
		budgets[target.level] = float64(budget)
		previousLevel, previousBudget = target.level, budget
	}
	defaultLevel := catalogReasoningDefault(levels)
	return map[string]any{
		"format":  format,
		"levels":  reasoningLevelsPayload(levels),
		"default": defaultLevel,
		"budgets": budgets,
	}
}

// catalogReasoningDefault 取 medium；没有时取不高于 medium 的最高非 none 档，再次取最低档。
func catalogReasoningDefault(levels []string) string {
	mediumRank := ReasoningLevelRank(ReasoningLevelMedium)
	fallback := ""
	for _, level := range levels {
		if level == ReasoningLevelNone {
			continue
		}
		if ReasoningLevelRank(level) <= mediumRank {
			fallback = level
		}
	}
	if fallback != "" {
		return fallback
	}
	for _, level := range levels {
		if level != ReasoningLevelNone {
			return level
		}
	}
	return levels[0]
}

func reasoningLevelsPayload(levels []string) []any {
	result := make([]any, 0, len(levels))
	for _, level := range levels {
		result = append(result, level)
	}
	return result
}

// WithoutExplicitReasoning 返回移除显式 reasoning 声明后的能力 JSON，用于计算「自动识别」时的生效能力；
// JSON 无法解析时返回空串（视为未配置）。
func WithoutExplicitReasoning(capabilitiesJSON string) string {
	payload, ok := parseCapabilities(capabilitiesJSON)
	if !ok {
		return ""
	}
	if _, declared := payload["reasoning"]; !declared {
		return capabilitiesJSON
	}
	delete(payload, "reasoning")
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// HasExplicitReasoning 判断能力 JSON 是否已显式声明 reasoning 字段（无论是否有效）。
func HasExplicitReasoning(capabilitiesJSON string) bool {
	payload, _ := parseCapabilities(capabilitiesJSON)
	_, declared := explicitReasoningPayload(payload)
	return declared
}
