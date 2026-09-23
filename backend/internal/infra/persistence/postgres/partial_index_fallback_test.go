package db

import (
	"strings"
	"testing"
)

func TestNilePartialUniqueIndexOnlyForAndPredicates(t *testing.T) {
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
			fallback, indexName, ok := nilePartialUniqueIndex(tt.statement)
			if !ok || indexName != tt.want {
				t.Fatalf("expected Nile form for %s, ok=%v name=%s", tt.want, ok, indexName)
			}
			if strings.Contains(fallback, " WHERE ") || strings.Contains(strings.ToUpper(fallback), " AND ") {
				t.Fatalf("Nile form still has a partial predicate: %s", fallback)
			}
			if !strings.Contains(fallback, "NULLIF") || !strings.Contains(fallback, tt.want) {
				t.Fatalf("unexpected Nile form: %s", fallback)
			}
		})
	}

	other := `CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_conversation_projects_public_id ON "chat_conversation_projects" ("public_id") WHERE deleted_at IS NULL`
	if _, _, ok := nilePartialUniqueIndex(other); ok {
		t.Fatal("single-predicate indexes must keep their original statements")
	}
	plain := `CREATE INDEX IF NOT EXISTS idx_identity_sessions_refresh_rotated_at ON "identity_sessions" ("refresh_rotated_at")`
	if _, _, ok := nilePartialUniqueIndex(plain); ok {
		t.Fatal("indexes without AND must keep their original statements")
	}
}
