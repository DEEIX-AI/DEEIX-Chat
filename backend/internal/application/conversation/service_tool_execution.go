package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/mcp"
)

type executeAssistantToolCallsInput struct {
	UserID            uint
	ConversationID    uint
	MessageID         uint
	RequestID         string
	RunID             string
	ToolCalls         []llm.ToolCall
	ToolCallLimit     int
	TraceRecorder     *messageTraceRecorder
	ToolNameMap       map[string]string
	MCPConfigs        map[string]mcp.CallConfig
	ToolSchemas       map[string]json.RawMessage
	Ledger            *toolExecutionLedger
	ResultTokenBudget int64
}

type executeAssistantToolCallsResult struct {
	Rows                  []model.ToolCall
	ToolResults           []llm.ToolResult
	ExecutedToolCalls     []llm.ToolCall
	PersistedToolCallKeys map[string]struct{}
	FatalErr              error
}

type toolExecutionRecord struct {
	row    model.ToolCall
	result llm.ToolResult
}

type toolExecutionSlot struct {
	row       model.ToolCall
	result    llm.ToolResult
	persisted bool
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
			MessageID:      input.MessageID,
			ConversationID: input.ConversationID,
			UserID:         input.UserID,
			RunID:          input.RunID,
			ToolCallID:     strings.TrimSpace(item.ToolCallID),
			ToolType:       normalizeToolType(item.ToolType),
			ToolName:       executionToolName,
			Status:         "requested",
			LatencyMS:      0,
			InputJSON:      strings.TrimSpace(item.ArgumentsJSON),
			OutputJSON:     "",
			ErrorJSON:      "",
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
				slot := buildRepeatedToolSlot(row, modelToolName, previous)
				persisted := s.persistToolCallResult(ctx, &slot.row)
				slot.result = buildToolResultForModel(slot.row, modelToolName)
				slot.persisted = persisted
				slots[i] = slot
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
		persisted := s.persistToolCallResult(ctx, &row)
		result := buildToolResultForModel(row, modelToolName)
		slots[i] = toolExecutionSlot{
			row:       row,
			result:    result,
			persisted: persisted,
		}
		if input.Ledger != nil {
			input.Ledger.store(row.ToolName, row.InputJSON, toolExecutionRecord{row: row, result: result})
		}
	}

	rows := make([]model.ToolCall, 0, len(slots))
	toolResults := make([]llm.ToolResult, 0, len(slots))
	persistedToolCallKeys := make(map[string]struct{})
	enforceToolResultAggregateBudget(slots, input.ResultTokenBudget)
	for _, slot := range slots {
		rows = append(rows, slot.row)
		toolResults = append(toolResults, slot.result)
		if slot.persisted {
			persistedToolCallKeys[toolCallPersistenceKey(slot.row)] = struct{}{}
		}
	}
	if input.TraceRecorder != nil {
		summary, markdown, payload := buildToolTrace(rows)
		input.TraceRecorder.appendToolSection(summary, markdown, payload, messageTraceStatusCompleted)
		input.TraceRecorder.completeTools()
	}
	return executeAssistantToolCallsResult{
		Rows:                  rows,
		ToolResults:           toolResults,
		ExecutedToolCalls:     executedToolCalls,
		PersistedToolCallKeys: persistedToolCallKeys,
		FatalErr:              fatalErr,
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

func (s *Service) persistToolCallResult(ctx context.Context, row *model.ToolCall) bool {
	if s == nil || s.repo == nil || row == nil {
		return false
	}
	if err := s.repo.CreateConversationToolCall(ctx, row); err != nil {
		return false
	}
	return row.ID > 0
}

func buildToolResultForModel(row model.ToolCall, modelToolName string) llm.ToolResult {
	return llm.ToolResult{
		ToolCallID: row.ToolCallID,
		ToolName:   modelToolName,
		OutputJSON: modelToolOutputForModel(row.OutputJSON),
		Status:     row.Status,
		Error:      modelToolOutputForModel(row.ErrorJSON),
	}
}

// enforceToolResultAggregateBudget 在模型可用 token 内分配本批工具结果。
// 小结果优先完整保留，剩余预算由较大结果均分，避免单个工具吞掉整批上下文。
func enforceToolResultAggregateBudget(slots []toolExecutionSlot, maxTokens int64) {
	if len(slots) == 0 {
		return
	}
	if maxTokens < 0 {
		maxTokens = 0
	}
	total := int64(0)
	type candidate struct {
		index  int
		tokens int64
	}
	candidates := make([]candidate, 0, len(slots))
	for index, slot := range slots {
		tokens := toolResultModelTokens(slot.result)
		total += tokens
		if tokens <= 0 {
			continue
		}
		candidates = append(candidates, candidate{index: index, tokens: tokens})
	}
	if total <= maxTokens || len(candidates) == 0 {
		return
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].tokens < candidates[j].tokens
	})

	allocations := make(map[int]int64, len(candidates))
	remaining := maxTokens
	for position, item := range candidates {
		count := int64(len(candidates) - position)
		share := remaining / count
		if item.tokens <= share {
			allocations[item.index] = item.tokens
			remaining -= item.tokens
			continue
		}
		for next := position; next < len(candidates); next++ {
			count = int64(len(candidates) - next)
			share = remaining / count
			allocations[candidates[next].index] = share
			remaining -= share
		}
		break
	}

	for index, tokenBudget := range allocations {
		if toolResultModelTokens(slots[index].result) <= tokenBudget {
			continue
		}
		applyToolResultTokenBudget(&slots[index].result, tokenBudget)
	}
}

