package channel

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidateReasoningCapabilityFormatsAndAllowedLevels(t *testing.T) {
	allLevels := []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}
	allowed := map[string][]string{
		ReasoningFormatOpenAI:          allLevels,
		ReasoningFormatAnthropicEffort: {"none", "low", "medium", "high", "xhigh", "max"},
		ReasoningFormatAnthropicBudget: allLevels,
		ReasoningFormatGeminiLevel:     {"minimal", "low", "medium", "high"},
		ReasoningFormatGeminiBudget:    allLevels,
		ReasoningFormatQwen:            allLevels,
	}
	for format, levels := range allowed {
		allowedSet := map[string]bool{}
		for _, level := range levels {
			allowedSet[level] = true
		}
		for _, level := range allLevels {
			budgets := `{}`
			if format == ReasoningFormatAnthropicBudget || format == ReasoningFormatGeminiBudget {
				budgets = `{"` + level + `": 2048}`
			}
			raw := `{"reasoning":{"format":"` + format + `","levels":["` + level + `"],"default":"` + level + `","budgets":` + budgets + `}}`
			err := ValidateModelCapsOverrides(raw)
			if allowedSet[level] && err != nil {
				t.Fatalf("%s should allow %s, got %v", format, level, err)
			}
			if !allowedSet[level] && !errors.Is(err, ErrInvalidReasoningLevels) {
				t.Fatalf("%s should reject %s with invalid levels, got %v", format, level, err)
			}
		}
	}
}

func TestValidateReasoningCapabilityRejectsInvalidDeclarations(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want error
	}{
		{name: "unknown format", raw: `{"reasoning":{"format":"bogus","levels":["low"],"default":"low"}}`, want: ErrInvalidReasoningFormat},
		{name: "missing format", raw: `{"reasoning":{"levels":["low"],"default":"low"}}`, want: ErrInvalidReasoningFormat},
		{name: "non object", raw: `{"reasoning":"openai"}`, want: ErrInvalidReasoningFormat},
		{name: "empty levels", raw: `{"reasoning":{"format":"openai","levels":[],"default":"low"}}`, want: ErrInvalidReasoningLevels},
		{name: "duplicate levels", raw: `{"reasoning":{"format":"openai","levels":["low","low"],"default":"low"}}`, want: ErrInvalidReasoningLevels},
		{name: "non canonical level", raw: `{"reasoning":{"format":"openai","levels":["low","turbo"],"default":"low"}}`, want: ErrInvalidReasoningLevels},
		{name: "toggle one level", raw: `{"reasoning":{"format":"toggle","levels":["high"],"default":"high"}}`, want: ErrInvalidReasoningLevels},
		{name: "toggle without none", raw: `{"reasoning":{"format":"toggle","levels":["low","high"],"default":"high"}}`, want: ErrInvalidReasoningLevels},
		{name: "toggle three levels", raw: `{"reasoning":{"format":"toggle","levels":["none","low","high"],"default":"high"}}`, want: ErrInvalidReasoningLevels},
		{name: "default missing", raw: `{"reasoning":{"format":"openai","levels":["low","high"]}}`, want: ErrInvalidReasoningDefault},
		{name: "default not declared", raw: `{"reasoning":{"format":"openai","levels":["low","high"],"default":"medium"}}`, want: ErrInvalidReasoningDefault},
		{name: "anthropic budget missing", raw: `{"reasoning":{"format":"anthropic_budget","levels":["none","low","high"],"default":"high","budgets":{"low":2048}}}`, want: ErrInvalidReasoningBudgets},
		{name: "anthropic budget below minimum", raw: `{"reasoning":{"format":"anthropic_budget","levels":["low"],"default":"low","budgets":{"low":512}}}`, want: ErrInvalidReasoningBudgets},
		{name: "gemini budget missing", raw: `{"reasoning":{"format":"gemini_budget","levels":["low","high"],"default":"low","budgets":{"low":0}}}`, want: ErrInvalidReasoningBudgets},
		{name: "negative budget", raw: `{"reasoning":{"format":"qwen","levels":["low"],"default":"low","budgets":{"low":-1}}}`, want: ErrInvalidReasoningBudgets},
		{name: "fractional budget", raw: `{"reasoning":{"format":"gemini_budget","levels":["low"],"default":"low","budgets":{"low":1.5}}}`, want: ErrInvalidReasoningBudgets},
		{name: "string budget", raw: `{"reasoning":{"format":"gemini_budget","levels":["low"],"default":"low","budgets":{"low":"1024"}}}`, want: ErrInvalidReasoningBudgets},
		{name: "unknown budget key", raw: `{"reasoning":{"format":"qwen","levels":["low"],"default":"low","budgets":{"turbo":1}}}`, want: ErrInvalidReasoningBudgets},
		{name: "budgets not object", raw: `{"reasoning":{"format":"qwen","levels":["low"],"default":"low","budgets":[1]}}`, want: ErrInvalidReasoningBudgets},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateModelCapsOverrides(tt.raw); !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}

