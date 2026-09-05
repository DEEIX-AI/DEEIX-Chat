package conversation

import (
	"context"
	"errors"
	"strings"

	appcompact "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/compact"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
	"go.uber.org/zap"
)

const estimatedConversationImageTokens int64 = 1024

const (
	defaultBranchPreviewDepth = 2000
	defaultBranchPreviewLimit = 32
	contextAncestorPageSize   = 256
	contextAncestorMaxCount   = 10000
	contextAncestorMaxBytes   = 64 << 20
)

var errContextAncestorSafetyLimit = errors.New("context ancestor safety limit exceeded")

type messageBranchState struct {
	ExistingMessages []model.Message
	ParentMessageID  *uint
	ParentPublicID   string
	SourceMessageID  *uint
	SourcePublicID   string
	ReuseUserMessage *model.Message
}

func (s *Service) resolveMessageBranch(
	ctx context.Context,
	conversationID uint,
	userID uint,
	parentPublicID string,
	sourcePublicID string,
	branchReason string,
) (*messageBranchState, error) {
	resolveByPublicID := func(publicID string) (*model.Message, error) {
		normalized := strings.TrimSpace(publicID)
		if normalized == "" {
			return nil, nil
		}
		item, findErr := s.repo.GetMessageByPublicID(ctx, conversationID, userID, normalized)
		if findErr != nil {
			return nil, ErrInvalidMessageBranch
		}
		return item, nil
	}

	parentMessage, err := resolveByPublicID(parentPublicID)
	if err != nil {
		return nil, err
	}
	sourceMessage, err := resolveByPublicID(sourcePublicID)
	if err != nil {
		return nil, err
	}

	if sourceMessage != nil {
		if branchReason != "retry" && branchReason != "edit" {
			return nil, ErrInvalidMessageBranch
		}

		expectedParentID := sourceMessage.ParentMessageID
		switch {
		case expectedParentID == nil && parentMessage != nil:
			return nil, ErrInvalidMessageBranch
		case expectedParentID != nil && parentMessage == nil:
			cachedParent, findErr := s.repo.GetMessageByID(ctx, conversationID, *expectedParentID)
			if findErr != nil {
				return nil, ErrInvalidMessageBranch
			}
			parentMessage = cachedParent
		case expectedParentID != nil && parentMessage != nil && parentMessage.ID != *expectedParentID:
			return nil, ErrInvalidMessageBranch
		}
		switch sourceMessage.Role {
		case "user":
			// User retry/edit creates a new user sibling and a fresh assistant child.
		case "assistant":
			if branchReason != "retry" || parentMessage == nil || parentMessage.Role != "user" {
				return nil, ErrInvalidMessageBranch
			}
		default:
			return nil, ErrInvalidMessageBranch
		}
	} else if branchReason != "default" {
		return nil, ErrInvalidMessageBranch
	}

	// 当没有指定 parent 和 source 时，从最近成功上下文里选择默认续聊锚点。
	// 不直接使用最新 DB 行，避免 pending/error 消息或分支 sibling 把上下文带偏。
	if parentMessage == nil && sourceMessage == nil {
		recent, latestErr := s.repo.ListLatestBranchPreviewMessages(
			ctx,
			conversationID,
			defaultBranchPreviewDepth,
			defaultBranchPreviewLimit,
		)
		if latestErr == nil {
			parentMessage = selectLatestDefaultParentCandidate(recent)
		} else {
			s.logger.Warn("list_branch_preview_for_default_branch_failed",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.Uint("conversation_id", conversationID),
				zap.Error(latestErr),
			)
		}
	}

	state := &messageBranchState{
		ExistingMessages: nil,
	}
	if parentMessage != nil {
		state.ParentMessageID = &parentMessage.ID
		state.ParentPublicID = parentMessage.PublicID
	}
	if sourceMessage != nil {
		state.SourceMessageID = &sourceMessage.ID
		state.SourcePublicID = sourceMessage.PublicID
		if sourceMessage.Role == "assistant" {
			userMessage := *parentMessage
			state.ReuseUserMessage = &userMessage
		}
	}
	return state, nil
}

