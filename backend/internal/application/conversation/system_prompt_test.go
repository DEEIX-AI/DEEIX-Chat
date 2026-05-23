package conversation

import (
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestResolveSystemPromptInjectionUsesNativeSystemPrompt(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		ModelSystemPrompt:     "model rule",
		ModelCapabilitiesJSON: `{"supportsSystemPrompt":true}`,
	}

	got := resolveSystemPromptInjection(config.Config{DefaultSystemPrompt: "global rule"}, route)
	if got.Content == "" {
		t.Fatal("expected system prompt content")
	}
	if got.InlineToUser {
		t.Fatal("expected native system prompt")
	}
	for _, want := range []string{"Global instructions", "global rule", "Model instructions", "model rule"} {
		if !strings.Contains(got.Content, want) {
			t.Fatalf("expected content to contain %q, got %q", want, got.Content)
		}
	}
}

func TestResolveSystemPromptInjectionFallsBackWhenCapabilitiesDisableSystemPrompt(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		ModelCapabilitiesJSON: `{"supportsSystemPrompt":false}`,
	}

	got := resolveSystemPromptInjection(config.Config{DefaultSystemPrompt: "global rule"}, route)
	if !got.InlineToUser {
		t.Fatal("expected user prompt fallback")
	}
}

func TestResolveSystemPromptInjectionFallsBackWithSnakeCaseCapabilities(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		ModelCapabilitiesJSON: `{"supports_system_prompt":false}`,
	}

	got := resolveSystemPromptInjection(config.Config{DefaultSystemPrompt: "global rule"}, route)
	if !got.InlineToUser {
		t.Fatal("expected snake_case capability to use user prompt fallback")
	}
}

func TestResolveSystemPromptInjectionFallsBackWhenModeRequestsUserPrompt(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		ModelCapabilitiesJSON: `{"systemPromptMode":"user"}`,
	}

	got := resolveSystemPromptInjection(config.Config{DefaultSystemPrompt: "global rule"}, route)
	if !got.InlineToUser {
		t.Fatal("expected systemPromptMode=user to use user prompt fallback")
	}
}

func TestResolveSystemPromptInjectionFallsBackForGemma(t *testing.T) {
	route := &channel.ResolvedRoute{
		PlatformModelName: "gemma-3-27b",
		Protocol:          llm.AdapterGoogleGenerateContent,
	}

	got := resolveSystemPromptInjection(config.Config{DefaultSystemPrompt: "global rule"}, route)
	if !got.InlineToUser {
		t.Fatal("expected Gemma to inline system prompt into user prompt")
	}
}

func TestInlineSystemPromptIntoLatestUserMessage(t *testing.T) {
	messages := []llm.Message{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "answer"},
		{Role: "user", Content: "second"},
	}

	got := inlineSystemPromptIntoLatestUserMessage(messages, "system rule")
	if got[0].Content != "first" {
		t.Fatalf("expected first user message to stay unchanged, got %q", got[0].Content)
	}
	if !strings.Contains(got[2].Content, "<system_instructions>") || !strings.Contains(got[2].Content, "system rule") || !strings.Contains(got[2].Content, "second") {
		t.Fatalf("expected latest user message to include inline system prompt and original content, got %q", got[2].Content)
	}
}
