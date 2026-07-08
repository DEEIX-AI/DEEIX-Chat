package conversation

import (
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestResolvePreviousResponseIDOnlyEnablesKnownSafeRoutes(t *testing.T) {
	t.Run("official openai responses defaults on", func(t *testing.T) {
		got := resolvePreviousResponseID(&channel.ResolvedRoute{
			Protocol: llm.AdapterOpenAIResponses,
			BaseURL:  "https://api.openai.com/v1",
		}, "default", "resp_123")
		if got != "resp_123" {
			t.Fatalf("expected previous response id, got %q", got)
		}
	})

	t.Run("custom responses defaults off", func(t *testing.T) {
		got := resolvePreviousResponseID(&channel.ResolvedRoute{
			Protocol: llm.AdapterOpenAIResponses,
			BaseURL:  "https://reverse.example.com/v1",
		}, "default", "resp_123")
		if got != "" {
			t.Fatalf("expected disabled custom route, got %q", got)
		}
	})

	t.Run("xai responses stays off", func(t *testing.T) {
		got := resolvePreviousResponseID(&channel.ResolvedRoute{
			Protocol: llm.AdapterXAIResponses,
			BaseURL:  "https://api.x.ai/v1",
		}, "default", "resp_123")
		if got != "" {
			t.Fatalf("expected xai previous response disabled, got %q", got)
		}
	})

	t.Run("non default branch stays off", func(t *testing.T) {
		got := resolvePreviousResponseID(&channel.ResolvedRoute{
			Protocol: llm.AdapterOpenAIResponses,
			BaseURL:  "https://api.openai.com/v1",
		}, "retry", "resp_123")
		if got != "" {
			t.Fatalf("expected retry branch disabled, got %q", got)
		}
	})
}

func TestSupportsPreviousResponseIDRouteOnlyAllowsOfficialOpenAIResponses(t *testing.T) {
	if !supportsPreviousResponseIDRoute(&channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "https://api.openai.com/v1",
	}) {
		t.Fatalf("expected official OpenAI Responses route to support previous_response_id")
	}
	if supportsPreviousResponseIDRoute(&channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "http://host.docker.internal:42113/v1",
	}) {
		t.Fatalf("expected custom Responses-compatible route to disable previous_response_id")
	}
	if supportsPreviousResponseIDRoute(&channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIChatCompletions,
		BaseURL:  "https://api.openai.com/v1",
	}) {
		t.Fatalf("expected non-Responses route to disable previous_response_id")
	}
}

func TestSupportsOpenAIResponsesBackgroundModeRequiresOfficialRouteAndCapability(t *testing.T) {
	if !supportsOpenAIResponsesBackgroundMode(&channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		BaseURL:               "https://api.openai.com/v1",
		ModelCapabilitiesJSON: `{"responsesBackgroundMode":true}`,
	}) {
		t.Fatalf("expected explicit official OpenAI Responses capability to enable background")
	}
	if !supportsOpenAIResponsesBackgroundMode(&channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		BaseURL:               "https://api.openai.com/v1",
		ModelCapabilitiesJSON: `{"responses":{"backgroundMode":true}}`,
	}) {
		t.Fatalf("expected nested official OpenAI Responses capability to enable background")
	}
	if supportsOpenAIResponsesBackgroundMode(&channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		BaseURL:               "https://reverse.example.com/v1",
		ModelCapabilitiesJSON: `{"responsesBackgroundMode":true}`,
	}) {
		t.Fatalf("expected custom Responses-compatible route to disable background")
	}
	if supportsOpenAIResponsesBackgroundMode(&channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenRouterResponses,
		BaseURL:               "https://openrouter.ai/api/v1",
		ModelCapabilitiesJSON: `{"responsesBackgroundMode":true}`,
	}) {
		t.Fatalf("expected non-official protocol to disable background")
	}
	if supportsOpenAIResponsesBackgroundMode(&channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		BaseURL:               "https://api.openai.com/v1",
		ModelCapabilitiesJSON: `{"responsesBackgroundMode":"true"}`,
	}) {
		t.Fatalf("expected non-boolean capability to disable background")
	}
}

func TestShouldRetryWithoutResponsesBackground(t *testing.T) {
	err := &llm.UpstreamError{
		StatusCode: 400,
		Message:    "Unknown parameter: background",
	}
	if !shouldRetryWithoutResponsesBackground(err) {
		t.Fatalf("expected unsupported background error to be retryable")
	}
	if shouldRetryWithoutResponsesBackground(&llm.UpstreamError{
		StatusCode: 429,
		Message:    "rate limit",
	}) {
		t.Fatalf("expected rate limit to stay non-retryable")
	}
	if shouldRetryWithoutResponsesBackground(&llm.UpstreamError{
		StatusCode: 400,
		Message:    "invalid temperature",
	}) {
		t.Fatalf("expected unrelated validation error to stay non-retryable")
	}
}

