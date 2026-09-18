package conversation

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/mcpauth"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/mcp"
)

type capturingMCPClient struct {
	cfg    mcp.CallConfig
	input  mcp.CallInput
	output string
	called bool
}

func (c *capturingMCPClient) CallTool(_ context.Context, cfg mcp.CallConfig, input mcp.CallInput) (string, error) {
	c.called = true
	c.cfg = cfg
	c.input = input
	return c.output, nil
}

func newToolService(secret string, client *capturingMCPClient) *Service {
	return &Service{
		cfg:       config.NewRuntime(config.Config{JWTSecret: "jwt-secret", MCPUserContextSecret: secret}),
		mcpClient: client,
	}
}

// verifyUserContextForTest 在测试内按协议重算 HMAC 并解析 payload，
// 与生产包保持解耦，避免为测试引入生产侧校验函数。
func verifyUserContextForTest(secret string, token string) (mcpauth.Payload, error) {
	parts := strings.SplitN(strings.TrimPrefix(token, "v1."), ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return mcpauth.Payload{}, fmt.Errorf("bad token format")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return mcpauth.Payload{}, fmt.Errorf("bad signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return mcpauth.Payload{}, err
	}
	var payload mcpauth.Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return mcpauth.Payload{}, err
	}
	if time.Now().Unix() >= payload.ExpiresAt {
		return mcpauth.Payload{}, fmt.Errorf("expired")
	}
	return payload, nil
}

