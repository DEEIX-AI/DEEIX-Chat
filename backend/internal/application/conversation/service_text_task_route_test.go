package conversation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

type textTaskRouteResolverStub struct {
	routes       map[string]*channel.ResolvedRoute
	defaultRoute *channel.ResolvedRoute
	fail         map[string]error
}

func (r *textTaskRouteResolverStub) ResolveRoute(_ context.Context, input channel.ResolveRouteInput) (*channel.ResolvedRoute, error) {
	if err := r.fail[input.PlatformModelName]; err != nil {
		return nil, err
	}
	route := r.routes[input.PlatformModelName]
	if route == nil {
		return nil, errors.New("route not found")
	}
	return route, nil
}

func (r *textTaskRouteResolverStub) ResolveDefaultRoute(context.Context, channel.ResolveRouteInput) (*channel.ResolvedRoute, error) {
	if r.defaultRoute == nil {
		return nil, errors.New("default route not found")
	}
	return r.defaultRoute, nil
}

func (r *textTaskRouteResolverStub) MarkRouteFailure(context.Context, *channel.ResolvedRoute, error) {
}

func (r *textTaskRouteResolverStub) MarkRouteSuccess(context.Context, *channel.ResolvedRoute) {}

func TestResolveTextTaskRouteCandidatesFollowUsesCurrentThenDefault(t *testing.T) {
	service := &Service{routeResolver: &textTaskRouteResolverStub{
		routes: map[string]*channel.ResolvedRoute{
			"grok-4.3": {PlatformModelName: "grok-4.3", BindingCode: "current", Protocol: "xai_responses", UpstreamModel: "grok-4.3"},
		},
		defaultRoute: &channel.ResolvedRoute{PlatformModelName: "gpt-5-mini", BindingCode: "default", Protocol: "openai_responses", UpstreamModel: "gpt-5-mini"},
	}}

	routes, err := service.resolveTextTaskRouteCandidates(context.Background(), textTaskRouteInput{
		ConfiguredModel: textTaskFollowModel, ConversationModel: "grok-4.3", UserID: 1, ConversationID: 2,
	})
	if err != nil {
		t.Fatalf("resolve candidates: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected current and default routes, got %#v", routes)
	}
	if routes[0].BindingCode != "current" || routes[1].BindingCode != "default" {
		t.Fatalf("unexpected route order: %#v", routes)
	}
}

func TestResolveTextTaskRouteCandidatesSpecifiedModelDoesNotAddDefault(t *testing.T) {
	service := &Service{routeResolver: &textTaskRouteResolverStub{
		routes: map[string]*channel.ResolvedRoute{
			"gpt-5-mini": {PlatformModelName: "gpt-5-mini", BindingCode: "specified", Protocol: "openai_responses", UpstreamModel: "gpt-5-mini"},
		},
		defaultRoute: &channel.ResolvedRoute{PlatformModelName: "fallback", BindingCode: "default", Protocol: "openai_responses", UpstreamModel: "fallback"},
	}}

	routes, err := service.resolveTextTaskRouteCandidates(context.Background(), textTaskRouteInput{
		ConfiguredModel: "gpt-5-mini", ConversationModel: "grok-4.3", UserID: 1, ConversationID: 2,
	})
	if err != nil {
		t.Fatalf("resolve candidates: %v", err)
	}
	if len(routes) != 1 || routes[0].BindingCode != "specified" {
		t.Fatalf("expected only specified route, got %#v", routes)
	}
}

func TestResolveTextTaskRouteCandidatesFollowFallsBackWhenCurrentRouteFails(t *testing.T) {
	service := &Service{routeResolver: &textTaskRouteResolverStub{
		routes: map[string]*channel.ResolvedRoute{},
		fail: map[string]error{
			"grok-4.3": errors.New("current route unavailable"),
		},
		defaultRoute: &channel.ResolvedRoute{PlatformModelName: "gpt-5-mini", BindingCode: "default", Protocol: "openai_responses", UpstreamModel: "gpt-5-mini"},
	}}

	routes, err := service.resolveTextTaskRouteCandidates(context.Background(), textTaskRouteInput{
		ConfiguredModel: textTaskFollowModel, ConversationModel: "grok-4.3", UserID: 1, ConversationID: 2,
	})
	if err != nil {
		t.Fatalf("resolve candidates: %v", err)
	}
	if len(routes) != 1 || routes[0].BindingCode != "default" {
		t.Fatalf("expected default route after current route failure, got %#v", routes)
	}
}

func TestBuildTextTaskGenerateInputAppliesDefaultsAndInstructions(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		BaseURL:               "https://api.openai.com/v1",
		ModelCapabilitiesJSON: `{"defaultOptions":{"reasoning":{"effort":"medium"}}}`,
	}
	input := buildTextTaskGenerateInput(route, config.Config{
		ModelOptionPolicyMode: modelOptionPolicyAllowlist,
		ModelOptionAllowedPaths: `{
			"openai_responses": ["reasoning.effort"]
		}`,
		ModelOptionDeniedPaths: config.DefaultModelOptionDeniedPathsJSON(),
	}, []llm.Message{
		{Role: "system", Content: "summarize carefully"},
		{Role: "user", Content: "hello"},
	})

	if input.Instructions != "summarize carefully" {
		t.Fatalf("expected official Responses instructions, got %q", input.Instructions)
	}
	if len(input.Messages) != 1 || input.Messages[0].Role != "user" {
		t.Fatalf("expected system message to be removed from input, got %#v", input.Messages)
	}
	reasoning := input.Options["reasoning"].(map[string]any)
	if reasoning["effort"] != "medium" {
		t.Fatalf("expected default reasoning effort, got %#v", input.Options)
	}
}

func TestBuildTextTaskGenerateInputInlinesSystemWhenCapabilitiesDisableSystemPrompt(t *testing.T) {
	route := &channel.ResolvedRoute{
		Protocol:              llm.AdapterOpenAIResponses,
		BaseURL:               "https://api.openai.com/v1",
		ModelCapabilitiesJSON: `{"supportsSystemPrompt":false}`,
	}
	input := buildTextTaskGenerateInput(route, config.Config{}, []llm.Message{
		{Role: "system", Content: "title only"},
		{Role: "user", Content: "hello"},
	})

	if input.Instructions != "" {
		t.Fatalf("expected no native instructions for inline-user capability, got %q", input.Instructions)
	}
	if len(input.Messages) != 1 || input.Messages[0].Role != "user" {
		t.Fatalf("expected one inlined user message, got %#v", input.Messages)
	}
	content := input.Messages[0].Content
	if !strings.Contains(content, "<system_instructions>") || !strings.Contains(content, "title only") || !strings.Contains(content, "hello") {
		t.Fatalf("expected system prompt to be inlined into user message, got %q", content)
	}
}
