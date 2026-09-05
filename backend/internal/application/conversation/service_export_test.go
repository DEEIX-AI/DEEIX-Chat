package conversation

import (
	"context"
	"errors"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestExportUserConversationDataRejectsWrongUser(t *testing.T) {
	svc := &Service{}
	conv := &model.Conversation{ID: 1, UserID: 42}

	_, err := svc.ExportUserConversationData(context.TODO(), 99, conv)
	if !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("expected ErrConversationNotFound, got %v", err)
	}
}

func TestExportDefaultMessagePublicIDsFiltersVisibleBranch(t *testing.T) {
	rootID := uint(1)
	messages := []model.Message{
		{ID: 1, PublicID: "msg_user", Role: "user", Status: "success"},
		{ID: 2, PublicID: "msg_assistant_v1", ParentMessageID: &rootID, Role: "assistant", Status: "success"},
		{ID: 3, PublicID: "msg_assistant_v2", ParentMessageID: &rootID, Role: "assistant", Status: "success"},
	}
	ids := exportDefaultMessagePublicIDs(messages)
	if len(ids) == 0 {
		t.Fatal("expected non-empty default message IDs")
	}
	for _, id := range ids {
		if id == "" {
			t.Error("default message public ID should not be empty")
		}
	}
}

func TestCollectExportMessageRunIDsDeduplicates(t *testing.T) {
	messages := []model.Message{
		{RunID: "run_1"},
		{RunID: "run_2"},
		{RunID: "run_1"},
		{RunID: ""},
		{RunID: "run_3"},
	}
	runIDs := model.CollectMessageRunIDs(messages)
	if len(runIDs) != 3 {
		t.Fatalf("expected 3 unique run IDs, got %d: %v", len(runIDs), runIDs)
	}
	expected := map[string]bool{"run_1": true, "run_2": true, "run_3": true}
	for _, id := range runIDs {
		if !expected[id] {
			t.Errorf("unexpected run ID: %s", id)
		}
	}
}

func TestCollectExportMessageRunIDsSkipsEmpty(t *testing.T) {
	messages := []model.Message{
		{RunID: ""},
		{RunID: "  "},
	}
	runIDs := model.CollectMessageRunIDs(messages)
	if len(runIDs) != 0 {
		t.Fatalf("expected 0 run IDs for empty inputs, got %d", len(runIDs))
	}
}
