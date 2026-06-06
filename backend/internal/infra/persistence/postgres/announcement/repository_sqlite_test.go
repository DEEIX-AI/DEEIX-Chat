package announcement

import (
	"context"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDismissAnnouncementTodaySQLiteUsesDeclaredUniqueIndex(t *testing.T) {
	db := openAnnouncementSQLiteTestDB(t)
	if !db.Migrator().HasIndex(&model.AnnouncementUserState{}, "idx_announcement_user_states_version") {
		t.Fatal("expected announcement user state version unique index")
	}

	now := time.Date(2026, 6, 6, 16, 42, 0, 0, time.UTC)
	item := model.Announcement{
		Title:           "notice",
		ContentMarkdown: "content",
		Status:          "active",
		Type:            "info",
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create announcement: %v", err)
	}

	repo := NewRepo(db)
	if err := repo.DismissAnnouncementToday(context.Background(), 1, item.ID, item.UpdatedAt, now, now.Add(8*time.Hour)); err != nil {
		t.Fatalf("dismiss announcement first time: %v", err)
	}
	if err := repo.DismissAnnouncementToday(context.Background(), 1, item.ID, item.UpdatedAt, now, now.Add(12*time.Hour)); err != nil {
		t.Fatalf("dismiss announcement second time: %v", err)
	}

	var count int64
	if err := db.Model(&model.AnnouncementUserState{}).Count(&count).Error; err != nil {
		t.Fatalf("count states: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one upserted state, got %d", count)
	}
}

func openAnnouncementSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:announcement_user_state?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(&model.Announcement{}, &model.AnnouncementUserState{}); err != nil {
		t.Fatalf("migrate announcement tables: %v", err)
	}
	return db
}
