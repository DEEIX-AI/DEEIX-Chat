package billing

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
)

func TestOfficialPricingOverrideSourcePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name      string
		overrides string
		want      []OfficialPricingOverride
	}{
		{
			name: "same threshold merges keys and later values win",
			overrides: `[
				{"min_prompt_tokens":200000,"prompt":"0.000002","input_cache_write":"0.000003"},
				{"min_prompt_tokens":200000,"completion":"0.000008","input_cache_write":"0"}
			]`,
			want: []OfficialPricingOverride{{MinPromptTokens: 200000, Prompt: "0.000002", Completion: "0.000008", InputCacheWrite: "0"}},
		},
		{
			name: "lower threshold later in source overrides higher threshold",
			overrides: `[
				{"min_prompt_tokens":256000,"prompt":"0.000005","completion":"0.000008"},
				{"min_prompt_tokens":200000,"prompt":"0.000002"}
			]`,
			want: []OfficialPricingOverride{
				{MinPromptTokens: 200000, Prompt: "0.000002", Completion: "0.000004"},
				{MinPromptTokens: 256000, Prompt: "0.000002", Completion: "0.000008"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw openRouterModelPricing
			if err := json.Unmarshal([]byte(`{"prompt":"0.000001","completion":"0.000004","overrides":`+tc.overrides+`}`), &raw); err != nil {
				t.Fatal(err)
			}
			pricing := normalizeOpenRouterOfficialPricingItem(openRouterModelItem{ID: "test/model", Pricing: raw}).Pricing
			if !reflect.DeepEqual(pricing.Overrides, tc.want) || len(pricing.UnsupportedFields) != 0 {
				t.Fatalf("pricing = %#v, want overrides %#v without ignored fields", pricing, tc.want)
			}
		})
	}
}

func TestOfficialPricingPreservesExplicitZeroCacheWrite(t *testing.T) {
	for _, zero := range []string{"0", "0.0", "0e-7"} {
		t.Run(zero, func(t *testing.T) {
			provider := &officialPricingProviderStub{payload: []byte(fmt.Sprintf(`{"data":[{
				"id":"anthropic/claude-test","pricing":{"prompt":"0.000003","completion":"0.000015",
				"input_cache_write":%q,"overrides":[{"min_prompt_tokens":200000,"prompt":"0.000006","input_cache_write":%q}]}
			}]}`, zero, zero))}
			items, err := NewOfficialPricingService(provider, nil).FetchOpenRouterOfficialPricing(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			pricing := items[0].Pricing
			if pricing.InputCacheWrite != zero || pricing.Overrides[0].InputCacheWrite != zero || len(pricing.UnsupportedFields) != 0 {
				t.Fatalf("explicit zero changed: %#v", pricing)
			}
		})
	}
}

func TestOfficialPricingLegacyCacheFallback(t *testing.T) {
	for _, version := range []int{1, 2, 3, 4} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			cache := openRouterPricingCacheFile{
				Version: version, FetchedAt: time.Now(),
				Items: []openRouterPricingCacheItem{{ID: "anthropic/claude-test", Pricing: openRouterPricingCacheUnitPricing{
					Prompt: "0.000003", Completion: "0.000015", InputCacheWrite: "0.00000375",
					Overrides: []openRouterPricingCacheOverride{{MinPromptTokens: 200000, Prompt: "0.000006"}},
				}}},
			}
			data, err := json.Marshal(cache)
			if err != nil {
				t.Fatal(err)
			}
			provider := &officialPricingProviderStub{err: errors.New("offline")}
			service := NewOfficialPricingService(provider, &officialPricingCacheStub{found: true, data: data})
			result, err := service.GetOpenRouterOfficialPricing(t.Context(), false)
			if err != nil {
				t.Fatal(err)
			}
			pricing := result.Items[0].Pricing
			if !result.Stale || provider.calls != 1 || pricing.Prompt != "0.000003" || pricing.Completion != "0.000015" || len(pricing.Overrides) != 0 {
				t.Fatalf("invalid fallback: %#v", result)
			}
			wantWrite := "0.00000375"
			wantIgnored := []string{legacyOfficialPricingField, "overrides"}
			if version >= 3 {
				wantWrite = ""
				wantIgnored = append([]string{"input_cache_write"}, wantIgnored...)
			}
			if pricing.InputCacheWrite != wantWrite || pricing.CacheWritePriceBasis != domainbilling.CacheWritePriceBasisAnthropic5m || !reflect.DeepEqual(pricing.UnsupportedFields, wantIgnored) {
				t.Fatalf("invalid migrated prices: %#v", pricing)
			}
		})
	}
}

