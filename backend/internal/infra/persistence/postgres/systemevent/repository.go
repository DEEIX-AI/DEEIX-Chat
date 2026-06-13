package systemevent

import (
	"context"
	"strings"

	domainsystemevent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/systemevent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// Repo 封装系统事件数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func translateError(err error) error {
	if dberror.IsRecordNotFound(err) {
		return repository.ErrNotFound
	}
	return err
}

// Create 创建系统事件。
func (r *Repo) Create(ctx context.Context, item *domainsystemevent.Event) error {
	row := toModelSystemEvent(item)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return translateError(err)
	}
	item.ID = row.ID
	item.CreatedAt = row.CreatedAt
	item.UpdatedAt = row.UpdatedAt
	return nil
}

// List 分页查询系统事件。
func (r *Repo) List(ctx context.Context, offset int, limit int, filter repository.SystemEventListFilter) ([]domainsystemevent.Event, int64, error) {
	rows := make([]model.SystemEvent, 0)
	var total int64

	query := r.db.WithContext(ctx).Model(&model.SystemEvent{})
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"request_id LIKE ? OR trace_id LIKE ? OR source LIKE ? OR event LIKE ? OR resource LIKE ? OR resource_id LIKE ? OR message LIKE ? OR detail_json LIKE ?",
			like, like, like, like, like, like, like, like,
		)
	}
	if level := strings.TrimSpace(filter.Level); level != "" {
		query = query.Where("level = ?", level)
	}
	if source := strings.TrimSpace(filter.Source); source != "" {
		query = query.Where("source = ?", source)
	}
	if event := strings.TrimSpace(filter.Event); event != "" {
		query = query.Where("event = ?", event)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}

	order := "created_at DESC, id DESC"
	switch strings.TrimSpace(filter.Sort) {
	case "created_asc":
		order = "created_at ASC, id ASC"
	case "id_desc":
		order = "id DESC"
	case "id_asc":
		order = "id ASC"
	}
	if err := query.Order(order).Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, translateError(err)
	}
	return toDomainSystemEvents(rows), total, nil
}

func toModelSystemEvent(item *domainsystemevent.Event) *model.SystemEvent {
	if item == nil {
		return &model.SystemEvent{}
	}
	return &model.SystemEvent{
		BaseModel: model.BaseModel{
			ID:        item.ID,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		},
		RequestID:  item.RequestID,
		TraceID:    item.TraceID,
		Level:      item.Level,
		Source:     item.Source,
		Event:      item.Event,
		Resource:   item.Resource,
		ResourceID: item.ResourceID,
		Message:    item.Message,
		DetailJSON: item.DetailJSON,
	}
}

func toDomainSystemEvents(rows []model.SystemEvent) []domainsystemevent.Event {
	results := make([]domainsystemevent.Event, 0, len(rows))
	for _, row := range rows {
		results = append(results, domainsystemevent.Event{
			ID:         row.ID,
			RequestID:  row.RequestID,
			TraceID:    row.TraceID,
			Level:      row.Level,
			Source:     row.Source,
			Event:      row.Event,
			Resource:   row.Resource,
			ResourceID: row.ResourceID,
			Message:    row.Message,
			DetailJSON: row.DetailJSON,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
		})
	}
	return results
}
