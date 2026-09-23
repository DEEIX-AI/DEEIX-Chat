package db

import (
	"strings"
	"testing"
)

func TestPostgresHostIsNile(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want bool
	}{
		{name: "regional nile host", dsn: "postgres://user:secret@us-west-2.db.thenile.dev:5432/deeix_pgdb?sslmode=require", want: true},
		{name: "uppercase host", dsn: "postgres://user:secret@US-WEST-2.DB.THENILE.DEV:5432/deeix_pgdb", want: true},
		{name: "keyword dsn", dsn: "host=eu-central-1.db.thenile.dev user=user password=secret dbname=deeix_pgdb port=5432", want: true},
		{name: "apex", dsn: "postgres://user:secret@thenile.dev:5432/deeix_pgdb", want: true},
		{name: "local database named nile", dsn: "postgres://user:secret@127.0.0.1:5432/nile", want: false},
		{name: "host merely contains the word", dsn: "postgres://user:secret@nile.example.com:5432/deeix", want: false},
		{name: "keyword local", dsn: "host=127.0.0.1 user=deeix_chat password=secret dbname=nile port=5432", want: false},
		{name: "empty", dsn: "", want: false},
		{name: "not a dsn", dsn: "://", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := postgresHostIsNile(tt.dsn); got != tt.want {
				t.Fatalf("postgresHostIsNile(%q) = %v, want %v", tt.dsn, got, tt.want)
			}
		})
	}
}

func TestRewriteNileConstraintQueryUsesPgConstraint(t *testing.T) {
	nameSQL := "SELECT constraint_name FROM information_schema.table_constraints tc JOIN information_schema.constraint_column_usage AS ccu USING (constraint_schema, constraint_catalog, table_name, constraint_name) JOIN information_schema.columns AS c ON c.table_schema = tc.constraint_schema AND tc.table_name = c.table_name AND ccu.column_name = c.column_name WHERE constraint_type IN ('PRIMARY KEY', 'UNIQUE') AND c.table_catalog = $1 AND c.table_schema = CURRENT_SCHEMA() AND c.table_name = $2 AND constraint_type = $3"
	got, vars, ok := rewriteNileConstraintQuery(nameSQL, []interface{}{"nile", "identity_users", "UNIQUE"})
	if !ok {
		t.Fatal("expected the unique-constraint name query to be rewritten")
	}
	if strings.Contains(got, "information_schema") || strings.Contains(got, "pg_index") {
		t.Fatalf("rewritten query is not the constraint lookup: %s", got)
	}
	if !strings.Contains(got, "pg_constraint") || !strings.Contains(got, "con.contype = 'u'") || !strings.Contains(got, "format('%I.%I', current_schema(), $1::text)::regclass") {
		t.Fatalf("unexpected unique-constraint query: %s", got)
	}
	if len(vars) != 1 || vars[0] != "identity_users" {
		t.Fatalf("vars = %#v", vars)
	}

	columnSQL := "SELECT c.column_name, constraint_name, constraint_type FROM information_schema.table_constraints tc JOIN information_schema.constraint_column_usage AS ccu USING (constraint_schema, constraint_catalog, table_name, constraint_name) JOIN information_schema.columns AS c ON c.table_schema = tc.constraint_schema AND tc.table_name = c.table_name AND ccu.column_name = c.column_name WHERE constraint_type IN ('PRIMARY KEY', 'UNIQUE') AND c.table_catalog = $1 AND c.table_schema = $2 AND c.table_name = $3"
	got, vars, ok = rewriteNileConstraintQuery(columnSQL, []interface{}{"deeix_pgdb", "public", "chat_messages"})
	if !ok {
		t.Fatal("expected the constraint column query to be rewritten")
	}
	if !strings.Contains(got, "WHEN 'p' THEN 'PRIMARY KEY'") || !strings.Contains(got, "WHEN 'u' THEN 'UNIQUE'") {
		t.Fatalf("constraint types GORM reads were not preserved: %s", got)
	}
	if !strings.Contains(got, "format('%I.%I', $1::text, $2::text)::regclass") || strings.Contains(got, "pg_index") || strings.Contains(got, "information_schema") {
		t.Fatalf("unexpected constraint column query: %s", got)
	}
	if len(vars) != 2 || vars[0] != "public" || vars[1] != "chat_messages" {
		t.Fatalf("vars = %#v", vars)
	}
}

func TestRewriteNileConstraintQueryLeavesOtherCatalogSQL(t *testing.T) {
	others := []string{
		"SELECT c.column_name, c.is_nullable = 'YES', c.udt_name FROM information_schema.columns AS c WHERE table_catalog = $1 AND table_schema = CURRENT_SCHEMA() AND table_name = $2",
		"SELECT count(*) FROM INFORMATION_SCHEMA.table_constraints WHERE table_schema = $1 AND table_name = $2 AND constraint_name = $3",
		"SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = $2",
		"SELECT 1",
	}
	for _, sql := range others {
		if _, _, ok := rewriteNileConstraintQuery(sql, []interface{}{"nile", "public", "identity_users"}); ok {
			t.Fatalf("unrelated SQL was rewritten: %s", sql)
		}
	}
}

func TestRewriteNileSlowColumnQueries(t *testing.T) {
	formatSQL := "SELECT a.attname as column_name, format_type(a.atttypid, a.atttypmod) AS data_type FROM pg_attribute a JOIN pg_class b ON a.attrelid = b.oid AND relnamespace = (SELECT oid FROM pg_catalog.pg_namespace WHERE nspname = CURRENT_SCHEMA()) WHERE a.attnum > 0 AND NOT a.attisdropped AND b.relname = $1"
	got, vars, ok := rewriteNileConstraintQuery(formatSQL, []interface{}{"chat_messages"})
	if !ok {
		t.Fatal("expected format_type query to be rewritten")
	}
	if !strings.Contains(got, "format_type(a.atttypid, a.atttypmod)") || !strings.Contains(got, "format('%I.%I', current_schema(), $1::text)::regclass") || strings.Contains(got, "pg_class b") {
		t.Fatalf("unexpected format_type query: %s", got)
	}
	if len(vars) != 1 || vars[0] != "chat_messages" {
		t.Fatalf("vars = %#v", vars)
	}

	columnSQL := "SELECT c.column_name, c.is_nullable = 'YES', c.udt_name, c.character_maximum_length, c.numeric_precision, c.numeric_precision_radix, c.numeric_scale, c.datetime_precision, 8 * typlen, c.column_default, pd.description, c.identity_increment FROM information_schema.columns AS c JOIN pg_type AS pgt ON c.udt_name = pgt.typname where table_catalog = $1 AND table_schema = CURRENT_SCHEMA() AND table_name = $2"
	got, vars, ok = rewriteNileConstraintQuery(columnSQL, []interface{}{"nile", "identity_users"})
	if !ok {
		t.Fatal("expected column metadata query to be rewritten")
	}
	for _, piece := range []string{
		"NOT a.attnotnull",
		"information_schema._pg_char_max_length(a.atttypid, a.atttypmod)",
		"8 * t.typlen",
		"pg_get_expr(ad.adbin, ad.adrelid)",
		"col_description(a.attrelid, a.attnum)",
		"NULL::text",
	} {
		if !strings.Contains(got, piece) {
			t.Fatalf("column query missing %s: %s", piece, got)
		}
	}
	if strings.Contains(got, "information_schema.columns") {
		t.Fatalf("column query still reads the slow view: %s", got)
	}
	if len(vars) != 1 || vars[0] != "identity_users" {
		t.Fatalf("vars = %#v", vars)
	}
}
