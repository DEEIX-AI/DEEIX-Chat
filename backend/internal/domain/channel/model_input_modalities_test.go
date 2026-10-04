package channel

import (
	"errors"
	"reflect"
	"testing"
)

func TestResolveInputModalitiesPrefersExplicitThenCatalog(t *testing.T) {
	catalog := []string{"image", "text", "pdf"}

	explicit := ResolveInputModalities(`{"inputModalities":["pdf"]}`, catalog)
	if explicit.Source != InputModalitiesSourceExplicit || !reflect.DeepEqual(explicit.Values, []string{"text", "pdf"}) {
		t.Fatalf("explicit declaration must win and always include text, got %#v", explicit)
	}
	if explicit.Supports(InputModalityImage) {
		t.Fatalf("explicit declaration must be able to remove a catalog modality, got %#v", explicit)
	}

	fromCatalog := ResolveInputModalities(`{"contextWindow":128000}`, catalog)
	if fromCatalog.Source != InputModalitiesSourceCatalog || !reflect.DeepEqual(fromCatalog.Values, []string{"text", "image", "pdf"}) {
		t.Fatalf("catalog must apply without an explicit declaration, got %#v", fromCatalog)
	}

	unknown := ResolveInputModalities("", nil)
	if unknown.Known() || unknown.Supports(InputModalityText) {
		t.Fatalf("no declaration and no catalog entry must stay unknown, got %#v", unknown)
	}

	invalid := ResolveInputModalities(`{"inputModalities":"pdf"}`, catalog)
	if invalid.Source != InputModalitiesSourceCatalog {
		t.Fatalf("an invalid declaration falls back to the catalog, got %#v", invalid)
	}
}

func TestValidateModelCapsOverridesRejectsInvalidInputModalities(t *testing.T) {
	for _, raw := range []string{
		`{"inputModalities":"pdf"}`,
		`{"inputModalities":["pdf","hologram"]}`,
		`{"inputModalities":[1]}`,
	} {
		if err := ValidateModelCapsOverrides(raw); !errors.Is(err, ErrInvalidModelCapsOverride) {
			t.Fatalf("%s: expected invalid override, got %v", raw, err)
		}
	}
	if err := ValidateModelCapsOverrides(`{"inputModalities":["text","image","pdf","audio","video"]}`); err != nil {
		t.Fatalf("valid declaration rejected: %v", err)
	}
}

func TestSplitCatalogEntriesKeepsReasoningIndexUnchanged(t *testing.T) {
	entries := []ModelCatalogEntry{
		{Provider: "openai", ModelID: "gpt-5", Reasoning: true, Options: []ReasoningCatalogOption{{Type: ReasoningCatalogOptionToggle}}, InputModalities: []string{"text", "image"}},
		{Provider: "openai", ModelID: "gpt-4o", InputModalities: []string{"text", "image", "pdf"}},
		{Provider: "deepseek", ModelID: "deepseek-chat", InputModalities: []string{"text"}},
	}

	reasoning, modalities := SplitCatalogEntries(entries)

	if len(reasoning) != 1 || reasoning[0].ModelID != "gpt-5" {
		t.Fatalf("reasoning index must only contain models with reasoning options, got %#v", reasoning)
	}
	if len(modalities) != 3 {
		t.Fatalf("modality index must include text-only models, got %#v", modalities)
	}
}
