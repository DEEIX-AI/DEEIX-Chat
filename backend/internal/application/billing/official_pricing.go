package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	openRouterPricingCacheTTL     = 24 * time.Hour
	openRouterPricingCacheVersion = 2
)

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

// OfficialUnitPricing 表示第三方官方价格字段。
type OfficialUnitPricing struct {
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
	Prompt          string `json:"prompt"`
	Completion      string `json:"completion"`
	InputCacheRead  string `json:"input_cache_read"`
	InputCacheWrite string `json:"input_cache_write"`
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
	Prompt          string `json:"prompt"`
	Completion      string `json:"completion"`
	InputCacheRead  string `json:"inputCacheRead"`
	InputCacheWrite string `json:"inputCacheWrite"`
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
	items := make([]OfficialPricingItem, 0, len(cache.Items))
	for _, item := range cache.Items {
		items = append(items, OfficialPricingItem{
			ID:                  item.ID,
			CanonicalSlug:       item.CanonicalSlug,
			Name:                item.Name,
			ContextLength:       item.ContextLength,
			MaxCompletionTokens: item.MaxCompletionTokens,
			Pricing: OfficialUnitPricing{
				Prompt:          item.Pricing.Prompt,
				Completion:      item.Pricing.Completion,
				InputCacheRead:  item.Pricing.InputCacheRead,
				InputCacheWrite: item.Pricing.InputCacheWrite,
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
				Prompt:          item.Pricing.Prompt,
				Completion:      item.Pricing.Completion,
				InputCacheRead:  item.Pricing.InputCacheRead,
				InputCacheWrite: item.Pricing.InputCacheWrite,
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
	return OfficialPricingItem{
		ID:                  id,
		CanonicalSlug:       canonicalSlug,
		Name:                name,
		ContextLength:       contextLength,
		MaxCompletionTokens: maxCompletionTokens,
		Pricing: OfficialUnitPricing{
			Prompt:          strings.TrimSpace(item.Pricing.Prompt),
			Completion:      strings.TrimSpace(item.Pricing.Completion),
			InputCacheRead:  strings.TrimSpace(item.Pricing.InputCacheRead),
			InputCacheWrite: strings.TrimSpace(item.Pricing.InputCacheWrite),
		},
	}
}
