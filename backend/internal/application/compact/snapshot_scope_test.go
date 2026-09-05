package compact

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func newTestService(cfg config.Config, repo repository.CompactRepository, logger *zap.Logger) *Service {
	return NewServiceWithRuntime(config.NewRuntime(cfg), repo, logger)
}

func TestSnapshotBoundaryIndexRequiresCoverageAnchors(t *testing.T) {
	messages := []domainconversation.Message{{ID: 1, PublicID: "m1", Role: "user"}}
	snapshot := &domainconversation.ContextSnapshot{
		SummaryText: "summary",
	}

	if _, ok := SnapshotBoundaryIndex(messages, snapshot); ok {
		t.Fatal("expected legacy snapshot without anchors to be rejected")
	}
}

func TestSnapshotBoundaryIndexAcceptsMatchingBranchPrefix(t *testing.T) {
	firstID := uint(1)
	messages := []domainconversation.Message{
		{ID: firstID, PublicID: "m1", Role: "user"},
		{ID: 2, PublicID: "m2", Role: "assistant", ParentMessageID: &firstID},
		{ID: 3, PublicID: "m3", Role: "user", ParentMessageID: uintPtr(2)},
	}
	covered := messages[:2]
	snapshot := &domainconversation.ContextSnapshot{
		SummaryText:           "summary",
		CoveredUntilMessageID: 2,
		CoveredUntilPublicID:  "m2",
		CoveredMessageCount:   len(covered),
		CoveragePathHash:      CoveragePathHash(covered),
	}

	index, ok := SnapshotBoundaryIndex(messages, snapshot)
	if !ok {
		t.Fatal("expected snapshot to match branch prefix")
	}
	if index != 1 {
		t.Fatalf("expected boundary index 1, got %d", index)
	}
}

func TestSnapshotBoundaryIndexRejectsDifferentBranchPath(t *testing.T) {
	firstID := uint(1)
	messages := []domainconversation.Message{
		{ID: firstID, PublicID: "m1", Role: "user"},
		{ID: 2, PublicID: "m2", Role: "assistant", ParentMessageID: &firstID},
	}
	otherParent := uint(99)
	otherCovered := []domainconversation.Message{
		{ID: firstID, PublicID: "m1", Role: "user"},
		{ID: 2, PublicID: "m2", Role: "assistant", ParentMessageID: &otherParent},
	}
	snapshot := &domainconversation.ContextSnapshot{
		SummaryText:           "summary",
		CoveredUntilMessageID: 2,
		CoveredUntilPublicID:  "m2",
		CoveredMessageCount:   len(otherCovered),
		CoveragePathHash:      CoveragePathHash(otherCovered),
	}

	if _, ok := SnapshotBoundaryIndex(messages, snapshot); ok {
		t.Fatal("expected snapshot from different branch path to be rejected")
	}
}

func TestSnapshotBoundaryAncestorIndexAcceptsPartialAncestorPath(t *testing.T) {
	firstID := uint(1)
	fullPath := []domainconversation.Message{
		{ID: firstID, PublicID: "m1", Role: "user"},
		{ID: 2, PublicID: "m2", Role: "assistant", ParentMessageID: &firstID},
		{ID: 3, PublicID: "m3", Role: "user", ParentMessageID: uintPtr(2)},
		{ID: 4, PublicID: "m4", Role: "assistant", ParentMessageID: uintPtr(3)},
	}
	snapshot := &domainconversation.ContextSnapshot{
		SummaryText:           "summary",
		CoveredUntilMessageID: 2,
		CoveredUntilPublicID:  "m2",
		CoveredMessageCount:   2,
		CoveragePathHash:      CoveragePathHash(fullPath[:2]),
	}

	index, ok := SnapshotBoundaryAncestorIndex(fullPath[1:], snapshot)
	if !ok {
		t.Fatal("expected partial ancestor path to match snapshot boundary")
	}
	if index != 0 {
		t.Fatalf("expected boundary at partial path start, got %d", index)
	}
}

