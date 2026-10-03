package conversation

import (
	"fmt"
	"reflect"
	"testing"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func reasoningPolicy(capabilities string) modelOptionPolicyConfig {
	return modelOptionPolicyConfig{
		Mode:                  modelOptionPolicyAllowlist,
		AllowedPathsJSON:      config.DefaultModelOptionAllowedPathsJSON(),
		DeniedPathsJSON:       config.DefaultModelOptionDeniedPathsJSON(),
		ModelCapabilitiesJSON: capabilities,
	}
}

func explicitReasoning(format string, levels string, defaultLevel string, budgets string) string {
	if budgets == "" {
		budgets = "{}"
	}
	return `{"reasoning":{"format":"` + format + `","levels":` + levels + `,"default":"` + defaultLevel + `","budgets":` + budgets + `}}`
}

func TestResolveChatModelOptionsTranslationTable(t *testing.T) {
	allLevels := `["none","minimal","low","medium","high","xhigh","max"]`
	tests := []struct {
		name         string
		protocol     string
		capabilities string
		options      map[string]any
		wantLevel    string
		want         map[string]any
	}{
		{
			name:         "openai chat none",
			protocol:     llm.AdapterOpenAIChatCompletions,
			capabilities: explicitReasoning("openai", allLevels, "high", ""),
			options:      map[string]any{"reasoning_effort": "none"},
			wantLevel:    "none",
			want:         map[string]any{"reasoning_effort": "none"},
		},
		{
			name:         "openai chat level",
			protocol:     llm.AdapterOpenAIChatCompletions,
			capabilities: explicitReasoning("openai", allLevels, "high", ""),
			options:      map[string]any{"reasoning_effort": "xhigh"},
			wantLevel:    "xhigh",
			want:         map[string]any{"reasoning_effort": "xhigh"},
		},
		{
			name:         "openrouter chat drops conflicting unified reasoning effort",
			protocol:     llm.AdapterOpenRouterChat,
			capabilities: explicitReasoning("openai", allLevels, "high", ""),
			options:      map[string]any{"reasoning_effort": "low", "reasoning": map[string]any{"effort": "high"}},
			wantLevel:    "low",
			want:         map[string]any{"reasoning_effort": "low"},
		},
		{
			name:         "openai responses none keeps summary",
			protocol:     llm.AdapterOpenAIResponses,
			capabilities: explicitReasoning("openai", allLevels, "high", ""),
			options:      map[string]any{"reasoning_effort": "none", "reasoning": map[string]any{"summary": "auto", "effort": "high"}},
			wantLevel:    "none",
			want:         map[string]any{"reasoning": map[string]any{"effort": "none", "summary": "auto"}},
		},
		{
			name:         "xai responses level",
			protocol:     llm.AdapterXAIResponses,
			capabilities: explicitReasoning("openai", `["none","low","medium","high"]`, "low", ""),
			options:      map[string]any{"reasoning_effort": "high"},
			wantLevel:    "high",
			want:         map[string]any{"reasoning": map[string]any{"effort": "high"}},
		},
		{
			name:         "openrouter responses level",
			protocol:     llm.AdapterOpenRouterResponses,
			capabilities: explicitReasoning("openai", allLevels, "high", ""),
			options:      map[string]any{"reasoning_effort": "medium"},
			wantLevel:    "medium",
			want:         map[string]any{"reasoning": map[string]any{"effort": "medium"}},
		},
		{
			name:         "anthropic effort none disables thinking and removes effort",
			protocol:     llm.AdapterAnthropicMessages,
			capabilities: `{"reasoning":{"format":"anthropic_effort","levels":["none","low","high"],"default":"high"},"defaultOptions":{"thinking":{"type":"adaptive","display":"summarized"},"output_config":{"effort":"high"}}}`,
			options:      map[string]any{"reasoning_effort": "none", "effort": "high", "enable_thinking": true},
			wantLevel:    "none",
			want:         map[string]any{"thinking": map[string]any{"type": "disabled"}},
		},
		{
			name:         "anthropic effort level sets adaptive when thinking missing",
			protocol:     llm.AdapterAnthropicMessages,
			capabilities: explicitReasoning("anthropic_effort", `["low","medium","high","xhigh","max"]`, "high", ""),
			options:      map[string]any{"reasoning_effort": "max"},
			wantLevel:    "max",
			want:         map[string]any{"output_config": map[string]any{"effort": "max"}, "thinking": map[string]any{"type": "adaptive"}},
		},
		{
			name:         "anthropic effort level re-enables disabled thinking",
			protocol:     llm.AdapterAnthropicMessages,
			capabilities: explicitReasoning("anthropic_effort", `["low","high"]`, "high", ""),
			options:      map[string]any{"reasoning_effort": "low", "thinking": map[string]any{"type": "disabled"}},
			wantLevel:    "low",
			want:         map[string]any{"output_config": map[string]any{"effort": "low"}, "thinking": map[string]any{"type": "adaptive"}},
		},
		{
			name:         "anthropic budget none",
			protocol:     llm.AdapterAnthropicMessages,
			capabilities: explicitReasoning("anthropic_budget", `["none","low","high"]`, "high", `{"low":2048,"high":16000}`),
			options:      map[string]any{"reasoning_effort": "none", "thinking": map[string]any{"type": "enabled", "budget_tokens": 4096}},
			wantLevel:    "none",
			want:         map[string]any{"thinking": map[string]any{"type": "disabled"}},
		},
		{
			name:         "anthropic budget level clamps to max_tokens",
			protocol:     llm.AdapterAnthropicMessages,
			capabilities: explicitReasoning("anthropic_budget", `["none","low","high"]`, "high", `{"low":2048,"high":16000}`),
			options:      map[string]any{"reasoning_effort": "high", "max_tokens": 8000},
			wantLevel:    "high",
			want:         map[string]any{"max_tokens": 8000, "thinking": map[string]any{"type": "enabled", "budget_tokens": 7999}},
		},
		{
			name:         "gemini level on generate content drops conflicting budget",
			protocol:     llm.AdapterGoogleGenerateContent,
			capabilities: `{"reasoning":{"format":"gemini_level","levels":["low","medium","high"],"default":"high"},"defaultOptions":{"generationConfig":{"thinkingConfig":{"includeThoughts":true,"thinkingBudget":1024}}}}`,
			options:      map[string]any{"reasoning_effort": "low"},
			wantLevel:    "low",
			want:         map[string]any{"generationConfig": map[string]any{"thinkingConfig": map[string]any{"includeThoughts": true, "thinkingLevel": "low"}}},
		},
		{
			name:         "gemini level on interactions",
			protocol:     llm.AdapterGeminiInteractions,
			capabilities: explicitReasoning("gemini_level", `["minimal","low","medium","high"]`, "medium", ""),
			options:      map[string]any{"reasoning_effort": "minimal", "generation_config": map[string]any{"thinking_summaries": "auto"}},
			wantLevel:    "minimal",
			want:         map[string]any{"generation_config": map[string]any{"thinking_level": "minimal", "thinking_summaries": "auto"}},
		},
		{
			name:         "gemini budget none",
			protocol:     llm.AdapterGoogleGenerateContent,
			capabilities: explicitReasoning("gemini_budget", `["none","low","high"]`, "high", `{"low":1024,"high":24576}`),
			options:      map[string]any{"reasoning_effort": "none"},
			wantLevel:    "none",
			want:         map[string]any{"generationConfig": map[string]any{"thinkingConfig": map[string]any{"thinkingBudget": 0}}},
		},
		{
			name:         "gemini budget level",
			protocol:     llm.AdapterGoogleGenerateContent,
			capabilities: explicitReasoning("gemini_budget", `["none","low","high"]`, "high", `{"low":1024,"high":24576}`),
			options:      map[string]any{"reasoning_effort": "high"},
			wantLevel:    "high",
			want:         map[string]any{"generationConfig": map[string]any{"thinkingConfig": map[string]any{"thinkingBudget": 24576}}},
		},
		{
			name:         "qwen none",
			protocol:     llm.AdapterOpenAIChatCompletions,
			capabilities: explicitReasoning("qwen", `["none","low","high"]`, "high", `{"low":1024}`),
			options:      map[string]any{"reasoning_effort": "none"},
			wantLevel:    "none",
			want:         map[string]any{"enable_thinking": false},
		},
		{
			name:         "qwen level with budget",
			protocol:     llm.AdapterOpenAIChatCompletions,
			capabilities: explicitReasoning("qwen", `["none","low","high"]`, "high", `{"low":1024}`),
			options:      map[string]any{"reasoning_effort": "low"},
			wantLevel:    "low",
			want:         map[string]any{"enable_thinking": true, "thinking_budget": 1024},
		},
		{
			name:         "qwen level without budget",
			protocol:     llm.AdapterOpenAIChatCompletions,
			capabilities: explicitReasoning("qwen", `["none","low","high"]`, "high", `{"low":1024}`),
			options:      map[string]any{"reasoning_effort": "high"},
			wantLevel:    "high",
			want:         map[string]any{"enable_thinking": true},
		},
		{
			name:         "toggle none",
			protocol:     llm.AdapterOpenAIChatCompletions,
			capabilities: explicitReasoning("toggle", `["none","high"]`, "high", ""),
			options:      map[string]any{"reasoning_effort": "none"},
			wantLevel:    "none",
			want:         map[string]any{"thinking": map[string]any{"type": "disabled"}},
		},
		{
			name:         "toggle on",
			protocol:     llm.AdapterOpenAIChatCompletions,
			capabilities: explicitReasoning("toggle", `["none","high"]`, "none", ""),
			options:      map[string]any{"reasoning_effort": "high"},
			wantLevel:    "high",
			want:         map[string]any{"thinking": map[string]any{"type": "enabled"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, level := resolveChatModelOptions(tt.options, tt.protocol, reasoningPolicy(tt.capabilities))
			if level != tt.wantLevel {
				t.Fatalf("expected level %q, got %q", tt.wantLevel, level)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected translated options:\n got  %#v\n want %#v", got, tt.want)
			}
		})
	}
}

func TestResolveChatModelOptionsDegradesAndDefaults(t *testing.T) {
	capabilities := explicitReasoning("openai", `["low","high","max"]`, "high", "")
	tests := map[string]struct {
		options map[string]any
		want    string
	}{
		"absent uses default":         {options: nil, want: "high"},
		"unknown string uses default": {options: map[string]any{"reasoning_effort": "turbo"}, want: "high"},
		"non string uses default":     {options: map[string]any{"reasoning_effort": 3}, want: "high"},
		"undeclared none uses lowest": {options: map[string]any{"reasoning_effort": "none"}, want: "low"},
		"tie degrades lower":          {options: map[string]any{"reasoning_effort": "medium"}, want: "low"},
		"nearest declared":            {options: map[string]any{"reasoning_effort": "xhigh"}, want: "high"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, level := resolveChatModelOptions(tt.options, llm.AdapterOpenAIResponses, reasoningPolicy(capabilities))
			if level != tt.want {
				t.Fatalf("expected level %q, got %q", tt.want, level)
			}
			if effort := got["reasoning"].(map[string]any)["effort"]; effort != tt.want {
				t.Fatalf("expected translated effort %q, got %#v", tt.want, got)
			}
			if _, leaked := got["reasoning_effort"]; leaked {
				t.Fatalf("expected canonical key removed on responses, got %#v", got)
			}
		})
	}
}

func TestResolveChatModelOptionsPrecedence(t *testing.T) {
	inferred := `{
		"defaultOptions": {"output_config": {"effort": "high"}},
		"optionControls": [{"path": "output_config.effort", "type": "select", "options": ["low", "medium", "high"]}]
	}`
	// 规范键优先于旧客户端原生路径。
	_, level := resolveChatModelOptions(map[string]any{
		"reasoning_effort": "low",
		"output_config":    map[string]any{"effort": "medium"},
	}, llm.AdapterAnthropicMessages, reasoningPolicy(inferred))
	if level != "low" {
		t.Fatalf("expected canonical key to win over native path, got %q", level)
	}
	// 管理员高级 JSON 中的原生档位原样透传（聊天链路不再应用参数白名单）。
	got, level := resolveChatModelOptions(map[string]any{
		"output_config": map[string]any{"effort": "MEDIUM"},
	}, llm.AdapterAnthropicMessages, reasoningPolicy(inferred))
	if level != "medium" || got["output_config"].(map[string]any)["effort"] != "MEDIUM" {
		t.Fatalf("expected raw native value to pass through verbatim, got %q %#v", level, got)
	}
	// 请求值优先于能力默认值（包括 defaultOptions 中的原生默认值）。
	got, level = resolveChatModelOptions(map[string]any{"reasoning_effort": "medium"}, llm.AdapterAnthropicMessages, reasoningPolicy(inferred))
	if level != "medium" || got["output_config"].(map[string]any)["effort"] != "medium" {
		t.Fatalf("expected request to override default, got %q %#v", level, got)
	}
	// 管理员原生默认值通过推断默认档位下发（修复白名单曾丢弃 output_config.effort 默认值的问题）。
	got, level = resolveChatModelOptions(nil, llm.AdapterAnthropicMessages, reasoningPolicy(inferred))
	if level != "high" || got["output_config"].(map[string]any)["effort"] != "high" {
		t.Fatalf("expected inferred default to be sent, got %q %#v", level, got)
	}
}

func TestResolveChatModelOptionsLocked(t *testing.T) {
	for _, lockedPath := range []string{"reasoning_effort", "reasoning.effort"} {
		t.Run(lockedPath, func(t *testing.T) {
			capabilities := `{"reasoning":{"format":"openai","levels":["low","medium","high"],"default":"medium"},"lockedOptionPaths":["` + lockedPath + `"]}`
			got, level := resolveChatModelOptions(map[string]any{
				"reasoning_effort": "high",
				"reasoning":        map[string]any{"effort": "low"},
			}, llm.AdapterOpenAIResponses, reasoningPolicy(capabilities))
			if level != "medium" || got["reasoning"].(map[string]any)["effort"] != "medium" {
				t.Fatalf("expected locked default, got %q %#v", level, got)
			}
		})
	}
}

func TestResolveChatModelOptionsIgnoresLegacyPolicy(t *testing.T) {
	capabilities := explicitReasoning("anthropic_effort", `["low","high"]`, "high", "")
	for _, mode := range []string{modelOptionPolicyAllowlist, modelOptionPolicyDenylist, modelOptionPolicyDisabled} {
		policy := reasoningPolicy(capabilities)
		policy.Mode = mode
		policy.AllowedPathsJSON = `{"default":[]}`
		policy.DeniedPathsJSON = `{"default":["reasoning_effort","temperature"]}`
		got, level := resolveChatModelOptions(map[string]any{"reasoning_effort": "low", "temperature": 0.3}, llm.AdapterAnthropicMessages, policy)
		if level != "low" || got["temperature"] != 0.3 || got["output_config"].(map[string]any)["effort"] != "low" {
			t.Fatalf("%s: expected chat options to ignore the legacy policy, got %q %#v", mode, level, got)
		}
	}
	// 系统字段依旧被硬拒绝。
	got, _ := resolveChatModelOptions(map[string]any{"stream": false, "model": "x", "api_key": "k"}, llm.AdapterAnthropicMessages, reasoningPolicy(capabilities))
	for _, key := range []string{"stream", "model", "api_key"} {
		if _, leaked := got[key]; leaked {
			t.Fatalf("expected %s to be hard denied, got %#v", key, got)
		}
	}
}

func TestResolveChatModelOptionsWithoutCapabilityIsUnchanged(t *testing.T) {
	cases := []struct {
		protocol     string
		capabilities string
		options      map[string]any
	}{
		{protocol: llm.AdapterOpenAIResponses, capabilities: `{"defaultOptions":{"reasoning":{"summary":"auto"}}}`, options: map[string]any{"reasoning": map[string]any{"effort": "high"}, "temperature": 0.2}},
		{protocol: llm.AdapterOpenAIChatCompletions, capabilities: ``, options: map[string]any{"reasoning_effort": "high", "thinking": map[string]any{"type": "enabled"}}},
		{protocol: llm.AdapterAnthropicMessages, capabilities: `{"defaultOptions":{"max_tokens":64000}}`, options: map[string]any{"reasoning_effort": "high", "thinking": map[string]any{"type": "enabled", "budget_tokens": 2048}}},
		// 显式能力与协议不兼容时不翻译，保持旧行为。
		{protocol: llm.AdapterOpenAIChatCompletions, capabilities: explicitReasoning("gemini_level", `["low","high"]`, "high", ""), options: map[string]any{"reasoning_effort": "low"}},
		// 媒体协议永远不翻译。
		{protocol: llm.AdapterOpenAIImageGenerations, capabilities: explicitReasoning("openai", `["low","high"]`, "high", ""), options: map[string]any{"quality": "high"}},
	}
	for _, tt := range cases {
		cfg := reasoningPolicy(tt.capabilities)
		want := filterModelOptions(tt.options, tt.protocol, cfg)
		got, level := resolveChatModelOptions(tt.options, tt.protocol, cfg)
		if level != "" {
			t.Fatalf("%s: expected no reasoning level, got %q", tt.protocol, level)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: expected passthrough %#v, got %#v", tt.protocol, want, got)
		}
	}
}

func TestResolveChatModelOptionsCatalogCapabilityOnlyOnRequest(t *testing.T) {
	catalog := &domainchannel.ReasoningCapability{
		Format:  domainchannel.ReasoningFormatAnthropicBudget,
		Levels:  []string{"low", "medium", "high"},
		Default: "medium",
		Budgets: map[string]int{"low": 2048, "medium": 8192, "high": 24576},
	}
	policy := reasoningPolicy(`{}`)
	policy.CatalogReasoning = catalog

	// 未显式选择档位：不下发任何推理参数，保持上游默认行为。
	options := map[string]any{"temperature": 0.3}
	got, level := resolveChatModelOptions(options, llm.AdapterAnthropicMessages, policy)
	if level != "" || !reflect.DeepEqual(got, filterModelOptions(options, llm.AdapterAnthropicMessages, policy)) {
		t.Fatalf("expected untouched options without a requested level, got %#v (%q)", got, level)
	}

	// 显式选择档位：按目录能力翻译为原生参数。
	got, level = resolveChatModelOptions(map[string]any{"reasoning_effort": "high"}, llm.AdapterAnthropicMessages, policy)
	thinking, _ := got["thinking"].(map[string]any)
	if level != "high" || thinking["type"] != "enabled" || thinking["budget_tokens"] != 24576 {
		t.Fatalf("expected translated catalog level, got %#v (%q)", got, level)
	}
	if _, exists := got["reasoning_effort"]; exists {
		t.Fatalf("expected canonical key to be consumed, got %#v", got)
	}

	// 锁定档位时即使未选择也按默认档位下发。
	locked := policy
	locked.ModelCapabilitiesJSON = `{"lockedOptionPaths":["reasoning_effort"]}`
	if _, level := resolveChatModelOptions(map[string]any{}, llm.AdapterAnthropicMessages, locked); level != "medium" {
		t.Fatalf("expected locked catalog capability to apply its default, got %q", level)
	}

	// 显式声明优先于目录能力。
	explicit := policy
	explicit.ModelCapabilitiesJSON = explicitReasoning("anthropic_effort", `["low","high"]`, "high", "")
	if _, level := resolveChatModelOptions(map[string]any{}, llm.AdapterAnthropicMessages, explicit); level != "high" {
		t.Fatalf("expected explicit capability default to apply, got %q", level)
	}
}

func TestResolveChatModelOptionsLayeredPriority(t *testing.T) {
	allowAnthropicThinking := func(capabilities string, catalog *domainchannel.ReasoningCapability) modelOptionPolicyConfig {
		policy := reasoningPolicy(capabilities)
		policy.AllowedPathsJSON = `{"anthropic_messages":["thinking","max_tokens","temperature"]}`
		policy.CatalogReasoning = catalog
		return policy
	}
	declared := explicitReasoning("anthropic_budget", `["none","low","high"]`, "high", `{"low":2048,"high":16000}`)
	catalog := &domainchannel.ReasoningCapability{Format: domainchannel.ReasoningFormatAnthropicEffort, Levels: []string{"low", "high"}, Default: "high"}

	// 1. 组件档位优先于用户手写的原生参数。
	got, level := resolveChatModelOptions(map[string]any{
		"reasoning_effort": "low",
		"thinking":         map[string]any{"type": "enabled", "budget_tokens": 5000},
	}, llm.AdapterAnthropicMessages, allowAnthropicThinking(declared, nil))
	if level != "low" || got["thinking"].(map[string]any)["budget_tokens"] != 2048 {
		t.Fatalf("expected component level to win, got %q %#v", level, got)
	}

	// 2. 用户手写原生参数原样透传，优先于管理员声明的默认档。
	userThinking := map[string]any{"type": "enabled", "budget_tokens": float64(5000)}
	got, level = resolveChatModelOptions(map[string]any{"thinking": userThinking}, llm.AdapterAnthropicMessages, allowAnthropicThinking(declared, nil))
	if !reflect.DeepEqual(got["thinking"], userThinking) || level != "low" {
		t.Fatalf("expected user native thinking to pass through, got %q %#v", level, got)
	}

	// 3. 管理员 defaultOptions 原生参数优先于声明的默认档，同样原样透传。
	withAdminDefault := `{"reasoning":{"format":"anthropic_budget","levels":["none","low","high"],"default":"high","budgets":{"low":2048,"high":16000}},
		"defaultOptions":{"thinking":{"type":"enabled","budget_tokens":12000}}}`
	got, level = resolveChatModelOptions(nil, llm.AdapterAnthropicMessages, allowAnthropicThinking(withAdminDefault, nil))
	if fmt.Sprint(got["thinking"].(map[string]any)["budget_tokens"]) != "12000" || level != "high" {
		t.Fatalf("expected admin native default to pass through, got %q %#v", level, got)
	}
	// 用户原生参数覆盖管理员原生默认值。
	got, _ = resolveChatModelOptions(map[string]any{"thinking": map[string]any{"type": "disabled"}}, llm.AdapterAnthropicMessages, allowAnthropicThinking(withAdminDefault, nil))
	if got["thinking"].(map[string]any)["type"] != "disabled" {
		t.Fatalf("expected user native value to override admin default, got %#v", got)
	}

	// 4. 没有任何原生参数时翻译管理员声明的默认档。
	got, level = resolveChatModelOptions(nil, llm.AdapterAnthropicMessages, allowAnthropicThinking(declared, nil))
	if level != "high" || got["thinking"].(map[string]any)["budget_tokens"] != 16000 {
		t.Fatalf("expected declared default to be translated, got %q %#v", level, got)
	}

	// 5. 只有目录能力时不下发；用户手写原生参数仍透传。
	got, level = resolveChatModelOptions(map[string]any{"temperature": 0.2}, llm.AdapterAnthropicMessages, allowAnthropicThinking(`{}`, catalog))
	if level != "" || got["thinking"] != nil || got["output_config"] != nil {
		t.Fatalf("expected catalog capability to send nothing, got %q %#v", level, got)
	}
	got, _ = resolveChatModelOptions(map[string]any{"thinking": map[string]any{"type": "disabled"}}, llm.AdapterAnthropicMessages, allowAnthropicThinking(`{}`, catalog))
	if got["thinking"].(map[string]any)["type"] != "disabled" {
		t.Fatalf("expected user native thinking to pass through on catalog models, got %#v", got)
	}
}

func TestResolveChatModelOptionsDoesNotMutateInput(t *testing.T) {
	options := map[string]any{"reasoning_effort": "none", "thinking": map[string]any{"type": "adaptive", "display": "summarized"}}
	capabilities := explicitReasoning("anthropic_effort", `["none","high"]`, "high", "")
	policy := reasoningPolicy(capabilities)
	policy.Mode = modelOptionPolicyDenylist
	resolveChatModelOptions(options, llm.AdapterAnthropicMessages, policy)
	if options["reasoning_effort"] != "none" || options["thinking"].(map[string]any)["type"] != "adaptive" {
		t.Fatalf("expected caller options untouched, got %#v", options)
	}
}

func TestClampAnthropicThinkingBudget(t *testing.T) {
	tests := []struct{ budget, maxTokens, want int }{
		{budget: 16000, maxTokens: 64000, want: 16000},
		{budget: 16000, maxTokens: 8000, want: 7999},
		{budget: 16000, maxTokens: 900, want: 1024},
		{budget: 500, maxTokens: 64000, want: 1024},
	}
	for _, tt := range tests {
		if got := clampAnthropicThinkingBudget(tt.budget, tt.maxTokens); got != tt.want {
			t.Fatalf("clamp(%d, %d) = %d, want %d", tt.budget, tt.maxTokens, got, tt.want)
		}
	}
	if got := anthropicRequestMaxTokens(map[string]any{"max_output_tokens": float64(4096), "max_tokens": 9000}); got != 4096 {
		t.Fatalf("expected max_output_tokens priority, got %d", got)
	}
	if got := anthropicRequestMaxTokens(nil); got != anthropicDefaultMaxTokens {
		t.Fatalf("expected default max tokens, got %d", got)
	}
}
