package sqlitevec

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateExpandsLegacyVectorsWithoutLosingValues(t *testing.T) {
	Register()
	db, err := gorm.Open(sqlite.Open("file:sqlitevec_dimension_migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	const legacyDimensions = 1536
	if err = db.Exec(`CREATE VIRTUAL TABLE file_chunk_vectors USING vec0(
		chunk_id INTEGER PRIMARY KEY,
		user_id INTEGER PARTITION KEY,
		file_obj_id INTEGER,
		embedding FLOAT[1536] distance_metric=cosine
	)`).Error; err != nil {
		t.Fatalf("create legacy vector table: %v", err)
	}
	legacyVector := make([]float32, legacyDimensions)
	legacyVector[0], legacyVector[1], legacyVector[len(legacyVector)-1] = 1.25, -2.5, 7.75
	legacyBlob, err := sqlite_vec.SerializeFloat32(legacyVector)
	if err != nil {
		t.Fatalf("serialize legacy vector: %v", err)
	}
	if err = db.Exec(
		`INSERT INTO file_chunk_vectors (chunk_id, user_id, file_obj_id, embedding) VALUES (?, ?, ?, ?)`,
		1, 2, 3, legacyBlob,
	).Error; err != nil {
		t.Fatalf("insert legacy vector: %v", err)
	}

	if err = Migrate(db); err != nil {
		t.Fatalf("migrate vector tables: %v", err)
	}
	if err = Migrate(db); err != nil {
		t.Fatalf("repeat vector migration: %v", err)
	}
	dimensions, _, err := vectorTableSchema(db, FileChunkVectorTable)
	if err != nil || dimensions != EmbeddingDimensions {
		t.Fatalf("migrated dimensions = %d, err=%v; want %d", dimensions, err, EmbeddingDimensions)
	}
	available, err := Available(t.Context(), db)
	if err != nil || !available {
		t.Fatalf("vector store available = %v, err=%v", available, err)
	}

	var migrated struct {
		Embedding []byte
	}
	if err = db.Raw(`SELECT embedding FROM file_chunk_vectors WHERE chunk_id = 1`).Scan(&migrated).Error; err != nil {
		t.Fatalf("read migrated vector: %v", err)
	}
	migratedBlob := migrated.Embedding
	if len(migratedBlob) != EmbeddingDimensions*4 {
		t.Fatalf("migrated vector bytes = %d, want %d", len(migratedBlob), EmbeddingDimensions*4)
	}
	assertFloat32At(t, migratedBlob, 0, 1.25)
	assertFloat32At(t, migratedBlob, 1, -2.5)
	assertFloat32At(t, migratedBlob, legacyDimensions-1, 7.75)
	assertFloat32At(t, migratedBlob, legacyDimensions, 0)
	assertFloat32At(t, migratedBlob, EmbeddingDimensions-1, 0)
}

func TestSerializeFloat32RejectsOversizedVectors(t *testing.T) {
	if _, err := SerializeFloat32(make([]float32, EmbeddingDimensions+1)); err == nil {
		t.Fatal("expected oversized vector to be rejected")
	}
}

func TestMigrateRejectsOversizedLegacyVectorTable(t *testing.T) {
	Register()
	db, err := gorm.Open(sqlite.Open("file:sqlitevec_oversized_migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	if err = db.Exec(`CREATE VIRTUAL TABLE file_chunk_vectors USING vec0(
		chunk_id INTEGER PRIMARY KEY,
		user_id INTEGER PARTITION KEY,
		file_obj_id INTEGER,
		embedding FLOAT[4097] distance_metric=cosine
	)`).Error; err != nil {
		t.Fatalf("create oversized legacy vector table: %v", err)
	}

	err = Migrate(db)
	if err == nil || !strings.Contains(err.Error(), "exceed supported maximum") {
		t.Fatalf("Migrate() error = %v, want explicit oversized-dimension error", err)
	}
}

func assertFloat32At(t *testing.T, blob []byte, index int, expected float32) {
	t.Helper()
	offset := index * 4
	actual := math.Float32frombits(binary.LittleEndian.Uint32(blob[offset : offset+4]))
	if actual != expected {
		t.Fatalf("vector[%d] = %v, want %v", index, actual, expected)
	}
}
