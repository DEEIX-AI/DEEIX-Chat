package channel

import (
	"context"
	"errors"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateModelVendor 创建技术厂商目录项。
func (r *Repo) CreateModelVendor(ctx context.Context, item *domainchannel.ModelVendor) error {
	if item == nil {
		return repository.ErrInvalidInput
	}
	entity := toModelVendorModel(item)
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if entity.SortOrder == 0 {
			var maxSortOrder int
			if err := tx.Model(&models.LLMModelVendor{}).
				Select("COALESCE(MAX(sort_order), 0)").
				Scan(&maxSortOrder).Error; err != nil {
				return translateError(err)
			}
			entity.SortOrder = maxSortOrder + 100
		}
		return translateError(tx.Create(&entity).Error)
	}); err != nil {
		return err
	}
	*item = toModelVendorDomain(entity)
	return nil
}

// UpdateModelVendor 更新技术厂商的展示名称与图标，稳定 key 不参与修改。
func (r *Repo) UpdateModelVendor(ctx context.Context, key string, input repository.UpdateModelVendorInput) error {
	updates := make(map[string]interface{})
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Icon != nil {
		updates["icon"] = *input.Icon
	}
	if len(updates) == 0 {
		return nil
	}
	result := r.db.WithContext(ctx).
		Model(&models.LLMModelVendor{}).
		Where("key = ?", key).
		Updates(updates)
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// GetModelVendorByKey 按稳定 key 获取技术厂商目录项。
func (r *Repo) GetModelVendorByKey(ctx context.Context, key string) (*domainchannel.ModelVendor, error) {
	var item models.LLMModelVendor
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toModelVendorDomain(item)
	return &result, nil
}

// ListModelVendors 分页查询技术厂商目录。
func (r *Repo) ListModelVendors(ctx context.Context, input repository.ListModelVendorsInput) ([]domainchannel.ModelVendor, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.LLMModelVendor{})
	if keyword := strings.ToLower(strings.TrimSpace(input.Query)); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("LOWER(key) LIKE ? OR LOWER(name) LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	entities := make([]models.LLMModelVendor, 0)
	if err := query.Order("sort_order ASC, id ASC").Offset(input.Offset).Limit(input.Limit).Find(&entities).Error; err != nil {
		return nil, 0, translateError(err)
	}
	items := make([]domainchannel.ModelVendor, 0, len(entities))
	for _, entity := range entities {
		items = append(items, toModelVendorDomain(entity))
	}
	return items, total, nil
}

// CreateModelDisplayGroup 创建自定义模型展示分组。
func (r *Repo) CreateModelDisplayGroup(ctx context.Context, item *domainchannel.ModelDisplayGroup, modelIDs []uint) error {
	if item == nil {
		return repository.ErrInvalidInput
	}
	entity := toModelDisplayGroupModel(item)
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if entity.SortOrder == 0 {
			var maxSortOrder int
			if err := tx.Model(&models.LLMModelDisplayGroup{}).
				Select("COALESCE(MAX(sort_order), 0)").
				Scan(&maxSortOrder).Error; err != nil {
				return translateError(err)
			}
			entity.SortOrder = maxSortOrder + 100
		}
		if err := tx.Create(&entity).Error; err != nil {
			return translateError(err)
		}
		return replaceModelDisplayGroupMembers(tx, entity.ID, modelIDs)
	}); err != nil {
		return err
	}
	*item = toModelDisplayGroupDomain(entity)
	return nil
}

// UpdateModelDisplayGroup 更新自定义模型展示分组。
func (r *Repo) UpdateModelDisplayGroup(ctx context.Context, groupID uint, input repository.UpdateModelDisplayGroupInput) error {
	updates := make(map[string]interface{})
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Icon != nil {
		updates["icon"] = *input.Icon
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item models.LLMModelDisplayGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", groupID).First(&item).Error; err != nil {
			return translateError(err)
		}
		if len(updates) > 0 {
			if err := tx.Model(&models.LLMModelDisplayGroup{}).
				Where("id = ?", groupID).
				Updates(updates).Error; err != nil {
				return translateError(err)
			}
		}
		if input.ModelIDs != nil {
			return replaceModelDisplayGroupMembers(tx, groupID, *input.ModelIDs)
		}
		return nil
	})
}

// replaceModelDisplayGroupMembers 在当前事务中完整替换分组成员；选中的模型会自动从原分组迁入。
func replaceModelDisplayGroupMembers(tx *gorm.DB, groupID uint, modelIDs []uint) error {
	if len(modelIDs) > 0 {
		var count int64
		if err := tx.Model(&models.LLMPlatformModel{}).Where("id IN ?", modelIDs).Count(&count).Error; err != nil {
			return translateError(err)
		}
		if count != int64(len(modelIDs)) {
			return repository.ErrInvalidInput
		}
	}

	clearQuery := tx.Model(&models.LLMPlatformModel{}).Where("display_group_id = ?", groupID)
	if len(modelIDs) > 0 {
		clearQuery = clearQuery.Where("id NOT IN ?", modelIDs)
	}
	if err := clearQuery.Update("display_group_id", nil).Error; err != nil {
		return translateError(err)
	}
	if len(modelIDs) == 0 {
		return nil
	}
	return translateError(tx.Model(&models.LLMPlatformModel{}).
		Where("id IN ?", modelIDs).
		Update("display_group_id", groupID).Error)
}

