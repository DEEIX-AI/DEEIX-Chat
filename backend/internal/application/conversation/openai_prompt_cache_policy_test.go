package conversation

import (
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestConfigureOpenAIPromptCacheForRoute(t *testing.T) {
	tests := []struct {
		name        string
		route       *channel.ResolvedRoute
		wantKey     string
		wantOptions bool
	}{
		{
			name: "official OpenAI defaults to enabled",
			route: &channel.ResolvedRoute{
				Protocol: llm.AdapterOpenAIResponses,
				BaseURL:  "https://api.openai.com/v1",
			},
			wantKey:     "session-1",
			wantOptions: true,
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
			wantKey:     "session-1",
			wantOptions: true,
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
					"mode": "explicit",
				},
			}
			key, options := configureOpenAIPromptCacheForRoute(test.route, " session-1 ", original)
			if key != test.wantKey {
				t.Fatalf("expected key %q, got %q", test.wantKey, key)
			}
			_, hasOptions := options["prompt_cache_options"]
			if hasOptions != test.wantOptions {
				t.Fatalf("expected prompt cache options=%v, got %#v", test.wantOptions, options)
			}
			if options["temperature"] != 0.2 {
				t.Fatalf("expected unrelated options to remain, got %#v", options)
			}
			if _, stillPresent := original["prompt_cache_options"]; !stillPresent {
				t.Fatalf("expected route filtering not to mutate caller options, got %#v", original)
			}
		})
	}
}

func TestConfigureOpenAIPromptCacheForRouteDropsFieldsAfterFailoverToUnsupportedRoute(t *testing.T) {
	supportedRoute := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		BaseURL:               "https://relay.example.com/v1",
		ModelCapabilitiesJSON: `{"promptCache":{"enabled":true}}`,
	}
	unsupportedRoute := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "https://legacy-relay.example.com/v1",
	}

	key, options := configureOpenAIPromptCacheForRoute(supportedRoute, "session-1", map[string]interface{}{
		"temperature": 0.2,
		"prompt_cache_options": map[string]interface{}{
			"mode": "explicit",
		},
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
	if options["temperature"] != 0.2 {
		t.Fatalf("expected unrelated options to survive failover filtering, got %#v", options)
	}
}
