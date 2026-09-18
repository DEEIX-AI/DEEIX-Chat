package channel

import (
	"encoding/json"
	"testing"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
)

func TestPublicPricingResponsePreservesCacheWriteMultipliers(t *testing.T) {
	response := toPublicModelPricingResponse(&appbilling.PublicModelPricing{
		Mode: "tiered", CacheWrite5mMultiplier: 1, CacheWrite1hMultiplier: 1.6,
		Tiers: []appbilling.PublicModelPricingTier{{CacheWriteUSDPerMTokens: 3.75}},
	})
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["cacheWrite5mMultiplier"] != float64(1) || decoded["cacheWrite1hMultiplier"] != 1.6 {
		t.Fatalf("cache multipliers missing from public API: %s", data)
	}
	if response.Tiers[0].CacheWriteUSDPerMTokens != 3.75 {
		t.Fatalf("configured tier price changed: %#v", response.Tiers[0])
	}
}
