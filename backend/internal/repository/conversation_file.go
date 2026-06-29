package repository

import (
	"context"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
)

// FileListingRepository 封装文件列表查询能力。
type FileListingRepository interface {
	ListFileObjectsByUserWithFilter(ctx context.Context, userID uint, offset int, limit int, searchQuery string, filterKind string, sortBy string) ([]domainconversation.FileObject, int64, error)
	MarkTimedOutFileEmbeddingsFailed(ctx context.Context, userID uint, cutoff time.Time, message string) (int64, error)
}

// FileLookupRepository 封装单文件读取与维护能力。
type FileLookupRepository interface {
	GetActiveFileObjectByID(ctx context.Context, userID uint, fileID string) (*domainconversation.FileObject, error)
	RenameFileObjectByID(ctx context.Context, userID uint, fileID string, fileName string) (*domainconversation.FileObject, error)
	UpdateFileObjectRagOptOut(ctx context.Context, userID uint, fileID string, ragOptOut bool) (*domainconversation.FileObject, error)
	TouchFileObjectLastAccessedAt(ctx context.Context, userID uint, fileID string, accessedAt time.Time) error
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

// EmbeddingRepository 封装文件 embedding 状态与分片能力。
type EmbeddingRepository interface {
	VectorStoreAvailable(ctx context.Context) (bool, error)
	GetActiveFileObjectByID(ctx context.Context, userID uint, fileID string) (*domainconversation.FileObject, error)
	GetFileObjectProcessingByObjectID(ctx context.Context, fileObjID uint) (*domainconversation.FileObjectProcessing, error)
	UpdateFileObjectEmbedStatus(ctx context.Context, userID uint, fileID string, status string, embedErr string) error
	UpdateFileObjectChunkCount(ctx context.Context, fileObjID uint, chunkCount int) error
	ReplaceFileChunks(ctx context.Context, fileObjID uint, chunks []domainconversation.FileChunk, embeddings [][]float32) error
	// MarkAllEmbeddedFilesStale 将所有 embed_status=ready 的文件标记为 stale，
	// 在 Embedding 模型变更后调用，使旧向量失效并等待重建。
	// 返回被标记的文件数量。
	MarkAllEmbeddedFilesStale(ctx context.Context) (int64, error)
	// CountFilesByEmbedStatus 统计指定 embed_status 的文件数量。
	CountFilesByEmbedStatus(ctx context.Context, status string) (int64, error)
	// ListFilesForReindex 分页返回需要重建向量的文件（embed_status 为 none、stale 或 failed）。
	ListFilesForReindex(ctx context.Context, limit int, offset int) ([]domainconversation.FileObject, error)
}

// RAGRepository 封装向量检索能力。
type RAGRepository interface {
	SearchFileChunks(ctx context.Context, userID uint, fileObjIDs []uint, queryEmbedding []float32, topK int) ([]domainconversation.FileChunkSearchResult, error)
	BM25SearchFileChunks(ctx context.Context, userID uint, fileObjIDs []uint, query string, topK int) ([]domainconversation.FileChunkSearchResult, error)
}

// FileProcessingRepository 封装文件处理流水线状态能力。
type FileProcessingRepository interface {
	GetActiveFileObjectByID(ctx context.Context, userID uint, fileID string) (*domainconversation.FileObject, error)
	UpdateFileObjectProcessingState(ctx context.Context, item *domainconversation.FileObjectProcessing) error
	GetFileObjectProcessingByObjectID(ctx context.Context, fileObjID uint) (*domainconversation.FileObjectProcessing, error)
	CloneFileObjectProcessingState(ctx context.Context, sourceFileObjID uint, targetFileObjID uint, userID uint) error
	UpdateFileObjectProcessing(ctx context.Context, userID uint, fileID string, input UpdateFileObjectProcessingInput) error
}

// UpdateFileObjectProcessingInput 定义文件处理状态更新字段。
type UpdateFileObjectProcessingInput struct {
	ProcessingStatus       *string
	ProcessingReady        *bool
	ProcessingErrorCode    *string
	ProcessingErrorMessage *string
	ExtractStatus          *string
	PageCount              *int
	ExtractorVersion       *string
	ExtractedAt            **time.Time
}

// IsZero 判断是否没有任何文件处理状态更新字段。
func (input UpdateFileObjectProcessingInput) IsZero() bool {
	return input.ProcessingStatus == nil &&
		input.ProcessingReady == nil &&
		input.ProcessingErrorCode == nil &&
		input.ProcessingErrorMessage == nil &&
		input.ExtractStatus == nil &&
		input.PageCount == nil &&
		input.ExtractorVersion == nil &&
		input.ExtractedAt == nil
}

// ConversationSettingsRepository 封装会话域设置读取能力。
type ConversationSettingsRepository interface {
	GetUserSettingValue(ctx context.Context, userID uint, key string) (string, error)
}
