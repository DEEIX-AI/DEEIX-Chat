package logcleanup

import (
	"context"
	"strings"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeleteConversationRunsDeletesOnlySelectedRunEvents(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:cleanup_conversation_runs?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err = db.AutoMigrate(&model.ChatRunEvent{}); err != nil {
		t.Fatalf("migrate chat run events: %v", err)
	}

	now := time.Now()
	events := []model.ChatRunEvent{
		{RunID: "run_delete", EventScope: "trace_block", EventID: "block", StartedAt: now},
		{RunID: "run_delete", EventScope: "trace_event", EventID: "event", StartedAt: now},
		{RunID: "run_keep", EventScope: "trace_event", EventID: "keep", StartedAt: now},
	}
	if err = db.Create(&events).Error; err != nil {
		t.Fatalf("create events: %v", err)
	}

	deletedCount, err := NewRepo(db).DeleteConversationRuns(context.Background(), []string{"run_delete"})
	if err != nil {
		t.Fatalf("DeleteConversationRuns() error = %v", err)
	}
	if deletedCount != 2 {
		t.Fatalf("deleted count = %d, want 2", deletedCount)
	}

	var remaining []model.ChatRunEvent
	if err = db.Order("id ASC").Find(&remaining).Error; err != nil {
		t.Fatalf("list remaining events: %v", err)
	}
	if len(remaining) != 1 || strings.TrimSpace(remaining[0].RunID) != "run_keep" {
		t.Fatalf("remaining events = %#v, want only run_keep", remaining)
	}
}
