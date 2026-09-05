package llm

import (
	"context"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// xAIResponsesAdapter 实现 xAI Responses API。请求/响应主体沿用 Responses
// 协议，xAI 自身的 include 和原生工具差异保留在本文件。
type xAIResponsesAdapter struct {
	client *Client
}

func (a *xAIResponsesAdapter) Name() string { return portllm.AdapterXAIResponses }

func (a *xAIResponsesAdapter) Generate(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	return a.generateXAIResponses(ctx, route, input)
}

func (a *xAIResponsesAdapter) GenerateStream(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput, onEvent func(portllm.GenerateStreamEvent) error) (*portllm.GenerateOutput, error) {
	return a.generateStreamXAIResponses(ctx, route, input, onEvent)
}

func (a *xAIResponsesAdapter) ListModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	return a.listXAIModels(ctx, route)
}

func (a *xAIResponsesAdapter) generateXAIResponses(ctx context.Context, route portllm.RouteConfig, input portllm.GenerateInput) (*portllm.GenerateOutput, error) {
	route = normalizeXAIResponsesRoute(route)
	return a.client.generateOpenAICompatible(ctx, route, input)
}

func (a *xAIResponsesAdapter) generateStreamXAIResponses(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	route = normalizeXAIResponsesRoute(route)
	return a.client.generateStreamOpenAICompatible(ctx, route, input, onEvent)
}

func (a *xAIResponsesAdapter) listXAIModels(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	route = normalizeXAIResponsesRoute(route)
	return a.client.listModelsOpenAICompatible(ctx, route)
}

func normalizeXAIResponsesRoute(route portllm.RouteConfig) portllm.RouteConfig {
	route.Protocol = portllm.AdapterXAIResponses
	route.Endpoint = portllm.EndpointResponses
	return route
}

func allResponsesProtocolExtensions() []responsesProtocolExtension {
	return []responsesProtocolExtension{xAIResponsesExtension()}
}

func responsesProtocolExtensionsForAdapter(adapter string) []responsesProtocolExtension {
	extensions := make([]responsesProtocolExtension, 0)
	for _, extension := range allResponsesProtocolExtensions() {
		if extension.matchesAdapter != nil && extension.matchesAdapter(adapter) {
			extensions = append(extensions, extension)
		}
	}
	return extensions
}

func xAIResponsesExtension() responsesProtocolExtension {
	return responsesProtocolExtension{
		matchesAdapter:                  isXAIResponsesAdapter,
		includeDefaults:                 xAIResponsesDefaultIncludeValues,
		serverToolIdentifierKeys:        xAIResponsesServerToolIdentifierKeys,
		serverToolCallID:                xAIResponsesServerToolCallID,
		isServerToolCallItem:            isXAIResponsesNativeCustomToolCall,
		isServerToolCallType:            isXAIResponsesServerToolCallType,
		normalizeServerSideToolUsageKey: normalizeXAIResponsesServerSideToolUsageKey,
	}
}

func isXAIResponsesAdapter(adapter string) bool {
	return portllm.NormalizeAdapter(adapter) == portllm.AdapterXAIResponses
}

func xAIResponsesDefaultIncludeValues(stream bool, tools []map[string]any) []string {
	values := make([]string, 0, 3)
	values = append(values, xAIResponsesToolIncludeValues(tools)...)
	return appendUniqueStrings(nil, values...)
}

func xAIResponsesServerToolIdentifierKeys() []string {
	return []string{"x_search_call_id"}
}

func xAIResponsesToolIncludeValues(tools []map[string]any) []string {
	values := make([]string, 0, 3)
	for _, tool := range tools {
		switch strings.TrimSpace(getString(tool["type"])) {
		case "web_search":
			values = append(values, "web_search_call.action.sources")
		case "file_search":
			values = append(values, "file_search_call.results")
		case "code_interpreter":
			values = append(values, "code_interpreter_call.outputs")
		}
	}
	return appendUniqueStrings(nil, values...)
}

func xAIResponsesServerToolCallID(item map[string]any, itemType string) (string, bool) {
	if itemType == "custom_tool_call" && isXAIResponsesNativeCustomToolName(getString(item["name"])) {
		return textutil.FirstNonEmpty(
			getString(item["item_id"]),
			getString(item["id"]),
			getString(item["call_id"]),
			getString(item["tool_call_id"]),
			getString(item["x_search_call_id"]),
		), true
	}
	if strings.TrimSpace(getString(item["x_search_call_id"])) == "" {
		return "", false
	}
	return textutil.FirstNonEmpty(
		getString(item["item_id"]),
		getString(item["call_id"]),
		getString(item["id"]),
		getString(item["tool_call_id"]),
		getString(item["x_search_call_id"]),
	), true
}

func isXAIResponsesNativeCustomToolCall(item map[string]any) bool {
	return strings.TrimSpace(getString(item["type"])) == "custom_tool_call" &&
		isXAIResponsesNativeCustomToolName(getString(item["name"]))
}

func isXAIResponsesNativeCustomToolName(name string) bool {
	switch strings.TrimSpace(name) {
	case "x_keyword_search", "x_semantic_search":
		return true
	default:
		return false
	}
}

func isXAIResponsesServerToolCallType(itemType string) bool {
	switch strings.TrimSpace(itemType) {
	case "x_search_call", "x_search_call_output":
		return true
	default:
		return false
	}
}

func normalizeXAIResponsesServerSideToolUsageKey(value string, original string) (string, bool) {
	switch strings.TrimSpace(value) {
	case "x_search":
		return "x_search", true
	case "attachment_search", "file_attachment_search", "file_attachments_search":
		return "attachment_search", true
	case "collection_search", "collections_search", "document_search":
		return "collections_search", true
	default:
		return strings.TrimSpace(original), false
	}
}
