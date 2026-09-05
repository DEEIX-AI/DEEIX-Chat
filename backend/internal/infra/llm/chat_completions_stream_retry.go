package llm

import (
	"context"
	"errors"
	"strings"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// generateChatCompletionsStreamWithAutoUsageFallback 只在首个可观察事件之前兼容不支持
// stream_options.include_usage 的上游；一旦调用方收到事件，禁止重试以避免重复输出。
func (c *Client) generateChatCompletionsStreamWithAutoUsageFallback(
	ctx context.Context,
	route portllm.RouteConfig,
	input portllm.GenerateInput,
	onEvent func(portllm.GenerateStreamEvent) error,
) (*portllm.GenerateOutput, error) {
	emitted := false
	attemptEvent := onEvent
	if onEvent != nil {
		attemptEvent = func(event portllm.GenerateStreamEvent) error {
			emitted = true
			return onEvent(event)
		}
	}
	output, err := c.generateStreamOpenAICompatible(ctx, route, input, attemptEvent)
	if err == nil || emitted || !shouldRetryChatCompletionsWithoutAutoStreamUsage(input.Options, err) {
		return output, err
	}
	retryInput := input
	retryInput.Options = disableChatCompletionsAutoStreamUsage(input.Options)
	return c.generateStreamOpenAICompatible(ctx, route, retryInput, onEvent)
}

func shouldRetryChatCompletionsWithoutAutoStreamUsage(options map[string]any, err error) bool {
	if chatCompletionsStreamUsageExplicit(options) {
		return false
	}
	var upstreamErr *portllm.UpstreamError
	if !errors.As(err, &upstreamErr) {
		return false
	}
	if upstreamErr.StatusCode != 400 && upstreamErr.StatusCode != 422 {
		return false
	}
	detail := strings.ToLower(strings.TrimSpace(upstreamErr.Message + " " + upstreamErr.Body))
	return strings.Contains(detail, "stream_options") || strings.Contains(detail, "include_usage")
}

func chatCompletionsStreamUsageExplicit(options map[string]any) bool {
	streamOptions, ok := options["stream_options"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = streamOptions["include_usage"]
	return ok
}

func disableChatCompletionsAutoStreamUsage(options map[string]any) map[string]any {
	result := cloneMap(options)
	streamOptions := cloneMap(asMap(result["stream_options"]))
	streamOptions["include_usage"] = false
	result["stream_options"] = streamOptions
	return result
}
