package conversation

import (
	"context"
	"errors"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type deleteMessageRepositoryStub struct {
	repository.ConversationRepository
	conversation *model.Conversation
	message      *model.Message
	deleteCalls  int
	deleteUserID uint
	deleteConvID uint
	deleteMsgID  uint
	deleteResult int64
	deleteErr    error
}

func (s *deleteMessageRepositoryStub) GetConversationByPublicID(context.Context, string, uint) (*model.Conversation, error) {
	return s.conversation, nil
}

func (s *deleteMessageRepositoryStub) GetMessageByPublicIDForUser(context.Context, uint, string) (*model.Message, error) {
	if s.message == nil {
		return nil, repository.ErrNotFound
	}
	return s.message, nil
}

func (s *deleteMessageRepositoryStub) DeleteMessageAndReparentChildren(_ context.Context, userID uint, conversationID uint, messageID uint) (int64, error) {
	s.deleteCalls++
	s.deleteUserID = userID
	s.deleteConvID = conversationID
	s.deleteMsgID = messageID
	if s.deleteErr != nil {
		return 0, s.deleteErr
	}
	return s.deleteResult, nil
}

func newDeleteMessageService(repo repository.ConversationRepository) *Service {
	return &Service{
		cfg:  config.NewRuntime(config.Config{}),
		repo: repo,
	}
}

func parentID(id uint) *uint { return &id }

func TestDeleteMessageSplicesAndReturnsReparentedCount(t *testing.T) {
	repo := &deleteMessageRepositoryStub{
		conversation: &model.Conversation{ID: 10, UserID: 7, PublicID: "conv_delete"},
		message:      &model.Message{ID: 22, UserID: 7, PublicID: "msg_mid", ConversationID: 10, Role: "user", Status: "success", ParentMessageID: parentID(21)},
		deleteResult: 2,
	}
	service := newDeleteMessageService(repo)

	result, err := service.DeleteMessage(context.Background(), 7, "conv_delete", "msg_mid")
	if err != nil {
		t.Fatalf("delete message: %v", err)
	}
	if result.ReparentedCount != 2 {
		t.Fatalf("reparented = %d, want 2", result.ReparentedCount)
	}
	if repo.deleteCalls != 1 || repo.deleteUserID != 7 || repo.deleteConvID != 10 || repo.deleteMsgID != 22 {
		t.Fatalf("unexpected repo call: calls=%d user=%d conv=%d msg=%d", repo.deleteCalls, repo.deleteUserID, repo.deleteConvID, repo.deleteMsgID)
	}
}

func TestDeleteMessageGuards(t *testing.T) {
	base := deleteMessageRepositoryStub{
		conversation: &model.Conversation{ID: 10, UserID: 7, PublicID: "conv_delete"},
		message:      &model.Message{ID: 22, UserID: 7, PublicID: "msg_mid", ConversationID: 10, Role: "user", Status: "success", ParentMessageID: parentID(21)},
	}

	cases := []struct {
		name    string
		mutate  func(stub *deleteMessageRepositoryStub)
		wantErr error
	}{
		{"message not found", func(s *deleteMessageRepositoryStub) { s.message = nil }, ErrMessageNotFound},
		{"message in other conversation", func(s *deleteMessageRepositoryStub) {
			s.message.ConversationID = 99
		}, ErrMessageNotFound},
		{"system role", func(s *deleteMessageRepositoryStub) { s.message.Role = "system" }, ErrMessageDeleteTargetInvalid},
		{"pending", func(s *deleteMessageRepositoryStub) { s.message.Status = "pending" }, ErrMessageDeleteStateInvalid},
		{"root message", func(s *deleteMessageRepositoryStub) { s.message.ParentMessageID = nil }, ErrMessageDeleteRootInvalid},
		{"repo state invalid", func(s *deleteMessageRepositoryStub) { s.deleteErr = repository.ErrMessageDeleteStateInvalid }, ErrMessageDeleteStateInvalid},
		{"repo root invalid", func(s *deleteMessageRepositoryStub) { s.deleteErr = repository.ErrMessageDeleteRootInvalid }, ErrMessageDeleteRootInvalid},
		{"repo not found", func(s *deleteMessageRepositoryStub) { s.deleteErr = repository.ErrNotFound }, ErrMessageNotFound},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := base
			messageCopy := *base.message
			stub.message = &messageCopy
			testCase.mutate(&stub)
			service := newDeleteMessageService(&stub)
			if _, err := service.DeleteMessage(context.Background(), 7, "conv_delete", "msg_mid"); !errors.Is(err, testCase.wantErr) {
				t.Fatalf("err = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}
