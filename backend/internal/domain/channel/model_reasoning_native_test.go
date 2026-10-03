package channel

import (
	"reflect"
	"testing"
)

func TestInferReasoningCapabilityFromAdminNativeDefaults(t *testing.T) {
	tests := []struct {
		name         string
		protocol     string
		capabilities string
		catalog      *ReasoningCapability
		want         *ReasoningCapability
	}{
		{
			name:         "openai chat effort default",
			protocol:     ReasoningProtocolOpenAIChat,
			capabilities: `{"defaultOptions":{"reasoning_effort":"high"}}`,
			want:         &ReasoningCapability{Format: ReasoningFormatOpenAI, Levels: []string{"low", "medium", "high"}, Default: "high", ControlPath: "reasoning_effort"},
		},
		{
			name:         "responses effort uses catalog levels",
			protocol:     ReasoningProtocolOpenAIResponses,
			capabilities: `{"defaultOptions":{"reasoning":{"effort":"minimal","summary":"auto"}}}`,
			catalog:      &ReasoningCapability{Format: ReasoningFormatOpenAI, Levels: []string{"minimal", "low", "medium", "high"}, Default: "medium"},
			want:         &ReasoningCapability{Format: ReasoningFormatOpenAI, Levels: []string{"minimal", "low", "medium", "high"}, Default: "minimal", ControlPath: "reasoning.effort"},
		},
		{
			name:         "anthropic budget snaps to nearest level and keeps admin budget",
			protocol:     ReasoningProtocolAnthropic,
			capabilities: `{"defaultOptions":{"thinking":{"type":"enabled","budget_tokens":10000}}}`,
			want: &ReasoningCapability{Format: ReasoningFormatAnthropicBudget, Levels: []string{"low", "medium", "high"}, Default: "medium",
				Budgets: map[string]int{"low": 2048, "medium": 10000, "high": 24576}},
		},
		{
			name:         "anthropic disabled follows catalog format",
			protocol:     ReasoningProtocolAnthropic,
			capabilities: `{"defaultOptions":{"thinking":{"type":"disabled"}}}`,
			catalog:      &ReasoningCapability{Format: ReasoningFormatAnthropicEffort, Levels: []string{"low", "high", "max"}, Default: "high"},
			want:         &ReasoningCapability{Format: ReasoningFormatAnthropicEffort, Levels: []string{"none", "low", "high", "max"}, Default: "none"},
		},
		{
			name:         "qwen enable_thinking with budget",
			protocol:     ReasoningProtocolOpenAIChat,
			capabilities: `{"defaultOptions":{"enable_thinking":true,"thinking_budget":2000}}`,
			want: &ReasoningCapability{Format: ReasoningFormatQwen, Levels: []string{"none", "low", "medium", "high"}, Default: "low",
				Budgets: map[string]int{"none": 0, "low": 2000, "medium": 8192, "high": 24576}},
		},
		{
			name:         "openai compatible thinking toggle",
			protocol:     ReasoningProtocolOpenAIChat,
			capabilities: `{"defaultOptions":{"thinking":{"type":"disabled"}}}`,
			want:         &ReasoningCapability{Format: ReasoningFormatToggle, Levels: []string{"none", "high"}, Default: "none"},
		},
		{
			name:         "gemini thinking budget zero",
			protocol:     ReasoningProtocolGeminiGenerate,
			capabilities: `{"defaultOptions":{"generationConfig":{"thinkingConfig":{"thinkingBudget":0}}}}`,
			want: &ReasoningCapability{Format: ReasoningFormatGeminiBudget, Levels: []string{"none", "low", "medium", "high"}, Default: "none",
				Budgets: map[string]int{"none": 0, "low": 2048, "medium": 8192, "high": 24576}},
		},
		{
			name:         "gemini thinking level alias",
			protocol:     ReasoningProtocolGeminiGenerate,
			capabilities: `{"defaultOptions":{"thinkingConfig":{"thinkingLevel":"low"}}}`,
			want:         &ReasoningCapability{Format: ReasoningFormatGeminiLevel, Levels: []string{"low", "high"}, Default: "low", ControlPath: "generationConfig.thinkingConfig.thinkingLevel"},
		},
		{
			name:         "native budget control without default",
			protocol:     ReasoningProtocolAnthropic,
			capabilities: `{"optionControls":[{"path":"thinking.budget_tokens","type":"number"}]}`,
			want: &ReasoningCapability{Format: ReasoningFormatAnthropicBudget, Levels: []string{"low", "medium", "high"}, Default: "medium",
				Budgets: map[string]int{"low": 2048, "medium": 8192, "high": 24576}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, source := ResolveReasoningCapability(tt.protocol, tt.capabilities, tt.catalog)
			if source != ReasoningSourceInferred || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v (%s)\nwant %#v", got, source, tt.want)
			}
		})
	}
}

func TestAdminNativeDefaultOverridesDeclaredDefault(t *testing.T) {
	raw := `{"reasoning":{"format":"openai","levels":["low","medium","high"],"default":"medium"},
		"defaultOptions":{"reasoning":{"effort":"high"}}}`
	capability, source := ResolveReasoningCapability(ReasoningProtocolOpenAIResponses, raw, nil)
	if source != ReasoningSourceExplicit || capability == nil || capability.Default != "high" {
		t.Fatalf("expected admin native default to win over declared default, got %#v (%s)", capability, source)
	}
	capability, _ = ResolveReasoningCapabilityForProtocols([]string{ReasoningProtocolOpenAIResponses}, raw, nil)
	if capability == nil || capability.Default != "high" {
		t.Fatalf("expected the catalog view to agree, got %#v", capability)
	}
}

func TestDetectReasoningNativeSignal(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		options  map[string]any
		want     ReasoningNativeSignal
		found    bool
	}{
		{name: "none", protocol: ReasoningProtocolAnthropic, options: map[string]any{"thinking": map[string]any{"display": "summarized"}}},
		{name: "anthropic effort alias", protocol: ReasoningProtocolAnthropic, options: map[string]any{"effort": "MAX"},
			want: ReasoningNativeSignal{Format: ReasoningFormatAnthropicEffort, Level: "max"}, found: true},
		{name: "anthropic enable_thinking false", protocol: ReasoningProtocolAnthropic, options: map[string]any{"enable_thinking": false},
			want: ReasoningNativeSignal{Format: ReasoningFormatAnthropicBudget, Level: "none", formatFlexible: true}, found: true},
		{name: "responses ignores chat key", protocol: ReasoningProtocolOpenAIResponses, options: map[string]any{"reasoning_effort": "high"}},
		{name: "gemini dynamic budget", protocol: ReasoningProtocolGeminiGenerate, options: map[string]any{"thinking_budget": float64(-1)},
			want: ReasoningNativeSignal{Format: ReasoningFormatGeminiBudget, Budget: -1, HasBudget: true}, found: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := DetectReasoningNativeSignal(tt.protocol, tt.options)
			if found != tt.found || got != tt.want {
				t.Fatalf("got %#v (%v), want %#v (%v)", got, found, tt.want, tt.found)
			}
		})
	}
}
