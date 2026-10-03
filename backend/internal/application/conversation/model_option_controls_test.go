package conversation

import (
	"fmt"
	"reflect"
	"testing"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

const controlsTestCapabilities = `{
	"defaultOptions": {"max_tokens": 64000, "text": {"format": "plain"}},
	"reasoning": {"format": "anthropic_effort", "levels": ["low", "high"], "default": "high"},
	"controls": [
		{"id": "verbosity", "type": "select", "default": "normal", "options": [
			{"value": "short", "patch": {"max_tokens": 1024, "text": {"verbosity": "low"}}},
			{"value": "normal"},
			{"value": "long", "patches": {"anthropic_messages": {"max_tokens": 128000}, "openai_responses": {"max_output_tokens": 128000}}}
		]},
		{"id": "fast", "type": "toggle", "on": {"patch": {"service_tier": "priority"}}, "off": {"patch": {"service_tier": null}}},
		{"id": "temperature", "type": "number", "path": "temperature", "min": 0, "max": 1, "step": 0.1},
		{"id": "pinned", "type": "select", "locked": true, "default": "a", "options": [
			{"value": "a", "patch": {"metadata": {"tier": "a"}}}, {"value": "b", "patch": {"metadata": {"tier": "b"}}}
		]}
	]
}`

func controlsPolicy(controls map[string]any) modelOptionPolicyConfig {
	policy := reasoningPolicy(controlsTestCapabilities)
	policy.Controls = controls
	return policy
}

func TestResolveChatModelOptionsAppliesControls(t *testing.T) {
	got, level := resolveChatModelOptions(nil, llm.AdapterAnthropicMessages, controlsPolicy(nil))
	want := map[string]any{
		"max_tokens":    64000,
		"text":          map[string]any{"format": "plain"},
		"metadata":      map[string]any{"tier": "a"},
		"output_config": map[string]any{"effort": "high"},
		"thinking":      map[string]any{"type": "adaptive"},
	}
	if level != "high" || !reflect.DeepEqual(normalizeTestNumbers(got), normalizeTestNumbers(want)) {
		t.Fatalf("unexpected defaults:\n got %#v (%q)\nwant %#v", got, level, want)
	}

	got, level = resolveChatModelOptions(nil, llm.AdapterAnthropicMessages, controlsPolicy(map[string]any{
		"verbosity":   "short",
		"fast":        true,
		"temperature": 0.3,
		"pinned":      "b",
		"reasoning":   "low",
	}))
	if level != "low" || got["max_tokens"] != int64(1024) || got["service_tier"] != "priority" || got["temperature"] != 0.3 {
		t.Fatalf("expected user controls applied, got %#v (%q)", got, level)
	}
	if text := got["text"].(map[string]any); text["format"] != "plain" || text["verbosity"] != "low" {
		t.Fatalf("expected patch to deep merge into defaults, got %#v", got["text"])
	}
	if got["metadata"].(map[string]any)["tier"] != "a" {
		t.Fatalf("expected locked control to keep its default, got %#v", got["metadata"])
	}

	got, _ = resolveChatModelOptions(nil, llm.AdapterAnthropicMessages, controlsPolicy(map[string]any{"verbosity": "long"}))
	if got["max_tokens"] != int64(128000) || got["max_output_tokens"] != nil {
		t.Fatalf("expected protocol specific patch, got %#v", got)
	}

	// 无效取值回退默认值。
	got, level = resolveChatModelOptions(nil, llm.AdapterAnthropicMessages, controlsPolicy(map[string]any{
		"verbosity":   "huge",
		"temperature": 5.0,
		"reasoning":   "turbo",
		"unknown":     "x",
	}))
	if fmt.Sprint(got["max_tokens"]) != "64000" || got["temperature"] != nil || level != "high" || got["unknown"] != nil {
		t.Fatalf("expected invalid selections to fall back to defaults, got %#v (%q)", got, level)
	}
}

func TestResolveChatModelOptionsCatalogAutoSendsNothing(t *testing.T) {
	policy := reasoningPolicy(`{}`)
	policy.CatalogReasoning = testCatalogCapability()
	policy.Controls = map[string]any{"reasoning": "auto"}
	if got, level := resolveChatModelOptions(nil, llm.AdapterAnthropicMessages, policy); got != nil || level != "" {
		t.Fatalf("expected auto to send nothing, got %#v (%q)", got, level)
	}
	policy.Controls = map[string]any{"reasoning": "high"}
	if got, level := resolveChatModelOptions(nil, llm.AdapterAnthropicMessages, policy); level != "high" || got["thinking"] == nil {
		t.Fatalf("expected selected catalog level to be translated, got %#v (%q)", got, level)
	}
}

func TestRestrictUserChatOptions(t *testing.T) {
	options := map[string]any{
		"temperature":      2,
		"tools":            []any{map[string]any{"type": "web_search"}},
		"reasoning_effort": "low",
	}
	restricted, controls := restrictUserChatOptions(options, nil, false)
	if !reflect.DeepEqual(restricted, map[string]any{"tools": options["tools"]}) {
		t.Fatalf("expected only tools to remain for users, got %#v", restricted)
	}
	if controls["reasoning"] != "low" {
		t.Fatalf("expected legacy reasoning_effort to become a control selection, got %#v", controls)
	}
	_, controls = restrictUserChatOptions(options, map[string]any{"reasoning": "high"}, false)
	if controls["reasoning"] != "high" {
		t.Fatalf("expected explicit control selection to win, got %#v", controls)
	}
	admin, _ := restrictUserChatOptions(options, nil, true)
	if !reflect.DeepEqual(admin, options) {
		t.Fatalf("expected admins to keep raw options, got %#v", admin)
	}
	if restricted, _ := restrictUserChatOptions(map[string]any{"temperature": 1}, nil, false); restricted != nil {
		t.Fatalf("expected nil options without tools, got %#v", restricted)
	}
}

func testCatalogCapability() *domainchannel.ReasoningCapability {
	return &domainchannel.ReasoningCapability{Format: "anthropic_effort", Levels: []string{"low", "high"}, Default: "high"}
}

func normalizeTestNumbers(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = normalizeTestNumbers(item)
		}
		return result
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	}
	return value
}

func TestResolveChatModelOptionsPatchedReasoningAndHiddenControls(t *testing.T) {
	capabilities := `{
		"reasoning": {"format": "openai", "levels": ["low", "high"], "default": "high"},
		"controls": [
			{"kind": "reasoning", "default": "high", "options": [
				{"value": "low", "patch": {"reasoning": {"effort": "low", "summary": "auto"}}},
				{"value": "high", "patch": {"reasoning": {"effort": "high", "summary": "detailed"}}}
			]},
			{"id": "internal", "type": "toggle", "hidden": true, "default": true, "on": {"patch": {"store": false}}}
		]
	}`
	policy := reasoningPolicy(capabilities)
	policy.Controls = map[string]any{"reasoning": "low", "internal": false}
	got, level := resolveChatModelOptions(nil, llm.AdapterOpenAIResponses, policy)
	want := map[string]any{"reasoning": map[string]any{"effort": "low", "summary": "auto"}, "store": false}
	if level != "low" || !reflect.DeepEqual(got, want) {
		t.Fatalf("expected patched reasoning and hidden default, got %#v (%q)", got, level)
	}
}
