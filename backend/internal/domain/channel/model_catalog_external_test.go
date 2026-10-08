package channel

import "testing"

func TestMatchExternalFallsBackToOpenRouterIDs(t *testing.T) {
	catalog := NewModelCatalog([]ModelCatalogEntry{
		{Provider: "anthropic", ModelID: "claude-sonnet-4-5", InputModalities: []string{"text", "image"}, ContextWindow: 200000},
		{Provider: "openrouter", ModelID: "anthropic/claude-sonnet-4.5", InputModalities: []string{"text", "image", "pdf"}, ContextWindow: 1000000},
	})
	openAIKeys := []string{"openai_chat_completions"}

	if entry := catalog.Match("anthropic/claude-sonnet-4.5", "anthropic", openAIKeys); entry != nil {
		t.Fatalf("Match must stay strict for platform routes, got %s/%s", entry.Provider, entry.ModelID)
	}
	for _, name := range []string{"anthropic/claude-sonnet-4.5", "claude-sonnet-4.5"} {
		entry := catalog.MatchExternal(name, "anthropic", openAIKeys)
		if entry == nil || entry.Provider != "openrouter" {
			t.Fatalf("MatchExternal(%q) = %+v, want the openrouter entry", name, entry)
		}
	}
	if entry := catalog.MatchExternal("claude-sonnet-4-5", "anthropic", openAIKeys); entry == nil || entry.Provider != "anthropic" {
		t.Fatalf("a first-party hit must win, got %+v", entry)
	}
	if entry := catalog.MatchExternal("unknown/model", "", openAIKeys); entry != nil {
		t.Fatalf("unknown model must not match, got %+v", entry)
	}
}