func TestExtendCoveragePathHashMatchesFullPath(t *testing.T) {
	firstID := uint(1)
	fullPath := []domainconversation.Message{
		{ID: firstID, PublicID: "m1", Role: "user"},
		{ID: 2, PublicID: "m2", Role: "assistant", ParentMessageID: &firstID},
		{ID: 3, PublicID: "m3", Role: "user", ParentMessageID: uintPtr(2)},
		{ID: 4, PublicID: "m4", Role: "assistant", ParentMessageID: uintPtr(3)},
	}

	prefixHash := CoveragePathHash(fullPath[:2])
	extendedHash := ExtendCoveragePathHash(prefixHash, fullPath[2:])

	if extendedHash != CoveragePathHash(fullPath) {
		t.Fatalf("expected extended hash to match full path hash")
	}
}

func TestSplitMessagesByPreservedTurns(t *testing.T) {
	messages := []domainconversation.Message{
		{ID: 1, Role: "user"},
		{ID: 2, Role: "assistant"},
		{ID: 3, Role: "user"},
		{ID: 4, Role: "assistant"},
		{ID: 5, Role: "user"},
		{ID: 6, Role: "assistant"},
	}

	covered, retained := splitMessagesByPreservedTurns(messages, 1)

	if len(covered) != 4 {
		t.Fatalf("expected 4 covered messages, got %d", len(covered))
	}
	if len(retained) != 2 {
		t.Fatalf("expected 2 retained messages, got %d", len(retained))
	}
	if retained[0].ID != 5 {
		t.Fatalf("expected retained segment to start at newest user turn, got %d", retained[0].ID)
	}
}

func TestMaybeCompactConversationRollsForwardExistingSnapshot(t *testing.T) {
	messages := []domainconversation.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: "old user"},
		{ID: 2, PublicID: "m2", ParentMessageID: uintPtr(1), Role: "assistant", Content: "old assistant"},
		{ID: 3, PublicID: "m3", ParentMessageID: uintPtr(2), Role: "user", Content: "new user"},
		{ID: 4, PublicID: "m4", ParentMessageID: uintPtr(3), Role: "assistant", Content: "new assistant"},
		{ID: 5, PublicID: "m5", ParentMessageID: uintPtr(4), Role: "user", Content: "latest user"},
		{ID: 6, PublicID: "m6", ParentMessageID: uintPtr(5), Role: "assistant", Content: "latest assistant"},
	}
	repo := &compactRepositoryStub{
		latest: &domainconversation.ContextSnapshot{
			SummaryText:           "previous summary",
			CoveredUntilMessageID: 2,
			CoveredUntilPublicID:  "m2",
			CoveredMessageCount:   2,
			CoveragePathHash:      CoveragePathHash(messages[:2]),
		},
	}
	svc := newTestService(config.Config{
		ContextCompactEnabled:           true,
		ContextMaxTurns:                 1,
		ContextCompactPreserve:          1,
		ContextCompactHighlightsPerRole: 4,
		ContextCompactSnippetChars:      200,
	}, repo, nil)

	snapshot, err := svc.MaybeCompactConversation(t.Context(), MaybeCompactConversationInput{
		ConversationID: 9,
		UserID:         7,
		RunID:          "run_1",
		Messages:       messages,
	})
	if err != nil {
		t.Fatalf("expected compaction to succeed, got %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected new snapshot")
	}
	if repo.created == nil {
		t.Fatal("expected snapshot to be persisted")
	}
	if snapshot.CoveredUntilMessageID != 4 || snapshot.CoveredMessageCount != 4 {
		t.Fatalf("expected snapshot to cover first four messages, got %#v", snapshot)
	}
	if !strings.Contains(snapshot.SummaryText, "previous summary") {
		t.Fatalf("expected previous summary to be carried forward, got %q", snapshot.SummaryText)
	}
	if strings.Contains(snapshot.SummaryText, "old user") || strings.Contains(snapshot.SummaryText, "old assistant") {
		t.Fatalf("expected already-covered messages to stay out of new summary source, got %q", snapshot.SummaryText)
	}
	if !strings.Contains(snapshot.SummaryText, "new user") || !strings.Contains(snapshot.SummaryText, "new assistant") {
		t.Fatalf("expected newly covered messages in summary, got %q", snapshot.SummaryText)
	}
	if snapshot.CoveragePathHash != CoveragePathHash(messages[:4]) {
		t.Fatal("expected rolled snapshot hash to match the full covered path")
	}
}

