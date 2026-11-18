package repository

import (
	"context"

	domaindiscussion "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/discussion"
)

// DiscussionSnapshotRepository 原子维护会话级讨论配置的 active/pending 两阶段状态。
type DiscussionSnapshotRepository interface {
	GetDiscussionSnapshot(ctx context.Context, conversationID uint, templatePublicID string) (*domaindiscussion.ConversationSnapshot, error)
	RefreshDiscussionSnapshot(ctx context.Context, conversationID uint, templatePublicID string, pending domaindiscussion.Snapshot, expectedPendingVersion int64) (*domaindiscussion.ConversationSnapshot, error)
	UpdateDiscussionSnapshotPending(ctx context.Context, conversationID uint, templatePublicID string, pending domaindiscussion.Snapshot, expectedPendingVersion int64) (*domaindiscussion.ConversationSnapshot, error)
	RollbackDiscussionSnapshot(ctx context.Context, conversationID uint, templatePublicID string, expectedPendingVersion int64) (*domaindiscussion.ConversationSnapshot, error)
	CommitDiscussionSnapshot(ctx context.Context, conversationID uint, templatePublicID string, snapshot domaindiscussion.Snapshot, usePending bool, expectedPendingVersion int64) (*domaindiscussion.ConversationSnapshot, bool, error)
}
