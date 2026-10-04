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
			"reasoning_options": [{"type": "effort", "values": ["minimal", "low", "medium", "high", null, "default"]}],
			"modalities": {"input": ["image", "text", "pdf", "hologram"], "output": ["text"]},
			"limit": {"context": 400000, "input": 272000, "output": 128000}},
		"gpt-4o": {"id": "gpt-4o", "reasoning": false, "modalities": {"input": ["text", "image"]}, "limit": {"context": 1000}},
		"gpt-3.5": {"id": "gpt-3.5", "reasoning": false},
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

// 目录同时保留推理能力与输入模态：非推理模型只要声明了模态也保留，模态按规范顺序归一化并丢弃未知值；
// limit.context 超出平台允许范围（如 1000）时视为未知。
func TestParseCatalogKeepsReasoningAndModalities(t *testing.T) {
	entries, err := ParseCatalog([]byte(modelsDevFixture))
	if err != nil {
		t.Fatalf("ParseCatalog() error = %v", err)
	}
	budgetMin := 1024
	want := []domainchannel.ModelCatalogEntry{
		{Provider: "anthropic", ModelID: "claude-sonnet-4-5", Reasoning: true,
			Options: []domainchannel.ReasoningCatalogOption{{Type: domainchannel.ReasoningCatalogOptionBudget, Min: &budgetMin}}},
		{Provider: "deepseek", ModelID: "deepseek-chat", Reasoning: true,
			Options: []domainchannel.ReasoningCatalogOption{{Type: domainchannel.ReasoningCatalogOptionToggle}}},
		{Provider: "openai", ModelID: "gpt-4o", InputModalities: []string{"text", "image"}},
		{Provider: "openai", ModelID: "gpt-5", CanonicalID: "openai/gpt-5", Reasoning: true,
			Options:         []domainchannel.ReasoningCatalogOption{{Type: domainchannel.ReasoningCatalogOptionEffort, Values: []string{"minimal", "low", "medium", "high", "default"}}},
			InputModalities: []string{"text", "image", "pdf"}, OutputModalities: []string{"text"}, ContextWindow: 400_000},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("unexpected entries:\n got %#v\nwant %#v", entries, want)
	}
}

func TestParseCatalogRejectsMalformedOrEmpty(t *testing.T) {
	if _, err := ParseCatalog([]byte(`[]`)); !errors.Is(err, ErrCatalogMalformed) {
		t.Fatalf("expected malformed error, got %v", err)
	}
	if _, err := ParseCatalog([]byte(`{"openai":{"models":{"gpt-4o":{"id":"gpt-4o"}}}}`)); !errors.Is(err, ErrCatalogEmpty) {
		t.Fatalf("expected empty error, got %v", err)
	}
}

func TestCatalogSnapshotRoundTrip(t *testing.T) {
	entries, err := ParseCatalog([]byte(modelsDevFixture))
	if err != nil {
		t.Fatal(err)
	}
	fetchedAt := time.Date(2025, 9, 30, 8, 0, 0, 0, time.UTC)
	snapshot := domainchannel.ModelCatalogSnapshot{Source: "https://models.dev/api.json", Origin: domainchannel.ModelCatalogOriginRemote, FetchedAt: fetchedAt, Entries: entries}
	encoded, err := EncodeSnapshot(snapshot)
	if err != nil {
		t.Fatalf("EncodeSnapshot() error = %v", err)
	}
	again, err := EncodeSnapshot(snapshot)
	if err != nil || !reflect.DeepEqual(encoded, again) {
		t.Fatal("expected deterministic snapshot encoding")
	}
	decoded, err := DecodeSnapshot(encoded)
	if err != nil {
		t.Fatalf("DecodeSnapshot() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, snapshot) {
		t.Fatalf("round trip mismatch:\n got %#v\nwant %#v", decoded, snapshot)
	}
	plain, err := MarshalSnapshot(snapshot)
	if err != nil || plain[0] != '{' {
		t.Fatalf("expected plain JSON, got %q, %v", plain[:1], err)
	}
	if decoded, err := DecodeSnapshot(plain); err != nil || !reflect.DeepEqual(decoded, snapshot) {
		t.Fatalf("plain round trip mismatch: %#v, %v", decoded, err)
	}
}

func TestDecodeSnapshotRejectsInvalidInput(t *testing.T) {
	cases := map[string]string{
		"garbage":       `nope`,
		"wrong version": `{"version":99,"models":[{"provider":"openai","id":"gpt-5","options":[{"type":"toggle"}]}]}`,
		"no models":     `{"version":2,"models":[]}`,
	}
	for name, raw := range cases {
		if _, err := DecodeSnapshot([]byte(raw)); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
	// v1 快照没有输入模态，按过期处理，调用方视同缺失并由内置快照重新写入。
	legacy := `{"version":1,"models":[{"provider":"openai","id":"gpt-5","options":[{"type":"toggle"}]}]}`
	if _, err := DecodeSnapshot([]byte(legacy)); !errors.Is(err, ErrSnapshotOutdated) {
		t.Fatalf("expected outdated error for v1 snapshot, got %v", err)
	}
	plain := `{"version":2,"source":"s","models":[{"provider":"openai","id":"gpt-5","options":[{"type":"toggle"}]},{"provider":"openai","id":"gpt-4o","inputs":["image","text"]}]}`
	snapshot, err := DecodeSnapshot([]byte(plain))
	if err != nil || len(snapshot.Entries) != 2 || !snapshot.Entries[0].Reasoning || snapshot.Entries[1].Reasoning ||
		!reflect.DeepEqual(snapshot.Entries[1].InputModalities, []string{"text", "image"}) {
		t.Fatalf("expected plain JSON snapshot to decode, got %#v, %v", snapshot, err)
	}
}
