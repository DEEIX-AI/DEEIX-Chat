package conversation

import (
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestConfigureOpenAIPromptCacheForRoute(t *testing.T) {
	tests := []struct {
		name     string
		route    *channel.ResolvedRoute
		wantKey  string
		wantMode string
		wantTTL  string
	}{
		{
			name: "official OpenAI defaults to enabled",
			route: &channel.ResolvedRoute{
				Protocol: llm.AdapterOpenAIResponses,
				BaseURL:  "https://api.openai.com/v1",
			},
			wantKey: "session-1",
		},
		{
			name: "custom relay requires explicit capability",
			route: &channel.ResolvedRoute{
				Protocol: llm.AdapterOpenAIResponses,
				BaseURL:  "https://relay.example.com/v1",
			},
		},
		{
			name: "custom relay can opt in",
			route: &channel.ResolvedRoute{
				Protocol:              llm.AdapterOpenAIChatCompletions,
				BaseURL:               "https://relay.example.com/v1",
				ModelCapabilitiesJSON: `{"promptCache":{"enabled":true}}`,
			},
			wantKey: "session-1",
		},
		{
			name: "custom relay can enable explicit caching",
			route: &channel.ResolvedRoute{
				Protocol:              llm.AdapterOpenAIResponses,
				BaseURL:               "https://relay.example.com/v1",
				ModelCapabilitiesJSON: `{"promptCache":{"enabled":true,"mode":"explicit","ttl":"30m"}}`,
			},
			wantKey:  "session-1",
			wantMode: "explicit",
			wantTTL:  "30m",
		},
		{
			name: "official OpenAI ignores legacy implicit retention",
			route: &channel.ResolvedRoute{
				Protocol:              llm.AdapterOpenAIChatCompletions,
				BaseURL:               "https://api.openai.com/v1",
				ModelCapabilitiesJSON: `{"promptCache":{"mode":"implicit","retention":"24h"}}`,
			},
			wantKey: "session-1",
		},
		{
			name: "official OpenAI ignores legacy default retention",
			route: &channel.ResolvedRoute{
				Protocol:              llm.AdapterOpenAIResponses,
				BaseURL:               "https://api.openai.com/v1",
				ModelCapabilitiesJSON: `{"defaultOptions":{"prompt_cache_retention":"24h"}}`,
			},
			wantKey: "session-1",
		},
		{
			name: "official OpenAI can be disabled",
			route: &channel.ResolvedRoute{
				Protocol:              llm.AdapterOpenAIResponses,
				BaseURL:               "https://api.openai.com/v1",
				ModelCapabilitiesJSON: `{"promptCache":{"enabled":false}}`,
			},
		},
		{
			name: "non OpenAI adapters stay disabled",
			route: &channel.ResolvedRoute{
				Protocol:              llm.AdapterOpenRouterResponses,
				BaseURL:               "https://openrouter.ai/api/v1",
				ModelCapabilitiesJSON: `{"promptCache":{"enabled":true}}`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := map[string]interface{}{
				"temperature": 0.2,
				"prompt_cache_options": map[string]interface{}{
					"mode": "user-controlled",
				},
				"prompt_cache_retention": "user-controlled",
			}
			key, options := configureOpenAIPromptCacheForRoute(test.route, " session-1 ", original)
			if key != test.wantKey {
				t.Fatalf("expected key %q, got %q", test.wantKey, key)
			}
			cacheOptions, _ := options["prompt_cache_options"].(map[string]interface{})
			if mode, _ := cacheOptions["mode"].(string); mode != test.wantMode {
				t.Fatalf("expected prompt cache mode %q, got %#v", test.wantMode, options)
			}
			if ttl, _ := cacheOptions["ttl"].(string); ttl != test.wantTTL {
				t.Fatalf("expected prompt cache ttl %q, got %#v", test.wantTTL, options)
			}
			if _, exists := options["prompt_cache_retention"]; exists {
				t.Fatalf("expected prompt cache retention to be discarded, got %#v", options)
			}
			if options["temperature"] != 0.2 {
				t.Fatalf("expected unrelated options to remain, got %#v", options)
			}
			if _, stillPresent := original["prompt_cache_options"]; !stillPresent {
				t.Fatalf("expected route filtering not to mutate caller options, got %#v", original)
			}
			if original["prompt_cache_retention"] != "user-controlled" {
				t.Fatalf("expected route filtering not to mutate caller retention, got %#v", original)
			}
		})
	}
}

func TestConfigureOpenAIPromptCacheForRouteDropsFieldsAfterFailoverToUnsupportedRoute(t *testing.T) {
	unsupportedRoute := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "https://legacy-relay.example.com/v1",
	}
	for _, capabilitiesJSON := range []string{
		`{"promptCache":{"enabled":true,"mode":"explicit","ttl":"30m"}}`,
		`{"promptCache":{"enabled":true,"mode":"implicit","retention":"24h"}}`,
	} {
		supportedRoute := &channel.ResolvedRoute{
			Protocol:              llm.AdapterOpenAIResponses,
			BaseURL:               "https://relay.example.com/v1",
			ModelCapabilitiesJSON: capabilitiesJSON,
		}
		key, options := configureOpenAIPromptCacheForRoute(supportedRoute, "session-1", map[string]interface{}{
			"temperature": 0.2,
		})
		if key != "session-1" {
			t.Fatalf("expected supported route cache key, got %q", key)
		}

		key, options = configureOpenAIPromptCacheForRoute(unsupportedRoute, "session-1", options)
		if key != "" {
			t.Fatalf("expected unsupported failover route to clear cache key, got %q", key)
		}
		if _, exists := options["prompt_cache_options"]; exists {
			t.Fatalf("expected unsupported failover route to drop prompt cache options, got %#v", options)
		}
		if _, exists := options["prompt_cache_retention"]; exists {
			t.Fatalf("expected unsupported failover route to drop prompt cache retention, got %#v", options)
		}
		if options["temperature"] != 0.2 {
			t.Fatalf("expected unrelated options to survive failover filtering, got %#v", options)
		}
	}
}

