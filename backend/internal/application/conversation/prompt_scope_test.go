package conversation

import (
	"testing"

	appcompact "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/compact"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestBuildPromptScopeKeepsFullHistoryWhenDisabled(t *testing.T) {
	messages := promptScopeMessages()
	snapshot := promptScopeSnapshot(messages[:2])

	scope := buildPromptScope(messages, snapshot, contextCompactionPolicy{AdminEnabled: true, UserEnabled: false})

	if scope.Snapshot != nil {
		t.Fatal("expected snapshot to be ignored when policy is disabled")
	}
	if len(scope.RetainedMessages) != len(messages) {
		t.Fatalf("expected full history retained, got %d", len(scope.RetainedMessages))
	}
}

func TestBuildPromptScopeRejectsLegacySnapshot(t *testing.T) {
	messages := promptScopeMessages()
	snapshot := &model.ContextSnapshot{SummaryText: "legacy summary"}

	scope := buildPromptScope(messages, snapshot, contextCompactionPolicy{AdminEnabled: true, UserEnabled: true})

	if scope.Snapshot != nil {
		t.Fatal("expected legacy snapshot without anchors to be ignored")
	}
	if len(scope.RetainedMessages) != len(messages) {
		t.Fatalf("expected full history retained, got %d", len(scope.RetainedMessages))
	}
}

func TestBuildPromptScopeReplacesCoveredPrefix(t *testing.T) {
	messages := promptScopeMessages()
	snapshot := promptScopeSnapshot(messages[:2])

	scope := buildPromptScope(messages, snapshot, contextCompactionPolicy{AdminEnabled: true, UserEnabled: true})

	if scope.Snapshot == nil {
		t.Fatal("expected snapshot to be applied")
	}
	if len(scope.CoveredMessages) != 2 {
		t.Fatalf("expected 2 covered messages, got %d", len(scope.CoveredMessages))
	}
	if len(scope.RetainedMessages) != 2 {
		t.Fatalf("expected 2 retained messages, got %d", len(scope.RetainedMessages))
	}
	if scope.RetainedMessages[0].ID != 3 {
		t.Fatalf("expected retained history to start after covered boundary, got %d", scope.RetainedMessages[0].ID)
	}
}

func TestPromptScopeHistoricalMessageScopeUsesSnapshotBoundary(t *testing.T) {
	messages := promptScopeMessages()
	scope := buildPromptScope(messages, promptScopeSnapshot(messages[:2]), contextCompactionPolicy{AdminEnabled: true, UserEnabled: true})

	historicalScope := scope.historicalMessageScope(7, 11, 4)
	if !historicalScope.Valid() {
		t.Fatal("expected valid historical scope")
	}
	if historicalScope.ConversationID != 7 || historicalScope.UserID != 11 || historicalScope.LeafMessageID != 4 || historicalScope.ExcludeThroughMessageID != 2 {
		t.Fatalf("unexpected historical scope: %#v", historicalScope)
	}
}

func TestPromptScopeHistoricalMessageScopeUsesFullBranchWithoutSnapshot(t *testing.T) {
	messages := promptScopeMessages()
	scope := buildPromptScope(messages, nil, contextCompactionPolicy{})

	historicalScope := scope.historicalMessageScope(7, 11, 4)
	if !historicalScope.Valid() || historicalScope.ExcludeThroughMessageID != 0 {
		t.Fatalf("unexpected historical scope: %#v", historicalScope)
	}
}

func TestPromptScopeHistoricalMessageScopeFailsClosedOnFirstTurn(t *testing.T) {
	scope := buildPromptScope([]model.Message{{ID: 9, Role: "user"}}, nil, contextCompactionPolicy{})

	if historicalScope := scope.historicalMessageScope(7, 11, 9); historicalScope.Valid() {
		t.Fatalf("expected no historical scope, got %#v", historicalScope)
	}
}

func TestHistoryMessagesFromDomainPassesBackAssistantReasoningWhenEnabled(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "question", ReasoningContent: "ignored"},
		{Role: "assistant", Content: "answer", ReasoningContent: "thinking"},
	}

	got := historyMessagesFromDomain(messages, historyMessageOptions{ReasoningContentPassback: true})

	if len(got) != 2 {
		t.Fatalf("expected 2 history messages, got %d", len(got))
	}
	if got[0].ReasoningContent != "" {
		t.Fatalf("expected user reasoning to be ignored, got %q", got[0].ReasoningContent)
	}
	if got[1].ReasoningContent != "thinking" {
		t.Fatalf("expected assistant reasoning passback, got %q", got[1].ReasoningContent)
	}
}

func TestHistoryMessagesFromDomainOmitsAssistantReasoningWhenDisabled(t *testing.T) {
	messages := []model.Message{
		{Role: "assistant", Content: "answer", ReasoningContent: "thinking"},
	}

	got := historyMessagesFromDomain(messages, historyMessageOptions{ReasoningContentPassback: false})

	if len(got) != 1 {
		t.Fatalf("expected 1 history message, got %d", len(got))
	}
	if got[0].ReasoningContent != "" {
		t.Fatalf("expected reasoning content to be omitted, got %q", got[0].ReasoningContent)
	}
}

func promptScopeMessages() []model.Message {
	firstID := uint(1)
	secondID := uint(2)
	thirdID := uint(3)
	return []model.Message{
		{ID: firstID, PublicID: "m1", Role: "user", Content: "one"},
		{ID: secondID, PublicID: "m2", ParentMessageID: &firstID, Role: "assistant", Content: "two"},
		{ID: thirdID, PublicID: "m3", ParentMessageID: &secondID, Role: "user", Content: "three"},
		{ID: 4, PublicID: "m4", ParentMessageID: &thirdID, Role: "assistant", Content: "four"},
	}
}

func promptScopeSnapshot(covered []model.Message) *model.ContextSnapshot {
	boundary := covered[len(covered)-1]
	return &model.ContextSnapshot{
		SummaryText:           "summary",
		CoveredUntilMessageID: boundary.ID,
		CoveredUntilPublicID:  boundary.PublicID,
		CoveredMessageCount:   len(covered),
		CoveragePathHash:      appcompact.CoveragePathHash(covered),
	}
}
