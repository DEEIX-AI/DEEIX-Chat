package db

import (
	"sync"
	"testing"

	gormschema "gorm.io/gorm/schema"
)

type schemaCommentProbe struct {
	ID   uint   `gorm:"primaryKey;comment:主键ID"`
	Name string `gorm:"size:32;comment:名称"`
}

func TestStripSchemaCommentsClearsParsedFieldComments(t *testing.T) {
	parsed, err := gormschema.Parse(&schemaCommentProbe{}, &sync.Map{}, gormschema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if parsed.LookUpField("ID").Comment == "" || parsed.LookUpField("Name").Comment == "" {
		t.Fatal("expected struct comment tags to be parsed")
	}

	stripSchemaComments(parsed)

	if parsed.LookUpField("ID").Comment != "" || parsed.LookUpField("Name").Comment != "" {
		t.Fatal("expected schema comments to be cleared")
	}
}

func TestIsSchemaCommentSQL(t *testing.T) {
	if !isSchemaCommentSQL("  comment on column \"users\".\"id\" is '主键ID'") {
		t.Fatal("expected COMMENT statement to be detected")
	}
	if isSchemaCommentSQL("ALTER TABLE \"users\" ADD COLUMN IF NOT EXISTS \"name\" text") {
		t.Fatal("expected non-comment SQL to be kept")
	}
}

func TestExecStatementsSkipsCommentsWhenDisabled(t *testing.T) {
	statements := []string{
		`ALTER TABLE "users" ADD COLUMN IF NOT EXISTS "name" text`,
		`COMMENT ON COLUMN "users"."name" IS '名称'`,
		`CREATE INDEX IF NOT EXISTS idx_users_name ON "users" ("name")`,
	}
	var executed []string
	for _, statement := range statements {
		if isSchemaCommentSQL(statement) {
			continue
		}
		executed = append(executed, statement)
	}
	if len(executed) != 2 {
		t.Fatalf("expected 2 statements after skipping comments, got %d", len(executed))
	}
}
