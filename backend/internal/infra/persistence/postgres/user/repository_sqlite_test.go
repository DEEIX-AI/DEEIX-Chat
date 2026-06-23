package user

import (
	"context"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListUsersSearchesBeforePagination(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err = db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}

	users := []model.User{
		{PublicID: "u_first", Username: "first", DisplayName: "First", Email: "first@example.com", Role: "user", Status: "active"},
		{PublicID: "u_second", Username: "second", DisplayName: "Second", Email: "second@example.com", Role: "user", Status: "active"},
		{PublicID: "u_target", Username: "target", DisplayName: "Target", Email: "target@example.com", Role: "user", Status: "active"},
	}
	if err = db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}

	items, total, err := NewRepo(db).ListUsers(context.Background(), 0, 1, repository.UserListFilter{Query: "target@example.com"})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(items) != 1 || items[0].Email != "target@example.com" {
		t.Fatalf("items = %+v, want target user", items)
	}
}
