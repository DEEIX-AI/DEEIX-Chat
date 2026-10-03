package llm

import (
	"encoding/json"
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// 以下测试使用应用层推理档位翻译产出的原生参数形状，证明每个原生键都真实到达上游请求体。

func reasoningTestMessages() []portllm.Message {
	return []portllm.Message{{Role: "user", Content: "hello"}}
}

// upstreamJSON 把请求体按真实发送路径序列化后再解码，确保断言的是上游实际收到的 JSON。
func upstreamJSON(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return decoded
}

func jsonPath(t *testing.T, payload map[string]any, path ...string) any {
	t.Helper()
	var current any = payload
	for _, segment := range path {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("expected object at %q in %#v", segment, payload)
		}
		current, ok = object[segment]
		if !ok {
			t.Fatalf("expected key %v in upstream payload %#v", path, payload)
		}
	}
	return current
}

func TestReasoningEffortReachesOpenAIChatCompletions(t *testing.T) {
	for _, adapter := range []string{portllm.AdapterOpenAIChatCompletions, portllm.AdapterOpenRouterChat} {
		for _, level := range []string{"none", "high"} {
			payload := upstreamJSON(t, mustBuildRequestBody(t, adapter, "gpt-5", portllm.EndpointChatCompletions, portllm.GenerateInput{
				Messages: reasoningTestMessages(),
				Options:  map[string]any{"reasoning_effort": level},
			}, true))
			if payload["reasoning_effort"] != level {
				t.Fatalf("%s: expected reasoning_effort %q, got %#v", adapter, level, payload)
			}
		}
	}
}

func TestReasoningQwenThinkingReachesOpenAIChatCompletions(t *testing.T) {
	payload := upstreamJSON(t, mustBuildRequestBody(t, portllm.AdapterOpenAIChatCompletions, "qwen3-max", portllm.EndpointChatCompletions, portllm.GenerateInput{
		Messages: reasoningTestMessages(),
		Options:  map[string]any{"enable_thinking": true, "thinking_budget": 8192},
	}, true))
	if payload["enable_thinking"] != true || payload["thinking_budget"] != float64(8192) {
		t.Fatalf("expected qwen thinking switch and budget at top level, got %#v", payload)
	}

	disabled := upstreamJSON(t, mustBuildRequestBody(t, portllm.AdapterOpenAIChatCompletions, "qwen3-max", portllm.EndpointChatCompletions, portllm.GenerateInput{
		Messages: reasoningTestMessages(),
		Options:  map[string]any{"enable_thinking": false},
	}, false))
	if disabled["enable_thinking"] != false {
		t.Fatalf("expected enable_thinking=false to be forwarded, got %#v", disabled)
	}
	if _, ok := disabled["thinking_budget"]; ok {
		t.Fatalf("expected no thinking_budget when not provided, got %#v", disabled)
	}
}

func TestReasoningToggleReachesOpenAIChatCompletions(t *testing.T) {
	for _, thinkingType := range []string{"enabled", "disabled"} {
		payload := upstreamJSON(t, mustBuildRequestBody(t, portllm.AdapterOpenAIChatCompletions, "deepseek-v4", portllm.EndpointChatCompletions, portllm.GenerateInput{
			Messages: reasoningTestMessages(),
			Options:  map[string]any{"thinking": map[string]any{"type": thinkingType}},
		}, true))
		if got := jsonPath(t, payload, "thinking", "type"); got != thinkingType {
			t.Fatalf("expected thinking.type %q, got %#v", thinkingType, payload["thinking"])
		}
	}
}

func TestReasoningEffortReachesResponsesAndKeepsSummary(t *testing.T) {
	for _, adapter := range []string{portllm.AdapterOpenAIResponses, portllm.AdapterXAIResponses, portllm.AdapterOpenRouterResponses} {
		payload := upstreamJSON(t, mustBuildRequestBody(t, adapter, "gpt-5", portllm.EndpointResponses, portllm.GenerateInput{
			Messages: reasoningTestMessages(),
			Options:  map[string]any{"reasoning": map[string]any{"effort": "none", "summary": "auto"}},
		}, true))
		if got := jsonPath(t, payload, "reasoning", "effort"); got != "none" {
			t.Fatalf("%s: expected reasoning.effort none, got %#v", adapter, payload["reasoning"])
		}
		if got := jsonPath(t, payload, "reasoning", "summary"); got != "auto" {
			t.Fatalf("%s: expected reasoning.summary preserved, got %#v", adapter, payload["reasoning"])
		}
	}
}

