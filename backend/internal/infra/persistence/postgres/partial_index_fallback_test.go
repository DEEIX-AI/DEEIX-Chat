package db

import (
	"errors"
	"strings"
	"testing"
)

func TestPartialUniqueIndexFallbackOnlyForRejectedAndPredicates(t *testing.T) {
	nileErr := errors.New("ERROR: unsupported element in index WHERE clause (SQLSTATE 0A000)")
	cases := []struct {
		name      string
		statement string
		want      string
	}{
		{
			name: "active file",
			statement: `CREATE UNIQUE INDEX IF NOT EXISTS uk_file_objects_active_user_content
				ON "file_objects" ("user_id", "sha256", "size_bytes")
				WHERE status = 'active' AND deleted_at IS NULL AND sha256 <> ''`,
			want: "uk_file_objects_active_user_content",
		},
		{
			name: "billing usage ref",
			statement: `CREATE UNIQUE INDEX IF NOT EXISTS idx_billing_balance_transactions_usage_ref
				ON "billing_balance_transactions" ("user_id", "type", "ref_no")
				WHERE ref_no <> '' AND type IN ('usage_reserve', 'usage_refund')`,
			want: "idx_billing_balance_transactions_usage_ref",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fallback, indexName, ok := partialUniqueIndexFallback(tt.statement, nileErr)
			if !ok || indexName != tt.want {
				t.Fatalf("expected fallback for %s, ok=%v name=%s", tt.want, ok, indexName)
			}
			if strings.Contains(fallback, " WHERE ") || strings.Contains(fallback, " AND ") {
				t.Fatalf("fallback still has a partial predicate: %s", fallback)
			}
			if !strings.Contains(fallback, "NULLIF") {
				t.Fatalf("unexpected fallback: %s", fallback)
			}
		})
	}

	statement := cases[0].statement
	if _, _, ok := partialUniqueIndexFallback(statement, errors.New("duplicate key")); ok {
		t.Fatal("unrelated index errors must remain fatal")
	}
	other := `CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_conversation_projects_public_id ON "chat_conversation_projects" ("public_id") WHERE deleted_at IS NULL`
	if _, _, ok := partialUniqueIndexFallback(other, nileErr); ok {
		t.Fatal("single-predicate indexes must keep their original statements")
	}
}
