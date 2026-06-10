package llm

import (
	"encoding/json"
	"reflect"
	"testing"
)

func mustDecodeObject(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode usage fixture: %v", err)
	}
	return payload
}

func assertJSONEqual(t *testing.T, actual string, expected string) {
	t.Helper()
	var actualPayload interface{}
	if err := json.Unmarshal([]byte(actual), &actualPayload); err != nil {
		t.Fatalf("decode actual json: %v; raw=%s", err, actual)
	}
	var expectedPayload interface{}
	if err := json.Unmarshal([]byte(expected), &expectedPayload); err != nil {
		t.Fatalf("decode expected json: %v; raw=%s", err, expected)
	}
	if !reflect.DeepEqual(actualPayload, expectedPayload) {
		t.Fatalf("json mismatch:\nactual:   %s\nexpected: %s", actual, expected)
	}
}

func TestParseChatCompletionsUsage(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"usage": {
			"prompt_tokens": 101,
			"completion_tokens": 23,
			"prompt_tokens_details": {"cached_tokens": 17},
			"completion_tokens_details": {"reasoning_tokens": 7}
		}
	}`)

	usage := parseChatStreamUsage(AdapterOpenAIChatCompletions, payload)
	if usage.InputTokens != 84 || usage.OutputTokens != 16 || usage.CacheReadTokens != 17 || usage.ReasoningTokens != 7 {
		t.Fatalf("unexpected chat completions usage: %+v", usage)
	}
	assertJSONEqual(t, usage.RawUsageJSON, `{
		"prompt_tokens": 101,
		"completion_tokens": 23,
		"prompt_tokens_details": {"cached_tokens": 17},
		"completion_tokens_details": {"reasoning_tokens": 7}
	}`)
}

func TestParseChatStreamUsageIgnoresServiceTierOnlyChunks(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"id": "chatcmpl_1",
		"service_tier": "priority",
		"choices": [{"delta": {"content": "hello"}}]
	}`)

	usage := parseChatStreamUsage(AdapterOpenAIChatCompletions, payload)
	if usage != (Usage{}) {
		t.Fatalf("expected no usage for service_tier-only stream chunk, got %+v", usage)
	}
}

func TestParseResponsesUsage(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"service_tier": "flex",
		"usage": {
			"input_tokens": 101,
			"output_tokens": 23,
			"input_tokens_details": {"cached_tokens": 17},
			"output_tokens_details": {"reasoning_tokens": 7}
		}
	}`)

	result := &GenerateOutput{}
	parseResponsesOutput(AdapterOpenAIResponses, payload, result)
	if result.Usage.InputTokens != 84 || result.Usage.OutputTokens != 16 || result.Usage.CacheReadTokens != 17 || result.Usage.ReasoningTokens != 7 || result.Usage.ServiceTier != "flex" {
		t.Fatalf("unexpected responses usage: %+v", result.Usage)
	}
}

func TestParseXAIResponsesUsage(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"usage": {
			"input_tokens": 125,
			"output_tokens": 48,
			"total_tokens": 173,
			"input_tokens_details": {
				"cached_tokens": 98
			},
			"output_tokens_details": {
				"reasoning_tokens": 13
			}
		}
	}`)

	usage := parseOpenAICompatibleUsageForAdapter(AdapterXAIResponses, payload)
	if usage.InputTokens != 27 ||
		usage.OutputTokens != 48 ||
		usage.CacheReadTokens != 98 ||
		usage.CacheWriteTokens != 0 ||
		usage.ReasoningTokens != 13 {
		t.Fatalf("unexpected xai responses usage: %+v", usage)
	}
}

func TestParseXAIChatCompletionsUsage(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"usage": {
			"prompt_tokens": 125,
			"completion_tokens": 48,
			"total_tokens": 173,
			"prompt_tokens_details": {
				"text_tokens": 125,
				"audio_tokens": 0,
				"image_tokens": 0,
				"cached_tokens": 98
			},
			"completion_tokens_details": {
				"reasoning_tokens": 13,
				"audio_tokens": 0,
				"accepted_prediction_tokens": 0,
				"rejected_prediction_tokens": 0
			}
		}
	}`)

	usage := parseOpenAICompatibleUsageForAdapter(AdapterXAIResponses, payload)
	if usage.InputTokens != 27 ||
		usage.OutputTokens != 48 ||
		usage.CacheReadTokens != 98 ||
		usage.CacheWriteTokens != 0 ||
		usage.ReasoningTokens != 13 {
		t.Fatalf("unexpected xai chat completions usage: %+v", usage)
	}
}

func TestParseOpenAICompatibleUsageAliases(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"service_tier": "priority",
		"usage": {
			"input_tokens": 101,
			"output_tokens": 23,
			"input_tokens_details": {
				"cache_read_tokens": 17,
				"cache_write_tokens": 9
			},
			"output_tokens_details": {"reasoning_tokens": 7}
		}
	}`)

	usage := parseOpenAICompatibleUsage(payload)
	if usage.InputTokens != 84 ||
		usage.OutputTokens != 16 ||
		usage.CacheReadTokens != 17 ||
		usage.CacheWriteTokens != 9 ||
		usage.ReasoningTokens != 7 ||
		usage.ServiceTier != "priority" {
		t.Fatalf("unexpected openai-compatible usage aliases: %+v", usage)
	}
}