func TestMaybeCompactConversationSerializesSameConversation(t *testing.T) {
	messages := []domainconversation.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: "first"},
		{ID: 2, PublicID: "m2", ParentMessageID: uintPtr(1), Role: "assistant", Content: "first reply"},
		{ID: 3, PublicID: "m3", ParentMessageID: uintPtr(2), Role: "user", Content: "second"},
		{ID: 4, PublicID: "m4", ParentMessageID: uintPtr(3), Role: "assistant", Content: "second reply"},
		{ID: 5, PublicID: "m5", ParentMessageID: uintPtr(4), Role: "user", Content: "latest"},
		{ID: 6, PublicID: "m6", ParentMessageID: uintPtr(5), Role: "assistant", Content: "latest reply"},
	}
	repo := &compactRepositoryStub{}
	svc := newTestService(config.Config{
		ContextCompactEnabled:  true,
		ContextMaxTurns:        1,
		ContextCompactPreserve: 1,
		CompactLLMEnabled:      true,
	}, repo, nil)

	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var summarizerCalls atomic.Int32
	svc.SetLLMSummarizer(func(ctx context.Context, platformModelName string, messages []domainconversation.Message, prompt string) (string, error) {
		summarizerCalls.Add(1)
		entered <- struct{}{}
		<-release
		return "serialized summary", nil
	})

	var wg sync.WaitGroup
	run := func(runID string) {
		defer wg.Done()
		_, _ = svc.MaybeCompactConversation(t.Context(), MaybeCompactConversationInput{
			ConversationID: 42,
			RunID:          runID,
			Messages:       messages,
		})
	}
	wg.Add(1)
	go run("run-1")
	<-entered
	wg.Add(1)
	go run("run-2")

	concurrent := false
	select {
	case <-entered:
		concurrent = true
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	wg.Wait()

	if concurrent {
		t.Fatal("expected same-conversation compaction to be serialized")
	}
	if got := summarizerCalls.Load(); got != 1 {
		t.Fatalf("expected the second compaction to reuse the first snapshot, got %d summaries", got)
	}
	if repo.createCount != 1 {
		t.Fatalf("expected one persisted snapshot, got %d", repo.createCount)
	}
}

func TestMaybeCompactConversationDoesNotRetriggerFromCoveredHistory(t *testing.T) {
	messages := []domainconversation.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: strings.Repeat("old", 20_000)},
		{ID: 2, PublicID: "m2", ParentMessageID: uintPtr(1), Role: "assistant", Content: strings.Repeat("history", 20_000)},
		{ID: 3, PublicID: "m3", ParentMessageID: uintPtr(2), Role: "user", Content: "new user"},
		{ID: 4, PublicID: "m4", ParentMessageID: uintPtr(3), Role: "assistant", Content: "new assistant"},
	}
	repo := &compactRepositoryStub{
		latest: &domainconversation.ContextSnapshot{
			SummaryText:           "previous summary",
			CoveredUntilMessageID: 2,
			CoveredUntilPublicID:  "m2",
			CoveredMessageCount:   2,
			CoveragePathHash:      CoveragePathHash(messages[:2]),
		},
	}
	svc := newTestService(config.Config{
		ContextCompactEnabled:        true,
		ContextMaxTurns:              1,
		ContextCompactTriggerPercent: 10,
		ContextCompactPreserve:       1,
	}, repo, nil)

	snapshot, err := svc.MaybeCompactConversation(t.Context(), MaybeCompactConversationInput{
		ConversationID:      9,
		UserID:              7,
		RunID:               "run_no_repeat",
		Messages:            messages,
		ExistingSnapshot:    repo.latest,
		PromptTokenEstimate: 128,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if snapshot != nil || repo.created != nil {
		t.Fatalf("expected covered raw history not to retrigger compaction, got %#v", snapshot)
	}
}

func TestMaybeCompactConversationRollsForwardFromPartialBoundaryWindow(t *testing.T) {
	fullPath := []domainconversation.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: "old user"},
		{ID: 2, PublicID: "m2", ParentMessageID: uintPtr(1), Role: "assistant", Content: "old assistant"},
		{ID: 3, PublicID: "m3", ParentMessageID: uintPtr(2), Role: "user", Content: "new user"},
		{ID: 4, PublicID: "m4", ParentMessageID: uintPtr(3), Role: "assistant", Content: "new assistant"},
		{ID: 5, PublicID: "m5", ParentMessageID: uintPtr(4), Role: "user", Content: "latest user"},
		{ID: 6, PublicID: "m6", ParentMessageID: uintPtr(5), Role: "assistant", Content: "latest assistant"},
	}
	repo := &compactRepositoryStub{
		latest: &domainconversation.ContextSnapshot{
			SummaryText:           "previous summary",
			ToTurn:                1,
			CoveredUntilMessageID: 2,
			CoveredUntilPublicID:  "m2",
			CoveredMessageCount:   2,
			CoveragePathHash:      CoveragePathHash(fullPath[:2]),
		},
	}
	svc := newTestService(config.Config{
		ContextCompactEnabled:           true,
		ContextMaxTurns:                 1,
		ContextCompactPreserve:          1,
		ContextCompactHighlightsPerRole: 4,
		ContextCompactSnippetChars:      200,
	}, repo, nil)

	partialWindow := fullPath[1:]
	snapshot, err := svc.MaybeCompactConversation(t.Context(), MaybeCompactConversationInput{
		ConversationID: 9,
		UserID:         7,
		RunID:          "run_partial",
		Messages:       partialWindow,
	})
	if err != nil {
		t.Fatalf("expected compaction to succeed, got %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected new snapshot")
	}
	if snapshot.CoveredUntilMessageID != 4 || snapshot.CoveredMessageCount != 4 {
		t.Fatalf("expected snapshot to roll forward to message 4, got %#v", snapshot)
	}
	if snapshot.CoveragePathHash != CoveragePathHash(fullPath[:4]) {
		t.Fatal("expected partial-window snapshot hash to match full covered path")
	}
	if !strings.Contains(snapshot.SummaryText, "previous summary") {
		t.Fatalf("expected previous summary to be carried forward, got %q", snapshot.SummaryText)
	}
	if strings.Contains(snapshot.SummaryText, "old user") || strings.Contains(snapshot.SummaryText, "old assistant") {
		t.Fatalf("expected already-covered messages to stay out of new summary source, got %q", snapshot.SummaryText)
	}
	if !strings.Contains(snapshot.SummaryText, "new user") || !strings.Contains(snapshot.SummaryText, "new assistant") {
		t.Fatalf("expected newly covered messages in summary, got %q", snapshot.SummaryText)
	}
}