// toolResultModelTokens 估算单个工具结果实际占用的模型 token。
func toolResultModelTokens(result llm.ToolResult) int64 {
	return estimateTokens(result.OutputJSON) + estimateTokens(result.Error)
}

// applyToolResultTokenBudget 按同一配额约束工具正文和错误信息。
func applyToolResultTokenBudget(result *llm.ToolResult, maxTokens int64) {
	if result == nil {
		return
	}
	outputTokens := estimateTokens(result.OutputJSON)
	errorTokens := estimateTokens(result.Error)
	total := outputTokens + errorTokens
	if total <= maxTokens {
		return
	}
	if outputTokens == 0 {
		result.Error = budgetToolOutputForModel(result.Error, maxTokens)
		return
	}
	if errorTokens == 0 {
		result.OutputJSON = budgetToolOutputForModel(result.OutputJSON, maxTokens)
		return
	}
	outputBudget := maxTokens * outputTokens / total
	result.OutputJSON = budgetToolOutputForModel(result.OutputJSON, outputBudget)
	result.Error = budgetToolOutputForModel(result.Error, maxTokens-outputBudget)
}

func toolNotEnabledForRunMessage(toolName string) string {
	name := strings.TrimSpace(toolName)
	if name == "" {
		return "tool is not enabled for this run"
	}
	return fmt.Sprintf("tool %s is not enabled for this run", name)
}

// modelToolOutputForModel 保留可读文本，并从 JSON 中移除不适合进入模型上下文的不透明载荷。
func modelToolOutputForModel(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if looksLikeOpaqueToolOutput(value) {
		return opaqueToolOutputSummary(len([]rune(value)))
	}
	if len(value) < 1024 {
		return value
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	var payload interface{}
	if err := decoder.Decode(&payload); err != nil {
		return value
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return value
	}
	sanitized, changed := sanitizeOpaqueToolOutputJSON(payload)
	if !changed {
		return value
	}
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		return value
	}
	return string(encoded)
}

// budgetToolOutputForModel 仅在文本超过本批 token 配额时保留头尾片段。
func budgetToolOutputForModel(value string, maxTokens int64) string {
	text := strings.TrimSpace(value)
	if text == "" || estimateTokens(text) <= maxTokens {
		return text
	}
	return headTailToolOutputByTokens(text, maxTokens)
}

// sanitizeOpaqueToolOutputJSON 递归替换 JSON 内的 base64 等大块不透明字符串。
func sanitizeOpaqueToolOutputJSON(value interface{}) (interface{}, bool) {
	switch item := value.(type) {
	case string:
		if looksLikeOpaqueToolOutput(item) {
			return opaqueToolOutputSummary(len([]rune(item))), true
		}
		return item, false
	case []interface{}:
		changed := false
		for index, child := range item {
			sanitized, childChanged := sanitizeOpaqueToolOutputJSON(child)
			if childChanged {
				item[index] = sanitized
			}
			changed = changed || childChanged
		}
		return item, changed
	case map[string]interface{}:
		changed := false
		for key, child := range item {
			sanitized, childChanged := sanitizeOpaqueToolOutputJSON(child)
			if childChanged {
				item[key] = sanitized
			}
			changed = changed || childChanged
		}
		return item, changed
	default:
		return value, false
	}
}