func TestBuildStatefulResponseMessagesKeepsLatestUserOnly(t *testing.T) {
	messages := []llm.Message{
		{Role: "system", Content: "behavior"},
		{Role: "system", Content: "tool policy"},
		{Role: "user", Content: "Q1"},
		{Role: "assistant", Content: "A1"},
		{Role: "user", Content: "<ctx>files</ctx><q>Q2</q>"},
	}

	got := buildStatefulResponseMessages(messages)
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %#v", got)
	}
	if got[0].Role != "user" || got[0].Content != "<ctx>files</ctx><q>Q2</q>" {
		t.Fatalf("expected latest user message, got %#v", got[0])
	}
}

func TestApplyOpenAIResponsesInstructionsOnlyForOfficialRoute(t *testing.T) {
	official := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "https://api.openai.com/v1",
	}
	input := llm.GenerateInput{
		Messages: []llm.Message{
			{Role: "system", Content: "platform policy"},
			{Role: "user", Content: "hello"},
			{Role: "system", Content: "final synthesis only"},
			{Role: "tool", ToolResults: []llm.ToolResult{{ToolCallID: "call_1", OutputJSON: `{"ok":true}`}}},
		},
	}

	applyOpenAIResponsesInstructions(official, llm.EndpointResponses, &input)

	if input.Instructions != "platform policy\n\nfinal synthesis only" {
		t.Fatalf("expected extracted instructions, got %q", input.Instructions)
	}
	if len(input.Messages) != 2 || input.Messages[0].Role != "user" || input.Messages[1].Role != "tool" {
		t.Fatalf("expected system messages removed from input, got %#v", input.Messages)
	}

	custom := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "https://reverse.example.com/v1",
	}
	compatInput := llm.GenerateInput{Messages: []llm.Message{{Role: "system", Content: "policy"}, {Role: "user", Content: "hello"}}}
	applyOpenAIResponsesInstructions(custom, llm.EndpointResponses, &compatInput)
	if compatInput.Instructions != "" || len(compatInput.Messages) != 2 {
		t.Fatalf("expected custom route to keep system messages, got %#v", compatInput)
	}
}

func TestResolveStatefulPreviousResponseIDRequiresMatchingFingerprint(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
		BaseURL:  "https://api.openai.com/v1",
	}

	enabled := resolveStatefulPreviousResponseID(route, "default", "resp_123", "fp_a", "fp_a")
	if enabled.PreviousResponseID != "resp_123" || enabled.DisabledReason != "" {
		t.Fatalf("expected enabled decision, got %#v", enabled)
	}

	missing := resolveStatefulPreviousResponseID(route, "default", "resp_123", "", "fp_a")
	if missing.PreviousResponseID != "" || missing.DisabledReason != "missing_stored_fingerprint" {
		t.Fatalf("expected missing fingerprint decision, got %#v", missing)
	}

	mismatch := resolveStatefulPreviousResponseID(route, "default", "resp_123", "fp_a", "fp_b")
	if mismatch.PreviousResponseID != "" || mismatch.DisabledReason != "prompt_fingerprint_mismatch" {
		t.Fatalf("expected mismatch decision, got %#v", mismatch)
	}
}

func TestPromptStateFingerprintMatchesPrefixAfterAssistantAppend(t *testing.T) {
	firstPrompt := []llm.Message{
		{Role: "system", Content: "policy"},
		{Role: "user", Content: "第一轮"},
	}
	stored := buildPromptStateFingerprint(promptStateFingerprintInput{
		Protocol:          llm.AdapterOpenAIResponses,
		Endpoint:          llm.EndpointResponses,
		UpstreamID:        1,
		UpstreamModel:     "gpt-5.5",
		PlatformModelName: "gpt-5.5",
		Messages:          appendAssistantStateMessage(firstPrompt, "第一轮回答", ""),
		Tools: []llm.ToolDefinition{
			{Name: "b", Description: "B", InputSchema: []byte(`{"type":"object"}`)},
			{Name: "a", Description: "A", InputSchema: []byte(`{"type":"object"}`)},
		},
	})
	secondPrompt := []llm.Message{
		{Role: "system", Content: "policy"},
		{Role: "user", Content: "第一轮"},
		{Role: "assistant", Content: "第一轮回答"},
		{Role: "user", Content: "第二轮"},
	}
	prefix := buildPromptStateFingerprint(promptStateFingerprintInput{
		Protocol:          llm.AdapterOpenAIResponses,
		Endpoint:          llm.EndpointResponses,
		UpstreamID:        1,
		UpstreamModel:     "gpt-5.5",
		PlatformModelName: "gpt-5.5",
		Messages:          promptStatePrefixMessages(secondPrompt),
		Tools: []llm.ToolDefinition{
			{Name: "a", Description: "A", InputSchema: []byte(`{"type":"object"}`)},
			{Name: "b", Description: "B", InputSchema: []byte(`{"type":"object"}`)},
		},
	})

	if stored != prefix {
		t.Fatalf("expected state fingerprint to match next prompt prefix")
	}
}

