package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/mcp"
)

const toolResultModelBudgetChars = 12000

type executeAssistantToolCallsInput struct {
	UserID         uint
	ConversationID uint
	RequestID      string
	RunID          string
	ToolCalls      []llm.ToolCall
	ToolCallLimit  int
	TraceRecorder  *messageTraceRecorder
	ToolNameMap    map[string]string
	MCPConfigs     map[string]mcp.CallConfig
	ToolSchemas    map[string]json.RawMessage
	Ledger         *toolExecutionLedger
}

type executeAssistantToolCallsResult struct {
	Rows              []model.ToolCall
	ToolResults       []llm.ToolResult
	ExecutedToolCalls []llm.ToolCall
	FatalErr          error
}

type toolExecutionRecord struct {
	row    model.ToolCall
	result llm.ToolResult
}

type toolExecutionSlot struct {
	row    model.ToolCall
	result llm.ToolResult
}

type toolExecutionLedger struct {
	records map[string]toolExecutionRecord
}

func newToolExecutionLedger() *toolExecutionLedger {
	return &toolExecutionLedger{records: map[string]toolExecutionRecord{}}
}

func (s *Service) executeAssistantToolCalls(ctx context.Context, input executeAssistantToolCallsInput) executeAssistantToolCallsResult {
	toolCalls := input.ToolCalls
	if input.ToolCallLimit > 0 && len(toolCalls) > input.ToolCallLimit {
		toolCalls = toolCalls[:input.ToolCallLimit]
	}
	if len(toolCalls) == 0 {
		return executeAssistantToolCallsResult{}
	}
	executedToolCalls := append([]llm.ToolCall(nil), toolCalls...)
	if input.TraceRecorder != nil {
		summary, markdown, payload := buildToolTrace(buildRequestedToolCallRows(toolCalls, input.ToolNameMap, input.RunID))
		input.TraceRecorder.syncToolSection(summary, markdown, payload, messageTraceStatusStreaming)
	}

	slots := make([]toolExecutionSlot, len(toolCalls))
	var fatalErr error
	for i, item := range toolCalls {
		modelToolName := strings.TrimSpace(item.ToolName)
		executionToolName := resolveExecutionToolName(modelToolName, input.ToolNameMap)
		row := model.ToolCall{
			RunID:      input.RunID,
			ToolCallID: strings.TrimSpace(item.ToolCallID),
			ToolType:   normalizeToolType(item.ToolType),
			ToolName:   executionToolName,
			Status:     "requested",
			LatencyMS:  0,
			InputJSON:  strings.TrimSpace(item.ArgumentsJSON),
			OutputJSON: "",
			ErrorJSON:  "",
		}

		mcpConfig := resolveMCPConfig(modelToolName, input.MCPConfigs)
		if mcpConfig == nil {
			row.Status = "error"
			row.ErrorJSON = toolNotEnabledForRunMessage(modelToolName)
			slots[i] = toolExecutionSlot{
				row:    row,
				result: buildToolResultForModel(row, modelToolName),
			}
			if fatalErr == nil {
				fatalErr = fmt.Errorf("model requested tool %q, but it is not enabled for this run", modelToolName)
			}
			if input.Ledger != nil {
				input.Ledger.store(row.ToolName, row.InputJSON, toolExecutionRecord{row: row, result: slots[i].result})
			}
			continue
		}

		normalizedInput, validationErr := normalizeToolArguments(row.InputJSON, input.ToolSchemas[modelToolName])
		if validationErr != nil {
			row.Status = "error"
			row.ErrorJSON = validationErr.Error()
			slots[i] = toolExecutionSlot{
				row:    row,
				result: buildToolResultForModel(row, modelToolName),
			}
			if input.Ledger != nil {
				input.Ledger.store(row.ToolName, row.InputJSON, toolExecutionRecord{row: row, result: slots[i].result})
			}
			continue
		}
		row.InputJSON = normalizedInput

		if input.Ledger != nil {
			if previous, ok := input.Ledger.lookup(row.ToolName, row.InputJSON); ok {
				slots[i] = buildRepeatedToolSlot(row, modelToolName, previous)
				continue
			}
		}

		toolStartedAt := time.Now()
		outputJSON, executeErr := s.executeToolCall(ctx, ExecuteToolInput{
			UserID:         input.UserID,
			ConversationID: input.ConversationID,
			RequestID:      strings.TrimSpace(input.RequestID),
			ToolName:       row.ToolName,
			ArgumentsJSON:  row.InputJSON,
			MCPConfig:      mcpConfig,
		})
		row.LatencyMS = time.Since(toolStartedAt).Milliseconds()
		if row.LatencyMS < 0 {
			row.LatencyMS = 0
		}
		if executeErr != nil {
			row.Status = "error"
			row.ErrorJSON = strings.TrimSpace(executeErr.Error())
		} else {
			row.Status = "success"
			row.OutputJSON = strings.TrimSpace(outputJSON)
			if row.OutputJSON == "" {
				row.OutputJSON = "{}"
			}
		}
		result := buildToolResultForModel(row, modelToolName)
		slots[i] = toolExecutionSlot{
			row:    row,
			result: result,
		}
		if input.Ledger != nil {
			input.Ledger.store(row.ToolName, row.InputJSON, toolExecutionRecord{row: row, result: result})
		}
	}

	rows := make([]model.ToolCall, 0, len(slots))
	toolResults := make([]llm.ToolResult, 0, len(slots))
	for _, slot := range slots {
		rows = append(rows, slot.row)
		toolResults = append(toolResults, slot.result)
	}
	if input.TraceRecorder != nil {
		summary, markdown, payload := buildToolTrace(rows)
		input.TraceRecorder.appendToolSection(summary, markdown, payload, messageTraceStatusCompleted)
		input.TraceRecorder.completeTools()
	}
	return executeAssistantToolCallsResult{
		Rows:              rows,
		ToolResults:       toolResults,
		ExecutedToolCalls: executedToolCalls,
		FatalErr:          fatalErr,
	}
}

