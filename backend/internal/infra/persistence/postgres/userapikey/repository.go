package userapikey

import (
	"context"
	"time"

	domainuserapikey "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/userapikey"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// Repo 聚合用户 API Key 数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(ctx context.Context, item *domainuserapikey.UserAPIKey) error {
	if item == nil {
		return nil
	}
	record := model.UserAPIKey{
		PublicID:  item.PublicID,
		UserID:    item.UserID,
		Name:      item.Name,
		KeyPrefix: item.KeyPrefix,
		KeyHash:   item.KeyHash,
		ExpiresAt: item.ExpiresAt,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return dberror.Translate(err)
	}
	item.ID = record.ID
	item.CreatedAt = record.CreatedAt
	item.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *Repo) ListByUserID(ctx context.Context, userID uint) ([]domainuserapikey.UserAPIKey, error) {
	var rows []model.UserAPIKey
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Order("id DESC").
		Find(&rows).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	items := make([]domainuserapikey.UserAPIKey, 0, len(rows))
	for _, row := range rows {
		items = append(items, toDomain(row))
	}
	return items, nil
}

func (r *Repo) GetByPublicID(ctx context.Context, userID uint, publicID string) (*domainuserapikey.UserAPIKey, error) {
	var row model.UserAPIKey
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND public_id = ?", userID, publicID).
		First(&row).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	item := toDomain(row)
	return &item, nil
}

func (r *Repo) GetActiveByHash(ctx context.Context, keyHash string) (*domainuserapikey.UserAPIKey, error) {
	var row model.UserAPIKey
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Where("key_hash = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)", keyHash, now).
		First(&row).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	item := toDomain(row)
	return &item, nil
}

func (r *Repo) Revoke(ctx context.Context, userID uint, publicID string, revokedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&model.UserAPIKey{}).
		Where("user_id = ? AND public_id = ? AND revoked_at IS NULL", userID, publicID).
		Update("revoked_at", revokedAt)
	if result.Error != nil {
		return dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *Repo) TouchLastUsed(ctx context.Context, id uint, usedAt time.Time) error {
	return dberror.Translate(r.db.WithContext(ctx).
		Model(&model.UserAPIKey{}).
		Where("id = ?", id).
		Update("last_used_at", usedAt).Error)
}

func toDomain(row model.UserAPIKey) domainuserapikey.UserAPIKey {
	return domainuserapikey.UserAPIKey{
		ID:         row.ID,
		PublicID:   row.PublicID,
		UserID:     row.UserID,
		Name:       row.Name,
		KeyPrefix:  row.KeyPrefix,
		KeyHash:    row.KeyHash,
		LastUsedAt: row.LastUsedAt,
		RevokedAt:  row.RevokedAt,
		ExpiresAt:  row.ExpiresAt,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
}
