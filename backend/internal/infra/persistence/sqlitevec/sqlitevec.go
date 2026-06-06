package sqlitevec

import (
	"context"
	"fmt"
	"sync"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"gorm.io/gorm"
)

const (
	EmbeddingDimensions = 1536

	FileChunkVectorTable    = "file_chunk_vectors"
	MessageChunkVectorTable = "chat_message_chunk_vectors"
	UserMemoryVectorTable   = "user_memory_vectors"
)

var registerOnce sync.Once

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
	statements := []string{
		fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS %s USING vec0(
			chunk_id INTEGER PRIMARY KEY,
			user_id INTEGER PARTITION KEY,
			file_obj_id INTEGER,
			embedding FLOAT[%d] distance_metric=cosine
		)`, FileChunkVectorTable, EmbeddingDimensions),
		fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS %s USING vec0(
			chunk_id INTEGER PRIMARY KEY,
			user_id INTEGER PARTITION KEY,
			conversation_id INTEGER,
			message_id INTEGER,
			embedding FLOAT[%d] distance_metric=cosine
		)`, MessageChunkVectorTable, EmbeddingDimensions),
		fmt.Sprintf(`CREATE VIRTUAL TABLE IF NOT EXISTS %s USING vec0(
			memory_id INTEGER PRIMARY KEY,
			user_id INTEGER PARTITION KEY,
			embedding FLOAT[%d] distance_metric=cosine
		)`, UserMemoryVectorTable, EmbeddingDimensions),
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
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
	for _, table := range []string{FileChunkVectorTable, MessageChunkVectorTable, UserMemoryVectorTable} {
		if !db.WithContext(ctx).Migrator().HasTable(table) {
			return false, nil
		}
	}
	return true, nil
}

// SerializeFloat32 returns the vector BLOB format accepted by sqlite-vec.
func SerializeFloat32(vector []float32) ([]byte, error) {
	aligned := alignDimensions(vector, EmbeddingDimensions)
	return sqlite_vec.SerializeFloat32(aligned)
}

func alignDimensions(vector []float32, dimensions int) []float32 {
	if dimensions <= 0 || len(vector) == dimensions {
		return vector
	}
	if len(vector) > dimensions {
		return append([]float32(nil), vector[:dimensions]...)
	}
	aligned := make([]float32, dimensions)
	copy(aligned, vector)
	return aligned
}
