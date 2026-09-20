package embedding

import (
	"context"
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	infraembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func emptyTestConfig() config.Config {
	return config.Config{
		EmbeddingEnabled: true,
		RAGModel:         "text-embedding-test",
		EmbeddingHost:    "http://127.0.0.1:8081",
	}
}

func TestProcessFileMarksEmptyWithoutReExtracting(t *testing.T) {
	repo := &reindexRepo{
		vectorAvailable: true,
		processing:      &domainconversation.FileObjectProcessing{ExtractStatus: domainconversation.FileSubprocessStatusEmpty},
	}
	// extractSvc 为 nil：如果代码走到重新提取，会返回 "extract service not configured" 并落 failed。
	service := newTestService(emptyTestConfig(), repo, nil, infraembedding.New(security.OutboundPolicy{}), nil)

	err := service.ProcessFile(context.Background(), domainconversation.FileObject{
		ID: 1, UserID: 1, FileID: "file_empty", FileName: "scan.pdf", MimeType: "application/pdf",
		FileCategory: "pdf", StoragePath: "uploads/scan.pdf", Status: "active",
	})

	if err != nil {
		t.Fatalf("empty file must not surface as an error, got %v", err)
	}
	last := repo.statusHistory[len(repo.statusHistory)-1]
	if last != domainconversation.FileSubprocessStatusEmpty {
		t.Fatalf("expected terminal status %q, got history %v", domainconversation.FileSubprocessStatusEmpty, repo.statusHistory)
	}
}

func TestProcessFileStillFailsWhenExtractionErrors(t *testing.T) {
	repo := &reindexRepo{vectorAvailable: true}
	service := newTestService(emptyTestConfig(), repo, nil, infraembedding.New(security.OutboundPolicy{}), nil)

	err := service.ProcessFile(context.Background(), domainconversation.FileObject{
		ID: 1, UserID: 1, FileID: "file_broken", FileName: "doc.txt", MimeType: "text/plain",
		StoragePath: "uploads/doc.txt", Status: "active",
	})

	if err == nil {
		t.Fatal("extraction error must still be reported")
	}
	last := repo.statusHistory[len(repo.statusHistory)-1]
	if last != "failed" {
		t.Fatalf("genuine extraction error must stay failed, got %v", repo.statusHistory)
	}
}

func TestReindexStaleFilesPassesIncludeEmptyToRepository(t *testing.T) {
	repo := &reindexRepo{vectorAvailable: true}
	service := newTestService(emptyTestConfig(), repo, nil, infraembedding.New(security.OutboundPolicy{}), nil)

	if _, err := service.ReindexStaleFiles(context.Background(), true); err != nil {
		t.Fatalf("ReindexStaleFiles() error = %v", err)
	}
	if len(repo.listIncludeEmpty) == 0 || !repo.listIncludeEmpty[0] {
		t.Fatalf("expected includeEmpty=true forwarded to repository, got %v", repo.listIncludeEmpty)
	}
}

func TestGetIndexStatusReportsEmptyCount(t *testing.T) {
	repo := &countingRepo{counts: map[string]int64{"ready": 3, "empty": 2, "failed": 1}}
	service := newTestService(emptyTestConfig(), repo, nil, nil, nil)

	status, err := service.GetIndexStatus(context.Background())
	if err != nil {
		t.Fatalf("GetIndexStatus() error = %v", err)
	}
	if status.EmptyCount != 2 || status.FailedCount != 1 || status.ReadyCount != 3 {
		t.Fatalf("unexpected counts: %+v", status)
	}
}

type countingRepo struct {
	reindexRepo
	counts map[string]int64
}

func (r *countingRepo) CountFilesByEmbedStatus(_ context.Context, status string) (int64, error) {
	return r.counts[status], nil
}
