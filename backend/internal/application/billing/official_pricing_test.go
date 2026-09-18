package billing

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
)

type officialPricingProviderStub struct {
	payload []byte
	err     error
	calls   int
}

func (s *officialPricingProviderStub) FetchModels(context.Context) ([]byte, error) {
	s.calls++
	return s.payload, s.err
}

type officialPricingCacheStub struct {
	data    []byte
	found   bool
	loadErr error
	stored  []byte
}

func (s *officialPricingCacheStub) Load(context.Context) ([]byte, bool, error) {
	return s.data, s.found, s.loadErr
}

func TestGetOpenRouterOfficialPricingSurfacesCacheReadFailure(t *testing.T) {
	service := NewOfficialPricingService(
		&officialPricingProviderStub{err: errors.New("must not fetch")},
		&officialPricingCacheStub{loadErr: errors.New("permission denied")},
	)

	_, err := service.GetOpenRouterOfficialPricing(t.Context(), false)
	if !errors.Is(err, ErrOfficialPricingCacheReadFailed) {
		t.Fatalf("error = %v, want ErrOfficialPricingCacheReadFailed", err)
	}
}

func (s *officialPricingCacheStub) Store(_ context.Context, data []byte) error {
	s.stored = append([]byte(nil), data...)
	return nil
}

func TestFetchOpenRouterOfficialPricingNormalizesCatalog(t *testing.T) {
	service := NewOfficialPricingService(&officialPricingProviderStub{payload: []byte(`{
		"data": [
			{
				"id": " openai/gpt-test ",
				"canonical_slug": "",
				"name": "",
				"context_length": 128000,
				"top_provider": {"context_length": 131072, "max_completion_tokens": 16384},
				"pricing": {"prompt": " 0.1 ", "completion": " 0.2 "}
			},
			{"id": "   ", "name": "ignored"}
		]
	}`)}, nil)

	items, err := service.FetchOpenRouterOfficialPricing(context.Background())
	if err != nil {
		t.Fatalf("fetch official pricing: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one valid item, got %d", len(items))
	}
	item := items[0]
	if item.ID != "openai/gpt-test" || item.CanonicalSlug != item.ID || item.Name != item.ID {
		t.Fatalf("unexpected normalized identity: %#v", item)
	}
	if item.Pricing.Prompt != "0.1" || item.Pricing.Completion != "0.2" {
		t.Fatalf("unexpected normalized pricing: %#v", item.Pricing)
	}
	if item.ContextLength != 131_072 || item.MaxCompletionTokens != 16_384 {
		t.Fatalf("unexpected model limits: %#v", item)
	}
}

