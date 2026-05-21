package repository

import (
	"context"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

// FileProcessingMessage 文件处理队列消息。
type FileProcessingMessage struct {
	// ID 是 Redis Stream 消息 ID。
	ID        string
	UserID    uint
	FileID    string
	Retry     int
	LastError string
}

// GenerationStreamMessage 是生成流中的一条可恢复事件。
type GenerationStreamMessage struct {
	ID          string
	Seq         int64
	PayloadJSON string
}

// FileProcessingQueueRepository 封装文件处理队列缓存能力。
type FileProcessingQueueRepository interface {
	InitFileProcessingStream(ctx context.Context) error
	EnqueueFileProcessing(ctx context.Context, userID uint, fileID string, retry int, lastError string) error
	ClaimTimedOutFileProcessingMessages(ctx context.Context, consumerName string) ([]FileProcessingMessage, error)
	ReadFileProcessingMessages(ctx context.Context, consumerName string) ([]FileProcessingMessage, error)
	AckFileProcessingMessage(ctx context.Context, messageID string) error
	DeleteFileProcessingMessage(ctx context.Context, messageID string) error
	SendFileProcessingToDLQ(ctx context.Context, userID uint, fileID string, retry int, lastError string) error
}

// RAGCacheRepository 封装 RAG 检索缓存能力。
type RAGCacheRepository interface {
	GetRAGCache(ctx context.Context, key string) (chunks []domainconversation.RAGChunk, ok bool)
	SetRAGCache(ctx context.Context, key string, chunks []domainconversation.RAGChunk, ttl time.Duration)
}

// GenerationStreamCacheRepository 封装对话生成流的短期恢复存储。
type GenerationStreamCacheRepository interface {
	RegisterGenerationStream(ctx context.Context, runID string, userID uint, ttl time.Duration) error
	GetGenerationStreamOwner(ctx context.Context, runID string) (uint, bool, error)
	TouchGenerationStreamActive(ctx context.Context, runID string, ttl time.Duration) error
	ClearGenerationStreamActive(ctx context.Context, runID string) error
	IsGenerationStreamActive(ctx context.Context, runID string) (bool, error)
	RequestGenerationStreamCancel(ctx context.Context, runID string, ttl time.Duration) error
	IsGenerationStreamCanceled(ctx context.Context, runID string) (bool, error)
	AppendGenerationStreamEvent(ctx context.Context, runID string, payloadJSON string, maxEvents int64, ttl time.Duration) (GenerationStreamMessage, error)
	ListGenerationStreamEvents(ctx context.Context, runID string, limit int64) ([]GenerationStreamMessage, error)
	ReadGenerationStreamEvents(ctx context.Context, runID string, afterID string, block time.Duration, limit int64) ([]GenerationStreamMessage, error)
	ExpireGenerationStream(ctx context.Context, runID string, ttl time.Duration) error
}

// ConversationCacheRepository 聚合 conversation 领域缓存能力。
type ConversationCacheRepository interface {
	FileProcessingQueueRepository
	RAGCacheRepository
	GenerationStreamCacheRepository
}
