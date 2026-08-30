package conversation

import (
	"context"
	"errors"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// DeleteMessageResult 返回消息删除结果。
type DeleteMessageResult struct {
	// ReparentedCount 被重接到被删消息父节点上的子消息数量。
	ReparentedCount int64
}

// DeleteMessage 删除会话中任意位置的一条消息（splice 语义）：被删消息的子消息
// 重接到其父消息上，后续消息保留并向前衔接。会话第一条消息与生成中的消息不允许
// 删除；被删消息覆盖范围内的压缩快照随删除一并失效。
func (s *Service) DeleteMessage(ctx context.Context, userID uint, conversationPublicID string, messagePublicID string) (*DeleteMessageResult, error) {
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
	role := strings.TrimSpace(message.Role)
	if role != "user" && role != "assistant" {
		return nil, ErrMessageDeleteTargetInvalid
	}
	if strings.EqualFold(strings.TrimSpace(message.Status), "pending") {
		return nil, ErrMessageDeleteStateInvalid
	}
	if message.ParentMessageID == nil {
		return nil, ErrMessageDeleteRootInvalid
	}

	reparented, err := s.repo.DeleteMessageAndReparentChildren(ctx, userID, conversation.ID, message.ID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return nil, ErrMessageNotFound
		case errors.Is(err, repository.ErrMessageDeleteStateInvalid):
			return nil, ErrMessageDeleteStateInvalid
		case errors.Is(err, repository.ErrMessageDeleteRootInvalid):
			return nil, ErrMessageDeleteRootInvalid
		default:
			return nil, err
		}
	}
	return &DeleteMessageResult{ReparentedCount: reparented}, nil
}
