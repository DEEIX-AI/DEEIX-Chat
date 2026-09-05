package channel

import (
	"context"
	"errors"
	"strings"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const modelVendorDeleteReferencePreviewLimit = 20

// lockModelPresentationReferences 对即将写入模型的厂商和展示分组加共享事务边界。
// 删除操作锁定同一目录行，因此不会在引用检查后产生悬空关联。
func lockModelPresentationReferences(tx *gorm.DB, vendor string, displayGroupID *uint) error {
	if normalizedVendor := strings.TrimSpace(vendor); normalizedVendor != "" {
		var item models.LLMModelVendor
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").Where("key = ?", normalizedVendor).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return repository.ErrModelVendorNotFound
			}
			return dberror.Translate(err)
		}
	}
	if displayGroupID != nil && *displayGroupID > 0 {
		var item models.LLMModelDisplayGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").Where("id = ?", *displayGroupID).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return repository.ErrModelDisplayGroupNotFound
			}
			return dberror.Translate(err)
		}
	}
	return nil
}

// CreateModelIconAsset 保存待写入或已经就绪的图标元数据。
func (r *Repo) CreateModelIconAsset(ctx context.Context, item *domainchannel.ModelIconAsset) error {
	if item == nil {
		return repository.ErrInvalidInput
	}
	entity := toModelIconAssetModel(item)
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return dberror.Translate(err)
	}
	*item = toModelIconAssetDomain(entity)
	return nil
}

// GetModelIconAssetByPublicID 按公开 ID 查询图标资产。
func (r *Repo) GetModelIconAssetByPublicID(ctx context.Context, publicID string) (*domainchannel.ModelIconAsset, error) {
	var item models.LLMModelIconAsset
	if err := r.db.WithContext(ctx).Where("public_id = ?", strings.TrimSpace(publicID)).First(&item).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	result := toModelIconAssetDomain(item)
	return &result, nil
}

// GetModelIconAssetBySHA256 按内容哈希查询图标资产，用于上传去重。
func (r *Repo) GetModelIconAssetBySHA256(ctx context.Context, sha256 string) (*domainchannel.ModelIconAsset, error) {
	var item models.LLMModelIconAsset
	if err := r.db.WithContext(ctx).Where("sha256 = ?", strings.TrimSpace(sha256)).First(&item).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	result := toModelIconAssetDomain(item)
	return &result, nil
}

// ListModelIconAssets 分页查询仍展示在管理员图标库中的就绪资产。
func (r *Repo) ListModelIconAssets(ctx context.Context, offset int, limit int) ([]domainchannel.ModelIconAsset, int64, error) {
	if offset < 0 || limit <= 0 {
		return []domainchannel.ModelIconAsset{}, 0, nil
	}
	query := r.db.WithContext(ctx).Model(&models.LLMModelIconAsset{}).
		Where("ready_at IS NOT NULL AND deleting_at IS NULL AND delete_requested_at IS NULL")
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	entities := make([]models.LLMModelIconAsset, 0)
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&entities).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	items := make([]domainchannel.ModelIconAsset, 0, len(entities))
	for _, entity := range entities {
		items = append(items, toModelIconAssetDomain(entity))
	}
	return items, total, nil
}

