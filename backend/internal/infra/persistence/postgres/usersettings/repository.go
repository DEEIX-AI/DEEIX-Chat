package usersettings

import (
	"context"

	domainusersettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/usersettings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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
	var items []models.UserSetting
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("key").Find(&items).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return toDomainUserSettings(items), nil
}

// GetByKey 查询指定用户的单个配置项，不存在时返回 nil。
func (r *Repo) GetByKey(ctx context.Context, userID uint, key string) (*models.UserSetting, error) {
	var item models.UserSetting
	err := r.db.WithContext(ctx).Where("user_id = ? AND key = ?", userID, key).First(&item).Error
	if err != nil {
		if dberror.IsRecordNotFound(err) {
			return nil, nil
		}
		return nil, dberror.Translate(err)
	}
	return &item, nil
}

// Upsert 批量写入或更新用户配置（基于 user_id+key 唯一约束）。
func (r *Repo) Upsert(ctx context.Context, items []domainusersettings.UserSetting) error {
	if len(items) == 0 {
		return nil
	}
	dbItems := toModelUserSettings(items)
	return dberror.Translate(r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
		}).
		Create(&dbItems).Error)
}

// Delete 删除指定用户的配置项。
func (r *Repo) Delete(ctx context.Context, userID uint, key string) error {
	return dberror.Translate(r.db.WithContext(ctx).Where("user_id = ? AND key = ?", userID, key).Delete(&models.UserSetting{}).Error)
}

func toDomainUserSettings(items []models.UserSetting) []domainusersettings.UserSetting {
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

func toModelUserSettings(items []domainusersettings.UserSetting) []models.UserSetting {
	results := make([]models.UserSetting, 0, len(items))
	for _, item := range items {
		results = append(results, models.UserSetting{
			ID:        item.ID,
			UserID:    item.UserID,
			Key:       item.Key,
			Value:     item.Value,
			UpdatedAt: item.UpdatedAt,
		})
	}
	return results
}
