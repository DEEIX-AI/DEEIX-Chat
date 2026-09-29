package logcleanup

import (
	"context"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDeleteBeforeRemovesModerationEventsOlderThanCutoff(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:cleanup_moderation?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err = db.AutoMigrate(&models.ContentModerationEvent{}); err != nil {
		t.Fatalf("migrate moderation events: %v", err)
	}

	cutoff := time.Now()
	events := []models.ContentModerationEvent{
		{PublicID: "old", UserID: 1},
		{PublicID: "new", UserID: 1},
	}
	if err = db.Create(&events).Error; err != nil {
		t.Fatalf("create events: %v", err)
	}
	if err = db.Model(&models.ContentModerationEvent{}).Where("public_id = ?", "old").Update("created_at", cutoff.Add(-time.Hour)).Error; err != nil {
		t.Fatalf("age event: %v", err)
	}
	if err = db.Model(&models.ContentModerationEvent{}).Where("public_id = ?", "new").Update("created_at", cutoff.Add(time.Hour)).Error; err != nil {
		t.Fatalf("age event: %v", err)
	}

	deletedCount, err := NewRepo(db).DeleteBefore(context.Background(), "moderation", cutoff)
	if err != nil {
		t.Fatalf("delete before: %v", err)
	}
	if deletedCount != 1 {
		t.Fatalf("expected 1 deleted event, got %d", deletedCount)
	}
	var remaining []models.ContentModerationEvent
	if err = db.Unscoped().Find(&remaining).Error; err != nil {
		t.Fatalf("list remaining: %v", err)
	}
	if len(remaining) != 1 || remaining[0].PublicID != "new" {
		t.Fatalf("expected only the newer event to remain, got %+v", remaining)
	}
}