// RefreshModelIconAssetUploadLease 刷新上传或去重流程持有的临时租约，并恢复待回收资产。
func (r *Repo) RefreshModelIconAssetUploadLease(ctx context.Context, publicID string, unreferencedAt time.Time, leaseExpiresAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&models.LLMModelIconAsset{}).
		Where("public_id = ? AND deleting_at IS NULL", strings.TrimSpace(publicID)).
		Updates(map[string]any{
			"lease_expires_at":    leaseExpiresAt,
			"unreferenced_at":     unreferencedAt,
			"delete_requested_at": nil,
		})
	if result.Error != nil {
		return dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// MarkModelIconAssetReady 标记对象内容已经写入并通过校验。
func (r *Repo) MarkModelIconAssetReady(ctx context.Context, publicID string, readyAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&models.LLMModelIconAsset{}).
		Where("public_id = ? AND deleting_at IS NULL", strings.TrimSpace(publicID)).
		Update("ready_at", gorm.Expr("COALESCE(ready_at, ?)", readyAt))
	if result.Error != nil {
		return dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// ReserveModelIconAssetReference 在业务配置保存前续租已就绪资产，并取消待回收状态。
func (r *Repo) ReserveModelIconAssetReference(ctx context.Context, publicID string, leaseExpiresAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&models.LLMModelIconAsset{}).
		Where("public_id = ? AND ready_at IS NOT NULL AND deleting_at IS NULL", strings.TrimSpace(publicID)).
		Updates(map[string]any{
			"lease_expires_at":    leaseExpiresAt,
			"unreferenced_at":     nil,
			"delete_requested_at": nil,
		})
	if result.Error != nil {
		return dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// ListExpiredModelIconAssets 查询租约过期或已经进入删除流程的资产。
func (r *Repo) ListExpiredModelIconAssets(ctx context.Context, expiredBefore time.Time, limit int) ([]domainchannel.ModelIconAsset, error) {
	if limit <= 0 {
		return []domainchannel.ModelIconAsset{}, nil
	}
	entities := make([]models.LLMModelIconAsset, 0)
	if err := r.db.WithContext(ctx).
		Where("lease_expires_at <= ? OR deleting_at IS NOT NULL", expiredBefore).
		Order("CASE WHEN deleting_at IS NOT NULL THEN 0 ELSE 1 END, lease_expires_at ASC, id ASC").
		Limit(limit).
		Find(&entities).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	items := make([]domainchannel.ModelIconAsset, 0, len(entities))
	for _, entity := range entities {
		items = append(items, toModelIconAssetDomain(entity))
	}
	return items, nil
}

// HasModelIconAssetReference 检查控制面配置或仍保留的会话运行快照是否引用资产。
func (r *Repo) HasModelIconAssetReference(ctx context.Context, ref string) (bool, error) {
	checks := []struct {
		model  any
		column string
	}{
		{model: &models.LLMPlatformModel{}, column: "icon"},
		{model: &models.LLMModelVendor{}, column: "icon"},
		{model: &models.LLMModelDisplayGroup{}, column: "icon"},
		{model: &models.ConversationRun{}, column: "model_icon"},
	}
	for _, check := range checks {
		var result struct{ Found int }
		if err := r.db.WithContext(ctx).
			Model(check.model).
			Select("1 AS found").
			Where(check.column+" = ?", strings.TrimSpace(ref)).
			Limit(1).
			Scan(&result).Error; err != nil {
			return false, dberror.Translate(err)
		}
		if result.Found == 1 {
			return true, nil
		}
	}
	return false, nil
}

// GetModelIconAssetReferenceSummary 汇总控制面配置与会话快照中的资产引用。
func (r *Repo) GetModelIconAssetReferenceSummary(ctx context.Context, ref string) (repository.ModelIconAssetReferenceSummary, error) {
	normalizedRef := strings.TrimSpace(ref)
	checks := []struct {
		model  any
		column string
		set    func(*repository.ModelIconAssetReferenceSummary, int64)
	}{
		{model: &models.LLMPlatformModel{}, column: "icon", set: func(result *repository.ModelIconAssetReferenceSummary, count int64) { result.Models = count }},
		{model: &models.LLMModelVendor{}, column: "icon", set: func(result *repository.ModelIconAssetReferenceSummary, count int64) { result.Vendors = count }},
		{model: &models.LLMModelDisplayGroup{}, column: "icon", set: func(result *repository.ModelIconAssetReferenceSummary, count int64) { result.DisplayGroups = count }},
		{model: &models.ConversationRun{}, column: "model_icon", set: func(result *repository.ModelIconAssetReferenceSummary, count int64) { result.ConversationRuns = count }},
	}
	var summary repository.ModelIconAssetReferenceSummary
	for _, check := range checks {
		var count int64
		if err := r.db.WithContext(ctx).Model(check.model).Where(check.column+" = ?", normalizedRef).Count(&count).Error; err != nil {
			return repository.ModelIconAssetReferenceSummary{}, dberror.Translate(err)
		}
		check.set(&summary, count)
	}
	return summary, nil
}

// MarkModelIconAssetUnreferenced 开始连续无引用保护期；并发续租优先于该状态切换。
func (r *Repo) MarkModelIconAssetUnreferenced(
	ctx context.Context,
	assetID uint,
	expiredBefore time.Time,
	unreferencedAt time.Time,
	leaseExpiresAt time.Time,
) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&models.LLMModelIconAsset{}).
		Where("id = ? AND lease_expires_at <= ? AND unreferenced_at IS NULL AND deleting_at IS NULL", assetID, expiredBefore).
		Updates(map[string]any{"unreferenced_at": unreferencedAt, "lease_expires_at": leaseExpiresAt})
	if result.Error != nil {
		return false, dberror.Translate(result.Error)
	}
	return result.RowsAffected > 0, nil
}

// RequestModelIconAssetDeletion 从图标库隐藏未使用资产，并开始 24 小时安全回收期。
func (r *Repo) RequestModelIconAssetDeletion(ctx context.Context, assetID uint, requestedAt time.Time, leaseExpiresAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&models.LLMModelIconAsset{}).
		Where("id = ? AND ready_at IS NOT NULL AND deleting_at IS NULL AND delete_requested_at IS NULL", assetID).
		Updates(map[string]any{
			"delete_requested_at": requestedAt,
			"unreferenced_at":     requestedAt,
			"lease_expires_at":    leaseExpiresAt,
		})
	if result.Error != nil {
		return dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// ClaimModelIconAssetDeletion 原子认领已过期且未被其他操作续租的资产删除任务。
func (r *Repo) ClaimModelIconAssetDeletion(ctx context.Context, assetID uint, expiredBefore time.Time, deletingAt time.Time) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&models.LLMModelIconAsset{}).
		Where("id = ? AND lease_expires_at <= ? AND unreferenced_at IS NOT NULL AND deleting_at IS NULL", assetID, expiredBefore).
		Update("deleting_at", deletingAt)
	if result.Error != nil {
		return false, dberror.Translate(result.Error)
	}
	return result.RowsAffected > 0, nil
}

