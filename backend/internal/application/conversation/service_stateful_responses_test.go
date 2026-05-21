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
		Messages:          appendAssistantStateMessage(firstPrompt, "第一轮回答"),
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
		Messages:          buildNextStatefulPrefixMessages(firstPrompt, "第一轮", "第一轮回答"),
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
	changedCfg.RAGMinSimilarity = baseCfg.RAGMinSimilarity + 0.1

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
