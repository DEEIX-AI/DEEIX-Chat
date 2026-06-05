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
	if googleSearch["time_range_filter"] != "week" {
		t.Fatalf("expected canonical google_search payload, got %#v", payload)
	}
	if _, ok := payload["type"]; ok {
		t.Fatalf("expected Gemini field-style tool payload without type, got %#v", payload)
	}
	if _, ok := payload["googleSearch"]; ok {
		t.Fatalf("expected googleSearch alias to be normalized away, got %#v", payload)
	}

	_, payload, ok = PayloadFromOption("gemini_generate_content", map[string]interface{}{
		"type":          "google_search",
		"google_search": nil,
	})
	if !ok {
		t.Fatal("expected google_search native tool payload from type")
	}
	if _, ok := payload["google_search"].(map[string]interface{}); !ok {
		t.Fatalf("expected nil google_search to normalize to object, got %#v", payload)
	}
	if _, ok := payload["type"]; ok {
		t.Fatalf("expected Gemini field-style tool payload without type, got %#v", payload)
	}

	for _, item := range []string{"google_search", "code_execution", "url_context"} {
		definition, ok := Find("gemini_generate_content", item)
		if !ok {
			t.Fatalf("expected %s definition", item)
		}
		if _, ok := definition.Payload[item].(map[string]interface{}); !ok {
			t.Fatalf("expected %s definition payload to preserve empty object, got %#v", item, definition.Payload)
		}
	}

	for _, item := range []string{"code_execution", "url_context"} {
		_, payload, ok = PayloadFromOption("gemini_generate_content", map[string]interface{}{
			item: map[string]interface{}{},
		})
		if !ok {
			t.Fatalf("expected %s native tool payload", item)
		}
		if _, ok := payload[item].(map[string]interface{}); !ok {
			t.Fatalf("expected canonical %s payload, got %#v", item, payload)
		}
		if _, ok := payload["type"]; ok {
			t.Fatalf("expected Gemini field-style tool payload without type, got %#v", payload)
		}
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
	if !ok || key != "xai.collections_search" {
		t.Fatalf("expected xAI collections search price key, got key=%q ok=%v", key, ok)
	}
	price, ok := UsagePriceByKey(key)
	if !ok || price.NanousdPerCall != priceUSD00025Nanousd {
		t.Fatalf("expected xAI collections search price, got %#v ok=%v", price, ok)
	}

	key, ok = UsagePricingKey("gemini_generate_content", "google_search")
	if !ok || key != "google.google_search" {
		t.Fatalf("expected Google search price key, got key=%q ok=%v", key, ok)
	}
	key, ok = UsagePricingKey("gemini_generate_content", "code_execution")
	if !ok || key != "google.code_execution" {
		t.Fatalf("expected Google code execution price key, got key=%q ok=%v", key, ok)
	}
	key, ok = UsagePricingKey("gemini_generate_content", "url_context")
	if !ok || key != "google.url_context" {
		t.Fatalf("expected Google URL context price key, got key=%q ok=%v", key, ok)
	}
}

func TestPricingOverridesApplyToDisplayAndUsagePricing(t *testing.T) {
	raw := `{"xai.web_search":{"priceNanousd":123000000,"unit":"call","priceLabel":"","billable":true}}`
	items := PricingDefinitionsWithOverrides(raw)
	var found PricingDefinition
	for _, item := range items {
		if item.ToolKey == "xai.web_search" {
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
	price, ok := UsagePriceByKeyWithOverrides("xai.web_search", overrides)
	if !ok || price.NanousdPerCall != 123000000 {
		t.Fatalf("expected usage price override, got %#v ok=%v", price, ok)
	}
}

func TestPricingDefinitionsMatchOfficialNativeToolKeys(t *testing.T) {
	officialKeys := make(map[string]struct{})
	for _, definition := range Definitions() {
		officialKeys[definition.Key] = struct{}{}
	}
	pricingKeys := make(map[string]struct{})
	for _, item := range PricingDefinitions() {
		if item.ToolKey == "" {
			t.Fatalf("pricing item has empty tool key: %#v", item)
		}
		if item.Label == "" {
			t.Fatalf("pricing item has empty label: %#v", item)
		}
		if item.Description == "" {
			t.Fatalf("pricing item has empty description: %#v", item)
		}
		if item.Type == "" {
			t.Fatalf("pricing item has empty type: %#v", item)
		}
		pricingKeys[item.ToolKey] = struct{}{}
	}
	if len(pricingKeys) != len(officialKeys) {
		t.Fatalf("expected pricing count to match official native tools: pricing=%d official=%d", len(pricingKeys), len(officialKeys))
	}
	for key := range officialKeys {
		if _, ok := pricingKeys[key]; !ok {
			t.Fatalf("missing pricing item for official native tool %q", key)
		}
	}
}

func TestModelCapabilityNativeToolsExtendPricingAndUsageCatalog(t *testing.T) {
	dynamic := DefinitionsFromCapabilitiesJSON(`{
		"nativeTools": [
			{
				"key": "custom.search_20260601",
				"provider": "Custom",
				"protocols": ["openai_responses", "xai_responses"],
				"type": "search_20260601",
				"label": "Custom Search",
				"description": "Custom hosted search.",
				"payload": {"type": "search_20260601", "enable_image_understanding": true},
				"defaultEnabled": true,
				"priceNanousd": 7000000,
				"usageAliases": ["custom_search"]
			}
		]
	}`)
	definitions := MergeDefinitions(dynamic)
	var protocols int
	for _, definition := range definitions {
		if definition.Key == "custom.search_20260601" {
			protocols++
		}
	}
	if protocols != 2 {
		t.Fatalf("expected custom native tool to expand to two protocol definitions, got %d", protocols)
	}

	pricing := PricingDefinitionsFromDefinitions(definitions)
	var found PricingDefinition
	for _, item := range pricing {
		if item.ToolKey == "custom.search_20260601" {
			found = item
			break
		}
	}
	if found.ToolKey == "" || found.Label != "Custom Search" || found.Type != "search_20260601" || found.PriceNanousd != 7000000 {
		t.Fatalf("expected dynamic native tool pricing row, got %#v", found)
	}

	overrides, err := ParsePricingOverridesJSONForDefinitions(`{"custom.search_20260601":{"priceNanousd":9000000,"unit":"call","priceLabel":"","billable":true}}`, definitions)
	if err != nil {
		t.Fatalf("parse dynamic pricing override: %v", err)
	}
	price, ok := UsagePriceForToolWithOverrides("xai_responses", "custom_search", definitions, overrides)
	if !ok || price.NanousdPerCall != 9000000 || price.Provider != "custom" {
		t.Fatalf("expected dynamic usage price override, got %#v ok=%v", price, ok)
	}
}

func TestZeroDefaultPricingCanBeCustomizedPerCall(t *testing.T) {
	raw := `{"openai.shell":{"priceNanousd":1000000,"unit":"search","priceLabel":"notMetered","billable":false}}`
	items := PricingDefinitionsWithOverrides(raw)
	var found PricingDefinition
	for _, item := range items {
		if item.ToolKey == "openai.shell" {
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
	price, ok := UsagePriceByKeyWithOverrides("openai.shell", overrides)
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
	if PricingOverridesUseDefaults(`{"google.google_search":{"priceNanousd":1000000,"unit":"call","priceLabel":"","billable":true}}`) {
		t.Fatal("expected customized Google search price to differ from defaults")
	}
}
