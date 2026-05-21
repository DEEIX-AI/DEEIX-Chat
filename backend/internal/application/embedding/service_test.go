package embedding

import (
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
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
