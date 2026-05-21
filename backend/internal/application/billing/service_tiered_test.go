package billing

import "testing"

func TestResolveTieredPricingTierSelectsTierByRawInputTokens(t *testing.T) {
	tiers, err := parseTieredPricingTiers(`{
		"tiers": [
			{"upToTokens": 1000, "inputUSDPerMTokens": 1, "cacheReadUSDPerMTokens": 0.2, "cacheWriteUSDPerMTokens": 1.25, "outputUSDPerMTokens": 2},
			{"upToTokens": 0, "inputUSDPerMTokens": 0.8, "cacheReadUSDPerMTokens": 0.1, "cacheWriteUSDPerMTokens": 1.1, "outputUSDPerMTokens": 1.6}
		]
	}`)
	if err != nil {
		t.Fatalf("parse tiers: %v", err)
	}

	rawInputTokens := tieredPricingInputTokens(700, 800, 0)
	resolved := resolveTieredPricingTier(rawInputTokens, tiers)

	if resolved.fromTokens != 1000 {
		t.Fatalf("from tokens = %d, want 1000", resolved.fromTokens)
	}
	if resolved.upToTokens != nil {
		t.Fatalf("up to tokens = %v, want nil", *resolved.upToTokens)
	}
	if resolved.tier.inputNanousdPerMTokens != 800000000 {
		t.Fatalf("input rate = %d, want 800000000", resolved.tier.inputNanousdPerMTokens)
	}
	if tierCacheReadRate(resolved.tier) != 100000000 {
		t.Fatalf("cache read rate = %d, want 100000000", tierCacheReadRate(resolved.tier))
	}
	if tierCacheWriteRate(resolved.tier) != 1100000000 {
		t.Fatalf("cache write rate = %d, want 1100000000", tierCacheWriteRate(resolved.tier))
	}
	if resolved.tier.outputNanousdPerMTokens != 1600000000 {
		t.Fatalf("output rate = %d, want 1600000000", resolved.tier.outputNanousdPerMTokens)
	}
}

func TestResolveTieredPricingTierUsesRawInputRangeForAllCategories(t *testing.T) {
	tiers, err := parseTieredPricingTiers(`{
		"tiers": [
			{"upToTokens": 1000, "inputUSDPerMTokens": 1, "cacheReadUSDPerMTokens": 0.2, "cacheWriteUSDPerMTokens": 1.25, "outputUSDPerMTokens": 2},
			{"upToTokens": 0, "inputUSDPerMTokens": 0.8, "cacheReadUSDPerMTokens": 0.1, "cacheWriteUSDPerMTokens": 1.1, "outputUSDPerMTokens": 1.6}
		]
	}`)
	if err != nil {
		t.Fatalf("parse tiers: %v", err)
	}

	rawInputTokens := tieredPricingInputTokens(400, 500, 0)
	resolved := resolveTieredPricingTier(rawInputTokens, tiers)
	tier := resolved.tier

	inputBilled := calcNanousdByToken(900, tier.inputNanousdPerMTokens)
	cacheReadBilled := calcNanousdByToken(2000, tierCacheReadRate(tier))
	cacheWriteBilled := calcNanousdByToken(3000, tierCacheWriteRate(tier))
	outputBilled := calcNanousdByToken(8000, tier.outputNanousdPerMTokens)

	if inputBilled != 900000 {
		t.Fatalf("input billed = %d, want 900000", inputBilled)
	}
	if cacheReadBilled != 400000 {
		t.Fatalf("cache read billed = %d, want 400000", cacheReadBilled)
	}
	if cacheWriteBilled != 3750000 {
		t.Fatalf("cache write billed = %d, want 3750000", cacheWriteBilled)
	}
	if outputBilled != 16000000 {
		t.Fatalf("output billed = %d, want 16000000", outputBilled)
	}
}

func TestResolveCacheWriteNanousdPerMTokensAnthropicTTL(t *testing.T) {
	configuredRate := int64(1_100_000_000)

	if got := resolveCacheWriteNanousdPerMTokens(configuredRate, "anthropic_messages", "5m"); got != 1_375_000_000 {
		t.Fatalf("5m cache write rate = %d, want 1375000000", got)
	}
	if got := resolveCacheWriteNanousdPerMTokens(configuredRate, "anthropic_messages", "1h"); got != 2_200_000_000 {
		t.Fatalf("1h cache write rate = %d, want 2200000000", got)
	}
}

func TestResolveCacheWriteNanousdPerMTokensKeepsConfiguredRateForOtherProtocols(t *testing.T) {
	configuredRate := int64(900_000_000)

	got := resolveCacheWriteNanousdPerMTokens(configuredRate, "openai_responses", "1h")

	if got != configuredRate {
		t.Fatalf("cache write rate = %d, want configured %d", got, configuredRate)
	}
}
