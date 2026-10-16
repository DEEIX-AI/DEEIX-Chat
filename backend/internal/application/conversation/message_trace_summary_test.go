package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

func traceSummaryFixture() *model.MessageProcessTrace {
	toolPayload := `{"tool_calls":[{"tool_call_id":"call_1","name":"web_search","status":"success","latency_ms":820,` +
		`"input_preview":"deeix","input_detail":"{\"query\":\"deeix\"}","output_preview":"3 results","output_detail":"` +
		strings.Repeat("result ", 400) + `","output_presentation":{"kind":"search"}}]}`
	think := strings.Repeat("思考", 200)
	return &model.MessageProcessTrace{
		Enabled:       true,
		Status:        messageTraceStatusCompleted,
		Process:       &model.MessageTraceBlock{Title: "处理完成", PayloadJSON: `{"prompt_trace":{"mode":"full"}}`},
		Tools:         &model.MessageTraceBlock{Title: "工具", PayloadJSON: toolPayload},
		UpstreamThink: &model.MessageTraceBlock{Title: "模型思考", ContentMarkdown: think, PayloadJSON: `{"reasoning":{"encrypted_content":"opaque"}}`},
		Events: []model.MessageTraceEvent{
			{EventID: "process", Phase: messageTraceTypeProcess, Stage: messageTraceStageProcess, PayloadJSON: `{"prompt_trace":{"mode":"full"}}`},
			{EventID: "think", Phase: messageTraceTypeUpstreamThink, Stage: messageTraceStageThink, Summary: "思考", ContentMarkdown: think, PayloadJSON: `{"reasoning":{}}`},
			{EventID: "tool", Phase: messageTraceTypeTools, Stage: messageTraceStageTool, ContentMarkdown: "调用 web_search", PayloadJSON: toolPayload},
			{EventID: "legacy-tool", EventType: "tool", PayloadJSON: "not json"},
		},
	}
}

func TestSummarizeMessageProcessTracesKeepsWhatCollapsedRowsNeed(t *testing.T) {
	original := traceSummaryFixture()
	items := []model.Message{{Role: "assistant", Status: "success", ProcessTrace: original}}
	SummarizeMessageProcessTraces(items)
	summary := items[0].ProcessTrace

	if !summary.DetailOmitted || summary.Process != original.Process || summary.PromptTrace != original.PromptTrace {
		t.Fatalf("process block and prompt trace must stay as they are: %#v", summary)
	}
	ids := make([]string, 0, len(summary.Events))
	for _, event := range summary.Events {
		ids = append(ids, event.EventID)
	}
	if strings.Join(ids, ",") != "think,tool,legacy-tool" {
		t.Fatalf("expected the process event to be dropped, got %v", ids)
	}

	think := summary.Events[0]
	if len([]rune(think.ContentMarkdown)) != traceSummaryThinkRunes || think.PayloadJSON != "" || think.Summary != "思考" {
		t.Fatalf("unexpected think event %#v", think)
	}
	if block := summary.UpstreamThink; len([]rune(block.ContentMarkdown)) != traceSummaryThinkRunes || block.PayloadJSON != "" {
		t.Fatalf("unexpected think block %#v", block)
	}

	for _, payload := range []string{summary.Events[1].PayloadJSON, summary.Tools.PayloadJSON} {
		var decoded struct {
			ToolCalls []map[string]any `json:"tool_calls"`
		}
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil || len(decoded.ToolCalls) != 1 {
			t.Fatalf("tool payload no longer decodes: %v %s", err, payload)
		}
		call := decoded.ToolCalls[0]
		for _, key := range traceSummaryToolCallDetailKeys {
			if _, present := call[key]; present {
				t.Fatalf("expected %s to be dropped from %v", key, call)
			}
		}
		if call["tool_call_id"] != "call_1" || call["name"] != "web_search" || call["latency_ms"] != float64(820) || call["input_preview"] != "deeix" {
			t.Fatalf("collapsed row fields were lost: %v", call)
		}
	}
	if summary.Events[1].ContentMarkdown != "调用 web_search" || summary.Events[2].PayloadJSON != "not json" {
		t.Fatalf("tool events kept more than expected or lost data: %#v", summary.Events[1:])
	}

	// 原始轨迹不被改写：同一条消息的完整轨迹还要用于其他响应。
	if original.DetailOmitted || len(original.Events) != 4 || original.UpstreamThink.PayloadJSON == "" || !strings.Contains(original.Tools.PayloadJSON, "output_detail") {
		t.Fatalf("original trace was mutated: %#v", original)
	}
}

func TestSummarizeMessageProcessTracesSkipsUnfinishedMessages(t *testing.T) {
	for _, item := range []model.Message{
		{Role: "assistant", Status: "pending"},
		{Role: "assistant", Status: "error"},
		{Role: "assistant", Status: "interrupted"},
		{Role: "user", Status: "success"},
	} {
		item.ProcessTrace = traceSummaryFixture()
		items := []model.Message{item}
		SummarizeMessageProcessTraces(items)
		if items[0].ProcessTrace.DetailOmitted || len(items[0].ProcessTrace.Events) != 4 {
			t.Fatalf("%s %s message should keep its full trace", item.Role, item.Status)
		}
	}
}

func TestGetMessageProcessTraceReturnsFullTrace(t *testing.T) {
	repo := &assistantEditTraceRepository{message: model.Message{ID: 41, UserID: 9, PublicID: "message_trace", Role: "assistant", Status: "success"}}
	service := &Service{cfg: config.NewRuntime(config.Config{ProcessTraceEnabled: true}), repo: repo}

	trace, err := service.GetMessageProcessTrace(context.Background(), 9, " message_trace ")
	if err != nil {
		t.Fatalf("get trace: %v", err)
	}
	if trace == nil || trace.DetailOmitted || trace.Process == nil || len(trace.Events) != 1 {
		t.Fatalf("expected the full trace, got %#v", trace)
	}

	repo.message.Status = "blocked"
	if trace, err = service.GetMessageProcessTrace(context.Background(), 9, "message_trace"); err != nil || trace != nil {
		t.Fatalf("blocked messages must not expose a trace, got %#v %v", trace, err)
	}
	if _, err = service.GetMessageProcessTrace(context.Background(), 9, " "); !errors.Is(err, ErrMessageNotFound) {
		t.Fatalf("expected not found for an empty id, got %v", err)
	}
}