func TestImportedCacheWritePricingAcrossBillingPaths(t *testing.T) {
	provider := &officialPricingProviderStub{payload: []byte(`{"data":[{
		"id":"anthropic/claude-test","pricing":{"prompt":"0.000003","completion":"0.000015","input_cache_write":"0.00000375",
		"overrides":[{"min_prompt_tokens":200000,"prompt":"0.000006","completion":"0.0000225","input_cache_write":"0.0000075"}]}
	}]}`)}
	items, err := NewOfficialPricingService(provider, nil).FetchOpenRouterOfficialPricing(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the persisted catalog representation before importing its prices.
	catalog := officialPricingResultFromCache(openRouterPricingCacheFile{Version: openRouterPricingCacheVersion, Items: officialPricingCacheItems(items)}, true, false).Items[0].Pricing
	pricePerMillion := func(raw string) float64 {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			t.Fatal(err)
		}
		return value * 1_000_000
	}
	tiersJSON := fmt.Sprintf(`{"tiers":[{"upToTokens":200000,"cacheWriteUSDPerMTokens":%g},{"upToTokens":0,"cacheWriteUSDPerMTokens":%g}]}`,
		pricePerMillion(catalog.InputCacheWrite), pricePerMillion(catalog.Overrides[0].InputCacheWrite))
	for _, tc := range []struct {
		mode   string
		tokens int64
		want5m int64
		want1h int64
	}{
		{"token", 1000, 3_750_000, 6_000_000},
		{"tiered", 200000, 750_000_000, 1_200_000_000},
		{"tiered", 200001, 1_500_007_500, 2_400_012_000},
	} {
		for _, protocol := range []string{"openai_chat_completions", "openai_responses", "anthropic_messages"} {
			for _, ttl := range []string{"5m", "1h"} {
				t.Run(fmt.Sprintf("%s/%d/%s/%s", tc.mode, tc.tokens, protocol, ttl), func(t *testing.T) {
					service := NewService(&billingRepositoryStub{mode: "usage", pricing: &domainbilling.ModelPricing{
						PlatformModelName: "claude-test", PricingMode: tc.mode,
						CacheWriteNanousdPerMTokens: usdToNanousd(pricePerMillion(catalog.InputCacheWrite)),
						CacheWritePriceBasis:        catalog.CacheWritePriceBasis, TieredPricingJSON: tiersJSON,
					}})
					want := tc.want5m
					if protocol == "anthropic_messages" && ttl == "1h" {
						want = tc.want1h
					}
					estimated, err := service.EstimateUsageNanousd(t.Context(), 1, UsageEstimateInput{
						PlatformModelName: "claude-test", ProviderProtocol: protocol, CacheTimeout: ttl, CacheWriteTokens: tc.tokens,
					})
					if err != nil || estimated != want {
						t.Fatalf("estimate = %d, %v; want %d", estimated, err, want)
					}
					for _, serviceOnly := range []bool{false, true} {
						input := UsagePricingInput{UserID: 1, PlatformModelName: "claude-test", ProviderProtocol: protocol, CacheTimeout: ttl, CacheWriteTokens: tc.tokens}
						if serviceOnly {
							input.ServiceOnly = true
							input.ServiceItems = []ServiceUsageInput{{ServiceCode: "compact", PlatformModelName: "claude-test", ProviderProtocol: protocol, CacheTimeout: ttl, CacheWriteTokens: tc.tokens}}
						}
						ledger, err := service.BuildUsageLedger(t.Context(), input)
						if err != nil {
							t.Fatal(err)
						}
						if ledger.BilledNanousd != want {
							t.Fatalf("serviceOnly=%t: billed=%d, want %d", serviceOnly, ledger.BilledNanousd, want)
						}
					}
				})
			}
		}
	}
}