func TestValidateReasoningCapabilityAcceptsValidDeclarations(t *testing.T) {
	for _, raw := range []string{
		`{}`,
		`{"reasoning":null}`,
		`{"reasoning":{"format":"openai","levels":["none","minimal","low","medium","high","xhigh"],"default":"high"}}`,
		`{"reasoning":{"format":"openai","levels":["high","low"],"default":"low","budgets":{"low":"ignored"}}}`,
		`{"reasoning":{"format":"anthropic_budget","levels":["none","low","high"],"default":"high","budgets":{"low":1024,"high":16000}}}`,
		`{"reasoning":{"format":"gemini_budget","levels":["none","low"],"default":"low","budgets":{"low":0}}}`,
		`{"reasoning":{"format":"qwen","levels":["none","high"],"default":"high"}}`,
		`{"reasoning":{"format":"toggle","levels":["high","none"],"default":"none"}}`,
	} {
		if err := ValidateModelCapsOverrides(raw); err != nil {
			t.Fatalf("expected %s to validate, got %v", raw, err)
		}
	}
}

// resolveWithoutCatalog 与 resolveForProtocolsWithoutCatalog 只关心能力本身，不传目录能力。
func resolveWithoutCatalog(protocolKey string, capabilitiesJSON string) *ReasoningCapability {
	capability, _ := ResolveReasoningCapability(protocolKey, capabilitiesJSON, nil)
	return capability
}

func resolveForProtocolsWithoutCatalog(protocolKeys []string, capabilitiesJSON string) *ReasoningCapability {
	capability, _ := ResolveReasoningCapabilityForProtocols(protocolKeys, capabilitiesJSON, nil)
	return capability
}

func TestResolveReasoningCapabilityNormalizesExplicitDeclaration(t *testing.T) {
	capability := resolveWithoutCatalog(ReasoningProtocolAnthropic, `{
		"reasoning": {"format": "anthropic_budget", "levels": ["high", "none", "low"], "default": "HIGH", "budgets": {"low": 2048, "high": 16000}}
	}`)
	if capability == nil {
		t.Fatal("expected explicit capability")
	}
	want := &ReasoningCapability{
		Format:  ReasoningFormatAnthropicBudget,
		Levels:  []string{"none", "low", "high"},
		Default: "high",
		Budgets: map[string]int{"low": 2048, "high": 16000},
	}
	if !reflect.DeepEqual(capability, want) {
		t.Fatalf("unexpected capability: %#v", capability)
	}
}

func TestResolveReasoningCapabilityExplicitRequiresCompatibleProtocol(t *testing.T) {
	raw := `{"reasoning":{"format":"gemini_level","levels":["low","high"],"default":"high"},
		"optionControls":[{"path":"reasoning_effort","type":"select","options":["low","high"]}]}`
	if capability := resolveWithoutCatalog(ReasoningProtocolOpenAIChat, raw); capability != nil {
		t.Fatalf("expected incompatible explicit capability to disable translation (no inference fallback), got %#v", capability)
	}
	if capability := resolveWithoutCatalog(ReasoningProtocolGeminiInteractions, raw); capability == nil || capability.Format != ReasoningFormatGeminiLevel {
		t.Fatalf("expected gemini_level on interactions, got %#v", capability)
	}
	if capability := resolveWithoutCatalog(ReasoningProtocolOpenAIChat, `{"reasoning":{"format":"bogus"}}`); capability != nil {
		t.Fatalf("expected invalid explicit capability to be ignored, got %#v", capability)
	}
}

