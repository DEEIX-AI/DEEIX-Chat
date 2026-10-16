package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// traceSummaryThinkRunes 是摘要保留的思考开头长度。客户端按思考文本是否为空决定是否展示该轮，
// 保留开头而不是清空，摘要算出的轮数与耗时才与完整轨迹一致。
const traceSummaryThinkRunes = 80

// traceSummaryToolCallDetailKeys 是工具调用里只在展开后才用到的大字段；预览、状态与耗时保留。
var traceSummaryToolCallDetailKeys = []string{"input", "input_detail", "output", "output_text", "output_detail", "output_presentation"}

// SummarizeMessageProcessTraces 把已成功完成的助手消息的处理轨迹换成摘要，供消息列表使用。
// 历史消息的思考与工具过程默认折叠，全文只在展开时通过 GetMessageProcessTrace 获取。
// 进行中、失败或中断的消息保留完整轨迹：前者仍在流式更新，后者的错误详情（upstream_debug）随轨迹下发。
func SummarizeMessageProcessTraces(items []model.Message) {
	for index := range items {
		item := &items[index]
		if item.Role != "assistant" || !strings.EqualFold(strings.TrimSpace(item.Status), "success") {
			continue
		}
		item.ProcessTrace = summarizeMessageProcessTrace(item.ProcessTrace)
	}
}

func summarizeMessageProcessTrace(trace *model.MessageProcessTrace) *model.MessageProcessTrace {
	if trace == nil || trace.DetailOmitted {
		return trace
	}
	summary := *trace
	summary.DetailOmitted = true
	summary.UpstreamThink = summarizeThinkTraceBlock(trace.UpstreamThink)
	summary.Tools = summarizeToolTraceBlock(trace.Tools)
	summary.Events = make([]model.MessageTraceEvent, 0, len(trace.Events))
	for _, event := range trace.Events {
		switch {
		case isThinkTraceEvent(event):
			event.ContentMarkdown = truncateTraceRunes(event.ContentMarkdown, traceSummaryThinkRunes)
			event.PayloadJSON = ""
		case isToolTraceEvent(event):
			event.PayloadJSON = slimToolTracePayload(event.PayloadJSON)
		default:
			// 客户端只按思考与工具事件绘制过程；处理阶段事件与 Process 块内容相同。
			continue
		}
		summary.Events = append(summary.Events, event)
	}
	return &summary
}

func isThinkTraceEvent(event model.MessageTraceEvent) bool {
	return event.Stage == messageTraceStageThink || event.Phase == messageTraceTypeUpstreamThink || event.EventType == messageTraceStageThink
}

func isToolTraceEvent(event model.MessageTraceEvent) bool {
	return event.Stage == messageTraceStageTool || event.Phase == messageTraceTypeTools || event.EventType == messageTraceStageTool
}

func summarizeThinkTraceBlock(block *model.MessageTraceBlock) *model.MessageTraceBlock {
	if block == nil {
		return nil
	}
	summary := *block
	summary.ContentMarkdown = truncateTraceRunes(block.ContentMarkdown, traceSummaryThinkRunes)
	summary.PayloadJSON = ""
	return &summary
}

func summarizeToolTraceBlock(block *model.MessageTraceBlock) *model.MessageTraceBlock {
	if block == nil {
		return nil
	}
	summary := *block
	summary.PayloadJSON = slimToolTracePayload(block.PayloadJSON)
	return &summary
}

// slimToolTracePayload 去掉每个工具调用的完整输入输出；无法解析的载荷原样返回。
func slimToolTracePayload(payloadJSON string) string {
	if strings.TrimSpace(payloadJSON) == "" {
		return payloadJSON
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return payloadJSON
	}
	rawCalls, ok := payload["tool_calls"]
	if !ok {
		return payloadJSON
	}
	var calls []map[string]json.RawMessage
	if err := json.Unmarshal(rawCalls, &calls); err != nil {
		return payloadJSON
	}
	for _, call := range calls {
		for _, key := range traceSummaryToolCallDetailKeys {
			delete(call, key)
		}
	}
	encodedCalls, err := json.Marshal(calls)
	if err != nil {
		return payloadJSON
	}
	payload["tool_calls"] = encodedCalls
	encoded, err := json.Marshal(payload)
	if err != nil {
		return payloadJSON
	}
	return string(encoded)
}

func truncateTraceRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

// GetMessageProcessTrace 返回当前用户一条助手消息的完整处理轨迹；没有轨迹或被审核拦截时返回 nil。
func (s *Service) GetMessageProcessTrace(ctx context.Context, userID uint, messagePublicID string) (*model.MessageProcessTrace, error) {
	publicID := strings.TrimSpace(messagePublicID)
	if publicID == "" {
		return nil, ErrMessageNotFound
	}
	message, err := s.repo.GetMessageByPublicIDForUser(ctx, userID, publicID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}
	// 与消息列表一致：被拦截的助手消息不下发轨迹。
	if message.Role != "assistant" || strings.EqualFold(strings.TrimSpace(message.Status), "blocked") {
		return nil, nil
	}
	items := []model.Message{*message}
	if err := s.hydrateMessageProcessTraces(ctx, items); err != nil {
		return nil, err
	}
	return items[0].ProcessTrace, nil
}
