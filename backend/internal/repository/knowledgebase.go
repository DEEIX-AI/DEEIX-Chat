package repository

import (
	"context"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	domainknowledgebase "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/knowledgebase"
)

// KnowledgeBaseListFilter 描述知识库列表筛选条件。
type KnowledgeBaseListFilter struct {
	Query         string
	Sort          string
	PublicIDs     []string
	Scope         string
	OwnerUserID   *uint
	Enabled       *bool
	VisibleUserID *uint
}

// KnowledgeBaseFileProcessingSnapshot 描述知识库文件处理状态及聚合计数快照。
type KnowledgeBaseFileProcessingSnapshot struct {
	Files               []domainconversation.FileObject
	FileCount           int64
	ReadyFileCount      int64
	ProcessingFileCount int64
}

// KnowledgeBasePatch 描述可更新的知识库字段。
type KnowledgeBasePatch struct {
	Name               *string
	Description        *string
	Enabled            *bool
	SortOrder          *int
	UpdatedByUserIDSet bool
	UpdatedByUserID    uint
}

// KnowledgeBaseFileCleanupCandidate 描述删除知识库后可尝试清理的文件。
// 文件是否仍被其他资源引用，由文件服务在实际删除时再次原子校验。
type KnowledgeBaseFileCleanupCandidate struct {
	UserID uint
	FileID string
}

// KnowledgeBaseRepository 定义知识库持久化能力。
type KnowledgeBaseRepository interface {
	ListKnowledgeBases(ctx context.Context, filter KnowledgeBaseListFilter, offset int, limit int) ([]domainknowledgebase.KnowledgeBase, int64, error)
	GetKnowledgeBaseByPublicID(ctx context.Context, publicID string) (*domainknowledgebase.KnowledgeBase, error)
	GetKnowledgeBaseAccessByPublicID(ctx context.Context, publicID string) (*domainknowledgebase.KnowledgeBase, error)
	CreateKnowledgeBase(ctx context.Context, item *domainknowledgebase.KnowledgeBase) (*domainknowledgebase.KnowledgeBase, error)
	PatchKnowledgeBase(ctx context.Context, id uint, patch KnowledgeBasePatch) (*domainknowledgebase.KnowledgeBase, error)
	DeleteKnowledgeBase(ctx context.Context, id uint) ([]KnowledgeBaseFileCleanupCandidate, error)
	ListKnowledgeBaseFiles(ctx context.Context, knowledgeBaseID uint, offset int, limit int) ([]domainconversation.FileObject, int64, error)
	GetKnowledgeBaseFileProcessingStatuses(ctx context.Context, knowledgeBaseID uint, fileIDs []string) ([]domainconversation.FileObject, error)
	GetKnowledgeBaseFileProcessingSnapshot(ctx context.Context, knowledgeBaseID uint, fileIDs []string) (*KnowledgeBaseFileProcessingSnapshot, error)
	ListKnowledgeBaseSourceFiles(ctx context.Context, ownerUserID uint, query string, offset int, limit int) ([]domainconversation.FileObject, int64, error)
	ListAvailableKnowledgeBaseFiles(ctx context.Context, knowledgeBaseID uint, ownerUserID uint, query string, offset int, limit int) ([]domainconversation.FileObject, int64, error)
	GetKnowledgeBaseFile(ctx context.Context, knowledgeBaseID uint, fileID string) (*domainconversation.FileObject, error)
	AddKnowledgeBaseFiles(ctx context.Context, knowledgeBaseID uint, scope string, ownerUserID uint, actorUserID uint, fileIDs []string) error
	RemoveKnowledgeBaseFile(ctx context.Context, knowledgeBaseID uint, fileID string) error
	ResolveVisibleKnowledgeBaseFiles(ctx context.Context, userID uint, publicIDs []string) ([]domainknowledgebase.KnowledgeBase, []domainconversation.FileObject, error)
}
