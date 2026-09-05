package repository

import (
	"context"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
)

// ContextArtifactRepository 封装对话上下文证据的写入与查询能力。
type ContextArtifactRepository interface {
	CreateContextArtifacts(ctx context.Context, items []domainconversation.ContextArtifact) error
	GetContextArtifactByIDForUser(ctx context.Context, userID uint, artifactID uint) (*domainconversation.ContextArtifact, error)
	ListRecentContextArtifacts(ctx context.Context, filter ContextArtifactListFilter) ([]domainconversation.ContextArtifact, error)
	DeleteExpiredContextArtifacts(ctx context.Context, before time.Time, limit int) (int64, error)
}

// ContextArtifactListFilter 描述当前分支内的历史证据召回范围。
type ContextArtifactListFilter struct {
	Scope HistoricalMessageScope
	Kinds []domainconversation.ContextArtifactKind
	Limit int
}
