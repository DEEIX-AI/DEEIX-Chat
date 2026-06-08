package llm

import (
	"context"
	"strconv"
	"strings"
	"unicode"
)

// openRouterResponsesAdapter 实现 OpenRouter Responses API Beta。
// OpenRouter 的历史 input item schema 与官方 OpenAI Responses 有差异：
// assistant message 和 tool output 历史需要稳定的 id/status 字段。
type openRouterResponsesAdapter struct {
	client *Client
}

func (a *openRouterResponsesAdapter) Name() string { return AdapterOpenRouterResponses }

// Generate 调用 OpenRouter Responses 非流式接口。
func (a *openRouterResponsesAdapter) Generate(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error) {
	route = normalizeOpenRouterResponsesRoute(route)
	return a.client.generateOpenAICompatible(ctx, route, input)
}

// GenerateStream 调用 OpenRouter Responses 流式接口。
func (a *openRouterResponsesAdapter) GenerateStream(ctx context.Context, route RouteConfig, input GenerateInput, onEvent func(GenerateStreamEvent) error) (*GenerateOutput, error) {
	route = normalizeOpenRouterResponsesRoute(route)
	return a.client.generateStreamOpenAICompatible(ctx, route, input, onEvent)
}

// ListModels 按 OpenRouter OpenAI-compatible 模型列表协议查询模型。
func (a *openRouterResponsesAdapter) ListModels(ctx context.Context, route RouteConfig) ([]ModelItem, error) {
	route = normalizeOpenRouterResponsesRoute(route)
	return a.client.listModelsOpenAICompatible(ctx, route)
}

// normalizeOpenRouterResponsesRoute 固定 OpenRouter Responses 的协议和端点。
func normalizeOpenRouterResponsesRoute(route RouteConfig) RouteConfig {
	route.Protocol = AdapterOpenRouterResponses
	route.Endpoint = EndpointResponses
	return route
}

// buildOpenRouterResponsesRequestBody 构造 OpenRouter Responses Beta 请求体。
func buildOpenRouterResponsesRequestBody(
	model string,
	input GenerateInput,
	messages []Message,
	providerTools []map[string]interface{},
	toolDefinitions []ToolDefinition,
	providerStreamOptions map[string]interface{},
	stream bool,
) map[string]interface{} {
	payload := map[string]interface{}{
		"model":  strings.TrimSpace(model),
		"input":  buildOpenRouterResponsesAPIInput(messages),
		"stream": stream,
	}
	if maxTokens := modelParamInt(input.Options, "max_output_tokens"); maxTokens > 0 {
		payload["max_output_tokens"] = maxTokens
	}
	applyOpenAICompatibleSamplingParams(payload, input.Options, false)
	applyOpenAIResponsesReasoningParams(payload, input.Options)
	applyOpenAIResponsesTextParams(payload, input.Options, false)
	appendToolDeclarations(payload, providerTools, buildOpenAITools(toolDefinitions, false))
	if streamOptions := responsesStreamOptions(providerStreamOptions); stream && len(streamOptions) > 0 {
		payload["stream_options"] = streamOptions
	}
	applyProviderOptions(payload, input.Options, responsesProtectedProviderOptionKeys(AdapterOpenRouterResponses)...)
	if include := responseIncludeValues(input.Options); len(include) > 0 {
		appendResponseInclude(payload, include...)
	}
	return payload
}

// buildOpenRouterResponsesAPIInput 将内部消息转换为 OpenRouter Responses input items。
func buildOpenRouterResponsesAPIInput(messages []Message) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		if len(msg.ToolCalls) > 0 {
			for _, item := range msg.ToolCalls {
				index := len(items)
				args := strings.TrimSpace(item.ArgumentsJSON)
				if args == "" {
					args = "{}"
				}
				callID := strings.TrimSpace(item.ToolCallID)
				items = append(items, map[string]interface{}{
					"type":      "function_call",
					"id":        openRouterResponsesItemID("fc", index, callID),
					"call_id":   callID,
					"name":      strings.TrimSpace(item.ToolName),
					"arguments": args,
				})
			}
			continue
		}
		if len(msg.ToolResults) > 0 {
			for _, item := range msg.ToolResults {
				index := len(items)
				callID := strings.TrimSpace(item.ToolCallID)
				items = append(items, map[string]interface{}{
					"type":    "function_call_output",
					"id":      openRouterResponsesItemID("fc_output", index, callID),
					"call_id": callID,
					"output":  buildToolResultContent(item),
				})
			}
			continue
		}
		index := len(items)
		role := normalizeRole(msg.Role)
		item := map[string]interface{}{
			"type":    "message",
			"role":    role,
			"content": buildResponsesAPIContent(msg),
		}
		if role == "assistant" {
			item["id"] = openRouterResponsesItemID("msg_deeix", index, msg.Content)
			item["status"] = "completed"
		}
		items = append(items, item)
	}
	return items
}

// openRouterResponsesItemID 为 OpenRouter 历史 item 生成请求内稳定 ID。
func openRouterResponsesItemID(prefix string, index int, seed string) string {
	token := sanitizeOpenRouterResponsesItemID(seed)
	if token == "" {
		token = strconv.Itoa(index + 1)
	}
	return strings.TrimSpace(prefix) + "_" + token
}

// sanitizeOpenRouterResponsesItemID 清理 OpenRouter item id 中不适合直接透传的字符。
func sanitizeOpenRouterResponsesItemID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(builder.String(), "_")
}
