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

func TestFilterModelOptionsPreservesOfficialNativeToolsOutsidePathPolicy(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"temperature": 0.4,
		"tools": []interface{}{
			map[string]interface{}{"type": "web_search_20260209", "max_uses": 3, "name": "override"},
			map[string]interface{}{"type": "custom_tool", "name": "provider_lookup"},
			map[string]interface{}{"type": "web_search_20260209"},
			"invalid",
		},
	}, llm.AdapterAnthropicMessages, modelOptionPolicyConfig{
		Mode:                  modelOptionPolicyAllowlist,
		AllowedPathsJSON:      `{"default":["temperature"]}`,
		DeniedPathsJSON:       config.DefaultModelOptionDeniedPathsJSON(),
		ModelCapabilitiesJSON: `{"nativeToolKeys":["anthropic.web_search_20260209"]}`,
	})

	if filtered["temperature"] != 0.4 {
		t.Fatalf("expected allowed scalar option to pass, got %#v", filtered)
	}
	tools, ok := filtered["tools"].([]map[string]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("expected one sanitized official native tool, got %#v", filtered["tools"])
	}
	if tools[0]["type"] != "web_search_20260209" || tools[0]["name"] != "web_search" {
		t.Fatalf("expected sanitized web_search tool, got %#v", tools[0])
	}
	if tools[0]["max_uses"] != 3 {
		t.Fatalf("expected official native tool parameters to pass, got %#v", tools[0])
	}
}

func TestFilterModelOptionsPreservesXAINativeToolsWhenToolsIsExplicitlyDenied(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"tools": []interface{}{
			map[string]interface{}{
				"type":                       "x_search",
				"enable_image_understanding": true,
				"allowed_domains":            []interface{}{"x.com"},
			},
			map[string]interface{}{"type": "not_official"},
		},
	}, llm.AdapterXAIResponses, modelOptionPolicyConfig{
		Mode:                  modelOptionPolicyDenylist,
		AllowedPathsJSON:      config.DefaultModelOptionAllowedPathsJSON(),
		DeniedPathsJSON:       `{"default":["tools"]}`,
		ModelCapabilitiesJSON: `{"nativeToolKeys":["xai.x_search"]}`,
	})

	tools, ok := filtered["tools"].([]map[string]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("expected official xAI native tool to bypass option denylist, got %#v", filtered)
	}
	if tools[0]["type"] != "x_search" {
		t.Fatalf("expected sanitized x_search tool, got %#v", tools[0])
	}
	if tools[0]["enable_image_understanding"] != true {
		t.Fatalf("expected xAI native tool parameters to pass, got %#v", tools[0])
	}
	domains, ok := tools[0]["allowed_domains"].([]interface{})
	if !ok || len(domains) != 1 || domains[0] != "x.com" {
		t.Fatalf("expected xAI domain parameters to pass, got %#v", tools[0])
	}
}

func TestFilterModelOptionsPreservesAllowedXAINativeToolParameters(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"store": false,
		"tools": []interface{}{
			map[string]interface{}{
				"type":                       "x_search",
				"enable_image_understanding": true,
			},
			map[string]interface{}{
				"type":                       "web_search",
				"enable_image_understanding": true,
				"enable_image_search":        true,
			},
			map[string]interface{}{
				"type": "code_interpreter",
				"container": map[string]interface{}{
					"type": "auto",
				},
			},
			map[string]interface{}{"type": "unknown_tool", "enable_image_understanding": true},
		},
	}, llm.AdapterXAIResponses, modelOptionPolicyConfig{
		Mode:                  modelOptionPolicyAllowlist,
		AllowedPathsJSON:      `{"default":["store"]}`,
		DeniedPathsJSON:       config.DefaultModelOptionDeniedPathsJSON(),
		ModelCapabilitiesJSON: `{"nativeToolKeys":["xai.x_search","xai.web_search","xai.code_interpreter"]}`,
	})

	if filtered["store"] != false {
		t.Fatalf("expected allowed non-tool option to pass, got %#v", filtered)
	}
	tools, ok := filtered["tools"].([]map[string]interface{})
	if !ok || len(tools) != 3 {
		t.Fatalf("expected three allowed xAI native tools, got %#v", filtered["tools"])
	}
	if tools[0]["type"] != "x_search" || tools[0]["enable_image_understanding"] != true {
		t.Fatalf("expected x_search image understanding parameter to pass, got %#v", tools[0])
	}
	if tools[1]["type"] != "web_search" || tools[1]["enable_image_understanding"] != true || tools[1]["enable_image_search"] != true {
		t.Fatalf("expected web_search image parameters to pass, got %#v", tools[1])
	}
	container, ok := tools[2]["container"].(map[string]interface{})
	if tools[2]["type"] != "code_interpreter" || !ok || container["type"] != "auto" {
		t.Fatalf("expected code_interpreter parameters to pass, got %#v", tools[2])
	}
}