func TestConfigureOpenAIPromptCacheDoesNotDependOnModelOptionAllowlist(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"temperature":            0.2,
		"prompt_cache_options":   map[string]interface{}{"mode": "user-controlled"},
		"prompt_cache_retention": "user-controlled",
	}, llm.AdapterOpenAIResponses, modelOptionPolicyConfig{
		Mode:             modelOptionPolicyAllowlist,
		AllowedPathsJSON: `{"default":["temperature"]}`,
	})
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		BaseURL:               "https://api.openai.com/v1",
		ModelCapabilitiesJSON: `{"promptCache":{"mode":"explicit","ttl":"30m"}}`,
	}

	key, options := configureOpenAIPromptCacheForRoute(route, "session-1", filtered)
	cacheOptions, _ := options["prompt_cache_options"].(map[string]interface{})
	if key != "session-1" || cacheOptions["mode"] != "explicit" || cacheOptions["ttl"] != "30m" {
		t.Fatalf("expected server cache policy to bypass the legacy user allowlist, key=%q options=%#v", key, options)
	}
}

func TestConfigureOpenAIPromptCacheRequestForRouteRecomputesAcrossFailover(t *testing.T) {
	supportedRoute := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		BaseURL:               "https://relay.example.com/v1",
		ModelCapabilitiesJSON: `{"promptCache":{"enabled":true,"mode":"explicit","ttl":"30m","messageBreakpoints":true}}`,
	}
	unsupportedRoute := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "https://legacy-relay.example.com/v1",
	}
	messages := []llm.Message{
		{Role: "system", Content: "stable policy"},
		{Role: "user", Content: "historical question"},
		{Role: "assistant", Content: "historical answer"},
		{Role: "user", Content: "current question"},
	}

	key, options, configuredMessages := configureOpenAIPromptCacheRequestForRoute(
		supportedRoute,
		"session-1",
		explicitOpenAIPromptCacheOptions(),
		messages,
	)
	if key != "session-1" || !usesExplicitOpenAIPromptCache(options) {
		t.Fatalf("expected supported route explicit cache fields, got key=%q options=%#v", key, options)
	}
	assertOpenAIPromptCacheMessageMarkers(t, configuredMessages, 0, 1)

	key, options, configuredMessages = configureOpenAIPromptCacheRequestForRoute(
		unsupportedRoute,
		"session-1",
		options,
		messages,
	)
	if key != "" {
		t.Fatalf("expected unsupported failover route to clear cache key, got %q", key)
	}
	if _, exists := options[openAIPromptCacheOptionKey]; exists {
		t.Fatalf("expected unsupported failover route to drop cache options, got %#v", options)
	}
	assertOpenAIPromptCacheMessageMarkers(t, configuredMessages)

	key, options, configuredMessages = configureOpenAIPromptCacheRequestForRoute(
		supportedRoute,
		"session-1",
		explicitOpenAIPromptCacheOptions(),
		messages,
	)
	if key != "session-1" || !usesExplicitOpenAIPromptCache(options) {
		t.Fatalf("expected supported failover route to restore explicit cache fields, got key=%q options=%#v", key, options)
	}
	assertOpenAIPromptCacheMessageMarkers(t, configuredMessages, 0, 1)
}

