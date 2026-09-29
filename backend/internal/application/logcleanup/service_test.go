package logcleanup

import (
	"context"
	"errors"
	"testing"
	"time"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
)

type cleanupTestRepository struct {
	logType string
	before  time.Time
}

func (r *cleanupTestRepository) DeleteBefore(_ context.Context, logType string, before time.Time) (int64, error) {
	r.logType = logType
	r.before = before
	return 3, nil
}

type cleanupTestAuditWriter struct {
	action string
	detail any
}

func (w *cleanupTestAuditWriter) Write(_ context.Context, input appaudit.WriteInput) {
	w.action = input.Action
	w.detail = input.Detail
}

func TestCleanupNormalizesTypeAndAudits(t *testing.T) {
	repo := &cleanupTestRepository{}
	writer := &cleanupTestAuditWriter{}
	before := time.Now().Add(-time.Hour)

	result, err := NewService(repo, writer).Cleanup(context.Background(), Input{Type: " Moderation ", Before: before, ActorUserID: 1})
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if repo.logType != TypeModeration || !repo.before.Equal(before) {
		t.Fatalf("repository received %q before %v", repo.logType, repo.before)
	}
	if result.DeletedCount != 3 || result.Type != TypeModeration {
		t.Fatalf("unexpected result %+v", result)
	}
	if writer.action != "admin_cleanup_logs" {
		t.Fatalf("audit action = %q", writer.action)
	}
}

func TestCleanupRejectsInvalidInput(t *testing.T) {
	service := NewService(&cleanupTestRepository{}, nil)
	cases := []struct {
		name  string
		input Input
		want  error
	}{
		{"unknown type", Input{Type: "runs", Before: time.Now().Add(-time.Hour)}, ErrInvalidType},
		{"zero before", Input{Type: TypeAudit}, ErrInvalidBefore},
		{"future before", Input{Type: TypeAudit, Before: time.Now().Add(time.Hour)}, ErrFutureBefore},
	}
	for _, tc := range cases {
		if _, err := service.Cleanup(context.Background(), tc.input); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, err, tc.want)
		}
	}
}
