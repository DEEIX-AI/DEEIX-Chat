package conversation

import (
	"context"
	"strings"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

type persistMessageGenerationInput struct {
	SendInput                 SendMessageInput
	Conversation              *model.Conversation
	UserMessage               *model.Message
	AssistantMessage          *model.Message
	AssistantText             string
	InputTokens               int64
	CacheReadTokens           int64
	CacheWriteTokens          int64
	OutputTokens              int64
	ReasoningTokens           int64
	AssistantLatency          int64
	ResponseID                string
	StatefulPromptFingerprint string
	ToolCallRows              []model.ToolCall
}

func (s *Service) persistSuccessfulMessageGeneration(ctx context.Context, input persistMessageGenerationInput) error {
	input.UserMessage.InputTokens = input.InputTokens
	input.UserMessage.CacheReadTokens = input.CacheReadTokens
	input.UserMessage.CacheWriteTokens = input.CacheWriteTokens
	input.UserMessage.TokenUsage = input.InputTokens + input.CacheReadTokens + input.CacheWriteTokens
	go func(msgID uint, inputTokens, cacheReadTokens, cacheWriteTokens int64) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.repo.UpdateMessageUsage(bgCtx, msgID, inputTokens, 0, cacheReadTokens, cacheWriteTokens, 0)
	}(input.UserMessage.ID, input.InputTokens, input.CacheReadTokens, input.CacheWriteTokens)

	if err := s.repo.UpdateAssistantMessageCompletion(
		ctx,
		input.AssistantMessage.ID,
		input.AssistantText,
		input.OutputTokens,
		input.ReasoningTokens,
		input.AssistantLatency,
		"success",
		"",
		"",
	); err != nil {
		return err
	}
	input.AssistantMessage.Content = input.AssistantText
	input.AssistantMessage.TokenUsage = input.OutputTokens + input.ReasoningTokens
	input.AssistantMessage.OutputTokens = input.OutputTokens
	input.AssistantMessage.ReasoningTokens = input.ReasoningTokens
	input.AssistantMessage.LatencyMS = input.AssistantLatency
	input.AssistantMessage.Status = "success"

	if len(input.ToolCallRows) > 0 {
		for i := range input.ToolCallRows {
			if input.ToolCallRows[i].ConversationID == 0 {
				input.ToolCallRows[i].ConversationID = input.SendInput.ConversationID
			}
			if input.ToolCallRows[i].UserID == 0 {
				input.ToolCallRows[i].UserID = input.SendInput.UserID
			}
			if input.ToolCallRows[i].MessageID == 0 {
				input.ToolCallRows[i].MessageID = input.AssistantMessage.ID
			}
			if strings.TrimSpace(input.ToolCallRows[i].RunID) == "" {
				input.ToolCallRows[i].RunID = input.AssistantMessage.RunID
			}
		}
		if err := s.repo.CreateConversationToolCalls(ctx, input.ToolCallRows); err != nil {
			return err
		}
		s.persistToolContextArtifacts(ctx, toolContextArtifactInput{
			ConversationID: input.SendInput.ConversationID,
			UserID:         input.SendInput.UserID,
			MessageID:      input.UserMessage.ID,
			RunID:          input.AssistantMessage.RunID,
			Rows:           input.ToolCallRows,
		})
	}

	s.updateStatefulResponseAsync(input.SendInput.ConversationID, input.ResponseID, input.StatefulPromptFingerprint)
	s.maybeGenerateConversationMetadataAsync(*input.Conversation, *input.UserMessage, *input.AssistantMessage)
	s.embedMessagePairAsync(input.SendInput, input.UserMessage, input.AssistantMessage)

	return nil
}

func (s *Service) updateStatefulResponseAsync(conversationID uint, responseID string, promptFingerprint string) {
	respID := strings.TrimSpace(responseID)
	if respID == "" {
		return
	}
	fingerprint := strings.TrimSpace(promptFingerprint)
	if fingerprint == "" {
		return
	}
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.repo.UpdateConversationStatefulResponse(bgCtx, conversationID, respID, fingerprint)
	}()
}

func (s *Service) embedMessagePairAsync(input SendMessageInput, userMessage *model.Message, assistantMessage *model.Message) {
	cfg := s.cfg.Snapshot()
	if !cfg.EmbeddingEnabled || !cfg.MessageEmbeddingEnabled {
		return
	}
	go func() {
		asyncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.embedMessagePair(asyncCtx, input.ConversationID, input.UserID, userMessage, assistantMessage)
	}()
}
