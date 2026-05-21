package conversation

import (
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestNormalizeDefaultBranchContextSkipsFailedTail(t *testing.T) {
	rootID := uint(1)
	assistantID := uint(2)
	ancestors := []model.Message{
		{ID: rootID, PublicID: "msg_success_user", Role: "user", Status: "success"},
		{ID: assistantID, PublicID: "msg_success_assistant", ParentMessageID: &rootID, Role: "assistant", Status: "success"},
		{ID: 3, PublicID: "msg_failed_assistant", ParentMessageID: &assistantID, Role: "assistant", Status: "error"},
	}

	normalized, parent := normalizeDefaultBranchContext(ancestors, &ancestors[2])

	if parent == nil || parent.ID != assistantID {
		t.Fatalf("expected latest successful ancestor as parent, got %#v", parent)
	}
	if len(normalized) != 2 || normalized[0].ID != rootID || normalized[1].ID != assistantID {
		t.Fatalf("expected failed tail removed from context, got %#v", normalized)
	}
}

func TestNormalizeDefaultBranchContextKeepsSuccessfulSegmentAfterFailedMiddle(t *testing.T) {
	firstUserID := uint(1)
	failedAssistantID := uint(2)
	recoveredUserID := uint(3)
	recoveredAssistantID := uint(4)
	ancestors := []model.Message{
		{ID: firstUserID, PublicID: "msg_first_user", Role: "user", Status: "success"},
		{ID: failedAssistantID, PublicID: "msg_failed_assistant", ParentMessageID: &firstUserID, Role: "assistant", Status: "error"},
		{ID: recoveredUserID, PublicID: "msg_recovered_user", ParentMessageID: &failedAssistantID, Role: "user", Status: "success"},
		{ID: recoveredAssistantID, PublicID: "msg_recovered_assistant", ParentMessageID: &recoveredUserID, Role: "assistant", Status: "success"},
	}

	normalized, parent := normalizeDefaultBranchContext(ancestors, &ancestors[3])

	if parent == nil || parent.ID != recoveredAssistantID {
		t.Fatalf("expected recovered assistant as parent, got %#v", parent)
	}
	if len(normalized) != 2 || normalized[0].ID != recoveredUserID || normalized[1].ID != recoveredAssistantID {
		t.Fatalf("expected successful segment after failed middle, got %#v", normalized)
	}
}

func TestNormalizeDefaultBranchContextReturnsEmptyForOnlyFailedMessages(t *testing.T) {
	ancestors := []model.Message{
		{ID: 1, PublicID: "msg_failed_user", Role: "user", Status: "error"},
		{ID: 2, PublicID: "msg_failed_assistant", Role: "assistant", Status: "error"},
	}

	normalized, parent := normalizeDefaultBranchContext(ancestors, &ancestors[1])

	if parent != nil {
		t.Fatalf("expected no parent, got %#v", parent)
	}
	if len(normalized) != 0 {
		t.Fatalf("expected empty context, got %#v", normalized)
	}
}

func TestSelectLatestDefaultParentCandidatePrefersSuccessfulAssistant(t *testing.T) {
	userOneID := uint(1)
	assistantOneID := uint(2)
	userTwoID := uint(3)
	messages := []model.Message{
		{ID: userOneID, PublicID: "msg_user_1", Role: "user", Status: "success"},
		{ID: assistantOneID, PublicID: "msg_assistant_1", ParentMessageID: &userOneID, Role: "assistant", Status: "success"},
		{ID: userTwoID, PublicID: "msg_user_2", ParentMessageID: &assistantOneID, Role: "user", Status: "success"},
		{ID: 4, PublicID: "msg_assistant_2", ParentMessageID: &userTwoID, Role: "assistant", Status: "pending"},
	}

	parent := selectLatestDefaultParentCandidate(messages)

	if parent == nil || parent.ID != assistantOneID {
		t.Fatalf("expected latest successful assistant as parent, got %#v", parent)
	}
}

func TestSelectLatestDefaultParentCandidateUsesLatestCompletedTurn(t *testing.T) {
	userOneID := uint(1)
	assistantOneID := uint(2)
	userTwoID := uint(3)
	assistantTwoID := uint(4)
	messages := []model.Message{
		{ID: userOneID, PublicID: "msg_user_1", Role: "user", Status: "success"},
		{ID: assistantOneID, PublicID: "msg_assistant_1", ParentMessageID: &userOneID, Role: "assistant", Status: "success"},
		{ID: userTwoID, PublicID: "msg_user_2", ParentMessageID: &assistantOneID, Role: "user", Status: "success"},
		{ID: assistantTwoID, PublicID: "msg_assistant_2", ParentMessageID: &userTwoID, Role: "assistant", Status: "success"},
	}

	parent := selectLatestDefaultParentCandidate(messages)

	if parent == nil || parent.ID != assistantTwoID {
		t.Fatalf("expected newest successful assistant as parent, got %#v", parent)
	}
}

func TestSelectLatestDefaultParentCandidateFallsBackToSuccessfulUser(t *testing.T) {
	messages := []model.Message{
		{ID: 1, PublicID: "msg_user_1", Role: "user", Status: "success"},
	}

	parent := selectLatestDefaultParentCandidate(messages)

	if parent == nil || parent.ID != 1 {
		t.Fatalf("expected successful user fallback as parent, got %#v", parent)
	}
}
