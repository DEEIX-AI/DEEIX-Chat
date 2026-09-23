package db

import (
	"errors"
	"strings"
	"testing"
)

func TestFileObjectActiveContentIndexFallbackOnlyForNileWhereClause(t *testing.T) {
	statement := `CREATE UNIQUE INDEX IF NOT EXISTS uk_file_objects_active_user_content
		ON "file_objects" ("user_id", "sha256", "size_bytes")
		WHERE status = 'active' AND deleted_at IS NULL AND sha256 <> ''`
	fallback, ok := fileObjectActiveContentIndexFallback(statement, errors.New("ERROR: unsupported element in index WHERE clause (SQLSTATE 0A000)"))
	if !ok {
		t.Fatal("expected Nile partial-index rejection to use the expression index")
	}
	if strings.Contains(fallback, " WHERE ") || strings.Contains(fallback, " AND ") {
		t.Fatalf("fallback still has a partial predicate: %s", fallback)
	}
	if !strings.Contains(fallback, "NULLIF") || !strings.Contains(fallback, `"file_objects"`) {
		t.Fatalf("unexpected fallback: %s", fallback)
	}

	if _, ok = fileObjectActiveContentIndexFallback(statement, errors.New("duplicate key")); ok {
		t.Fatal("unrelated index errors must remain fatal")
	}
	other := `CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_conversation_projects_public_id ON "chat_conversation_projects" ("public_id") WHERE deleted_at IS NULL`
	if _, ok = fileObjectActiveContentIndexFallback(other, errors.New("unsupported element in index WHERE clause")); ok {
		t.Fatal("other indexes must keep their original statements")
	}
}
