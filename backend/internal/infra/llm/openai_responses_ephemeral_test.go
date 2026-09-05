package llm

import (
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestBuildResponsesRequestBodyEnforcesEphemeralRequest(t *testing.T) {
	payload := buildResponsesRequestBody(responsesRequestInput{
		Adapter: portllm.AdapterOpenAIResponses,
		Model:   "gpt-test",
		Generate: portllm.GenerateInput{
			Messages:            []portllm.Message{{Role: "user", Content: "hello"}},
			PromptCacheKey:      "cache_should_not_be_sent",
			PreviousResponseID:  "resp_should_not_be_sent",
			ResponsesBackground: true,
			Ephemeral:           true,
			Options: map[string]any{
				"store":      true,
				"background": true,
				"prompt_cache_options": map[string]any{
					"mode": "explicit",
				},
			},
		},
		Stream: true,
	})

	if store, ok := payload["store"].(bool); !ok || store {
		t.Fatalf("store = %#v, want false", payload["store"])
	}
	if _, ok := payload["background"]; ok {
		t.Fatalf("background must be omitted for ephemeral requests: %#v", payload)
	}
	if _, ok := payload["previous_response_id"]; ok {
		t.Fatalf("previous_response_id must be omitted for ephemeral requests: %#v", payload)
	}
	if _, ok := payload["prompt_cache_key"]; ok {
		t.Fatalf("prompt_cache_key must be omitted for ephemeral requests: %#v", payload)
	}
	if _, ok := payload["prompt_cache_options"]; ok {
		t.Fatalf("prompt_cache_options must be omitted for ephemeral requests: %#v", payload)
	}
}

func TestBuildAnthropicRequestBodyDisablesEphemeralPromptCache(t *testing.T) {
	cacheControl := &portllm.CacheControl{Type: "ephemeral", TTL: "1h"}
	payload, err := buildAnthropicRequestBody("claude-test", portllm.GenerateInput{
		Messages:  []portllm.Message{{Role: "system", Content: "system", CacheControl: cacheControl}, {Role: "user", Content: "hello"}},
		Ephemeral: true,
		Options: map[string]any{
			"cache_control": map[string]any{"type": "ephemeral", "ttl": "1h"},
		},
	}, true)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if _, ok := payload["cache_control"]; ok {
		t.Fatalf("cache_control must be omitted for ephemeral requests: %#v", payload)
	}
	if system, ok := payload["system"].([]map[string]any); ok {
		for _, block := range system {
			if _, exists := block["cache_control"]; exists {
				t.Fatalf("system cache_control must be omitted for ephemeral requests: %#v", payload)
			}
		}
	}
}
