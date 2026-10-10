package repository

import (
	"context"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
)

// ListFileObjectsInput 定义用户文件列表的分页、筛选与排序条件。
type ListFileObjectsInput struct {
	UserID      uint
	Offset      int
	Limit       int
	SearchQuery string
	FilterKind  string
	SortBy      string
}

// FileListingRepository 封装文件列表查询能力。
type FileListingRepository interface {
	ListFileObjectsByUserWithFilter(ctx context.Context, input ListFileObjectsInput) ([]domainconversation.FileObject, int64, error)
}

// FileLookupRepository 封装单文件读取与维护能力。
type FileLookupRepository interface {
	GetActiveFileObjectByID(ctx context.Context, userID uint, fileID string) (*domainconversation.FileObject, error)
	RenameFileObjectByID(ctx context.Context, userID uint, fileID string, fileName string) (*domainconversation.FileObject, error)
	UpdateFileObjectRAGOptOut(ctx context.Context, userID uint, fileID string, ragOptOut bool) (*domainconversation.FileObject, error)
	TouchFileObjectLastAccessedAt(ctx context.Context, userID uint, fileID string, accessedAt time.Time) error
}

// ModerationFileRepository 封装内容审核清理所需的文件操作能力。
// 与 FileLookupRepository 隔离，避免上传模块及其测试 mock 依赖审核专用方法。
type ModerationFileRepository interface {
	// RevokeGeneratedFileForModeration 将生成文件标记为不可访问，并解除用户归属。
	RevokeGeneratedFileForModeration(ctx context.Context, fileID string) error
	// DeleteGeneratedFileArtifactsForModeration 将附件标记为已删除（保留 storage_path 以便重试）。
	DeleteGeneratedFileArtifactsForModeration(ctx context.Context, fileID string) error
	// ClearGeneratedFileStoragePath 在物理删除成功后清空 storage_path。
	ClearGeneratedFileStoragePath(ctx context.Context, fileID string) error
	// GetFileObjectByFileIDAnyStatus 加载文件，不限状态（用于审核清理）。
	GetFileObjectByFileIDAnyStatus(ctx context.Context, fileID string) (*domainconversation.FileObject, error)
}

// FileBatchRepository 封装批量读取文件能力。
type FileBatchRepository interface {
	GetActiveFileObjectsByIDs(ctx context.Context, userID uint, fileIDs []string) ([]domainconversation.FileObject, error)
}

// DeleteFileObjectOptions 定义文件对象删除的仓储约束。
type DeleteFileObjectOptions struct {
	RequireUnreferenced bool
}

// UploadRepository 封装上传、去重和配额能力。
type UploadRepository interface {
	FileListingRepository
	FileLookupRepository
	GetUserByID(ctx context.Context, userID uint) (*domainuser.User, error)
	GetLatestActiveFileObjectBySHA(ctx context.Context, userID uint, sha256 string, sizeBytes int64) (*domainconversation.FileObject, error)
	CreateFileObjectAndConsumeQuota(ctx context.Context, item *domainconversation.FileObject, quotaLimit int64) (*domainconversation.StorageQuota, error)
	DeleteFileObjectAndReleaseQuota(ctx context.Context, userID uint, fileID string, quotaLimit int64, options DeleteFileObjectOptions) (*domainconversation.FileObject, *domainconversation.StorageQuota, bool, error)
	GetOrInitUserStorageQuota(ctx context.Context, userID uint, quotaLimit int64) (*domainconversation.StorageQuota, error)
}

// FileEmbeddingArtifactsRepository 封装 embedding 工件克隆能力。
type FileEmbeddingArtifactsRepository interface {
	CloneFileEmbeddingArtifacts(ctx context.Context, source *domainconversation.FileObject, target *domainconversation.FileObject) error
}

// EmbeddableFileScope 描述当前向量化配置下可以建立索引的文件范围，由 application 按运行时配置计算。
// 仓储只把它翻译成查询条件，用于区分“尚未索引”与“类型不支持”的 none 文件。
type EmbeddableFileScope struct {
	// Categories 是可向量化的 file_category 列表。
	Categories []string
	// ImageMIMETypes 非空时，image 类别只有这些 MIME 可向量化；为空时 image 类别整体按 Categories 判断。
	ImageMIMETypes []string
}

