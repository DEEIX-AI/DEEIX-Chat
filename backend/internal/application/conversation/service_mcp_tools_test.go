package conversation

import (
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

func TestInjectMCPToolGuidanceOnlyAddsPolicy(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "搜索 DEEIX Chat"}}
	runtime := selectedToolRuntime{
		definitions: []llm.ToolDefinition{{
			Name:        "bing_search",
			Description: "搜索网页",
			InputSchema: []byte(`{"type":"object","properties":{"query":{"type":"string"},"count":{"type":"number"}},"required":["query"]}`),
		}},
	}

	result := injectMCPToolGuidance(messages, runtime, "")
	if len(result) != 2 {
		t.Fatalf("expected guidance message to be injected, got %#v", result)
	}
	guidance := result[0].Content
	for _, want := range []string{"# tool_use", "declared separately via the API schema", "Use the fewest useful calls"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("expected guidance to contain %q, got %q", want, guidance)
		}
	}
	for _, unwanted := range []string{"# tools", "bing_search", "query:string", "count:number"} {
		if strings.Contains(guidance, unwanted) {
			t.Fatalf("expected guidance not to duplicate tool schema %q, got %q", unwanted, guidance)
		}
	}
}

func TestInjectMCPToolGuidanceUsesCustomPrompt(t *testing.T) {
	messages := []llm.Message{{Role: "user", Content: "搜索 DEEIX Chat"}}
	runtime := selectedToolRuntime{
		definitions: []llm.ToolDefinition{{Name: "bing_search"}},
	}

	result := injectMCPToolGuidance(messages, runtime, "Use MCP tools only after checking user intent.")
	if len(result) != 2 {
		t.Fatalf("expected guidance message to be injected, got %#v", result)
	}
	if result[0].Content != "Use MCP tools only after checking user intent." {
		t.Fatalf("expected custom prompt, got %q", result[0].Content)
	}
}
