package conversation

import (
	"reflect"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestExportDefaultMessagePublicIDsUsesLatestVisibleBranch(t *testing.T) {
	messages := []model.Message{
		{ID: 1, PublicID: "u1", BranchReason: "default"},
		{ID: 2, PublicID: "a1", ParentPublicID: "u1", BranchReason: "default"},
		{ID: 3, PublicID: "u2-old", ParentPublicID: "a1", BranchReason: "default"},
		{ID: 4, PublicID: "a2-old", ParentPublicID: "u2-old", BranchReason: "default"},
		{ID: 5, PublicID: "u2-new", ParentPublicID: "a1", BranchReason: "retry"},
		{ID: 6, PublicID: "a2-new", ParentPublicID: "u2-new", BranchReason: "retry"},
	}

	got := exportDefaultMessagePublicIDs(messages)
	want := []string{"u1", "a1", "u2-new", "a2-new"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected default export message ids: got %#v want %#v", got, want)
	}
}

func TestCollectExportMessageRunIDsTrimsAndDeduplicates(t *testing.T) {
	messages := []model.Message{
		{RunID: " run_1 "},
		{RunID: "run_1"},
		{RunID: ""},
		{RunID: "run_2"},
		{RunID: "run_2"},
	}

	got := collectExportMessageRunIDs(messages)
	want := []string{"run_1", "run_2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected export run ids: got %#v want %#v", got, want)
	}
}

func TestNormalizeMessagePageAllowsRestoreWindow(t *testing.T) {
	_, generalLimit := normalizePage(1, 1000)
	if generalLimit != maxPageSize {
		t.Fatalf("expected normal page limit to stay capped at %d, got %d", maxPageSize, generalLimit)
	}

	_, messageLimit := normalizeMessagePage(1, 1000)
	if messageLimit != 1000 {
		t.Fatalf("expected message page limit to allow 1000, got %d", messageLimit)
	}
}

func TestNormalizeRecentMessageLimitUsesMessageWindow(t *testing.T) {
	if got := normalizeRecentMessageLimit(5000); got != maxMessagePageSize {
		t.Fatalf("expected recent message limit capped at %d, got %d", maxMessagePageSize, got)
	}
}
