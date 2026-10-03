package channel

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidateModelControls(t *testing.T) {
	valid := `{"controls":[
		{"id":"verbosity","type":"select","label":"回复长度","icon":"text","placement":"toolbar","default":"low",
			"options":[{"value":"low","patch":{"text":{"verbosity":"low"}}},{"value":"high","patches":{"openai_chat_completions":{"verbosity":"high"}}}]},
		{"id":"web","type":"toggle","default":true,"on":{"patch":{"web_search_options":{}}},"off":{"patch":{"web_search_options":null}}},
		{"id":"temperature","type":"number","path":"temperature","min":0,"max":2,"step":0.1,"default":1},
		{"kind":"reasoning","placement":"menu","icon":"sparkles"}
	]}`
	if err := ValidateModelCapsOverrides(valid); err != nil {
		t.Fatalf("expected valid controls, got %v", err)
	}
	invalid := map[string]string{
		"not array":        `{"controls":{}}`,
		"bad id":           `{"controls":[{"id":" ","type":"toggle"}]}`,
		"duplicate id":     `{"controls":[{"id":"a","type":"toggle"},{"id":"a","type":"toggle"}]}`,
		"unknown type":     `{"controls":[{"id":"a","type":"text"}]}`,
		"empty options":    `{"controls":[{"id":"a","type":"select","options":[]}]}`,
		"unknown default":  `{"controls":[{"id":"a","type":"select","default":"x","options":[{"value":"y"}]}]}`,
		"number range":     `{"controls":[{"id":"a","type":"number","path":"temperature","min":2,"max":1}]}`,
		"number default":   `{"controls":[{"id":"a","type":"number","path":"temperature","max":1,"default":3}]}`,
		"bad placement":    `{"controls":[{"id":"a","type":"toggle","placement":"sidebar"}]}`,
		"bad icon":         `{"controls":[{"id":"a","type":"toggle","icon":"Not An Icon"}]}`,
		"reasoning levels": `{"controls":[{"kind":"reasoning","options":[{"value":"turbo"}]}]}`,
	}
	for name, raw := range invalid {
		if err := ValidateModelCapsOverrides(raw); !errors.Is(err, ErrInvalidModelControls) {
			t.Fatalf("%s: expected invalid controls, got %v", name, err)
		}
	}
	forbidden := map[string]string{
		"patch stream":   `{"controls":[{"id":"a","type":"toggle","on":{"patch":{"stream":false}}}]}`,
		"patches model":  `{"controls":[{"id":"a","type":"select","options":[{"value":"x","patches":{"anthropic_messages":{"model":"x"}}}]}]}`,
		"number tools":   `{"controls":[{"id":"a","type":"number","path":"tools.0"}]}`,
		"patch api key":  `{"controls":[{"id":"a","type":"select","options":[{"value":"x","patch":{"api_key":"k"}}]}]}`,
		"patch messages": `{"controls":[{"id":"a","type":"toggle","off":{"patch":{"messages":[]}}}]}`,
	}
	for name, raw := range forbidden {
		if err := ValidateModelCapsOverrides(raw); !errors.Is(err, ErrModelControlForbiddenPath) {
			t.Fatalf("%s: expected forbidden path, got %v", name, err)
		}
	}
	if err := ValidateModelCapsOverrides(`{"controls":[{"id":"a","type":"toggle","on":{"patch":{"stream":false}}}]}`); !errors.Is(err, ErrModelControlForbiddenPath) {
		t.Fatalf("expected capability validation to include controls, got %v", err)
	}
}

func TestModelControlNormalizeValue(t *testing.T) {
	min, max, step := 0.0, 2.0, 0.5
	number := ModelControl{Type: ModelControlTypeNumber, Min: &min, Max: &max, Step: &step}
	for raw, want := range map[any]string{1.5: "1.5", "2": "2", float64(0): "0"} {
		if got, ok := number.NormalizeValue(raw); !ok || got != want {
			t.Fatalf("NormalizeValue(%v) = %q, %v", raw, got, ok)
		}
	}
	for _, raw := range []any{1.2, 3.0, "x", true} {
		if _, ok := number.NormalizeValue(raw); ok {
			t.Fatalf("expected %v to be rejected", raw)
		}
	}
	toggle := ModelControl{Type: ModelControlTypeToggle, Options: []ModelControlOption{{Value: "off"}, {Value: "on"}}}
	if got, ok := toggle.NormalizeValue(true); !ok || got != ModelControlToggleOn {
		t.Fatalf("toggle true = %q, %v", got, ok)
	}
	selectControl := ModelControl{Type: ModelControlTypeSelect, Options: []ModelControlOption{{Value: "a"}}}
	if _, ok := selectControl.NormalizeValue("b"); ok {
		t.Fatal("expected undeclared select value to be rejected")
	}
}