// loadMessageBranchContext hydrates one active ancestor path in bounded pages.
// Branch selection is deliberately kept lightweight; full messages are loaded
// only after the route and latest rolling snapshot are known. A verified
// snapshot boundary stops the scan early, while conversations without a
// snapshot are read to their root so the first compaction never summarizes a
// silently truncated suffix.
func (s *Service) loadMessageBranchContext(
	ctx context.Context,
	conversationID uint,
	branch *messageBranchState,
	snapshot *model.ContextSnapshot,
	branchReason string,
) error {
	if branch == nil || branch.ParentMessageID == nil || *branch.ParentMessageID == 0 {
		return nil
	}

	leafID := *branch.ParentMessageID
	path := make([]model.Message, 0, contextAncestorPageSize)
	loadedCount := 0
	loadedBytes := 0
	boundaryFound := false
	for leafID > 0 {
		page, err := s.repo.ListMessageAncestors(ctx, conversationID, leafID, contextAncestorPageSize)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			break
		}

		pageStart := 0
		if appcompact.SnapshotHasCoverage(snapshot) {
			for index := range page {
				if page[index].ID == snapshot.CoveredUntilMessageID &&
					strings.TrimSpace(page[index].PublicID) == strings.TrimSpace(snapshot.CoveredUntilPublicID) {
					pageStart = index
					boundaryFound = true
					break
				}
			}
		}

		segment := page[pageStart:]
		loadedCount += len(segment)
		loadedBytes += contextMessagesPayloadBytes(segment)
		if loadedCount > contextAncestorMaxCount || loadedBytes > contextAncestorMaxBytes {
			return errContextAncestorSafetyLimit
		}
		path = append(append(make([]model.Message, 0, len(segment)+len(path)), segment...), path...)
		if boundaryFound || len(page) < contextAncestorPageSize || page[0].ParentMessageID == nil {
			break
		}
		leafID = *page[0].ParentMessageID
	}

	if appcompact.SnapshotHasCoverage(snapshot) && !boundaryFound && s.logger != nil {
		s.logger.Warn("context_snapshot_not_on_active_branch",
			zap.String("trace_id", traceid.FromContext(ctx)),
			zap.Uint("conversation_id", conversationID),
			zap.Uint("snapshot_boundary_message_id", snapshot.CoveredUntilMessageID),
		)
	}
	path = recoverAssistantRetryUserStates(path)
	if branchReason == "default" {
		path, _ = normalizeDefaultBranchContext(path, nil)
	}
	branch.ExistingMessages = path
	return nil
}

func contextMessagesPayloadBytes(messages []model.Message) int {
	total := 0
	for _, message := range messages {
		total += len(message.Content)
		total += len(message.ReasoningContent)
		total += len(message.Attachments)
		total += len(message.ErrorMessage)
	}
	return total
}

func normalizeDefaultBranchContext(
	ancestors []model.Message,
	parent *model.Message,
) ([]model.Message, *model.Message) {
	if len(ancestors) == 0 {
		if isContextMessage(parent) {
			return ancestors, parent
		}
		return nil, nil
	}

	contextMessages := recoverAssistantRetryUserStates(ancestors)

	end := len(contextMessages)
	for end > 0 && !isContextMessage(&contextMessages[end-1]) {
		end--
	}
	if end == 0 {
		return nil, nil
	}

	start := 0
	for index := end - 1; index >= 0; index-- {
		if !isContextMessage(&contextMessages[index]) {
			start = index + 1
			break
		}
	}

	normalized := append([]model.Message(nil), contextMessages[start:end]...)
	if len(normalized) == 0 {
		return nil, nil
	}
	nextParent := normalized[len(normalized)-1]
	return normalized, &nextParent
}

// recoverAssistantRetryUserStates makes a reused user message valid context
// after its assistant retry produced usable output. Persisted history remains
// unchanged so the original failed run can still be diagnosed.
func recoverAssistantRetryUserStates(messages []model.Message) []model.Message {
	var recovered []model.Message
	for index := range messages {
		if !isRecoveredAssistantRetryUser(messages, index) {
			continue
		}
		if recovered == nil {
			recovered = append([]model.Message(nil), messages...)
		}
		recovered[index].Status = "success"
		recovered[index].ErrorCode = ""
		recovered[index].ErrorMessage = ""
	}
	if recovered == nil {
		return messages
	}
	return recovered
}

func isRecoveredAssistantRetryUser(messages []model.Message, index int) bool {
	if index < 0 || index+1 >= len(messages) {
		return false
	}
	userMessage := &messages[index]
	retryAssistant := &messages[index+1]
	if userMessage.Role != "user" || !strings.EqualFold(strings.TrimSpace(userMessage.Status), "error") {
		return false
	}
	if retryAssistant.Role != "assistant" ||
		!strings.EqualFold(strings.TrimSpace(retryAssistant.BranchReason), "retry") ||
		retryAssistant.SourceMessageID == nil ||
		!isContextMessage(retryAssistant) {
		return false
	}
	return retryAssistant.ParentMessageID != nil && *retryAssistant.ParentMessageID == userMessage.ID
}

