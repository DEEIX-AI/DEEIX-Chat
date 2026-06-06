package memory

import (
	"context"
	"testing"

	domainmemory "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/memory"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/sqlitevec"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLiteVectorStoreSearchesAndDeletesUserMemories(t *testing.T) {
	db := openMemorySQLiteVectorTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	target := &domainmemory.UserMemory{UserID: 1, MemoryKey: "favorite_topic", Value: "likes databases", Scope: "global", UpdatedBy: "system"}
	other := &domainmemory.UserMemory{UserID: 1, MemoryKey: "favorite_color", Value: "likes green", Scope: "global", UpdatedBy: "system"}
	if err := repo.UpsertUserMemory(ctx, target); err != nil {
		t.Fatalf("UpsertUserMemory(target) error = %v", err)
	}
	if err := repo.UpsertUserMemory(ctx, other); err != nil {
		t.Fatalf("UpsertUserMemory(other) error = %v", err)
	}
	if err := repo.UpsertUserMemoryEmbedding(ctx, 1, "favorite_topic", "likes databases", []float32{1, 0, 0}); err != nil {
		t.Fatalf("UpsertUserMemoryEmbedding(target) error = %v", err)
	}
	if err := repo.UpsertUserMemoryEmbedding(ctx, 1, "favorite_color", "likes green", []float32{0, 1, 0}); err != nil {
		t.Fatalf("UpsertUserMemoryEmbedding(other) error = %v", err)
	}

	results, err := repo.SearchUserMemoriesByEmbedding(ctx, 1, []float32{1, 0, 0}, 2, 0)
	if err != nil {
		t.Fatalf("SearchUserMemoriesByEmbedding() error = %v", err)
	}
	if len(results) == 0 || results[0].MemoryKey != "favorite_topic" {
		t.Fatalf("expected nearest memory first, got %#v", results)
	}

	target.Value = "likes vector databases"
	if err := repo.UpsertUserMemory(ctx, target); err != nil {
		t.Fatalf("UpsertUserMemory(update target) error = %v", err)
	}
	results, err = repo.SearchUserMemoriesByEmbedding(ctx, 1, []float32{1, 0, 0}, 2, 0.5)
	if err != nil {
		t.Fatalf("SearchUserMemoriesByEmbedding(after update) error = %v", err)
	}
	for _, item := range results {
		if item.MemoryKey == "favorite_topic" {
			t.Fatalf("expected stale target memory vector to be cleared, got %#v", results)
		}
	}
	if err := repo.UpsertUserMemoryEmbedding(ctx, 1, "favorite_topic", "likes vector databases", []float32{1, 0, 0}); err != nil {
		t.Fatalf("UpsertUserMemoryEmbedding(updated target) error = %v", err)
	}
	results, err = repo.SearchUserMemoriesByEmbedding(ctx, 1, []float32{1, 0, 0}, 2, 0.5)
	if err != nil {
		t.Fatalf("SearchUserMemoriesByEmbedding(after reembed) error = %v", err)
	}
	if len(results) == 0 || results[0].Value != "likes vector databases" {
		t.Fatalf("expected updated memory after reembedding, got %#v", results)
	}

	if err := repo.DeleteUserMemory(ctx, 1, "favorite_topic"); err != nil {
		t.Fatalf("DeleteUserMemory() error = %v", err)
	}
	results, err = repo.SearchUserMemoriesByEmbedding(ctx, 1, []float32{1, 0, 0}, 2, 0)
	if err != nil {
		t.Fatalf("SearchUserMemoriesByEmbedding(after delete) error = %v", err)
	}
	if len(results) != 1 || results[0].MemoryKey != "favorite_color" {
		t.Fatalf("expected deleted memory vector to be removed, got %#v", results)
	}
}

func openMemorySQLiteVectorTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	sqlitevec.Register()
	db, err := gorm.Open(sqlite.Open("file:memory_vector?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&model.UserMemory{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	if err := sqlitevec.Migrate(db); err != nil {
		t.Fatalf("migrate sqlite vectors: %v", err)
	}
	return db
}
