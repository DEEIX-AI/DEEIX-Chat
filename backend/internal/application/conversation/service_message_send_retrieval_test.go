package conversation

import (
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

func TestRetrievedImageEvidenceMapsImageChunksToFilesOnce(t *testing.T) {
	files := []model.FileObject{
		{ID: 1, FileID: "img", FileName: "a.png", MimeType: "image/png", FileCategory: "image", StoragePath: "u/a.png"},
		{ID: 2, FileID: "doc", FileName: "b.pdf", StoragePath: "u/b.pdf"},
		{ID: 3, FileID: "sent", FileName: "c.png", MimeType: "image/png", FileCategory: "image", StoragePath: "u/c.png"},
	}
	chunks := []model.RAGChunk{
		{FileID: "doc", ChunkIndex: 0, Modality: model.FileChunkModalityText},
		{FileID: "img", ChunkIndex: 0, Modality: model.FileChunkModalityImage},
		{FileID: "img", ChunkIndex: 0, Modality: model.FileChunkModalityImage},
		{FileID: "sent", ChunkIndex: 0, Modality: model.FileChunkModalityImage},
		{FileID: "missing", ChunkIndex: 0, Modality: model.FileChunkModalityImage},
	}
	evidence := retrievedImageEvidence(chunks, files, []AttachmentInput{{FileID: "sent"}})
	if len(evidence) != 1 {
		t.Fatalf("expected one image evidence, got %#v", evidence)
	}
	got := evidence[0]
	if got.FileID != "img" || got.Kind != "image" || got.StoragePath != "u/a.png" || !got.Current || got.ContextMode != fileContextModeRAG {
		t.Fatalf("unexpected evidence %#v", got)
	}
}
