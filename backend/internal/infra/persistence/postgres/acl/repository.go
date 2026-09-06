package acl

import (
	"context"

	domainacl "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/acl"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo 聚合资源 ACL 数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Upsert(ctx context.Context, entry *domainacl.Entry) error {
	if entry == nil {
		return nil
	}
	record := model.ResourceACLEntry{
		ResourceType:     entry.ResourceType,
		ResourcePublicID: entry.ResourcePublicID,
		GranteeUserID:    entry.GranteeUserID,
		Role:             entry.Role,
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "resource_type"},
			{Name: "resource_public_id"},
			{Name: "grantee_user_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"role", "updated_at"}),
	}).Create(&record).Error
	if err != nil {
		return dberror.Translate(err)
	}
	entry.ID = record.ID
	entry.CreatedAt = record.CreatedAt
	entry.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *Repo) Delete(ctx context.Context, resourceType string, resourcePublicID string, granteeUserID uint) error {
	result := r.db.WithContext(ctx).
		Where("resource_type = ? AND resource_public_id = ? AND grantee_user_id = ?", resourceType, resourcePublicID, granteeUserID).
		Delete(&model.ResourceACLEntry{})
	if result.Error != nil {
		return dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *Repo) ListByResource(ctx context.Context, resourceType string, resourcePublicID string) ([]domainacl.Entry, error) {
	var rows []model.ResourceACLEntry
	if err := r.db.WithContext(ctx).
		Where("resource_type = ? AND resource_public_id = ?", resourceType, resourcePublicID).
		Order("created_at ASC").
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	items := make([]domainacl.Entry, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomain(row))
	}
	return items, nil
}

func (r *Repo) Get(ctx context.Context, resourceType string, resourcePublicID string, granteeUserID uint) (*domainacl.Entry, error) {
	var row model.ResourceACLEntry
	if err := r.db.WithContext(ctx).
		Where("resource_type = ? AND resource_public_id = ? AND grantee_user_id = ?", resourceType, resourcePublicID, granteeUserID).
		First(&row).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	item := toDomain(row)
	return &item, nil
}

func (r *Repo) ListResourcePublicIDsByGrantee(ctx context.Context, resourceType string, granteeUserID uint) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&model.ResourceACLEntry{}).
		Where("resource_type = ? AND grantee_user_id = ?", resourceType, granteeUserID).
		Pluck("resource_public_id", &ids).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return ids, nil
}

func toDomain(row model.ResourceACLEntry) domainacl.Entry {
	return domainacl.Entry{
		ID:               row.ID,
		ResourceType:     row.ResourceType,
		ResourcePublicID: row.ResourcePublicID,
		GranteeUserID:    row.GranteeUserID,
		Role:             row.Role,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}
