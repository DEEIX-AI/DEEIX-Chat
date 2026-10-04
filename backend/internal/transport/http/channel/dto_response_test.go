package channel

import (
	"encoding/json"
	"testing"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
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

func TestModelCatalogResolveResponse(t *testing.T) {
	empty := toModelCatalogResolveResponse(appchannel.ModelCatalogResolution{})
	data, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	// 未匹配时数组为空数组、可空字段为 null，与契约一致。
	want := `{"matched":false,"provider":null,"modelId":null,"inputModalities":[],"outputModalities":[],"contextWindow":null,"reasoning":null,"reasoningTemplate":null}`
	if string(data) != want {
		t.Fatalf("unexpected empty response:\n got %s\nwant %s", data, want)
	}

	budgets := map[string]int{"low": 2048}
	resolved := toModelCatalogResolveResponse(appchannel.ModelCatalogResolution{
		Matched: true, Provider: "anthropic", ModelID: "claude-sonnet-4-5", ContextWindow: 200_000,
		AutoReasoning:       &domainchannel.ReasoningCapability{Format: "anthropic_budget", Levels: []string{"low"}, Default: "low"},
		AutoReasoningSource: domainchannel.ReasoningSourceCatalog,
		ReasoningTemplate:   &domainchannel.ReasoningTemplate{Format: "anthropic_budget", Levels: []string{"low"}, Budgets: budgets},
	})
	budgets["low"] = 1
	if resolved.ContextWindow == nil || *resolved.ContextWindow != 200_000 || resolved.Reasoning == nil || resolved.Reasoning.Source != "catalog" ||
		resolved.ReasoningTemplate == nil || resolved.ReasoningTemplate.Budgets["low"] != 2048 {
		t.Fatalf("unexpected resolved response: %#v", resolved)
	}
}