func TestFilterModelOptionsPreservesConfiguredNativeToolsAndDropsExternalTools(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"store": false,
		"tools": []interface{}{
			map[string]interface{}{
				"type":                       "x_search",
				"enable_image_understanding": true,
			},
			map[string]interface{}{
				"type":            "future_search",
				"fresh_parameter": "enabled",
			},
			map[string]interface{}{
				"type":   "external_function",
				"name":   "server_attack",
				"strict": true,
			},
			map[string]interface{}{
				"type": "disabled_native_tool",
			},
		},
	}, llm.AdapterXAIResponses, modelOptionPolicyConfig{
		Mode:             modelOptionPolicyAllowlist,
		AllowedPathsJSON: `{"default":["store"]}`,
		DeniedPathsJSON:  config.DefaultModelOptionDeniedPathsJSON(),
		ModelCapabilitiesJSON: `{
			"nativeTools": [
				{
					"key": "xai.x_search",
					"protocols": ["xai_responses"],
					"type": "x_search",
					"enabled": true,
					"payload": {"type": "x_search"}
				},
				{
					"key": "xai.future_search",
					"protocols": ["xai_responses"],
					"type": "future_search",
					"enabled": true,
					"payload": {"type": "future_search"}
				},
				{
					"key": "xai.disabled_native_tool",
					"protocols": ["xai_responses"],
					"type": "disabled_native_tool",
					"enabled": false,
					"payload": {"type": "disabled_native_tool"}
				}
			]
		}`,
	})

	if filtered["store"] != false {
		t.Fatalf("expected allowed non-tool option to pass, got %#v", filtered)
	}
	tools, ok := filtered["tools"].([]map[string]interface{})
	if !ok || len(tools) != 2 {
		t.Fatalf("expected configured native tools only, got %#v", filtered["tools"])
	}
	if tools[0]["type"] != "x_search" || tools[0]["enable_image_understanding"] != true {
		t.Fatalf("expected catalog native tool parameters to pass, got %#v", tools[0])
	}
	if tools[1]["type"] != "future_search" || tools[1]["fresh_parameter"] != "enabled" {
		t.Fatalf("expected administrator-defined native tool parameters to pass, got %#v", tools[1])
	}
}

func TestFilterModelOptionsPreservesNativeToolAcrossConfiguredProtocols(t *testing.T) {
	capabilitiesJSON := `{
		"nativeTools": [
			{
				"key": "openai.web_search",
				"protocols": ["openai_chat_completions", "openai_responses"],
				"type": "web_search",
				"enabled": true,
				"payload": {"type": "web_search"}
			}
		]
	}`
	for _, adapter := range []string{llm.AdapterOpenAIChatCompletions, llm.AdapterOpenAIResponses} {
		t.Run(adapter, func(t *testing.T) {
			filtered := filterModelOptions(map[string]interface{}{
				"tools": []interface{}{
					map[string]interface{}{
						"type":                "web_search",
						"search_context_size": "low",
					},
				},
			}, adapter, modelOptionPolicyConfig{
				Mode:                  modelOptionPolicyAllowlist,
				AllowedPathsJSON:      `{"default":[]}`,
				DeniedPathsJSON:       config.DefaultModelOptionDeniedPathsJSON(),
				ModelCapabilitiesJSON: capabilitiesJSON,
			})

			tools, ok := filtered["tools"].([]map[string]interface{})
			if !ok || len(tools) != 1 {
				t.Fatalf("expected one official tool for %s, got %#v", adapter, filtered)
			}
			if tools[0]["type"] != "web_search" || tools[0]["search_context_size"] != "low" {
				t.Fatalf("expected web_search parameters to pass for %s, got %#v", adapter, tools[0])
			}
		})
	}
}

