package channel

import "testing"

func TestResolveModelCapsUsesCatalogAndConservativeFallback(t *testing.T) {
	tests := []struct {
		name   string
		model  string
		window int
		source ModelCapsSource
	}{
		{name: "vendor prefixed model", model: "openai/gpt-4.1-mini", window: 1_047_576, source: ModelCapsSourceCatalog},
		{name: "domestic family uses fallback", model: "deepseek/deepseek-chat", window: 128_000, source: ModelCapsSourceFallback},
		{name: "qwen generation uses fallback", model: "qwen/qwen2.5-72b-instruct", window: 128_000, source: ModelCapsSourceFallback},
		{name: "unknown model", model: "enterprise-private-v2", window: 128_000, source: ModelCapsSourceFallback},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved := ResolveModelCapsWithFallback(test.model, defaultContextWindow)
			if resolved.ContextWindow != test.window || resolved.Source != test.source {
				t.Fatalf("unexpected resolution: %#v", resolved)
			}
		})
	}
}

func TestResolveModelCapsToleratesSeparatorsWithoutMixingVersions(t *testing.T) {
	equivalent := []string{
		"openai/gpt-4.1-mini",
		"openai/gpt_4_1_mini",
		"openai/gpt 4 1 mini",
	}
	for _, modelName := range equivalent {
		resolved := ResolveModelCapsWithFallback(modelName, defaultContextWindow)
		if resolved.Source != ModelCapsSourceCatalog || resolved.ContextWindow != 1_047_576 {
			t.Fatalf("expected separator-tolerant gpt-4.1 match for %q, got %#v", modelName, resolved)
		}
	}

	for _, modelName := range []string{"anthropic/claude-sonnet-4.5", "anthropic/claude_sonnet_4_6", "openai/gpt-sol-5.6", "openai/gpt-luna-5.6"} {
		resolved := ResolveModelCapsWithFallback(modelName, defaultContextWindow)
		if resolved.Source != ModelCapsSourceFallback {
			t.Fatalf("expected version-specific model %q to use fallback without an exact catalog rule, got %#v", modelName, resolved)
		}
	}
	if resolved := ResolveModelCapsWithFallback("openai/gpt-41-mini", defaultContextWindow); resolved.Source == ModelCapsSourceCatalog {
		t.Fatalf("numeric version separators must remain significant: %#v", resolved)
	}
}

func TestResolveModelCapsDoesNotMatchShortReasoningNameInsideWord(t *testing.T) {
	resolved := ResolveModelCapsWithFallback("vendor/foo1-preview", defaultContextWindow)
	if resolved.Source != ModelCapsSourceFallback {
		t.Fatalf("expected fallback, got %#v", resolved)
	}
	resolved = ResolveModelCapsWithFallback("openai/gpt-4.10-preview", defaultContextWindow)
	if resolved.Source == ModelCapsSourceCatalog {
		t.Fatalf("gpt-4.10 must not be classified as gpt-4.1: %#v", resolved)
	}
}

func TestResolveModelCapsWithFallbackOnlyChangesUnknownModels(t *testing.T) {
	unknown := ResolveModelCapsWithFallback("enterprise-private-v2", 256_000)
	if unknown.ContextWindow != 256_000 || unknown.Source != ModelCapsSourceFallback {
		t.Fatalf("expected configured fallback for unknown model, got %#v", unknown)
	}

	known := ResolveModelCapsWithFallback("openai/gpt-4.1-mini", 256_000)
	if known.ContextWindow != 1_047_576 || known.Source != ModelCapsSourceCatalog {
		t.Fatalf("configured fallback must not override a catalog match, got %#v", known)
	}

	overridden := ResolveModelCapsFromCapabilitiesWithFallback(
		"enterprise-private-v2",
		`{"contextWindow":64000}`,
		256_000,
	)
	if overridden.ContextWindow != 64_000 || overridden.Source != ModelCapsSourceCapabilities {
		t.Fatalf("configured fallback must not override explicit capabilities, got %#v", overridden)
	}
}

func TestCompactionThresholdUsesEffectiveBudgetPercentage(t *testing.T) {
	const fallbackWindow = 256_000
	wantBudget := fallbackWindow - defaultMaxOutputTokens - autocompactBufferTokens
	wantThreshold := int64(wantBudget * 80 / 100)

	got := CompactionThresholdFromCapabilitiesWithFallback(
		"enterprise-private-v2",
		"",
		fallbackWindow,
		80,
	)
	if got != wantThreshold {
		t.Fatalf("threshold = %d, want %d", got, wantThreshold)
	}
	if disabled := CompactionThresholdFromCapabilitiesWithFallback(
		"enterprise-private-v2",
		"",
		fallbackWindow,
		0,
	); disabled != 0 {
		t.Fatalf("disabled threshold = %d, want 0", disabled)
	}
}

func TestModelCapsFromCapabilitiesOverridesNameInference(t *testing.T) {
	resolved := ResolveModelCapsFromCapabilitiesWithFallback("custom-enterprise-model", `{
		"contextWindow": 64000,
		"maxOutputTokens": 12000
	}`, defaultContextWindow)

	if resolved.ContextWindow != 64_000 {
		t.Fatalf("expected context window from capabilities, got %d", resolved.ContextWindow)
	}
	if resolved.MaxOutputTokens != 12_000 {
		t.Fatalf("expected max output from capabilities, got %d", resolved.MaxOutputTokens)
	}
	if resolved.Source != ModelCapsSourceCapabilities {
		t.Fatalf("expected override source, got %q", resolved.Source)
	}
}

func TestInvalidCapabilitiesKeepCatalogResolution(t *testing.T) {
	resolved := ResolveModelCapsFromCapabilitiesWithFallback("google/gemini-2.5-pro", `{"contextWindow":0}`, defaultContextWindow)
	if resolved.Source != ModelCapsSourceCatalog || resolved.ContextWindow != 1_000_000 {
		t.Fatalf("unexpected resolution: %#v", resolved)
	}
}

func TestValidateModelCapsOverrides(t *testing.T) {
	valid := []string{"{}", `{"contextWindow":128000}`, `{"context_window_tokens":"256000","max_output_tokens":8192}`}
	for _, raw := range valid {
		if err := ValidateModelCapsOverrides(raw); err != nil {
			t.Fatalf("expected valid override %s: %v", raw, err)
		}
	}
	invalid := []string{`{"contextWindow":0}`, `{"contextWindow":4096,"maxOutputTokens":4096}`, `{"contextWindow":20000000}`}
	for _, raw := range invalid {
		if err := ValidateModelCapsOverrides(raw); err == nil {
			t.Fatalf("expected invalid override %s", raw)
		}
	}
}

func TestEffectiveContextBudgetFromCapabilitiesUsesConfiguredWindow(t *testing.T) {
	got := EffectiveContextBudgetFromCapabilitiesWithFallback("custom-enterprise-model", `{
		"context_window_tokens": "64000",
		"max_output_tokens": "12000"
	}`, defaultContextWindow)
	want := 64_000 - 12_000 - autocompactBufferTokens
	if got != want {
		t.Fatalf("expected budget %d, got %d", want, got)
	}
}
