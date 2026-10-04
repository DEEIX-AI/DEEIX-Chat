package channel

import (
	"strings"
	"testing"
)

func TestReasoningCustomizationTemplate(t *testing.T) {
	toggle := &ReasoningCapability{Format: ReasoningFormatToggle, Levels: []string{ReasoningLevelNone, ReasoningLevelHigh}, Default: ReasoningLevelHigh}
	template := ReasoningCustomizationTemplate([]string{ReasoningProtocolOpenAIChat}, toggle, nil, 0, 0)
	if template == nil || template.Format != ReasoningFormatToggle || strings.Join(template.Levels, ",") != "none,high" || template.Budgets != nil {
		t.Fatalf("unexpected toggle template: %#v", template)
	}

	// 生效能力与协议不兼容时改用目录建议，再退回协议的档位格式；建议预算钳制到目录声明的上限。
	catalog := &ReasoningCapability{Format: ReasoningFormatGeminiBudget, Levels: []string{ReasoningLevelLow}, Default: ReasoningLevelLow, Budgets: map[string]int{ReasoningLevelLow: 512}}
	template = ReasoningCustomizationTemplate([]string{ReasoningProtocolGeminiGenerate}, toggle, catalog, 0, 32768)
	if template == nil || template.Format != ReasoningFormatGeminiBudget || template.Budgets[ReasoningLevelLow] != 512 ||
		template.Budgets[ReasoningLevelHigh] != 24576 || template.Budgets[ReasoningLevelMax] != 32768 || template.Budgets[ReasoningLevelNone] != 0 {
		t.Fatalf("unexpected catalog template: %#v", template)
	}
	if template := ReasoningCustomizationTemplate([]string{ReasoningProtocolGeminiGenerate}, nil, catalog, 128, 32768); template == nil || template.Levels[0] == ReasoningLevelNone {
		t.Fatalf("gemini_budget with a positive minimum must not offer none: %#v", template)
	}
	template = ReasoningCustomizationTemplate([]string{ReasoningProtocolGeminiInteractions}, nil, nil, 0, 0)
	if template == nil || template.Format != ReasoningFormatGeminiLevel || strings.Join(template.Levels, ",") != "minimal,low,medium,high" {
		t.Fatalf("unexpected protocol template: %#v", template)
	}
	if ReasoningCustomizationTemplate(nil, nil, nil, 0, 0) != nil {
		t.Fatal("expected no template without protocols or capability")
	}
}

func TestWithoutExplicitReasoning(t *testing.T) {
	if got := WithoutExplicitReasoning(`{"reasoning":{"format":"openai"},"contextWindow":200000}`); got != `{"contextWindow":200000}` {
		t.Fatalf("unexpected result: %s", got)
	}
	if got := WithoutExplicitReasoning(`{"contextWindow":1}`); got != `{"contextWindow":1}` {
		t.Fatalf("expected JSON without reasoning to stay unchanged, got %s", got)
	}
	if got := WithoutExplicitReasoning(`not json`); got != "" {
		t.Fatalf("expected invalid JSON to be treated as empty, got %q", got)
	}
}