func TestUpsertModelPricingPreservesCacheWriteBasis(t *testing.T) {
	service := NewService(&billingRepositoryStub{})
	service.SetModelPricingCatalogProvider(modelPricingCatalogStub{names: map[string]struct{}{"claude-test": {}}})
	for _, basis := range []string{"", domainbilling.CacheWritePriceBasisDirect, domainbilling.CacheWritePriceBasisAnthropic5m, "invalid"} {
		item, err := service.UpsertModelPricing(t.Context(), ModelPricingInput{
			PlatformModelName: "claude-test", PricingMode: "token",
			CacheWritePriceBasis: basis, CacheWriteNanousdPerMTokens: 3_750_000_000,
		})
		if basis == "invalid" {
			if !errors.Is(err, ErrInvalidModelPricing) {
				t.Fatalf("invalid basis: %v", err)
			}
			continue
		}
		if err != nil || item.CacheWritePriceBasis != basis || item.CacheWriteNanousdPerMTokens != 3_750_000_000 {
			t.Fatalf("upsert changed cache pricing: %#v, %v", item, err)
		}
	}
}

func TestPublicCacheWritePricingMatchesLedger(t *testing.T) {
	for _, tc := range []struct {
		basis  string
		want5m float64
		want1h float64
	}{
		{"", 1.25, 2},
		{domainbilling.CacheWritePriceBasisDirect, 1, 1},
		{domainbilling.CacheWritePriceBasisAnthropic5m, 1, 1.6},
	} {
		for _, mode := range []string{"token", "tiered"} {
			t.Run(tc.basis+"/"+mode, func(t *testing.T) {
				pricing := &domainbilling.ModelPricing{
					PlatformModelName: "claude-test", PricingMode: mode, CacheWritePriceBasis: tc.basis,
					CacheWriteNanousdPerMTokens: 3_750_000_000,
					TieredPricingJSON:           `{"tiers":[{"upToTokens":200000,"cacheWriteUSDPerMTokens":3.75},{"upToTokens":0,"cacheWriteUSDPerMTokens":7.5}]}`,
				}
				public := toPublicModelPricing(*pricing)
				if public.CacheWrite5mMultiplier != tc.want5m || public.CacheWrite1hMultiplier != tc.want1h {
					t.Fatalf("public multipliers = %g, %g; want %g, %g", public.CacheWrite5mMultiplier, public.CacheWrite1hMultiplier, tc.want5m, tc.want1h)
				}
				service := NewService(&billingRepositoryStub{mode: "usage", pricing: pricing})
				for _, tokens := range []int64{200000, 200001} {
					configured := public.CacheWriteUSDPerMTokens
					if mode == "tiered" {
						configured = public.Tiers[0].CacheWriteUSDPerMTokens
						if tokens > 200000 {
							configured = public.Tiers[1].CacheWriteUSDPerMTokens
						}
					}
					for _, protocol := range []string{"anthropic_messages", "openai_chat_completions"} {
						for _, ttl := range []string{"5m", "1h"} {
							multiplier := 1.0
							if protocol == "anthropic_messages" {
								multiplier = public.CacheWrite5mMultiplier
								if ttl == "1h" {
									multiplier = public.CacheWrite1hMultiplier
								}
							}
							ledger, err := service.BuildUsageLedger(t.Context(), UsagePricingInput{
								UserID: 1, PlatformModelName: "claude-test", ProviderProtocol: protocol, CacheTimeout: ttl, CacheWriteTokens: tokens,
							})
							if err != nil {
								t.Fatal(err)
							}
							want := calcNanousdByToken(tokens, usdToNanousd(configured*multiplier))
							if ledger.BilledNanousd != want {
								t.Fatalf("%s/%s/%d: billed %d, public pricing implies %d", protocol, ttl, tokens, ledger.BilledNanousd, want)
							}
							var snapshot map[string]any
							if err := json.Unmarshal([]byte(ledger.PricingSnapshotJSON), &snapshot); err != nil {
								t.Fatal(err)
							}
							if snapshot["cache_write_"+ttl+"_multiplier"] != multiplier {
								t.Fatalf("snapshot multiplier disagrees with public pricing: %#v", snapshot)
							}
						}
					}
				}
			})
		}
	}
}
