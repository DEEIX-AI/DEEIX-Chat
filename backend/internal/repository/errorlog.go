package repository

import (
	"context"
	"time"

	domainerrorlog "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/errorlog"
)

// ErrorLogListFilter 描述错误日志列表查询条件。RequestID 精确匹配，Query 模糊匹配路由、错误码与原因。
type ErrorLogListFilter struct {
	RequestID   string
	Query       string
	ErrorCode   string
	StatusClass string
	UserID      uint
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Sort        string
}

// ErrorLogRepository 定义错误日志持久化能力。
type ErrorLogRepository interface {
	Create(ctx context.Context, item *domainerrorlog.Log) error
	List(ctx context.Context, offset int, limit int, filter ErrorLogListFilter) ([]domainerrorlog.Log, int64, error)
}
