package channel

import "testing"

func TestReasoningCatalogMatchOrder(t *testing.T) {
	effort := []ReasoningCatalogOption{{Type: ReasoningCatalogOptionEffort, Values: []string{"low", "high"}}}
	catalog := NewReasoningCatalog([]ReasoningCatalogEntry{
		{Provider: "openrouter", ModelID: "openai/gpt-5", CanonicalID: "openai/gpt-5", Reasoning: true, Options: effort},
		{Provider: "azure", ModelID: "gpt-5", CanonicalID: "openai/gpt-5", Reasoning: true, Options: effort},
		{Provider: "openai", ModelID: "gpt-5", CanonicalID: "openai/gpt-5", Reasoning: true, Options: effort},
		{Provider: "zenmux", ModelID: "vendor-alias", CanonicalID: "moonshotai/kimi-k2-thinking", Reasoning: true, Options: effort},
		{Provider: "openrouter", ModelID: "acme/secret-model", Reasoning: true, Options: effort},
	})
	tests := []struct {
		name      string
		model     string
		vendor    string
		protocols []string
		provider  string
		modelID   string
	}{
		{name: "vendor provider first", model: "GPT-5", vendor: "openai", provider: "openai", modelID: "gpt-5"},
		{name: "first party without vendor", model: "gpt-5", vendor: "unknown", provider: "openai", modelID: "gpt-5"},
		{name: "prefixed name uses bare id", model: "openai/gpt-5", provider: "openai", modelID: "gpt-5"},
		{name: "canonical name", model: "kimi-k2-thinking", vendor: "moonshot", provider: "zenmux", modelID: "vendor-alias"},
		{name: "openrouter route only", model: "acme/secret-model", protocols: []string{"openrouter_chat_completions"}, provider: "openrouter", modelID: "acme/secret-model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := catalog.Match(tt.model, tt.vendor, tt.protocols)
			if entry == nil || entry.Provider != tt.provider || entry.ModelID != tt.modelID {
				t.Fatalf("Match() = %#v, want %s/%s", entry, tt.provider, tt.modelID)
			}
		})
	}
	if entry := catalog.Match("acme/secret-model", "", []string{"openai_chat_completions"}); entry != nil {
		t.Fatalf("expected no openrouter fallback without an openrouter route, got %#v", entry)
	}
	if entry := catalog.Match("unknown-model", "openai", nil); entry != nil {
		t.Fatalf("expected no fuzzy match, got %#v", entry)
	}
}

func TestCatalogReasoningCapabilityPerProtocol(t *testing.T) {
	budgetMin := 1024
	entry := &ReasoningCatalogEntry{Provider: "anthropic", ModelID: "claude", Reasoning: true, Options: []ReasoningCatalogOption{
		{Type: ReasoningCatalogOptionEffort, Values: []string{"low", "medium", "high", "max"}},
		{Type: ReasoningCatalogOptionBudget, Min: &budgetMin},
	}}
	capability, reason := CatalogReasoningCapability(entry, ReasoningProtocolAnthropic, "anthropic")
	if capability == nil || capability.Format != ReasoningFormatAnthropicEffort || capability.Default != ReasoningLevelMedium {
		t.Fatalf("expected anthropic effort capability, got %#v (%s)", capability, reason)
	}
	capability, _ = CatalogReasoningCapability(entry, ReasoningProtocolOpenAIChat, "anthropic")
	if capability == nil || capability.Format != ReasoningFormatOpenAI {
		t.Fatalf("expected openai effort capability on chat completions, got %#v", capability)
	}
	if capability, reason := CatalogReasoningCapability(entry, "openai_image_generations", "anthropic"); capability != nil || reason != ReasoningCatalogReasonUnsupportedProtocol {
		t.Fatalf("expected unsupported protocol, got %#v (%s)", capability, reason)
	}
	if capability, reason := CatalogReasoningCapability(nil, ReasoningProtocolOpenAIChat, ""); capability != nil || reason != ReasoningCatalogReasonNotReasoning {
		t.Fatalf("expected not reasoning for missing entry, got %#v (%s)", capability, reason)
	}
}

func TestResolveReasoningCapabilityCatalogPrecedence(t *testing.T) {
	catalog := &ReasoningCapability{Format: ReasoningFormatOpenAI, Levels: []string{"low", "high"}, Default: "low"}
	if capability, source := ResolveReasoningCapability(ReasoningProtocolOpenAIChat, `{}`, catalog); capability == nil || source != ReasoningSourceCatalog {
		t.Fatalf("expected catalog fallback, got %#v (%s)", capability, source)
	}
	if capability, source := ResolveReasoningCapability(ReasoningProtocolAnthropic, `{}`, catalog); capability != nil || source != "" {
		t.Fatalf("expected incompatible catalog capability to be ignored, got %#v (%s)", capability, source)
	}
	inferred := `{"optionControls":[{"path":"reasoning_effort","type":"select","options":["medium"]}]}`
	if capability, source := ResolveReasoningCapability(ReasoningProtocolOpenAIChat, inferred, catalog); capability == nil || source != ReasoningSourceInferred {
		t.Fatalf("expected inferred capability to win, got %#v (%s)", capability, source)
	}
	explicit := `{"reasoning":{"format":"bogus"}}`
	if capability, source := ResolveReasoningCapability(ReasoningProtocolOpenAIChat, explicit, catalog); capability != nil || source != ReasoningSourceExplicit {
		t.Fatalf("expected invalid explicit declaration to block the catalog, got %#v (%s)", capability, source)
	}
}

func TestCatalogReasoningCapabilityQwenFormatFollowsVendor(t *testing.T) {
	options := []ReasoningCatalogOption{{Type: ReasoningCatalogOptionToggle}, {Type: ReasoningCatalogOptionBudget}}
	hostedKimi := &ReasoningCatalogEntry{Provider: "alibaba-cn", ModelID: "kimi-k2-thinking", CanonicalID: "moonshotai/kimi-k2-thinking", Reasoning: true, Options: options}
	for _, vendor := range []string{"moonshot", "unknown"} {
		capability, _ := CatalogReasoningCapability(hostedKimi, ReasoningProtocolOpenAIChat, vendor)
		if capability == nil || capability.Format != ReasoningFormatToggle {
			t.Fatalf("vendor %q: expected toggle for a non-Qwen model hosted by Alibaba, got %#v", vendor, capability)
		}
	}
	qwen := &ReasoningCatalogEntry{Provider: "alibaba", ModelID: "qwen-plus", Reasoning: true, Options: options}
	for _, vendor := range []string{"qwen", ""} {
		capability, _ := CatalogReasoningCapability(qwen, ReasoningProtocolOpenAIChat, vendor)
		if capability == nil || capability.Format != ReasoningFormatQwen {
			t.Fatalf("vendor %q: expected qwen format, got %#v", vendor, capability)
		}
	}
}