func TestReasoningAnthropicEffortReachesMessages(t *testing.T) {
	payload := upstreamJSON(t, mustBuildAnthropicRequestBody(t, "claude-opus-4-7", portllm.GenerateInput{
		Messages: reasoningTestMessages(),
		Options: map[string]any{
			"thinking":      map[string]any{"type": "adaptive", "display": "summarized"},
			"output_config": map[string]any{"effort": "xhigh"},
		},
	}, true))
	if jsonPath(t, payload, "output_config", "effort") != "xhigh" {
		t.Fatalf("expected output_config.effort, got %#v", payload["output_config"])
	}
	if jsonPath(t, payload, "thinking", "type") != "adaptive" || jsonPath(t, payload, "thinking", "display") != "summarized" {
		t.Fatalf("expected adaptive thinking with display, got %#v", payload["thinking"])
	}

	disabled := upstreamJSON(t, mustBuildAnthropicRequestBody(t, "claude-opus-4-7", portllm.GenerateInput{
		Messages: reasoningTestMessages(),
		Options:  map[string]any{"thinking": map[string]any{"type": "disabled"}},
	}, true))
	if jsonPath(t, disabled, "thinking", "type") != "disabled" {
		t.Fatalf("expected thinking disabled, got %#v", disabled["thinking"])
	}
	if _, ok := disabled["output_config"]; ok {
		t.Fatalf("expected no output_config when effort removed, got %#v", disabled["output_config"])
	}
}

func TestReasoningAnthropicBudgetReachesMessages(t *testing.T) {
	payload := upstreamJSON(t, mustBuildAnthropicRequestBody(t, "claude-sonnet-3.7", portllm.GenerateInput{
		Messages: reasoningTestMessages(),
		Options: map[string]any{
			"max_tokens": 32000,
			"thinking":   map[string]any{"type": "enabled", "budget_tokens": 16000, "display": "omitted"},
		},
	}, true))
	thinking := jsonPath(t, payload, "thinking").(map[string]any)
	if thinking["type"] != "enabled" || thinking["budget_tokens"] != float64(16000) || thinking["display"] != "omitted" {
		t.Fatalf("expected enabled thinking budget, got %#v", thinking)
	}
	if payload["max_tokens"] != float64(32000) {
		t.Fatalf("expected max_tokens forwarded, got %#v", payload["max_tokens"])
	}
}

func TestReasoningGeminiLevelAndBudgetReachGenerateContent(t *testing.T) {
	level := upstreamJSON(t, mustBuildGeminiRequestBody(t, portllm.GenerateInput{
		Messages: reasoningTestMessages(),
		Options: map[string]any{
			"generationConfig": map[string]any{"thinkingConfig": map[string]any{"includeThoughts": true, "thinkingLevel": "low"}},
		},
	}))
	if jsonPath(t, level, "generationConfig", "thinkingConfig", "thinkingLevel") != "low" {
		t.Fatalf("expected thinkingLevel, got %#v", level["generationConfig"])
	}
	if jsonPath(t, level, "generationConfig", "thinkingConfig", "includeThoughts") != true {
		t.Fatalf("expected includeThoughts preserved, got %#v", level["generationConfig"])
	}

	for _, budget := range []int{0, 8192} {
		payload := upstreamJSON(t, mustBuildGeminiRequestBody(t, portllm.GenerateInput{
			Messages: reasoningTestMessages(),
			Options: map[string]any{
				"generationConfig": map[string]any{"thinkingConfig": map[string]any{"thinkingBudget": budget}},
			},
		}))
		if jsonPath(t, payload, "generationConfig", "thinkingConfig", "thinkingBudget") != float64(budget) {
			t.Fatalf("expected thinkingBudget %d, got %#v", budget, payload["generationConfig"])
		}
	}
}

func TestReasoningGeminiLevelReachesInteractions(t *testing.T) {
	payload, err := buildGeminiInteractionRequestBody(portllm.RouteConfig{
		Endpoint:      portllm.EndpointInteractions,
		UpstreamModel: "gemini-3-pro",
	}, portllm.GenerateInput{
		Messages: reasoningTestMessages(),
		Options: map[string]any{
			"generation_config": map[string]any{"thinking_level": "minimal", "thinking_summaries": "auto"},
		},
	})
	if err != nil {
		t.Fatalf("build interactions body: %v", err)
	}
	decoded := upstreamJSON(t, payload)
	if jsonPath(t, decoded, "generation_config", "thinking_level") != "minimal" ||
		jsonPath(t, decoded, "generation_config", "thinking_summaries") != "auto" {
		t.Fatalf("expected thinking_level and summaries, got %#v", decoded["generation_config"])
	}
}
