package repository

import (
	"context"
	"time"

	domainuserapikey "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/userapikey"
)

// UserAPIKeyRepository 定义用户网关 API Key 持久化能力。
type UserAPIKeyRepository interface {
	Create(ctx context.Context, item *domainuserapikey.UserAPIKey) error
	ListByUserID(ctx context.Context, userID uint) ([]domainuserapikey.UserAPIKey, error)
	GetByPublicID(ctx context.Context, userID uint, publicID string) (*domainuserapikey.UserAPIKey, error)
	GetActiveByHash(ctx context.Context, keyHash string) (*domainuserapikey.UserAPIKey, error)
	Revoke(ctx context.Context, userID uint, publicID string, revokedAt time.Time) error
	TouchLastUsed(ctx context.Context, id uint, usedAt time.Time) error
}