func TestReasoningFormatSupportsProtocol(t *testing.T) {
	protocols := []string{
		ReasoningProtocolOpenAIChat, ReasoningProtocolOpenRouterChat, ReasoningProtocolOpenAIResponses,
		ReasoningProtocolXAIResponses, ReasoningProtocolOpenRouterResponses, ReasoningProtocolAnthropic,
		ReasoningProtocolGeminiGenerate, ReasoningProtocolGeminiInteractions, "openai_image_generations",
	}
	supported := map[string]map[string]bool{
		ReasoningFormatOpenAI: {
			ReasoningProtocolOpenAIChat: true, ReasoningProtocolOpenRouterChat: true, ReasoningProtocolOpenAIResponses: true,
			ReasoningProtocolXAIResponses: true, ReasoningProtocolOpenRouterResponses: true,
		},
		ReasoningFormatAnthropicEffort: {ReasoningProtocolAnthropic: true},
		ReasoningFormatAnthropicBudget: {ReasoningProtocolAnthropic: true},
		ReasoningFormatGeminiLevel:     {ReasoningProtocolGeminiGenerate: true, ReasoningProtocolGeminiInteractions: true},
		ReasoningFormatGeminiBudget:    {ReasoningProtocolGeminiGenerate: true},
		ReasoningFormatQwen:            {ReasoningProtocolOpenAIChat: true, ReasoningProtocolOpenRouterChat: true},
		ReasoningFormatToggle:          {ReasoningProtocolOpenAIChat: true, ReasoningProtocolOpenRouterChat: true},
	}
	for format, expected := range supported {
		for _, protocol := range protocols {
			if got := ReasoningFormatSupportsProtocol(format, protocol); got != expected[protocol] {
				t.Fatalf("%s on %s: expected %v, got %v", format, protocol, expected[protocol], got)
			}
		}
	}
}