func TestMaybeCompactConversationUsesPromptTokenEstimateForTokenTrigger(t *testing.T) {
	messages := []domainconversation.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: "small user"},
		{ID: 2, PublicID: "m2", ParentMessageID: uintPtr(1), Role: "assistant", Content: "small assistant"},
		{ID: 3, PublicID: "m3", ParentMessageID: uintPtr(2), Role: "user", Content: "latest user"},
		{ID: 4, PublicID: "m4", ParentMessageID: uintPtr(3), Role: "assistant", Content: "latest assistant"},
	}
	repo := &compactRepositoryStub{}
	svc := newTestService(config.Config{
		ContextCompactEnabled:           true,
		ContextMaxTurns:                 0,
		ContextCompactTriggerPercent:    10,
		ContextCompactPreserve:          1,
		ContextCompactHighlightsPerRole: 4,
		ContextCompactSnippetChars:      200,
	}, repo, nil)

	snapshot, err := svc.MaybeCompactConversation(t.Context(), MaybeCompactConversationInput{
		ConversationID:      9,
		UserID:              7,
		RunID:               "run_prompt_tokens",
		Messages:            messages,
		PromptTokenEstimate: 20_000,
	})
	if err != nil {
		t.Fatalf("expected compaction to succeed, got %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected prompt token estimate to trigger compaction")
	}
	if snapshot.Strategy != "token_cap" {
		t.Fatalf("expected token_cap strategy, got %q", snapshot.Strategy)
	}
}

func TestMaybeCompactConversationReducesPreserveWindowNearTokenLimit(t *testing.T) {
	messages := []domainconversation.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: "small user"},
		{ID: 2, PublicID: "m2", ParentMessageID: uintPtr(1), Role: "assistant", Content: "small assistant"},
		{ID: 3, PublicID: "m3", ParentMessageID: uintPtr(2), Role: "user", Content: "latest user"},
		{ID: 4, PublicID: "m4", ParentMessageID: uintPtr(3), Role: "assistant", Content: "latest assistant"},
	}
	repo := &compactRepositoryStub{}
	svc := newTestService(config.Config{
		ContextCompactEnabled:        true,
		ContextCompactTriggerPercent: 10,
		ContextCompactPreserve:       8,
	}, repo, nil)

	snapshot, err := svc.MaybeCompactConversation(t.Context(), MaybeCompactConversationInput{
		ConversationID:      9,
		UserID:              7,
		RunID:               "run_preserve_all",
		Messages:            messages,
		PromptTokenEstimate: 20_000,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected compaction to reduce the preserve window instead of silently truncating history")
	}
	if snapshot.ToTurn != 1 || repo.created == nil {
		t.Fatalf("unexpected compacted range: %#v", snapshot)
	}
}

