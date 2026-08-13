package conversation

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSearchMessageChunksFiltersPostgresBranchBeforeTopK(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("DEEIX_TEST_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("set DEEIX_TEST_DATABASE_DSN to run PostgreSQL branch-scoped vector integration test")
	}

	db, cleanup := openConversationPostgresIntegrationDB(t, dsn)
	t.Cleanup(cleanup)
	if err := db.AutoMigrate(&model.Message{}, &model.MessageChunk{}); err != nil {
		t.Fatalf("migrate conversation vector models: %v", err)
	}
	if err := db.Exec(`ALTER TABLE chat_message_chunks ADD COLUMN IF NOT EXISTS embedding vector(1536)`).Error; err != nil {
		t.Fatalf("add message embedding column: %v", err)
	}

	root := model.Message{ConversationID: 20, UserID: 1, PublicID: "msg_pg_vector_root", Role: "user", Status: "success"}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create root message: %v", err)
	}
	active := model.Message{
		ConversationID: 20, UserID: 1, PublicID: "msg_pg_vector_active", ParentMessageID: &root.ID,
		Role: "assistant", Status: "success",
	}
	if err := db.Create(&active).Error; err != nil {
		t.Fatalf("create active message: %v", err)
	}
	sibling := model.Message{
		ConversationID: 20, UserID: 1, PublicID: "msg_pg_vector_sibling", ParentMessageID: &root.ID,
		Role: "assistant", BranchReason: "retry", Status: "success",
	}
	if err := db.Create(&sibling).Error; err != nil {
		t.Fatalf("create sibling message: %v", err)
	}
	leaf := model.Message{
		ConversationID: 20, UserID: 1, PublicID: "msg_pg_vector_leaf", ParentMessageID: &active.ID,
		Role: "user", Status: "pending",
	}
	if err := db.Create(&leaf).Error; err != nil {
		t.Fatalf("create leaf message: %v", err)
	}

	chunks := []model.MessageChunk{
		{ConversationID: 20, MessageID: active.ID, UserID: 1, Role: "assistant", Content: "active branch target"},
		{ConversationID: 20, MessageID: sibling.ID, UserID: 1, Role: "assistant", Content: "closer sibling target"},
	}
	if err := db.Create(&chunks).Error; err != nil {
		t.Fatalf("create message chunks: %v", err)
	}
	queryEmbedding := make([]float32, 1536)
	queryEmbedding[0] = 1
	activeEmbedding := make([]float32, 1536)
	activeEmbedding[0], activeEmbedding[1] = 0.8, 0.6
	if err := db.Exec(`UPDATE chat_message_chunks SET embedding = ?::vector WHERE id = ?`, float32SliceToPostgresVector(activeEmbedding), chunks[0].ID).Error; err != nil {
		t.Fatalf("write active embedding: %v", err)
	}
	if err := db.Exec(`UPDATE chat_message_chunks SET embedding = ?::vector WHERE id = ?`, float32SliceToPostgresVector(queryEmbedding), chunks[1].ID).Error; err != nil {
		t.Fatalf("write sibling embedding: %v", err)
	}
	if err := db.Exec(`CREATE INDEX message_chunks_embedding_test_idx ON chat_message_chunks USING ivfflat (embedding vector_cosine_ops) WITH (lists = 1)`).Error; err != nil {
		t.Fatalf("create message embedding index: %v", err)
	}

	results, err := NewRepo(db).SearchMessageChunks(context.Background(), repository.MessageChunkSearchInput{
		Scope: repository.HistoricalMessageScope{
			ConversationID: 20,
			UserID:         1,
			LeafMessageID:  leaf.ID,
		},
		QueryEmbedding: queryEmbedding,
		TopK:           1,
	})
	if err != nil {
		t.Fatalf("SearchMessageChunks() error = %v", err)
	}
	if len(results) != 1 || results[0].MessageID != active.ID {
		t.Fatalf("expected active branch result despite closer sibling, got %#v", results)
	}
}

func openConversationPostgresIntegrationDB(t *testing.T, dsn string) (*gorm.DB, func()) {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve postgres db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	var vectorAvailable bool
	if err := db.Raw(`SELECT to_regtype('vector') IS NOT NULL`).Scan(&vectorAvailable).Error; err != nil {
		_ = sqlDB.Close()
		t.Fatalf("check pgvector extension: %v", err)
	}
	if !vectorAvailable {
		_ = sqlDB.Close()
		t.Skip("pgvector extension is required for PostgreSQL branch-scoped vector integration test")
	}
	schemaName := fmt.Sprintf("deeix_test_conversation_scope_%d", time.Now().UnixNano())
	if err := db.Exec(`CREATE SCHEMA ` + schemaName).Error; err != nil {
		_ = sqlDB.Close()
		t.Fatalf("create test schema: %v", err)
	}
	cleanup := func() {
		_ = db.Exec(`DROP SCHEMA IF EXISTS ` + schemaName + ` CASCADE`).Error
		_ = sqlDB.Close()
	}
	if err := db.Exec(`SET search_path TO ` + schemaName + `, public`).Error; err != nil {
		cleanup()
		t.Fatalf("set test search path: %v", err)
	}
	return db, cleanup
}