func TestExecuteToolCallExpandsSignedUserContextHeader(t *testing.T) {
	client := &capturingMCPClient{output: "ok"}
	svc := newToolService("test-secret", client)
	_, err := svc.executeToolCall(context.Background(), ExecuteToolInput{
		UserID:         42,
		ConversationID: 7,
		RequestID:      "req_1",
		ToolName:       "demo.tool",
		ArgumentsJSON:  `{}`,
		MCPConfig: &mcp.CallConfig{
			BaseURL: "http://127.0.0.1/mcp",
			Headers: map[string]string{
				"X-Static":         "keep",
				mcpauth.HeaderName: mcpauth.TemplateSignedUserContext,
			},
		},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	token := client.cfg.Headers[mcpauth.HeaderName]
	if token == "" || token == mcpauth.TemplateSignedUserContext {
		t.Fatalf("expected signed token, got %q", token)
	}
	payload, err := verifyUserContextForTest("test-secret", token)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if payload.UserID != 42 || payload.ConversationID != 7 || payload.RequestID != "req_1" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if client.cfg.Headers["X-Static"] != "keep" {
		t.Fatalf("static header lost: %#v", client.cfg.Headers)
	}
}

func TestExecuteToolCallLeavesHeadersUntouchedWithoutTemplate(t *testing.T) {
	client := &capturingMCPClient{output: "ok"}
	svc := newToolService("test-secret", client)
	headers := map[string]string{"X-Static": "keep"}
	_, err := svc.executeToolCall(context.Background(), ExecuteToolInput{
		UserID:        42,
		ToolName:      "demo.tool",
		ArgumentsJSON: `{}`,
		MCPConfig:     &mcp.CallConfig{BaseURL: "http://127.0.0.1/mcp", Headers: headers},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if len(client.cfg.Headers) != 1 || client.cfg.Headers["X-Static"] != "keep" {
		t.Fatalf("headers changed unexpectedly: %#v", client.cfg.Headers)
	}
	if len(headers) != 1 || headers["X-Static"] != "keep" {
		t.Fatalf("original headers map mutated: %#v", headers)
	}
}

func TestExecuteToolCallFailsClosedWhenSigningFails(t *testing.T) {
	client := &capturingMCPClient{output: "ok"}
	svc := newToolService("", client)
	_, err := svc.executeToolCall(context.Background(), ExecuteToolInput{
		UserID:        42,
		ToolName:      "demo.tool",
		ArgumentsJSON: `{}`,
		MCPConfig: &mcp.CallConfig{
			BaseURL: "http://127.0.0.1/mcp",
			Headers: map[string]string{mcpauth.HeaderName: mcpauth.TemplateSignedUserContext},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "mcp user context signing failed") {
		t.Fatalf("expected signing failure, got %v", err)
	}
	if client.called {
		t.Fatal("MCP client was called after signing failed")
	}
}

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

func TestExecuteAssistantToolCallsEphemeralSkipsToolCallPersistence(t *testing.T) {
	repo := &temporaryPersistenceRepositoryStub{}
	svc := &Service{repo: repo}
	ledger := newToolExecutionLedger()
	ledger.store("search", `{"query":"privacy"}`, toolExecutionRecord{
		row:    model.ToolCall{ToolName: "search", InputJSON: `{"query":"privacy"}`, OutputJSON: `{"ok":true}`, Status: "success"},
		result: llm.ToolResult{ToolName: "search", OutputJSON: `{"ok":true}`, Status: "success"},
	})

	result := svc.executeAssistantToolCalls(t.Context(), executeAssistantToolCallsInput{
		RunID: "temporary-run",
		ToolCalls: []llm.ToolCall{{
			ToolCallID:    "call-1",
			ToolType:      "function",
			ToolName:      "search",
			ArgumentsJSON: `{"query":"privacy"}`,
		}},
		MCPBindings: map[string]mcpToolCallBinding{"search": {}},
		Ledger:      ledger,
		Ephemeral:   true,
	})

	if len(result.Rows) != 1 || result.Rows[0].Status != "reused" {
		t.Fatalf("expected reused tool result, got %#v", result.Rows)
	}
	if repo.toolCallWrites != 0 {
		t.Fatalf("ephemeral tool call wrote %d persistence rows", repo.toolCallWrites)
	}
}

func TestResolveMaxLLMCallsPerRunRequiresFollowUpRound(t *testing.T) {
	svc := &Service{cfg: config.NewRuntime(config.Config{MCPMaxLLMCallsPerRun: 1})}
	if got := svc.resolveMaxLLMCallsPerRun(); got != 2 {
		t.Fatalf("expected minimum LLM calls per run to be 2, got %d", got)
	}
}

func TestValidateSelectedToolIDsUsesRuntimeLimit(t *testing.T) {
	service := &Service{cfg: config.NewRuntime(config.Config{MCPMaxSelectedToolsPerMessage: 2})}

	if err := service.ValidateSelectedToolIDs([]uint{1, 2}); err != nil {
		t.Fatalf("expected two selected tools to pass, got %v", err)
	}
	if err := service.ValidateSelectedToolIDs([]uint{1, 2, 3}); err != ErrTooManySelectedTools {
		t.Fatalf("expected ErrTooManySelectedTools, got %v", err)
	}
}

func TestDiffLLMUsageTreatsStreamUsageAsCallCumulative(t *testing.T) {
	previous := llm.Usage{
		InputTokens:     10,
		OutputTokens:    3,
		CacheReadTokens: 2,
		ReasoningTokens: 1,
		Speed:           "standard",
		ServiceTier:     "default",
	}
	current := llm.Usage{
		InputTokens:     18,
		OutputTokens:    7,
		CacheReadTokens: 2,
		ReasoningTokens: 4,
		Speed:           "fast",
		ServiceTier:     "priority",
	}

	got := diffLLMUsage(current, previous)
	if got.InputTokens != 8 || got.OutputTokens != 4 || got.CacheReadTokens != 0 || got.ReasoningTokens != 3 {
		t.Fatalf("unexpected usage delta: %#v", got)
	}
	if got.Speed != "fast" || got.ServiceTier != "priority" {
		t.Fatalf("expected latest usage metadata to be kept, got %#v", got)
	}
}

func TestAddServerSideToolUsageAggregatesPositiveCounts(t *testing.T) {
	got := addServerSideToolUsage(
		map[string]int64{"web_search": 1, "ignored": 0},
		map[string]int64{"web_search": 2, "code_interpreter": 1, " ": 3},
	)

	if got["web_search"] != 3 || got["code_interpreter"] != 1 {
		t.Fatalf("unexpected server-side tool usage: %#v", got)
	}
	if _, ok := got["ignored"]; ok {
		t.Fatalf("expected non-positive usage to be ignored: %#v", got)
	}
}

func TestSyncUpstreamOutputThinkingDoesNotReturnThinkingOnlyContent(t *testing.T) {
	output := &llm.GenerateOutput{
		Text: "<think>Need to call a tool.</think>",
		ToolCalls: []llm.ToolCall{{
			ToolCallID:    "call_1",
			ToolType:      "function",
			ToolName:      "memory.list",
			ArgumentsJSON: "{}",
			Status:        "requested",
		}},
	}

	if got := syncUpstreamOutputThinking(nil, output); got != "" {
		t.Fatalf("expected thinking-only tool call content to stay out of assistant text, got %q", got)
	}
}

func TestOutputReasoningContentPrefersStructuredReasoning(t *testing.T) {
	output := &llm.GenerateOutput{
		Reasoning: &llm.ReasoningOutput{Text: "need a tool"},
		Text:      "<think>fallback</think>",
	}

	got := outputReasoningContent(output)
	if got != "need a tool" {
		t.Fatalf("expected structured reasoning content, got %q", got)
	}

	got = outputReasoningContent(&llm.GenerateOutput{Text: "<think>fallback</think>"})
	if got != "fallback" {
		t.Fatalf("expected parsed thinking fallback, got %q", got)
	}
}

func TestToolRunFinalAnswerMissingWhenBudgetEndsWithStructuredToolCall(t *testing.T) {
	output := &llm.GenerateOutput{
		ToolCalls: []llm.ToolCall{{
			ToolCallID:    "call_1",
			ToolType:      "function",
			ToolName:      "search",
			ArgumentsJSON: `{"query":"mcp"}`,
			Status:        "requested",
		}},
	}

	if !toolRunFinalAnswerMissing(output, true, 5, 5, 1) {
		t.Fatalf("expected exhausted tool run with pending tool call to be missing a final answer")
	}
}

func TestToolRunFinalAnswerMissingAcceptsNaturalFinalAnswer(t *testing.T) {
	text := "没有更多工具调用空间时，应基于已获取的结果直接回答。"

	if toolRunFinalAnswerMissing(&llm.GenerateOutput{Text: text}, true, 5, 5, 1) {
		t.Fatalf("expected natural final answer to be accepted")
	}
}
