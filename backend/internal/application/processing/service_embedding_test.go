package processing

import (
	"context"
	"errors"
	"testing"
	"time"

	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	memorycache "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	infraembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

type targetedEmbeddingRepositoryStub struct {
	files         []domainconversation.FileObject
	queueErrors   map[string]error
	vectorError   error
	statusHistory map[string][]string
	resetFileIDs  []string
}

func (r *targetedEmbeddingRepositoryStub) VectorStoreAvailable(context.Context) (bool, error) {
	if r.vectorError != nil {
		return false, r.vectorError
	}
	return true, nil
}

func (r *targetedEmbeddingRepositoryStub) GetActiveFileObjectByID(_ context.Context, userID uint, fileID string) (*domainconversation.FileObject, error) {
	for i := range r.files {
		if r.files[i].UserID == userID && r.files[i].FileID == fileID {
			file := r.files[i]
			return &file, nil
		}
	}
	return nil, nil
}

func (r *targetedEmbeddingRepositoryStub) GetActiveFileObjectsByIDs(_ context.Context, userID uint, fileIDs []string) ([]domainconversation.FileObject, error) {
	wanted := make(map[string]struct{}, len(fileIDs))
	for _, fileID := range fileIDs {
		wanted[fileID] = struct{}{}
	}
	files := make([]domainconversation.FileObject, 0, len(fileIDs))
	for i := range r.files {
		if r.files[i].UserID != userID {
			continue
		}
		if _, ok := wanted[r.files[i].FileID]; ok {
			files = append(files, r.files[i])
		}
	}
	return files, nil
}

func (r *targetedEmbeddingRepositoryStub) GetActiveFileObjectsByFileIDs(_ context.Context, fileIDs []string) ([]domainconversation.FileObject, error) {
	wanted := make(map[string]struct{}, len(fileIDs))
	for _, fileID := range fileIDs {
		wanted[fileID] = struct{}{}
	}
	files := make([]domainconversation.FileObject, 0, len(fileIDs))
	for i := range r.files {
		if _, ok := wanted[r.files[i].FileID]; ok {
			files = append(files, r.files[i])
		}
	}
	return files, nil
}

func (*targetedEmbeddingRepositoryStub) GetFileObjectProcessingByObjectID(context.Context, uint) (*domainconversation.FileObjectProcessing, error) {
	return nil, nil
}

func (r *targetedEmbeddingRepositoryStub) QueueFileEmbedding(_ context.Context, _ uint, fileID, _ string, _ time.Time) (bool, error) {
	if err := r.queueErrors[fileID]; err != nil {
		return false, err
	}
	return true, nil
}

func (*targetedEmbeddingRepositoryStub) ClaimFileEmbedding(context.Context, uint, string, string) (bool, error) {
	return true, nil
}

func (r *targetedEmbeddingRepositoryStub) UpdateFileObjectEmbedStatus(_ context.Context, _ uint, fileID, _ string, status, _ string) (bool, error) {
	if r.statusHistory == nil {
		r.statusHistory = make(map[string][]string)
	}
	r.statusHistory[fileID] = append(r.statusHistory[fileID], status)
	return true, nil
}

func (*targetedEmbeddingRepositoryStub) UpdateFileObjectChunkCount(context.Context, uint, string, int) (bool, error) {
	return true, nil
}

func (*targetedEmbeddingRepositoryStub) ReplaceFileChunks(context.Context, uint, string, []domainconversation.FileChunk, [][]float32) (bool, error) {
	return true, nil
}

func (*targetedEmbeddingRepositoryStub) MarkEmbeddedFilesStale(context.Context, string) (int64, error) {
	return 0, nil
}

func (r *targetedEmbeddingRepositoryStub) ResetFileForReprocessing(_ context.Context, _ uint, fileID string, _ string, _ time.Time) (bool, error) {
	r.resetFileIDs = append(r.resetFileIDs, fileID)
	return true, nil
}

func (*targetedEmbeddingRepositoryStub) ListFilesForReprocessing(context.Context, repository.ListFilesForReprocessingInput) ([]domainconversation.FileObject, error) {
	return nil, nil
}

func (*targetedEmbeddingRepositoryStub) MarkStalledFileEmbeddingsFailed(context.Context, time.Time, string) (int64, error) {
	return 0, nil
}

func (*targetedEmbeddingRepositoryStub) CountFileEmbeddingStates(context.Context, repository.EmbeddableFileScope, time.Time) ([]repository.FileEmbeddingStateCount, error) {
	return nil, nil
}

func (*targetedEmbeddingRepositoryStub) ListFileEmbeddingTasks(context.Context, repository.ListFileEmbeddingTasksInput) ([]domainconversation.FileObject, int64, error) {
	return nil, 0, nil
}

func (*targetedEmbeddingRepositoryStub) ListFilesForReindex(context.Context, repository.ListFilesForReindexInput) ([]domainconversation.FileObject, error) {
	return nil, nil
}

type selectiveEmbeddingQueue struct {
	*memorycache.Cache
	enqueueErrors map[string]error
}

func (q *selectiveEmbeddingQueue) EnqueueFileEmbedding(
	ctx context.Context,
	userID uint,
	fileID string,
	embeddingSignature string,
	embeddingHost string,
) error {
	if err := q.enqueueErrors[fileID]; err != nil {
		return err
	}
	return q.Cache.EnqueueFileEmbedding(ctx, userID, fileID, embeddingSignature, embeddingHost)
}

func TestSubmitFileEmbeddingsKeepsPerFileFailuresIsolated(t *testing.T) {
	cfg := targetedEmbeddingTestConfig()
	repo := &targetedEmbeddingRepositoryStub{
		files: []domainconversation.FileObject{
			targetedEmbeddingTestFile("submitted"),
			targetedEmbeddingTestFile("state_failed"),
			targetedEmbeddingTestFile("queue_failed"),
		},
		queueErrors: map[string]error{"state_failed": errors.New("database unavailable")},
	}
	queue := &selectiveEmbeddingQueue{
		Cache:         memorycache.New(),
		enqueueErrors: map[string]error{"queue_failed": errors.New("queue unavailable")},
	}
	embeddingSvc := appembedding.NewServiceWithRuntime(
		config.NewRuntime(cfg),
		repo,
		nil,
		infraembedding.New(security.OutboundPolicy{}),
		nil,
	)
	service := NewServiceWithRuntime(Dependencies{
		Config:           config.NewRuntime(cfg),
		Cache:            queue,
		EmbeddingService: embeddingSvc,
		ExtractorVersion: DefaultExtractorVersion,
	})

	result, err := service.SubmitFileEmbeddings(
		context.Background(),
		7,
		[]string{"submitted", "state_failed", "queue_failed"},
	)
	if err != nil {
		t.Fatalf("submit embeddings: %v", err)
	}
	if len(result.SubmittedFileIDs) != 1 || result.SubmittedFileIDs[0] != "submitted" {
		t.Fatalf("submitted files = %#v", result.SubmittedFileIDs)
	}
	wantSkipped := map[string]string{
		"state_failed": appembedding.SkipReasonSubmitFailed,
		"queue_failed": appembedding.SkipReasonQueueBusy,
	}
	if len(result.Skipped) != len(wantSkipped) {
		t.Fatalf("skipped files = %#v", result.Skipped)
	}
	for _, skipped := range result.Skipped {
		if wantSkipped[skipped.FileID] != skipped.Reason {
			t.Fatalf("skip %s = %s", skipped.FileID, skipped.Reason)
		}
	}
	if history := repo.statusHistory["queue_failed"]; len(history) != 1 || history[0] != "failed" {
		t.Fatalf("queue failure status history = %#v", history)
	}
	messages, err := queue.ReadFileEmbeddingMessages(context.Background(), "embedding_worker")
	if err != nil || len(messages) != 1 || messages[0].FileID != "submitted" {
		t.Fatalf("queued messages = %#v, err=%v", messages, err)
	}
}

func TestSubmitAdminFileEmbeddingsQueuesJobsForEachFileOwner(t *testing.T) {
	cfg := targetedEmbeddingTestConfig()
	failed := targetedEmbeddingTestFile("failed_other_user")
	failed.UserID = 9
	failed.EmbedStatus = "failed"
	stalled := targetedEmbeddingTestFile("stalled_queue")
	stalled.EmbedStatus = "queued"
	stalled.EmbedSignature = appembedding.ComputeModelSignature(cfg.RAGModel, cfg.EmbeddingOutputDimensions)
	stalled.UpdatedAt = time.Now().Add(-2 * time.Hour)
	running := targetedEmbeddingTestFile("running")
	running.EmbedStatus = "processing"
	running.EmbedSignature = stalled.EmbedSignature
	running.UpdatedAt = time.Now()
	repo := &targetedEmbeddingRepositoryStub{files: []domainconversation.FileObject{failed, stalled, running}}
	queue := memorycache.New()
	embeddingSvc := appembedding.NewServiceWithRuntime(
		config.NewRuntime(cfg),
		repo,
		nil,
		infraembedding.New(security.OutboundPolicy{}),
		nil,
	)
	service := NewServiceWithRuntime(Dependencies{
		Config:           config.NewRuntime(cfg),
		Cache:            queue,
		EmbeddingService: embeddingSvc,
		ExtractorVersion: DefaultExtractorVersion,
	})

	result, err := service.SubmitAdminFileEmbeddings(
		context.Background(),
		[]string{"failed_other_user", "stalled_queue", "running"},
	)
	if err != nil {
		t.Fatalf("submit admin embeddings: %v", err)
	}
	if len(result.SubmittedFileIDs) != 2 {
		t.Fatalf("submitted files = %#v", result.SubmittedFileIDs)
	}
	// 仍在执行中的任务不能被重复提交；停滞的排队任务允许重新投递。
	if len(result.Skipped) != 1 || result.Skipped[0].FileID != "running" || result.Skipped[0].Reason != appembedding.SkipReasonProcessing {
		t.Fatalf("skipped files = %#v", result.Skipped)
	}
	owners := map[string]uint{}
	for range result.SubmittedFileIDs {
		messages, readErr := queue.ReadFileEmbeddingMessages(context.Background(), "embedding_worker")
		if readErr != nil || len(messages) != 1 {
			t.Fatalf("queued messages = %#v, err=%v", messages, readErr)
		}
		owners[messages[0].FileID] = messages[0].UserID
	}
	// 任务必须归属文件所有者，worker 才能按所有者作用域读取文件。
	if owners["failed_other_user"] != 9 || owners["stalled_queue"] != 7 {
		t.Fatalf("queued message owners = %#v", owners)
	}
}

func TestEmbeddingDeadLetterFinalizesFileStatus(t *testing.T) {
	cfg := targetedEmbeddingTestConfig()
	repo := &targetedEmbeddingRepositoryStub{vectorError: errors.New("vector store unavailable")}
	queue := memorycache.New()
	embeddingSvc := appembedding.NewServiceWithRuntime(
		config.NewRuntime(cfg),
		repo,
		nil,
		infraembedding.New(security.OutboundPolicy{}),
		nil,
	)
	service := NewServiceWithRuntime(Dependencies{
		Config:           config.NewRuntime(cfg),
		Cache:            queue,
		EmbeddingService: embeddingSvc,
		ExtractorVersion: DefaultExtractorVersion,
	})
	signature := appembedding.ComputeModelSignature(cfg.RAGModel, cfg.EmbeddingOutputDimensions)
	if err := queue.EnqueueFileEmbedding(context.Background(), 7, "file_1", signature, cfg.EmbeddingHost); err != nil {
		t.Fatalf("enqueue embedding: %v", err)
	}
	messages, err := queue.ReadFileEmbeddingMessages(context.Background(), "embedding_worker")
	if err != nil || len(messages) != 1 {
		t.Fatalf("read embedding: messages=%#v err=%v", messages, err)
	}
	message := messages[0]
	message.Retry = fileProcessingMaxRetries
	service.handleEmbeddingMessage(context.Background(), "embedding_worker", message)

	if history := repo.statusHistory["file_1"]; len(history) != 1 || history[0] != "failed" {
		t.Fatalf("terminal embedding status history = %#v", history)
	}
	if owned, renewErr := queue.RenewFileProcessingMessageLease(context.Background(), "embedding_worker", message); renewErr != nil || owned {
		t.Fatalf("dead-lettered message still owned: owned=%v err=%v", owned, renewErr)
	}
}

func targetedEmbeddingTestConfig() config.Config {
	return config.Config{
		EmbeddingEnabled:          true,
		RAGModel:                  "text-embedding-test",
		EmbeddingHost:             "https://embedding.example/v1",
		EmbeddingOutputDimensions: 1536,
	}
}

func targetedEmbeddingTestFile(fileID string) domainconversation.FileObject {
	return domainconversation.FileObject{
		ID:              1,
		UserID:          7,
		FileID:          fileID,
		FileName:        fileID + ".md",
		MimeType:        "text/markdown",
		FileCategory:    "text",
		StoragePath:     "uploads/" + fileID + ".md",
		Status:          "active",
		ProcessingReady: true,
		ExtractStatus:   "ready",
		EmbedStatus:     "none",
	}
}

var _ repository.EmbeddingRepository = (*targetedEmbeddingRepositoryStub)(nil)
var _ repository.FileProcessingQueueRepository = (*selectiveEmbeddingQueue)(nil)
