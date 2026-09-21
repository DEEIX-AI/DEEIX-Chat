package uicomponent

import (
	"context"
	"strings"

	domainuicomponent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/uicomponent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo 封装交互式组件数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建组件仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// ListUIComponents 分页查询组件。
func (r *Repo) ListUIComponents(ctx context.Context, filter repository.UIComponentListFilter, offset int, limit int) ([]domainuicomponent.Component, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	var items []model.UIComponent
	var total int64
	query := applyFilter(r.db.WithContext(ctx).Model(&model.UIComponent{}), filter)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	if err := query.Order(orderClause(filter)).Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	results := make([]domainuicomponent.Component, 0, len(items))
	for _, item := range items {
		results = append(results, toDomain(item))
	}
	return results, total, nil
}

// GetUIComponent 按主键查询组件。
func (r *Repo) GetUIComponent(ctx context.Context, id uint) (*domainuicomponent.Component, error) {
	if id == 0 {
		return nil, repository.ErrInvalidInput
	}
	var record model.UIComponent
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	result := toDomain(record)
	return &result, nil
}

// CreateUIComponent 创建组件。
func (r *Repo) CreateUIComponent(ctx context.Context, item *domainuicomponent.Component) (*domainuicomponent.Component, error) {
	if item == nil {
		return nil, repository.ErrInvalidInput
	}
	record := model.UIComponent{
		Scope:           strings.TrimSpace(item.Scope),
		OwnerUserID:     item.OwnerUserID,
		Name:            strings.TrimSpace(item.Name),
		Version:         item.Version,
		Description:     strings.TrimSpace(item.Description),
		PropsSummary:    strings.TrimSpace(item.PropsSummary),
		PropsSchema:     strings.TrimSpace(item.PropsSchema),
		RendererKind:    strings.TrimSpace(item.RendererKind),
		RendererSource:  item.RendererSource,
		Enabled:         item.Enabled,
		SortOrder:       item.SortOrder,
		CreatedByUserID: item.CreatedByUserID,
		UpdatedByUserID: item.UpdatedByUserID,
	}
	var result domainuicomponent.Component
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if record.SortOrder <= 0 {
			var maxSortOrder int
			if err := tx.Model(&model.UIComponent{}).
				Where("scope = ? AND owner_user_id = ?", record.Scope, record.OwnerUserID).
				Select("COALESCE(MAX(sort_order), 0)").
				Scan(&maxSortOrder).Error; err != nil {
				return dberror.Translate(err)
			}
			record.SortOrder = maxSortOrder + 1
		}
		if err := tx.Create(&record).Error; err != nil {
			return dberror.Translate(err)
		}
		result = toDomain(record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// PatchUIComponent 更新组件字段。
func (r *Repo) PatchUIComponent(ctx context.Context, id uint, patch repository.UIComponentPatch) (*domainuicomponent.Component, error) {
	if id == 0 {
		return nil, repository.ErrInvalidInput
	}
	var result domainuicomponent.Component
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record model.UIComponent
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&record).Error; err != nil {
			return dberror.Translate(err)
		}
		updates := map[string]any{}
		if patch.Name != nil {
			updates["name"] = strings.TrimSpace(*patch.Name)
		}
		if patch.Version != nil {
			updates["version"] = *patch.Version
		}
		if patch.Description != nil {
			updates["description"] = strings.TrimSpace(*patch.Description)
		}
		if patch.PropsSummary != nil {
			updates["props_summary"] = strings.TrimSpace(*patch.PropsSummary)
		}
		if patch.PropsSchema != nil {
			updates["props_schema"] = strings.TrimSpace(*patch.PropsSchema)
		}
		if patch.RendererSource != nil {
			updates["renderer_source"] = *patch.RendererSource
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
				return dberror.Translate(err)
			}
		}
		if err := tx.Where("id = ?", id).First(&record).Error; err != nil {
			return dberror.Translate(err)
		}
		result = toDomain(record)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteUIComponent 删除组件。
func (r *Repo) DeleteUIComponent(ctx context.Context, id uint) error {
	if id == 0 {
		return repository.ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Delete(&model.UIComponent{}, id)
	if result.Error != nil {
		return dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func applyFilter(query *gorm.DB, filter repository.UIComponentListFilter) *gorm.DB {
	if len(filter.IDs) > 0 {
		query = query.Where("id IN ?", filter.IDs)
	}
	if filter.VisibleUserID != nil {
		query = query.Where(
			"(scope IN ? AND enabled = ?) OR (scope = ? AND owner_user_id = ? AND enabled = ?)",
			[]string{domainuicomponent.ScopeBuiltin, domainuicomponent.ScopePlatform},
			true,
			domainuicomponent.ScopeUser,
			*filter.VisibleUserID,
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
		query = query.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", like, like)
	}
	return query
}

func orderClause(filter repository.UIComponentListFilter) string {
	if filter.VisibleUserID != nil {
		return "CASE scope WHEN 'builtin' THEN 0 WHEN 'platform' THEN 1 ELSE 2 END ASC, sort_order ASC, id ASC"
	}
	return "CASE WHEN enabled THEN 0 ELSE 1 END ASC, sort_order ASC, updated_at DESC, id DESC"
}

func toDomain(item model.UIComponent) domainuicomponent.Component {
	return domainuicomponent.Component{
		ID:              item.ID,
		Scope:           item.Scope,
		OwnerUserID:     item.OwnerUserID,
		Name:            item.Name,
		Version:         item.Version,
		Description:     item.Description,
		PropsSummary:    item.PropsSummary,
		PropsSchema:     item.PropsSchema,
		RendererKind:    item.RendererKind,
		RendererSource:  item.RendererSource,
		Enabled:         item.Enabled,
		SortOrder:       item.SortOrder,
		CreatedByUserID: item.CreatedByUserID,
		UpdatedByUserID: item.UpdatedByUserID,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}
