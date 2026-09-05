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
	const embeddingSignature = "test-model@3"
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
	files := []model.FileObject{
		{BaseModel: model.BaseModel{ID: 10}, FileID: "file_10", UserID: 1, Status: "active", EmbedStatus: "processing", EmbedSignature: embeddingSignature},
		{BaseModel: model.BaseModel{ID: 11}, FileID: "file_11", UserID: 2, Status: "active", EmbedStatus: "processing", EmbedSignature: embeddingSignature},
		{BaseModel: model.BaseModel{ID: 12}, FileID: "file_12", UserID: 2, Status: "active", EmbedStatus: "processing", EmbedSignature: embeddingSignature},
	}
	if err := db.Create(&files).Error; err != nil {
		t.Fatalf("create vector test files: %v", err)
	}

	fileChunks := []domainconversation.FileChunk{
		{FileObjID: 10, UserID: 1, ChunkIndex: 0, Content: "alpha search target", TokenCount: 3, EmbeddingSignature: embeddingSignature},
		{FileObjID: 10, UserID: 1, ChunkIndex: 1, Content: "beta unrelated", TokenCount: 2, EmbeddingSignature: embeddingSignature},
	}
	fileEmbeddings := [][]float32{
		{1, 0, 0},
		{0, 1, 0},
	}
	if published, err := repo.ReplaceFileChunks(ctx, 10, embeddingSignature, fileChunks, fileEmbeddings); err != nil || !published {
		t.Fatalf("ReplaceFileChunks() error = %v", err)
	}
	fileResults, err := repo.SearchFileChunks(ctx, 1, []uint{10}, []float32{1, 0, 0}, embeddingSignature, 2)
	if err != nil {
		t.Fatalf("SearchFileChunks() error = %v", err)
	}
	if len(fileResults) == 0 || fileResults[0].Content != "alpha search target" {
		t.Fatalf("expected nearest file chunk first, got %#v", fileResults)
	}
	otherOwnerChunks := []domainconversation.FileChunk{
		{FileObjID: 11, UserID: 2, ChunkIndex: 0, Content: "shared knowledge target", TokenCount: 3, EmbeddingSignature: embeddingSignature},
	}
	if published, err := repo.ReplaceFileChunks(ctx, 11, embeddingSignature, otherOwnerChunks, [][]float32{{0.9, 0.1, 0}}); err != nil || !published {
		t.Fatalf("ReplaceFileChunks(other owner) error = %v", err)
	}
	builtinBase := model.KnowledgeBase{PublicID: "builtin", Scope: "builtin", Name: "Built in", Enabled: true}
	if err := db.Create(&builtinBase).Error; err != nil {
		t.Fatalf("create built-in knowledge base: %v", err)
	}
	if err := db.Create(&model.KnowledgeBaseFile{KnowledgeBaseID: builtinBase.ID, FileObjectID: 11}).Error; err != nil {
		t.Fatalf("link built-in knowledge base file: %v", err)
	}
	privateChunks := []domainconversation.FileChunk{
		{FileObjID: 12, UserID: 2, ChunkIndex: 0, Content: "private target", TokenCount: 2, EmbeddingSignature: embeddingSignature},
	}
	if published, err := repo.ReplaceFileChunks(ctx, 12, embeddingSignature, privateChunks, [][]float32{{1, 0, 0}}); err != nil || !published {
		t.Fatalf("ReplaceFileChunks(private other owner) error = %v", err)
	}
	sharedResults, err := repo.SearchFileChunks(ctx, 1, []uint{10, 11, 12, 10}, []float32{1, 0, 0}, embeddingSignature, 3)
	if err != nil {
		t.Fatalf("SearchFileChunks(multiple owners) error = %v", err)
	}
	foundFileIDs := make(map[uint]bool, len(sharedResults))
	for _, result := range sharedResults {
		foundFileIDs[result.FileObjID] = true
	}
	if !foundFileIDs[10] || !foundFileIDs[11] {
		t.Fatalf("expected exact authorized file IDs across owners, got %#v", sharedResults)
	}
	if foundFileIDs[12] {
		t.Fatalf("expected unshared file from another owner to stay hidden, got %#v", sharedResults)
	}
	keywordResults, err := repo.BM25SearchFileChunks(ctx, 1, []uint{10, 11, 12}, "target", 3)
	if err != nil {
		t.Fatalf("BM25SearchFileChunks() error = %v", err)
	}
	foundFileIDs = make(map[uint]bool, len(keywordResults))
	for _, result := range keywordResults {
		foundFileIDs[result.FileObjID] = true
	}
	if !foundFileIDs[10] || !foundFileIDs[11] || foundFileIDs[12] {
		t.Fatalf("expected keyword search to preserve owner and built-in ACLs, got %#v", keywordResults)
	}
	if err := db.Model(&builtinBase).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable built-in knowledge base: %v", err)
	}
	disabledResults, err := repo.SearchFileChunks(ctx, 1, []uint{11}, []float32{1, 0, 0}, embeddingSignature, 1)
	if err != nil || len(disabledResults) != 0 {
		t.Fatalf("expected disabled built-in knowledge to stay hidden, got %#v, err=%v", disabledResults, err)
	}
	isolatedFileResults, err := repo.SearchFileChunks(ctx, 1, []uint{10}, []float32{1, 0, 0}, "other-model@3", 2)
	if err != nil || len(isolatedFileResults) != 0 {
		t.Fatalf("expected file vectors from another signature to stay hidden, got %#v, err=%v", isolatedFileResults, err)
	}

	messageChunks := []domainconversation.MessageChunk{
		{ConversationID: 20, MessageID: 30, UserID: 1, Role: "assistant", ChunkIndex: 0, Content: "message target", TokenCount: 2, EmbeddingSignature: embeddingSignature},
		{ConversationID: 20, MessageID: 31, UserID: 1, Role: "assistant", ChunkIndex: 0, Content: "message unrelated", TokenCount: 2, EmbeddingSignature: embeddingSignature},
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
		QueryEmbedding:     []float32{1, 0, 0},
		EmbeddingSignature: embeddingSignature,
		TopK:               1,
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
		QueryEmbedding:     []float32{1, 0, 0},
		EmbeddingSignature: embeddingSignature,
		TopK:               1,
	})
	if err != nil {
		t.Fatalf("SearchMessageChunks(snapshot scope) error = %v", err)
	}
	if len(coveredResults) != 0 {
		t.Fatalf("expected snapshot boundary to exclude covered chunk, got %#v", coveredResults)
	}
}

