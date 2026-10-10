package conversation

import (
	"context"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const embeddingTaskTestSignature = "model@1536"

func openEmbeddingTaskSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:embedding_tasks?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.FileObject{}); err != nil {
		t.Fatalf("migrate file objects: %v", err)
	}
	return db
}

func seedEmbeddingTaskFiles(t *testing.T, db *gorm.DB, now time.Time) {
	t.Helper()
	old := now.Add(-2 * time.Hour)
	file := func(id uint, fileID string, userID uint, category, mimeType, embedStatus string, updatedAt time.Time) models.FileObject {
		item := models.FileObject{
			FileID:           fileID,
			UserID:           userID,
			FileName:         fileID + ".bin",
			MimeType:         mimeType,
			DetectedMIME:     mimeType,
			FileCategory:     category,
			StoragePath:      "uploads/" + fileID,
			Status:           "active",
			ProcessingStatus: "ready",
			ProcessingReady:  true,
			ExtractStatus:    "ready",
			EmbedStatus:      embedStatus,
			EmbedSignature:   embeddingTaskTestSignature,
		}
		item.ID = id
		item.CreatedAt = updatedAt
		item.UpdatedAt = updatedAt
		return item
	}
	missingObject := file(10, "text_without_object", 1, "text", "text/plain", "none", now)
	missingObject.StoragePath = ""
	deleted := file(11, "deleted_failed", 1, "text", "text/plain", "failed", now)
	deleted.Status = "deleted"
	files := []models.FileObject{
		file(1, "text_none", 1, "text", "text/plain", "none", now),
		file(2, "video_none", 1, "video", "video/mp4", "none", now),
		file(3, "png_none", 2, "image", "image/png", "none", now),
		file(4, "bmp_none", 2, "image", "image/bmp", "none", now),
		file(5, "queued_stalled", 2, "text", "text/plain", "queued", old),
		file(6, "processing_stalled", 3, "pdf", "application/pdf", "processing", old),
		file(7, "processing_fresh", 3, "pdf", "application/pdf", "processing", now),
		file(8, "failed_text", 3, "text", "text/plain", "failed", now),
		file(9, "ready_text", 3, "text", "text/plain", "ready", now),
		missingObject,
		deleted,
	}
	if err := db.Create(&files).Error; err != nil {
		t.Fatalf("seed files: %v", err)
	}
}

func embeddingTaskTestScope() repository.EmbeddableFileScope {
	return repository.EmbeddableFileScope{
		Categories:     []string{"text", "pdf", "image"},
		ImageMIMETypes: []string{"image/png", "image/jpeg"},
	}
}

func TestCountFileEmbeddingStatesSplitsUnsupportedAndStalledFiles(t *testing.T) {
	db := openEmbeddingTaskSQLiteTestDB(t)
	now := time.Now()
	seedEmbeddingTaskFiles(t, db, now)

	counts, err := NewRepo(db).CountFileEmbeddingStates(context.Background(), embeddingTaskTestScope(), now.Add(-30*time.Minute))
	if err != nil {
		t.Fatalf("count states: %v", err)
	}
	type key struct {
		status     string
		embeddable bool
		stalled    bool
	}
	got := map[key]int64{}
	for _, item := range counts {
		got[key{item.Status, item.Embeddable, item.Stalled}] += item.Count
	}
	want := map[key]int64{
		{"none", true, false}:       2, // text_none, png_none
		{"none", false, false}:      3, // video、bmp（格式不受支持）、缺少文件实体
		{"queued", true, true}:      1,
		{"processing", true, true}:  1,
		{"processing", true, false}: 1,
		{"failed", true, false}:     1, // 已删除文件不计入
		{"ready", true, false}:      1,
	}
	if len(got) != len(want) {
		t.Fatalf("state counts = %#v, want %#v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("state %#v = %d, want %d (all=%#v)", k, got[k], v, got)
		}
	}
}

