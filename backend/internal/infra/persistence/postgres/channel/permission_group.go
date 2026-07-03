package channel

import (
	"context"
	"errors"
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// 权限组（模型访问控制）
// ---------------------------------------------------------------------------

// ListPermissionGroups 返回全部权限组，默认组优先。
func (r *Repo) ListPermissionGroups(ctx context.Context) ([]domainchannel.PermissionGroup, error) {
	items := make([]model.PermissionGroup, 0)
	if err := r.db.WithContext(ctx).
		Order("is_default DESC, id ASC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	results := make([]domainchannel.PermissionGroup, 0, len(items))
	for _, item := range items {
		results = append(results, toPermissionGroupDomain(item))
	}
	return results, nil
}

// GetPermissionGroup 按 ID 获取权限组。
func (r *Repo) GetPermissionGroup(ctx context.Context, id uint) (*domainchannel.PermissionGroup, error) {
	var item model.PermissionGroup
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, translateError(err)
	}
	result := toPermissionGroupDomain(item)
	return &result, nil
}

// CreatePermissionGroup 创建权限组。
func (r *Repo) CreatePermissionGroup(ctx context.Context, item *domainchannel.PermissionGroup) error {
	if item == nil {
		return repository.ErrInvalidInput
	}
	entity := model.PermissionGroup{
		Code:                  strings.TrimSpace(item.Code),
		Name:                  strings.TrimSpace(item.Name),
		Description:           strings.TrimSpace(item.Description),
		IsDefault:             item.IsDefault,
		RateMultiplierPercent: normalizeRateMultiplierPercent(item.RateMultiplierPercent),
	}
	if entity.Code == "" || entity.Name == "" {
		return repository.ErrInvalidInput
	}
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return translateError(err)
	}
	*item = toPermissionGroupDomain(entity)
	return nil
}

// UpdatePermissionGroup 更新分组名称、说明与计费倍率。
func (r *Repo) UpdatePermissionGroup(ctx context.Context, id uint, name string, description string, rateMultiplierPercent int) (*domainchannel.PermissionGroup, error) {
	result := r.db.WithContext(ctx).
		Model(&model.PermissionGroup{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"name":                    strings.TrimSpace(name),
			"description":             strings.TrimSpace(description),
			"rate_multiplier_percent": normalizeRateMultiplierPercent(rateMultiplierPercent),
		})
	if result.Error != nil {
		return nil, translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, repository.ErrNotFound
	}
	return r.GetPermissionGroup(ctx, id)
}

// DeletePermissionGroup 硬删除权限组及其模型/用户关联。
func (r *Repo) DeletePermissionGroup(ctx context.Context, id uint) error {
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.PermissionGroup
		if err := tx.Where("id = ?", id).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return repository.ErrNotFound
			}
			return err
		}
		if err := tx.Where("group_id = ?", id).Delete(&model.PermissionGroupModelAccess{}).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", id).Delete(&model.PermissionGroupUserAccess{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.PermissionGroup{}, id).Error
	}))
}

// ---------------------------------------------------------------------------
// 权限组 - 模型关联
// ---------------------------------------------------------------------------

// ListGroupModelIDs 返回权限组已授权的平台模型 ID。
func (r *Repo) ListGroupModelIDs(ctx context.Context, groupID uint) ([]uint, error) {
	ids := make([]uint, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupModelAccess{}).
		Where("group_id = ?", groupID).
		Order("platform_model_id ASC").
		Pluck("platform_model_id", &ids).Error; err != nil {
		return nil, translateError(err)
	}
	return ids, nil
}

// SetGroupModels 全量替换权限组授权的平台模型集合。
func (r *Repo) SetGroupModels(ctx context.Context, groupID uint, modelIDs []uint) error {
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupID).Delete(&model.PermissionGroupModelAccess{}).Error; err != nil {
			return err
		}
		rows := dedupeAccessModelRows(groupID, modelIDs)
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	}))
}

// ---------------------------------------------------------------------------
// 权限组 - 用户关联
// ---------------------------------------------------------------------------

// ListGroupUserIDs 返回权限组内的用户 ID。
func (r *Repo) ListGroupUserIDs(ctx context.Context, groupID uint) ([]uint, error) {
	ids := make([]uint, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupUserAccess{}).
		Where("group_id = ?", groupID).
		Order("user_id ASC").
		Pluck("user_id", &ids).Error; err != nil {
		return nil, translateError(err)
	}
	return ids, nil
}

// SetGroupUsers 全量替换权限组内的用户集合。
func (r *Repo) SetGroupUsers(ctx context.Context, groupID uint, userIDs []uint) error {
	return translateError(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", groupID).Delete(&model.PermissionGroupUserAccess{}).Error; err != nil {
			return err
		}
		rows := dedupeAccessUserRows(groupID, userIDs)
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	}))
}

// ---------------------------------------------------------------------------
// 访问判定
// ---------------------------------------------------------------------------

