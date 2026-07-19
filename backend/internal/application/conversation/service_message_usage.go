package conversation

import (
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

type messageUsageAccumulator struct {
	observedUsage                   llm.Usage
	estimatedUnobservedInputTokens  int64
	currentCallEstimatedInputTokens int64
}

func (a *messageUsageAccumulator) beginCall(input llm.GenerateInput) {
	a.currentCallEstimatedInputTokens = estimateGenerateInputTokens(input)
}

func (a *messageUsageAccumulator) finishCall(observedInput bool) {
	if observedInput {
		a.currentCallEstimatedInputTokens = 0
		return
	}
	if a.currentCallEstimatedInputTokens <= 0 {
		return
	}
	a.estimatedUnobservedInputTokens += a.currentCallEstimatedInputTokens
	a.currentCallEstimatedInputTokens = 0
}

func (a *messageUsageAccumulator) addObservedUsage(delta llm.Usage) llm.Usage {
	if delta == (llm.Usage{}) {
		return a.observedUsage
	}
	a.observedUsage = addLLMUsage(a.observedUsage, delta)
	if delta.InputTokens > 0 {
		a.currentCallEstimatedInputTokens = 0
	}
	return a.observedUsage
}

func (a *messageUsageAccumulator) setObservedUsage(usage llm.Usage) {
	a.observedUsage = usage
	if usage.InputTokens > 0 {
		a.currentCallEstimatedInputTokens = 0
	}
}

func (a *messageUsageAccumulator) usage() llm.Usage {
	return a.observedUsage
}

func (a *messageUsageAccumulator) interruptedInputTokens() int64 {
	return a.observedUsage.InputTokens + a.estimatedUnobservedInputTokens + a.currentCallEstimatedInputTokens
}

func (a *messageUsageAccumulator) effectiveInputTokens(promptFallback int64) int64 {
	inputTokens := a.observedUsage.InputTokens + a.estimatedUnobservedInputTokens
	if inputTokens > 0 {
		return inputTokens
	}
	if promptFallback > 0 {
		return promptFallback
	}
	return 0
}

func resolveObservedOrEstimatedOutputTokens(observedTokens int64, assistantText string) int64 {
	return resolveObservedOrEstimatedTokens(observedTokens, estimateTokens(assistantText))
}

func resolveObservedOrEstimatedTokens(observedTokens int64, estimatedTokens int64) int64 {
	if observedTokens > 0 {
		return observedTokens
	}
	if estimatedTokens > 0 {
		return estimatedTokens
	}
	return 0
}

func resolveObservedOrHigherEstimatedOutputTokens(observedTokens int64, assistantText string) int64 {
	return resolveObservedOrHigherEstimatedTokens(observedTokens, estimateTokens(assistantText))
}

func resolveObservedOrHigherEstimatedTokens(observedTokens int64, estimatedTokens int64) int64 {
	if estimatedTokens > observedTokens {
		return estimatedTokens
	}
	if observedTokens > 0 {
		return observedTokens
	}
	return 0
}

func estimateGenerateInputTokens(input llm.GenerateInput) int64 {
	tokens := estimatePromptTokens(input.Messages)
	if instructions := strings.TrimSpace(input.Instructions); instructions != "" {
		tokens += estimateTokens(instructions) + 4
	}
	if !input.DisableTools {
		tokens += estimateToolDefinitionTokens(input.Tools)
	}
	return tokens
}

func estimateToolDefinitionTokens(tools []llm.ToolDefinition) int64 {
	if len(tools) == 0 {
		return 0
	}
	var tokens int64 = 2
	for _, tool := range tools {
		tokens += estimateTokens(tool.Name)
		tokens += estimateTokens(tool.Description)
		tokens += estimateTokens(string(tool.InputSchema))
		tokens += 12
	}
	return tokens
}

// resolveToolResultTokenBudget 计算当前用户轮次的全部工具结果可使用的模型输入预算。
// 新批次先使用该上限，回灌前再对同轮全部结果统一分配，不额外透支有效上下文。
func resolveToolResultTokenBudget(
	generateInput llm.GenerateInput,
	messages []llm.Message,
	pendingAssistant llm.Message,
	modelName string,
	capabilitiesJSON string,
) int64 {
	budgetMessages := toolResultPayloadPlaceholders(prioritizeCurrentToolMessages(messages))
	placeholderResults := make([]llm.ToolResult, 0, len(pendingAssistant.ToolCalls))
	for _, call := range pendingAssistant.ToolCalls {
		placeholderResults = append(placeholderResults, llm.ToolResult{
			ToolCallID: call.ToolCallID,
			ToolName:   call.ToolName,
			OutputJSON: "{}",
		})
	}
	budgetMessages = append(
		budgetMessages,
		pendingAssistant,
		llm.Message{Role: "tool", ToolResults: placeholderResults},
	)
	available := int64(llm.EffectiveContextBudgetFromCapabilities(modelName, capabilitiesJSON)) -
		estimateToolFollowUpInputTokens(generateInput, budgetMessages)
	if available < 0 {
		return 0
	}
	return available
}

// rebalanceToolFollowUpResults 在完整工具回灌请求超预算时，统一压缩当前轮的全部工具结果。
func rebalanceToolFollowUpResults(
	generateInput llm.GenerateInput,
	messages []llm.Message,
	modelName string,
	capabilitiesJSON string,
) ([]llm.Message, bool) {
	effectiveBudget := int64(llm.EffectiveContextBudgetFromCapabilities(modelName, capabilitiesJSON))
	if estimateToolFollowUpInputTokens(generateInput, messages) <= effectiveBudget {
		return messages, false
	}

	_, currentUserIndex := toolHistoryBounds(messages)
	if currentUserIndex < 0 {
		return messages, false
	}
	fixedMessages := toolResultPayloadPlaceholders(messages)
	resultBudget := effectiveBudget - estimateToolFollowUpInputTokens(generateInput, fixedMessages)
	if resultBudget < 0 {
		resultBudget = 0
	}

	type resultRef struct {
		messageIndex int
		resultIndex  int
	}
	result := append([]llm.Message(nil), messages...)
	refs := make([]resultRef, 0)
	slots := make([]toolExecutionSlot, 0)
	for messageIndex := currentUserIndex + 1; messageIndex < len(result); messageIndex++ {
		if len(result[messageIndex].ToolResults) == 0 {
			continue
		}
		result[messageIndex].ToolResults = append([]llm.ToolResult(nil), result[messageIndex].ToolResults...)
		for resultIndex, toolResult := range result[messageIndex].ToolResults {
			refs = append(refs, resultRef{messageIndex: messageIndex, resultIndex: resultIndex})
			slots = append(slots, toolExecutionSlot{result: toolResult})
		}
	}
	if len(slots) == 0 {
		return messages, false
	}

	enforceToolResultAggregateBudget(slots, resultBudget)
	changed := false
	for index, ref := range refs {
		if result[ref.messageIndex].ToolResults[ref.resultIndex] != slots[index].result {
			changed = true
			result[ref.messageIndex].ToolResults[ref.resultIndex] = slots[index].result
		}
	}
	if !changed {
		return messages, false
	}
	return result, true
}

// toolResultPayloadPlaceholders 保留工具结果的协议结构，但移除可变正文以计算固定上下文开销。
func toolResultPayloadPlaceholders(messages []llm.Message) []llm.Message {
	result := append([]llm.Message(nil), messages...)
	for messageIndex := range result {
		if len(result[messageIndex].ToolResults) == 0 {
			continue
		}
		placeholders := make([]llm.ToolResult, 0, len(result[messageIndex].ToolResults))
		for _, toolResult := range result[messageIndex].ToolResults {
			placeholders = append(placeholders, llm.ToolResult{
				ToolCallID: toolResult.ToolCallID,
				ToolName:   toolResult.ToolName,
				OutputJSON: "{}",
				Status:     toolResult.Status,
			})
		}
		result[messageIndex].ToolResults = placeholders
	}
	return result
}

// trimToolFollowUpHistory 仅在工具回灌请求超预算时删除最老的完整历史轮次。
func trimToolFollowUpHistory(
	generateInput llm.GenerateInput,
	messages []llm.Message,
	modelName string,
	capabilitiesJSON string,
) ([]llm.Message, bool) {
	effectiveBudget := int64(llm.EffectiveContextBudgetFromCapabilities(modelName, capabilitiesJSON))
	estimatedTokens := estimateToolFollowUpInputTokens(generateInput, messages)
	if estimatedTokens <= effectiveBudget {
		return messages, false
	}

	systemEnd, currentUserIndex := toolHistoryBounds(messages)
	if currentUserIndex <= systemEnd {
		return messages, false
	}
	for cutFrom := systemEnd; cutFrom < currentUserIndex; cutFrom++ {
		estimatedTokens -= estimateMessageTokens(messages[cutFrom])
		nextIndex := cutFrom + 1
		if nextIndex < currentUserIndex && messages[nextIndex].Role != "user" {
			continue
		}
		if estimatedTokens <= effectiveBudget || nextIndex == currentUserIndex {
			trimmed := make([]llm.Message, 0, systemEnd+len(messages)-nextIndex)
			trimmed = append(trimmed, messages[:systemEnd]...)
			trimmed = append(trimmed, messages[nextIndex:]...)
			return trimmed, true
		}
	}
	return messages, false
}

// prioritizeCurrentToolMessages 返回系统指令和当前用户轮次，供工具结果计算最大可用预算。
func prioritizeCurrentToolMessages(messages []llm.Message) []llm.Message {
	systemEnd, currentUserIndex := toolHistoryBounds(messages)
	if currentUserIndex <= systemEnd {
		return append([]llm.Message(nil), messages...)
	}
	result := make([]llm.Message, 0, systemEnd+len(messages)-currentUserIndex)
	result = append(result, messages[:systemEnd]...)
	result = append(result, messages[currentUserIndex:]...)
	return result
}

// toolHistoryBounds 定位系统前缀结束位置和当前轮用户消息。
func toolHistoryBounds(messages []llm.Message) (int, int) {
	systemEnd := 0
	for systemEnd < len(messages) && messages[systemEnd].Role == "system" {
		systemEnd++
	}
	currentUserIndex := -1
	for index := len(messages) - 1; index >= systemEnd; index-- {
		if messages[index].Role == "user" {
			currentUserIndex = index
			break
		}
	}
	return systemEnd, currentUserIndex
}

// estimateToolFollowUpInputTokens 按全量请求形状估算工具回灌输入。
func estimateToolFollowUpInputTokens(generateInput llm.GenerateInput, messages []llm.Message) int64 {
	budgetMessages := messages
	if strings.TrimSpace(generateInput.Instructions) != "" {
		_, budgetMessages = extractOpenAIResponsesInstructions(messages)
	}
	budgetInput := generateInput
	budgetInput.Messages = budgetMessages
	budgetInput.PreviousResponseID = ""
	return estimateGenerateInputTokens(budgetInput)
}