func toolExecutionHasError(rows []model.ToolCall) bool {
	for _, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.Status), "error") {
			return true
		}
	}
	return false
}

func buildRequestedToolCallRows(toolCalls []llm.ToolCall, toolNameMap map[string]string, runID string) []model.ToolCall {
	rows := make([]model.ToolCall, 0, len(toolCalls))
	for _, item := range toolCalls {
		modelToolName := strings.TrimSpace(item.ToolName)
		rows = append(rows, model.ToolCall{
			RunID:      runID,
			ToolCallID: strings.TrimSpace(item.ToolCallID),
			ToolType:   normalizeToolType(item.ToolType),
			ToolName:   resolveExecutionToolName(modelToolName, toolNameMap),
			Status:     "requested",
			InputJSON:  strings.TrimSpace(item.ArgumentsJSON),
		})
	}
	return rows
}

func buildRepeatedToolSlot(row model.ToolCall, modelToolName string, previous toolExecutionRecord) toolExecutionSlot {
	row.LatencyMS = 0
	switch previous.row.Status {
	case "success", "reused":
		row.Status = "reused"
		row.OutputJSON = previous.row.OutputJSON
		result := previous.result
		result.ToolCallID = row.ToolCallID
		result.ToolName = modelToolName
		result.Status = "success"
		return toolExecutionSlot{
			row:    row,
			result: result,
		}
	default:
		row.Status = "error"
		row.ErrorJSON = "same tool call already failed in this run; adjust arguments, choose another source, or answer from available results"
		return toolExecutionSlot{
			row: row,
			result: llm.ToolResult{
				ToolCallID: row.ToolCallID,
				ToolName:   modelToolName,
				Status:     row.Status,
				Error:      row.ErrorJSON,
			},
		}
	}
}

func buildToolResultForModel(row model.ToolCall, modelToolName string) llm.ToolResult {
	return llm.ToolResult{
		ToolCallID: row.ToolCallID,
		ToolName:   modelToolName,
		OutputJSON: budgetToolOutputForModel(row.OutputJSON, toolResultModelBudgetChars),
		Status:     row.Status,
		Error:      row.ErrorJSON,
	}
}

func toolNotEnabledForRunMessage(toolName string) string {
	name := strings.TrimSpace(toolName)
	if name == "" {
		return "tool is not enabled for this run"
	}
	return fmt.Sprintf("tool %s is not enabled for this run", name)
}

func budgetToolOutputForModel(raw string, maxChars int) string {
	value := strings.TrimSpace(raw)
	if value == "" || maxChars <= 0 || len([]rune(value)) <= maxChars {
		return value
	}
	runes := []rune(value)
	preview := strings.TrimSpace(string(runes[:maxChars]))
	if preview == "" {
		preview = string(runes[:maxChars])
	}
	payload := map[string]interface{}{
		"content": []map[string]string{{
			"type": "text",
			"text": preview + "\n\n[Tool result truncated for model context. The full result is retained in the run trace.]",
		}},
		"structuredContent": map[string]interface{}{
			"truncated_for_model": true,
			"original_chars":      len(runes),
			"preview_chars":       maxChars,
		},
	}
	if encoded, err := json.Marshal(payload); err == nil {
		return string(encoded)
	}
	return preview
}

func (l *toolExecutionLedger) lookup(toolName string, argumentsJSON string) (toolExecutionRecord, bool) {
	if l == nil {
		return toolExecutionRecord{}, false
	}
	record, ok := l.records[toolExecutionKey(toolName, argumentsJSON)]
	return record, ok
}

func (l *toolExecutionLedger) store(toolName string, argumentsJSON string, record toolExecutionRecord) {
	if l == nil {
		return
	}
	l.records[toolExecutionKey(toolName, argumentsJSON)] = record
}

func toolExecutionKey(toolName string, argumentsJSON string) string {
	return strings.ToLower(strings.TrimSpace(toolName)) + "\x00" + canonicalToolArguments(argumentsJSON)
}

func canonicalToolArguments(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "{}"
	}
	var payload interface{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return value
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return value
	}
	return string(normalized)
}

func resolveExecutionToolName(toolName string, toolNameMap map[string]string) string {
	value := strings.TrimSpace(toolName)
	if value == "" {
		return ""
	}
	if mapped := strings.TrimSpace(toolNameMap[value]); mapped != "" {
		return mapped
	}
	return value
}

func resolveMCPConfig(toolName string, configs map[string]mcp.CallConfig) *mcp.CallConfig {
	value := strings.TrimSpace(toolName)
	if value == "" || len(configs) == 0 {
		return nil
	}
	cfg, ok := configs[value]
	if !ok {
		return nil
	}
	return &cfg
}
