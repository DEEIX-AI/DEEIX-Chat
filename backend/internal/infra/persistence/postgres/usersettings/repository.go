package usersettings

import (
	"context"

	domainusersettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/usersettings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// translateError 将 gorm 底层错误统一映射为仓储语义错误。
func translateError(err error) error {
	if dberror.IsRecordNotFound(err) {
		return repository.ErrNotFound
	}
	return err
}

// Repo 封装 user_settings 数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// ListByUserID 查询指定用户的全部配置项。
func (r *Repo) ListByUserID(ctx context.Context, userID uint) ([]domainusersettings.UserSetting, error) {
	var items []model.UserSetting
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("key").Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomainUserSettings(items), nil
}

// GetByKey 查询指定用户的单个配置项，不存在时返回 nil。
func (r *Repo) GetByKey(ctx context.Context, userID uint, key string) (*model.UserSetting, error) {
	var item model.UserSetting
	err := r.db.WithContext(ctx).Where("user_id = ? AND key = ?", userID, key).First(&item).Error
	if err != nil {
		if dberror.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, translateError(err)
	}
	return &item, nil
}

// Upsert 批量写入或更新用户配置（基于 user_id+key 唯一约束）。
func (r *Repo) Upsert(ctx context.Context, items []domainusersettings.UserSetting) error {
	if len(items) == 0 {
		return nil
	}
	dbItems := toModelUserSettings(items)
	return translateError(r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).
		Create(&dbItems).Error)
}

// Delete 删除指定用户的配置项。
func (r *Repo) Delete(ctx context.Context, userID uint, key string) error {
	return translateError(r.db.WithContext(ctx).Where("user_id = ? AND key = ?", userID, key).Delete(&model.UserSetting{}).Error)
}

func toDomainUserSettings(items []model.UserSetting) []domainusersettings.UserSetting {
	results := make([]domainusersettings.UserSetting, 0, len(items))
	for _, item := range items {
		results = append(results, domainusersettings.UserSetting{
			ID:        item.ID,
			UserID:    item.UserID,
			Key:       item.Key,
			Value:     item.Value,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return results
}

func toModelUserSettings(items []domainusersettings.UserSetting) []model.UserSetting {
	results := make([]model.UserSetting, 0, len(items))
	for _, item := range items {
		results = append(results, model.UserSetting{
			ID:        item.ID,
			UserID:    item.UserID,
			Key:       item.Key,
			Value:     item.Value,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return results
}
