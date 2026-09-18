package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	openRouterPricingCacheTTL     = 24 * time.Hour
	openRouterPricingCacheVersion = 5
	legacyOfficialPricingField    = "legacy_cache"
)

// openRouterPricingIgnoredFields are provider-side charges that are handled by
// another DEEIX billing path instead of the model token price.
var openRouterPricingIgnoredFields = map[string]struct{}{
	"web_search": {},
}

// openRouterPricingConditionalFields change when a price applies. Dropping
// these conditions and keeping their prices would turn a time-based schedule
// into a token-only tier, so the affected override is skipped as a whole.
var openRouterPricingConditionalFields = map[string]struct{}{
	"utc_days":  {},
	"utc_end":   {},
	"utc_start": {},
}

var (
	// ErrOfficialPricingProviderUnavailable 表示官方定价提供方未完成装配。
	ErrOfficialPricingProviderUnavailable = errors.New("official pricing provider unavailable")
	// ErrOfficialPricingEmpty 表示上游返回的官方定价目录没有有效模型。
	ErrOfficialPricingEmpty = errors.New("official pricing catalog is empty")
	// ErrOfficialPricingCacheUnavailable 表示官方定价缓存仓储未完成装配。
	ErrOfficialPricingCacheUnavailable = errors.New("official pricing cache unavailable")
	// ErrOfficialPricingCacheReadFailed 表示官方定价持久缓存读取失败。
	ErrOfficialPricingCacheReadFailed = errors.New("official pricing cache read failed")
	// ErrOfficialPricingCacheWriteFailed 表示官方定价已刷新但持久缓存写入失败。
	ErrOfficialPricingCacheWriteFailed = errors.New("official pricing cache write failed")
)

type openRouterPricingProvider interface {
	FetchModels(ctx context.Context) ([]byte, error)
}

// OfficialPricingService 编排 OpenRouter 官方定价读取、持久缓存和降级策略。
type OfficialPricingService struct {
	provider openRouterPricingProvider
	cache    repository.OpenRouterPricingCacheRepository
	mu       sync.Mutex
}

// OfficialPricingItem 表示第三方官方模型定价项。
type OfficialPricingItem struct {
	ID                  string
	CanonicalSlug       string
	Name                string
	ContextLength       int
	MaxCompletionTokens int
	Pricing             OfficialUnitPricing
}

// OfficialUnitPricing 表示第三方官方价格字段。UnsupportedFields 仅记录当前
// token 计费模型不会写入的字段；只要基础 prompt/completion 可用，快速配置仍可导入。
type OfficialUnitPricing struct {
	Prompt               string
	Completion           string
	InputCacheRead       string
	InputCacheWrite      string
	CacheWritePriceBasis string
	Overrides            []OfficialPricingOverride
	UnsupportedFields    []string
}

// OfficialPricingOverride 表示 OpenRouter 的一个输入 token 价格覆盖档位。
// MinPromptTokens 与当前项目的阶梯上限语义配合使用：基础档包含该阈值，下一档从阈值之后开始。
type OfficialPricingOverride struct {
	MinPromptTokens int64
	Prompt          string
	Completion      string
	InputCacheRead  string
	InputCacheWrite string
}

// OfficialPricingResult 表示官方定价查询结果及其缓存状态。
type OfficialPricingResult struct {
	FetchedAt time.Time
	Cached    bool
	Stale     bool
	Items     []OfficialPricingItem
}

type openRouterModelsResponse struct {
	Data []openRouterModelItem `json:"data"`
}

type openRouterModelItem struct {
	ID            string                     `json:"id"`
	CanonicalSlug string                     `json:"canonical_slug"`
	Name          string                     `json:"name"`
	ContextLength int                        `json:"context_length"`
	TopProvider   openRouterModelTopProvider `json:"top_provider"`
	Pricing       openRouterModelPricing     `json:"pricing"`
}

type openRouterModelTopProvider struct {
	ContextLength       int `json:"context_length"`
	MaxCompletionTokens int `json:"max_completion_tokens"`
}

