package audit

import (
	"context"
	"strings"

	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// translateError 将 gorm 底层错误统一映射为仓储语义错误。
func translateError(err error) error {
	if dberror.IsRecordNotFound(err) {
		return repository.ErrNotFound
	}
	return err
}

// Repo 封装审计数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// Create 创建审计日志。
func (r *Repo) Create(ctx context.Context, item *domainaudit.Log) error {
	dbItem := toModelAuditLog(item)
	if err := r.db.WithContext(ctx).Create(dbItem).Error; err != nil {
		return translateError(err)
	}
	item.ID = dbItem.ID
	item.CreatedAt = dbItem.CreatedAt
	item.UpdatedAt = dbItem.UpdatedAt
	return nil
}

// List 分页查询审计日志。
func (r *Repo) List(ctx context.Context, offset int, limit int, filter repository.AuditLogListFilter) ([]domainaudit.Log, int64, error) {
	items := make([]model.AuditLog, 0)
	var total int64

	query := r.db.WithContext(ctx).Model(&model.AuditLog{})
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"request_id LIKE ? OR action LIKE ? OR resource LIKE ? OR resource_id LIKE ? OR ip LIKE ? OR user_agent LIKE ? OR detail_json LIKE ?",
			like,
			like,
			like,
			like,
			like,
			like,
			like,
		)
	}
	if resource := strings.TrimSpace(filter.Resource); resource != "" {
		query = query.Where("resource = ?", resource)
	}
	if action := strings.TrimSpace(filter.Action); action != "" {
		query = query.Where("action = ?", action)
	}
	if filter.ActorUserID > 0 {
		query = query.Where("actor_user_id = ?", filter.ActorUserID)
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
	order := "id DESC"
	switch strings.TrimSpace(filter.Sort) {
	case "id_asc":
		order = "id ASC"
	case "created_asc":
		order = "created_at ASC, id ASC"
	case "created_desc":
		order = "created_at DESC, id DESC"
	}
	if err := query.
		Order(order).
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}
	return toDomainAuditLogs(items), total, nil
}

func toModelAuditLog(item *domainaudit.Log) *model.AuditLog {
	if item == nil {
		return &model.AuditLog{}
	}
	return &model.AuditLog{
		BaseModel: model.BaseModel{
			ID:        item.ID,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		},
		RequestID:   item.RequestID,
		ActorUserID: item.ActorUserID,
		Action:      item.Action,
		Resource:    item.Resource,
		ResourceID:  item.ResourceID,
		IP:          item.IP,
		UserAgent:   item.UserAgent,
		DetailJSON:  item.DetailJSON,
	}
}

func toDomainAuditLogs(items []model.AuditLog) []domainaudit.Log {
	results := make([]domainaudit.Log, 0, len(items))
	for _, item := range items {
		results = append(results, domainaudit.Log{
			ID:          item.ID,
			RequestID:   item.RequestID,
			ActorUserID: item.ActorUserID,
			Action:      item.Action,
			Resource:    item.Resource,
			ResourceID:  item.ResourceID,
			IP:          item.IP,
			UserAgent:   item.UserAgent,
			DetailJSON:  item.DetailJSON,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return results
}
