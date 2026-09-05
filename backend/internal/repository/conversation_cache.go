package repository

import (
	"context"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

const FileProcessingKindEmbedding = "embedding"

type FileProcessingQueue string

const (
	FileProcessingQueueDefault   FileProcessingQueue = "file_processing"
	FileProcessingQueueEmbedding FileProcessingQueue = "file_embedding"
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
	Kind      string
	// Queue 标记消息实际所属队列；旧消息即使携带 embedding kind，也能在原队列完成续租和确认。
	Queue FileProcessingQueue
	// EmbeddingSignature 与 EmbeddingHost 将显式向量化任务固定到接收任务时的运行时配置。
	EmbeddingSignature string
	EmbeddingHost      string
}

// GenerationStreamMessage 是生成流中的一条可恢复事件。
type GenerationStreamMessage struct {
	ID          string
	Seq         int64
	PayloadJSON string
}

// GenerationStreamAppend 是一次原子追加所需的数据。
// TextDelta 仅在可见文本 delta 事件中设置，用于同步维护完整恢复快照。
// UpstreamThink 仅在上游思考事件中设置，用于维护当前思考轮次的完整恢复快照。
type GenerationStreamAppend struct {
	PayloadJSON   string
	TextDelta     string
	UpstreamThink *GenerationStreamUpstreamThinkAppend
}

// GenerationStreamUpstreamThinkAppend 描述上游思考快照的原子更新。
type GenerationStreamUpstreamThinkAppend struct {
	RoundID         string
	Delta           string
	ContentMarkdown string
	Replace         bool
	MetadataJSON    string
}

// GenerationStreamTextSnapshot 是生成期间可恢复的完整可见文本及其事件序号。
type GenerationStreamTextSnapshot struct {
	Seq     int64
	Content string
}

// GenerationStreamUpstreamThinkSnapshot 是当前思考轮次的完整内容及其最后事件序号。
type GenerationStreamUpstreamThinkSnapshot struct {
	Seq             int64
	RoundID         string
	ContentMarkdown string
	MetadataJSON    string
}

// GenerationStreamLease 标识一次受执行权隔离保护的生成运行。
type GenerationStreamLease struct {
	RunID                string
	ExecutionID          string
	UserID               uint
	ConversationPublicID string
}

// FileProcessingQueueRepository 封装文件处理队列缓存能力。
type FileProcessingQueueRepository interface {
	InitFileProcessingStream(ctx context.Context) error
	EnqueueFileProcessing(ctx context.Context, userID uint, fileID string, retry int, lastError string) error
	EnqueueFileEmbedding(ctx context.Context, userID uint, fileID string, embeddingSignature string, embeddingHost string) error
	ClaimTimedOutFileProcessingMessages(ctx context.Context, consumerName string) ([]FileProcessingMessage, error)
	ClaimTimedOutFileEmbeddingMessages(ctx context.Context, consumerName string) ([]FileProcessingMessage, error)
	ReadFileProcessingMessages(ctx context.Context, consumerName string) ([]FileProcessingMessage, error)
	ReadFileEmbeddingMessages(ctx context.Context, consumerName string) ([]FileProcessingMessage, error)
	RenewFileProcessingMessageLease(ctx context.Context, consumerName string, message FileProcessingMessage) (bool, error)
	SettleFileProcessingMessage(ctx context.Context, consumerName string, message FileProcessingMessage) (bool, error)
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
	// ClaimGenerationStream 原子获取运行执行权；同一租约可幂等重入，
	// 逻辑运行归属保留期间拒绝其他执行接管。
	ClaimGenerationStream(ctx context.Context, lease GenerationStreamLease, leaseTTL time.Duration, ownershipTTL time.Duration) (bool, error)
	GetGenerationStreamOwner(ctx context.Context, runID string) (uint, bool, error)
	RenewGenerationStreamLease(ctx context.Context, lease GenerationStreamLease, leaseTTL time.Duration, ownershipTTL time.Duration) (bool, error)
	CompleteGenerationStream(ctx context.Context, lease GenerationStreamLease, retention time.Duration) (bool, error)
	AbandonGenerationStream(ctx context.Context, lease GenerationStreamLease) (bool, error)
	IsGenerationStreamActive(ctx context.Context, runID string) (bool, error)
	ListActiveGenerationStreams(ctx context.Context, userID uint) ([]ActiveGenerationStream, error)
	RequestGenerationStreamCancel(ctx context.Context, runID string, userID uint, ttl time.Duration) (bool, error)
	IsGenerationStreamCanceled(ctx context.Context, runID string) (bool, error)
	AppendGenerationStreamEvent(ctx context.Context, lease GenerationStreamLease, input GenerationStreamAppend, maxEvents int64, ttl time.Duration) (GenerationStreamMessage, bool, error)
	AppendActiveGenerationEvent(ctx context.Context, input GenerationStreamAppend, maxEvents int64, ttl time.Duration) (GenerationStreamMessage, error)
	GetGenerationStreamTextSnapshot(ctx context.Context, runID string) (GenerationStreamTextSnapshot, bool, error)
	GetGenerationStreamUpstreamThinkSnapshot(ctx context.Context, runID string) (GenerationStreamUpstreamThinkSnapshot, bool, error)
	ListGenerationStreamEvents(ctx context.Context, runID string, limit int64) ([]GenerationStreamMessage, error)
	ReadGenerationStreamEvents(ctx context.Context, runID string, afterID string, block time.Duration, limit int64) ([]GenerationStreamMessage, error)
	// ResetGenerationStreamEvents clears retained events while keeping owner metadata so
	// blocked rounds cannot be replayed with withdrawn content on reconnect.
	ResetGenerationStreamEvents(ctx context.Context, lease GenerationStreamLease) (bool, error)
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

// ConversationCacheRepository 描述同一个缓存后端（Redis 或进程内存）对外提供的 conversation 领域全部缓存能力，
// 由 infra 构造并在组合根按需拆分注入：处理流水线、RAG 与会话服务各自只依赖其中的子接口。
type ConversationCacheRepository interface {
	FileProcessingQueueRepository
	RAGCacheRepository
	GenerationStreamCacheRepository
	UserSettingCacheRepository
}
