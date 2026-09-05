package conversation

import (
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestBuildMessageRoutePromptRebuildsRouteSpecificFields(t *testing.T) {
	service := &Service{cfg: config.NewRuntime(config.Config{})}
	domainMessages := []model.Message{
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer", ReasoningContent: "private reasoning"},
		{Role: "user", Content: "follow up"},
	}
	baseInput := messageRoutePromptInput{
		UserContent:         "follow up",
		DomainMessages:      domainMessages,
		ProjectSystemPrompt: "project policy",
		Config: config.Config{
			DefaultSystemPrompt: "platform policy",
		},
	}

	chatInput := baseInput
	chatInput.ReasoningContentPassback = true
	chatPlan, err := service.buildMessageRoutePrompt(t.Context(), &channel.ResolvedRoute{
		Protocol:      llm.AdapterOpenAIChatCompletions,
		UpstreamModel: "deepseek-chat",
	}, chatInput)
	if err != nil {
		t.Fatalf("build chat prompt: %v", err)
	}
	if len(chatPlan.Messages) < 4 || chatPlan.Messages[0].Role != "system" {
		t.Fatalf("expected native system prompt, got %#v", chatPlan.Messages)
	}
	if chatPlan.Messages[2].ReasoningContent != "private reasoning" {
		t.Fatalf("expected reasoning passback, got %#v", chatPlan.Messages[2])
	}

	interactionInput := baseInput
	interactionPlan, err := service.buildMessageRoutePrompt(t.Context(), &channel.ResolvedRoute{
		Protocol:      llm.AdapterGeminiInteractions,
		UpstreamModel: "gemini-2.5-pro",
	}, interactionInput)
	if err != nil {
		t.Fatalf("build interaction prompt: %v", err)
	}
	for _, message := range interactionPlan.Messages {
		if message.Role == "system" {
			t.Fatalf("expected system prompt to be inlined, got %#v", interactionPlan.Messages)
		}
		if message.Role == "assistant" && message.ReasoningContent != "" {
			t.Fatalf("expected reasoning to be removed, got %#v", message)
		}
	}
	latest := interactionPlan.Messages[len(interactionPlan.Messages)-1]
	if latest.Role != "user" || !strings.Contains(latest.Content, "platform policy") || !strings.Contains(latest.Content, "follow up") {
		t.Fatalf("expected inlined system prompt on latest user message, got %#v", latest)
	}
}

func TestWithMessageRouteReasoningPassbackOptions(t *testing.T) {
	route := &channel.ResolvedRoute{
		ReasoningPassbackRequestOptions: map[string]any{
			"preserve_thinking": true,
		},
	}
	messages := []llm.Message{{Role: "assistant", ReasoningContent: "historical reasoning"}}

	got := withMessageRouteReasoningPassbackOptions(nil, nil, route, true, messages)
	if got["preserve_thinking"] != true {
		t.Fatalf("expected fallback route reasoning option, got %#v", got)
	}

	explicit := withMessageRouteReasoningPassbackOptions(
		nil,
		map[string]any{"preserve_thinking": false},
		route,
		true,
		messages,
	)
	if _, ok := explicit["preserve_thinking"]; ok {
		t.Fatalf("expected explicit option to prevent automatic override, got %#v", explicit)
	}

	withoutHistory := withMessageRouteReasoningPassbackOptions(nil, nil, route, true, []llm.Message{{Role: "user", Content: "hello"}})
	if _, ok := withoutHistory["preserve_thinking"]; ok {
		t.Fatalf("expected no option without historical reasoning, got %#v", withoutHistory)
	}
}
