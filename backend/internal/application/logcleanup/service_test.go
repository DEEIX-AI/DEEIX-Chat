package logcleanup

import (
	"context"
	"testing"
	"time"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
)

type cleanupTestRepository struct {
	runIDs []string
}

func (r *cleanupTestRepository) DeleteBefore(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}

func (r *cleanupTestRepository) DeleteConversationRuns(_ context.Context, runIDs []string) (int64, error) {
	r.runIDs = append([]string(nil), runIDs...)
	return 4, nil
}

type cleanupTestAuditWriter struct {
	action string
	detail any
}

func (w *cleanupTestAuditWriter) Write(_ context.Context, input appaudit.WriteInput) {
	w.action = input.Action
	w.detail = input.Detail
}

func TestCleanupConversationRunsNormalizesAndAudits(t *testing.T) {
	repo := &cleanupTestRepository{}
	auditWriter := &cleanupTestAuditWriter{}
	service := NewService(repo, auditWriter)

	result, err := service.CleanupConversationRuns(context.Background(), ConversationRunInput{
		RunIDs: []string{" run_1 ", "run_2", "run_1"},
	})
	if err != nil {
		t.Fatalf("cleanup conversation runs: %v", err)
	}
	if result.RunCount != 2 || result.DeletedCount != 4 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(repo.runIDs) != 2 || repo.runIDs[0] != "run_1" || repo.runIDs[1] != "run_2" {
		t.Fatalf("unexpected normalized run ids: %#v", repo.runIDs)
	}
	if auditWriter.action != "admin_cleanup_conversation_runs" || auditWriter.detail == nil {
		t.Fatalf("expected cleanup audit record, got action=%q detail=%#v", auditWriter.action, auditWriter.detail)
	}
}

func TestCleanupConversationRunsRejectsInvalidInput(t *testing.T) {
	service := NewService(&cleanupTestRepository{}, nil)
	for _, runIDs := range [][]string{nil, {""}, {string(make([]byte, 65))}} {
		if _, err := service.CleanupConversationRuns(context.Background(), ConversationRunInput{RunIDs: runIDs}); err != ErrInvalidRunIDs {
			t.Fatalf("run ids %#v: expected ErrInvalidRunIDs, got %v", runIDs, err)
		}
	}
}
