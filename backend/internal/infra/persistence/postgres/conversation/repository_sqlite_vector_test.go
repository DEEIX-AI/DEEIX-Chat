package conversation

import (
	"context"
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/sqlitevec"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLiteVectorStoreSearchesFileAndMessageChunks(t *testing.T) {
	db := openConversationSQLiteVectorTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	available, err := repo.VectorStoreAvailable(ctx)
	if err != nil {
		t.Fatalf("VectorStoreAvailable() error = %v", err)
	}
	if !available {
		t.Fatal("expected sqlite vector store to be available")
	}

	fileChunks := []domainconversation.FileChunk{
		{FileObjID: 10, UserID: 1, ChunkIndex: 0, Content: "alpha search target", TokenCount: 3},
		{FileObjID: 10, UserID: 1, ChunkIndex: 1, Content: "beta unrelated", TokenCount: 2},
	}
	fileEmbeddings := [][]float32{
		{1, 0, 0},
		{0, 1, 0},
	}
	if err := repo.ReplaceFileChunks(ctx, 10, fileChunks, fileEmbeddings); err != nil {
		t.Fatalf("ReplaceFileChunks() error = %v", err)
	}
	fileResults, err := repo.SearchFileChunks(ctx, 1, []uint{10}, []float32{1, 0, 0}, 2)
	if err != nil {
		t.Fatalf("SearchFileChunks() error = %v", err)
	}
	if len(fileResults) == 0 || fileResults[0].Content != "alpha search target" {
		t.Fatalf("expected nearest file chunk first, got %#v", fileResults)
	}

	messageChunks := []domainconversation.MessageChunk{
		{ConversationID: 20, MessageID: 30, UserID: 1, Role: "user", ChunkIndex: 0, Content: "message target", TokenCount: 2},
		{ConversationID: 20, MessageID: 31, UserID: 1, Role: "assistant", ChunkIndex: 0, Content: "message unrelated", TokenCount: 2},
	}
	messageEmbeddings := [][]float32{
		{0, 0, 1},
		{0, 1, 0},
	}
	if err := repo.UpsertMessageChunks(ctx, messageChunks, messageEmbeddings); err != nil {
		t.Fatalf("UpsertMessageChunks() error = %v", err)
	}
	messageResults, err := repo.SearchMessageChunks(ctx, 20, 1, []float32{0, 0, 1}, 2, 0)
	if err != nil {
		t.Fatalf("SearchMessageChunks() error = %v", err)
	}
	if len(messageResults) == 0 || messageResults[0].Content != "message target" {
		t.Fatalf("expected nearest message chunk first, got %#v", messageResults)
	}
}

func openConversationSQLiteVectorTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	sqlitevec.Register()
	db, err := gorm.Open(sqlite.Open("file:conversation_vector?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&model.FileChunk{}, &model.MessageChunk{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	if err := sqlitevec.Migrate(db); err != nil {
		t.Fatalf("migrate sqlite vectors: %v", err)
	}
	return db
}