func TestResolveModelControlsOrderAndSources(t *testing.T) {
	raw := `{
		"defaultOptions": {"temperature": 0.7, "text": {"verbosity": "medium"}, "parallel_tool_calls": true, "reasoning": {"effort": "high"}},
		"lockedOptionPaths": ["parallel_tool_calls"],
		"optionControls": [
			{"path": "temperature", "type": "number", "label": "Temperature"},
			{"path": "text.verbosity", "type": "select", "options": ["low", "medium", "high"]},
			{"path": "parallel_tool_calls", "type": "boolean"},
			{"path": "reasoning.effort", "type": "select", "options": ["low", "high"]},
			{"path": "metadata", "type": "json"}
		],
		"controls": [
			{"id": "verbosity-fast", "type": "toggle", "placement": "toolbar", "on": {"patch": {"service_tier": "priority"}}},
			{"kind": "reasoning", "placement": "menu", "label": "Thinking"},
			{"id": "claude-only", "type": "toggle", "protocols": ["anthropic_messages"]}
		]
	}`
	reasoning := &ReasoningCapability{Format: ReasoningFormatOpenAI, Levels: []string{"low", "high"}, Default: "high", ControlPath: "reasoning.effort"}
	controls := ResolveModelControls(raw, []string{ReasoningProtocolOpenAIResponses}, reasoning, ReasoningSourceInferred)
	ids := make([]string, 0, len(controls))
	for _, control := range controls {
		ids = append(ids, control.ID)
	}
	if want := []string{"verbosity-fast", "reasoning", "temperature", "text.verbosity", "parallel_tool_calls"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("unexpected control order %v, want %v", ids, want)
	}
	byID := map[string]ModelControl{}
	for _, control := range controls {
		byID[control.ID] = control
	}
	if control := byID["reasoning"]; control.Label != "Thinking" || control.Placement != ModelControlPlacementMenu || control.Default != "high" || len(control.Options) != 2 {
		t.Fatalf("unexpected reasoning control %#v", control)
	}
	if control := byID["temperature"]; control.Type != ModelControlTypeNumber || control.Path != "temperature" || control.Default != "0.7" {
		t.Fatalf("unexpected number control %#v", control)
	}
	if control := byID["text.verbosity"]; control.Default != "medium" || !reflect.DeepEqual(control.Options[0].Patch, map[string]any{"text": map[string]any{"verbosity": "low"}}) {
		t.Fatalf("unexpected legacy select control %#v", control)
	}
	if control := byID["parallel_tool_calls"]; !control.Locked || control.Type != ModelControlTypeToggle || control.Default != ModelControlToggleOn {
		t.Fatalf("unexpected locked toggle %#v", control)
	}
}

func TestResolveModelControlsCatalogReasoningHasAuto(t *testing.T) {
	catalog := &ReasoningCapability{Format: ReasoningFormatOpenAI, Levels: []string{"low", "high"}, Default: "low"}
	controls := ResolveModelControls(`{}`, []string{ReasoningProtocolOpenAIChat}, catalog, ReasoningSourceCatalog)
	if len(controls) != 1 || controls[0].Default != ModelControlReasoningAuto || controls[0].Options[0].Value != ModelControlReasoningAuto {
		t.Fatalf("expected catalog reasoning control with auto default, got %#v", controls)
	}
	locked := ResolveModelControls(`{"lockedOptionPaths":["reasoning_effort"]}`, []string{ReasoningProtocolOpenAIChat}, catalog, ReasoningSourceCatalog)
	if !locked[0].Locked || locked[0].Default != "low" {
		t.Fatalf("expected locked catalog reasoning to pin its default, got %#v", locked[0])
	}
	if controls := ResolveModelControls(`{"controls":[{"kind":"reasoning"}]}`, nil, nil, ""); len(controls) != 0 {
		t.Fatalf("expected presentation override without capability to be dropped, got %#v", controls)
	}
}

// 思考强度默认固定到输入框工具栏：显式条目未写 placement 时不应把它挤到「…」菜单。
func TestResolveModelControlsReasoningDefaultsToToolbar(t *testing.T) {
	capability := &ReasoningCapability{Format: ReasoningFormatOpenAI, Levels: []string{"low", "high"}, Default: "high"}
	cases := map[string]string{
		"no explicit entry":                  `{}`,
		"presentation override":              `{"controls":[{"kind":"reasoning","label":"Thinking"}]}`,
		"explicit control without placement": `{"controls":[{"kind":"reasoning","options":[{"value":"low"},{"value":"high"}],"default":"high"}]}`,
	}
	for name, raw := range cases {
		controls := ResolveModelControls(raw, []string{ReasoningProtocolOpenAIChat}, capability, ReasoningSourceExplicit)
		if len(controls) != 1 || controls[0].ID != ModelControlReasoningID || controls[0].Placement != ModelControlPlacementToolbar {
			t.Fatalf("%s: expected toolbar placement, got %#v", name, controls)
		}
	}
	menu := ResolveModelControls(`{"controls":[{"kind":"reasoning","placement":"menu"}]}`, []string{ReasoningProtocolOpenAIChat}, capability, ReasoningSourceExplicit)
	if len(menu) != 1 || menu[0].Placement != ModelControlPlacementMenu {
		t.Fatalf("expected an explicit menu placement to win, got %#v", menu)
	}
}