func TestConfigureOpenAIPromptCacheRequestForRouteKeepsRelayExplicitOptionsWithoutMessageBreakpoints(t *testing.T) {
	marker := &llm.CacheControl{Type: "old"}
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		BaseURL:               "https://relay.example.com/v1",
		ModelCapabilitiesJSON: `{"promptCache":{"enabled":true,"mode":"explicit","ttl":"30m"}}`,
	}
	messages := []llm.Message{
		{Role: "system", Content: "stable policy", CacheControl: marker},
		{Role: "user", Content: "historical question"},
		{Role: "assistant", Content: "historical answer"},
		{Role: "user", Content: "current question"},
	}

	key, options, configuredMessages := configureOpenAIPromptCacheRequestForRoute(
		route,
		"session-1",
		nil,
		messages,
	)

	if key != "session-1" || !usesExplicitOpenAIPromptCache(options) {
		t.Fatalf("expected relay to retain top-level explicit cache fields, key=%q options=%#v", key, options)
	}
	assertOpenAIPromptCacheMessageMarkers(t, configuredMessages)
	if messages[0].CacheControl != marker {
		t.Fatalf("expected caller messages to remain unchanged, got %#v", messages[0].CacheControl)
	}
}

func TestConfigureOpenAIPromptCacheRequestForRouteCanDisableOfficialMessageBreakpoints(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		BaseURL:               "https://api.openai.com/v1",
		ModelCapabilitiesJSON: `{"promptCache":{"mode":"explicit","ttl":"30m","messageBreakpoints":false}}`,
	}
	messages := []llm.Message{
		{Role: "system", Content: "stable policy"},
		{Role: "user", Content: "historical question"},
		{Role: "assistant", Content: "historical answer"},
		{Role: "user", Content: "current question"},
	}

	key, options, configuredMessages := configureOpenAIPromptCacheRequestForRoute(
		route,
		"session-1",
		nil,
		messages,
	)

	if key != "session-1" || !usesExplicitOpenAIPromptCache(options) {
		t.Fatalf("expected official route to retain top-level explicit cache fields, key=%q options=%#v", key, options)
	}
	assertOpenAIPromptCacheMessageMarkers(t, configuredMessages)
}

func TestApplyOpenAIPromptCacheMessagePolicyMarksStableSystemAndHistoricalUsers(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "https://api.openai.com/v1",
	}
	messages := []llm.Message{
		{Role: "system", Content: "platform policy"},
		{Role: "system", Content: "stable tool policy"},
		{Role: "user", Content: "question one"},
		{Role: "assistant", Content: "answer one"},
		{Role: "user", Content: "question two"},
		{Role: "assistant", Content: "answer two"},
		{Role: "user", Content: "current question with dynamic RAG"},
	}

	result := applyOpenAIPromptCacheMessagePolicy(route, explicitOpenAIPromptCacheOptions(), messages)
	assertOpenAIPromptCacheMessageMarkers(t, result, 1, 2, 4)
	if result[6].CacheControl != nil {
		t.Fatalf("expected current user to remain unmarked, got %#v", result[6].CacheControl)
	}
}

func TestApplyOpenAIPromptCacheMessagePolicyKeepsAllHistoricalUsers(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIChatCompletions,
		BaseURL:  "https://api.openai.com/v1",
	}
	messages := []llm.Message{{Role: "system", Content: "stable policy"}}
	for index := 1; index <= 5; index++ {
		messages = append(messages,
			llm.Message{Role: "user", Content: "historical question"},
			llm.Message{Role: "assistant", Content: "historical answer"},
		)
	}
	messages = append(messages,
		llm.Message{Role: "user", Parts: []llm.ContentPart{{Kind: llm.ContentPartText, Text: " "}}},
		llm.Message{Role: "user", Content: "current question"},
	)

	result := applyOpenAIPromptCacheMessagePolicy(route, explicitOpenAIPromptCacheOptions(), messages)
	assertOpenAIPromptCacheMessageMarkers(t, result, 0, 1, 3, 5, 7, 9)
	for _, index := range []int{11, 12} {
		if result[index].CacheControl != nil {
			t.Fatalf("expected message %d to remain unmarked, got %#v", index, result[index].CacheControl)
		}
	}
}

