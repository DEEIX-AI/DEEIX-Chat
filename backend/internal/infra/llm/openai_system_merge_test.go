package llm

import (
	"strings"
	"testing"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestMergeLeadingSystemMessagesMergesLeadingRun(t *testing.T) {
	messages := []portllm.Message{
		{Role: "system", Content: "platform policy"},
		{Role: "system", Content: "# tool_use\n- rules"},
		{Role: "user", Content: "金星和木星"},
	}

	merged := mergeLeadingSystemMessages(messages, false)

	if len(merged) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(merged))
	}
	if merged[0].Role != "system" {
		t.Fatalf("expected merged system first, got %q", merged[0].Role)
	}
	if merged[0].Content != "platform policy\n\n# tool_use\n- rules" {
		t.Fatalf("unexpected merged content: %q", merged[0].Content)
	}
	if merged[1].Role != "user" || merged[1].Content != "金星和木星" {
		t.Fatalf("user message must be untouched, got %+v", merged[1])
	}
}

func TestMergeLeadingSystemMessagesKeepsSingleSystem(t *testing.T) {
	messages := []portllm.Message{
		{Role: "system", Content: "only one"},
		{Role: "user", Content: "hi"},
	}

	merged := mergeLeadingSystemMessages(messages, false)

	if len(merged) != 2 || merged[0].Content != "only one" {
		t.Fatalf("single system run must stay unchanged, got %+v", merged)
	}
}

func TestMergeLeadingSystemMessagesCarriesLastCacheControl(t *testing.T) {
	marker := &portllm.CacheControl{Type: "ephemeral"}
	messages := []portllm.Message{
		{Role: "system", Content: "first"},
		{Role: "system", Content: "second", CacheControl: marker},
		{Role: "user", Content: "hi"},
	}

	merged := mergeLeadingSystemMessages(messages, false)

	if merged[0].CacheControl != marker {
		t.Fatalf("expected cache control carried onto merged message, got %+v", merged[0].CacheControl)
	}
}

func TestMergeLeadingSystemMessagesSkipsStructuredSystem(t *testing.T) {
	messages := []portllm.Message{
		{Role: "system", Content: "plain"},
		{Role: "system", Content: "with parts", Parts: []portllm.ContentPart{{Kind: "text", Text: "x"}}},
		{Role: "user", Content: "hi"},
	}

	merged := mergeLeadingSystemMessages(messages, false)

	if len(merged) != 3 {
		t.Fatalf("structured system run must stay unchanged, got %+v", merged)
	}
}

func TestMergeLeadingSystemMessagesLeavesMidConversationSystem(t *testing.T) {
	messages := []portllm.Message{
		{Role: "system", Content: "first"},
		{Role: "system", Content: "second"},
		{Role: "user", Content: "hi"},
		{Role: "system", Content: "mid"},
		{Role: "user", Content: "again"},
	}

	merged := mergeLeadingSystemMessages(messages, false)

	if len(merged) != 4 {
		t.Fatalf("expected only leading run merged, got %+v", merged)
	}
	if merged[2].Role != "system" || merged[2].Content != "mid" {
		t.Fatalf("mid-conversation system must stay in place, got %+v", merged[2])
	}
}

func TestBuildOpenAIRequestBodySendsSingleLeadingSystem(t *testing.T) {
	input := portllm.GenerateInput{
		Messages: []portllm.Message{
			{Role: "system", Content: "platform policy"},
			{Role: "system", Content: "# tool_use\n- rules"},
			{Role: "user", Content: "金星和木星"},
		},
	}

	payload, err := buildOpenAIRequestBody(portllm.AdapterOpenAIChatCompletions, "qwen3-8b", portllm.EndpointChatCompletions, input, true)
	if err != nil {
		t.Fatalf("buildOpenAIRequestBody: %v", err)
	}
	items, ok := payload["messages"].([]map[string]any)
	if !ok {
		t.Fatalf("expected messages payload, got %#v", payload["messages"])
	}

	systemCount := 0
	for _, item := range items {
		if item["role"] == "system" {
			systemCount++
		}
	}
	if systemCount != 1 {
		t.Fatalf("expected exactly one system message, got %d in %#v", systemCount, items)
	}
	if items[0]["role"] != "system" {
		t.Fatalf("system message must stay first, got %#v", items[0])
	}
	content, _ := items[0]["content"].(string)
	if !strings.Contains(content, "platform policy") || !strings.Contains(content, "# tool_use") {
		t.Fatalf("merged system content missing sources: %q", content)
	}
}

func TestMergeLeadingSystemMessagesKeepsExplicitPromptCacheBreakpoints(t *testing.T) {
	messages := []portllm.Message{
		{Role: "system", Content: "first", CacheControl: &portllm.CacheControl{Type: "ephemeral"}},
		{Role: "system", Content: "second", CacheControl: &portllm.CacheControl{Type: "ephemeral"}},
		{Role: "user", Content: "hi"},
	}

	merged := mergeLeadingSystemMessages(messages, true)

	if len(merged) != 3 {
		t.Fatalf("explicit prompt cache must keep per-message breakpoints, got %+v", merged)
	}
}
