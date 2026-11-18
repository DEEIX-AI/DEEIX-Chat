package conversation

import (
	"context"
	"errors"
	"testing"

	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
)

func snapshotForRepositoryTest(model string) domaindiscussion.Snapshot {
	return domaindiscussion.Snapshot{
		Template: domaindiscussion.Template{PublicID: "tpl", Name: "Discussion"},
		Config: domaindiscussion.TemplateConfig{
			Placeholders: []domaindiscussion.Placeholder{{Key: "p1", DefaultModel: model}},
			Rounds:       []domaindiscussion.Round{{Index: 1, Steps: []domaindiscussion.Step{{ID: "s1", Participant: "p1"}}}},
			Summary:      domaindiscussion.Summary{Participant: "p1"},
		},
		Plan: domaindiscussion.RuntimePlan{
			Rounds:  []domaindiscussion.RuntimeRound{{Index: 1, Steps: []domaindiscussion.RuntimeStep{{ID: "s1", Participant: "p1", Model: model}}}},
			Summary: domaindiscussion.RuntimeSummary{Participant: "p1", Model: model},
		},
	}
}

func TestDiscussionSnapshotRefreshCommitRollbackAndVersionConflict(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	if err := db.AutoMigrate(&models.ConversationDiscussionSnapshot{}); err != nil {
		t.Fatalf("migrate snapshots: %v", err)
	}
	repo := NewRepo(db)
	ctx := context.Background()

	current, err := repo.RefreshDiscussionSnapshot(ctx, 10, "tpl", snapshotForRepositoryTest("model-v1"), 0)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if current.Active != nil || current.Pending == nil || current.PendingVersion != 1 {
		t.Fatalf("unexpected initial pending: %#v", current)
	}
	current, committed, err := repo.CommitDiscussionSnapshot(ctx, 10, "tpl", domaindiscussion.Snapshot{}, true, 1)
	if err != nil || !committed {
		t.Fatalf("commit pending: committed=%v err=%v", committed, err)
	}
	if current.Active == nil || current.Active.Plan.Summary.Model != "model-v1" || current.Pending != nil || current.ActiveVersion != 2 {
		t.Fatalf("unexpected committed snapshot: %#v", current)
	}

	current, err = repo.RefreshDiscussionSnapshot(ctx, 10, "tpl", snapshotForRepositoryTest("model-v2"), 0)
	if err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if current.Active.Plan.Summary.Model != "model-v1" || current.Pending.Plan.Summary.Model != "model-v2" || current.PendingVersion != 3 {
		t.Fatalf("refresh changed active or wrong version: %#v", current)
	}
	if _, err = repo.RefreshDiscussionSnapshot(ctx, 10, "tpl", snapshotForRepositoryTest("model-stale"), 1); !errors.Is(err, domaindiscussion.ErrSnapshotConflict) {
		t.Fatalf("expected stale refresh conflict, got %v", err)
	}
	pending := *current.Pending
	pending.Plan.Summary.Model = "model-edited"
	if _, err = repo.UpdateDiscussionSnapshotPending(ctx, 10, "tpl", pending, 3); err != nil {
		t.Fatalf("update pending: %v", err)
	}
	if _, err = repo.RollbackDiscussionSnapshot(ctx, 10, "tpl", 2); !errors.Is(err, domaindiscussion.ErrSnapshotConflict) {
		t.Fatalf("expected stale rollback conflict, got %v", err)
	}
	current, err = repo.RollbackDiscussionSnapshot(ctx, 10, "tpl", 3)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if current.Active == nil || current.Active.Plan.Summary.Model != "model-v1" || current.Pending != nil || current.PendingVersion != 0 {
		t.Fatalf("rollback did not preserve active: %#v", current)
	}
}

func TestDiscussionSnapshotInitialCommitOnlySucceedsOnce(t *testing.T) {
	db := openConversationRepositoryTestDB(t)
	if err := db.AutoMigrate(&models.ConversationDiscussionSnapshot{}); err != nil {
		t.Fatalf("migrate snapshots: %v", err)
	}
	repo := NewRepo(db)
	ctx := context.Background()
	current, committed, err := repo.CommitDiscussionSnapshot(ctx, 20, "tpl", snapshotForRepositoryTest("model-first"), false, 0)
	if err != nil || !committed || current.ActiveVersion != 1 {
		t.Fatalf("initial commit: %#v committed=%v err=%v", current, committed, err)
	}
	current, committed, err = repo.CommitDiscussionSnapshot(ctx, 20, "tpl", snapshotForRepositoryTest("model-second"), false, 0)
	if err != nil || committed {
		t.Fatalf("second initial commit should be a no-op: committed=%v err=%v", committed, err)
	}
	if current.Active.Plan.Summary.Model != "model-first" {
		t.Fatalf("active binding was overwritten: %#v", current.Active.Plan)
	}
}
