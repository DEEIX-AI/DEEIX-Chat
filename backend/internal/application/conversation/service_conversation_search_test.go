package conversation

import (
	"context"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type conversationSearchRepositoryStub struct {
	repository.ConversationRepository
	items          []model.Conversation
	requestedLimit int
}

func (s *conversationSearchRepositoryStub) ListConversationsForSearch(
	_ context.Context,
	_ uint,
	_ int,
	limit int,
	_ string,
) ([]model.Conversation, error) {
	s.requestedLimit = limit
	return append([]model.Conversation(nil), s.items...), nil
}

func TestSearchConversationsUsesLookaheadInsteadOfExactCount(t *testing.T) {
	repo := &conversationSearchRepositoryStub{
		items: []model.Conversation{
			{ID: 3, PublicID: "conv_3", Title: "Three"},
			{ID: 2, PublicID: "conv_2", Title: "Two"},
			{ID: 1, PublicID: "conv_1", Title: "One"},
		},
	}
	service := &Service{repo: repo}

	items, hasMore, err := service.SearchConversations(context.Background(), 7, 1, 2, "query")
	if err != nil {
		t.Fatalf("SearchConversations() error = %v", err)
	}
	if repo.requestedLimit != 3 {
		t.Fatalf("requested limit = %d, want page size + 1", repo.requestedLimit)
	}
	if !hasMore {
		t.Fatal("hasMore = false, want true")
	}
	if len(items) != 2 || items[0].Conversation.PublicID != "conv_3" || items[1].Conversation.PublicID != "conv_2" {
		t.Fatalf("items = %#v, want first two search results", items)
	}
}
