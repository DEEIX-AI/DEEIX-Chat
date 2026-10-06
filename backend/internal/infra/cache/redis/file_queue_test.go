package redis

import (
	"context"
	"os"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	goredis "github.com/go-redis/redis/v8"
)

func TestParseFileEmbeddingMessagePreservesMetadata(t *testing.T) {
	message, err := parseFileProcessingMessage(goredis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"user_id":             "7",
			"file_id":             "file_1",
			"retry":               "1",
			"kind":                repository.FileProcessingKindEmbedding,
			"embedding_signature": "model@1536",
			"embedding_host":      "https://embedding.example/v1/",
		},
	})
	if err != nil {
		t.Fatalf("parse embedding message: %v", err)
	}
	if message.UserID != 7 || message.FileID != "file_1" || message.Retry != 1 ||
		message.Kind != repository.FileProcessingKindEmbedding ||
		message.EmbeddingSignature != "model@1536" ||
		message.EmbeddingHost != "https://embedding.example/v1" {
		t.Fatalf("unexpected embedding message: %#v", message)
	}
}

func TestFileProcessingRedisRecoveryEnqueueIsIdempotent(t *testing.T) {
	addr := os.Getenv("DEEIX_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set DEEIX_TEST_REDIS_ADDR to a dedicated Redis test instance")
	}
	client := goredis.NewClient(&goredis.Options{Addr: addr, DB: 15})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.FlushDB(ctx).Err() })
	cache := &conversationCache{client: client}
	if err := cache.InitFileProcessingStream(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if err := cache.EnqueueFileProcessing(ctx, 1, "file", 0, ""); err != nil {
			t.Fatal(err)
		}
	}
	if count, err := client.XLen(ctx, fileProcessingStreamName).Result(); err != nil || count != 1 {
		t.Fatalf("stream length %d: %v", count, err)
	}
	messages, err := cache.ReadFileProcessingMessages(ctx, "worker")
	if err != nil || len(messages) != 1 {
		t.Fatalf("read: %#v / %v", messages, err)
	}
	if ok, retryErr := cache.RequeueFileProcessingMessage(ctx, "worker", messages[0], 1, "retry"); retryErr != nil || !ok {
		t.Fatal(retryErr)
	}
	if err = cache.EnqueueFileProcessing(ctx, 1, "file", 0, ""); err != nil {
		t.Fatal(err)
	}
	if count, err := client.XLen(ctx, fileProcessingStreamName).Result(); err != nil || count != 1 {
		t.Fatalf("retry duplicated stream: %d / %v", count, err)
	}
	messages, err = cache.ReadFileProcessingMessages(ctx, "worker")
	if err != nil || len(messages) != 1 {
		t.Fatal(err)
	}
	if ok, settleErr := cache.SettleFileProcessingMessage(ctx, "worker", messages[0]); settleErr != nil || !ok {
		t.Fatal(settleErr)
	}
	if client.Exists(ctx, processingOutstandingKey(1, "file")).Val() != 0 {
		t.Fatal("settlement leaked key")
	}
	if err = cache.EnqueueFileProcessing(ctx, 1, "file", 0, ""); err != nil {
		t.Fatal(err)
	}
	messages, err = cache.ReadFileProcessingMessages(ctx, "worker")
	if err != nil || len(messages) != 1 {
		t.Fatal(err)
	}
	if ok, dlqErr := cache.DeadLetterFileProcessingMessage(ctx, "worker", messages[0], "terminal"); dlqErr != nil || !ok {
		t.Fatal(dlqErr)
	}
	if client.Exists(ctx, processingOutstandingKey(1, "file")).Val() != 0 {
		t.Fatal("dead letter leaked key")
	}
	// Stream IDs are only unique within a stream. An embedding settlement
	// must not touch an extraction marker even if the IDs happen to match.
	if err = cache.EnqueueFileEmbedding(ctx, 1, "file", "sig", "https://embedding.example"); err != nil {
		t.Fatal(err)
	}
	embed, err := cache.ReadFileEmbeddingMessages(ctx, "worker")
	if err != nil || len(embed) != 1 {
		t.Fatal(err)
	}
	if err = client.Set(ctx, processingOutstandingKey(1, "file"), embed[0].ID, 0).Err(); err != nil {
		t.Fatal(err)
	}
	if ok, settleErr := cache.SettleFileProcessingMessage(ctx, "worker", embed[0]); settleErr != nil || !ok {
		t.Fatal(settleErr)
	}
	if client.Exists(ctx, processingOutstandingKey(1, "file")).Val() != 1 {
		t.Fatal("embedding settled extraction marker")
	}
	// The legacy parser rejects zero-owner extraction. Invalid-message DLQ
	// cleanup must still remove the outstanding marker atomically.
	if err = cache.EnqueueFileProcessing(ctx, 0, "invalid", 0, ""); err != nil {
		t.Fatal(err)
	}
	_, err = cache.ReadFileProcessingMessages(ctx, "worker")
	if err != nil {
		t.Fatal(err)
	}
	if client.Exists(ctx, processingOutstandingKey(0, "invalid")).Val() != 0 {
		t.Fatal("invalid-message DLQ leaked marker")
	}
}

func TestRedisQueueForMessagePreservesLegacySourceQueue(t *testing.T) {
	legacy := repository.FileProcessingMessage{
		Kind:  repository.FileProcessingKindEmbedding,
		Queue: repository.FileProcessingQueueDefault,
	}
	if queue := redisQueueForMessage(legacy); queue.stream != fileProcessingStreamName {
		t.Fatalf("legacy message routed to %q, want %q", queue.stream, fileProcessingStreamName)
	}

	current := repository.FileProcessingMessage{
		Kind:  repository.FileProcessingKindEmbedding,
		Queue: repository.FileProcessingQueueEmbedding,
	}
	if queue := redisQueueForMessage(current); queue.stream != fileEmbeddingStreamName {
		t.Fatalf("embedding message routed to %q, want %q", queue.stream, fileEmbeddingStreamName)
	}
}
