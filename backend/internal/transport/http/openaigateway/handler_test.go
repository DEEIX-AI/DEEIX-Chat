package openaigateway

import (
	"encoding/json"
	"testing"
)

func TestFlattenMessageContent(t *testing.T) {
	text, err := flattenMessageContent(json.RawMessage(`"hello"`))
	if err != nil || text != "hello" {
		t.Fatalf("string content = %q err=%v", text, err)
	}
	parts, err := flattenMessageContent(json.RawMessage(`[{"type":"text","text":"a"},{"type":"text","text":"b"}]`))
	if err != nil || parts != "ab" {
		t.Fatalf("parts content = %q err=%v", parts, err)
	}
}

func TestMapOpenAIMessages(t *testing.T) {
	items, err := mapOpenAIMessages([]chatCompletionsMessage{
		{Role: "user", Content: json.RawMessage(`"hi"`)},
		{Role: "assistant", Content: json.RawMessage(`"hello"`)},
	})
	if err != nil {
		t.Fatalf("mapOpenAIMessages() error = %v", err)
	}
	if len(items) != 2 || items[0].Role != "user" || items[0].Content != "hi" {
		t.Fatalf("unexpected messages %#v", items)
	}
}
