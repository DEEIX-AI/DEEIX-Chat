package sqlite

import (
	"path/filepath"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
)

func TestNewMigratesAndSeedsSQLite(t *testing.T) {
	db, err := New(config.Config{
		Env:                 "dev",
		SQLitePath:          filepath.Join(t.TempDir(), "deeix.db"),
		SQLiteMaxOpenConns:  1,
		SQLiteBusyTimeoutMS: 5000,
		SQLiteCacheSizeKB:   20480,
		SQLiteMmapSizeBytes: 268435456,
		SQLiteSynchronous:   "NORMAL",
		SQLiteTempStore:     "MEMORY",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	if !db.Migrator().HasTable(&model.User{}) {
		t.Fatal("expected users table to be migrated")
	}
	var planCount int64
	if err := db.Model(&model.BillingPlan{}).Count(&planCount).Error; err != nil {
		t.Fatalf("count billing plans: %v", err)
	}
	if planCount == 0 {
		t.Fatal("expected billing catalog seed data")
	}
}
