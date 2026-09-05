package db

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/vectorutil"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresVectorColumnTypmodRemovalSQLPreservesNativeDimensions(t *testing.T) {
	statement := postgresVectorColumnTypmodRemovalSQL("public", "file_chunks", "embedding")
	if strings.Contains(statement, "%!") {
		t.Fatalf("vector migration SQL contains formatting errors: %s", statement)
	}
	for _, expected := range []string{
		`ALTER TABLE "public"."file_chunks"`,
		`ALTER COLUMN "embedding" TYPE vector`,
	} {
		if !strings.Contains(statement, expected) {
			t.Fatalf("vector migration SQL missing %q: %s", expected, statement)
		}
	}
	if strings.Contains(statement, "vector(4096)") || strings.Contains(statement, "array_fill") {
		t.Fatalf("vector typmod removal must not expand stored rows: %s", statement)
	}
}

func TestPostgresExtensionVersionAtLeast(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{version: "0.7.4", want: false},
		{version: "0.8.0", want: true},
		{version: "0.8.2", want: true},
		{version: "0.10.0", want: true},
		{version: "invalid", want: false},
	}
	for _, test := range tests {
		if got := postgresExtensionVersionAtLeast(test.version, 0, 8); got != test.want {
			t.Errorf("postgresExtensionVersionAtLeast(%q) = %v, want %v", test.version, got, test.want)
		}
	}
}

func TestPostgresVectorColumnTypmodRemovalSQLDoesNotTruncateVectors(t *testing.T) {
	statement := postgresVectorColumnTypmodRemovalSQL("public", "file_chunks", "embedding")
	if strings.Contains(statement, `[1:4096]`) {
		t.Fatalf("vector migration SQL must not silently truncate oversized vectors: %s", statement)
	}
}

func TestPostgresVectorIndexSQLUsesHalfVectorCandidates(t *testing.T) {
	statement := postgresVectorIndexSQL("public", "file_chunks", "embedding", "idx_file_chunks_embedding")
	for _, expected := range []string{
		`CREATE INDEX CONCURRENTLY "idx_file_chunks_embedding"`,
		`USING hnsw`,
		`vector_dims("embedding")`,
		`subvector(`,
		`::halfvec(4000)`,
		`halfvec_cosine_ops`,
	} {
		if !strings.Contains(statement, expected) {
			t.Fatalf("vector index SQL missing %q: %s", expected, statement)
		}
	}
}

