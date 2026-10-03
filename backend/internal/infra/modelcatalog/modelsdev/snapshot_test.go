package modelsdev

import (
	"errors"
	"reflect"
	"testing"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
)

const modelsDevFixture = `{
	"openai": {"id": "openai", "models": {
		"gpt-5": {"id": "gpt-5", "reasoning": true, "canonical_model_id": "openai/gpt-5",
			"reasoning_options": [{"type": "effort", "values": ["minimal", "low", "medium", "high", null, "default"]}]},
		"gpt-4o": {"id": "gpt-4o", "reasoning": false},
		"o-broken": {"id": "o-broken", "reasoning": true, "reasoning_options": "oops"}
	}},
	"anthropic": {"id": "anthropic", "models": {
		"claude-sonnet-4-5": {"id": "claude-sonnet-4-5", "reasoning": true,
			"reasoning_options": [{"type": "budget_tokens", "min": 1024, "max": 64000.5}, {"type": "budget_tokens", "min": 1}]},
		"claude-empty": {"id": "claude-empty", "reasoning": true, "reasoning_options": []}
	}},
	"deepseek": {"id": "deepseek", "models": {
		"deepseek-chat": {"id": "deepseek-chat", "reasoning": true, "reasoning_options": [{"type": "toggle"}, {"type": "unknown"}]}
	}},
	"broken": "not an object"
}`

func TestParseReasoningCatalogKeepsReasoningModelsWithKnownOptions(t *testing.T) {
	entries, err := ParseReasoningCatalog([]byte(modelsDevFixture))
	if err != nil {
		t.Fatalf("ParseReasoningCatalog() error = %v", err)
	}
	budgetMin := 1024
	want := []domainchannel.ReasoningCatalogEntry{
		{Provider: "anthropic", ModelID: "claude-sonnet-4-5", Reasoning: true,
			Options: []domainchannel.ReasoningCatalogOption{{Type: domainchannel.ReasoningCatalogOptionBudget, Min: &budgetMin}}},
		{Provider: "deepseek", ModelID: "deepseek-chat", Reasoning: true,
			Options: []domainchannel.ReasoningCatalogOption{{Type: domainchannel.ReasoningCatalogOptionToggle}}},
		{Provider: "openai", ModelID: "gpt-5", CanonicalID: "openai/gpt-5", Reasoning: true,
			Options: []domainchannel.ReasoningCatalogOption{{Type: domainchannel.ReasoningCatalogOptionEffort, Values: []string{"minimal", "low", "medium", "high", "default"}}}},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("unexpected entries:\n got %#v\nwant %#v", entries, want)
	}
}

func TestParseReasoningCatalogRejectsMalformedOrEmpty(t *testing.T) {
	if _, err := ParseReasoningCatalog([]byte(`[]`)); !errors.Is(err, ErrReasoningCatalogMalformed) {
		t.Fatalf("expected malformed error, got %v", err)
	}
	if _, err := ParseReasoningCatalog([]byte(`{"openai":{"models":{"gpt-4o":{"id":"gpt-4o"}}}}`)); !errors.Is(err, ErrReasoningCatalogEmpty) {
		t.Fatalf("expected empty error, got %v", err)
	}
}

func TestReasoningCatalogSnapshotRoundTrip(t *testing.T) {
	entries, err := ParseReasoningCatalog([]byte(modelsDevFixture))
	if err != nil {
		t.Fatal(err)
	}
	fetchedAt := time.Date(2025, 9, 30, 8, 0, 0, 0, time.UTC)
	snapshot := domainchannel.ReasoningCatalogSnapshot{Source: "https://models.dev/api.json", Origin: domainchannel.ReasoningCatalogOriginRemote, FetchedAt: fetchedAt, Entries: entries}
	encoded, err := EncodeReasoningSnapshot(snapshot)
	if err != nil {
		t.Fatalf("EncodeReasoningSnapshot() error = %v", err)
	}
	again, err := EncodeReasoningSnapshot(snapshot)
	if err != nil || !reflect.DeepEqual(encoded, again) {
		t.Fatal("expected deterministic snapshot encoding")
	}
	decoded, err := DecodeReasoningSnapshot(encoded)
	if err != nil {
		t.Fatalf("DecodeReasoningSnapshot() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, snapshot) {
		t.Fatalf("round trip mismatch:\n got %#v\nwant %#v", decoded, snapshot)
	}
	plain, err := MarshalReasoningSnapshot(snapshot)
	if err != nil || plain[0] != '{' {
		t.Fatalf("expected plain JSON, got %q, %v", plain[:1], err)
	}
	if decoded, err := DecodeReasoningSnapshot(plain); err != nil || !reflect.DeepEqual(decoded, snapshot) {
		t.Fatalf("plain round trip mismatch: %#v, %v", decoded, err)
	}
}

func TestDecodeReasoningSnapshotRejectsInvalidInput(t *testing.T) {
	cases := map[string]string{
		"garbage":       `nope`,
		"wrong version": `{"version":99,"models":[{"provider":"openai","id":"gpt-5","options":[{"type":"toggle"}]}]}`,
		"no models":     `{"version":1,"models":[]}`,
	}
	for name, raw := range cases {
		if _, err := DecodeReasoningSnapshot([]byte(raw)); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
	plain := `{"version":1,"source":"s","models":[{"provider":"openai","id":"gpt-5","options":[{"type":"toggle"}]}]}`
	snapshot, err := DecodeReasoningSnapshot([]byte(plain))
	if err != nil || len(snapshot.Entries) != 1 || !snapshot.Entries[0].Reasoning {
		t.Fatalf("expected plain JSON snapshot to decode, got %#v, %v", snapshot, err)
	}
}
