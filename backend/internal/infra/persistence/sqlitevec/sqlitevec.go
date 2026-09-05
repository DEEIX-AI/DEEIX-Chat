package sqlitevec

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/vectorutil"
	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"gorm.io/gorm"
)

const (
	EmbeddingDimensions = vectorutil.MaxDimensions

	FileChunkVectorTable    = "file_chunk_vectors"
	MessageChunkVectorTable = "chat_message_chunk_vectors"
	UserMemoryVectorTable   = "user_memory_vectors"
)

var (
	registerOnce              sync.Once
	vectorDimensionPattern    = regexp.MustCompile(`(?i)embedding\s+FLOAT\[(\d+)\]`)
	vectorTableMigrationSpecs = []vectorTableSpec{
		{name: FileChunkVectorTable, keyColumns: []string{"chunk_id", "user_id", "file_obj_id"}, partitionColumn: "user_id"},
		{name: MessageChunkVectorTable, keyColumns: []string{"chunk_id", "user_id", "conversation_id", "message_id"}, partitionColumn: "user_id"},
		{name: UserMemoryVectorTable, keyColumns: []string{"memory_id", "user_id"}, partitionColumn: "user_id"},
	}
)

type vectorTableSpec struct {
	name            string
	keyColumns      []string
	partitionColumn string
}

// Register loads sqlite-vec into all SQLite connections opened after this call.
func Register() {
	registerOnce.Do(func() {
		sqlite_vec.Auto()
	})
}

// Migrate creates the local vector tables used by SQLite deployments.
func Migrate(db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "sqlite" {
		return nil
	}
	var version string
	if err := db.Raw(`SELECT vec_version()`).Scan(&version).Error; err != nil {
		return fmt.Errorf("sqlite vector extension unavailable: %w", err)
	}
	for _, spec := range vectorTableMigrationSpecs {
		if err := ensureVectorTable(db, spec); err != nil {
			return err
		}
	}
	return nil
}

// Available checks whether sqlite-vec is loaded and all vector tables exist.
func Available(ctx context.Context, db *gorm.DB) (bool, error) {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "sqlite" {
		return false, nil
	}
	var version string
	if err := db.WithContext(ctx).Raw(`SELECT vec_version()`).Scan(&version).Error; err != nil {
		return false, nil
	}
	for _, spec := range vectorTableMigrationSpecs {
		dimensions, hasSignature, err := vectorTableSchema(db.WithContext(ctx), spec.name)
		if err != nil {
			return false, err
		}
		if dimensions != EmbeddingDimensions || !hasSignature {
			return false, nil
		}
	}
	return true, nil
}

// SerializeFloat32 returns the vector BLOB format accepted by sqlite-vec.
func SerializeFloat32(vector []float32) ([]byte, error) {
	aligned, err := vectorutil.AlignForStorage(vector)
	if err != nil {
		return nil, err
	}
	return sqlite_vec.SerializeFloat32(aligned)
}

func ensureVectorTable(db *gorm.DB, spec vectorTableSpec) error {
	dimensions, hasSignature, err := vectorTableSchema(db, spec.name)
	if err != nil {
		return err
	}
	if dimensions == EmbeddingDimensions && hasSignature {
		return nil
	}
	if dimensions == 0 {
		return db.Exec(createVectorTableSQL(spec)).Error
	}
	return migrateVectorTableSchema(db, spec, dimensions, hasSignature)
}

func vectorTableSchema(db *gorm.DB, table string) (int, bool, error) {
	var createSQL string
	result := db.Raw(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&createSQL)
	if result.Error != nil {
		return 0, false, result.Error
	}
	if strings.TrimSpace(createSQL) == "" {
		return 0, false, nil
	}
	match := vectorDimensionPattern.FindStringSubmatch(createSQL)
	if len(match) != 2 {
		return 0, false, fmt.Errorf("sqlite vector table %s has an unsupported schema", table)
	}
	dimensions, err := strconv.Atoi(match[1])
	if err != nil || dimensions <= 0 {
		return 0, false, fmt.Errorf("sqlite vector table %s has invalid dimensions", table)
	}
	hasSignature := strings.Contains(strings.ToLower(createSQL), "embedding_signature text partition key")
	return dimensions, hasSignature, nil
}

func createVectorTableSQL(spec vectorTableSpec) string {
	definitions := make([]string, 0, len(spec.keyColumns)+1)
	for index, column := range spec.keyColumns {
		switch {
		case index == 0:
			definitions = append(definitions, column+" INTEGER PRIMARY KEY")
		case column == spec.partitionColumn:
			definitions = append(definitions, column+" INTEGER PARTITION KEY")
		default:
			definitions = append(definitions, column+" INTEGER")
		}
	}
	definitions = append(definitions, "embedding_signature TEXT PARTITION KEY")
	definitions = append(definitions, fmt.Sprintf("embedding FLOAT[%d] distance_metric=cosine", EmbeddingDimensions))
	return fmt.Sprintf("CREATE VIRTUAL TABLE IF NOT EXISTS %s USING vec0(%s)", spec.name, strings.Join(definitions, ", "))
}

func migrateVectorTableSchema(db *gorm.DB, spec vectorTableSpec, currentDimensions int, hasSignature bool) error {
	if currentDimensions > EmbeddingDimensions {
		return fmt.Errorf(
			"sqlite vector table %s dimensions %d exceed supported maximum %d",
			spec.name,
			currentDimensions,
			EmbeddingDimensions,
		)
	}
	backupTable := spec.name + "_dimension_backup"
	backupColumns := append([]string(nil), spec.keyColumns...)
	if hasSignature {
		backupColumns = append(backupColumns, "embedding_signature")
	}
	backupColumns = append(backupColumns, "embedding")
	backupColumnList := strings.Join(backupColumns, ", ")
	resizeExpression := "vec_f32(CAST(embedding AS BLOB))"
	if currentDimensions < EmbeddingDimensions {
		resizeExpression = fmt.Sprintf(
			"vec_f32(CAST(CAST(embedding AS BLOB) || zeroblob(%d) AS BLOB))",
			(EmbeddingDimensions-currentDimensions)*4,
		)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DROP TABLE IF EXISTS " + backupTable).Error; err != nil {
			return err
		}
		if err := tx.Exec(fmt.Sprintf("CREATE TEMP TABLE %s AS SELECT %s FROM %s", backupTable, backupColumnList, spec.name)).Error; err != nil {
			return err
		}
		if err := tx.Exec("DROP TABLE " + spec.name).Error; err != nil {
			return err
		}
		if err := tx.Exec(createVectorTableSQL(spec)).Error; err != nil {
			return err
		}
		selectColumns := strings.Join(spec.keyColumns, ", ")
		signatureExpression := "''"
		if hasSignature {
			signatureExpression = "embedding_signature"
		}
		insertColumns := strings.Join(append(append([]string(nil), spec.keyColumns...), "embedding_signature", "embedding"), ", ")
		if err := tx.Exec(fmt.Sprintf(
			"INSERT INTO %s (%s) SELECT %s, %s, %s FROM %s",
			spec.name, insertColumns, selectColumns, signatureExpression, resizeExpression, backupTable,
		)).Error; err != nil {
			return err
		}
		return tx.Exec("DROP TABLE " + backupTable).Error
	})
}