func TestEnsurePostgresVectorColumnPreservesLegacyVectors(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("DEEIX_TEST_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("set DEEIX_TEST_DATABASE_DSN to run PostgreSQL vector migration integration test")
	}
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("resolve postgres database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	var vectorAvailable bool
	if err = database.Raw(`SELECT to_regtype('vector') IS NOT NULL`).Scan(&vectorAvailable).Error; err != nil {
		t.Fatalf("check pgvector extension: %v", err)
	}
	if !vectorAvailable {
		t.Skip("pgvector extension is required for PostgreSQL vector migration integration test")
	}

	schemaName := fmt.Sprintf("deeix_test_vector_migration_%d", time.Now().UnixNano())
	if err = database.Exec(`CREATE SCHEMA ` + schemaName).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() { _ = database.Exec(`DROP SCHEMA IF EXISTS ` + schemaName + ` CASCADE`).Error })
	if err = database.Exec(`SET search_path TO ` + schemaName + `, public`).Error; err != nil {
		t.Fatalf("set search path: %v", err)
	}
	if err = database.Exec(`CREATE TABLE file_chunks (id bigint PRIMARY KEY, embedding vector(1536))`).Error; err != nil {
		t.Fatalf("create legacy vector table: %v", err)
	}
	if err = database.Exec(`INSERT INTO file_chunks (id, embedding) VALUES (
		1, (ARRAY[1::real, -2::real] || array_fill(0::real, ARRAY[1534]))::vector
	)`).Error; err != nil {
		t.Fatalf("insert legacy vector: %v", err)
	}
	if err = database.Exec(`CREATE INDEX idx_file_chunks_embedding
		ON file_chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 1)`).Error; err != nil {
		t.Fatalf("create legacy vector index: %v", err)
	}

	migrateVectorColumn := func() error {
		return withPostgresVectorMigrationLock(database, func(connection *gorm.DB) error {
			return ensurePostgresVectorColumnLocked(connection, "file_chunks", "embedding", "idx_file_chunks_embedding")
		})
	}
	if err = migrateVectorColumn(); err != nil {
		t.Fatalf("migrate legacy vector column: %v", err)
	}
	if err = migrateVectorColumn(); err != nil {
		t.Fatalf("repeat legacy vector migration: %v", err)
	}

	var columnType string
	if err = database.Raw(`
		SELECT format_type(attribute.atttypid, attribute.atttypmod)
		FROM pg_attribute AS attribute
		JOIN pg_class AS relation ON relation.oid = attribute.attrelid
		JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
		WHERE namespace.nspname = current_schema()
		  AND relation.relname = 'file_chunks'
		  AND attribute.attname = 'embedding'`).Scan(&columnType).Error; err != nil {
		t.Fatalf("read migrated column type: %v", err)
	}
	expectedType := "vector"
	if columnType != expectedType {
		t.Fatalf("migrated column type = %q, want %q", columnType, expectedType)
	}

	var values struct {
		Dimensions int     `gorm:"column:dimensions"`
		First      float32 `gorm:"column:first"`
		Second     float32 `gorm:"column:second"`
	}
	if err = database.Raw(`
		SELECT vector_dims(embedding) AS dimensions,
		       (embedding::real[])[1] AS first,
		       (embedding::real[])[2] AS second
		FROM file_chunks WHERE id = 1`).Scan(&values).Error; err != nil {
		t.Fatalf("read migrated vector: %v", err)
	}
	if values.Dimensions != 1536 {
		t.Fatalf("migrated dimensions = %d, want original 1536", values.Dimensions)
	}
	if values.First != 1 || values.Second != -2 {
		t.Fatalf("migrated vector values were not preserved")
	}
	if err = database.Exec(`INSERT INTO file_chunks (id, embedding) VALUES
		(2, (ARRAY[3::real] || array_fill(0::real, ARRAY[4095]))::vector),
		(3, (ARRAY[4::real] || array_fill(0::real, ARRAY[1535]))::vector)`).Error; err != nil {
		t.Fatalf("insert mixed-dimension vectors after migration: %v", err)
	}
	var mixedDimensions struct {
		Legacy  int `gorm:"column:legacy"`
		Maximum int `gorm:"column:maximum"`
		Reduced int `gorm:"column:reduced"`
		Padded  int `gorm:"column:padded"`
	}
	paddedExpression := vectorutil.PostgresPaddedExpression("embedding")
	if err = database.Raw(fmt.Sprintf(`
		SELECT
			MAX(CASE WHEN id = 1 THEN vector_dims(embedding) END) AS legacy,
			MAX(CASE WHEN id = 2 THEN vector_dims(embedding) END) AS maximum,
			MAX(CASE WHEN id = 3 THEN vector_dims(embedding) END) AS reduced,
			MIN(vector_dims(%s)) AS padded
		FROM file_chunks`, paddedExpression)).Scan(&mixedDimensions).Error; err != nil {
		t.Fatalf("read mixed-dimension vectors: %v", err)
	}
	if mixedDimensions.Legacy != 1536 || mixedDimensions.Maximum != 4096 || mixedDimensions.Reduced != 1536 {
		t.Fatalf("stored vector dimensions changed unexpectedly: %#v", mixedDimensions)
	}
	if mixedDimensions.Padded != vectorutil.MaxDimensions {
		t.Fatalf("query boundary dimensions = %d, want %d", mixedDimensions.Padded, vectorutil.MaxDimensions)
	}
	var indexDefinition string
	if err = database.Raw(`SELECT indexdef FROM pg_indexes
		WHERE schemaname = current_schema() AND indexname = 'idx_file_chunks_embedding'`).Scan(&indexDefinition).Error; err != nil {
		t.Fatalf("check replacement index: %v", err)
	}
	if !postgresVectorIndexMatches(indexDefinition) {
		t.Fatalf("expected legacy IVFFlat index to be replaced by halfvec HNSW, got %q", indexDefinition)
	}
}
