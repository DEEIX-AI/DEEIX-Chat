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
	Reclaimed bool
}

// GenerationStreamMessage 是生成流中的一条可恢复事件。
type GenerationStreamMessage struct {
	ID          string
	Seq         int64
	PayloadJSON string
}

// GenerationStreamAppend 是一次原子追加所需的数据。
// TextDelta 仅在可见文本 delta 事件中设置，用于同步维护完整恢复快照。
type GenerationStreamAppend struct {
	PayloadJSON string
	TextDelta   string
}

// GenerationStreamTextSnapshot 是生成期间可恢复的完整可见文本及其事件序号。
type GenerationStreamTextSnapshot struct {
	Seq     int64
	Content string
}

// FileProcessingQueueRepository 封装文件处理队列缓存能力。
type FileProcessingQueueRepository interface {
	InitFileProcessingStream(ctx context.Context) error
	EnqueueFileProcessing(ctx context.Context, userID uint, fileID string, retry int, lastError string) error
	ClaimTimedOutFileProcessingMessages(ctx context.Context, consumerName string) ([]FileProcessingMessage, error)
	ReadFileProcessingMessages(ctx context.Context, consumerName string) ([]FileProcessingMessage, error)
	RenewFileProcessingMessageLease(ctx context.Context, consumerName, messageID string) (bool, error)
	SettleFileProcessingMessage(ctx context.Context, consumerName, messageID string) (bool, error)
	RequeueFileProcessingMessage(ctx context.Context, consumerName string, message FileProcessingMessage, retry int, lastError string) (bool, error)
	DeadLetterFileProcessingMessage(ctx context.Context, consumerName string, message FileProcessingMessage, lastError string) (bool, error)
}

// RAGCacheRepository 封装 RAG 检索缓存能力。
type RAGCacheRepository interface {
	GetRAGCache(ctx context.Context, key string) (chunks []domainconversation.RAGChunk, ok bool)
	SetRAGCache(ctx context.Context, key string, chunks []domainconversation.RAGChunk, ttl time.Duration)
}

// GenerationStreamCacheRepository 封装对话生成流的短期恢复存储。
type GenerationStreamCacheRepository interface {
	RegisterGenerationStream(ctx context.Context, runID string, userID uint, conversationPublicID string, ttl time.Duration) error
	GetGenerationStreamOwner(ctx context.Context, runID string) (uint, bool, error)
	TouchGenerationStreamActive(ctx context.Context, runID string, userID uint, ttl time.Duration) error
	ClearGenerationStreamActive(ctx context.Context, runID string, userID uint) error
	IsGenerationStreamActive(ctx context.Context, runID string) (bool, error)
	ListActiveGenerationStreams(ctx context.Context, userID uint) ([]ActiveGenerationStream, error)
	RequestGenerationStreamCancel(ctx context.Context, runID string, ttl time.Duration) error
	IsGenerationStreamCanceled(ctx context.Context, runID string) (bool, error)
	AppendGenerationStreamEvent(ctx context.Context, runID string, input GenerationStreamAppend, maxEvents int64, ttl time.Duration) (GenerationStreamMessage, error)
	GetGenerationStreamTextSnapshot(ctx context.Context, runID string) (GenerationStreamTextSnapshot, bool, error)
	ListGenerationStreamEvents(ctx context.Context, runID string, limit int64) ([]GenerationStreamMessage, error)
	ReadGenerationStreamEvents(ctx context.Context, runID string, afterID string, block time.Duration, limit int64) ([]GenerationStreamMessage, error)
	// ResetGenerationStreamEvents clears retained events while keeping owner metadata so
	// blocked rounds cannot be replayed with withdrawn content on reconnect.
	ResetGenerationStreamEvents(ctx context.Context, runID string) error
	ExpireGenerationStream(ctx context.Context, runID string, ttl time.Duration) error
}

// ActiveGenerationStream identifies one currently leased generation owned by a user.
type ActiveGenerationStream struct {
	RunID                string
	ConversationPublicID string
}

// UserSettingCacheRepository 封装用户会话设置的共享缓存能力。
// Version 是带 TTL 的不透明令牌；Advance 必须替换当前令牌，数据按令牌隔离。
type UserSettingCacheRepository interface {
	GetUserSettingCacheVersion(ctx context.Context, userID uint, key string, ttl time.Duration) (string, error)
	AdvanceUserSettingCacheVersion(ctx context.Context, userID uint, key string, ttl time.Duration) (string, error)
	GetUserSettingCache(ctx context.Context, userID uint, key, version string) (value string, ok bool, err error)
	SetUserSettingCache(ctx context.Context, userID uint, key, version, value string, ttl time.Duration) error
}

// ConversationCacheRepository 聚合 conversation 领域缓存能力。
type ConversationCacheRepository interface {
	FileProcessingQueueRepository
	RAGCacheRepository
	GenerationStreamCacheRepository
	UserSettingCacheRepository
}