// FileEmbeddingStateCount 是按 embed_status 聚合的文件数量。
type FileEmbeddingStateCount struct {
	Status string
	// Embeddable 表示这些文件是否落在 EmbeddableFileScope 内。
	Embeddable bool
	// ProcessingFailed 表示文件处理流水线失败，没有可用于向量化的文本。
	ProcessingFailed bool
	// Stalled 表示 queued/processing 文件的最近更新时间早于停滞阈值。
	Stalled bool
	Count   int64
}

// FileEmbedStatusMatch 描述一个 embed_status 匹配条件。
type FileEmbedStatusMatch struct {
	Status string
	// Embeddable 非空时，额外要求文件是否落在 EmbeddableFileScope 内。
	Embeddable *bool
	// ProcessingFailed 非空时，额外要求文件处理流水线是否失败。
	ProcessingFailed *bool
}

// ListFileEmbeddingTasksInput 定义管理员查看全平台文件向量化任务的筛选与分页条件。
type ListFileEmbeddingTasksInput struct {
	// Matches 之间为 OR 关系，不能为空。
	Matches []FileEmbedStatusMatch
	Scope   EmbeddableFileScope
	// Query 按文件 ID 或文件名模糊匹配。
	Query  string
	Offset int
	Limit  int
}

// ListFilesForReindexInput 定义批量重建扫描的分页与范围。
type ListFilesForReindexInput struct {
	Limit   int
	AfterID uint
	// StalledBefore 非零时纳入最近更新时间早于该时刻的 queued 文件，回收丢失队列消息的任务。
	StalledBefore time.Time
}

// EmbeddingRepository 封装文件 embedding 状态与分片能力。
type EmbeddingRepository interface {
	VectorStoreAvailable(ctx context.Context) (bool, error)
	GetActiveFileObjectByID(ctx context.Context, userID uint, fileID string) (*domainconversation.FileObject, error)
	GetActiveFileObjectsByIDs(ctx context.Context, userID uint, fileIDs []string) ([]domainconversation.FileObject, error)
	// GetActiveFileObjectsByFileIDs 跨用户按文件 ID 批量读取，仅供管理员任务管理使用。
	GetActiveFileObjectsByFileIDs(ctx context.Context, fileIDs []string) ([]domainconversation.FileObject, error)
	GetFileObjectProcessingByObjectID(ctx context.Context, fileObjID uint) (*domainconversation.FileObjectProcessing, error)
	// QueueFileEmbedding 原子登记待执行任务；同一签名已排队、执行或完成时不重复登记，
	// 但最近更新时间早于 stalledBefore 的 queued/processing 任务视为停滞，允许重新登记。
	QueueFileEmbedding(ctx context.Context, userID uint, fileID string, embeddingSignature string, stalledBefore time.Time) (bool, error)
	ClaimFileEmbedding(ctx context.Context, userID uint, fileID string, embeddingSignature string) (bool, error)
	UpdateFileObjectEmbedStatus(ctx context.Context, userID uint, fileID string, embeddingSignature string, status string, embedErr string) (bool, error)
	UpdateFileObjectChunkCount(ctx context.Context, fileObjID uint, embeddingSignature string, chunkCount int) (bool, error)
	ReplaceFileChunks(ctx context.Context, fileObjID uint, embeddingSignature string, chunks []domainconversation.FileChunk, embeddings [][]float32) (bool, error)
	// MarkEmbeddedFilesStale 将缺少当前向量空间签名分片的 queued/processing/ready 文件标记为 stale。
	// 在 Embedding 配置变更及服务启动时调用，使旧向量失效并等待重建。
	// 返回被标记的文件数量。
	MarkEmbeddedFilesStale(ctx context.Context, activeSignature string) (int64, error)
	// MarkStalledFileEmbeddingsFailed 将全平台最近更新时间早于 cutoff 的 processing 文件标记为失败。
	MarkStalledFileEmbeddingsFailed(ctx context.Context, cutoff time.Time, message string) (int64, error)
	// CountFileEmbeddingStates 按 embed_status、是否可向量化与是否停滞聚合全平台文件数量。
	CountFileEmbeddingStates(ctx context.Context, scope EmbeddableFileScope, stalledBefore time.Time) ([]FileEmbeddingStateCount, error)
	// ListFileEmbeddingTasks 分页返回全平台匹配的文件，按最近更新时间倒序。
	ListFileEmbeddingTasks(ctx context.Context, input ListFileEmbeddingTasksInput) ([]domainconversation.FileObject, int64, error)
	// ListFilesForReindex 分页返回需要重建向量的文件：已有可用文本且 embed_status 为 none、stale、failed
	// 或停滞的 queued；处理未完成或失败的文件不在其中，避免向量化服务重复提取。
	ListFilesForReindex(ctx context.Context, input ListFilesForReindexInput) ([]domainconversation.FileObject, error)
	// ResetFileForReprocessing 把没有可用文本的文件（处理失败、无文本、排队早于 stalledBefore 仍未开始）原子重置为待处理，
	// 并在文件行上登记指定向量空间的向量化请求，处理流水线提取完成后据此建立索引；处理仍在进行时返回 false。
	ResetFileForReprocessing(ctx context.Context, userID uint, fileID string, embeddingSignature string, stalledBefore time.Time) (bool, error)
	// ListFilesForReprocessing 按 ID 升序分页返回全平台处理失败或排队停滞的文件；includeEmpty 为 true 时同时返回无文本文件。
	ListFilesForReprocessing(ctx context.Context, input ListFilesForReprocessingInput) ([]domainconversation.FileObject, error)
}

