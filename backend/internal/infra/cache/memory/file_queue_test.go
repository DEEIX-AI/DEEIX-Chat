package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestFileProcessingLeaseTransfersOnlyAfterTimeout(t *testing.T) {
	cache := New()
	ctx := context.Background()
	if err := cache.EnqueueFileProcessing(ctx, 7, "file_1", 0, ""); err != nil {
		t.Fatalf("enqueue file: %v", err)
	}
	messages, err := cache.ReadFileProcessingMessages(ctx, "worker_a")
	if err != nil || len(messages) != 1 {
		t.Fatalf("read file message: messages=%#v err=%v", messages, err)
	}
	message := messages[0]
	if claimed, err := cache.ClaimTimedOutFileProcessingMessages(ctx, "worker_b"); err != nil || len(claimed) != 0 {
		t.Fatalf("active lease must not transfer: claimed=%#v err=%v", claimed, err)
	}

	cache.mu.Lock()
	lease := cache.fileProcessingQueue.inflight[message.ID]
	lease.leasedAt = time.Now().Add(-fileProcessingMinIdle - time.Second)
	cache.fileProcessingQueue.inflight[message.ID] = lease
	cache.mu.Unlock()

	claimed, err := cache.ClaimTimedOutFileProcessingMessages(ctx, "worker_b")
	if err != nil || len(claimed) != 1 || !claimed[0].Reclaimed {
		t.Fatalf("claim expired lease: claimed=%#v err=%v", claimed, err)
	}
	if owned, err := cache.RenewFileProcessingMessageLease(ctx, "worker_a", message); err != nil || owned {
		t.Fatalf("previous owner retained lease: owned=%v err=%v", owned, err)
	}
	if settled, err := cache.SettleFileProcessingMessage(ctx, "worker_a", message); err != nil || settled {
		t.Fatalf("previous owner settled transferred message: settled=%v err=%v", settled, err)
	}
	if owned, err := cache.RenewFileProcessingMessageLease(ctx, "worker_b", message); err != nil || !owned {
		t.Fatalf("new owner does not own lease: owned=%v err=%v", owned, err)
	}
	if settled, err := cache.SettleFileProcessingMessage(ctx, "worker_b", message); err != nil || !settled {
		t.Fatalf("new owner failed to settle message: settled=%v err=%v", settled, err)
	}
}

func TestEnqueueFileProcessingRejectsWhenQueueFull(t *testing.T) {
	cache := New()
	ctx := context.Background()

	// 先制造一条 inflight 消息，用于后续验证重入队路径不受上限约束。
	if err := cache.EnqueueFileProcessing(ctx, 1, "file_inflight", 0, ""); err != nil {
		t.Fatalf("enqueue inflight file: %v", err)
	}
	messages, err := cache.ReadFileProcessingMessages(ctx, "worker_a")
	if err != nil || len(messages) != 1 {
		t.Fatalf("read inflight message: messages=%#v err=%v", messages, err)
	}

	cache.mu.Lock()
	cache.fileProcessingQueue.queue = make([]repository.FileProcessingMessage, maxFileQueueLength)
	cache.mu.Unlock()

	if err := cache.EnqueueFileProcessing(ctx, 2, "file_overflow", 0, ""); !errors.Is(err, repository.ErrFileProcessingQueueFull) {
		t.Fatalf("expected ErrFileProcessingQueueFull, got %v", err)
	}

	requeued, err := cache.RequeueFileProcessingMessage(ctx, "worker_a", messages[0], 1, "retry")
	if err != nil || !requeued {
		t.Fatalf("requeue must bypass queue length limit: requeued=%v err=%v", requeued, err)
	}
}

func TestFileEmbeddingMessagePreservesJobMetadataWhenRequeued(t *testing.T) {
	cache := New()
	ctx := context.Background()
	if err := cache.EnqueueFileEmbedding(ctx, 0, "file_platform", "signature@1536", "https://embedding.example/v1/"); err != nil {
		t.Fatalf("enqueue embedding: %v", err)
	}

	messages, err := cache.ReadFileEmbeddingMessages(ctx, "worker_a")
	if err != nil || len(messages) != 1 {
		t.Fatalf("read embedding message: messages=%#v err=%v", messages, err)
	}
	message := messages[0]
	if message.Kind != repository.FileProcessingKindEmbedding || message.UserID != 0 ||
		message.EmbeddingSignature != "signature@1536" || message.EmbeddingHost != "https://embedding.example/v1" {
		t.Fatalf("unexpected embedding message: %#v", message)
	}

	requeued, err := cache.RequeueFileProcessingMessage(ctx, "worker_a", message, 1, "temporary failure")
	if err != nil || !requeued {
		t.Fatalf("requeue embedding message: requeued=%v err=%v", requeued, err)
	}
	messages, err = cache.ReadFileEmbeddingMessages(ctx, "worker_b")
	if err != nil || len(messages) != 1 {
		t.Fatalf("read requeued embedding message: messages=%#v err=%v", messages, err)
	}
	retried := messages[0]
	if retried.Kind != message.Kind || retried.UserID != message.UserID || retried.FileID != message.FileID ||
		retried.EmbeddingSignature != message.EmbeddingSignature || retried.EmbeddingHost != message.EmbeddingHost ||
		retried.Retry != 1 || retried.LastError != "temporary failure" {
		t.Fatalf("requeued embedding metadata changed: %#v", retried)
	}
}

func TestFileEmbeddingQueueIsIsolatedFromExtractionQueue(t *testing.T) {
	cache := New()
	ctx := context.Background()
	if err := cache.EnqueueFileProcessing(ctx, 7, "extract_file", 0, ""); err != nil {
		t.Fatalf("enqueue extraction: %v", err)
	}
	if err := cache.EnqueueFileEmbedding(ctx, 7, "embed_file", "signature@1536", "https://embedding.example/v1"); err != nil {
		t.Fatalf("enqueue embedding: %v", err)
	}

	extractionMessages, err := cache.ReadFileProcessingMessages(ctx, "extract_worker")
	if err != nil || len(extractionMessages) != 1 || extractionMessages[0].FileID != "extract_file" {
		t.Fatalf("extraction queue returned %#v, err=%v", extractionMessages, err)
	}
	if extractionMessages[0].Queue != repository.FileProcessingQueueDefault {
		t.Fatalf("extraction message queue = %q", extractionMessages[0].Queue)
	}

	embeddingMessages, err := cache.ReadFileEmbeddingMessages(ctx, "embedding_worker")
	if err != nil || len(embeddingMessages) != 1 || embeddingMessages[0].FileID != "embed_file" {
		t.Fatalf("embedding queue returned %#v, err=%v", embeddingMessages, err)
	}
	if embeddingMessages[0].Queue != repository.FileProcessingQueueEmbedding {
		t.Fatalf("embedding message queue = %q", embeddingMessages[0].Queue)
	}
}
