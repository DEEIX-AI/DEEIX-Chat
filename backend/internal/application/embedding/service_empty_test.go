package embedding

import (
	"context"
	"testing"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	infraembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
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

func TestPlanReprocessingTargetsEmbeddableFilesOnly(t *testing.T) {
	repo := &reindexRepo{vectorAvailable: true, files: []domainconversation.FileObject{
		{ID: 1, UserID: 2, FileID: "scan", FileName: "scan.pdf", MimeType: "application/pdf", FileCategory: "pdf", StoragePath: "uploads/scan.pdf", Status: "active", ProcessingStatus: "ready", ExtractStatus: "empty"},
		{ID: 2, UserID: 3, FileID: "clip", FileName: "clip.mp4", MimeType: "video/mp4", FileCategory: "video", StoragePath: "uploads/clip.mp4", Status: "active", ProcessingStatus: "failed"},
	}}
	service := newTestService(emptyTestConfig(), repo, nil, infraembedding.New(security.OutboundPolicy{}), nil)

	jobs, nextAfterID, err := service.PlanReprocessing(context.Background(), 0, 100, true)
	if err != nil {
		t.Fatalf("PlanReprocessing() error = %v", err)
	}
	if !repo.reprocessInput.IncludeEmpty || repo.reprocessInput.StalledBefore.IsZero() {
		t.Fatalf("include_empty and the stall cutoff must reach the repository: %+v", repo.reprocessInput)
	}
	// 当前配置下无法向量化的文件（视频）重新提取也建不了索引，不能为它们花提取成本。
	if len(jobs) != 1 || jobs[0].FileID != "scan" || jobs[0].UserID != 2 || !jobs[0].Reprocess {
		t.Fatalf("unexpected reprocessing jobs: %+v", jobs)
	}
	if nextAfterID != 2 {
		t.Fatalf("cursor must advance past every scanned file, got %d", nextAfterID)
	}
}

func TestOnlyAdminPlansReprocessFilesWithoutText(t *testing.T) {
	empty := domainconversation.FileObject{
		ID: 1, UserID: 2, FileID: "scan", FileName: "scan.pdf", MimeType: "application/pdf", FileCategory: "pdf",
		StoragePath: "uploads/scan.pdf", Status: "active", ProcessingStatus: "ready", ProcessingReady: false,
		ExtractStatus: "empty", EmbedStatus: "empty",
	}
	repo := &reindexRepo{vectorAvailable: true, files: []domainconversation.FileObject{empty}}
	service := newTestService(emptyTestConfig(), repo, nil, infraembedding.New(security.OutboundPolicy{}), nil)

	userPlan, err := service.PlanFiles(context.Background(), 2, []string{"scan"})
	if err != nil {
		t.Fatalf("PlanFiles() error = %v", err)
	}
	// 普通用户的“向量化”不触发重新提取，避免可能的付费 OCR 调用。
	if len(userPlan.Jobs) != 0 || len(userPlan.Skipped) != 1 || userPlan.Skipped[0].Reason != SkipReasonNotReady {
		t.Fatalf("user plan must skip files without text: %+v", userPlan)
	}
	adminPlan, err := service.PlanAdminFiles(context.Background(), []string{"scan"})
	if err != nil {
		t.Fatalf("PlanAdminFiles() error = %v", err)
	}
	if len(adminPlan.Jobs) != 1 || !adminPlan.Jobs[0].Reprocess {
		t.Fatalf("admin plan must reprocess files without text: %+v", adminPlan)
	}
}

func TestEmbedAfterReprocessingMarksChangedVectorSpaceStale(t *testing.T) {
	repo := &reindexRepo{vectorAvailable: true}
	service := newTestService(emptyTestConfig(), repo, nil, infraembedding.New(security.OutboundPolicy{}), nil)

	err := service.EmbedAfterReprocessing(context.Background(), domainconversation.FileObject{
		ID: 1, UserID: 2, FileID: "scan", EmbedStatus: "queued", EmbedSignature: "previous-space@1024",
	})
	if err != nil {
		t.Fatalf("EmbedAfterReprocessing() error = %v", err)
	}
	if len(repo.statusHistory) != 1 || repo.statusHistory[0] != "stale" {
		t.Fatalf("a request for an older vector space must become stale, got %v", repo.statusHistory)
	}
}

func TestGetIndexStatusSplitsPendingFromUnsupportedAndStalled(t *testing.T) {
	repo := &reindexRepo{stateCounts: []repository.FileEmbeddingStateCount{
		{Status: "ready", Embeddable: true, Count: 3},
		{Status: "empty", Embeddable: true, Count: 2},
		{Status: "failed", Embeddable: true, Count: 1},
		{Status: "none", Embeddable: true, Count: 4},
		{Status: "none", Embeddable: false, Count: 28},
		{Status: "queued", Embeddable: true, Count: 2},
		{Status: "queued", Embeddable: true, Stalled: true, Count: 1},
		{Status: "processing", Embeddable: true, Count: 1},
	}}
	service := newTestService(emptyTestConfig(), repo, nil, nil, nil)

	status, err := service.GetIndexStatus(context.Background())
	if err != nil {
		t.Fatalf("GetIndexStatus() error = %v", err)
	}
	if status.ReadyCount != 3 || status.EmptyCount != 2 || status.FailedCount != 1 {
		t.Fatalf("unexpected terminal counts: %+v", status)
	}
	// 不支持的 none 文件永远不会被处理，不能计入待处理。
	if status.PendingCount != 8 || status.StalledCount != 1 || status.ActiveCount != 3 || status.UnsupportedCount != 28 {
		t.Fatalf("unexpected pending split: %+v", status)
	}
}
