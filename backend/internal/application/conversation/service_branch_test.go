package conversation

import (
	"encoding/json"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestNormalizeDefaultBranchContextKeepsHistoryAfterSuccessfulAssistantRetry(t *testing.T) {
	rootUserID := uint(1)
	rootAssistantID := uint(2)
	failedUserID := uint(3)
	retryAssistantID := uint(4)
	failedAssistantID := uint(5)
	historicalAttachments, err := json.Marshal([]attachmentSnapshotRef{{FileID: "file_from_history"}})
	if err != nil {
		t.Fatalf("marshal attachments: %v", err)
	}
	retriedAttachments, err := json.Marshal([]attachmentSnapshotRef{{FileID: "file_from_retried_user"}})
	if err != nil {
		t.Fatalf("marshal retried user attachments: %v", err)
	}
	ancestors := []model.Message{
		{ID: rootUserID, PublicID: "msg_root_user", Role: "user", Status: "success", Attachments: string(historicalAttachments)},
		{ID: rootAssistantID, PublicID: "msg_root_assistant", ParentMessageID: &rootUserID, Role: "assistant", Status: "success"},
		{ID: failedUserID, PublicID: "msg_failed_user", ParentMessageID: &rootAssistantID, Role: "user", Status: "error", Attachments: string(retriedAttachments)},
		{ID: retryAssistantID, PublicID: "msg_retry_assistant", ParentMessageID: &failedUserID, SourceMessageID: &failedAssistantID, Role: "assistant", BranchReason: "retry", Status: "success"},
	}

	normalized, parent := normalizeDefaultBranchContext(ancestors, &ancestors[3])

	if parent == nil || parent.ID != retryAssistantID {
		t.Fatalf("expected successful retry assistant as parent, got %#v", parent)
	}
	if len(normalized) != len(ancestors) {
		t.Fatalf("expected complete history after successful retry, got %#v", normalized)
	}
	for index, expected := range ancestors {
		if normalized[index].ID != expected.ID {
			t.Fatalf("expected message %d at index %d, got %#v", expected.ID, index, normalized)
		}
	}
	if normalized[2].Status != "success" {
		t.Fatalf("expected recovered user to be usable as context, got status %q", normalized[2].Status)
	}
	if ancestors[2].Status != "error" {
		t.Fatalf("expected normalization to leave persisted message state unchanged, got status %q", ancestors[2].Status)
	}
	fileIDs := collectConversationFileIDs(normalized, nil)
	if len(fileIDs) != 2 || fileIDs[0] != "file_from_history" || fileIDs[1] != "file_from_retried_user" {
		t.Fatalf("expected historical attachment to remain in context, got %#v", fileIDs)
	}
}

func TestRecoverAssistantRetryUserStatesRestoresExpandedContext(t *testing.T) {
	rootUserID := uint(1)
	rootAssistantID := uint(2)
	failedUserID := uint(3)
	sourceAssistantID := uint(4)
	retryAssistantID := uint(5)
	retriedAttachments, err := json.Marshal([]attachmentSnapshotRef{{FileID: "file_from_retried_user"}})
	if err != nil {
		t.Fatalf("marshal retried user attachments: %v", err)
	}
	messages := []model.Message{
		{ID: rootUserID, Role: "user", Status: "success"},
		{ID: rootAssistantID, ParentMessageID: &rootUserID, Role: "assistant", Status: "success"},
		{ID: failedUserID, ParentMessageID: &rootAssistantID, Role: "user", Status: "error", Attachments: string(retriedAttachments)},
		{ID: retryAssistantID, ParentMessageID: &failedUserID, SourceMessageID: &sourceAssistantID, Role: "assistant", BranchReason: "retry", Status: "success"},
		{ID: 6, ParentMessageID: &retryAssistantID, Role: "user", Status: "pending"},
	}

	recovered := recoverAssistantRetryUserStates(messages)

	if len(recovered) != len(messages) {
		t.Fatalf("expected expanded context length %d, got %#v", len(messages), recovered)
	}
	if recovered[2].Status != "success" {
		t.Fatalf("expected retried user to be recovered, got status %q", recovered[2].Status)
	}
	if recovered[4].Status != "pending" {
		t.Fatalf("expected unrelated message state to remain pending, got %q", recovered[4].Status)
	}
	if messages[2].Status != "error" {
		t.Fatalf("expected persisted state copy to remain unchanged, got %q", messages[2].Status)
	}
	fileIDs := collectConversationFileIDs(recovered, nil)
	if len(fileIDs) != 1 || fileIDs[0] != "file_from_retried_user" {
		t.Fatalf("expected recovered attachment in expanded context, got %#v", fileIDs)
	}
}

func TestNormalizeDefaultBranchContextDoesNotRecoverFailedRetry(t *testing.T) {
	rootUserID := uint(1)
	rootAssistantID := uint(2)
	failedUserID := uint(3)
	failedAssistantID := uint(5)
	ancestors := []model.Message{
		{ID: rootUserID, PublicID: "msg_root_user", Role: "user", Status: "success"},
		{ID: rootAssistantID, PublicID: "msg_root_assistant", ParentMessageID: &rootUserID, Role: "assistant", Status: "success"},
		{ID: failedUserID, PublicID: "msg_failed_user", ParentMessageID: &rootAssistantID, Role: "user", Status: "error"},
		{ID: 4, PublicID: "msg_failed_retry", ParentMessageID: &failedUserID, SourceMessageID: &failedAssistantID, Role: "assistant", BranchReason: "retry", Status: "error"},
	}

	normalized, parent := normalizeDefaultBranchContext(ancestors, &ancestors[3])

	if parent == nil || parent.ID != rootAssistantID {
		t.Fatalf("expected latest successful ancestor as parent, got %#v", parent)
	}
	if len(normalized) != 2 || normalized[0].ID != rootUserID || normalized[1].ID != rootAssistantID {
		t.Fatalf("expected failed retry tail removed from context, got %#v", normalized)
	}
}

func TestIsRecoveredAssistantRetryUserRequiresGenuineUsableRetry(t *testing.T) {
	failedUserID := uint(10)
	sourceAssistantID := uint(11)
	otherUserID := uint(12)
	tests := []struct {
		name      string
		user      model.Message
		assistant model.Message
		expected  bool
	}{
		{
			name:      "successful retry",
			user:      model.Message{ID: failedUserID, Role: "user", Status: "error"},
			assistant: model.Message{Role: "assistant", ParentMessageID: &failedUserID, SourceMessageID: &sourceAssistantID, BranchReason: "retry", Status: "success"},
			expected:  true,
		},
		{
			name:      "interrupted retry",
			user:      model.Message{ID: failedUserID, Role: "user", Status: "error"},
			assistant: model.Message{Role: "assistant", ParentMessageID: &failedUserID, SourceMessageID: &sourceAssistantID, BranchReason: "retry", Status: "interrupted"},
			expected:  true,
		},
		{
			name:      "failed retry",
			user:      model.Message{ID: failedUserID, Role: "user", Status: "error"},
			assistant: model.Message{Role: "assistant", ParentMessageID: &failedUserID, SourceMessageID: &sourceAssistantID, BranchReason: "retry", Status: "error"},
		},
		{
			name:      "non-retry assistant",
			user:      model.Message{ID: failedUserID, Role: "user", Status: "error"},
			assistant: model.Message{Role: "assistant", ParentMessageID: &failedUserID, SourceMessageID: &sourceAssistantID, BranchReason: "default", Status: "success"},
		},
		{
			name:      "retry without source",
			user:      model.Message{ID: failedUserID, Role: "user", Status: "error"},
			assistant: model.Message{Role: "assistant", ParentMessageID: &failedUserID, BranchReason: "retry", Status: "success"},
		},
		{
			name:      "retry with different parent",
			user:      model.Message{ID: failedUserID, Role: "user", Status: "error"},
			assistant: model.Message{Role: "assistant", ParentMessageID: &otherUserID, SourceMessageID: &sourceAssistantID, BranchReason: "retry", Status: "success"},
		},
		{
			name:      "already successful user",
			user:      model.Message{ID: failedUserID, Role: "user", Status: "success"},
			assistant: model.Message{Role: "assistant", ParentMessageID: &failedUserID, SourceMessageID: &sourceAssistantID, BranchReason: "retry", Status: "success"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			messages := []model.Message{test.user, test.assistant}
			if actual := isRecoveredAssistantRetryUser(messages, 0); actual != test.expected {
				t.Fatalf("expected recovered=%v, got %v", test.expected, actual)
			}
		})
	}
}

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

func TestTruncateContextByTokenBudgetCountsAssistantReasoningWhenEnabled(t *testing.T) {
	messages := []model.Message{
		{ID: 1, Role: "user", Content: "first"},
		{ID: 2, Role: "assistant", Content: "ok", ReasoningContent: "this reasoning content is deliberately long enough to exceed the tiny budget"},
		{ID: 3, Role: "user", Content: "next"},
	}

	withReasoning := truncateContextByTokenBudget(messages, 6, true)
	if len(withReasoning) != 1 || withReasoning[0].ID != 3 {
		t.Fatalf("expected only latest message when reasoning is counted, got %#v", withReasoning)
	}

	withoutReasoning := truncateContextByTokenBudget(messages, 6, false)
	if len(withoutReasoning) != 3 {
		t.Fatalf("expected all messages when reasoning is omitted, got %#v", withoutReasoning)
	}
}

func TestBuildBranchMessagePathReusesExistingUserForAssistantRetry(t *testing.T) {
	rootID := uint(1)
	userID := uint(2)
	assistantID := uint(3)
	branch := &messageBranchState{
		ExistingMessages: []model.Message{
			{ID: rootID, PublicID: "msg_root", Role: "assistant", Status: "success"},
			{ID: userID, PublicID: "msg_user", ParentMessageID: &rootID, Role: "user", Status: "success"},
		},
		ReuseUserMessage: &model.Message{ID: userID, PublicID: "msg_user", ParentMessageID: &rootID, Role: "user", Status: "success"},
	}
	assistantMessage := &model.Message{ID: assistantID, PublicID: "msg_assistant_retry", ParentMessageID: &userID, Role: "assistant", Status: "pending"}

	path := buildBranchMessagePath(branch, assistantMessage)

	if len(path) != 2 {
		t.Fatalf("expected reused user path without pending assistant, got %#v", path)
	}
	if path[0].ID != rootID || path[1].ID != userID {
		t.Fatalf("expected root -> reused user path, got %#v", path)
	}
}
