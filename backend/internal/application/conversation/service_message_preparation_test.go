package conversation

import (
	"context"
	"errors"
	"testing"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

type rejectedMessageRepositoryStub struct {
	repository.ConversationRepository
	conversation      model.Conversation
	userMessage       *model.Message
	assistantMessage  *model.Message
	attachments       []model.Attachment
	run               *model.Run
	runCreateErr      error
	metadataPatch     repository.ConversationMetadataPatch
	pairCreateCalls   int
	branchCreateCalls int
	runCreateCalls    int
	runUpdateCalls    int
}

func (r *rejectedMessageRepositoryStub) GetConversationByUser(_ context.Context, conversationID uint, userID uint) (*model.Conversation, error) {
	if r.conversation.ID != conversationID || r.conversation.UserID != userID {
		return nil, repository.ErrNotFound
	}
	item := r.conversation
	return &item, nil
}

func (r *rejectedMessageRepositoryStub) ListLatestBranchPreviewMessages(context.Context, uint, int, int) ([]model.Message, error) {
	return nil, nil
}

func (r *rejectedMessageRepositoryStub) CreateConversationRun(_ context.Context, run *model.Run) error {
	r.runCreateCalls++
	if r.runCreateErr != nil {
		return r.runCreateErr
	}
	copy := *run
	r.run = &copy
	return nil
}

func TestPersistMessageUsageRejectionRejectsDuplicateBeforeMessages(t *testing.T) {
	repo := &rejectedMessageRepositoryStub{
		conversation: model.Conversation{ID: 7, UserID: 9, Model: "gpt-test"},
		runCreateErr: repository.ErrDuplicate,
	}
	service := &Service{
		cfg:    config.NewRuntime(config.Config{MaxMessageFiles: 10}),
		repo:   repo,
		logger: zap.NewNop(),
	}

	err := service.PersistMessageUsageRejection(
		context.Background(),
		SendMessageInput{UserID: 9, ConversationID: 7, ClientRunID: "run_duplicate", BranchReason: "default"},
		appbilling.ErrUsageBalanceInsufficient,
	)
	if !errors.Is(err, ErrDuplicateMessageGenerationRun) {
		t.Fatalf("duplicate rejection error = %v", err)
	}
	if repo.pairCreateCalls != 0 || repo.branchCreateCalls != 0 || repo.runUpdateCalls != 0 {
		t.Fatalf("duplicate rejection wrote state: pair=%d branch=%d run_update=%d", repo.pairCreateCalls, repo.branchCreateCalls, repo.runUpdateCalls)
	}
}

func (r *rejectedMessageRepositoryStub) UpdateConversationRun(_ context.Context, run *model.Run) error {
	r.runUpdateCalls++
	copy := *run
	r.run = &copy
	return nil
}

func (r *rejectedMessageRepositoryStub) CreateMessagePairWithUserAttachments(
	_ context.Context,
	userMessage *model.Message,
	assistantMessage *model.Message,
	attachments []model.Attachment,
) error {
	r.pairCreateCalls++
	userMessage.ID = 11
	assistantMessage.ID = 12
	parentID := userMessage.ID
	assistantMessage.ParentMessageID = &parentID
	r.userMessage = userMessage
	r.assistantMessage = assistantMessage
	r.attachments = append([]model.Attachment(nil), attachments...)
	return nil
}

func (r *rejectedMessageRepositoryStub) CreateAssistantBranchMessage(
	_ context.Context,
	assistantMessage *model.Message,
) error {
	r.branchCreateCalls++
	assistantMessage.ID = 13
	r.assistantMessage = assistantMessage
	return nil
}

func (r *rejectedMessageRepositoryStub) ListAllMessages(context.Context, uint) ([]model.Message, error) {
	return []model.Message{*r.userMessage, *r.assistantMessage}, nil
}

func (r *rejectedMessageRepositoryStub) UpdateConversationMetadata(
	_ context.Context,
	_ uint,
	patch repository.ConversationMetadataPatch,
) (*model.Conversation, error) {
	r.metadataPatch = patch
	item := r.conversation
	item.Title = patch.Title
	return &item, nil
}

func TestPersistMessageUsageRejectionStoresStableFailedTurn(t *testing.T) {
	runtimeCfg := config.NewRuntime(config.Config{MaxMessageFiles: 10})
	repo := &rejectedMessageRepositoryStub{
		conversation: model.Conversation{
			ID:       7,
			UserID:   9,
			PublicID: "conversation-7",
			Title:    "New chat",
			Model:    "gpt-test",
		},
	}
	service := &Service{
		cfg:    runtimeCfg,
		repo:   repo,
		logger: zap.NewNop(),
	}

	err := service.PersistMessageUsageRejection(
		context.Background(),
		SendMessageInput{
			UserID:            9,
			ConversationID:    7,
			ContentType:       "text",
			Content:           "keep this failed message",
			PlatformModelName: "gpt-test",
			ClientRunID:       "run_issue_544",
			BranchReason:      "default",
		},
		appbilling.ErrUsageBalanceInsufficient,
	)
	if err != nil {
		t.Fatalf("PersistMessageUsageRejection() error = %v", err)
	}
	if repo.userMessage == nil || repo.assistantMessage == nil {
		t.Fatal("rejected turn did not persist both messages")
	}
	if repo.userMessage.Status != "error" || repo.assistantMessage.Status != "error" {
		t.Fatalf("message statuses = (%q, %q), want both error", repo.userMessage.Status, repo.assistantMessage.Status)
	}
	if repo.userMessage.Content != "keep this failed message" {
		t.Fatalf("user content = %q", repo.userMessage.Content)
	}
	if repo.userMessage.ErrorCode != messageUsageBalanceErrorCode || repo.assistantMessage.ErrorCode != messageUsageBalanceErrorCode {
		t.Fatalf("message error codes = (%q, %q)", repo.userMessage.ErrorCode, repo.assistantMessage.ErrorCode)
	}
	if repo.assistantMessage.ErrorMessage != messageUsageBalanceErrorText {
		t.Fatalf("assistant error message = %q", repo.assistantMessage.ErrorMessage)
	}
	if repo.assistantMessage.ParentMessageID == nil || *repo.assistantMessage.ParentMessageID != repo.userMessage.ID {
		t.Fatal("assistant message is not attached to the rejected user message")
	}
	if repo.pairCreateCalls != 1 || repo.branchCreateCalls != 0 {
		t.Fatalf("repository create calls = pair:%d branch:%d", repo.pairCreateCalls, repo.branchCreateCalls)
	}
	if repo.runCreateCalls != 1 || repo.runUpdateCalls != 1 || repo.run == nil {
		t.Fatalf("run persistence calls = create:%d update:%d run:%#v", repo.runCreateCalls, repo.runUpdateCalls, repo.run)
	}
	if repo.run.Status != "error" || repo.run.ErrorCode != messageUsageBalanceErrorCode || repo.run.EndedAt == nil {
		t.Fatalf("rejected run = %#v", repo.run)
	}
	wantTitle := conversationTitleFromFirstUserMessage("keep this failed message")
	if repo.metadataPatch.Title != wantTitle {
		t.Fatalf("fallback title = %q", repo.metadataPatch.Title)
	}
}

func TestShouldPersistMessageUsageRejection(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "insufficient balance", err: appbilling.ErrUsageBalanceInsufficient, want: true},
		{name: "wrapped insufficient balance", err: errors.Join(errors.New("authorize usage"), appbilling.ErrUsageBalanceInsufficient), want: true},
		{name: "concurrency limit", err: appbilling.ErrUsageConcurrencyLimitExceeded, want: false},
		{name: "reservation conflict", err: appbilling.ErrUsageReservationConflict, want: false},
		{name: "pricing missing", err: appbilling.ErrModelPricingRequired, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldPersistMessageUsageRejection(tt.err); got != tt.want {
				t.Fatalf("shouldPersistMessageUsageRejection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPersistMessageUsageRejectionLeavesRetryableFailuresSideEffectFree(t *testing.T) {
	service := &Service{}
	for _, err := range []error{
		appbilling.ErrUsageConcurrencyLimitExceeded,
		appbilling.ErrUsageReservationConflict,
		appbilling.ErrModelPricingRequired,
	} {
		if persistErr := service.PersistMessageUsageRejection(context.Background(), SendMessageInput{}, err); persistErr != nil {
			t.Fatalf("PersistMessageUsageRejection(%v) error = %v", err, persistErr)
		}
	}
}

func TestCreateRejectedAssistantRetryReusesExistingUserMessage(t *testing.T) {
	repo := &rejectedMessageRepositoryStub{}
	service := &Service{repo: repo}
	existingUser := &model.Message{
		ID:             21,
		ConversationID: 7,
		UserID:         9,
		PublicID:       "msg_existing_user",
		Role:           "user",
		ContentType:    "text",
		Content:        "retry this message",
		Status:         "error",
	}
	sourceAssistantID := uint(22)

	pair, err := service.createMessagePair(
		context.Background(),
		SendMessageInput{UserID: 9, ConversationID: 7, Content: existingUser.Content},
		"run_retry_issue_544",
		&messageSendBranchPreparation{
			branchState: &messageBranchState{
				SourceMessageID:  &sourceAssistantID,
				SourcePublicID:   "msg_failed_assistant",
				ReuseUserMessage: existingUser,
			},
			normalizedBranchReason: "retry",
			reuseUserMessage:       true,
		},
		nil,
		&rejectedMessageState{
			errorCode:    messageUsageBalanceErrorCode,
			errorMessage: messageUsageBalanceErrorText,
		},
	)
	if err != nil {
		t.Fatalf("createMessagePair() error = %v", err)
	}
	if pair.user.ID != existingUser.ID {
		t.Fatalf("user message ID = %d, want reused ID %d", pair.user.ID, existingUser.ID)
	}
	if repo.pairCreateCalls != 0 || repo.branchCreateCalls != 1 {
		t.Fatalf("repository create calls = pair:%d branch:%d", repo.pairCreateCalls, repo.branchCreateCalls)
	}
	if pair.assistant.Status != "error" || pair.assistant.ErrorCode != messageUsageBalanceErrorCode {
		t.Fatalf("assistant rejection state = status:%q code:%q", pair.assistant.Status, pair.assistant.ErrorCode)
	}
	if pair.assistant.ParentMessageID == nil || *pair.assistant.ParentMessageID != existingUser.ID {
		t.Fatal("retry assistant is not attached to the reused user message")
	}
	if pair.assistant.SourceMessageID == nil || *pair.assistant.SourceMessageID != sourceAssistantID {
		t.Fatal("retry assistant does not reference the failed source assistant")
	}
}
