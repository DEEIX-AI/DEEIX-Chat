package embedding

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	infraembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func TestListFileTasksRejectsUnknownBucket(t *testing.T) {
	service := newTestService(emptyTestConfig(), &reindexRepo{}, nil, nil, nil)

	if _, err := service.ListFileTasks(context.Background(), ListFileTasksInput{Bucket: "archived"}); !errors.Is(err, ErrInvalidTaskBucket) {
		t.Fatalf("ListFileTasks() error = %v, want ErrInvalidTaskBucket", err)
	}
}

func TestListFileTasksReportsRetryabilityPerFile(t *testing.T) {
	cfg := emptyTestConfig()
	signature := configuredModelSignature(cfg)
	now := time.Now()
	task := func(fileID, category, status string, updatedAt time.Time) domainconversation.FileObject {
		return domainconversation.FileObject{
			FileID:          fileID,
			UserID:          3,
			FileName:        fileID + ".txt",
			MimeType:        "text/plain",
			FileCategory:    category,
			StoragePath:     "uploads/" + fileID,
			Status:          "active",
			ProcessingReady: true,
			EmbedStatus:     status,
			EmbedSignature:  signature,
			UpdatedAt:       updatedAt,
		}
	}
	repo := &reindexRepo{files: []domainconversation.FileObject{
		task("running", "text", "queued", now),
		task("stalled", "text", "queued", now.Add(-2*time.Hour)),
		task("failed", "text", "failed", now),
		task("video", "video", "none", now),
	}}
	service := newTestService(cfg, repo, nil, nil, nil)

	page, err := service.ListFileTasks(context.Background(), ListFileTasksInput{Bucket: TaskBucketPending, Page: 2, PageSize: 10})
	if err != nil {
		t.Fatalf("ListFileTasks() error = %v", err)
	}
	if repo.taskInput.Offset != 10 || repo.taskInput.Limit != 10 || len(repo.taskInput.Matches) != 3 {
		t.Fatalf("unexpected repository input: %+v", repo.taskInput)
	}
	if page.Total != 4 || len(page.Items) != 4 {
		t.Fatalf("unexpected page: %+v", page)
	}
	want := map[string]struct {
		stalled bool
		reason  string
	}{
		"running": {stalled: false, reason: SkipReasonProcessing},
		"stalled": {stalled: true, reason: ""},
		"failed":  {stalled: false, reason: ""},
		"video":   {stalled: false, reason: SkipReasonUnsupported},
	}
	for _, item := range page.Items {
		expected := want[item.File.FileID]
		if item.Stalled != expected.stalled || item.RetryBlockedReason != expected.reason {
			t.Fatalf("%s: stalled=%v reason=%q, want stalled=%v reason=%q",
				item.File.FileID, item.Stalled, item.RetryBlockedReason, expected.stalled, expected.reason)
		}
	}
}

func TestListFileTasksBucketMatches(t *testing.T) {
	cases := map[string][]string{
		TaskBucketReady:       {"ready"},
		TaskBucketFailed:      {"failed", "none"},
		TaskBucketStale:       {"stale"},
		TaskBucketEmpty:       {"empty"},
		TaskBucketPending:     {"none", "queued", "processing"},
		TaskBucketUnsupported: {"none"},
	}
	for bucket, wantStatuses := range cases {
		matches, ok := taskBucketMatches(bucket)
		if !ok {
			t.Fatalf("bucket %q must be supported", bucket)
		}
		statuses := make([]string, 0, len(matches))
		for _, match := range matches {
			statuses = append(statuses, match.Status)
		}
		if !slices.Equal(statuses, wantStatuses) {
			t.Fatalf("bucket %q matches %v, want %v", bucket, statuses, wantStatuses)
		}
	}
}