func TestInferReasoningCapabilityPerProtocol(t *testing.T) {
	tests := []struct {
		protocol string
		path     string
		format   string
		defaults string
	}{
		{ReasoningProtocolOpenAIChat, "reasoning_effort", ReasoningFormatOpenAI, `{"reasoning_effort":"high"}`},
		{ReasoningProtocolOpenRouterChat, "reasoning_effort", ReasoningFormatOpenAI, `{"reasoning_effort":"high"}`},
		{ReasoningProtocolOpenAIResponses, "reasoning.effort", ReasoningFormatOpenAI, `{"reasoning":{"effort":"high"}}`},
		{ReasoningProtocolXAIResponses, "reasoning.effort", ReasoningFormatOpenAI, `{"reasoning":{"effort":"high"}}`},
		{ReasoningProtocolOpenRouterResponses, "reasoning.effort", ReasoningFormatOpenAI, `{"reasoning":{"effort":"high"}}`},
		{ReasoningProtocolAnthropic, "output_config.effort", ReasoningFormatAnthropicEffort, `{"output_config":{"effort":"high"}}`},
		{ReasoningProtocolGeminiGenerate, "generationConfig.thinkingConfig.thinkingLevel", ReasoningFormatGeminiLevel, `{"generationConfig":{"thinkingConfig":{"thinkingLevel":"high"}}}`},
		{ReasoningProtocolGeminiInteractions, "generation_config.thinking_level", ReasoningFormatGeminiLevel, `{"generation_config":{"thinking_level":"high"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			raw := `{"defaultOptions":` + tt.defaults + `,"optionControls":[
				{"path":"` + tt.path + `","type":"select","options":["high","low","medium","turbo"]}
			]}`
			capability := resolveWithoutCatalog(tt.protocol, raw)
			if capability == nil {
				t.Fatal("expected inferred capability")
			}
			if capability.Format != tt.format || capability.ControlPath != tt.path || capability.Default != "high" {
				t.Fatalf("unexpected inferred capability: %#v", capability)
			}
			if !reflect.DeepEqual(capability.Levels, []string{"low", "medium", "high"}) {
				t.Fatalf("expected canonical ordered levels without non-canonical values, got %#v", capability.Levels)
			}
		})
	}
}

func TestInferReasoningCapabilityRequiresKnownPathSelectAndCanonicalLevels(t *testing.T) {
	cases := map[string]string{
		"wrong protocol path": `{"optionControls":[{"path":"reasoning.effort","type":"select","options":["low"]}]}`,
		"not select":          `{"optionControls":[{"path":"reasoning_effort","type":"text","options":["low"]}]}`,
		"no canonical":        `{"optionControls":[{"path":"reasoning_effort","type":"select","options":["fast","slow"]}]}`,
		"no reasoning params": `{"defaultOptions":{"temperature":0.2,"thinking":{"display":"summarized"}}}`,
	}
	for name, raw := range cases {
		if capability := resolveWithoutCatalog(ReasoningProtocolOpenAIChat, raw); capability != nil {
			t.Fatalf("%s: expected no inference, got %#v", name, capability)
		}
	}
	if capability := resolveWithoutCatalog("openai_image_generations", `{"optionControls":[{"path":"reasoning_effort","type":"select","options":["low"]}]}`); capability != nil {
		t.Fatalf("expected media protocol to never infer, got %#v", capability)
	}
	// 格式不允许的档位（Gemini 没有 none/xhigh）不参与推断。
	capability := resolveWithoutCatalog(ReasoningProtocolGeminiGenerate, `{"optionControls":[
		{"path":"generationConfig.thinkingConfig.thinkingLevel","type":"select","options":["none","low","high","xhigh"]}
	]}`)
	if capability == nil || !reflect.DeepEqual(capability.Levels, []string{"low", "high"}) {
		t.Fatalf("expected format-filtered gemini levels, got %#v", capability)
	}
}

func TestInferReasoningCapabilityDefaultFallbacks(t *testing.T) {
	tests := []struct {
		name     string
		options  string
		defaults string
		want     string
	}{
		{name: "default option wins", options: `["low","medium","high"]`, defaults: `{"reasoning_effort":"low"}`, want: "low"},
		{name: "undeclared default option ignored", options: `["low","medium","high"]`, defaults: `{"reasoning_effort":"xhigh"}`, want: "medium"},
		{name: "non canonical default ignored", options: `["low","medium","high"]`, defaults: `{"reasoning_effort":"auto"}`, want: "medium"},
		{name: "highest at or below medium", options: `["none","low","high"]`, defaults: `{}`, want: "low"},
		{name: "first level when all above medium", options: `["xhigh","high","max"]`, defaults: `{}`, want: "high"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := `{"defaultOptions":` + tt.defaults + `,"optionControls":[{"path":"reasoning_effort","type":"select","options":` + tt.options + `}]}`
			capability := resolveWithoutCatalog(ReasoningProtocolOpenAIChat, raw)
			if capability == nil || capability.Default != tt.want {
				t.Fatalf("expected default %s, got %#v", tt.want, capability)
			}
		})
	}
}

func TestReasoningCapabilityResolveLevel(t *testing.T) {
	capability := ReasoningCapability{Levels: []string{"low", "high", "max"}, Default: "high"}
	tests := map[string]string{
		"":        "high",
		"turbo":   "high",
		"HIGH":    "high",
		"low":     "low",
		"none":    "low",
		"minimal": "low",
		"medium":  "low", // 与 low、high 等距时取较低档
		"xhigh":   "high",
		"max":     "max",
	}
	for requested, want := range tests {
		if got := capability.ResolveLevel(requested); got != want {
			t.Fatalf("ResolveLevel(%q) = %q, want %q", requested, got, want)
		}
	}
}

func TestResolveReasoningCapabilityForProtocols(t *testing.T) {
	inferred := `{"optionControls":[{"path":"output_config.effort","type":"select","options":["low","high"]}]}`
	capability := resolveForProtocolsWithoutCatalog([]string{ReasoningProtocolOpenAIChat, ReasoningProtocolAnthropic}, inferred)
	if capability == nil || capability.ControlPath != "output_config.effort" {
		t.Fatalf("expected inference from the matching protocol, got %#v", capability)
	}
	if capability := resolveForProtocolsWithoutCatalog(nil, inferred); capability != nil {
		t.Fatalf("expected no inference without protocols, got %#v", capability)
	}
	explicit := `{"reasoning":{"format":"anthropic_effort","levels":["low","high"],"default":"high"}}`
	if capability := resolveForProtocolsWithoutCatalog(nil, explicit); capability == nil || capability.ControlPath != "" {
		t.Fatalf("expected explicit capability without routes, got %#v", capability)
	}
	if capability := resolveForProtocolsWithoutCatalog([]string{ReasoningProtocolOpenAIChat}, explicit); capability != nil {
		t.Fatalf("expected incompatible explicit capability to be hidden, got %#v", capability)
	}
}

func TestReasoningNativeLevelPath(t *testing.T) {
	tests := []struct {
		format   string
		protocol string
		want     string
	}{
		{ReasoningFormatOpenAI, ReasoningProtocolOpenAIChat, "reasoning_effort"},
		{ReasoningFormatOpenAI, ReasoningProtocolXAIResponses, "reasoning.effort"},
		{ReasoningFormatAnthropicEffort, ReasoningProtocolAnthropic, "output_config.effort"},
		{ReasoningFormatGeminiLevel, ReasoningProtocolGeminiInteractions, "generation_config.thinking_level"},
		{ReasoningFormatAnthropicBudget, ReasoningProtocolAnthropic, ""},
		{ReasoningFormatQwen, ReasoningProtocolOpenAIChat, ""},
		{ReasoningFormatOpenAI, ReasoningProtocolAnthropic, ""},
	}
	for _, tt := range tests {
		if got := ReasoningNativeLevelPath(tt.format, tt.protocol); got != tt.want {
			t.Fatalf("ReasoningNativeLevelPath(%s, %s) = %q, want %q", tt.format, tt.protocol, got, tt.want)
		}
	}
}