// SetModelsDisplayGroup 在单个事务中将指定模型归入展示分组；groupID 为 0 时清除自定义分组。
func (r *Repo) SetModelsDisplayGroup(ctx context.Context, modelIDs []uint, groupID uint) error {
	if len(modelIDs) == 0 {
		return repository.ErrInvalidInput
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if groupID > 0 {
			var group models.LLMModelDisplayGroup
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", groupID).First(&group).Error; err != nil {
				return translateError(err)
			}
		}

		var count int64
		if err := tx.Model(&models.LLMPlatformModel{}).Where("id IN ?", modelIDs).Count(&count).Error; err != nil {
			return translateError(err)
		}
		if count != int64(len(modelIDs)) {
			return repository.ErrInvalidInput
		}

		var value interface{}
		if groupID > 0 {
			value = groupID
		}
		return translateError(tx.Model(&models.LLMPlatformModel{}).
			Where("id IN ?", modelIDs).
			Update("display_group_id", value).Error)
	})
}

// GetModelDisplayGroupByID 按 ID 获取自定义模型展示分组。
func (r *Repo) GetModelDisplayGroupByID(ctx context.Context, groupID uint) (*domainchannel.ModelDisplayGroup, error) {
	var item models.LLMModelDisplayGroup
	if err := r.db.WithContext(ctx).Where("id = ?", groupID).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	result := toModelDisplayGroupDomain(item)
	return &result, nil
}

// ListModelDisplayGroups 分页查询自定义模型展示分组。
func (r *Repo) ListModelDisplayGroups(ctx context.Context, input repository.ListModelDisplayGroupsInput) ([]domainchannel.ModelDisplayGroup, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.LLMModelDisplayGroup{})
	if keyword := strings.ToLower(strings.TrimSpace(input.Query)); keyword != "" {
		query = query.Where("LOWER(name) LIKE ?", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	entities := make([]models.LLMModelDisplayGroup, 0)
	if err := query.Order("sort_order ASC, id ASC").Offset(input.Offset).Limit(input.Limit).Find(&entities).Error; err != nil {
		return nil, 0, translateError(err)
	}
	items := make([]domainchannel.ModelDisplayGroup, 0, len(entities))
	for _, entity := range entities {
		items = append(items, toModelDisplayGroupDomain(entity))
	}
	return items, total, nil
}

// DeleteModelDisplayGroup 删除自定义展示分组，并让关联模型恢复按技术厂商展示。
func (r *Repo) DeleteModelDisplayGroup(ctx context.Context, groupID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item models.LLMModelDisplayGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", groupID).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return repository.ErrNotFound
			}
			return translateError(err)
		}
		if err := tx.Model(&models.LLMPlatformModel{}).
			Where("display_group_id = ?", groupID).
			Update("display_group_id", nil).Error; err != nil {
			return translateError(err)
		}
		return translateError(tx.Delete(&models.LLMModelDisplayGroup{}, groupID).Error)
	})
}

func toModelVendorDomain(item models.LLMModelVendor) domainchannel.ModelVendor {
	return domainchannel.ModelVendor{
		ID: item.ID, Key: item.Key, Name: item.Name, Icon: item.Icon,
		BuiltIn: item.BuiltIn, SortOrder: item.SortOrder,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toModelVendorModel(item *domainchannel.ModelVendor) models.LLMModelVendor {
	if item == nil {
		return models.LLMModelVendor{}
	}
	return models.LLMModelVendor{
		Key: item.Key, Name: item.Name, Icon: item.Icon,
		BuiltIn: item.BuiltIn, SortOrder: item.SortOrder,
	}
}

func toModelDisplayGroupDomain(item models.LLMModelDisplayGroup) domainchannel.ModelDisplayGroup {
	return domainchannel.ModelDisplayGroup{
		ID: item.ID, Name: item.Name, Icon: item.Icon, SortOrder: item.SortOrder,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toModelDisplayGroupModel(item *domainchannel.ModelDisplayGroup) models.LLMModelDisplayGroup {
	if item == nil {
		return models.LLMModelDisplayGroup{}
	}
	return models.LLMModelDisplayGroup{Name: item.Name, Icon: item.Icon, SortOrder: item.SortOrder}
}

var _ repository.ModelPresentationRepository = (*Repo)(nil)