// looksLikeOpaqueToolOutput 识别 data URI 和高密度 base64 风格内容。
func looksLikeOpaqueToolOutput(value string) bool {
	text := strings.TrimSpace(value)
	runes := []rune(text)
	if len(runes) < 1024 {
		return false
	}
	prefix := text
	if len(prefix) > 128 {
		prefix = prefix[:128]
	}
	if strings.HasPrefix(strings.ToLower(prefix), "data:") && strings.Contains(strings.ToLower(prefix), ";base64,") {
		return true
	}
	if strings.ContainsAny(text, " \n\t{}[],:") {
		return false
	}
	base64ish := 0
	for _, r := range runes {
		if (r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '+' || r == '/' || r == '=' || r == '-' || r == '_' {
			base64ish++
		}
	}
	return float64(base64ish)/float64(len(runes)) > 0.95
}

// opaqueToolOutputSummary 为被移除的不透明载荷生成可读说明。
func opaqueToolOutputSummary(originalChars int) string {
	return fmt.Sprintf("[Opaque tool payload omitted from model context: %d characters]", originalChars)
}

// headTailToolOutputByTokens 按项目 token 估算保留文本头尾。
func headTailToolOutputByTokens(value string, maxTokens int64) string {
	text := strings.TrimSpace(value)
	if text == "" || maxTokens <= 0 {
		return ""
	}
	if estimateTokens(text) <= maxTokens {
		return text
	}
	runes := []rune(text)
	marker := fmt.Sprintf("\n\n[... %d characters omitted to fit the model context ...]\n\n", len(runes))
	contentTokens := maxTokens - estimateTokens(marker) - 4
	if contentTokens <= 0 {
		summary := "[Tool result omitted to fit the model context]"
		if estimateTokens(summary) <= maxTokens {
			return summary
		}
		return toolOutputPrefixByTokenBudget(runes, maxTokens)
	}

	headUnits := contentTokens * 6
	tailUnits := contentTokens*12 - headUnits
	headEnd := toolOutputPrefixRuneCount(runes, headUnits)
	tailStart := toolOutputSuffixRuneIndex(runes[headEnd:], tailUnits) + headEnd
	omitted := tailStart - headEnd
	marker = fmt.Sprintf("\n\n[... %d characters omitted to fit the model context ...]\n\n", omitted)
	return strings.TrimSpace(string(runes[:headEnd])) + marker + strings.TrimSpace(string(runes[tailStart:]))
}

// toolOutputPrefixByTokenBudget 在预算不足以容纳截断标记时保留安全前缀。
func toolOutputPrefixByTokenBudget(runes []rune, maxTokens int64) string {
	return strings.TrimSpace(string(runes[:toolOutputPrefixRuneCount(runes, maxTokens*12)]))
}

// toolOutputPrefixRuneCount 返回给定估算单位内可保留的前缀长度。
func toolOutputPrefixRuneCount(runes []rune, maxUnits int64) int {
	used := int64(0)
	for index, r := range runes {
		units := toolOutputRuneTokenUnits(r)
		if used+units > maxUnits {
			return index
		}
		used += units
	}
	return len(runes)
}

// toolOutputSuffixRuneIndex 返回给定估算单位内可保留的后缀起点。
func toolOutputSuffixRuneIndex(runes []rune, maxUnits int64) int {
	used := int64(0)
	for index := len(runes) - 1; index >= 0; index-- {
		units := toolOutputRuneTokenUnits(runes[index])
		if used+units > maxUnits {
			return index + 1
		}
		used += units
	}
	return 0
}

// toolOutputRuneTokenUnits 使用十二分之一 token 表示单个字符的估算权重。
func toolOutputRuneTokenUnits(r rune) int64 {
	if isCJKRune(r) {
		return 8
	}
	return 3
}

func headTailToolOutput(value string, maxChars int) string {
	runes := []rune(strings.TrimSpace(value))
	if maxChars <= 0 || len(runes) <= maxChars {
		return string(runes)
	}
	const separatorTemplate = "\n\n[... %d chars omitted ...]\n\n"
	separator := fmt.Sprintf(separatorTemplate, len(runes)-maxChars)
	separatorChars := len([]rune(separator))
	if separatorChars >= maxChars {
		return strings.TrimSpace(string(runes[:maxChars]))
	}
	available := maxChars - separatorChars
	headChars := available / 2
	tailChars := available - headChars
	head := strings.TrimSpace(string(runes[:headChars]))
	tail := strings.TrimSpace(string(runes[len(runes)-tailChars:]))
	return head + separator + tail
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

func toolCallPersistenceKey(row model.ToolCall) string {
	if value := strings.TrimSpace(row.ToolCallID); value != "" {
		return "id:" + value
	}
	return "tool:" + strings.ToLower(strings.TrimSpace(row.ToolName)) + "\x00" + canonicalToolArguments(row.InputJSON)
}

func mergeToolCallPersistenceKeys(target *map[string]struct{}, source map[string]struct{}) {
	if target == nil || len(source) == 0 {
		return
	}
	if *target == nil {
		*target = make(map[string]struct{}, len(source))
	}
	for key := range source {
		if strings.TrimSpace(key) != "" {
			(*target)[key] = struct{}{}
		}
	}
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