func TestMaybeCompactConversationUsesConfiguredBudgetPercentage(t *testing.T) {
	messages := []domainconversation.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: "old user"},
		{ID: 2, PublicID: "m2", ParentMessageID: uintPtr(1), Role: "assistant", Content: "old assistant"},
		{ID: 3, PublicID: "m3", ParentMessageID: uintPtr(2), Role: "user", Content: "latest user"},
		{ID: 4, PublicID: "m4", ParentMessageID: uintPtr(3), Role: "assistant", Content: "latest assistant"},
	}
	repo := &compactRepositoryStub{}
	svc := newTestService(config.Config{
		ContextCompactEnabled:        true,
		ContextCompactTriggerPercent: 80,
		ContextCompactPreserve:       1,
	}, repo, nil)

	snapshot, err := svc.MaybeCompactConversation(t.Context(), MaybeCompactConversationInput{
		ConversationID:      9,
		UserID:              7,
		RunID:               "run_model_budget",
		Messages:            messages,
		PromptTokenEstimate: 100_000,
		ContextModelName:    "unknown-128k-model",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if snapshot == nil || snapshot.Strategy != "token_cap" {
		t.Fatalf("expected model-aware token compaction, got %#v", snapshot)
	}
}

func TestMaybeCompactConversationUsesConfiguredFallbackWindow(t *testing.T) {
	messages := []domainconversation.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: "old user"},
		{ID: 2, PublicID: "m2", ParentMessageID: uintPtr(1), Role: "assistant", Content: "old assistant"},
		{ID: 3, PublicID: "m3", ParentMessageID: uintPtr(2), Role: "user", Content: "latest user"},
		{ID: 4, PublicID: "m4", ParentMessageID: uintPtr(3), Role: "assistant", Content: "latest assistant"},
	}
	repo := &compactRepositoryStub{}
	svc := newTestService(config.Config{
		ContextCompactEnabled:        true,
		ContextWindowFallbackTokens:  256_000,
		ContextCompactTriggerPercent: 80,
		ContextCompactPreserve:       1,
	}, repo, nil)

	snapshot, err := svc.MaybeCompactConversation(t.Context(), MaybeCompactConversationInput{
		ConversationID:      9,
		UserID:              7,
		RunID:               "run_configured_fallback",
		Messages:            messages,
		PromptTokenEstimate: 100_000,
		ContextModelName:    "enterprise-private-v2",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if snapshot != nil || repo.created != nil {
		t.Fatalf("expected 100k tokens to stay below 80%% of the configured 256k fallback, got %#v", snapshot)
	}
}

func TestMaybeCompactConversationDoesNotUseLegacyFixedThresholdForLargeModel(t *testing.T) {
	messages := []domainconversation.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: "old user"},
		{ID: 2, PublicID: "m2", ParentMessageID: uintPtr(1), Role: "assistant", Content: "old assistant"},
		{ID: 3, PublicID: "m3", ParentMessageID: uintPtr(2), Role: "user", Content: "latest user"},
		{ID: 4, PublicID: "m4", ParentMessageID: uintPtr(3), Role: "assistant", Content: "latest assistant"},
	}
	repo := &compactRepositoryStub{}
	svc := newTestService(config.Config{
		ContextCompactEnabled:        true,
		ContextCompactTriggerPercent: 80,
		ContextCompactPreserve:       1,
	}, repo, nil)

	snapshot, err := svc.MaybeCompactConversation(t.Context(), MaybeCompactConversationInput{
		ConversationID:      9,
		UserID:              7,
		RunID:               "run_large_model_budget",
		Messages:            messages,
		PromptTokenEstimate: 100_000,
		ContextModelName:    "custom-large-model",
		CapabilitiesJSON:    `{"contextWindow":1000000,"maxOutputTokens":8192}`,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if snapshot != nil || repo.created != nil {
		t.Fatalf("expected no compaction below the large model's percentage threshold, got %#v", snapshot)
	}
}

func TestContextBudgetExceededUsesSelectedModelCapabilities(t *testing.T) {
	svc := newTestService(config.Config{}, nil, nil)
	messages := []domainconversation.Message{{Role: "user", Content: strings.Repeat("x", 40_000)}}
	if svc.ContextBudgetExceeded(MaybeCompactConversationInput{
		Messages:         messages,
		ContextModelName: "custom-model",
		CapabilitiesJSON: `{"contextWindow":8192,"maxOutputTokens":1024}`,
	}) == false {
		t.Fatal("expected oversized branch to cross the configured model budget")
	}
	if svc.ContextBudgetExceeded(MaybeCompactConversationInput{
		Messages:         messages,
		ContextModelName: "custom-model",
		CapabilitiesJSON: `{"contextWindow":131072,"maxOutputTokens":4096}`,
	}) {
		t.Fatal("expected the same branch to fit a larger configured model budget")
	}
}

func TestContextBudgetExceededTreatsPromptScopeEstimateAsAuthoritative(t *testing.T) {
	svc := newTestService(config.Config{}, nil, nil)
	messages := []domainconversation.Message{{Role: "user", Content: strings.Repeat("covered", 40_000)}}
	if svc.ContextBudgetExceeded(MaybeCompactConversationInput{
		Messages:            messages,
		PromptTokenEstimate: 512,
		ContextModelName:    "custom-model",
		CapabilitiesJSON:    `{"contextWindow":8192,"maxOutputTokens":1024}`,
	}) {
		t.Fatal("expected the active prompt estimate to take precedence over covered raw history")
	}
}

func TestMaybeCompactConversationForceUsesHardBudgetStrategy(t *testing.T) {
	repo := &compactRepositoryStub{}
	svc := newTestService(config.Config{
		ContextCompactEnabled:  true,
		ContextCompactPreserve: 1,
	}, repo, nil)
	messages := []domainconversation.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: "first"},
		{ID: 2, PublicID: "m2", ParentMessageID: uintPtr(1), Role: "assistant", Content: "first reply"},
		{ID: 3, PublicID: "m3", ParentMessageID: uintPtr(2), Role: "user", Content: "second"},
		{ID: 4, PublicID: "m4", ParentMessageID: uintPtr(3), Role: "assistant", Content: "second reply"},
		{ID: 5, PublicID: "m5", ParentMessageID: uintPtr(4), Role: "user", Content: "third"},
		{ID: 6, PublicID: "m6", ParentMessageID: uintPtr(5), Role: "assistant", Content: "third reply"},
	}

	snapshot, err := svc.MaybeCompactConversation(t.Context(), MaybeCompactConversationInput{
		ConversationID:      1,
		UserID:              1,
		RunID:               "run-hard-budget",
		Messages:            messages,
		PromptTokenEstimate: 8000,
		ContextModelName:    "custom-model",
		CapabilitiesJSON:    `{"contextWindow":8192,"maxOutputTokens":1024}`,
		Force:               true,
	})
	if err != nil {
		t.Fatalf("MaybeCompactConversation() error = %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected a hard-budget snapshot")
	}
	if snapshot.Strategy != "hard_budget" {
		t.Fatalf("strategy = %q, want hard_budget", snapshot.Strategy)
	}
	if repo.created == nil || repo.created.CoveredUntilMessageID != 4 {
		t.Fatalf("expected the two oldest turns to be covered, got %#v", repo.created)
	}
}