func TestFilterModelOptionsDerivesNativeToolKeysFromCapabilityDefaultTools(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"store": false,
		"tools": []interface{}{
			map[string]interface{}{
				"type":                       "x_search",
				"enable_image_understanding": true,
			},
			map[string]interface{}{
				"type":                       "web_search",
				"enable_image_understanding": true,
			},
			map[string]interface{}{
				"type": "code_interpreter",
				"container": map[string]interface{}{
					"type": "auto",
				},
			},
		},
	}, llm.AdapterXAIResponses, modelOptionPolicyConfig{
		Mode:             modelOptionPolicyAllowlist,
		AllowedPathsJSON: `{"default":["store"]}`,
		DeniedPathsJSON:  config.DefaultModelOptionDeniedPathsJSON(),
		ModelCapabilitiesJSON: `{
			"defaultOptions": {
				"tools": [
					{"type": "x_search"},
					{"type": "web_search"},
					{"type": "code_interpreter"}
				]
			}
		}`,
	})

	tools, ok := filtered["tools"].([]map[string]interface{})
	if !ok || len(tools) != 3 {
		t.Fatalf("expected native tool keys to be derived from capability default tools, got %#v", filtered)
	}
	if tools[0]["type"] != "x_search" || tools[0]["enable_image_understanding"] != true {
		t.Fatalf("expected derived x_search to preserve parameters, got %#v", tools[0])
	}
	if tools[1]["type"] != "web_search" || tools[1]["enable_image_understanding"] != true {
		t.Fatalf("expected derived web_search to preserve parameters, got %#v", tools[1])
	}
	container, ok := tools[2]["container"].(map[string]interface{})
	if tools[2]["type"] != "code_interpreter" || !ok || container["type"] != "auto" {
		t.Fatalf("expected derived code_interpreter to preserve parameters, got %#v", tools[2])
	}
}

func TestFilterModelOptionsDropsProviderNativeToolsDisabledByPolicy(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"temperature": 0.4,
		"tools": []interface{}{
			map[string]interface{}{"type": "web_search_20260209"},
		},
	}, llm.AdapterAnthropicMessages, modelOptionPolicyConfig{
		Mode:                  modelOptionPolicyAllowlist,
		AllowedPathsJSON:      `{"default":["temperature"]}`,
		DeniedPathsJSON:       config.DefaultModelOptionDeniedPathsJSON(),
		ModelCapabilitiesJSON: `{"nativeToolKeys":[]}`,
	})

	if filtered["temperature"] != 0.4 {
		t.Fatalf("expected allowed scalar option to pass, got %#v", filtered)
	}
	if _, ok := filtered["tools"]; ok {
		t.Fatalf("expected disabled native tools to be removed, got %#v", filtered)
	}
}