type openRouterModelPricing struct {
	Prompt            string
	Completion        string
	InputCacheRead    string
	InputCacheWrite   string
	Overrides         []openRouterPricingOverride
	UnsupportedFields []string
}

type openRouterPricingOverride struct {
	MinPromptTokens int64
	Prompt          string
	Completion      string
	InputCacheRead  string
	InputCacheWrite string
	hasPrompt       bool
	hasCompletion   bool
	hasCacheRead    bool
	hasCacheWrite   bool
}

type openRouterPricingCacheFile struct {
	Version   int                          `json:"version,omitempty"`
	FetchedAt time.Time                    `json:"fetchedAt"`
	Items     []openRouterPricingCacheItem `json:"items"`
}

type openRouterPricingCacheItem struct {
	ID                  string                            `json:"id"`
	CanonicalSlug       string                            `json:"canonicalSlug"`
	Name                string                            `json:"name"`
	ContextLength       int                               `json:"contextLength,omitempty"`
	MaxCompletionTokens int                               `json:"maxCompletionTokens,omitempty"`
	Pricing             openRouterPricingCacheUnitPricing `json:"pricing"`
}

type openRouterPricingCacheUnitPricing struct {
	Prompt               string                           `json:"prompt"`
	Completion           string                           `json:"completion"`
	InputCacheRead       string                           `json:"inputCacheRead"`
	InputCacheWrite      string                           `json:"inputCacheWrite"`
	CacheWritePriceBasis string                           `json:"cacheWritePriceBasis"`
	Overrides            []openRouterPricingCacheOverride `json:"overrides,omitempty"`
	UnsupportedFields    []string                         `json:"unsupportedFields,omitempty"`
}

type openRouterPricingCacheOverride struct {
	MinPromptTokens int64  `json:"minPromptTokens"`
	Prompt          string `json:"prompt"`
	Completion      string `json:"completion"`
	InputCacheRead  string `json:"inputCacheRead"`
	InputCacheWrite string `json:"inputCacheWrite"`
}

func (p *openRouterModelPricing) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*p = openRouterModelPricing{}

	knownFields := map[string]struct{}{
		"prompt":            {},
		"completion":        {},
		"input_cache_read":  {},
		"input_cache_write": {},
		"overrides":         {},
	}
	for key := range fields {
		if _, ok := knownFields[key]; !ok {
			if _, ignored := openRouterPricingIgnoredFields[key]; ignored {
				continue
			}
			p.UnsupportedFields = append(p.UnsupportedFields, key)
		}
	}
	decodeOpenRouterPricingField(fields, "prompt", &p.Prompt, &p.UnsupportedFields, true)
	decodeOpenRouterPricingField(fields, "completion", &p.Completion, &p.UnsupportedFields, true)
	decodeOpenRouterPricingField(fields, "input_cache_read", &p.InputCacheRead, &p.UnsupportedFields, false)
	decodeOpenRouterPricingField(fields, "input_cache_write", &p.InputCacheWrite, &p.UnsupportedFields, false)

	if raw, ok := fields["overrides"]; ok {
		var rawOverrides []json.RawMessage
		if string(raw) != "null" {
			if err := json.Unmarshal(raw, &rawOverrides); err != nil {
				p.UnsupportedFields = append(p.UnsupportedFields, "overrides")
			} else {
				for index, rawOverride := range rawOverrides {
					override, unsupported, usable := parseOpenRouterPricingOverride(rawOverride, index)
					p.UnsupportedFields = append(p.UnsupportedFields, unsupported...)
					if usable && override.MinPromptTokens > 0 {
						p.Overrides = append(p.Overrides, override)
					}
				}
			}
		}
	}

	base := openRouterPricingOverride{
		Prompt:          p.Prompt,
		Completion:      p.Completion,
		InputCacheRead:  p.InputCacheRead,
		InputCacheWrite: p.InputCacheWrite,
	}
	thresholds := make([]int64, 0, len(p.Overrides))
	seen := make(map[int64]bool, len(p.Overrides))
	for _, override := range p.Overrides {
		if !seen[override.MinPromptTokens] {
			thresholds = append(thresholds, override.MinPromptTokens)
			seen[override.MinPromptTokens] = true
		}
	}
	sort.Slice(thresholds, func(i, j int) bool { return thresholds[i] < thresholds[j] })
	effective := make([]openRouterPricingOverride, 0, len(thresholds))
	// Evaluate every interval from base prices. Matching entries overwrite
	// individual keys in source order, including entries sharing a threshold.
	for _, threshold := range thresholds {
		tier := base
		tier.MinPromptTokens = threshold
		for _, override := range p.Overrides {
			if override.MinPromptTokens > threshold {
				continue
			}
			if override.hasPrompt {
				tier.Prompt = override.Prompt
			}
			if override.hasCompletion {
				tier.Completion = override.Completion
			}
			if override.hasCacheRead {
				tier.InputCacheRead = override.InputCacheRead
			}
			if override.hasCacheWrite {
				tier.InputCacheWrite = override.InputCacheWrite
			}
		}
		effective = append(effective, tier)
	}
	p.Overrides = effective
	p.UnsupportedFields = uniqueSortedOfficialPricingFields(p.UnsupportedFields)
	return nil
}

