package promptpreset

import (
	"context"
	"strings"

	domainpromptpreset "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/promptpreset"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo 封装预制提示词数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建预制提示词仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// ListPromptPresets 分页查询预制提示词。
func (r *Repo) ListPromptPresets(ctx context.Context, filter repository.PromptPresetListFilter, offset int, limit int) ([]domainpromptpreset.PromptPreset, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	items := make([]model.PromptPreset, 0, limit)
	var total int64
	query := r.db.WithContext(ctx).Model(&model.PromptPreset{})
	query = applyPromptPresetFilter(query, filter)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	if err := query.
		Order(promptPresetOrderClause(filter)).
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}

	results := make([]domainpromptpreset.PromptPreset, 0, len(items))
	for _, item := range items {
		results = append(results, toDomain(item))
	}
	return results, total, nil
}

// GetPromptPreset 按主键查询预制提示词。
func (r *Repo) GetPromptPreset(ctx context.Context, id uint) (*domainpromptpreset.PromptPreset, error) {
	if id == 0 {
		return nil, repository.ErrInvalidInput
	}
	var record model.PromptPreset
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		return nil, translateError(err)
	}
	result := toDomain(record)
	return &result, nil
}

// CreatePromptPreset 创建预制提示词。
func (r *Repo) CreatePromptPreset(ctx context.Context, item *domainpromptpreset.PromptPreset) (*domainpromptpreset.PromptPreset, error) {
	if item == nil {
		return nil, repository.ErrInvalidInput
	}
	record := model.PromptPreset{
		Scope:           strings.TrimSpace(item.Scope),
		OwnerUserID:     item.OwnerUserID,
		Title:           strings.TrimSpace(item.Title),
		Trigger:         strings.TrimSpace(item.Trigger),
		Description:     strings.TrimSpace(item.Description),
		Content:         strings.TrimSpace(item.Content),
		Enabled:         item.Enabled,
		SortOrder:       item.SortOrder,
		CreatedByUserID: item.CreatedByUserID,
		UpdatedByUserID: item.UpdatedByUserID,
	}
	var result domainpromptpreset.PromptPreset
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if record.SortOrder <= 0 {
			var maxSortOrder int
			if err := tx.Model(&model.PromptPreset{}).
				Where("scope = ? AND owner_user_id = ?", record.Scope, record.OwnerUserID).
				Select("COALESCE(MAX(sort_order), 0)").
				Scan(&maxSortOrder).Error; err != nil {
				return translateError(err)
			}
			record.SortOrder = maxSortOrder + 1
		}
		if err := tx.Create(&record).Error; err != nil {
			return translateError(err)
		}
		result = toDomain(record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// PatchPromptPreset 更新预制提示词字段。
func (r *Repo) PatchPromptPreset(ctx context.Context, id uint, patch repository.PromptPresetPatch) (*domainpromptpreset.PromptPreset, error) {
	if id == 0 {
		return nil, repository.ErrInvalidInput
	}
	var result domainpromptpreset.PromptPreset
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record model.PromptPreset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).
			First(&record).Error; err != nil {
			return translateError(err)
		}

		updates := map[string]interface{}{}
		if patch.Title != nil {
			updates["title"] = strings.TrimSpace(*patch.Title)
		}
		if patch.Trigger != nil {
			updates["trigger"] = strings.TrimSpace(*patch.Trigger)
		}
		if patch.Description != nil {
			updates["description"] = strings.TrimSpace(*patch.Description)
		}
		if patch.Content != nil {
			updates["content"] = strings.TrimSpace(*patch.Content)
		}
		if patch.Enabled != nil {
			updates["enabled"] = *patch.Enabled
		}
		if patch.SortOrder != nil {
			updates["sort_order"] = *patch.SortOrder
		}
		if patch.UpdatedByUserIDSet {
			updates["updated_by_user_id"] = patch.UpdatedByUserID
		}
		if len(updates) > 0 {
			if err := tx.Model(&record).Updates(updates).Error; err != nil {
				return translateError(err)
			}
		}
		if err := tx.Where("id = ?", id).First(&record).Error; err != nil {
			return translateError(err)
		}
		result = toDomain(record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// DeletePromptPreset 删除预制提示词。
func (r *Repo) DeletePromptPreset(ctx context.Context, id uint) error {
	if id == 0 {
		return repository.ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Delete(&model.PromptPreset{}, id)
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func applyPromptPresetFilter(query *gorm.DB, filter repository.PromptPresetListFilter) *gorm.DB {
	if filter.VisibleUserID != nil {
		userID := *filter.VisibleUserID
		query = query.Where(
			"(scope = ? AND enabled = ?) OR (scope = ? AND owner_user_id = ? AND enabled = ?)",
			domainpromptpreset.ScopeBuiltin,
			true,
			domainpromptpreset.ScopeUser,
			userID,
			true,
		)
	} else {
		if scope := strings.TrimSpace(filter.Scope); scope != "" {
			query = query.Where("scope = ?", scope)
		}
		if filter.OwnerUserID != nil {
			query = query.Where("owner_user_id = ?", *filter.OwnerUserID)
		}
		if filter.Enabled != nil {
			query = query.Where("enabled = ?", *filter.Enabled)
		}
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Where(
			"LOWER(title) LIKE ? OR LOWER(trigger) LIKE ? OR LOWER(description) LIKE ? OR LOWER(content) LIKE ?",
			like,
			like,
			like,
			like,
		)
	}
	return query
}

func promptPresetOrderClause(filter repository.PromptPresetListFilter) string {
	if filter.VisibleUserID != nil {
		return "CASE WHEN scope = 'user' THEN 0 ELSE 1 END ASC, sort_order ASC, updated_at DESC, id DESC"
	}
	return "CASE WHEN enabled THEN 0 ELSE 1 END ASC, sort_order ASC, updated_at DESC, id DESC"
}

func toDomain(item model.PromptPreset) domainpromptpreset.PromptPreset {
	return domainpromptpreset.PromptPreset{
		ID:              item.ID,
		Scope:           item.Scope,
		OwnerUserID:     item.OwnerUserID,
		Title:           item.Title,
		Trigger:         item.Trigger,
		Description:     item.Description,
		Content:         item.Content,
		Enabled:         item.Enabled,
		SortOrder:       item.SortOrder,
		CreatedByUserID: item.CreatedByUserID,
		UpdatedByUserID: item.UpdatedByUserID,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func translateError(err error) error {
	if err == nil {
		return nil
	}
	if dberror.IsRecordNotFound(err) {
		return repository.ErrNotFound
	}
	if dberror.IsUniqueConstraint(err) {
		return repository.ErrDuplicate
	}
	return err
}
