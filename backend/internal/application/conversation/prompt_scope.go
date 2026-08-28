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