// DeleteClaimedModelIconAsset 在对象删除成功后移除已经认领的元数据。
func (r *Repo) DeleteClaimedModelIconAsset(ctx context.Context, assetID uint) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND deleting_at IS NOT NULL", assetID).
		Delete(&models.LLMModelIconAsset{})
	if result.Error != nil {
		return dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

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
				return dberror.Translate(err)
			}
			entity.SortOrder = maxSortOrder + 100
		}
		return dberror.Translate(tx.Create(&entity).Error)
	}); err != nil {
		return err
	}
	*item = toModelVendorDomain(entity)
	return nil
}

// UpdateModelVendor 更新技术厂商的展示名称与图标，稳定 key 不参与修改。
func (r *Repo) UpdateModelVendor(ctx context.Context, key string, input repository.UpdateModelVendorInput) error {
	updates := make(map[string]any)
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
		return dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// DeleteModelVendor 删除未被引用的自定义技术厂商。
// 厂商行、引用检查和删除位于同一事务，避免删除检查与并发写入相互穿透。
func (r *Repo) DeleteModelVendor(ctx context.Context, key string) error {
	normalizedKey := strings.TrimSpace(key)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item models.LLMModelVendor
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("key = ?", normalizedKey).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return repository.ErrModelVendorNotFound
			}
			return dberror.Translate(err)
		}
		if item.BuiltIn {
			return &repository.ModelVendorDeleteBlockedError{Reason: repository.ModelVendorDeleteReasonBuiltIn}
		}

		var referenceCount int64
		if err := tx.Model(&models.LLMPlatformModel{}).Where("vendor = ?", normalizedKey).Count(&referenceCount).Error; err != nil {
			return dberror.Translate(err)
		}
		if referenceCount > 0 {
			var referencedModels []struct {
				ID                uint
				PlatformModelName string `gorm:"column:name"`
			}
			if err := tx.Model(&models.LLMPlatformModel{}).
				Select("id", "name").
				Where("vendor = ?", normalizedKey).
				Order("id ASC").
				Limit(modelVendorDeleteReferencePreviewLimit).
				Scan(&referencedModels).Error; err != nil {
				return dberror.Translate(err)
			}
			modelsPreview := make([]repository.ModelVendorReference, 0, len(referencedModels))
			for _, referencedModel := range referencedModels {
				modelsPreview = append(modelsPreview, repository.ModelVendorReference{
					ID: referencedModel.ID, PlatformModelName: referencedModel.PlatformModelName,
				})
			}
			return &repository.ModelVendorDeleteBlockedError{
				Reason:         repository.ModelVendorDeleteReasonReferencedModels,
				ReferenceCount: referenceCount,
				Models:         modelsPreview,
			}
		}
		return dberror.Translate(tx.Delete(&item).Error)
	})
}

