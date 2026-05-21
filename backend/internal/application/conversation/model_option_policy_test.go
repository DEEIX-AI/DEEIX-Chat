package conversation

import (
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestFilterModelOptionsAllowlistUsesDefaultAndProtocolPaths(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"temperature":  0.7,
		"service_tier": "PRIORITY",
		"model":        "override",
		"reasoning": map[string]interface{}{
			"effort":  "high",
			"summary": "auto",
			"extra":   true,
		},
		"text": map[string]interface{}{
			"verbosity": "low",
		},
		"stream_options": map[string]interface{}{
			"include_usage": true,
		},
	}, llm.AdapterOpenAIResponses, modelOptionPolicyConfig{
		Mode:             modelOptionPolicyAllowlist,
		AllowedPathsJSON: config.DefaultModelOptionAllowedPathsJSON(),
		DeniedPathsJSON:  `{"default":["reasoning.effort"]}`,
	})

	if filtered["temperature"] != 0.7 {
		t.Fatalf("expected temperature to pass, got %#v", filtered)
	}
	if filtered["service_tier"] != "priority" {
		t.Fatalf("expected service_tier to pass, got %#v", filtered)
	}
	if _, ok := filtered["model"]; ok {
		t.Fatalf("expected model to be denied, got %#v", filtered)
	}
	reasoning := filtered["reasoning"].(map[string]interface{})
	if reasoning["effort"] != "high" || reasoning["summary"] != "auto" {
		t.Fatalf("expected allowed reasoning fields, got %#v", reasoning)
	}
	if _, ok := reasoning["extra"]; ok {
		t.Fatalf("expected unlisted reasoning.extra to be removed, got %#v", reasoning)
	}
	if _, ok := filtered["stream_options"]; ok {
		t.Fatalf("expected chat-only stream_options to be removed for responses, got %#v", filtered)
	}
}

func TestFilterModelOptionsRejectsUnsupportedOpenAIServiceTier(t *testing.T) {
	for _, serviceTier := range []string{"auto", "scale", "unknown"} {
		t.Run(serviceTier, func(t *testing.T) {
			filtered := filterModelOptions(map[string]interface{}{
				"temperature":  0.7,
				"service_tier": serviceTier,
			}, llm.AdapterOpenAIResponses, modelOptionPolicyConfig{
				Mode:             modelOptionPolicyAllowlist,
				AllowedPathsJSON: config.DefaultModelOptionAllowedPathsJSON(),
				DeniedPathsJSON:  config.DefaultModelOptionDeniedPathsJSON(),
			})

			if _, ok := filtered["service_tier"]; ok {
				t.Fatalf("expected unsupported service_tier to be removed, got %#v", filtered)
			}
			if filtered["temperature"] != 0.7 {
				t.Fatalf("expected other allowed options to remain, got %#v", filtered)
			}
		})
	}
}

func TestFilterModelOptionsDenylistAllowsUnlistedAndRemovesDenied(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"temperature":          0.2,
		"custom_vendor_option": true,
		"previous_response_id": "resp_123",
		"reasoning": map[string]interface{}{
			"effort": "high",
		},
	}, llm.AdapterXAIResponses, modelOptionPolicyConfig{
		Mode:             modelOptionPolicyDenylist,
		AllowedPathsJSON: config.DefaultModelOptionAllowedPathsJSON(),
		DeniedPathsJSON:  `{"default":["reasoning.effort"]}`,
	})

	if filtered["custom_vendor_option"] != true {
		t.Fatalf("expected custom option to pass in denylist mode, got %#v", filtered)
	}
	if _, ok := filtered["previous_response_id"]; ok {
		t.Fatalf("expected previous_response_id to be hard denied, got %#v", filtered)
	}
	if reasoning, ok := filtered["reasoning"].(map[string]interface{}); ok {
		if _, ok := reasoning["effort"]; ok {
			t.Fatalf("expected configured deny path removed, got %#v", filtered)
		}
	}
}

func TestFilterModelOptionsOpenAIChatCompletionsAllowsThinkingType(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"thinking": map[string]interface{}{
			"type":          "enabled",
			"budget_tokens": 1024,
		},
	}, llm.AdapterOpenAIChatCompletions, modelOptionPolicyConfig{
		Mode:             modelOptionPolicyAllowlist,
		AllowedPathsJSON: config.DefaultModelOptionAllowedPathsJSON(),
		DeniedPathsJSON:  config.DefaultModelOptionDeniedPathsJSON(),
	})

	thinking := filtered["thinking"].(map[string]interface{})
	if thinking["type"] != "enabled" {
		t.Fatalf("expected thinking.type to pass for chat completions, got %#v", filtered)
	}
	if _, ok := thinking["budget_tokens"]; ok {
		t.Fatalf("expected unlisted thinking.budget_tokens to be removed for chat completions, got %#v", filtered)
	}
}

func TestFilterModelOptionsGeminiPolicyKeyMatchesGoogleAdapter(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"generationConfig": map[string]interface{}{
			"temperature":      0.4,
			"responseMimeType": "application/json",
			"candidateCount":   3,
		},
	}, llm.AdapterGoogleGenerateContent, modelOptionPolicyConfig{
		Mode:             modelOptionPolicyAllowlist,
		AllowedPathsJSON: config.DefaultModelOptionAllowedPathsJSON(),
		DeniedPathsJSON:  config.DefaultModelOptionDeniedPathsJSON(),
	})

	generationConfig := filtered["generationConfig"].(map[string]interface{})
	if generationConfig["temperature"] != 0.4 || generationConfig["responseMimeType"] != "application/json" {
		t.Fatalf("expected gemini allowlist fields, got %#v", generationConfig)
	}
	if _, ok := generationConfig["candidateCount"]; ok {
		t.Fatalf("expected unlisted gemini option removed, got %#v", generationConfig)
	}
}

func TestFilterModelOptionsOpenAIImageGenerationsAllowsImageParams(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"size":               "1024x1024",
		"quality":            "high",
		"response_format":    "b64_json",
		"output_format":      "webp",
		"output_compression": 80,
		"partial_images":     2,
		"prompt":             "override",
		"stream":             true,
	}, llm.AdapterOpenAIImageGenerations, modelOptionPolicyConfig{
		Mode:             modelOptionPolicyAllowlist,
		AllowedPathsJSON: config.DefaultModelOptionAllowedPathsJSON(),
		DeniedPathsJSON:  config.DefaultModelOptionDeniedPathsJSON(),
	})

	if filtered["size"] != "1024x1024" || filtered["quality"] != "high" || filtered["response_format"] != "b64_json" {
		t.Fatalf("expected image generation params to pass, got %#v", filtered)
	}
	if filtered["output_format"] != "webp" || filtered["output_compression"] != 80 {
		t.Fatalf("expected image output params to pass, got %#v", filtered)
	}
	if _, ok := filtered["prompt"]; ok {
		t.Fatalf("expected prompt override to be hard denied, got %#v", filtered)
	}
	if _, ok := filtered["stream"]; ok {
		t.Fatalf("expected stream override to be hard denied, got %#v", filtered)
	}
	if filtered["partial_images"] != 2 {
		t.Fatalf("expected partial_images to pass for upstream image streaming, got %#v", filtered)
	}
}
