package conversation

import (
	"strings"

	appcompact "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/compact"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func stringsEqualFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

type promptScope struct {
	FullBranchMessages []model.Message
	CoveredMessages    []model.Message
	RetainedMessages   []model.Message
	Snapshot           *model.ContextSnapshot
	CoveredUntilID     uint
}

func buildPromptScope(messages []model.Message, snapshot *model.ContextSnapshot, policy contextCompactionPolicy) promptScope {
	scope := promptScope{
		FullBranchMessages: append([]model.Message(nil), messages...),
		RetainedMessages:   append([]model.Message(nil), messages...),
	}
	if !policy.EffectiveEnabled() {
		return scope
	}
	boundaryIndex, ok := appcompact.SnapshotBoundaryIndex(messages, snapshot)
	if !ok {
		boundaryIndex, ok = appcompact.SnapshotBoundaryAncestorIndex(messages, snapshot)
	}
	if !ok || boundaryIndex+1 >= len(messages) {
		return scope
	}
	scope.Snapshot = snapshot
	scope.CoveredMessages = append([]model.Message(nil), messages[:boundaryIndex+1]...)
	scope.RetainedMessages = append([]model.Message(nil), messages[boundaryIndex+1:]...)
	scope.CoveredUntilID = snapshot.CoveredUntilMessageID
	return scope
}

func (s promptScope) activeMessages() []model.Message {
	if len(s.RetainedMessages) > 0 {
		return s.RetainedMessages
	}
	return s.FullBranchMessages
}

// estimatePromptScopeTokens mirrors the exact rolling-snapshot scope that is
// eligible for the next upstream request. Keeping this estimate beside
// buildPromptScope prevents the hard-budget preflight from double-counting
// covered history or overlooking the summary and image-token reserve.
func estimatePromptScopeTokens(
	messages []model.Message,
	snapshot *model.ContextSnapshot,
	policy contextCompactionPolicy,
	includeReasoningContent bool,
) int64 {
	scope := buildPromptScope(messages, snapshot, policy)
	activeMessages := scope.activeMessages()
	imageTokenReserve := conversationImageTokenReserveByMessage(activeMessages)
	var total int64
	for index, message := range activeMessages {
		total += estimateDomainMessageTokens(message, includeReasoningContent)
		total += imageTokenReserve[index]
	}
	if scope.Snapshot != nil {
		total += estimateTokens(scope.Snapshot.SummaryText)
	}
	return total
}

func (s promptScope) historicalMessageScope(conversationID uint, userID uint, currentMessageID uint) repository.HistoricalMessageScope {
	if conversationID == 0 || userID == 0 || currentMessageID == 0 {
		return repository.HistoricalMessageScope{}
	}
	messages := s.FullBranchMessages
	if s.Snapshot != nil {
		messages = s.RetainedMessages
	}
	for _, message := range messages {
		if message.ID > 0 && message.ID != currentMessageID {
			return repository.HistoricalMessageScope{
				ConversationID:          conversationID,
				UserID:                  userID,
				LeafMessageID:           currentMessageID,
				ExcludeThroughMessageID: s.CoveredUntilID,
			}
		}
	}
	return repository.HistoricalMessageScope{}
}

type historyMessageOptions struct {
	ReasoningContentPassback bool
}

func historyMessagesFromDomain(messages []model.Message, options historyMessageOptions) []llm.Message {
	historyMsgs := make([]llm.Message, 0, len(messages))
	for _, item := range messages {
		if item.Role != "user" && item.Role != "assistant" && item.Role != "system" {
			continue
		}
		if stringsEqualFold(item.Status, "blocked") {
			continue
		}
		message := llm.Message{
			Role:    item.Role,
			Content: item.Content,
		}
		if options.ReasoningContentPassback && item.Role == "assistant" {
			message.ReasoningContent = item.ReasoningContent
		}
		historyMsgs = append(historyMsgs, message)
	}
	return historyMsgs
}

// mergeConsecutiveSameRoleMessages 合并历史中相邻的同角色消息（仅 user/assistant）。
// 消息删除（splice）会让后续消息向前衔接，历史里可能出现连续同角色轮次；要求严格
// 交替的上游协议会拒收这类请求，这里在出站前做规范化，只影响发送给模型的消息形态，
// 界面展示与落库内容不变。工具调用链路（ToolCalls/ToolResults）与缓存断点不参与合并。
func mergeConsecutiveSameRoleMessages(messages []llm.Message) []llm.Message {
	if len(messages) < 2 {
		return messages
	}
	merged := make([]llm.Message, 0, len(messages))
	for _, message := range messages {
		if last := len(merged) - 1; last >= 0 && mergeableSameRolePair(merged[last], message) {
			merged[last] = mergeSameRoleMessagePair(merged[last], message)
			continue
		}
		merged = append(merged, message)
	}
	return merged
}

func mergeableSameRolePair(left llm.Message, right llm.Message) bool {
	role := strings.ToLower(strings.TrimSpace(left.Role))
	if role != strings.ToLower(strings.TrimSpace(right.Role)) {
		return false
	}
	if role != "user" && role != "assistant" {
		return false
	}
	if len(left.ToolCalls) > 0 || len(left.ToolResults) > 0 ||
		len(right.ToolCalls) > 0 || len(right.ToolResults) > 0 ||
		left.CacheControl != nil || right.CacheControl != nil {
		return false
	}
	return true
}

func mergeSameRoleMessagePair(left llm.Message, right llm.Message) llm.Message {
	merged := left
	merged.Content = joinMessageTextSegments(left.Content, right.Content)
	merged.ReasoningContent = joinMessageTextSegments(left.ReasoningContent, right.ReasoningContent)
	// 双方都是纯文本时保持 Content 承载；任一侧带多模态片段才合并 Parts。
	if len(left.Parts) > 0 || len(right.Parts) > 0 {
		leftParts := messageContentParts(left)
		rightParts := messageContentParts(right)
		merged.Parts = append(append([]llm.ContentPart(nil), leftParts...), rightParts...)
		merged.Content = ""
	}
	return merged
}

// messageContentParts 把消息规整为内容片段列表：多模态消息直接取 Parts（Content 非空时
// 兜底转成首个文本片段，与图片注入的转换约定一致），纯文本消息保持 Content 承载。
func messageContentParts(message llm.Message) []llm.ContentPart {
	if len(message.Parts) > 0 {
		parts := append([]llm.ContentPart(nil), message.Parts...)
		if strings.TrimSpace(message.Content) != "" {
			parts = append([]llm.ContentPart{{Kind: llm.ContentPartText, Text: message.Content}}, parts...)
		}
		return parts
	}
	if strings.TrimSpace(message.Content) == "" {
		return nil
	}
	return []llm.ContentPart{{Kind: llm.ContentPartText, Text: message.Content}}
}

func joinMessageTextSegments(left string, right string) string {
	switch {
	case strings.TrimSpace(left) == "":
		return right
	case strings.TrimSpace(right) == "":
		return left
	default:
		return left + "\n\n" + right
	}
}
