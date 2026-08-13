package conversation

import (
	"context"
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/sqlitevec"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
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
		{ConversationID: 20, MessageID: 30, UserID: 1, Role: "assistant", ChunkIndex: 0, Content: "message target", TokenCount: 2},
		{ConversationID: 20, MessageID: 31, UserID: 1, Role: "assistant", ChunkIndex: 0, Content: "message unrelated", TokenCount: 2},
	}
	rootMessageID := uint(29)
	activeMessageID := uint(30)
	branchMessages := []model.Message{
		{
			BaseModel: model.BaseModel{ID: rootMessageID}, ConversationID: 20, UserID: 1,
			PublicID: "msg_vector_root", Role: "user", Status: "success",
		},
		{
			BaseModel: model.BaseModel{ID: activeMessageID}, ConversationID: 20, UserID: 1,
			PublicID: "msg_vector_active", ParentMessageID: &rootMessageID, Role: "assistant", Status: "success",
		},
		{
			BaseModel: model.BaseModel{ID: 31}, ConversationID: 20, UserID: 1,
			PublicID: "msg_vector_sibling", ParentMessageID: &rootMessageID, Role: "assistant", Status: "success",
		},
		{
			BaseModel: model.BaseModel{ID: 32}, ConversationID: 20, UserID: 1,
			PublicID: "msg_vector_leaf", ParentMessageID: &activeMessageID, Role: "user", Status: "pending",
		},
	}
	if err := db.Create(&branchMessages).Error; err != nil {
		t.Fatalf("create message branch: %v", err)
	}
	messageEmbeddings := [][]float32{
		{0.8, 0.6, 0},
		{1, 0, 0},
	}
	if err := repo.UpsertMessageChunks(ctx, messageChunks, messageEmbeddings); err != nil {
		t.Fatalf("UpsertMessageChunks() error = %v", err)
	}
	messageResults, err := repo.SearchMessageChunks(ctx, repository.MessageChunkSearchInput{
		Scope: repository.HistoricalMessageScope{
			ConversationID: 20,
			UserID:         1,
			LeafMessageID:  32,
		},
		QueryEmbedding: []float32{1, 0, 0},
		TopK:           1,
	})
	if err != nil {
		t.Fatalf("SearchMessageChunks() error = %v", err)
	}
	if len(messageResults) == 0 || messageResults[0].Content != "message target" {
		t.Fatalf("expected nearest message chunk first, got %#v", messageResults)
	}

	coveredResults, err := repo.SearchMessageChunks(ctx, repository.MessageChunkSearchInput{
		Scope: repository.HistoricalMessageScope{
			ConversationID:          20,
			UserID:                  1,
			LeafMessageID:           32,
			ExcludeThroughMessageID: activeMessageID,
		},
		QueryEmbedding: []float32{1, 0, 0},
		TopK:           1,
	})
	if err != nil {
		t.Fatalf("SearchMessageChunks(snapshot scope) error = %v", err)
	}
	if len(coveredResults) != 0 {
		t.Fatalf("expected snapshot boundary to exclude covered chunk, got %#v", coveredResults)
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
	if err := db.AutoMigrate(&model.FileChunk{}, &model.MessageChunk{}, &model.Message{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	if err := sqlitevec.Migrate(db); err != nil {
		t.Fatalf("migrate sqlite vectors: %v", err)
	}
	return db
}