func TestMaybeCompactConversationLogsPersistenceFailure(t *testing.T) {
	createErr := errors.New("persist snapshot")
	repo := &compactRepositoryStub{createErr: createErr}
	core, logs := observer.New(zap.ErrorLevel)
	svc := newTestService(config.Config{
		ContextCompactEnabled:  true,
		ContextMaxTurns:        1,
		ContextCompactPreserve: 1,
	}, repo, zap.New(core))
	messages := []domainconversation.Message{
		{ID: 1, PublicID: "m1", Role: "user", Content: "first"},
		{ID: 2, PublicID: "m2", ParentMessageID: uintPtr(1), Role: "assistant", Content: "first reply"},
		{ID: 3, PublicID: "m3", ParentMessageID: uintPtr(2), Role: "user", Content: "second"},
		{ID: 4, PublicID: "m4", ParentMessageID: uintPtr(3), Role: "assistant", Content: "second reply"},
	}

	_, err := svc.MaybeCompactConversation(t.Context(), MaybeCompactConversationInput{
		ConversationID: 1,
		RunID:          "run-failed",
		Messages:       messages,
	})
	if !errors.Is(err, createErr) {
		t.Fatalf("expected persistence error, got %v", err)
	}
	entries := logs.FilterMessage("context_compaction_failed").All()
	if len(entries) != 1 {
		t.Fatalf("expected one canonical failure log, got %d", len(entries))
	}
	if stage := entries[0].ContextMap()["stage"]; stage != "create_snapshot" {
		t.Fatalf("unexpected failure stage: %#v", stage)
	}
}