func decodeOpenRouterPricingField(fields map[string]json.RawMessage, key string, target *string, unsupported *[]string, required bool) {
	raw, ok := fields[key]
	if !ok {
		if required {
			*unsupported = append(*unsupported, key)
		}
		return
	}
	value, valid := parseOpenRouterPrice(raw)
	if !valid {
		*unsupported = append(*unsupported, key)
		return
	}
	*target = value
}

func parseOpenRouterPricingOverride(raw json.RawMessage, index int) (openRouterPricingOverride, []string, bool) {
	path := func(field string) string {
		return fmt.Sprintf("overrides[%d].%s", index, field)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return openRouterPricingOverride{}, []string{fmt.Sprintf("overrides[%d]", index)}, false
	}
	unsupported := make([]string, 0)
	usable := true
	knownFields := map[string]struct{}{
		"min_prompt_tokens": {},
		"prompt":            {},
		"completion":        {},
		"input_cache_read":  {},
		"input_cache_write": {},
	}
	for key := range fields {
		if _, ok := knownFields[key]; !ok {
			if _, ignored := openRouterPricingIgnoredFields[key]; ignored {
				continue
			}
			if _, conditional := openRouterPricingConditionalFields[key]; conditional {
				// A known schedule cannot be represented by the token-only
				// tier shape. Retain the base model price, but skip this
				// conditional override rather than applying it unconditionally.
				usable = false
			}
			// A tier can still be represented when OpenRouter adds a provider
			// dimension or another field that this billing form does not store.
			// Record the field for the UI and omit only that field from the
			// normalized override. The minimum token threshold remains usable.
			unsupported = append(unsupported, path(key))
		}
	}

	override := openRouterPricingOverride{}
	minRaw, ok := fields["min_prompt_tokens"]
	if !ok {
		unsupported = append(unsupported, path("min_prompt_tokens"))
		usable = false
	} else if minTokens, valid := parseOpenRouterMinPromptTokens(minRaw); valid {
		override.MinPromptTokens = minTokens
	} else {
		unsupported = append(unsupported, path("min_prompt_tokens"))
		usable = false
	}

	decodeOverridePrice := func(key string, target *string, present *bool) {
		rawValue, exists := fields[key]
		if !exists {
			return
		}
		value, valid := parseOpenRouterPrice(rawValue)
		if !valid {
			unsupported = append(unsupported, path(key))
			return
		}
		*target = value
		*present = true
	}
	decodeOverridePrice("prompt", &override.Prompt, &override.hasPrompt)
	decodeOverridePrice("completion", &override.Completion, &override.hasCompletion)
	decodeOverridePrice("input_cache_read", &override.InputCacheRead, &override.hasCacheRead)
	decodeOverridePrice("input_cache_write", &override.InputCacheWrite, &override.hasCacheWrite)
	return override, uniqueSortedOfficialPricingFields(unsupported), usable
}

