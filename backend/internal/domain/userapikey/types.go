package userapikey

import "time"

// UserAPIKey 是用户网关 API Key 的领域对象（不含明文）。
type UserAPIKey struct {
	ID         uint
	PublicID   string
	UserID     uint
	Name       string
	KeyPrefix  string
	KeyHash    string
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	ExpiresAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AuthenticatedKey 是鉴权成功后的主体快照。
type AuthenticatedKey struct {
	Key    UserAPIKey
	UserID uint
}
