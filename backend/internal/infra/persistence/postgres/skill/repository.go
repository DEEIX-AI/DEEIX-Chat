package skill

import (
	"context"
	"strings"

	domainskill "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/skill"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo 封装技能数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建技能仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// ListSkills 分页查询技能。
func (r *Repo) ListSkills(ctx context.Context, filter repository.SkillListFilter, offset int, limit int) ([]domainskill.Skill, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	items := make([]model.Skill, 0, limit)
	var total int64
	query := r.db.WithContext(ctx).Model(&model.Skill{})
	query = applySkillFilter(query, filter)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	if err := query.
		Order(skillOrderClause(filter)).
		Offset(offset).
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, 0, translateError(err)
	}

	results := make([]domainskill.Skill, 0, len(items))
	for _, item := range items {
		results = append(results, toDomain(item))
	}
	return results, total, nil
}

// GetSkill 按主键查询技能。
func (r *Repo) GetSkill(ctx context.Context, id uint) (*domainskill.Skill, error) {
	if id == 0 {
		return nil, repository.ErrInvalidInput
	}
	var record model.Skill
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		return nil, translateError(err)
	}
	result := toDomain(record)
	return &result, nil
}

// CreateSkill 创建技能。
func (r *Repo) CreateSkill(ctx context.Context, item *domainskill.Skill) (*domainskill.Skill, error) {
	if item == nil {
		return nil, repository.ErrInvalidInput
	}
	record := model.Skill{
		Scope:           strings.TrimSpace(item.Scope),
		OwnerUserID:     item.OwnerUserID,
		Title:           strings.TrimSpace(item.Title),
		Trigger:         strings.TrimSpace(item.Trigger),
		Description:     strings.TrimSpace(item.Description),
		Markdown:        strings.TrimSpace(item.Markdown),
		Enabled:         item.Enabled,
		SortOrder:       item.SortOrder,
		CreatedByUserID: item.CreatedByUserID,
		UpdatedByUserID: item.UpdatedByUserID,
	}
	var result domainskill.Skill
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if record.SortOrder <= 0 {
			var maxSortOrder int
			if err := tx.Model(&model.Skill{}).
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

// PatchSkill 更新技能字段。
func (r *Repo) PatchSkill(ctx context.Context, id uint, patch repository.SkillPatch) (*domainskill.Skill, error) {
	if id == 0 {
		return nil, repository.ErrInvalidInput
	}
	var result domainskill.Skill
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record model.Skill
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
		if patch.Markdown != nil {
			updates["markdown"] = strings.TrimSpace(*patch.Markdown)
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

// DeleteSkill 删除技能。
func (r *Repo) DeleteSkill(ctx context.Context, id uint) error {
	if id == 0 {
		return repository.ErrInvalidInput
	}
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Delete(&model.Skill{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return repository.ErrNotFound
		}
		return tx.Where("skill_id = ?", id).Delete(&model.ConversationProjectSkill{}).Error
	}); err != nil {
		return translateError(err)
	}
	return nil
}

func applySkillFilter(query *gorm.DB, filter repository.SkillListFilter) *gorm.DB {
	if len(filter.IDs) > 0 {
		query = query.Where("id IN ?", filter.IDs)
	}
	if filter.VisibleUserID != nil {
		userID := *filter.VisibleUserID
		query = query.Where(
			"(scope = ? AND enabled = ?) OR (scope = ? AND owner_user_id = ? AND enabled = ?)",
			domainskill.ScopeBuiltin,
			true,
			domainskill.ScopeUser,
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
		if filter.SearchMarkdown {
			query = query.Where(
				"LOWER(title) LIKE ? OR LOWER(trigger) LIKE ? OR LOWER(description) LIKE ? OR LOWER(markdown) LIKE ?",
				like,
				like,
				like,
				like,
			)
		} else {
			query = query.Where(
				"LOWER(title) LIKE ? OR LOWER(trigger) LIKE ? OR LOWER(description) LIKE ?",
				like,
				like,
				like,
			)
		}
	}
	return query
}

func skillOrderClause(filter repository.SkillListFilter) string {
	if filter.VisibleUserID != nil {
		return "CASE WHEN scope = 'user' THEN 0 ELSE 1 END ASC, sort_order ASC, updated_at DESC, id DESC"
	}
	return "CASE WHEN enabled THEN 0 ELSE 1 END ASC, sort_order ASC, updated_at DESC, id DESC"
}

func toDomain(item model.Skill) domainskill.Skill {
	return domainskill.Skill{
		ID:              item.ID,
		Scope:           item.Scope,
		OwnerUserID:     item.OwnerUserID,
		Title:           item.Title,
		Trigger:         item.Trigger,
		Description:     item.Description,
		Markdown:        item.Markdown,
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
