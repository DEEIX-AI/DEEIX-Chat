package billing

import (
	"context"
	"encoding/json"
	"errors"
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
}

func TestGetOpenRouterOfficialPricingUsesFreshPersistentCache(t *testing.T) {
	provider := &officialPricingProviderStub{err: errors.New("must not fetch")}
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
		t.Fatalf("get official pricing: %v", err)
	}
	if !result.Cached || result.Stale || len(result.Items) != 1 {
		t.Fatalf("unexpected cache result: %#v", result)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
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
		"data":[{"id":"openai/gpt-test","name":"GPT Test","pricing":{"prompt":"0.1"}}]
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
}