func isContextMessage(item *model.Message) bool {
	if item == nil {
		return false
	}
	status := strings.TrimSpace(item.Status)
	if strings.EqualFold(status, "success") {
		return true
	}
	return item.Role == "assistant" && strings.EqualFold(status, "interrupted")
}

func selectLatestDefaultParentCandidate(messages []model.Message) *model.Message {
	for index := len(messages) - 1; index >= 0; index-- {
		item := messages[index]
		if item.Role == "assistant" && isContextMessage(&item) {
			return &item
		}
	}
	for index := len(messages) - 1; index >= 0; index-- {
		item := messages[index]
		if (item.Role == "user" || item.Role == "system") && isContextMessage(&item) {
			return &item
		}
	}
	return nil
}

// buildBranchMessagePath 使用祖先消息链构建完整活跃分支路径。
func buildBranchMessagePath(branch *messageBranchState, userMessage *model.Message) []model.Message {
	if branch == nil || userMessage == nil {
		return nil
	}
	if branch.ReuseUserMessage != nil {
		return buildMessagePath(branch.ExistingMessages, branch.ReuseUserMessage.ID)
	}
	allMessages := make([]model.Message, 0, len(branch.ExistingMessages)+1)
	allMessages = append(allMessages, branch.ExistingMessages...)
	allMessages = append(allMessages, *userMessage)
	return buildMessagePath(allMessages, userMessage.ID)
}

func conversationImageTokenReserveByMessage(messages []model.Message) map[int]int64 {
	reserve := make(map[int]int64)
	remaining := maxConversationImageContextCount
	for messageIndex := len(messages) - 1; messageIndex >= 0 && remaining > 0; messageIndex-- {
		message := messages[messageIndex]
		if !strings.EqualFold(strings.TrimSpace(message.Role), "user") {
			continue
		}
		refs := parseAttachmentSnapshotRefs(message.Attachments)
		for attachmentIndex := len(refs) - 1; attachmentIndex >= 0 && remaining > 0; attachmentIndex-- {
			ref := refs[attachmentIndex]
			mimeType := textutil.FirstNonEmpty(ref.DetectedMIME, ref.MimeType)
			if normalizeAttachmentKind(ref.Kind, mimeType) != "image" {
				continue
			}
			reserve[messageIndex] += estimatedConversationImageTokens
			remaining--
		}
	}
	return reserve
}

func estimateDomainMessageTokens(message model.Message, includeReasoningContent bool) int64 {
	tokens := tokenestimate.Estimate(message.Content)
	if includeReasoningContent && message.Role == "assistant" {
		tokens += tokenestimate.Estimate(message.ReasoningContent)
	}
	return tokens
}

func buildRAGQuery(contextMessages []model.Message, currentContent string, historyTurns int) string {
	current := strings.TrimSpace(currentContent)
	if historyTurns <= 0 || len(contextMessages) == 0 {
		return current
	}

	recentUserSnippets := make([]string, 0, historyTurns)
	for i := len(contextMessages) - 2; i >= 0 && len(recentUserSnippets) < historyTurns; i-- {
		item := contextMessages[i]
		if item.Role != "user" {
			continue
		}
		snippet := textutil.CompactSnippet(item.Content, 240)
		if snippet == "" {
			continue
		}
		recentUserSnippets = append(recentUserSnippets, snippet)
	}
	if len(recentUserSnippets) == 0 {
		return current
	}
	for left, right := 0, len(recentUserSnippets)-1; left < right; left, right = left+1, right-1 {
		recentUserSnippets[left], recentUserSnippets[right] = recentUserSnippets[right], recentUserSnippets[left]
	}

	var builder strings.Builder
	builder.WriteString(current)
	builder.WriteString("\n\nRecent user context:\n")
	for _, snippet := range recentUserSnippets {
		builder.WriteString("- ")
		builder.WriteString(snippet)
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func buildMessagePath(messages []model.Message, leafID uint) []model.Message {
	if leafID == 0 || len(messages) == 0 {
		return []model.Message{}
	}

	byID := make(map[uint]model.Message, len(messages))
	for _, item := range messages {
		byID[item.ID] = item
	}

	path := make([]model.Message, 0, len(messages))
	visited := make(map[uint]struct{}, len(messages))
	currentID := leafID
	for currentID != 0 {
		item, ok := byID[currentID]
		if !ok {
			break
		}
		if _, seen := visited[currentID]; seen {
			break
		}
		visited[currentID] = struct{}{}
		path = append(path, item)
		if item.ParentMessageID == nil {
			break
		}
		currentID = *item.ParentMessageID
	}

	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return path
}