// GetModelVendorByKey 按稳定 key 获取技术厂商目录项。
func (r *Repo) GetModelVendorByKey(ctx context.Context, key string) (*domainchannel.ModelVendor, error) {
	var item models.LLMModelVendor
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&item).Error; err != nil {
		return nil, dberror.Translate(err)
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
		return nil, 0, dberror.Translate(err)
	}
	entities := make([]models.LLMModelVendor, 0)
	if err := query.Order("sort_order ASC, id ASC").Offset(input.Offset).Limit(input.Limit).Find(&entities).Error; err != nil {
		return nil, 0, dberror.Translate(err)
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
				return dberror.Translate(err)
			}
			entity.SortOrder = maxSortOrder + 100
		}
		if err := tx.Create(&entity).Error; err != nil {
			return dberror.Translate(err)
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
	updates := make(map[string]any)
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Icon != nil {
		updates["icon"] = *input.Icon
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item models.LLMModelDisplayGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", groupID).First(&item).Error; err != nil {
			return dberror.Translate(err)
		}
		if len(updates) > 0 {
			if err := tx.Model(&models.LLMModelDisplayGroup{}).
				Where("id = ?", groupID).
				Updates(updates).Error; err != nil {
				return dberror.Translate(err)
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
			return dberror.Translate(err)
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
		return dberror.Translate(err)
	}
	if len(modelIDs) == 0 {
		return nil
	}
	return dberror.Translate(tx.Model(&models.LLMPlatformModel{}).
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
				return dberror.Translate(err)
			}
		}

		var count int64
		if err := tx.Model(&models.LLMPlatformModel{}).Where("id IN ?", modelIDs).Count(&count).Error; err != nil {
			return dberror.Translate(err)
		}
		if count != int64(len(modelIDs)) {
			return repository.ErrInvalidInput
		}

		var value any
		if groupID > 0 {
			value = groupID
		}
		return dberror.Translate(tx.Model(&models.LLMPlatformModel{}).
			Where("id IN ?", modelIDs).
			Update("display_group_id", value).Error)
	})
}

// GetModelDisplayGroupByID 按 ID 获取自定义模型展示分组。
func (r *Repo) GetModelDisplayGroupByID(ctx context.Context, groupID uint) (*domainchannel.ModelDisplayGroup, error) {
	var item models.LLMModelDisplayGroup
	if err := r.db.WithContext(ctx).Where("id = ?", groupID).First(&item).Error; err != nil {
		return nil, dberror.Translate(err)
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
		return nil, 0, dberror.Translate(err)
	}
	entities := make([]models.LLMModelDisplayGroup, 0)
	if err := query.Order("sort_order ASC, id ASC").Offset(input.Offset).Limit(input.Limit).Find(&entities).Error; err != nil {
		return nil, 0, dberror.Translate(err)
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
			return dberror.Translate(err)
		}
		if err := tx.Model(&models.LLMPlatformModel{}).
			Where("display_group_id = ?", groupID).
			Update("display_group_id", nil).Error; err != nil {
			return dberror.Translate(err)
		}
		return dberror.Translate(tx.Delete(&models.LLMModelDisplayGroup{}, groupID).Error)
	})
}

func toModelVendorDomain(item models.LLMModelVendor) domainchannel.ModelVendor {
	return domainchannel.ModelVendor{
		ID: item.ID, Key: item.Key, Name: item.Name, Icon: item.Icon,
		BuiltIn: item.BuiltIn, SortOrder: item.SortOrder,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toModelIconAssetDomain(item models.LLMModelIconAsset) domainchannel.ModelIconAsset {
	return domainchannel.ModelIconAsset{
		ID: item.ID, PublicID: item.PublicID, SHA256: item.SHA256,
		StoragePath: item.StoragePath, ContentType: item.ContentType,
		SizeBytes: item.SizeBytes, Width: item.Width, Height: item.Height,
		CreatedByUserID: item.CreatedByUserID, ReadyAt: item.ReadyAt,
		LeaseExpiresAt: item.LeaseExpiresAt, UnreferencedAt: item.UnreferencedAt,
		DeleteRequestedAt: item.DeleteRequestedAt, DeletingAt: item.DeletingAt,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func toModelIconAssetModel(item *domainchannel.ModelIconAsset) models.LLMModelIconAsset {
	if item == nil {
		return models.LLMModelIconAsset{}
	}
	return models.LLMModelIconAsset{
		PublicID: item.PublicID, SHA256: item.SHA256, StoragePath: item.StoragePath,
		ContentType: item.ContentType, SizeBytes: item.SizeBytes, Width: item.Width, Height: item.Height,
		CreatedByUserID: item.CreatedByUserID, ReadyAt: item.ReadyAt,
		LeaseExpiresAt: item.LeaseExpiresAt, UnreferencedAt: item.UnreferencedAt,
		DeleteRequestedAt: item.DeleteRequestedAt, DeletingAt: item.DeletingAt,
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
var _ repository.ModelIconAssetRepository = (*Repo)(nil)
