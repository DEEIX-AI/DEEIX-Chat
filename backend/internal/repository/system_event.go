package repository

import (
	"context"
	"time"

	domainsystemevent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/systemevent"
)

// SystemEventListFilter 描述系统事件列表查询条件。
type SystemEventListFilter struct {
	Query       string
	Level       string
	Source      string
	Event       string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	Sort        string
}

// SystemEventRepository 定义系统事件持久化能力。
type SystemEventRepository interface {
	Create(ctx context.Context, item *domainsystemevent.Event) error
	List(ctx context.Context, offset int, limit int, filter SystemEventListFilter) ([]domainsystemevent.Event, int64, error)
}
