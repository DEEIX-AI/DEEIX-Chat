package embedding

import (
	"context"
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	infraembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/embedding"
)

func TestShouldTriggerIncludesOCRImages(t *testing.T) {
	service := NewService(config.Config{
		RAGEnabled:             true,
		EmbeddingEnabled:       true,
		EmbedTriggerOnUpload:   true,
		RAGModel:               "text-embedding-test",
		EmbeddingHost:          "http://127.0.0.1:8081",
		ExtractImageOCREnabled: true,
	}, nil, nil, nil, nil)

	fileObj := domainconversation.FileObject{
		FileID:       "file_1",
		FileName:     "photo.png",
		MimeType:     "image/png",
		FileCategory: "image",
		StoragePath:  "uploads/photo.png",
		Status:       "active",
	}
	if !service.ShouldTrigger(fileObj) {
		t.Fatal("expected OCR image to trigger embedding")
	}
}

func TestShouldTriggerSkipsImagesWhenOCRDisabled(t *testing.T) {
	service := NewService(config.Config{
		RAGEnabled:             true,
		EmbeddingEnabled:       true,
		EmbedTriggerOnUpload:   true,
		RAGModel:               "text-embedding-test",
		EmbeddingHost:          "http://127.0.0.1:8081",
		ExtractImageOCREnabled: false,
	}, nil, nil, nil, nil)

	fileObj := domainconversation.FileObject{
		FileID:       "file_1",
		FileName:     "photo.png",
		MimeType:     "image/png",
		FileCategory: "image",
		StoragePath:  "uploads/photo.png",
		Status:       "active",
	}
	if service.ShouldTrigger(fileObj) {
		t.Fatal("expected image embedding to stay disabled when OCR is disabled")
	}
}

func TestShouldTriggerDoesNotRequireRAGEnabled(t *testing.T) {
	service := NewService(config.Config{
		RAGEnabled:           false,
		EmbeddingEnabled:     true,
		EmbedTriggerOnUpload: true,
		RAGModel:             "text-embedding-test",
		EmbeddingHost:        "http://127.0.0.1:8081",
	}, nil, nil, nil, nil)

	fileObj := domainconversation.FileObject{
		FileID:      "file_1",
		FileName:    "doc.txt",
		MimeType:    "text/plain",
		StoragePath: "uploads/doc.txt",
		Status:      "active",
	}
	if !service.ShouldTrigger(fileObj) {
		t.Fatal("expected embedding trigger to ignore chat RAG switch")
	}
}

func TestIndexingAvailableDoesNotRequireRAGEnabled(t *testing.T) {
	repo := &reindexRepo{vectorAvailable: true}
	service := NewService(config.Config{
		RAGEnabled:       false,
		EmbeddingEnabled: true,
		RAGModel:         "text-embedding-test",
		EmbeddingHost:    "http://127.0.0.1:8081",
	}, repo, nil, infraembedding.New(), nil)

	available, reason := service.IndexingAvailable(context.Background())
	if !available {
		t.Fatalf("expected indexing to ignore chat RAG switch, got %s", reason)
	}
}

func TestReindexStaleFilesDoesNotRequireRAGEnabled(t *testing.T) {
	repo := &reindexRepo{vectorAvailable: true}
	service := NewService(config.Config{
		RAGEnabled:       false,
		EmbeddingEnabled: true,
		RAGModel:         "text-embedding-test",
		EmbeddingHost:    "http://127.0.0.1:8081",
	}, repo, nil, infraembedding.New(), nil)

	submitted, err := service.ReindexStaleFiles(context.Background())
	if err != nil {
		t.Fatalf("expected reindex to ignore chat RAG switch, got %v", err)
	}
	if submitted != 0 {
		t.Fatalf("expected no submitted files, got %d", submitted)
	}
	if repo.listCalls != 1 {
		t.Fatalf("expected reindex list query to run once, got %d", repo.listCalls)
	}
}

func TestProcessFileDoesNotRequireRAGEnabled(t *testing.T) {
	repo := &reindexRepo{vectorAvailable: true}
	service := NewService(config.Config{
		RAGEnabled:       false,
		EmbeddingEnabled: true,
		RAGModel:         "text-embedding-test",
		EmbeddingHost:    "http://127.0.0.1:8081",
	}, repo, nil, infraembedding.New(), nil)

	_ = service.ProcessFile(context.Background(), domainconversation.FileObject{
		ID:          1,
		UserID:      1,
		FileID:      "file_1",
		FileName:    "doc.txt",
		MimeType:    "text/plain",
		StoragePath: "uploads/doc.txt",
		Status:      "active",
	})

	if repo.updateStatusCalls == 0 {
		t.Fatal("expected ProcessFile to start embedding even when chat RAG is disabled")
	}
}

type reindexRepo struct {
	vectorAvailable   bool
	listCalls         int
	updateStatusCalls int
}

func (r *reindexRepo) VectorStoreAvailable(context.Context) (bool, error) {
	return r.vectorAvailable, nil
}

func (r *reindexRepo) GetActiveFileObjectByID(context.Context, uint, string) (*domainconversation.FileObject, error) {
	return nil, nil
}

func (r *reindexRepo) GetFileObjectProcessingByObjectID(context.Context, uint) (*domainconversation.FileObjectProcessing, error) {
	return nil, nil
}

func (r *reindexRepo) UpdateFileObjectEmbedStatus(context.Context, uint, string, string, string) error {
	r.updateStatusCalls++
	return nil
}

func (r *reindexRepo) UpdateFileObjectChunkCount(context.Context, uint, int) error {
	return nil
}

func (r *reindexRepo) ReplaceFileChunks(context.Context, uint, []domainconversation.FileChunk, [][]float32) error {
	return nil
}

func (r *reindexRepo) MarkAllEmbeddedFilesStale(context.Context) (int64, error) {
	return 0, nil
}

func (r *reindexRepo) CountFilesByEmbedStatus(context.Context, string) (int64, error) {
	return 0, nil
}

func (r *reindexRepo) ListFilesForReindex(context.Context, int, int) ([]domainconversation.FileObject, error) {
	r.listCalls++
	return nil, nil
}