func parseOpenRouterPrice(raw json.RawMessage) (string, bool) {
	value, err := decodeOpenRouterJSONScalar(raw)
	if err != nil {
		return "", false
	}
	var text string
	switch typed := value.(type) {
	case string:
		text = strings.TrimSpace(typed)
	case json.Number:
		text = typed.String()
	default:
		return "", false
	}
	if text == "" {
		return "", false
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
		return "", false
	}
	return text, true
}

func parseOpenRouterMinPromptTokens(raw json.RawMessage) (int64, bool) {
	value, err := decodeOpenRouterJSONScalar(raw)
	if err != nil {
		return 0, false
	}
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseInt(number.String(), 10, 64)
	return parsed, err == nil && parsed > 0
}

func decodeOpenRouterJSONScalar(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func uniqueSortedOfficialPricingFields(fields []string) []string {
	if len(fields) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(fields))
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		result = append(result, field)
	}
	sort.Strings(result)
	return result
}

func isOpenRouterAnthropicModelID(modelID string) bool {
	normalized := strings.ToLower(strings.TrimSpace(modelID))
	return strings.HasPrefix(normalized, "anthropic/") || strings.HasPrefix(normalized, "~anthropic/")
}

func openRouterCacheWritePriceBasis(modelID string) string {
	if isOpenRouterAnthropicModelID(modelID) {
		return domainbilling.CacheWritePriceBasisAnthropic5m
	}
	return domainbilling.CacheWritePriceBasisDirect
}

// NewOfficialPricingService 创建依赖完整的官方定价应用服务。
func NewOfficialPricingService(provider openRouterPricingProvider, cache repository.OpenRouterPricingCacheRepository) *OfficialPricingService {
	return &OfficialPricingService{provider: provider, cache: cache}
}

