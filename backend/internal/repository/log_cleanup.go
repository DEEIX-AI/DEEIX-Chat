package repository

import (
	"context"
	"time"
)

const (
	LogCleanupTypeAudit        = "audit"
	LogCleanupTypeAuth         = "auth"
	LogCleanupTypeUsage        = "usage"
	LogCleanupTypeOrders       = "orders"
	LogCleanupTypeConversation = "conversation"
	LogCleanupTypeSystem       = "system"
)

// LogCleanupRepository 定义管理员日志物理清理能力。
type LogCleanupRepository interface {
	DeleteBefore(ctx context.Context, logType string, before time.Time) (int64, error)
	DeleteConversationRuns(ctx context.Context, runIDs []string) (int64, error)
}