func TestMarkEmbeddedFilesStaleKeepsCurrentSignatureReady(t *testing.T) {
	db := openConversationSQLiteVectorTestDB(t)
	repo := NewRepo(db)
	files := []model.FileObject{
		{BaseModel: model.BaseModel{ID: 101}, FileID: "old-ready", UserID: 1, FileName: "old.txt", Status: "active", EmbedStatus: "ready"},
		{BaseModel: model.BaseModel{ID: 102}, FileID: "current-ready", UserID: 1, FileName: "current.txt", Status: "active", EmbedStatus: "ready"},
		{BaseModel: model.BaseModel{ID: 103}, FileID: "old-processing", UserID: 1, FileName: "processing.txt", Status: "active", EmbedStatus: "processing"},
	}
	if err := db.Create(&files).Error; err != nil {
		t.Fatalf("create files: %v", err)
	}
	chunks := []model.FileChunk{
		{FileObjID: files[0].ID, UserID: 1, ChunkIndex: 0, Content: "old", EmbeddingSignature: "old-space"},
		{FileObjID: files[1].ID, UserID: 1, ChunkIndex: 0, Content: "current", EmbeddingSignature: "current-space"},
		{FileObjID: files[2].ID, UserID: 1, ChunkIndex: 0, Content: "old processing", EmbeddingSignature: "old-space"},
	}
	if err := db.Create(&chunks).Error; err != nil {
		t.Fatalf("create chunks: %v", err)
	}

	updated, err := repo.MarkEmbeddedFilesStale(context.Background(), "current-space")
	if err != nil {
		t.Fatalf("MarkEmbeddedFilesStale() error = %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}
	statuses := make(map[string]string, len(files))
	var got []model.FileObject
	if err = db.Order("id ASC").Find(&got).Error; err != nil {
		t.Fatalf("load files: %v", err)
	}
	for _, file := range got {
		statuses[file.FileID] = file.EmbedStatus
	}
	if statuses["old-ready"] != "stale" || statuses["old-processing"] != "stale" || statuses["current-ready"] != "ready" {
		t.Fatalf("unexpected statuses: %#v", statuses)
	}
}

func TestFileEmbeddingQueueStateAndProcessingProjection(t *testing.T) {
	db := openConversationSQLiteVectorTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	file := model.FileObject{
		FileID:          "file_queue_state",
		UserID:          7,
		FileName:        "manual.md",
		MimeType:        "text/markdown",
		FileCategory:    "text",
		StoragePath:     "uploads/manual.md",
		Status:          "active",
		ProcessingReady: true,
		ExtractStatus:   "ready",
		EmbedStatus:     "none",
	}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}

	queued, err := repo.QueueFileEmbedding(ctx, 7, file.FileID, "model@1536")
	if err != nil || !queued {
		t.Fatalf("queue embedding: queued=%v err=%v", queued, err)
	}
	if duplicate, duplicateErr := repo.QueueFileEmbedding(ctx, 7, file.FileID, "model@1536"); duplicateErr != nil || duplicate {
		t.Fatalf("duplicate queue must be rejected: queued=%v err=%v", duplicate, duplicateErr)
	}

	statuses, err := repo.GetActiveFileProcessingStatusesByIDs(ctx, 7, []string{file.FileID})
	if err != nil || len(statuses) != 1 {
		t.Fatalf("load processing projection: statuses=%#v err=%v", statuses, err)
	}
	status := statuses[0]
	if status.FileName != file.FileName || status.MimeType != file.MimeType ||
		status.FileCategory != file.FileCategory || status.StoragePath != file.StoragePath ||
		status.Status != "active" || !status.ProcessingReady || status.ExtractStatus != "ready" ||
		status.EmbedStatus != "queued" || status.EmbedSignature != "model@1536" {
		t.Fatalf("incomplete processing projection: %#v", status)
	}

	claimed, err := repo.ClaimFileEmbedding(ctx, 7, file.FileID, "model@1536")
	if err != nil || !claimed {
		t.Fatalf("claim queued embedding: claimed=%v err=%v", claimed, err)
	}
	var stored model.FileObject
	if err = db.Where("id = ?", file.ID).Take(&stored).Error; err != nil {
		t.Fatalf("load claimed file: %v", err)
	}
	if stored.EmbedStatus != "processing" || stored.EmbedSignature != "model@1536" {
		t.Fatalf("unexpected claimed state: %#v", stored)
	}
}

func TestFileEmbeddingGenerationRejectsSupersededPublisher(t *testing.T) {
	db := openConversationSQLiteVectorTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	file := model.FileObject{
		FileID:         "file_generation",
		UserID:         1,
		Status:         "active",
		EmbedStatus:    "processing",
		EmbedSignature: "space-4096",
	}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}

	claimed, err := repo.ClaimFileEmbedding(ctx, 1, file.FileID, "space-1536")
	if err != nil || !claimed {
		t.Fatalf("claim new vector space: claimed=%v err=%v", claimed, err)
	}
	oldChunks := []domainconversation.FileChunk{{FileObjID: file.ID, UserID: 1, Content: "old", EmbeddingSignature: "space-4096"}}
	if published, publishErr := repo.ReplaceFileChunks(ctx, file.ID, "space-4096", oldChunks, [][]float32{{1, 0}}); publishErr != nil || published {
		t.Fatalf("superseded publisher must be rejected: published=%v err=%v", published, publishErr)
	}
	if updated, updateErr := repo.UpdateFileObjectEmbedStatus(ctx, 1, file.FileID, "space-4096", "ready", ""); updateErr != nil || updated {
		t.Fatalf("superseded status update must be rejected: updated=%v err=%v", updated, updateErr)
	}

	newChunks := []domainconversation.FileChunk{{FileObjID: file.ID, UserID: 1, Content: "new", EmbeddingSignature: "space-1536"}}
	if published, publishErr := repo.ReplaceFileChunks(ctx, file.ID, "space-1536", newChunks, [][]float32{{0, 1}}); publishErr != nil || !published {
		t.Fatalf("current publisher must succeed: published=%v err=%v", published, publishErr)
	}
	if updated, updateErr := repo.UpdateFileObjectEmbedStatus(ctx, 1, file.FileID, "space-1536", "ready", ""); updateErr != nil || !updated {
		t.Fatalf("current status update must succeed: updated=%v err=%v", updated, updateErr)
	}

	var stored model.FileChunk
	if err := db.Where("file_obj_id = ?", file.ID).Take(&stored).Error; err != nil {
		t.Fatalf("load current chunk: %v", err)
	}
	if stored.Content != "new" || stored.EmbeddingSignature != "space-1536" {
		t.Fatalf("unexpected stored chunk: %#v", stored)
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
	if err := db.AutoMigrate(
		&model.FileObject{},
		&model.FileChunk{},
		&model.MessageChunk{},
		&model.Message{},
		&model.KnowledgeBase{},
		&model.KnowledgeBaseFile{},
	); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	if err := sqlitevec.Migrate(db); err != nil {
		t.Fatalf("migrate sqlite vectors: %v", err)
	}
	return db
}
