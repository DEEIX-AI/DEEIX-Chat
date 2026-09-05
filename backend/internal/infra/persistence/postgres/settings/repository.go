package settings

import (
	"context"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo 封装 system_settings 数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// ListAll 查询全部动态配置。
func (r *Repo) ListAll(ctx context.Context) ([]domainsettings.SystemSetting, error) {
	var items []model.SystemSetting
	if err := r.db.WithContext(ctx).Order("namespace, key").Find(&items).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return toDomainSystemSettings(items), nil
}

// ListByNamespace 按 namespace 查询配置。
func (r *Repo) ListByNamespace(ctx context.Context, namespace string) ([]domainsettings.SystemSetting, error) {
	var items []model.SystemSetting
	if err := r.db.WithContext(ctx).Where("namespace = ?", namespace).Order("key").Find(&items).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return toDomainSystemSettings(items), nil
}

// Upsert 批量写入或更新配置（基于 namespace+key 唯一约束）。
func (r *Repo) Upsert(ctx context.Context, items []domainsettings.SystemSetting) error {
	if len(items) == 0 {
		return nil
	}
	dbItems := toModelSystemSettings(items)
	return dberror.Translate(r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "namespace"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).
		Create(&dbItems).Error)
}

// UpsertWithDescription 批量写入含描述的配置（种子用）。
func (r *Repo) UpsertWithDescription(ctx context.Context, items []domainsettings.SystemSetting) error {
	if len(items) == 0 {
		return nil
	}
	dbItems := toModelSystemSettings(items)
	return dberror.Translate(r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "namespace"}, {Name: "key"}},
			DoNothing: true,
		}).
		Create(&dbItems).Error)
}

// Delete 删除指定配置项。
func (r *Repo) Delete(ctx context.Context, namespace, key string) error {
	return dberror.Translate(r.db.WithContext(ctx).
		Where("namespace = ? AND key = ?", namespace, key).
		Delete(&model.SystemSetting{}).Error)
}

func toDomainSystemSettings(items []model.SystemSetting) []domainsettings.SystemSetting {
	results := make([]domainsettings.SystemSetting, 0, len(items))
	for _, item := range items {
		results = append(results, domainsettings.SystemSetting{
			ID:          item.ID,
			Namespace:   item.Namespace,
			Key:         item.Key,
			Value:       item.Value,
			ValueType:   item.ValueType,
			Description: item.Description,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return results
}

func toModelSystemSettings(items []domainsettings.SystemSetting) []model.SystemSetting {
	results := make([]model.SystemSetting, 0, len(items))
	for _, item := range items {
		results = append(results, model.SystemSetting{
			ID:          item.ID,
			Namespace:   item.Namespace,
			Key:         item.Key,
			Value:       item.Value,
			ValueType:   item.ValueType,
			Description: item.Description,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return results
}
