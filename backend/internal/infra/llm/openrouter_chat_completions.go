package llm

import (
	"context"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// openRouterChatCompletionsAdapter 实现 OpenRouter Chat Completions API。
type openRouterChatCompletionsAdapter struct {
	client *Client
}

func (a *openRouterChatCompletionsAdapter) Name() string { return portllm.AdapterOpenRouterChat }

// Generate 调用 OpenRouter Chat Completions 非流式接口。
func (a *openRouterChatCompletionsAdapter) Generate(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	route = normalizeOpenRouterChatCompletionsRoute(route)
	return a.client.generateOpenAICompatible(ctx, route, input)
}

// GenerateStream 调用 OpenRouter Chat Completions 流式接口。
func (a *openRouterChatCompletionsAdapter) GenerateStream(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput, onEvent func(portllm.GenerateStreamEvent) error) (*portllm.GenerateOutput, error) {
	route = normalizeOpenRouterChatCompletionsRoute(route)
	return a.client.generateChatCompletionsStreamWithAutoUsageFallback(ctx, route, input, onEvent)
}

// ListModels 按 OpenRouter OpenAI-compatible 模型列表协议查询模型。
func (a *openRouterChatCompletionsAdapter) ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	route = normalizeOpenRouterChatCompletionsRoute(route)
	return a.client.listModelsOpenAICompatible(ctx, route)
}

// normalizeOpenRouterChatCompletionsRoute 固定 OpenRouter Chat Completions 的协议和端点。
func normalizeOpenRouterChatCompletionsRoute(route portllm.RouteConfig) portllm.RouteConfig {
	route.Protocol = portllm.AdapterOpenRouterChat
	route.Endpoint = portllm.EndpointChatCompletions
	return route
}
