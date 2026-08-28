package llm

import "testing"

func TestBuildResponsesRequestBodyEnforcesEphemeralRequest(t *testing.T) {
	payload := buildResponsesRequestBody(
		AdapterOpenAIResponses,
		"gpt-test",
		GenerateInput{
			Messages:            []Message{{Role: "user", Content: "hello"}},
			PromptCacheKey:      "cache_should_not_be_sent",
			PreviousResponseID:  "resp_should_not_be_sent",
			ResponsesBackground: true,
			Ephemeral:           true,
			Options: map[string]interface{}{
				"store":      true,
				"background": true,
				"prompt_cache_options": map[string]interface{}{
					"mode": "explicit",
				},
			},
		},
		nil,
		nil,
		nil,
		false,
		nil,
		true,
	)

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
	cacheControl := &CacheControl{Type: "ephemeral", TTL: "1h"}
	payload, err := buildAnthropicRequestBody("claude-test", GenerateInput{
		Messages:  []Message{{Role: "system", Content: "system", CacheControl: cacheControl}, {Role: "user", Content: "hello"}},
		Ephemeral: true,
		Options: map[string]interface{}{
			"cache_control": map[string]interface{}{"type": "ephemeral", "ttl": "1h"},
		},
	}, true)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if _, ok := payload["cache_control"]; ok {
		t.Fatalf("cache_control must be omitted for ephemeral requests: %#v", payload)
	}
	if system, ok := payload["system"].([]map[string]interface{}); ok {
		for _, block := range system {
			if _, exists := block["cache_control"]; exists {
				t.Fatalf("system cache_control must be omitted for ephemeral requests: %#v", payload)
			}
		}
	}
}