// RAGRepository 封装向量检索能力。
type RAGRepository interface {
	// fileObjIDs 由 application 层按本次会话选择解析；仓储层仍按 userID 二次校验，
	// 仅允许检索用户自己的文件或当前启用的内置知识库文件。
	SearchFileChunks(ctx context.Context, userID uint, fileObjIDs []uint, queryEmbedding []float32, embeddingSignature string, topK int) ([]domainconversation.FileChunkSearchResult, error)
	BM25SearchFileChunks(ctx context.Context, userID uint, fileObjIDs []uint, query string, topK int) ([]domainconversation.FileChunkSearchResult, error)
}

// FileProcessingRepository 封装文件处理流水线状态能力。
type FileProcessingRepository interface {
	GetActiveFileObjectByID(ctx context.Context, userID uint, fileID string) (*domainconversation.FileObject, error)
	UpdateFileObjectProcessingState(ctx context.Context, item *domainconversation.FileObjectProcessing) error
	UpdateClaimedFileObjectProcessingState(ctx context.Context, item *domainconversation.FileObjectProcessing, attemptID string) (bool, error)
	GetFileObjectProcessingByObjectID(ctx context.Context, fileObjID uint) (*domainconversation.FileObjectProcessing, error)
	CloneFileObjectProcessingState(ctx context.Context, sourceFileObjID uint, targetFileObjID uint, userID uint) error
	TryClaimFileObjectProcessing(ctx context.Context, userID uint, fileID string, allowRecovery bool, extractorVersion string, attemptID string) (bool, error)
	ResetFileObjectProcessingForRetry(ctx context.Context, userID uint, fileID string, attemptID string) (bool, error)
}

// ListFilesForReprocessingInput 定义需要重新走处理流水线的文件扫描范围。
type ListFilesForReprocessingInput struct {
	Limit   int
	AfterID uint
	// IncludeEmpty 为 true 时同时纳入提取完成但无文本的文件，供更换 OCR 引擎后重试。
	IncludeEmpty bool
	// StalledBefore 是排队停滞的判定时刻，早于该时刻仍未开始处理的文件视为队列消息丢失。
	StalledBefore time.Time
}

// FileProcessingStatusRepository 封装单个与批量文件处理状态读取能力。
type FileProcessingStatusRepository interface {
	FileProcessingRepository
	GetActiveFileProcessingStatusesByIDs(ctx context.Context, userID uint, fileIDs []string) ([]domainconversation.FileObject, error)
}

// ConversationSettingsRepository 封装会话域设置读取能力。
type ConversationSettingsRepository interface {
	GetUserSettingValue(ctx context.Context, userID uint, key string) (string, error)
	GetUserSettingValues(ctx context.Context, userID uint, keys []string) (map[string]string, error)
}
