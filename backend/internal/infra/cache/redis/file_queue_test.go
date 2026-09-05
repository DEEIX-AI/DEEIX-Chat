package cache

import (
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/go-redis/redis/v8"
)

func TestParseFileEmbeddingMessagePreservesMetadata(t *testing.T) {
	message, err := parseFileProcessingMessage(redis.XMessage{
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
