package conversation

import (
	"context"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestExecuteToolCallRejectsToolsNotEnabledForRun(t *testing.T) {
	svc := &Service{}
	_, err := svc.executeToolCall(context.Background(), ExecuteToolInput{
		ToolName:      "memory.upsert",
		ArgumentsJSON: `{"memory_key":"k","value":"v"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "not enabled for this run") {
		t.Fatalf("expected disabled tool error, got %v", err)
	}
}

func TestExecuteAssistantToolCallsStopsWhenToolNotEnabledForRun(t *testing.T) {
	svc := &Service{}
	result := svc.executeAssistantToolCalls(context.Background(), executeAssistantToolCallsInput{
		RunID: "run_1",
		ToolCalls: []llm.ToolCall{{
			ToolCallID:    "toolu_1",
			ToolType:      "function",
			ToolName:      "web_search",
			ArgumentsJSON: `{"query":"weather"}`,
			Status:        "requested",
		}},
	})

	if result.FatalErr == nil || !strings.Contains(result.FatalErr.Error(), "not enabled for this run") {
		t.Fatalf("expected fatal disabled tool error, got %v", result.FatalErr)
	}
	if len(result.Rows) != 1 || result.Rows[0].Status != "error" || result.Rows[0].ToolName != "web_search" {
		t.Fatalf("expected one failed tool row, got %#v", result.Rows)
	}
	if len(result.ToolResults) != 1 || result.ToolResults[0].Status != "error" {
		t.Fatalf("expected failed model tool result, got %#v", result.ToolResults)
	}
}

func TestResolveMaxLLMCallsPerRunRequiresFollowUpRound(t *testing.T) {
	svc := &Service{cfg: config.NewRuntime(config.Config{MCPMaxLLMCallsPerRun: 1})}
	if got := svc.resolveMaxLLMCallsPerRun(); got != 2 {
		t.Fatalf("expected minimum LLM calls per run to be 2, got %d", got)
	}
}
