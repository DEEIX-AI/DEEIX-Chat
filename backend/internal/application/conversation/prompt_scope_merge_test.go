package conversation

import (
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestMergeConsecutiveSameRoleMessages(t *testing.T) {
	toolMessage := llm.Message{Role: "tool", ToolResults: []llm.ToolResult{{}}}

	cases := []struct {
		name     string
		input    []llm.Message
		roles    []string
		contents []string
	}{
		{
			name: "consecutive users merge",
			input: []llm.Message{
				{Role: "user", Content: "first"},
				{Role: "user", Content: "second"},
				{Role: "assistant", Content: "answer"},
			},
			roles:    []string{"user", "assistant"},
			contents: []string{"first\n\nsecond", "answer"},
		},
		{
			name: "consecutive assistants merge with reasoning",
			input: []llm.Message{
				{Role: "user", Content: "question"},
				{Role: "assistant", Content: "part one", ReasoningContent: "thinking one"},
				{Role: "assistant", Content: "part two", ReasoningContent: "thinking two"},
			},
			roles:    []string{"user", "assistant"},
			contents: []string{"question", "part one\n\npart two"},
		},
		{
			name: "three consecutive users collapse into one",
			input: []llm.Message{
				{Role: "user", Content: "a"},
				{Role: "user", Content: "b"},
				{Role: "user", Content: "c"},
			},
			roles:    []string{"user"},
			contents: []string{"a\n\nb\n\nc"},
		},
		{
			name: "alternating messages untouched",
			input: []llm.Message{
				{Role: "user", Content: "q1"},
				{Role: "assistant", Content: "a1"},
				{Role: "user", Content: "q2"},
			},
			roles:    []string{"user", "assistant", "user"},
			contents: []string{"q1", "a1", "q2"},
		},
		{
			name: "tool results break the run",
			input: []llm.Message{
				{Role: "user", Content: "q1"},
				{Role: "user", Content: "q2"},
				toolMessage,
			},
			roles:    []string{"user", "tool"},
			contents: []string{"q1\n\nq2", ""},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			merged := mergeConsecutiveSameRoleMessages(testCase.input)
			if len(merged) != len(testCase.roles) {
				t.Fatalf("merged length = %d, want %d", len(merged), len(testCase.roles))
			}
			for index, message := range merged {
				if message.Role != testCase.roles[index] {
					t.Fatalf("merged[%d].role = %q, want %q", index, message.Role, testCase.roles[index])
				}
				if message.Content != testCase.contents[index] {
					t.Fatalf("merged[%d].content = %q, want %q", index, message.Content, testCase.contents[index])
				}
			}
		})
	}
}

func TestMergeConsecutiveSameRoleMessagesMergesParts(t *testing.T) {
	merged := mergeConsecutiveSameRoleMessages([]llm.Message{
		{Role: "user", Content: "look at this"},
		{Role: "user", Parts: []llm.ContentPart{{Kind: llm.ContentPartImage, MimeType: "image/png"}}},
		{Role: "assistant", Content: "done"},
	})
	if len(merged) != 2 {
		t.Fatalf("merged length = %d, want 2", len(merged))
	}
	first := merged[0]
	if len(first.Parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(first.Parts))
	}
	if first.Parts[0].Kind != llm.ContentPartText || !strings.Contains(first.Parts[0].Text, "look at this") {
		t.Fatalf("unexpected first part: %+v", first.Parts[0])
	}
	if first.Parts[1].Kind != llm.ContentPartImage {
		t.Fatalf("unexpected second part: %+v", first.Parts[1])
	}
	if first.Content != "" {
		t.Fatalf("content should be moved into parts, got %q", first.Content)
	}
}
