package model

import "time"

// UserAPIKey 存储用户 OpenAI 兼容网关凭据（仅存哈希）。
type UserAPIKey struct {
	BaseModel
	PublicID   string     `gorm:"size:64;not null;default:'';uniqueIndex:idx_identity_user_api_keys_public_id;comment:对外ID"`
	UserID     uint       `gorm:"not null;default:0;index:idx_identity_user_api_keys_user_id;comment:用户ID"`
	Name       string     `gorm:"size:128;not null;default:'';comment:显示名称"`
	KeyPrefix  string     `gorm:"size:16;not null;default:'';index:idx_identity_user_api_keys_prefix;comment:密钥前缀"`
	KeyHash    string     `gorm:"size:64;not null;default:'';uniqueIndex:idx_identity_user_api_keys_hash;comment:密钥SHA256"`
	LastUsedAt *time.Time `gorm:"index:idx_identity_user_api_keys_last_used;comment:最近使用时间"`
	RevokedAt  *time.Time `gorm:"index:idx_identity_user_api_keys_revoked;comment:吊销时间"`
	ExpiresAt  *time.Time `gorm:"index:idx_identity_user_api_keys_expires;comment:过期时间"`
}

// TableName 指定表名（禁止复用已废弃的 user_api_keys）。
func (UserAPIKey) TableName() string {
	return "identity_user_api_keys"
}