func TestFilterModelOptionsPreservesOpenAIResponsesNativeTools(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"tools": []interface{}{
			map[string]interface{}{"type": "web_search_preview", "search_context_size": "low"},
			map[string]interface{}{"type": "shell"},
		},
	}, llm.AdapterOpenAIResponses, modelOptionPolicyConfig{
		Mode:                  modelOptionPolicyAllowlist,
		AllowedPathsJSON:      `{"default":[]}`,
		DeniedPathsJSON:       config.DefaultModelOptionDeniedPathsJSON(),
		ModelCapabilitiesJSON: `{"nativeToolKeys":["openai.web_search_preview","openai.shell"]}`,
	})

	tools, ok := filtered["tools"].([]map[string]interface{})
	if !ok || len(tools) != 2 {
		t.Fatalf("expected sanitized OpenAI native tools, got %#v", filtered)
	}
	if tools[0]["type"] != "web_search_preview" {
		t.Fatalf("expected web_search_preview to pass, got %#v", tools[0])
	}
	if tools[0]["search_context_size"] != "low" {
		t.Fatalf("expected OpenAI native tool parameters to pass, got %#v", tools[0])
	}
	environment, ok := tools[1]["environment"].(map[string]interface{})
	if !ok || environment["type"] != "container_auto" {
		t.Fatalf("expected shell environment to be normalized, got %#v", tools[1])
	}
}

func TestFilterModelOptionsPreservesNativeToolsForcedByModelCapabilitiesAcrossProtocol(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"quality": "auto",
		"tools": []interface{}{
			map[string]interface{}{"type": "web_search_preview", "search_context_size": "medium"},
		},
	}, llm.AdapterOpenAIImageEdits, modelOptionPolicyConfig{
		Mode:                  modelOptionPolicyAllowlist,
		AllowedPathsJSON:      config.DefaultModelOptionAllowedPathsJSON(),
		DeniedPathsJSON:       config.DefaultModelOptionDeniedPathsJSON(),
		ModelCapabilitiesJSON: `{"nativeToolKeys":["openai.web_search_preview"]}`,
	})

	if filtered["quality"] != "auto" {
		t.Fatalf("expected image edit option to pass, got %#v", filtered)
	}
	tools, ok := filtered["tools"].([]map[string]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("expected forced native tool to pass across protocol, got %#v", filtered)
	}
	if tools[0]["type"] != "web_search_preview" {
		t.Fatalf("expected canonical web_search_preview tool, got %#v", tools[0])
	}
	if tools[0]["search_context_size"] != "medium" {
		t.Fatalf("expected forced native tool parameters to pass, got %#v", tools[0])
	}
}

func TestFilterModelOptionsGeminiPolicyKeyMatchesGoogleAdapter(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"generationConfig": map[string]interface{}{
			"temperature":      0.4,
			"responseMimeType": "application/json",
			"candidateCount":   3,
		},
		"tools": []interface{}{
			map[string]interface{}{"type": "google_search"},
		},
	}, llm.AdapterGoogleGenerateContent, modelOptionPolicyConfig{
		Mode:                  modelOptionPolicyAllowlist,
		AllowedPathsJSON:      config.DefaultModelOptionAllowedPathsJSON(),
		DeniedPathsJSON:       config.DefaultModelOptionDeniedPathsJSON(),
		ModelCapabilitiesJSON: `{"nativeToolKeys":["google.google_search"]}`,
	})

	generationConfig := filtered["generationConfig"].(map[string]interface{})
	if generationConfig["temperature"] != 0.4 || generationConfig["responseMimeType"] != "application/json" {
		t.Fatalf("expected gemini allowlist fields, got %#v", generationConfig)
	}
	if _, ok := generationConfig["candidateCount"]; ok {
		t.Fatalf("expected unlisted gemini option removed, got %#v", generationConfig)
	}
	tools := filtered["tools"].([]map[string]interface{})
	if len(tools) != 1 || tools[0]["type"] != "google_search" {
		t.Fatalf("expected Gemini google_search tool, got %#v", tools)
	}
}

