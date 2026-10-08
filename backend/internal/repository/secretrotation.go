package repository

import (
	"context"
	"time"
)

// SecretField 标识一个以 secretbox 载荷存储敏感值的字段。
type SecretField string

const (
	SecretFieldUpstreamAPIKeys          SecretField = "llm_upstreams.api_keys_enc"
	SecretFieldPersonalProviderAPIKey   SecretField = "llm_user_providers.api_key_enc"
	SecretFieldMCPAuthToken             SecretField = "mcp_servers.auth_token_enc"
	SecretFieldIdentityProviderSecret   SecretField = "identity_providers.client_secret_encrypted"
	SecretFieldTOTPSecret               SecretField = "identity_mfa_settings.totp_secret_encrypted"
	SecretFieldModerationText           SecretField = "content_moderation_events.encrypted_text"
	SecretFieldSystemSetting            SecretField = "system_settings.value"
	SecretFieldRedemptionCodePlaintext  SecretField = "billing_redemption_codes.code_encrypted"
	SecretFieldModerationIsolatedImages SecretField = "content_moderation.isolated_images"
)

// StoredSecret 是一条非空的密文记录。
// OwnerUserID 与 PublicID 只在密文绑定到记录的字段（个人服务商 API Key）上填写，用于还原绑定上下文。
type StoredSecret struct {
	ID          uint
	Value       string
	OwnerUserID uint
	PublicID    string
}

// SettingKey 定位一个系统配置项。
type SettingKey struct {
	Namespace string
	Key       string
}

// StoredSettingSecret 是一条加密存储的系统配置。
type StoredSettingSecret struct {
	ID        uint
	Namespace string
	Key       string
	Value     string
}

// StoredRedemptionCode 是兑换码的查找索引与加密原文。CodeEncrypted 为空表示早期兑换码未保存原文。
type StoredRedemptionCode struct {
	ID            uint
	CodeHash      string
	CodeEncrypted string
}

// SecretRotationRepository 支持把存量密文改用当前主密钥重新加密。
// 所有替换都以「仍是读取时的值」为条件，返回 false 表示该记录已被其他实例或请求改写。
// 遍历包含已软删除的记录：它们仍在库里，同样依赖旧主密钥。
type SecretRotationRepository interface {
	ListSecrets(ctx context.Context, field SecretField, afterID uint, limit int) ([]StoredSecret, error)
	ReplaceSecret(ctx context.Context, field SecretField, id uint, previous string, next string) (bool, error)
	ListSettingSecrets(ctx context.Context, keys []SettingKey) ([]StoredSettingSecret, error)
	ListRedemptionCodes(ctx context.Context, afterID uint, limit int) ([]StoredRedemptionCode, error)
	ReplaceRedemptionCode(ctx context.Context, id uint, previous StoredRedemptionCode, nextHash string, nextEncrypted string) (bool, error)
	// LatestIsolatedImageExpiry 返回尚未过期的隔离图片中最晚的过期时间；没有时返回 nil。
	LatestIsolatedImageExpiry(ctx context.Context, now time.Time) (*time.Time, error)
}