// GetOpenRouterOfficialPricing 按缓存策略获取 OpenRouter 官方模型定价。
// 同一进程内串行刷新，避免缓存过期时并发请求击穿上游。
func (s *OfficialPricingService) GetOpenRouterOfficialPricing(ctx context.Context, refresh bool) (OfficialPricingResult, error) {
	if s == nil || s.cache == nil {
		return OfficialPricingResult{}, ErrOfficialPricingCacheUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cache, cacheOK, err := s.loadOpenRouterPricingCache(ctx)
	if err != nil {
		return OfficialPricingResult{}, fmt.Errorf("%w: %w", ErrOfficialPricingCacheReadFailed, err)
	}
	cacheCurrent := cache.Version >= openRouterPricingCacheVersion
	if cacheOK && cacheCurrent && !refresh && !openRouterOfficialPricingCacheStale(cache.FetchedAt) {
		return officialPricingResultFromCache(cache, true, false), nil
	}

	items, err := s.FetchOpenRouterOfficialPricing(ctx)
	if err != nil {
		if cacheOK {
			return officialPricingResultFromCache(cache, true, true), nil
		}
		return OfficialPricingResult{}, err
	}

	nextCache := openRouterPricingCacheFile{
		Version:   openRouterPricingCacheVersion,
		FetchedAt: time.Now().UTC(),
		Items:     officialPricingCacheItems(items),
	}
	data, err := json.MarshalIndent(nextCache, "", "  ")
	if err != nil {
		return OfficialPricingResult{}, fmt.Errorf("encode openrouter official pricing cache: %w", err)
	}
	if err := s.cache.Store(ctx, data); err != nil {
		return OfficialPricingResult{}, fmt.Errorf("%w: %w", ErrOfficialPricingCacheWriteFailed, err)
	}
	return OfficialPricingResult{
		FetchedAt: nextCache.FetchedAt,
		Items:     append([]OfficialPricingItem(nil), items...),
	}, nil
}

func (s *OfficialPricingService) loadOpenRouterPricingCache(ctx context.Context) (openRouterPricingCacheFile, bool, error) {
	data, found, err := s.cache.Load(ctx)
	if err != nil {
		return openRouterPricingCacheFile{}, false, err
	}
	if !found {
		return openRouterPricingCacheFile{}, false, nil
	}
	var cache openRouterPricingCacheFile
	if err := json.Unmarshal(data, &cache); err != nil || cache.FetchedAt.IsZero() || len(cache.Items) == 0 {
		return openRouterPricingCacheFile{}, false, nil
	}
	return cache, true, nil
}

func officialPricingResultFromCache(cache openRouterPricingCacheFile, cached bool, stale bool) OfficialPricingResult {
	legacyCache := cache.Version < openRouterPricingCacheVersion
	items := make([]OfficialPricingItem, 0, len(cache.Items))
	for _, item := range cache.Items {
		unsupportedFields := filterIgnoredOfficialPricingFields(item.Pricing.UnsupportedFields)
		if legacyCache {
			unsupportedFields = append(unsupportedFields, legacyOfficialPricingField)
			item.Pricing.CacheWritePriceBasis = openRouterCacheWritePriceBasis(item.ID)
			// v3/v4 rewrote Anthropic cache prices and lost source precision.
			// Old materialized tiers also lost override precedence. Neither
			// can be recovered reliably without a successful upstream refresh.
			if cache.Version >= 3 && isOpenRouterAnthropicModelID(item.ID) {
				item.Pricing.InputCacheWrite = ""
				unsupportedFields = append(unsupportedFields, "input_cache_write")
			}
			if len(item.Pricing.Overrides) > 0 {
				item.Pricing.Overrides = nil
				unsupportedFields = append(unsupportedFields, "overrides")
			}
			unsupportedFields = uniqueSortedOfficialPricingFields(unsupportedFields)
		}
		items = append(items, OfficialPricingItem{
			ID:                  item.ID,
			CanonicalSlug:       item.CanonicalSlug,
			Name:                item.Name,
			ContextLength:       item.ContextLength,
			MaxCompletionTokens: item.MaxCompletionTokens,
			Pricing: OfficialUnitPricing{
				Prompt:               item.Pricing.Prompt,
				Completion:           item.Pricing.Completion,
				InputCacheRead:       item.Pricing.InputCacheRead,
				InputCacheWrite:      item.Pricing.InputCacheWrite,
				CacheWritePriceBasis: item.Pricing.CacheWritePriceBasis,
				Overrides:            officialPricingOverridesFromCache(item.Pricing.Overrides),
				UnsupportedFields:    unsupportedFields,
			},
		})
	}
	return OfficialPricingResult{
		FetchedAt: cache.FetchedAt,
		Cached:    cached,
		Stale:     stale,
		Items:     items,
	}
}

func filterIgnoredOfficialPricingFields(fields []string) []string {
	if len(fields) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(fields))
	for _, field := range fields {
		if _, ignored := openRouterPricingIgnoredFields[field]; ignored {
			continue
		}
		filtered = append(filtered, field)
	}
	return uniqueSortedOfficialPricingFields(filtered)
}

func officialPricingCacheItems(items []OfficialPricingItem) []openRouterPricingCacheItem {
	result := make([]openRouterPricingCacheItem, 0, len(items))
	for _, item := range items {
		result = append(result, openRouterPricingCacheItem{
			ID:                  item.ID,
			CanonicalSlug:       item.CanonicalSlug,
			Name:                item.Name,
			ContextLength:       item.ContextLength,
			MaxCompletionTokens: item.MaxCompletionTokens,
			Pricing: openRouterPricingCacheUnitPricing{
				Prompt:               item.Pricing.Prompt,
				Completion:           item.Pricing.Completion,
				InputCacheRead:       item.Pricing.InputCacheRead,
				InputCacheWrite:      item.Pricing.InputCacheWrite,
				CacheWritePriceBasis: item.Pricing.CacheWritePriceBasis,
				Overrides:            officialPricingCacheOverrides(item.Pricing.Overrides),
				UnsupportedFields:    filterIgnoredOfficialPricingFields(item.Pricing.UnsupportedFields),
			},
		})
	}
	return result
}

func openRouterOfficialPricingCacheStale(fetchedAt time.Time) bool {
	return fetchedAt.IsZero() || time.Since(fetchedAt) > openRouterPricingCacheTTL
}