func TestParseOpenAICompatibleUsageCacheCreationAliases(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"usage": {
			"prompt_tokens": 101,
			"completion_tokens": 23,
			"prompt_tokens_details": {
				"cached_tokens": 17,
				"cache_creation_tokens": 9
			},
			"completion_tokens_details": {"reasoning_tokens": 7}
		}
	}`)

	usage := parseOpenAICompatibleUsage(payload)
	if usage.InputTokens != 84 ||
		usage.OutputTokens != 16 ||
		usage.CacheReadTokens != 17 ||
		usage.CacheWriteTokens != 9 ||
		usage.ReasoningTokens != 7 {
		t.Fatalf("unexpected openai-compatible cache creation aliases: %+v", usage)
	}
}

func TestParseAnthropicMessagesUsage(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"usage": {
			"input_tokens": 101,
			"output_tokens": 23,
			"cache_creation_input_tokens": 11,
			"cache_read_input_tokens": 17,
			"speed": "fast",
			"cache_creation": {
				"ephemeral_1h_input_tokens": 3,
				"ephemeral_5m_input_tokens": 5
			}
		}
	}`)

	usage := parseAnthropicUsage(payload)
	if usage.InputTokens != 101 || usage.OutputTokens != 23 || usage.CacheReadTokens != 17 || usage.CacheWriteTokens != 8 || usage.CacheWrite5mTokens != 5 || usage.CacheWrite1hTokens != 3 || usage.ReasoningTokens != 0 || usage.Speed != "fast" {
		t.Fatalf("unexpected anthropic usage: %+v", usage)
	}
	assertJSONEqual(t, usage.RawUsageJSON, `{
		"input_tokens": 101,
		"output_tokens": 23,
		"cache_creation_input_tokens": 11,
		"cache_read_input_tokens": 17,
		"speed": "fast",
		"cache_creation": {
			"ephemeral_1h_input_tokens": 3,
			"ephemeral_5m_input_tokens": 5
		}
	}`)
}

func TestParseAnthropicMessagesUsageFallsBackToLegacyCacheCreation(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"usage": {
			"input_tokens": 101,
			"output_tokens": 23,
			"cache_creation_input_tokens": 11,
			"cache_read_input_tokens": 17
		}
	}`)

	usage := parseAnthropicUsage(payload)
	if usage.CacheWriteTokens != 11 {
		t.Fatalf("unexpected anthropic legacy cache write usage: %+v", usage)
	}
}

func TestApplyAnthropicStreamEventKeepsSplitRawUsage(t *testing.T) {
	result := &GenerateOutput{}
	start := mustDecodeObject(t, `{
		"type": "message_start",
		"message": {
			"id": "msg_1",
			"usage": {
				"input_tokens": 101,
				"cache_read_input_tokens": 17
			}
		}
	}`)
	if err := applyAnthropicStreamEvent(start, "", result, nil, anthropicToolClassifier{}); err != nil {
		t.Fatalf("apply message_start: %v", err)
	}
	delta := mustDecodeObject(t, `{
		"type": "message_delta",
		"usage": {
			"output_tokens": 23
		}
	}`)
	if err := applyAnthropicStreamEvent(delta, "", result, nil, anthropicToolClassifier{}); err != nil {
		t.Fatalf("apply message_delta: %v", err)
	}
	if result.Usage.InputTokens != 101 || result.Usage.OutputTokens != 23 || result.Usage.CacheReadTokens != 17 {
		t.Fatalf("unexpected anthropic stream usage: %+v", result.Usage)
	}
	assertJSONEqual(t, result.Usage.RawUsageJSON, `[
		{
			"input_tokens": 101,
			"cache_read_input_tokens": 17
		},
		{
			"output_tokens": 23
		}
	]`)
}

func TestParseAnthropicServerSideToolUsage(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"usage": {
			"server_tool_use": {
				"web_search_requests": 2,
				"web_fetch_requests": 1,
				"ignored_requests": 0
			}
		}
	}`)

	usage := parseAnthropicServerSideToolUsage(payload)
	if usage["web_search"] != 2 || usage["web_fetch"] != 1 {
		t.Fatalf("unexpected anthropic server-side tool usage: %#v", usage)
	}
	if _, ok := usage["ignored"]; ok {
		t.Fatalf("expected zero server-side tool usage to be removed, got %#v", usage)
	}
}

func TestParseAnthropicResponseCapturesServerSideToolUsage(t *testing.T) {
	output, err := parseAnthropicResponse([]byte(`{
		"id": "msg_1",
		"content": [
			{"type":"server_tool_use","id":"srv_1","name":"web_search","input":{"query":"today"}},
			{"type":"text","text":"done"}
		],
		"usage": {
			"input_tokens": 10,
			"output_tokens": 5,
			"server_tool_use": {"web_search_requests": 1}
		}
	}`), anthropicToolClassifier{})
	if err != nil {
		t.Fatalf("parse anthropic response: %v", err)
	}
	if output.ServerSideToolUsage["web_search"] != 1 {
		t.Fatalf("expected server-side tool usage on output, got %#v", output.ServerSideToolUsage)
	}
}

func TestParseGoogleGenerateContentUsage(t *testing.T) {
	payload := mustDecodeObject(t, `{
		"usageMetadata": {
			"promptTokenCount": 101,
			"candidatesTokenCount": 23,
			"cachedContentTokenCount": 17,
			"thoughtsTokenCount": 7
		}
	}`)

	usage := parseGeminiUsage(payload)
	if usage.InputTokens != 84 || usage.OutputTokens != 23 || usage.CacheReadTokens != 17 || usage.ReasoningTokens != 7 {
		t.Fatalf("unexpected google generateContent usage: %+v", usage)
	}
	assertJSONEqual(t, usage.RawUsageJSON, `{
		"promptTokenCount": 101,
		"candidatesTokenCount": 23,
		"cachedContentTokenCount": 17,
		"thoughtsTokenCount": 7
	}`)
}

func TestNonCachedInputTokensNeverNegative(t *testing.T) {
	if got := nonCachedInputTokens(12, 20); got != 0 {
		t.Fatalf("expected non-cached input tokens to clamp at zero, got %d", got)
	}
}
