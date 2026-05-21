package repository

import (
	"context"
	"time"

	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
)

// AuditLogListFilter 描述审计日志列表查询条件。
type AuditLogListFilter struct {
	Query       string
	Resource    string
	Action      string
	ActorUserID uint
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Sort        string
}

// AuditRepository 定义审计日志持久化能力。
type AuditRepository interface {
	Create(ctx context.Context, item *domainaudit.Log) error
	List(ctx context.Context, offset int, limit int, filter AuditLogListFilter) ([]domainaudit.Log, int64, error)
}