func TestListFileEmbeddingTasksFiltersBucketsAcrossUsers(t *testing.T) {
	db := openEmbeddingTaskSQLiteTestDB(t)
	now := time.Now()
	seedEmbeddingTaskFiles(t, db, now)
	repo := NewRepo(db)
	embeddable := true
	unsupported := false

	pending, total, err := repo.ListFileEmbeddingTasks(context.Background(), repository.ListFileEmbeddingTasksInput{
		Matches: []repository.FileEmbedStatusMatch{
			{Status: "none", Embeddable: &embeddable},
			{Status: "queued"},
			{Status: "processing"},
		},
		Scope: embeddingTaskTestScope(),
		Limit: 50,
	})
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if total != 5 || len(pending) != 5 {
		t.Fatalf("pending total=%d items=%d", total, len(pending))
	}
	// 按最近更新时间倒序：停滞任务排在最后，便于先看到正在进行的任务。
	if pending[len(pending)-1].UpdatedAt.After(pending[0].UpdatedAt) {
		t.Fatalf("pending must be ordered by updated_at desc: %#v", pending)
	}
	owners := map[uint]bool{}
	for _, item := range pending {
		owners[item.UserID] = true
	}
	if len(owners) != 3 {
		t.Fatalf("pending tasks must span all users, got owners %#v", owners)
	}

	unsupportedFiles, total, err := repo.ListFileEmbeddingTasks(context.Background(), repository.ListFileEmbeddingTasksInput{
		Matches: []repository.FileEmbedStatusMatch{{Status: "none", Embeddable: &unsupported}},
		Scope:   embeddingTaskTestScope(),
		Limit:   50,
	})
	if err != nil || total != 3 || len(unsupportedFiles) != 3 {
		t.Fatalf("unsupported total=%d items=%#v err=%v", total, unsupportedFiles, err)
	}

	searched, total, err := repo.ListFileEmbeddingTasks(context.Background(), repository.ListFileEmbeddingTasksInput{
		Matches: []repository.FileEmbedStatusMatch{{Status: "none", Embeddable: &unsupported}},
		Scope:   embeddingTaskTestScope(),
		Query:   "VIDEO",
		Limit:   50,
	})
	if err != nil || total != 1 || len(searched) != 1 || searched[0].FileID != "video_none" {
		t.Fatalf("search total=%d items=%#v err=%v", total, searched, err)
	}
}

func TestMarkStalledFileEmbeddingsFailedOnlyTouchesStalledProcessing(t *testing.T) {
	db := openEmbeddingTaskSQLiteTestDB(t)
	now := time.Now()
	seedEmbeddingTaskFiles(t, db, now)

	affected, err := NewRepo(db).MarkStalledFileEmbeddingsFailed(context.Background(), now.Add(-6*time.Minute), "向量化任务超时或被中断，请重试。")
	if err != nil || affected != 1 {
		t.Fatalf("mark stalled: affected=%d err=%v", affected, err)
	}
	var statuses []models.FileObject
	if err := db.Where("file_id IN ?", []string{"processing_stalled", "processing_fresh", "queued_stalled"}).Find(&statuses).Error; err != nil {
		t.Fatalf("load files: %v", err)
	}
	for _, item := range statuses {
		want := map[string]string{
			"processing_stalled": "failed",
			"processing_fresh":   "processing",
			// 排队任务可能只是在等待 worker，不由巡检标记失败。
			"queued_stalled": "queued",
		}[item.FileID]
		if item.EmbedStatus != want {
			t.Fatalf("%s status = %s, want %s", item.FileID, item.EmbedStatus, want)
		}
	}
}

