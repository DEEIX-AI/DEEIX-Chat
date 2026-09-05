package conversation

import (
	"context"
	"strings"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type messageSendBranchPreparation struct {
	branchState            *messageBranchState
	normalizedBranchReason string
	reuseUserMessage       bool
}

type rejectedMessageState struct {
	errorCode    string
	errorMessage string
}

// prepareMessageSendBranch 在消息落库前解析并规范化分支。
// 正常生成与发送拒绝共用该准备过程，保证重试和编辑的祖先关系一致。
func (s *Service) prepareMessageSendBranch(ctx context.Context, input *SendMessageInput) (*messageSendBranchPreparation, error) {
	if input == nil {
		return nil, ErrInvalidMessageContent
	}

	normalizedBranchReason := normalizeBranchReason(input.BranchReason)
	branchState, err := s.resolveMessageBranch(
		ctx,
		input.ConversationID,
		input.UserID,
		input.ParentMessagePublicID,
		input.SourceMessagePublicID,
		normalizedBranchReason,
	)
	if err != nil {
		return nil, err
	}

	reuseUserMessage := branchState.ReuseUserMessage != nil
	if reuseUserMessage {
		input.Content = branchState.ReuseUserMessage.Content
		input.FileIDs = parseAttachmentSnapshotFileIDs(branchState.ReuseUserMessage.Attachments)
	}
	maxFiles := s.cfg.Snapshot().MaxMessageFiles
	if maxFiles <= 0 {
		maxFiles = 10
	}
	if len(input.FileIDs) > maxFiles {
		return nil, ErrTooManyMessageFiles
	}

	return &messageSendBranchPreparation{
		branchState:            branchState,
		normalizedBranchReason: normalizedBranchReason,
		reuseUserMessage:       reuseUserMessage,
	}, nil
}

type messagePair struct {
	user      *model.Message
	assistant *model.Message
}

// createMessagePair 为已准备的分支原子写入用户消息、助手占位消息与附件引用。
func (s *Service) createMessagePair(
	ctx context.Context,
	input SendMessageInput,
	runID string,
	preparation *messageSendBranchPreparation,
	resolvedAttachments []AttachmentInput,
	rejected *rejectedMessageState,
) (*messagePair, error) {
	if preparation == nil || preparation.branchState == nil {
		return nil, ErrInvalidMessageBranch
	}

	userStatus := "pending"
	assistantStatus := "pending"
	messageErrorCode := ""
	messageErrorMessage := ""
	if rejected != nil {
		userStatus = "error"
		assistantStatus = "error"
		messageErrorCode = strings.TrimSpace(rejected.errorCode)
		messageErrorMessage = textutil.TruncateTrimmed(strings.TrimSpace(rejected.errorMessage), 255)
	}

	userContentEstimatedInputTokens := tokenestimate.Estimate(input.Content)
	if rejected != nil {
		userContentEstimatedInputTokens = 0
	}
	assistantMessage := &model.Message{
		ConversationID:   input.ConversationID,
		UserID:           input.UserID,
		PublicID:         normalizePublicID(uuid.NewString()),
		RunID:            runID,
		Role:             "assistant",
		ContentType:      "text",
		Content:          "",
		BranchReason:     preparation.normalizedBranchReason,
		TokenUsage:       0,
		InputTokens:      0,
		OutputTokens:     0,
		CacheReadTokens:  0,
		CacheWriteTokens: 0,
		ReasoningTokens:  0,
		LatencyMS:        0,
		Status:           assistantStatus,
		ErrorCode:        messageErrorCode,
		ErrorMessage:     messageErrorMessage,
		Attachments:      "[]",
	}

	if preparation.reuseUserMessage {
		reused := *preparation.branchState.ReuseUserMessage
		userMessage := &reused
		assistantMessage.ParentMessageID = &userMessage.ID
		assistantMessage.SourceMessageID = preparation.branchState.SourceMessageID
		if err := s.repo.CreateAssistantBranchMessage(ctx, assistantMessage); err != nil {
			return nil, err
		}
		assistantMessage.ParentPublicID = userMessage.PublicID
		assistantMessage.SourcePublicID = preparation.branchState.SourcePublicID
		return &messagePair{user: userMessage, assistant: assistantMessage}, nil
	}

	attachmentsJSON := []byte(marshalAttachmentSnapshots(resolvedAttachments))
	userMessage := &model.Message{
		ConversationID:  input.ConversationID,
		UserID:          input.UserID,
		PublicID:        normalizePublicID(uuid.NewString()),
		ParentMessageID: preparation.branchState.ParentMessageID,
		RunID:           runID,
		Role:            "user",
		ContentType:     fallbackContentType(input.ContentType),
		Content:         input.Content,
		BranchReason:    preparation.normalizedBranchReason,
		SourceMessageID: preparation.branchState.SourceMessageID,
		TokenUsage:      userContentEstimatedInputTokens,
		InputTokens:     userContentEstimatedInputTokens,
		Attachments:     string(attachmentsJSON),
		Status:          userStatus,
		ErrorCode:       messageErrorCode,
		ErrorMessage:    messageErrorMessage,
	}
	attachmentRows := make([]model.Attachment, 0, len(resolvedAttachments))
	now := time.Now()
	for _, item := range resolvedAttachments {
		attachmentRows = append(attachmentRows, model.Attachment{
			ConversationID: input.ConversationID,
			UserID:         input.UserID,
			FileID:         strings.TrimSpace(item.FileID),
			Kind:           normalizeAttachmentKind(item.Kind, item.MimeType),
			FileName:       strings.TrimSpace(item.FileName),
			MimeType:       strings.TrimSpace(item.MimeType),
			FileSize:       item.FileSize,
			SHA256:         strings.TrimSpace(item.SHA256),
			StoragePath:    strings.TrimSpace(item.StoragePath),
			Status:         "active",
			MetaJSON:       strings.TrimSpace(item.MetaJSON),
			UploadedAt:     now,
		})
	}

	if err := s.repo.CreateMessagePairWithUserAttachments(ctx, userMessage, assistantMessage, attachmentRows); err != nil {
		return nil, err
	}
	userMessage.ParentPublicID = preparation.branchState.ParentPublicID
	userMessage.SourcePublicID = preparation.branchState.SourcePublicID
	assistantMessage.ParentPublicID = userMessage.PublicID
	return &messagePair{user: userMessage, assistant: assistantMessage}, nil
}

// persistRejectedMessageSend 在上游请求开始前保存被业务规则拒绝的对话回合。
// 消息创建和最终错误状态位于同一事务，刷新时不会出现半个回合或永久 pending 消息。
func (s *Service) persistRejectedMessageSend(
	ctx context.Context,
	input SendMessageInput,
	errorCode string,
	errorMessage string,
) (retErr error) {
	if err := s.ValidateSelectedToolIDs(input.SelectedToolIDs); err != nil {
		return err
	}

	conversation, err := s.repo.GetConversationByUser(ctx, input.ConversationID, input.UserID)
	if err != nil {
		return ErrConversationNotFound
	}
	preparation, err := s.prepareMessageSendBranch(ctx, &input)
	if err != nil {
		return err
	}
	resolvedAttachments, err := s.resolveAttachments(ctx, input.UserID, input.FileIDs)
	if err != nil {
		return err
	}

	runID := normalizeRunID(input.ClientRunID)
	if runID == "" {
		runID = "run_" + normalizePublicID(uuid.NewString())
	}
	requestedModelName := strings.TrimSpace(input.PlatformModelName)
	if requestedModelName == "" {
		requestedModelName = strings.TrimSpace(conversation.Model)
	}
	run := &model.Run{
		RunID:              runID,
		RequestID:          strings.TrimSpace(input.RequestID),
		UserID:             input.UserID,
		ConversationID:     input.ConversationID,
		TaskType:           "chat",
		Provider:           strings.TrimSpace(conversation.Provider),
		RequestedModelName: requestedModelName,
		Status:             "running",
		StartedAt:          time.Now(),
	}
	if err = s.claimConversationRun(ctx, run); err != nil {
		return err
	}
	defer func() {
		finalizeCtx, cancel := background.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		endedAt := time.Now()
		run.EndedAt = &endedAt
		run.TotalLatencyMS = endedAt.Sub(run.StartedAt).Milliseconds()
		run.Status = "error"
		if retErr == nil {
			run.ErrorCode = strings.TrimSpace(errorCode)
			run.ErrorMessage = textutil.TruncateTrimmed(strings.TrimSpace(errorMessage), 255)
		} else {
			run.ErrorCode = classifyRunErrorCode(retErr)
			run.ErrorMessage = textutil.TruncateTrimmed(messageErrorSummary(retErr), 255)
		}
		if updateErr := s.repo.UpdateConversationRun(finalizeCtx, run); updateErr != nil {
			if s.logger != nil {
				s.logger.Error("update_rejected_conversation_run_failed",
					zap.String("run_id", run.RunID),
					zap.Error(updateErr),
				)
			}
			if retErr == nil {
				retErr = updateErr
			}
		}
	}()
	pair, err := s.createMessagePair(
		ctx,
		input,
		runID,
		preparation,
		resolvedAttachments,
		&rejectedMessageState{
			errorCode:    errorCode,
			errorMessage: errorMessage,
		},
	)
	if err != nil {
		return err
	}
	s.persistInitialConversationFallbackTitle(ctx, *conversation, *pair.user)
	return nil
}
