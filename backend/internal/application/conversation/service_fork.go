package conversation

import (
	"context"
	"errors"
	"strings"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
)

// forkAncestorMaxDepth 限制 fork 复制的祖先链长度，与仓储层递归 CTE 的上限对齐。
const forkAncestorMaxDepth = 2000

// ForkConversationFromMessage 将会话从开头到指定消息（含）的祖先链复制为一个新会话。
// 新会话保留历史消息的上下文规划、思考和工具展示轨迹，但不携带可执行的生成运行、
// 原始工具审计日志、计费与压缩快照；附件以引用方式复用原文件对象（同用户，不重复
// 占用存储配额）。原会话保持不变。
func (s *Service) ForkConversationFromMessage(ctx context.Context, userID uint, conversationPublicID string, messagePublicID string) (*model.Conversation, error) {
	normalizedConversationID := strings.TrimSpace(conversationPublicID)
	if normalizedConversationID == "" {
		return nil, ErrConversationNotFound
	}
	normalizedMessageID := strings.TrimSpace(messagePublicID)
	if normalizedMessageID == "" {
		return nil, ErrMessageNotFound
	}

	conversation, err := s.repo.GetConversationByPublicID(ctx, normalizedConversationID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}

	message, err := s.repo.GetMessageByPublicIDForUser(ctx, userID, normalizedMessageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}
	if message.ConversationID != conversation.ID {
		return nil, ErrMessageNotFound
	}
	if !strings.EqualFold(strings.TrimSpace(message.Role), "assistant") {
		return nil, ErrMessageForkTargetInvalid
	}
	if strings.EqualFold(strings.TrimSpace(message.Status), "pending") {
		return nil, ErrMessageForkStateInvalid
	}

	// 祖先链 CTE 按 id 升序返回根→目标消息的线性路径，父消息必然先于子消息，
	// 可按顺序逐条克隆并即时回填父 ID 映射。
	path, err := s.repo.ListMessageAncestors(ctx, conversation.ID, message.ID, forkAncestorMaxDepth)
	if err != nil {
		return nil, err
	}
	if len(path) == 0 {
		return nil, ErrMessageNotFound
	}
	if path[len(path)-1].ID != message.ID {
		return nil, ErrMessageForkHistoryIncomplete
	}

	target := &model.Conversation{
		UserID:                userID,
		ProjectID:             conversation.ProjectID,
		PublicID:              normalizePublicID(uuid.NewString()),
		Title:                 conversation.Title,
		LabelsJSON:            conversation.LabelsJSON,
		LabelsManuallyManaged: conversation.LabelsManuallyManaged,
		Model:                 conversation.Model,
		Provider:              conversation.Provider,
		SessionKey:            uuid.NewString(),
		MessageCount:          len(path),
		Status:                "active",
		ContextPolicy:         buildContextPolicyJSON(s.cfg.Snapshot()),
		LastCompactedAt:       nil,
		LastResponseID:        "",
	}
	messages := make([]repository.ForkConversationMessage, 0, len(path))
	seenSourceMessageIDs := make(map[uint]struct{}, len(path))
	for index, sourceMessage := range path {
		if sourceMessage.ID == 0 {
			return nil, ErrMessageForkHistoryIncomplete
		}
		if _, exists := seenSourceMessageIDs[sourceMessage.ID]; exists {
			return nil, ErrMessageForkHistoryIncomplete
		}
		if index == 0 {
			if sourceMessage.ParentMessageID != nil {
				return nil, ErrMessageForkHistoryIncomplete
			}
		} else if sourceMessage.ParentMessageID == nil {
			return nil, ErrMessageForkHistoryIncomplete
		} else if _, exists := seenSourceMessageIDs[*sourceMessage.ParentMessageID]; !exists {
			return nil, ErrMessageForkHistoryIncomplete
		}
		seenSourceMessageIDs[sourceMessage.ID] = struct{}{}
		messages = append(messages, repository.ForkConversationMessage{
			SourceMessageID:       sourceMessage.ID,
			SourceParentMessageID: sourceMessage.ParentMessageID,
			Message:               buildForkedMessage(userID, sourceMessage),
		})
	}
	if err = s.repo.CreateForkedConversation(ctx, repository.CreateForkedConversationInput{
		SourceConversationID: conversation.ID,
		Conversation:         target,
		Messages:             messages,
	}); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}
	target.ProjectPublicID = conversation.ProjectPublicID
	target.ProjectName = conversation.ProjectName
	target.ProjectSystemPrompt = conversation.ProjectSystemPrompt
	return target, nil
}

func buildForkedMessage(
	userID uint,
	source model.Message,
) model.Message {
	contentType := strings.TrimSpace(source.ContentType)
	if contentType == "" {
		contentType = "text"
	}
	return model.Message{
		UserID:           userID,
		PublicID:         normalizePublicID(uuid.NewString()),
		Role:             strings.TrimSpace(source.Role),
		ContentType:      contentType,
		Content:          source.Content,
		ReasoningContent: source.ReasoningContent,
		BranchReason:     "default",
		TokenUsage:       source.TokenUsage,
		InputTokens:      source.InputTokens,
		OutputTokens:     source.OutputTokens,
		CacheReadTokens:  source.CacheReadTokens,
		CacheWriteTokens: source.CacheWriteTokens,
		ReasoningTokens:  source.ReasoningTokens,
		LatencyMS:        source.LatencyMS,
		BilledCurrency:   "USD",
		BilledNanousd:    0,
		PricingSnapshot:  "",
		Status:           normalizeForkedMessageStatus(source.Status),
		ErrorCode:        source.ErrorCode,
		ErrorMessage:     source.ErrorMessage,
		KnowledgeSources: append([]model.MessageKnowledgeSource(nil), source.KnowledgeSources...),
		EditedAt:         source.EditedAt,
	}
}

// normalizeForkedMessageStatus fork 不复制运行记录，pending 消息没有可续传的运行，
// 统一落到 interrupted 保持「可继续/可重试」的语义，避免界面停留在永久加载态。
func normalizeForkedMessageStatus(status string) string {
	trimmed := strings.TrimSpace(status)
	if trimmed == "" {
		return "success"
	}
	if strings.EqualFold(trimmed, "pending") {
		return "interrupted"
	}
	return trimmed
}
