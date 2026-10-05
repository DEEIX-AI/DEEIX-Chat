//go:build nopostgres

package openwebui

import (
	"errors"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// 不含 PostgreSQL 驱动的构建只支持从 SQLite 导入，PostgreSQL 来源按无效 DSN 拒绝。
func TestOpenDBRejectsPostgresWithoutDriver(t *testing.T) {
	if _, err := openDB("postgres://user:pass@localhost:5432/openwebui"); !errors.Is(err, repository.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}