func TestEmbeddableFileScopeFollowsImageCapabilities(t *testing.T) {
	textOnly := embeddableFileScope(config.Config{})
	for _, category := range textOnly.Categories {
		if category == "image" {
			t.Fatalf("images must be unsupported without OCR or an image-capable protocol: %+v", textOnly)
		}
	}

	ocr := embeddableFileScope(config.Config{ExtractImageOCREnabled: true})
	if !slices.Contains(ocr.Categories, "image") || len(ocr.ImageMIMETypes) != 0 {
		t.Fatalf("OCR makes every image embeddable: %+v", ocr)
	}

	pixels := embeddableFileScope(config.Config{EmbeddingProtocol: config.EmbeddingProtocolGemini})
	if !slices.Contains(pixels.Categories, "image") || !slices.Contains(pixels.ImageMIMETypes, "image/png") || slices.Contains(pixels.ImageMIMETypes, "image/bmp") {
		t.Fatalf("image-capable protocols only accept supported formats: %+v", pixels)
	}
}

func TestPlanAdminFilesKeepsEachFileOwner(t *testing.T) {
	repo := &reindexRepo{vectorAvailable: true, files: []domainconversation.FileObject{
		{FileID: "a", UserID: 3, FileName: "a.txt", MimeType: "text/plain", StoragePath: "uploads/a", Status: "active", ProcessingReady: true, EmbedStatus: "failed"},
		{FileID: "b", UserID: 9, FileName: "b.txt", MimeType: "text/plain", StoragePath: "uploads/b", Status: "active", ProcessingReady: true, EmbedStatus: "none"},
	}}
	service := newTestService(emptyTestConfig(), repo, nil, infraembedding.New(security.OutboundPolicy{}), nil)

	plan, err := service.PlanAdminFiles(context.Background(), []string{"a", "b", "missing"})
	if err != nil {
		t.Fatalf("PlanAdminFiles() error = %v", err)
	}
	owners := map[string]uint{}
	for _, job := range plan.Jobs {
		owners[job.FileID] = job.UserID
	}
	if owners["a"] != 3 || owners["b"] != 9 {
		t.Fatalf("jobs must keep each file owner: %#v", plan.Jobs)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].FileID != "missing" || plan.Skipped[0].Reason != SkipReasonNotFound {
		t.Fatalf("unexpected skipped files: %#v", plan.Skipped)
	}
}

type processingFileRepo struct {
	reindexRepo
	file domainconversation.FileObject
}

func (r *processingFileRepo) GetActiveFileObjectByID(context.Context, uint, string) (*domainconversation.FileObject, error) {
	file := r.file
	return &file, nil
}

func TestProcessTargetedJobOnlyTakesOverProcessingFilesWhenReclaimed(t *testing.T) {
	cfg := emptyTestConfig()
	signature := configuredModelSignature(cfg)
	newRepo := func() *processingFileRepo {
		return &processingFileRepo{
			reindexRepo: reindexRepo{vectorAvailable: true},
			file: domainconversation.FileObject{
				ID: 1, UserID: 3, FileID: "file_running", FileName: "notes.txt", MimeType: "text/plain",
				StoragePath: "uploads/notes.txt", Status: "active", ProcessingReady: true,
				EmbedStatus: "processing", EmbedSignature: signature,
			},
		}
	}
	job := TargetedJob{FileID: "file_running", UserID: 3, EmbeddingSignature: signature, EmbeddingHost: cfg.EmbeddingHost}

	fresh := newRepo()
	service := newTestService(cfg, fresh, nil, infraembedding.New(security.OutboundPolicy{}), nil)
	if err := service.ProcessTargetedJob(context.Background(), job); err != nil {
		t.Fatalf("fresh delivery: %v", err)
	}
	// 另一执行者正在处理时，首次投递的消息不能再跑一遍。
	if len(fresh.statusHistory) != 0 || len(fresh.claimedFileIDs) != 0 {
		t.Fatalf("fresh delivery must leave a running file alone: history=%v claims=%v", fresh.statusHistory, fresh.claimedFileIDs)
	}

	reclaimed := newRepo()
	service = newTestService(cfg, reclaimed, nil, infraembedding.New(security.OutboundPolicy{}), nil)
	job.Reclaimed = true
	_ = service.ProcessTargetedJob(context.Background(), job)
	if len(reclaimed.statusHistory) == 0 || reclaimed.statusHistory[0] != "processing" {
		t.Fatalf("reclaimed delivery must take over and refresh the run, got history %v", reclaimed.statusHistory)
	}
}
