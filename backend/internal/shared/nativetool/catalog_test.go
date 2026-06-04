package nativetool

import "testing"

func TestPayloadFromOptionPreservesToolParametersAndFixesIdentity(t *testing.T) {
	_, payload, ok := PayloadFromOption("openai_responses", map[string]interface{}{
		"type": "shell",
		"environment": map[string]interface{}{
			"type":        "host",
			"max_runtime": "10m",
		},
	})
	if !ok {
		t.Fatal("expected shell native tool payload")
	}
	environment := payload["environment"].(map[string]interface{})
	if environment["type"] != "container_auto" {
		t.Fatalf("expected canonical shell environment, got %#v", payload)
	}
	if environment["max_runtime"] != "10m" {
		t.Fatalf("expected shell parameters to pass, got %#v", payload)
	}

	_, payload, ok = PayloadFromOption("google_image_generation", map[string]interface{}{
		"googleSearch":  map[string]interface{}{"dynamic_retrieval_config": map[string]interface{}{"mode": "MODE_DYNAMIC"}},
		"google_search": map[string]interface{}{"time_range_filter": "week"},
	})
	if !ok {
		t.Fatal("expected google_search native tool payload")
	}
	googleSearch := payload["google_search"].(map[string]interface{})
	if googleSearch["time_range_filter"] != "week" || payload["type"] != "google_search" {
		t.Fatalf("expected canonical google_search payload, got %#v", payload)
	}
	if _, ok := payload["googleSearch"]; ok {
		t.Fatalf("expected googleSearch alias to be normalized away, got %#v", payload)
	}
}

func TestPayloadFromOptionRemovesSystemControlledToolFields(t *testing.T) {
	_, payload, ok := PayloadFromOption("anthropic_messages", map[string]interface{}{
		"type":     "advisor_20260301",
		"name":     "override",
		"model":    "attacker-model",
		"headers":  map[string]interface{}{"Authorization": "Bearer token"},
		"max_uses": 2,
	})
	if !ok {
		t.Fatal("expected advisor native tool payload")
	}
	if payload["name"] != "advisor" {
		t.Fatalf("expected advisor identity to be fixed, got %#v", payload)
	}
	if payload["max_uses"] != 2 {
		t.Fatalf("expected safe advisor parameters to pass, got %#v", payload)
	}
	if _, exists := payload["model"]; exists {
		t.Fatalf("expected advisor model override to be removed, got %#v", payload)
	}
	if _, exists := payload["headers"]; exists {
		t.Fatalf("expected advisor headers override to be removed, got %#v", payload)
	}
}

func TestUsagePricingKeyMapsObservedToolUsage(t *testing.T) {
	key, ok := UsagePricingKey("xai_responses", "collections_search")
	if !ok || key != "xaiCollectionsSearch" {
		t.Fatalf("expected xAI collections search price key, got key=%q ok=%v", key, ok)
	}
	price, ok := UsagePriceByKey(key)
	if !ok || price.NanousdPerCall != priceUSD00025Nanousd {
		t.Fatalf("expected xAI collections search price, got %#v ok=%v", price, ok)
	}

	key, ok = UsagePricingKey("gemini_generate_content", "google_search")
	if !ok || key != "googleGoogleSearch" {
		t.Fatalf("expected Google search price key, got key=%q ok=%v", key, ok)
	}
}

func TestPricingOverridesApplyToDisplayAndUsagePricing(t *testing.T) {
	raw := `{"xaiWebSearch":{"priceNanousd":123000000,"unit":"call","priceLabel":"","billable":true}}`
	items := PricingDefinitionsWithOverrides(raw)
	var found PricingDefinition
	for _, item := range items {
		if item.ToolKey == "xaiWebSearch" {
			found = item
			break
		}
	}
	if found.PriceNanousd != 123000000 || !found.Billable {
		t.Fatalf("expected display pricing override, got %#v", found)
	}
	overrides, err := ParsePricingOverridesJSON(raw)
	if err != nil {
		t.Fatalf("parse pricing overrides: %v", err)
	}
	price, ok := UsagePriceByKeyWithOverrides("xaiWebSearch", overrides)
	if !ok || price.NanousdPerCall != 123000000 {
		t.Fatalf("expected usage price override, got %#v ok=%v", price, ok)
	}
}

func TestZeroDefaultPricingCanBeCustomizedPerCall(t *testing.T) {
	raw := `{"openaiShell":{"priceNanousd":1000000,"unit":"search","priceLabel":"notMetered","billable":false}}`
	items := PricingDefinitionsWithOverrides(raw)
	var found PricingDefinition
	for _, item := range items {
		if item.ToolKey == "openaiShell" {
			found = item
			break
		}
	}
	if found.PriceNanousd != 1000000 || found.Unit != "call" || found.PriceLabel != "" || !found.Billable {
		t.Fatalf("expected zero-default tool to normalize to custom per-call price, got %#v", found)
	}
	overrides, err := ParsePricingOverridesJSON(raw)
	if err != nil {
		t.Fatalf("parse pricing overrides: %v", err)
	}
	price, ok := UsagePriceByKeyWithOverrides("openaiShell", overrides)
	if !ok || price.NanousdPerCall != 1000000 {
		t.Fatalf("expected OpenAI shell usage price override, got %#v ok=%v", price, ok)
	}
}

func TestPricingOverridesRejectUnknownKeys(t *testing.T) {
	if _, err := ParsePricingOverridesJSON(`{"unknownTool":{"priceNanousd":1,"unit":"call","priceLabel":"","billable":true}}`); err == nil {
		t.Fatal("expected unknown pricing key to fail")
	}
}

func TestPricingOverridesUseDefaults(t *testing.T) {
	if !PricingOverridesUseDefaults(DefaultPricingJSON()) {
		t.Fatal("expected default pricing JSON to be treated as provider defaults")
	}
	if PricingOverridesUseDefaults(`{"googleGoogleSearch":{"priceNanousd":1000000,"unit":"call","priceLabel":"","billable":true}}`) {
		t.Fatal("expected customized Google search price to differ from defaults")
	}
}