// ListUserGroupIDs 返回用户显式归属的权限组 ID。
func (r *Repo) ListUserGroupIDs(ctx context.Context, userID uint) ([]uint, error) {
	ids := make([]uint, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupUserAccess{}).
		Where("user_id = ?", userID).
		Order("group_id ASC").
		Pluck("group_id", &ids).Error; err != nil {
		return nil, translateError(err)
	}
	return ids, nil
}

// ListModelGroupIDs 返回授权某平台模型的权限组 ID。
func (r *Repo) ListModelGroupIDs(ctx context.Context, platformModelID uint) ([]uint, error) {
	ids := make([]uint, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroupModelAccess{}).
		Where("platform_model_id = ?", platformModelID).
		Order("group_id ASC").
		Pluck("group_id", &ids).Error; err != nil {
		return nil, translateError(err)
	}
	return ids, nil
}

// ListModelsWithGroupAccess 返回所有已配置权限组的模型到权限组 ID 的映射。
func (r *Repo) ListModelsWithGroupAccess(ctx context.Context) (map[uint][]uint, error) {
	var rows []model.PermissionGroupModelAccess
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, translateError(err)
	}
	result := make(map[uint][]uint)
	for _, row := range rows {
		result[row.PlatformModelID] = append(result[row.PlatformModelID], row.GroupID)
	}
	return result, nil
}

// ListDefaultGroupIDs 返回内置默认权限组 ID（所有用户隐式归属）。
func (r *Repo) ListDefaultGroupIDs(ctx context.Context) ([]uint, error) {
	ids := make([]uint, 0)
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroup{}).
		Where("is_default = ?", true).
		Order("id ASC").
		Pluck("id", &ids).Error; err != nil {
		return nil, translateError(err)
	}
	return ids, nil
}

// CountBillingPlansWithGroupID 统计引用指定权限组的计费套餐数量。
func (r *Repo) CountBillingPlansWithGroupID(ctx context.Context, groupID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Table("billing_plans").
		Where("permission_group_id = ?", groupID).
		Count(&count).Error; err != nil {
		return 0, translateError(err)
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// 映射与辅助
// ---------------------------------------------------------------------------

func toPermissionGroupDomain(item model.PermissionGroup) domainchannel.PermissionGroup {
	return domainchannel.PermissionGroup{
		ID:                    item.ID,
		Code:                  item.Code,
		Name:                  item.Name,
		Description:           item.Description,
		IsDefault:             item.IsDefault,
		RateMultiplierPercent: normalizeRateMultiplierPercent(item.RateMultiplierPercent),
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
	}
}

func normalizeRateMultiplierPercent(value int) int {
	if value <= 0 {
		return 100
	}
	return value
}

// GetUserGroupRateMultiplierPercent 返回用户的计费倍率百分比。
//
// 显式分组（直接绑定 + 额外指定组，如订阅绑定组）优先于默认组：
// 用户有显式分组时在显式分组中取最低倍率；否则回退到默认组倍率；
// 均无时返回 100（1.0x）。
func (r *Repo) GetUserGroupRateMultiplierPercent(ctx context.Context, userID uint, extraGroupIDs []uint) (int, error) {
	explicitIDs := make([]uint, 0, len(extraGroupIDs)+4)
	seen := make(map[uint]struct{}, len(extraGroupIDs)+4)
	appendID := func(id uint) {
		if id == 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		explicitIDs = append(explicitIDs, id)
	}

	if userID > 0 {
		directIDs, err := r.ListUserGroupIDs(ctx, userID)
		if err != nil {
			return 100, translateError(err)
		}
		for _, id := range directIDs {
			appendID(id)
		}
	}
	for _, id := range extraGroupIDs {
		appendID(id)
	}

	groupIDs := explicitIDs
	if len(groupIDs) == 0 {
		defaultIDs, err := r.ListDefaultGroupIDs(ctx)
		if err != nil {
			return 100, translateError(err)
		}
		groupIDs = defaultIDs
	}
	if len(groupIDs) == 0 {
		return 100, nil
	}

	var minPercent int
	if err := r.db.WithContext(ctx).
		Model(&model.PermissionGroup{}).
		Where("id IN ?", groupIDs).
		Select("COALESCE(MIN(rate_multiplier_percent), 100)").
		Scan(&minPercent).Error; err != nil {
		return 100, translateError(err)
	}
	return normalizeRateMultiplierPercent(minPercent), nil
}

func dedupeAccessModelRows(groupID uint, modelIDs []uint) []model.PermissionGroupModelAccess {
	seen := make(map[uint]struct{}, len(modelIDs))
	rows := make([]model.PermissionGroupModelAccess, 0, len(modelIDs))
	for _, id := range modelIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		rows = append(rows, model.PermissionGroupModelAccess{GroupID: groupID, PlatformModelID: id})
	}
	return rows
}

func dedupeAccessUserRows(groupID uint, userIDs []uint) []model.PermissionGroupUserAccess {
	seen := make(map[uint]struct{}, len(userIDs))
	rows := make([]model.PermissionGroupUserAccess, 0, len(userIDs))
	for _, id := range userIDs {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		rows = append(rows, model.PermissionGroupUserAccess{GroupID: groupID, UserID: id})
	}
	return rows
}