func TestFetchOpenRouterOfficialPricingNormalizesTokenOverrides(t *testing.T) {
	service := NewOfficialPricingService(&officialPricingProviderStub{payload: []byte(`{
		"data": [{
			"id": "qwen/qwen-test",
			"pricing": {
				"prompt": "0.0000025",
				"completion": "0.00001",
				"input_cache_read": "0.0000005",
				"input_cache_write": "0.000001",
				"overrides": [
					{"min_prompt_tokens": 256000, "prompt": "0.000005"},
					{"min_prompt_tokens": 200000, "completion": "0.00003", "input_cache_read": "0"}
				]
			}
		}]
	}`)}, nil)

	items, err := service.FetchOpenRouterOfficialPricing(context.Background())
	if err != nil {
		t.Fatalf("fetch official pricing: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	overrides := items[0].Pricing.Overrides
	if len(overrides) != 2 {
		t.Fatalf("overrides = %#v, want two sorted overrides", overrides)
	}
	if overrides[0].MinPromptTokens != 200000 || overrides[1].MinPromptTokens != 256000 {
		t.Fatalf("override thresholds = %#v, want 200000 and 256000", overrides)
	}
	if overrides[0].Prompt != "0.0000025" || overrides[0].Completion != "0.00003" {
		t.Fatalf("first override did not inherit/retain prices: %#v", overrides[0])
	}
	if overrides[0].InputCacheRead != "0" || overrides[0].InputCacheWrite != "0.000001" {
		t.Fatalf("first override cache prices = %#v", overrides[0])
	}
	if overrides[1].Prompt != "0.000005" || overrides[1].Completion != "0.00003" || overrides[1].InputCacheRead != "0" || overrides[1].InputCacheWrite != "0.000001" {
		t.Fatalf("second override did not inherit/retain prices: %#v", overrides[1])
	}
	if len(items[0].Pricing.UnsupportedFields) != 0 {
		t.Fatalf("unsupported fields = %#v, want none", items[0].Pricing.UnsupportedFields)
	}
}

func TestFetchOpenRouterOfficialPricingKeepsTokenPricesWithMultimodalFields(t *testing.T) {
	service := NewOfficialPricingService(&officialPricingProviderStub{payload: []byte(`{
		"data": [{
			"id": "google/gemini-test",
			"pricing": {
				"prompt": "0.00000125",
				"completion": "0.00001",
				"input_cache_read": "0.000000125",
				"input_cache_write": "0.000000375",
				"image": "0.00000125",
				"audio": "0.00000125",
				"input_audio_cache": "0.000000125",
				"internal_reasoning": "0.00001",
				"overrides": [{
					"min_prompt_tokens": 200000,
					"prompt": "0.0000025",
					"completion": "0.000015",
					"audio": "0.0000025",
					"future_dimension": "0.000003"
				}]
			}
		}]
	}`)}, nil)

	items, err := service.FetchOpenRouterOfficialPricing(context.Background())
	if err != nil {
		t.Fatalf("fetch official pricing: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	pricing := items[0].Pricing
	if pricing.Prompt != "0.00000125" || pricing.Completion != "0.00001" || len(pricing.Overrides) != 1 {
		t.Fatalf("usable token prices were not preserved: %#v", pricing)
	}
	if len(pricing.UnsupportedFields) == 0 {
		t.Fatalf("multimodal fields were not reported: %#v", pricing)
	}
	for _, field := range []string{"audio", "image", "input_audio_cache", "internal_reasoning", "overrides[0].audio", "overrides[0].future_dimension"} {
		found := false
		for _, unsupported := range pricing.UnsupportedFields {
			if unsupported == field {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("unsupported field %q missing from %#v", field, pricing.UnsupportedFields)
		}
	}
}

func TestFetchOpenRouterOfficialPricingDropsTimeBasedOverridesButKeepsBasePrice(t *testing.T) {
	service := NewOfficialPricingService(&officialPricingProviderStub{payload: []byte(`{
		"data": [{
			"id": "tencent/time-based-test",
			"pricing": {
				"prompt": "0.000001",
				"completion": "0.000004",
				"overrides": [{
					"prompt": "0.0000005",
					"completion": "0.000002",
					"utc_start": 0,
					"utc_end": 1600
				}]
			}
		}]
	}`)}, nil)

	items, err := service.FetchOpenRouterOfficialPricing(context.Background())
	if err != nil {
		t.Fatalf("fetch official pricing: %v", err)
	}
	if len(items) != 1 || items[0].Pricing.Prompt != "0.000001" || len(items[0].Pricing.Overrides) != 0 {
		t.Fatalf("time-based override was not omitted safely: %#v", items)
	}
	for _, field := range items[0].Pricing.UnsupportedFields {
		if field == "overrides[0].utc_start" || field == "overrides[0].utc_end" {
			return
		}
	}
	t.Fatalf("time-based fields were not reported: %#v", items[0].Pricing.UnsupportedFields)
}

func TestFetchOpenRouterOfficialPricingKeepsTokenOverrideWithUnknownExtraField(t *testing.T) {
	service := NewOfficialPricingService(&officialPricingProviderStub{payload: []byte(`{
		"data": [{
			"id": "openai/future-pricing-test",
			"pricing": {
				"prompt": "0.000001",
				"completion": "0.000004",
				"overrides": [{
					"min_prompt_tokens": 200000,
					"prompt": "0.000002",
					"completion": "0.000008",
					"future_dimension": "0.000003"
				}]
			}
		}]
	}`)}, nil)

	items, err := service.FetchOpenRouterOfficialPricing(context.Background())
	if err != nil {
		t.Fatalf("fetch official pricing: %v", err)
	}
	if len(items) != 1 || len(items[0].Pricing.Overrides) != 1 {
		t.Fatalf("usable token override was dropped: %#v", items)
	}
	if items[0].Pricing.Overrides[0].MinPromptTokens != 200000 || items[0].Pricing.Overrides[0].Prompt != "0.000002" {
		t.Fatalf("token override was not preserved: %#v", items[0].Pricing.Overrides[0])
	}
	if len(items[0].Pricing.UnsupportedFields) != 1 || items[0].Pricing.UnsupportedFields[0] != "overrides[0].future_dimension" {
		t.Fatalf("unknown field was not reported: %#v", items[0].Pricing.UnsupportedFields)
	}
}

func TestFetchOpenRouterOfficialPricingSkipsScheduledOverrideEvenWithTokenThreshold(t *testing.T) {
	service := NewOfficialPricingService(&officialPricingProviderStub{payload: []byte(`{
		"data": [{
			"id": "openai/scheduled-pricing-test",
			"pricing": {
				"prompt": "0.000001",
				"completion": "0.000004",
				"overrides": [{
					"min_prompt_tokens": 200000,
					"prompt": "0.000002",
					"completion": "0.000008",
					"utc_start": 0,
					"utc_end": 1600
				}]
			}
		}]
	}`)}, nil)

	items, err := service.FetchOpenRouterOfficialPricing(context.Background())
	if err != nil {
		t.Fatalf("fetch official pricing: %v", err)
	}
	if len(items) != 1 || len(items[0].Pricing.Overrides) != 0 {
		t.Fatalf("scheduled override was imported as unconditional tier: %#v", items)
	}
	for _, field := range []string{"overrides[0].utc_start", "overrides[0].utc_end"} {
		found := false
		for _, unsupported := range items[0].Pricing.UnsupportedFields {
			if unsupported == field {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("scheduled field %q was not reported: %#v", field, items[0].Pricing.UnsupportedFields)
		}
	}
}

func TestFetchOpenRouterOfficialPricingPreservesAnthropicCacheWritePrices(t *testing.T) {
	service := NewOfficialPricingService(&officialPricingProviderStub{payload: []byte(`{
		"data": [{
			"id": "anthropic/claude-test",
			"pricing": {
				"prompt": "0.000003",
				"completion": "0.000015",
				"input_cache_read": "0.0000003",
				"input_cache_write": "0.00000375",
				"input_cache_write_1h": "0.000006",
				"overrides": [{
					"min_prompt_tokens": 200000,
					"prompt": "0.000006",
					"completion": "0.0000225",
					"input_cache_read": "0.0000006",
					"input_cache_write": "0.0000075",
					"input_cache_write_1h": "0.000012"
				}]
			}
		}]
	}`)}, nil)

	items, err := service.FetchOpenRouterOfficialPricing(context.Background())
	if err != nil {
		t.Fatalf("fetch official pricing: %v", err)
	}
	pricing := items[0].Pricing
	if pricing.InputCacheWrite != "0.00000375" || pricing.CacheWritePriceBasis != "anthropic_5m" {
		t.Fatalf("cache write price or basis changed: %#v", pricing)
	}
	if len(pricing.Overrides) != 1 || pricing.Overrides[0].InputCacheWrite != "0.0000075" {
		t.Fatalf("override cache write price changed: %#v", pricing.Overrides)
	}
	if !reflect.DeepEqual(pricing.UnsupportedFields, []string{"input_cache_write_1h", "overrides[0].input_cache_write_1h"}) {
		t.Fatalf("unexpected ignored fields: %#v", pricing.UnsupportedFields)
	}
}

func TestFetchOpenRouterOfficialPricingAcceptsRoundedAnthropicCacheWritePrice(t *testing.T) {
	service := NewOfficialPricingService(&officialPricingProviderStub{payload: []byte(`{
		"data": [{
			"id": "anthropic/claude-3-haiku",
			"pricing": {
				"prompt": "0.00000025",
				"completion": "0.00000125",
				"input_cache_write": "0.0000003",
				"input_cache_write_1h": "0.0000005"
			}
		}]
	}`)}, nil)

	items, err := service.FetchOpenRouterOfficialPricing(context.Background())
	if err != nil {
		t.Fatalf("fetch official pricing: %v", err)
	}
	if items[0].Pricing.InputCacheWrite != "0.0000003" {
		t.Fatalf("published Anthropic cache pricing was changed: %#v", items[0].Pricing)
	}
}

func TestFetchOpenRouterOfficialPricingPreservesCustomAnthropicCacheWritePrices(t *testing.T) {
	service := NewOfficialPricingService(&officialPricingProviderStub{payload: []byte(`{
		"data": [{
			"id": "anthropic/claude-test",
			"pricing": {
				"prompt": "0.000003",
				"completion": "0.000015",
				"input_cache_write": "0.000005",
				"input_cache_write_1h": "0.000007"
			}
		}]
	}`)}, nil)

	items, err := service.FetchOpenRouterOfficialPricing(context.Background())
	if err != nil {
		t.Fatalf("fetch official pricing: %v", err)
	}
	if items[0].Pricing.InputCacheWrite != "0.000005" {
		t.Fatalf("custom cache price was changed: %#v", items[0].Pricing)
	}
	want := []string{"input_cache_write_1h"}
	if len(items[0].Pricing.UnsupportedFields) != len(want) {
		t.Fatalf("unsupported fields = %#v, want %#v", items[0].Pricing.UnsupportedFields, want)
	}
	for index, field := range want {
		if items[0].Pricing.UnsupportedFields[index] != field {
			t.Fatalf("unsupported fields = %#v, want %#v", items[0].Pricing.UnsupportedFields, want)
		}
	}
}

func TestFetchOpenRouterOfficialPricingMarksUnsupportedPricingFields(t *testing.T) {
	service := NewOfficialPricingService(&officialPricingProviderStub{payload: []byte(`{
		"data": [{
			"id": "openai/audio-test",
			"pricing": {
				"prompt": "0.1",
				"completion": "0.2",
				"web_search": "0.01",
				"overrides": [{"min_prompt_tokens": 200000, "prompt": "0.3", "utc_start": 9}]
			}
		}]
	}`)}, nil)

	items, err := service.FetchOpenRouterOfficialPricing(context.Background())
	if err != nil {
		t.Fatalf("fetch official pricing: %v", err)
	}
	fields := items[0].Pricing.UnsupportedFields
	want := []string{"overrides[0].utc_start"}
	if len(fields) != len(want) {
		t.Fatalf("unsupported fields = %#v, want %#v", fields, want)
	}
	for index, field := range want {
		if fields[index] != field {
			t.Fatalf("unsupported fields = %#v, want %#v", fields, want)
		}
	}
}

func TestFetchOpenRouterOfficialPricingIgnoresNativeToolCharges(t *testing.T) {
	service := NewOfficialPricingService(&officialPricingProviderStub{payload: []byte(`{
		"data": [{
			"id": "openai/gpt-test",
			"pricing": {
				"prompt": "0.00001",
				"completion": "0.00005",
				"web_search": "0.01",
				"overrides": [{
					"min_prompt_tokens": 272000,
					"prompt": "0.00002",
					"completion": "0.000075",
					"web_search": "0.01"
				}]
			}
		}]
	}`)}, nil)

	items, err := service.FetchOpenRouterOfficialPricing(context.Background())
	if err != nil {
		t.Fatalf("fetch official pricing: %v", err)
	}
	if len(items) != 1 || len(items[0].Pricing.UnsupportedFields) != 0 {
		t.Fatalf("native tool charge blocked model import: %#v", items)
	}
	if len(items[0].Pricing.Overrides) != 1 || items[0].Pricing.Overrides[0].MinPromptTokens != 272000 {
		t.Fatalf("token override was not preserved: %#v", items[0].Pricing.Overrides)
	}
}

func TestGetOpenRouterOfficialPricingUsesFreshPersistentCache(t *testing.T) {
	provider := &officialPricingProviderStub{err: errors.New("must not fetch")}
	cache := &officialPricingCacheStub{
		found: true,
		data: []byte(`{
			"version":5,
			"fetchedAt":"` + time.Now().UTC().Format(time.RFC3339Nano) + `",
			"items":[{"id":"openai/gpt-test","canonicalSlug":"openai/gpt-test","name":"GPT Test","contextLength":128000,"maxCompletionTokens":8192,"pricing":{}}]
		}`),
	}
	service := NewOfficialPricingService(provider, cache)

	result, err := service.GetOpenRouterOfficialPricing(t.Context(), false)
	if err != nil {
		t.Fatalf("get official pricing: %v", err)
	}
	if !result.Cached || result.Stale || len(result.Items) != 1 {
		t.Fatalf("unexpected cache result: %#v", result)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
	if result.Items[0].ContextLength != 128_000 || result.Items[0].MaxCompletionTokens != 8_192 {
		t.Fatalf("cached model limits were not preserved: %#v", result.Items[0])
	}
}

func TestGetOpenRouterOfficialPricingRestoresOverridesFromPersistentCache(t *testing.T) {
	cache := &officialPricingCacheStub{
		found: true,
		data: []byte(`{
			"version":5,
			"fetchedAt":"` + time.Now().UTC().Format(time.RFC3339Nano) + `",
			"items":[{"id":"qwen/qwen-test","pricing":{"prompt":"0.1","completion":"0.2","overrides":[{"minPromptTokens":200000,"prompt":"0.3","completion":"0.4"}],"unsupportedFields":["web_search"]}}]
		}`),
	}
	service := NewOfficialPricingService(&officialPricingProviderStub{err: errors.New("must not fetch")}, cache)

	result, err := service.GetOpenRouterOfficialPricing(t.Context(), false)
	if err != nil {
		t.Fatalf("get official pricing: %v", err)
	}
	if len(result.Items) != 1 || len(result.Items[0].Pricing.Overrides) != 1 {
		t.Fatalf("cached overrides were not restored: %#v", result.Items)
	}
	if result.Items[0].Pricing.Overrides[0].MinPromptTokens != 200000 || len(result.Items[0].Pricing.UnsupportedFields) != 0 {
		t.Fatalf("cached pricing metadata was not restored: %#v", result.Items[0].Pricing)
	}
}

func TestGetOpenRouterOfficialPricingMarksLegacyCacheFallback(t *testing.T) {
	provider := &officialPricingProviderStub{err: errors.New("upstream unavailable")}
	cache := &officialPricingCacheStub{
		found: true,
		data: []byte(`{
			"version":2,
			"fetchedAt":"2020-01-01T00:00:00Z",
			"items":[{"id":"openai/gpt-test","pricing":{"prompt":"0.1","completion":"0.2"}}]
		}`),
	}
	service := NewOfficialPricingService(provider, cache)

	result, err := service.GetOpenRouterOfficialPricing(t.Context(), false)
	if err != nil {
		t.Fatalf("get legacy fallback pricing: %v", err)
	}
	if len(result.Items) != 1 || len(result.Items[0].Pricing.UnsupportedFields) != 1 || result.Items[0].Pricing.UnsupportedFields[0] != legacyOfficialPricingField {
		t.Fatalf("legacy cache fallback was not marked: %#v", result.Items)
	}
}

func TestGetOpenRouterOfficialPricingRefreshesLegacyCacheWithoutModelLimits(t *testing.T) {
	provider := &officialPricingProviderStub{payload: []byte(`{
		"data":[{"id":"openai/gpt-test","context_length":128000,"pricing":{}}]
	}`)}
	cache := &officialPricingCacheStub{
		found: true,
		data: []byte(`{
			"fetchedAt":"` + time.Now().UTC().Format(time.RFC3339Nano) + `",
			"items":[{"id":"openai/gpt-test","canonicalSlug":"openai/gpt-test","name":"GPT Test","pricing":{}}]
		}`),
	}
	service := NewOfficialPricingService(provider, cache)

	result, err := service.GetOpenRouterOfficialPricing(t.Context(), false)
	if err != nil {
		t.Fatalf("refresh legacy official pricing cache: %v", err)
	}
	if provider.calls != 1 || result.Cached || result.Items[0].ContextLength != 128_000 {
		t.Fatalf("legacy cache was not refreshed: result=%#v calls=%d", result, provider.calls)
	}
}

func TestGetOpenRouterOfficialPricingFallsBackToStaleCache(t *testing.T) {
	provider := &officialPricingProviderStub{err: errors.New("upstream unavailable")}
	cache := &officialPricingCacheStub{
		found: true,
		data: []byte(`{
			"fetchedAt":"2020-01-01T00:00:00Z",
			"items":[{"id":"openai/gpt-test","canonicalSlug":"openai/gpt-test","name":"GPT Test","pricing":{}}]
		}`),
	}
	service := NewOfficialPricingService(provider, cache)

	result, err := service.GetOpenRouterOfficialPricing(t.Context(), false)
	if err != nil {
		t.Fatalf("get official pricing: %v", err)
	}
	if !result.Cached || !result.Stale || provider.calls != 1 {
		t.Fatalf("unexpected stale fallback: result=%#v calls=%d", result, provider.calls)
	}
}

func TestGetOpenRouterOfficialPricingRefreshesAndPersistsSnapshot(t *testing.T) {
	provider := &officialPricingProviderStub{payload: []byte(`{
		"data":[{"id":"openai/gpt-test","name":"GPT Test","context_length":128000,"top_provider":{"max_completion_tokens":8192},"pricing":{"prompt":"0.1"}}]
	}`)}
	cache := &officialPricingCacheStub{}
	service := NewOfficialPricingService(provider, cache)

	result, err := service.GetOpenRouterOfficialPricing(t.Context(), true)
	if err != nil {
		t.Fatalf("refresh official pricing: %v", err)
	}
	if result.Cached || result.Stale || result.FetchedAt.IsZero() || len(result.Items) != 1 {
		t.Fatalf("unexpected refresh result: %#v", result)
	}
	if provider.calls != 1 || len(cache.stored) == 0 {
		t.Fatalf("refresh did not fetch and persist: calls=%d stored=%d", provider.calls, len(cache.stored))
	}
	var persisted openRouterPricingCacheFile
	if err := json.Unmarshal(cache.stored, &persisted); err != nil {
		t.Fatalf("decode persisted cache: %v", err)
	}
	if len(persisted.Items) != 1 || persisted.Items[0].ID != "openai/gpt-test" {
		t.Fatalf("unexpected persisted cache: %#v", persisted)
	}
	if persisted.Version != openRouterPricingCacheVersion {
		t.Fatalf("cache version = %d, want %d", persisted.Version, openRouterPricingCacheVersion)
	}
	if persisted.Items[0].ContextLength != 128_000 || persisted.Items[0].MaxCompletionTokens != 8_192 {
		t.Fatalf("model limits were not persisted: %#v", persisted.Items[0])
	}
}