func TestPromptStateFingerprintIncludesAssistantReasoning(t *testing.T) {
	messages := []llm.Message{
		{Role: "user", Content: "第一轮"},
		{Role: "assistant", Content: "第一轮回答", ReasoningContent: "推理 A"},
	}

	left := buildPromptStateFingerprint(promptStateFingerprintInput{Messages: messages})
	messages[1].ReasoningContent = "推理 B"
	right := buildPromptStateFingerprint(promptStateFingerprintInput{Messages: messages})

	if left == right {
		t.Fatal("expected reasoning content to affect prompt state fingerprint")
	}
}

func TestBuildNextStatefulPrefixMessagesKeepsAssistantReasoning(t *testing.T) {
	messages := []llm.Message{
		{Role: "system", Content: "policy"},
		{Role: "user", Content: "<ctx>dynamic</ctx><q>第一轮</q>"},
	}

	got := buildNextStatefulPrefixMessages(messages, "第一轮", "第一轮回答", "推理内容")

	if len(got) != 3 {
		t.Fatalf("expected 3 messages, got %#v", got)
	}
	if got[2].ReasoningContent != "推理内容" {
		t.Fatalf("expected assistant reasoning in state prefix, got %#v", got[2])
	}
}

func TestPromptStateFingerprintUsesRebuildableHistoryWhenCurrentUserHasDynamicContext(t *testing.T) {
	firstPrompt := []llm.Message{
		{Role: "system", Content: "<ctx><files><file name=\"A.md\">稳定文件</file></files></ctx>"},
		{Role: "system", Content: "# tool_use\n- use tools only when useful"},
		{Role: "user", Content: "<ctx><rag><doc name=\"A.md\" i=\"1\">动态片段</doc></rag></ctx>\n\n<q>第一轮</q>"},
	}
	stored := buildPromptStateFingerprint(promptStateFingerprintInput{
		Protocol:          llm.AdapterOpenAIResponses,
		Endpoint:          llm.EndpointResponses,
		UpstreamID:        1,
		UpstreamModel:     "gpt-5.5",
		PlatformModelName: "gpt-5.5",
		Messages:          buildNextStatefulPrefixMessages(firstPrompt, "第一轮", "第一轮回答", ""),
	})
	secondPrompt := []llm.Message{
		{Role: "system", Content: "<ctx><files><file name=\"A.md\">稳定文件</file></files></ctx>"},
		{Role: "system", Content: "# tool_use\n- use tools only when useful"},
		{Role: "user", Content: "第一轮"},
		{Role: "assistant", Content: "第一轮回答"},
		{Role: "user", Content: "<ctx><rag><doc name=\"A.md\" i=\"2\">新片段</doc></rag></ctx>\n\n<q>第二轮</q>"},
	}
	prefix := buildPromptStateFingerprint(promptStateFingerprintInput{
		Protocol:          llm.AdapterOpenAIResponses,
		Endpoint:          llm.EndpointResponses,
		UpstreamID:        1,
		UpstreamModel:     "gpt-5.5",
		PlatformModelName: "gpt-5.5",
		Messages:          promptStatePrefixMessages(secondPrompt),
	})

	if stored != prefix {
		t.Fatalf("expected dynamic first round to match rebuildable second prefix")
	}
}

func TestPromptStateFingerprintChangesWhenContextConfigChanges(t *testing.T) {
	messages := []llm.Message{
		{Role: "system", Content: "policy"},
		{Role: "user", Content: "第一轮"},
	}
	baseCfg := config.Config{
		RAGEnabled:                true,
		RAGModel:                  "embed-a",
		RAGMinSimilarity:          0.45,
		EmbeddingOutputDimensions: 1536,
		EmbeddingNormalize:        true,
	}
	changedCfg := baseCfg
	changedCfg.ContextCompactEnabled = !baseCfg.ContextCompactEnabled

	first := buildPromptStateFingerprint(promptStateFingerprintInput{
		Protocol:          llm.AdapterOpenAIResponses,
		Endpoint:          llm.EndpointResponses,
		UpstreamID:        1,
		UpstreamModel:     "gpt-5.5",
		PlatformModelName: "gpt-5.5",
		ContextConfig:     buildPromptContextConfigSignature(baseCfg),
		Messages:          messages,
	})
	second := buildPromptStateFingerprint(promptStateFingerprintInput{
		Protocol:          llm.AdapterOpenAIResponses,
		Endpoint:          llm.EndpointResponses,
		UpstreamID:        1,
		UpstreamModel:     "gpt-5.5",
		PlatformModelName: "gpt-5.5",
		ContextConfig:     buildPromptContextConfigSignature(changedCfg),
		Messages:          messages,
	})

	if first == second {
		t.Fatal("expected context config change to invalidate state fingerprint")
	}
}
