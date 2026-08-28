package conversation

import (
	"context"
	"errors"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type forkConversationRepositoryStub struct {
	repository.ConversationRepository
	conversation    *model.Conversation
	conversationErr error
	message         *model.Message
	messageErr      error
	path            []model.Message
	pathErr         error
	createInput     repository.CreateForkedConversationInput
	createErr       error
	createCalls     int
}

func (s *forkConversationRepositoryStub) GetConversationByPublicID(context.Context, string, uint) (*model.Conversation, error) {
	return s.conversation, s.conversationErr
}

func (s *forkConversationRepositoryStub) GetMessageByPublicIDForUser(context.Context, uint, string) (*model.Message, error) {
	return s.message, s.messageErr
}

func (s *forkConversationRepositoryStub) ListMessageAncestors(context.Context, uint, uint, int) ([]model.Message, error) {
	return s.path, s.pathErr
}

func (s *forkConversationRepositoryStub) CreateForkedConversation(_ context.Context, input repository.CreateForkedConversationInput) error {
	s.createCalls++
	s.createInput = input
	if s.createErr != nil {
		return s.createErr
	}
	input.Conversation.ID = 999
	input.Conversation.CreatedAt = time.Now().UTC()
	return nil
}

func newForkConversationService(repo repository.ConversationRepository) *Service {
	return &Service{
		cfg:  config.NewRuntime(config.Config{}),
		repo: repo,
	}
}

func TestForkConversationFromMessageBuildsAtomicFork(t *testing.T) {
	projectID := uint(3)
	rootID := uint(101)
	leafID := uint(102)
	sourceID := uint(88)
	editedAt := time.Now().UTC().Add(-time.Minute)
	repo := &forkConversationRepositoryStub{
		conversation: &model.Conversation{
			ID:                    10,
			UserID:                7,
			ProjectID:             &projectID,
			ProjectPublicID:       "project_public",
			ProjectName:           "Project",
			ProjectSystemPrompt:   "Project instructions",
			PublicID:              "conv_source",
			Title:                 "Source title",
			LabelsJSON:            `["important"]`,
			LabelsManuallyManaged: true,
			Model:                 "model-a",
			Provider:              "provider-a",
			LastResponseID:        "response-to-drop",
		},
		message: &model.Message{ID: leafID, ConversationID: 10, UserID: 7, PublicID: "msg_leaf", Role: "assistant", Status: "success"},
		path: []model.Message{
			{
				ID: rootID, ConversationID: 10, UserID: 7, PublicID: "msg_root",
				Role: "user", ContentType: "text", Content: "hello", BranchReason: "retry",
				SourceMessageID: &sourceID, RunID: "run-root", BilledNanousd: 50, PricingSnapshot: `{"input":1}`,
				Status: "success",
			},
			{
				ID: leafID, ConversationID: 10, UserID: 7, PublicID: "msg_leaf", ParentMessageID: &rootID,
				Role: "assistant", ContentType: "text", Content: "world", ReasoningContent: "reasoning",
				BranchReason: "edit", SourceMessageID: &sourceID, RunID: "run-leaf",
				InputTokens: 10, OutputTokens: 20, BilledNanousd: 100, PricingSnapshot: `{"output":1}`,
				Status: "interrupted", EditedAt: &editedAt,
			},
		},
	}

	created, err := newForkConversationService(repo).ForkConversationFromMessage(context.Background(), 7, " conv_source ", " msg_leaf ")
	if err != nil {
		t.Fatalf("ForkConversationFromMessage() error = %v", err)
	}
	if repo.createCalls != 1 {
		t.Fatalf("CreateForkedConversation() calls = %d, want 1", repo.createCalls)
	}
	if created.ID != 999 || created.MessageCount != 2 {
		t.Fatalf("created conversation = %+v, want persisted two-message fork", created)
	}
	if created.PublicID == "" || created.PublicID == repo.conversation.PublicID || created.SessionKey == "" {
		t.Fatalf("fork identifiers were not regenerated: publicID=%q sessionKey=%q", created.PublicID, created.SessionKey)
	}
	if created.LastResponseID != "" || created.LastCompactedAt != nil {
		t.Fatalf("fork retained upstream state: lastResponseID=%q lastCompactedAt=%v", created.LastResponseID, created.LastCompactedAt)
	}
	if created.ProjectPublicID != repo.conversation.ProjectPublicID ||
		created.ProjectName != repo.conversation.ProjectName ||
		created.ProjectSystemPrompt != repo.conversation.ProjectSystemPrompt {
		t.Fatalf("fork lost project summary: %+v", created)
	}
	if len(repo.createInput.Messages) != 2 {
		t.Fatalf("len(createInput.Messages) = %d, want 2", len(repo.createInput.Messages))
	}
	for index, item := range repo.createInput.Messages {
		if item.Message.PublicID == "" || item.Message.PublicID == repo.path[index].PublicID {
			t.Fatalf("message %d public ID was not regenerated", index)
		}
		if item.Message.BranchReason != "default" || item.Message.SourceMessageID != nil || item.Message.RunID != "" {
			t.Fatalf("message %d retained source branch metadata: %+v", index, item.Message)
		}
		if item.Message.BilledNanousd != 0 || item.Message.PricingSnapshot != "" {
			t.Fatalf("message %d retained source billing data: %+v", index, item.Message)
		}
	}
	if repo.createInput.Messages[0].SourceParentMessageID != nil {
		t.Fatal("root message unexpectedly has a source parent")
	}
	if parent := repo.createInput.Messages[1].SourceParentMessageID; parent == nil || *parent != rootID {
		t.Fatalf("leaf source parent = %v, want %d", parent, rootID)
	}
}

func TestForkConversationFromMessageRejectsIncompleteHistory(t *testing.T) {
	omittedParentID := uint(99)
	message := &model.Message{ID: 102, ConversationID: 10, UserID: 7, PublicID: "msg_leaf", Role: "assistant", Status: "success"}
	repo := &forkConversationRepositoryStub{
		conversation: &model.Conversation{ID: 10, UserID: 7, PublicID: "conv_source"},
		message:      message,
		path: []model.Message{{
			ID: message.ID, ConversationID: 10, UserID: 7, PublicID: message.PublicID,
			ParentMessageID: &omittedParentID, Role: "assistant", ContentType: "text", Status: "success",
		}},
	}

	_, err := newForkConversationService(repo).ForkConversationFromMessage(context.Background(), 7, "conv_source", "msg_leaf")
	if !errors.Is(err, ErrMessageForkHistoryIncomplete) {
		t.Fatalf("ForkConversationFromMessage() error = %v, want ErrMessageForkHistoryIncomplete", err)
	}
	if repo.createCalls != 0 {
		t.Fatalf("CreateForkedConversation() calls = %d, want 0", repo.createCalls)
	}
}

func TestForkConversationFromMessageRejectsPendingTarget(t *testing.T) {
	for _, status := range []string{"pending", " Pending "} {
		t.Run(status, func(t *testing.T) {
			repo := &forkConversationRepositoryStub{
				conversation: &model.Conversation{ID: 10, UserID: 7, PublicID: "conv_source"},
				message:      &model.Message{ID: 102, ConversationID: 10, UserID: 7, PublicID: "msg_leaf", Role: "assistant", Status: status},
			}

			_, err := newForkConversationService(repo).ForkConversationFromMessage(context.Background(), 7, "conv_source", "msg_leaf")
			if !errors.Is(err, ErrMessageForkStateInvalid) {
				t.Fatalf("ForkConversationFromMessage() error = %v, want ErrMessageForkStateInvalid", err)
			}
			if repo.createCalls != 0 {
				t.Fatalf("CreateForkedConversation() calls = %d, want 0", repo.createCalls)
			}
		})
	}
}

func TestForkConversationFromMessageRejectsNonAssistantTarget(t *testing.T) {
	for _, role := range []string{"user", "system", " User "} {
		t.Run(role, func(t *testing.T) {
			repo := &forkConversationRepositoryStub{
				conversation: &model.Conversation{ID: 10, UserID: 7, PublicID: "conv_source"},
				message: &model.Message{
					ID: 101, ConversationID: 10, UserID: 7, PublicID: "msg_user", Role: role, Status: "success",
				},
			}

			_, err := newForkConversationService(repo).ForkConversationFromMessage(context.Background(), 7, "conv_source", "msg_user")
			if !errors.Is(err, ErrMessageForkTargetInvalid) {
				t.Fatalf("ForkConversationFromMessage() error = %v, want ErrMessageForkTargetInvalid", err)
			}
			if repo.createCalls != 0 {
				t.Fatalf("CreateForkedConversation() calls = %d, want 0", repo.createCalls)
			}
		})
	}
}

func TestForkConversationFromMessageMapsConcurrentSourceRemoval(t *testing.T) {
	message := &model.Message{ID: 102, ConversationID: 10, UserID: 7, PublicID: "msg_leaf", Role: "assistant", Status: "success"}
	repo := &forkConversationRepositoryStub{
		conversation: &model.Conversation{ID: 10, UserID: 7, PublicID: "conv_source"},
		message:      message,
		path: []model.Message{{
			ID: message.ID, ConversationID: 10, UserID: 7, PublicID: message.PublicID,
			Role: "assistant", ContentType: "text", Status: "success",
		}},
		createErr: repository.ErrNotFound,
	}

	_, err := newForkConversationService(repo).ForkConversationFromMessage(context.Background(), 7, "conv_source", "msg_leaf")
	if !errors.Is(err, ErrMessageNotFound) {
		t.Fatalf("ForkConversationFromMessage() error = %v, want ErrMessageNotFound", err)
	}
}

func TestForkConversationFromMessageMapsOnlyMissingConversation(t *testing.T) {
	storageFailure := errors.New("database unavailable")
	for _, tc := range []struct {
		name    string
		repoErr error
		want    error
	}{
		{name: "not found", repoErr: repository.ErrNotFound, want: ErrConversationNotFound},
		{name: "storage failure", repoErr: storageFailure, want: storageFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &forkConversationRepositoryStub{conversationErr: tc.repoErr}
			_, err := newForkConversationService(repo).ForkConversationFromMessage(context.Background(), 7, "conv_source", "msg_leaf")
			if !errors.Is(err, tc.want) {
				t.Fatalf("ForkConversationFromMessage() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNormalizeForkedMessageStatus(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "keeps success", in: "success", want: "success"},
		{name: "keeps interrupted", in: "interrupted", want: "interrupted"},
		{name: "keeps error", in: "error", want: "error"},
		{name: "keeps blocked", in: "blocked", want: "blocked"},
		{name: "maps pending to interrupted", in: "pending", want: "interrupted"},
		{name: "maps pending with spaces to interrupted", in: "  pending  ", want: "interrupted"},
		{name: "maps mixed-case pending to interrupted", in: "Pending", want: "interrupted"},
		{name: "falls back to success for empty", in: "", want: "success"},
		{name: "falls back to success for blank", in: "   ", want: "success"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeForkedMessageStatus(tc.in); got != tc.want {
				t.Fatalf("normalizeForkedMessageStatus(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
