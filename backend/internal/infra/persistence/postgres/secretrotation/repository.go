// Package secretrotation 遍历并原地替换各表中以 secretbox 载荷存储的敏感字段。
package secretrotation

import (
	"context"
	"fmt"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// secretColumns 把字段映射到表与列。列名来自固定白名单，不接受外部输入。
var secretColumns = map[repository.SecretField]struct{ table, column string }{
	repository.SecretFieldUpstreamAPIKeys:        {"llm_upstreams", "api_keys_enc"},
	repository.SecretFieldPersonalProviderAPIKey: {"llm_user_providers", "api_key_enc"},
	repository.SecretFieldMCPAuthToken:           {"mcp_servers", "auth_token_enc"},
	repository.SecretFieldIdentityProviderSecret: {"identity_providers", "client_secret_encrypted"},
	repository.SecretFieldTOTPSecret:             {"identity_mfa_settings", "totp_secret_encrypted"},
	repository.SecretFieldModerationText:         {"content_moderation_events", "encrypted_text"},
	repository.SecretFieldSystemSetting:          {"system_settings", "value"},
}

// bindingColumns 列出密文绑定到记录的字段还需读取的列，见 repository.StoredSecret。
var bindingColumns = map[repository.SecretField]string{
	repository.SecretFieldPersonalProviderAPIKey: ", owner_user_id, public_id",
}

// Repo 实现 repository.SecretRotationRepository。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建密文轮换仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func lookupColumn(field repository.SecretField) (string, string, error) {
	target, ok := secretColumns[field]
	if !ok {
		return "", "", fmt.Errorf("secret rotation: unsupported field %q", field)
	}
	return target.table, target.column, nil
}

// ListSecrets 按主键顺序分页列出非空密文；使用 Table 而非模型，因此包含已软删除的记录。
func (r *Repo) ListSecrets(ctx context.Context, field repository.SecretField, afterID uint, limit int) ([]repository.StoredSecret, error) {
	table, column, err := lookupColumn(field)
	if err != nil {
		return nil, err
	}
	var rows []repository.StoredSecret
	err = r.db.WithContext(ctx).
		Table(table).
		Select("id, "+column+" AS value"+bindingColumns[field]).
		Where("id > ? AND "+column+" <> ''", afterID).
		Order("id ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, dberror.Translate(err)
}

// ReplaceSecret 仅在字段仍为 previous 时写入 next；不改动 updated_at，轮换不应表现为用户修改。
func (r *Repo) ReplaceSecret(ctx context.Context, field repository.SecretField, id uint, previous string, next string) (bool, error) {
	table, column, err := lookupColumn(field)
	if err != nil {
		return false, err
	}
	result := r.db.WithContext(ctx).
		Table(table).
		Where("id = ? AND "+column+" = ?", id, previous).
		UpdateColumn(column, next)
	if result.Error != nil {
		return false, dberror.Translate(result.Error)
	}
	return result.RowsAffected == 1, nil
}

// ListSettingSecrets 列出指定配置项中非空的值。
func (r *Repo) ListSettingSecrets(ctx context.Context, keys []repository.SettingKey) ([]repository.StoredSettingSecret, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	query := r.db.WithContext(ctx).Table("system_settings").Where("value <> ''")
	match := r.db.Where("1 = 0")
	for _, key := range keys {
		match = match.Or("namespace = ? AND key = ?", key.Namespace, key.Key)
	}
	var rows []repository.StoredSettingSecret
	err := query.Where(match).
		Select("id, namespace, key, value").
		Order("id ASC").
		Scan(&rows).Error
	return rows, dberror.Translate(err)
}

// ListRedemptionCodes 按主键顺序分页列出兑换码的索引与加密原文（包含已软删除的记录）。
func (r *Repo) ListRedemptionCodes(ctx context.Context, afterID uint, limit int) ([]repository.StoredRedemptionCode, error) {
	var rows []repository.StoredRedemptionCode
	err := r.db.WithContext(ctx).
		Table("billing_redemption_codes").
		Select("id, code_hash, code_encrypted").
		Where("id > ?", afterID).
		Order("id ASC").
		Limit(limit).
		Scan(&rows).Error
	return rows, dberror.Translate(err)
}

// ReplaceRedemptionCode 同时替换查找索引与加密原文，两者须一起更新才能在移除旧主密钥后继续兑换。
func (r *Repo) ReplaceRedemptionCode(ctx context.Context, id uint, previous repository.StoredRedemptionCode, nextHash string, nextEncrypted string) (bool, error) {
	result := r.db.WithContext(ctx).
		Table("billing_redemption_codes").
		Where("id = ? AND code_hash = ? AND code_encrypted = ?", id, previous.CodeHash, previous.CodeEncrypted).
		UpdateColumns(map[string]any{"code_hash": nextHash, "code_encrypted": nextEncrypted})
	if result.Error != nil {
		return false, dberror.Translate(result.Error)
	}
	return result.RowsAffected == 1, nil
}

// LatestIsolatedImageExpiry 返回尚未过期、且仍有隔离图片的审核事件中最晚的原文过期时间。
// 取列本身而非 MAX()：SQLite 的聚合结果会丢失时间类型，无法扫描为 time.Time。
func (r *Repo) LatestIsolatedImageExpiry(ctx context.Context, now time.Time) (*time.Time, error) {
	var rows []struct{ ContentExpiresAt time.Time }
	err := r.db.WithContext(ctx).
		Table("content_moderation_events").
		Select("content_expires_at").
		Where("deleted_at IS NULL AND modality = ? AND image_meta_json NOT IN ('', '[]') AND content_expires_at > ?", "image", now).
		Order("content_expires_at DESC").
		Limit(1).
		Scan(&rows).Error
	if err != nil {
		return nil, dberror.Translate(err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0].ContentExpiresAt, nil
}