// FetchOpenRouterOfficialPricing 获取并规范化 OpenRouter 官方模型定价。
func (s *OfficialPricingService) FetchOpenRouterOfficialPricing(ctx context.Context) ([]OfficialPricingItem, error) {
	if s == nil || s.provider == nil {
		return nil, ErrOfficialPricingProviderUnavailable
	}
	body, err := s.provider.FetchModels(ctx)
	if err != nil {
		return nil, err
	}
	var payload openRouterModelsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	items := make([]OfficialPricingItem, 0, len(payload.Data))
	for _, item := range payload.Data {
		normalized := normalizeOpenRouterOfficialPricingItem(item)
		if normalized.ID != "" {
			items = append(items, normalized)
		}
	}
	if len(items) == 0 {
		return nil, ErrOfficialPricingEmpty
	}
	return items, nil
}

func normalizeOpenRouterOfficialPricingItem(item openRouterModelItem) OfficialPricingItem {
	id := strings.TrimSpace(item.ID)
	if id == "" {
		return OfficialPricingItem{}
	}
	canonicalSlug := strings.TrimSpace(item.CanonicalSlug)
	if canonicalSlug == "" {
		canonicalSlug = id
	}
	name := strings.TrimSpace(item.Name)
	if name == "" {
		name = id
	}
	contextLength := item.TopProvider.ContextLength
	if contextLength <= 0 {
		contextLength = item.ContextLength
	}
	if contextLength < 0 {
		contextLength = 0
	}
	maxCompletionTokens := item.TopProvider.MaxCompletionTokens
	if maxCompletionTokens < 0 {
		maxCompletionTokens = 0
	}
	unsupportedFields := filterIgnoredOfficialPricingFields(item.Pricing.UnsupportedFields)
	if strings.TrimSpace(item.Pricing.Prompt) == "" {
		unsupportedFields = append(unsupportedFields, "prompt")
	}
	if strings.TrimSpace(item.Pricing.Completion) == "" {
		unsupportedFields = append(unsupportedFields, "completion")
	}
	return OfficialPricingItem{
		ID:                  id,
		CanonicalSlug:       canonicalSlug,
		Name:                name,
		ContextLength:       contextLength,
		MaxCompletionTokens: maxCompletionTokens,
		Pricing: OfficialUnitPricing{
			Prompt:               strings.TrimSpace(item.Pricing.Prompt),
			Completion:           strings.TrimSpace(item.Pricing.Completion),
			InputCacheRead:       strings.TrimSpace(item.Pricing.InputCacheRead),
			InputCacheWrite:      strings.TrimSpace(item.Pricing.InputCacheWrite),
			CacheWritePriceBasis: openRouterCacheWritePriceBasis(id),
			Overrides:            officialPricingOverrides(item.Pricing.Overrides),
			UnsupportedFields:    uniqueSortedOfficialPricingFields(unsupportedFields),
		},
	}
}

func officialPricingOverrides(overrides []openRouterPricingOverride) []OfficialPricingOverride {
	if len(overrides) == 0 {
		return nil
	}
	result := make([]OfficialPricingOverride, 0, len(overrides))
	for _, override := range overrides {
		if override.MinPromptTokens <= 0 {
			continue
		}
		result = append(result, OfficialPricingOverride{
			MinPromptTokens: override.MinPromptTokens,
			Prompt:          override.Prompt,
			Completion:      override.Completion,
			InputCacheRead:  override.InputCacheRead,
			InputCacheWrite: override.InputCacheWrite,
		})
	}
	return result
}

func officialPricingCacheOverrides(overrides []OfficialPricingOverride) []openRouterPricingCacheOverride {
	if len(overrides) == 0 {
		return nil
	}
	result := make([]openRouterPricingCacheOverride, 0, len(overrides))
	for _, override := range overrides {
		result = append(result, openRouterPricingCacheOverride(override))
	}
	return result
}

func officialPricingOverridesFromCache(overrides []openRouterPricingCacheOverride) []OfficialPricingOverride {
	if len(overrides) == 0 {
		return nil
	}
	result := make([]OfficialPricingOverride, 0, len(overrides))
	for _, override := range overrides {
		result = append(result, OfficialPricingOverride(override))
	}
	return result
}