func TestFilterModelOptionsGoogleImageAllowsImageConfigAndGoogleSearch(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"generationConfig": map[string]interface{}{
			"responseModalities": "IMAGE",
			"imageConfig": map[string]interface{}{
				"aspectRatio": "1:1",
				"imageSize":   "1K",
			},
			"responseFormat": map[string]interface{}{"image": map[string]interface{}{"aspectRatio": "4:3"}},
			"temperature":    0.5,
		},
		"tools": []interface{}{
			map[string]interface{}{"google_search": map[string]interface{}{}},
		},
	}, llm.AdapterGoogleImageGeneration, modelOptionPolicyConfig{
		Mode:                  modelOptionPolicyAllowlist,
		AllowedPathsJSON:      config.DefaultModelOptionAllowedPathsJSON(),
		DeniedPathsJSON:       config.DefaultModelOptionDeniedPathsJSON(),
		ModelCapabilitiesJSON: `{"nativeToolKeys":["google.google_search"]}`,
	})

	generationConfig := filtered["generationConfig"].(map[string]interface{})
	if generationConfig["responseModalities"] != "IMAGE" {
		t.Fatalf("expected responseModalities, got %#v", generationConfig)
	}
	imageConfig := generationConfig["imageConfig"].(map[string]interface{})
	if imageConfig["aspectRatio"] != "1:1" || imageConfig["imageSize"] != "1K" {
		t.Fatalf("expected image config, got %#v", imageConfig)
	}
	if _, ok := generationConfig["responseFormat"]; ok {
		t.Fatalf("expected responseFormat to be filtered for Google image requests, got %#v", generationConfig)
	}
	if _, ok := generationConfig["temperature"]; ok {
		t.Fatalf("expected unlisted Gemini image option removed, got %#v", generationConfig)
	}
	tools := filtered["tools"].([]map[string]interface{})
	if len(tools) != 1 {
		t.Fatalf("expected one normalized google_search tool, got %#v", tools)
	}
	if tools[0]["type"] != "google_search" {
		t.Fatalf("expected google_search tool type, got %#v", tools)
	}
	if _, ok := tools[0]["google_search"]; !ok {
		t.Fatalf("expected google_search tool, got %#v", tools)
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

func TestFilterModelOptionsOpenAIImageEditsAllowsEditParams(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"background":         "transparent",
		"input_fidelity":     "high",
		"n":                  1,
		"output_compression": 80,
		"output_format":      "webp",
		"partial_images":     2,
		"quality":            "high",
		"size":               "1024x1024",
		"prompt":             "override",
		"stream":             true,
	}, llm.AdapterOpenAIImageEdits, modelOptionPolicyConfig{
		Mode:             modelOptionPolicyAllowlist,
		AllowedPathsJSON: config.DefaultModelOptionAllowedPathsJSON(),
		DeniedPathsJSON:  config.DefaultModelOptionDeniedPathsJSON(),
	})

	if filtered["background"] != "transparent" || filtered["input_fidelity"] != "high" {
		t.Fatalf("expected image edit params to pass, got %#v", filtered)
	}
	if filtered["partial_images"] != 2 || filtered["output_format"] != "webp" {
		t.Fatalf("expected image edit output params to pass, got %#v", filtered)
	}
	for _, key := range []string{"prompt", "stream"} {
		if _, ok := filtered[key]; ok {
			t.Fatalf("expected %s to be hard denied, got %#v", key, filtered)
		}
	}
}

func TestFilterModelOptionsXAIImageAllowsImageParams(t *testing.T) {
	filtered := filterModelOptions(map[string]interface{}{
		"aspect_ratio":    "16:9",
		"n":               2,
		"resolution":      "2K",
		"response_format": "b64_json",
		"prompt":          "override",
		"stream":          true,
		"quality":         "high",
	}, llm.AdapterXAIImage, modelOptionPolicyConfig{
		Mode:             modelOptionPolicyAllowlist,
		AllowedPathsJSON: config.DefaultModelOptionAllowedPathsJSON(),
		DeniedPathsJSON:  config.DefaultModelOptionDeniedPathsJSON(),
	})

	if filtered["aspect_ratio"] != "16:9" || filtered["resolution"] != "2K" || filtered["response_format"] != "b64_json" {
		t.Fatalf("expected xAI image params to pass, got %#v", filtered)
	}
	if filtered["n"] != 2 {
		t.Fatalf("expected xAI n param to pass, got %#v", filtered)
	}
	for _, key := range []string{"prompt", "stream", "quality"} {
		if _, ok := filtered[key]; ok {
			t.Fatalf("expected %s to be removed, got %#v", key, filtered)
		}
	}
}
