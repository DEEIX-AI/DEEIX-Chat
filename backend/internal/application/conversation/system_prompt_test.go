package conversation

import (
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestResolveMessageSystemPromptInjectionUsesNativeSystemPrompt(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		ModelSystemPrompt:     "model rule",
		ModelCapabilitiesJSON: `{"supportsSystemPrompt":true}`,
	}

	got := resolveMessageSystemPromptInjection(config.Config{DefaultSystemPrompt: "global rule"}, route, "project rule", false, "")
	if got.Content == "" {
		t.Fatal("expected system prompt content")
	}
	if got.InlineToUser {
		t.Fatal("expected native system prompt")
	}
	for _, want := range []string{`<layers order="high_to_low">`, `<platform p="100">`, "global rule", `<model p="80">`, "model rule", `<project p="50" override="no">`, "project rule"} {
		if !strings.Contains(got.Content, want) {
			t.Fatalf("expected content to contain %q, got %q", want, got.Content)
		}
	}
	if strings.Contains(got.Content, "# Global instructions") {
		t.Fatalf("expected XML prompt layers, got %q", got.Content)
	}
}

func TestResolveMessageSystemPromptInjectionAddsHTMLVisualPrompt(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
	}

	got := resolveMessageSystemPromptInjection(config.Config{}, route, "", true, "")
	if got.Content == "" {
		t.Fatal("expected request-level system prompt content")
	}
	if got.InlineToUser {
		t.Fatal("expected native system prompt")
	}
	for _, want := range []string{`<format p="30" scope="request">`, "html-visual", "遵循用户语言", "HTML 实时渲染"} {
		if !strings.Contains(got.Content, want) {
			t.Fatalf("expected content to contain %q, got %q", want, got.Content)
		}
	}
	if strings.Contains(got.Content, "使用简体中文") {
		t.Fatalf("expected user-language prompt, got %q", got.Content)
	}
}

func TestResolveMessageSystemPromptInjectionAddsHTMLVisualColorMode(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
	}

	got := resolveMessageSystemPromptInjection(config.Config{}, route, "", true, "dark")
	if !strings.Contains(got.Content, "默认视觉风格需适配当前深色模式") {
		t.Fatalf("expected dark color mode instruction, got %q", got.Content)
	}
	if strings.Contains(got.Content, "color-theme") || strings.Contains(got.Content, "theme-context") {
		t.Fatalf("expected only color mode injection, got %q", got.Content)
	}
}

func TestResolveMessageSystemPromptInjectionOrdersProjectBeforeResponseFormat(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
	}

	got := resolveMessageSystemPromptInjection(config.Config{}, route, "project rule", true, "")
	projectIndex := strings.Index(got.Content, `<project p="50" override="no">`)
	responseIndex := strings.Index(got.Content, `<format p="30" scope="request">`)
	if projectIndex < 0 || responseIndex < 0 {
		t.Fatalf("expected project and response format layers, got %q", got.Content)
	}
	if projectIndex > responseIndex {
		t.Fatalf("expected project instructions before response format instructions, got %q", got.Content)
	}
}

func TestResolveMessageSystemPromptInjectionMarksProjectOverrideBoundary(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
	}

	got := resolveMessageSystemPromptInjection(config.Config{}, route, "project rule", false, "")
	for _, want := range []string{`<project p="50" override="no">`, "must not override platform or model instructions"} {
		if !strings.Contains(got.Content, want) {
			t.Fatalf("expected project boundary %q, got %q", want, got.Content)
		}
	}
}

func TestResolveMessageSystemPromptInjectionPreservesXMLLikeContent(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
	}

	got := resolveMessageSystemPromptInjection(config.Config{DefaultSystemPrompt: `keep <tag> and ]]> safely`}, route, "", false, "")
	for _, want := range []string{`<![CDATA[keep <tag> and ]]]]><![CDATA[> safely]]>`, `<platform p="100">`} {
		if !strings.Contains(got.Content, want) {
			t.Fatalf("expected XML-safe content %q, got %q", want, got.Content)
		}
	}
}

func TestResolveMessageSystemPromptInjectionSkipsHTMLVisualPromptWhenDisabled(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol: llm.AdapterOpenAIResponses,
	}

	got := resolveMessageSystemPromptInjection(config.Config{}, route, "", false, "")
	if got.Content != "" {
		t.Fatalf("expected no system prompt content, got %q", got.Content)
	}
}

func TestResolveMessageSystemPromptInjectionFallsBackWhenCapabilitiesDisableSystemPrompt(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		ModelCapabilitiesJSON: `{"supportsSystemPrompt":false}`,
	}

	got := resolveMessageSystemPromptInjection(config.Config{DefaultSystemPrompt: "global rule"}, route, "", false, "")
	if !got.InlineToUser {
		t.Fatal("expected user prompt fallback")
	}
}

func TestResolveMessageSystemPromptInjectionFallsBackWithSnakeCaseCapabilities(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		ModelCapabilitiesJSON: `{"supports_system_prompt":false}`,
	}

	got := resolveMessageSystemPromptInjection(config.Config{DefaultSystemPrompt: "global rule"}, route, "", false, "")
	if !got.InlineToUser {
		t.Fatal("expected snake_case capability to use user prompt fallback")
	}
}

func TestResolveMessageSystemPromptInjectionFallsBackWhenModeRequestsUserPrompt(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		ModelCapabilitiesJSON: `{"systemPromptMode":"user"}`,
	}

	got := resolveMessageSystemPromptInjection(config.Config{DefaultSystemPrompt: "global rule"}, route, "", false, "")
	if !got.InlineToUser {
		t.Fatal("expected systemPromptMode=user to use user prompt fallback")
	}
}

func TestResolveMessageSystemPromptInjectionFallsBackForGemma(t *testing.T) {
	route := &channel.ResolvedRoute{
		PlatformModelName: "gemma-3-27b",
		Protocol:          llm.AdapterGoogleGenerateContent,
	}

	got := resolveMessageSystemPromptInjection(config.Config{DefaultSystemPrompt: "global rule"}, route, "", false, "")
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