func TestListFilesForReindexIncludesStalledQueuedFiles(t *testing.T) {
	db := openEmbeddingTaskSQLiteTestDB(t)
	now := time.Now()
	seedEmbeddingTaskFiles(t, db, now)

	files, err := NewRepo(db).ListFilesForReindex(context.Background(), repository.ListFilesForReindexInput{
		Limit:         50,
		StalledBefore: now.Add(-30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("list reindex: %v", err)
	}
	got := map[string]bool{}
	for _, item := range files {
		got[item.FileID] = true
	}
	if !got["queued_stalled"] || !got["failed_text"] || !got["text_none"] {
		t.Fatalf("reindex candidates = %#v", got)
	}
	if got["processing_fresh"] || got["processing_stalled"] || got["ready_text"] || got["deleted_failed"] {
		t.Fatalf("reindex must skip running, ready and deleted files: %#v", got)
	}
}

func TestQueueFileEmbeddingRequeuesOnlyStalledTasks(t *testing.T) {
	db := openEmbeddingTaskSQLiteTestDB(t)
	now := time.Now()
	seedEmbeddingTaskFiles(t, db, now)
	repo := NewRepo(db)
	stalledBefore := now.Add(-30 * time.Minute)

	requeued, err := repo.QueueFileEmbedding(context.Background(), 2, "queued_stalled", embeddingTaskTestSignature, stalledBefore)
	if err != nil || !requeued {
		t.Fatalf("stalled queue must be requeued: requeued=%v err=%v", requeued, err)
	}
	var refreshed models.FileObject
	if err := db.Where("file_id = ?", "queued_stalled").Take(&refreshed).Error; err != nil {
		t.Fatalf("load requeued file: %v", err)
	}
	// 重新登记必须刷新更新时间，否则同一任务会立即再次被判定为停滞。
	if !refreshed.UpdatedAt.After(stalledBefore) {
		t.Fatalf("requeue must refresh updated_at, got %s", refreshed.UpdatedAt)
	}
	if duplicate, dupErr := repo.QueueFileEmbedding(context.Background(), 3, "processing_fresh", embeddingTaskTestSignature, stalledBefore); dupErr != nil || duplicate {
		t.Fatalf("running task must not be requeued: requeued=%v err=%v", duplicate, dupErr)
	}
	if duplicate, dupErr := repo.QueueFileEmbedding(context.Background(), 3, "ready_text", embeddingTaskTestSignature, stalledBefore); dupErr != nil || duplicate {
		t.Fatalf("ready file must not be requeued: requeued=%v err=%v", duplicate, dupErr)
	}
}

func seedReprocessingFiles(t *testing.T, db *gorm.DB, now time.Time) {
	t.Helper()
	old := now.Add(-2 * time.Hour)
	file := func(id uint, fileID, processingStatus, extractStatus, embedStatus string, updatedAt time.Time) models.FileObject {
		item := models.FileObject{
			FileID:           fileID,
			UserID:           5,
			FileName:         fileID + ".pdf",
			MimeType:         "application/pdf",
			DetectedMIME:     "application/pdf",
			FileCategory:     "pdf",
			StoragePath:      "uploads/" + fileID,
			Status:           "active",
			ProcessingStatus: processingStatus,
			ProcessingReady:  processingStatus == "ready" && extractStatus == "ready",
			ExtractStatus:    extractStatus,
			EmbedStatus:      embedStatus,
		}
		item.ID = id
		item.CreatedAt = updatedAt
		item.UpdatedAt = updatedAt
		return item
	}
	files := []models.FileObject{
		file(1, "empty_scan", "ready", "empty", "empty", now),
		file(2, "failed_extract", "failed", "failed", "none", now),
		file(3, "stalled_queue", "queued", "none", "none", old),
		file(4, "fresh_queue", "queued", "none", "none", now),
		file(5, "extracting", "extracting", "processing", "none", old),
		file(6, "has_text", "ready", "ready", "none", now),
	}
	if err := db.Create(&files).Error; err != nil {
		t.Fatalf("seed files: %v", err)
	}
}

func TestResetFileForReprocessingOnlyResetsFilesWithoutText(t *testing.T) {
	db := openEmbeddingTaskSQLiteTestDB(t)
	now := time.Now()
	seedReprocessingFiles(t, db, now)
	repo := NewRepo(db)
	stalledBefore := now.Add(-30 * time.Minute)

	cases := map[string]bool{
		"empty_scan":     true,
		"failed_extract": true,
		"stalled_queue":  true,
		// 刚入队的文件可能马上就会被处理，正在提取的文件不能被打断，已有文本的文件不需要重新提取。
		"fresh_queue": false,
		"extracting":  false,
		"has_text":    false,
	}
	for fileID, want := range cases {
		reset, err := repo.ResetFileForReprocessing(context.Background(), 5, fileID, embeddingTaskTestSignature, stalledBefore)
		if err != nil || reset != want {
			t.Fatalf("%s: reset=%v err=%v, want %v", fileID, reset, err, want)
		}
	}

	var reset models.FileObject
	if err := db.Where("file_id = ?", "empty_scan").Take(&reset).Error; err != nil {
		t.Fatalf("load reset file: %v", err)
	}
	if reset.ProcessingStatus != "queued" || reset.ProcessingReady || reset.ExtractStatus != "none" {
		t.Fatalf("reset file must wait for the pipeline: %+v", reset)
	}
	// 向量化请求登记在文件行上，流水线据此在提取完成后建立索引。
	if reset.EmbedStatus != "queued" || reset.EmbedSignature != embeddingTaskTestSignature {
		t.Fatalf("reset file must carry the embedding request: status=%s signature=%s", reset.EmbedStatus, reset.EmbedSignature)
	}
	// 重置后文件处于排队中：重复提交不能再次投递。
	if again, err := repo.ResetFileForReprocessing(context.Background(), 5, "empty_scan", embeddingTaskTestSignature, stalledBefore); err != nil || again {
		t.Fatalf("duplicate reset must be rejected: reset=%v err=%v", again, err)
	}
}

func TestListFilesForReprocessingHonorsIncludeEmpty(t *testing.T) {
	db := openEmbeddingTaskSQLiteTestDB(t)
	now := time.Now()
	seedReprocessingFiles(t, db, now)
	repo := NewRepo(db)
	collect := func(includeEmpty bool) map[string]bool {
		files, err := repo.ListFilesForReprocessing(context.Background(), repository.ListFilesForReprocessingInput{
			Limit:         50,
			IncludeEmpty:  includeEmpty,
			StalledBefore: now.Add(-30 * time.Minute),
		})
		if err != nil {
			t.Fatalf("list reprocessing: %v", err)
		}
		got := map[string]bool{}
		for _, item := range files {
			got[item.FileID] = true
		}
		return got
	}

	withoutEmpty := collect(false)
	if !withoutEmpty["failed_extract"] || !withoutEmpty["stalled_queue"] || withoutEmpty["empty_scan"] {
		t.Fatalf("default scan covers failed and stalled files only: %v", withoutEmpty)
	}
	withEmpty := collect(true)
	if !withEmpty["empty_scan"] || len(withEmpty) != 3 {
		t.Fatalf("include_empty adds files without text and nothing else: %v", withEmpty)
	}
}

func TestListFilesForReindexSkipsFilesWithoutText(t *testing.T) {
	db := openEmbeddingTaskSQLiteTestDB(t)
	now := time.Now()
	seedReprocessingFiles(t, db, now)

	files, err := NewRepo(db).ListFilesForReindex(context.Background(), repository.ListFilesForReindexInput{
		Limit:         50,
		StalledBefore: now.Add(-30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("list reindex: %v", err)
	}
	// 只有已有文本的文件交给向量化服务；其他文件由处理流水线重新提取，避免重复付费提取。
	if len(files) != 1 || files[0].FileID != "has_text" {
		t.Fatalf("reindex candidates = %+v", files)
	}
}

func TestCountFileEmbeddingStatesFlagsProcessingFailures(t *testing.T) {
	db := openEmbeddingTaskSQLiteTestDB(t)
	now := time.Now()
	seedReprocessingFiles(t, db, now)

	counts, err := NewRepo(db).CountFileEmbeddingStates(context.Background(), embeddingTaskTestScope(), now.Add(-30*time.Minute))
	if err != nil {
		t.Fatalf("count states: %v", err)
	}
	var failedNone, pendingNone int64
	for _, item := range counts {
		if item.Status != "none" || !item.Embeddable {
			continue
		}
		if item.ProcessingFailed {
			failedNone += item.Count
		} else {
			pendingNone += item.Count
		}
	}
	if failedNone != 1 || pendingNone != 4 {
		t.Fatalf("processing failures must be separated from pending files: failed=%d pending=%d", failedNone, pendingNone)
	}
}
