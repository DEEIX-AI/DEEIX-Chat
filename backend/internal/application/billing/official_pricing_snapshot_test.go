package billing

import (
	"errors"
	"testing"
	"time"
)

const builtinPricingPayload = `{
	"data":[{"id":"openai/gpt-builtin","name":"GPT Builtin","context_length":128000,"pricing":{"prompt":"0.1","completion":"0.2"}}]
}`

func encodedBuiltinPricing(t *testing.T, fetchedAt time.Time) []byte {
	t.Helper()
	data, count, err := EncodeOpenRouterPricingSnapshot(t.Context(), &officialPricingProviderStub{payload: []byte(builtinPricingPayload)}, fetchedAt)
	if err != nil || count != 1 {
		t.Fatalf("encode builtin snapshot: count=%d err=%v", count, err)
	}
	again, _, err := EncodeOpenRouterPricingSnapshot(t.Context(), &officialPricingProviderStub{payload: []byte(builtinPricingPayload)}, fetchedAt)
	if err != nil || string(again) != string(data) {
		t.Fatal("expected deterministic snapshot encoding")
	}
	return data
}

// 没有持久缓存且远端不可达时，内置快照兜底并写入缓存作为种子，结果标明来自内置快照。
func TestGetOpenRouterOfficialPricingFallsBackToBuiltinSnapshot(t *testing.T) {
	provider := &officialPricingProviderStub{err: errors.New("upstream unavailable")}
	cache := &officialPricingCacheStub{}
	service := NewOfficialPricingService(provider, cache)
	if err := service.SetBuiltinSnapshot(encodedBuiltinPricing(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatalf("set builtin snapshot: %v", err)
	}

	result, err := service.GetOpenRouterOfficialPricing(t.Context(), false)
	if err != nil {
		t.Fatalf("expected builtin fallback, got %v", err)
	}
	if result.Origin != OfficialPricingOriginBuiltin || !result.Stale || len(result.Items) != 1 || result.Items[0].ID != "openai/gpt-builtin" {
		t.Fatalf("unexpected builtin fallback: %#v", result)
	}
	if len(cache.stored) == 0 {
		t.Fatal("expected the builtin snapshot to seed the persistent cache")
	}
}

// 远端可用时照常同步，覆盖内置快照。
func TestGetOpenRouterOfficialPricingPrefersRemoteOverBuiltin(t *testing.T) {
	provider := &officialPricingProviderStub{payload: []byte(`{"data":[{"id":"openai/gpt-remote","name":"GPT Remote","pricing":{"prompt":"0.3"}}]}`)}
	service := NewOfficialPricingService(provider, &officialPricingCacheStub{})
	if err := service.SetBuiltinSnapshot(encodedBuiltinPricing(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatalf("set builtin snapshot: %v", err)
	}

	result, err := service.GetOpenRouterOfficialPricing(t.Context(), false)
	if err != nil || result.Origin != OfficialPricingOriginRemote || result.Stale || result.Items[0].ID != "openai/gpt-remote" {
		t.Fatalf("expected remote pricing to replace the stale builtin snapshot, got %#v err=%v", result, err)
	}
}

// 已同步且比内置快照新的持久缓存优先，不被旧的内置快照覆盖。
func TestGetOpenRouterOfficialPricingKeepsNewerPersistentCache(t *testing.T) {
	provider := &officialPricingProviderStub{err: errors.New("must not fetch")}
	cache := &officialPricingCacheStub{found: true, data: []byte(`{
		"version":5,"fetchedAt":"` + time.Now().UTC().Format(time.RFC3339) + `","origin":"remote",
		"items":[{"id":"openai/gpt-synced","canonicalSlug":"openai/gpt-synced","name":"GPT Synced","pricing":{}}]
	}`)}
	service := NewOfficialPricingService(provider, cache)
	if err := service.SetBuiltinSnapshot(encodedBuiltinPricing(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatalf("set builtin snapshot: %v", err)
	}

	result, err := service.GetOpenRouterOfficialPricing(t.Context(), false)
	if err != nil || result.Origin != OfficialPricingOriginRemote || result.Items[0].ID != "openai/gpt-synced" || provider.calls != 0 {
		t.Fatalf("expected the newer persistent cache, got %#v err=%v calls=%d", result, err, provider.calls)
	}
}

func TestSetBuiltinSnapshotRejectsInvalidData(t *testing.T) {
	service := NewOfficialPricingService(&officialPricingProviderStub{}, &officialPricingCacheStub{})
	for _, data := range [][]byte{nil, []byte("not gzip"), {0x1f, 0x8b}} {
		if err := service.SetBuiltinSnapshot(data); err == nil {
			t.Fatalf("expected invalid snapshot %q to be rejected", data)
		}
	}
}