func TestApplyOpenAIPromptCacheMessagePolicyDoesNotRemoveEarlierBreakpointsAsHistoryAdvances(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "https://api.openai.com/v1",
	}
	firstTurn := []llm.Message{
		{Role: "system", Content: "stable policy"},
		{Role: "user", Content: "question one"},
		{Role: "assistant", Content: "answer one"},
		{Role: "user", Content: "question two"},
		{Role: "assistant", Content: "answer two"},
		{Role: "user", Content: "current question three"},
	}
	secondTurn := append(append([]llm.Message{}, firstTurn...),
		llm.Message{Role: "assistant", Content: "answer three"},
		llm.Message{Role: "user", Content: "current question four"},
	)

	firstResult := applyOpenAIPromptCacheMessagePolicy(route, explicitOpenAIPromptCacheOptions(), firstTurn)
	secondResult := applyOpenAIPromptCacheMessagePolicy(route, explicitOpenAIPromptCacheOptions(), secondTurn)
	assertOpenAIPromptCacheMessageMarkers(t, firstResult, 0, 1, 3)
	assertOpenAIPromptCacheMessageMarkers(t, secondResult, 0, 1, 3, 5)
}

func TestApplyOpenAIPromptCacheMessagePolicyLeavesImplicitMessagesUntouched(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "https://api.openai.com/v1",
	}
	marker := &llm.CacheControl{Type: "ephemeral"}
	messages := []llm.Message{
		{Role: "system", Content: "stable policy", CacheControl: marker},
		{Role: "user", Content: "current question"},
	}
	key, options := configureOpenAIPromptCacheForRoute(route, "session-implicit", map[string]interface{}{"temperature": 0.2})

	result := applyOpenAIPromptCacheMessagePolicy(route, options, messages)
	if key != "session-implicit" {
		t.Fatalf("expected implicit route to retain stable cache key, got %q", key)
	}
	if &result[0] != &messages[0] || result[0].CacheControl != marker {
		t.Fatalf("expected implicit policy to leave caller messages untouched, got %#v", result)
	}
}

func TestApplyOpenAIPromptCacheMessagePolicyLeavesUnsupportedRouteUntouched(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "https://relay.example.com/v1",
	}
	marker := &llm.CacheControl{Type: "ephemeral"}
	messages := []llm.Message{
		{Role: "system", Content: "stable policy", CacheControl: marker},
		{Role: "user", Content: "current question"},
	}
	key, options := configureOpenAIPromptCacheForRoute(route, "session-1", explicitOpenAIPromptCacheOptions())

	result := applyOpenAIPromptCacheMessagePolicy(route, options, messages)
	if key != "" {
		t.Fatalf("expected unsupported route not to send cache key, got %q", key)
	}
	if _, exists := options[openAIPromptCacheOptionKey]; exists {
		t.Fatalf("expected unsupported route not to send cache options, got %#v", options)
	}
	if &result[0] != &messages[0] || result[0].CacheControl != marker {
		t.Fatalf("expected unsupported route policy to leave messages untouched, got %#v", result)
	}
}

func TestApplyOpenAIPromptCacheMessagePolicyDoesNotMutateCallerMessages(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "https://api.openai.com/v1",
	}
	oldSystemMarker := &llm.CacheControl{Type: "old-system"}
	oldCurrentMarker := &llm.CacheControl{Type: "old-current"}
	messages := []llm.Message{
		{Role: "system", Content: "first system", CacheControl: oldSystemMarker},
		{Role: "system", Content: "last system"},
		{Role: "user", Content: "historical question"},
		{Role: "assistant", Content: "historical answer"},
		{Role: "user", Content: "current question", CacheControl: oldCurrentMarker},
	}

	result := applyOpenAIPromptCacheMessagePolicy(route, explicitOpenAIPromptCacheOptions(), messages)
	if &result[0] == &messages[0] {
		t.Fatal("expected explicit policy to clone the caller slice")
	}
	if messages[0].CacheControl != oldSystemMarker || messages[1].CacheControl != nil ||
		messages[2].CacheControl != nil || messages[4].CacheControl != oldCurrentMarker {
		t.Fatalf("expected caller messages to remain unchanged, got %#v", messages)
	}
	assertOpenAIPromptCacheMessageMarkers(t, result, 1, 2)
	if result[0].CacheControl != nil || result[4].CacheControl != nil {
		t.Fatalf("expected obsolete and current-user markers to be cleared, got %#v", result)
	}
}

func explicitOpenAIPromptCacheOptions() map[string]interface{} {
	return map[string]interface{}{
		openAIPromptCacheOptionKey: map[string]interface{}{"mode": "explicit"},
	}
}

func assertOpenAIPromptCacheMessageMarkers(t *testing.T, messages []llm.Message, expected ...int) {
	t.Helper()
	want := make(map[int]struct{}, len(expected))
	for _, index := range expected {
		want[index] = struct{}{}
	}
	for index := range messages {
		_, marked := want[index]
		if (messages[index].CacheControl != nil) != marked {
			t.Fatalf("expected message %d marked=%v, got %#v", index, marked, messages[index].CacheControl)
		}
	}
}