func TestBuildCompactionSummaryLiteFallbackKeepsPreviousSummary(t *testing.T) {
	svc := newTestService(config.Config{CompactLLMEnabled: true}, nil, nil)
	type summarizerCall struct {
		messages []domainconversation.Message
		prompt   string
	}
	var calls []summarizerCall
	svc.SetLLMSummarizer(func(ctx context.Context, platformModelName string, messages []domainconversation.Message, prompt string) (string, error) {
		cloned := append([]domainconversation.Message(nil), messages...)
		calls = append(calls, summarizerCall{
			messages: cloned,
			prompt:   prompt,
		})
		if len(calls) == 1 {
			return "", errors.New("force full summary fallback")
		}
		return "merged summary", nil
	})

	summary := svc.buildCompactionSummary(t.Context(), compactionSummaryInput{
		Messages: []domainconversation.Message{
			{ID: 3, Role: "user", Content: "new user"},
			{ID: 4, Role: "assistant", Content: "new assistant"},
		},
		PreviousSummary:   "previous summary",
		Strategy:          "turn_cap",
		FromTurn:          1,
		ToTurn:            2,
		PreserveTurns:     1,
		PlatformModelName: "model",
	})

	if summary != "merged summary" {
		t.Fatalf("expected lite LLM summary, got %q", summary)
	}
	if len(calls) != 2 {
		t.Fatalf("expected full and lite summarizer calls, got %d", len(calls))
	}
	if len(calls[0].messages) == 0 || len(calls[1].messages) == 0 {
		t.Fatalf("expected LLM messages, got %#v", calls)
	}
	if calls[0].messages[0].Role != "user" || calls[1].messages[0].Role != "user" {
		t.Fatalf("expected previous summary to be carried as source material, got %#v", calls)
	}
	if !strings.Contains(calls[0].messages[0].Content, "previous summary") || !strings.Contains(calls[1].messages[0].Content, "previous summary") {
		t.Fatalf("expected both LLM calls to carry previous summary, got %#v", calls)
	}
	if !strings.Contains(calls[1].prompt, "standalone rolling summary") {
		t.Fatalf("expected lite fallback to require rolling summary, got %q", calls[1].prompt)
	}
	if !strings.Contains(calls[1].prompt, "untrusted source material") {
		t.Fatalf("expected lite fallback to keep source material untrusted, got %q", calls[1].prompt)
	}
}

func uintPtr(value uint) *uint {
	return &value
}

type compactRepositoryStub struct {
	mu          sync.Mutex
	latest      *domainconversation.ContextSnapshot
	created     *domainconversation.ContextSnapshot
	createCount int
	compactedAt time.Time
	createErr   error
}

func (r *compactRepositoryStub) CreateContextSnapshot(ctx context.Context, item *domainconversation.ContextSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	if item == nil {
		return repository.ErrInvalidInput
	}
	cloned := *item
	cloned.ID = 99
	r.created = &cloned
	r.latest = &cloned
	r.createCount++
	*item = cloned
	return nil
}

func (r *compactRepositoryStub) GetContextSnapshotByRunID(ctx context.Context, runID string) (*domainconversation.ContextSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.created != nil && r.created.RunID == runID {
		return r.created, nil
	}
	return nil, repository.ErrNotFound
}

func (r *compactRepositoryStub) GetLatestContextSnapshot(ctx context.Context, conversationID uint) (*domainconversation.ContextSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latest == nil {
		return nil, repository.ErrNotFound
	}
	item := *r.latest
	return &item, nil
}

func (r *compactRepositoryStub) UpdateConversationCompactedAt(ctx context.Context, conversationID uint, compactedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.compactedAt = compactedAt
	return nil
}
