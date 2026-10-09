// Package personalprovider 提供用户自带 Key 模型服务的数据访问。
package personalprovider

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	domainpersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/personalprovider"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"gorm.io/gorm"
)

// maxBulkPublicIDs 限制单次批量操作的条数，避免超长 IN 列表。
const maxBulkPublicIDs = 500

// Repo 封装个人模型服务数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

// ListByOwner 按创建顺序列出用户自己的服务。
func (r *Repo) ListByOwner(ctx context.Context, ownerUserID uint) ([]domainpersonalprovider.Provider, error) {
	if ownerUserID == 0 {
		return nil, repository.ErrInvalidInput
	}
	var records []models.LLMUserProvider
	if err := r.db.WithContext(ctx).
		Where("owner_user_id = ?", ownerUserID).
		Order("id ASC").
		Find(&records).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return toDomainList(records), nil
}

// CountByOwner 返回用户已有服务数量（含停用），用于数量上限校验。
func (r *Repo) CountByOwner(ctx context.Context, ownerUserID uint) (int64, error) {
	if ownerUserID == 0 {
		return 0, repository.ErrInvalidInput
	}
	var total int64
	if err := r.db.WithContext(ctx).Model(&models.LLMUserProvider{}).
		Where("owner_user_id = ?", ownerUserID).
		Count(&total).Error; err != nil {
		return 0, dberror.Translate(err)
	}
	return total, nil
}

// GetByOwner 只返回属于 ownerUserID 的服务；不属于该用户时与不存在同样返回 ErrNotFound。
func (r *Repo) GetByOwner(ctx context.Context, ownerUserID uint, publicID string) (*domainpersonalprovider.Provider, error) {
	publicID = strings.TrimSpace(publicID)
	if ownerUserID == 0 || publicID == "" {
		return nil, repository.ErrNotFound
	}
	var record models.LLMUserProvider
	if err := r.db.WithContext(ctx).
		Where("owner_user_id = ? AND public_id = ?", ownerUserID, publicID).
		First(&record).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	result := toDomain(record)
	return &result, nil
}

// Create 新建服务。
func (r *Repo) Create(ctx context.Context, item *domainpersonalprovider.Provider) (*domainpersonalprovider.Provider, error) {
	if item == nil || item.OwnerUserID == 0 || strings.TrimSpace(item.PublicID) == "" {
		return nil, repository.ErrInvalidInput
	}
	modelsJSON, err := encodeModels(item.Models)
	if err != nil {
		return nil, err
	}
	record := models.LLMUserProvider{
		PublicID:      strings.TrimSpace(item.PublicID),
		OwnerUserID:   item.OwnerUserID,
		Name:          item.Name,
		Icon:          item.Icon,
		Protocol:      item.Protocol,
		BaseURL:       item.BaseURL,
		Host:          item.Host,
		APIKeyEnc:     item.APIKeyEnc,
		KeyHint:       item.KeyHint,
		ModelsJSON:    modelsJSON,
		Status:        item.Status,
		Source:        item.Source,
		LastError:     item.LastError,
		LastCheckedAt: item.LastCheckedAt,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	result := toDomain(record)
	return &result, nil
}

// UpdateByOwner 更新属于 ownerUserID 的服务；不属于该用户时返回 ErrNotFound。
func (r *Repo) UpdateByOwner(ctx context.Context, ownerUserID uint, publicID string, patch repository.PersonalProviderPatch) (*domainpersonalprovider.Provider, error) {
	updates := map[string]any{}
	if patch.Name != nil {
		updates["name"] = *patch.Name
	}
	if patch.Icon != nil {
		updates["icon"] = *patch.Icon
	}
	if patch.APIKeyEnc != nil {
		updates["api_key_enc"] = *patch.APIKeyEnc
	}
	if patch.KeyHint != nil {
		updates["key_hint"] = *patch.KeyHint
	}
	if patch.Models != nil {
		modelsJSON, err := encodeModels(*patch.Models)
		if err != nil {
			return nil, err
		}
		updates["models_json"] = modelsJSON
	}
	if patch.Status != nil {
		updates["status"] = *patch.Status
	}
	if patch.LastError != nil {
		updates["last_error"] = *patch.LastError
	}
	if patch.LastCheckedAt != nil {
		updates["last_checked_at"] = *patch.LastCheckedAt
	}
	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		result := r.db.WithContext(ctx).Model(&models.LLMUserProvider{}).
			Where("owner_user_id = ? AND public_id = ?", ownerUserID, strings.TrimSpace(publicID)).
			Updates(updates)
		if result.Error != nil {
			return nil, dberror.Translate(result.Error)
		}
		if result.RowsAffected == 0 {
			return nil, repository.ErrNotFound
		}
	}
	return r.GetByOwner(ctx, ownerUserID, publicID)
}

// DeleteByOwner 删除属于 ownerUserID 的服务；不属于该用户时返回 ErrNotFound。
func (r *Repo) DeleteByOwner(ctx context.Context, ownerUserID uint, publicID string) error {
	result := r.db.WithContext(ctx).
		Where("owner_user_id = ? AND public_id = ?", ownerUserID, strings.TrimSpace(publicID)).
		Delete(&models.LLMUserProvider{})
	if result.Error != nil {
		return dberror.Translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// ListForAdmin 供管理员分页检索。返回的领域对象包含密文字段，调用方不得向外输出。
func (r *Repo) ListForAdmin(ctx context.Context, filter repository.PersonalProviderAdminFilter, offset int, limit int) ([]domainpersonalprovider.Provider, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > pagination.MaxPageSize {
		limit = pagination.MaxPageSize
	}
	query := r.db.WithContext(ctx).Model(&models.LLMUserProvider{})
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if host := strings.ToLower(strings.TrimSpace(filter.Host)); host != "" {
		query = query.Where("host = ?", host)
	}
	if filter.OwnerUserID > 0 {
		query = query.Where("owner_user_id = ?", filter.OwnerUserID)
	}
	if keyword := strings.ToLower(strings.TrimSpace(filter.Query)); keyword != "" {
		like := "%" + keyword + "%"
		// 同时按所属用户（用户名、显示名、邮箱）搜索，管理员通常从用户找到他的服务。
		query = query.Where(`LOWER(name) LIKE ? OR LOWER(host) LIKE ? OR owner_user_id IN (
			SELECT users.id FROM identity_users AS users
			WHERE users.deleted_at IS NULL
				AND (LOWER(users.username) LIKE ? OR LOWER(users.display_name) LIKE ? OR LOWER(users.email) LIKE ?)
		)`, like, like, like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	var records []models.LLMUserProvider
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&records).Error; err != nil {
		return nil, 0, dberror.Translate(err)
	}
	return toDomainList(records), total, nil
}

// SetStatusByPublicIDs 批量修改状态，返回实际影响的条数。
func (r *Repo) SetStatusByPublicIDs(ctx context.Context, publicIDs []string, status string) (int64, error) {
	ids := normalizePublicIDs(publicIDs)
	if len(ids) == 0 {
		return 0, nil
	}
	if len(ids) > maxBulkPublicIDs {
		return 0, repository.ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Model(&models.LLMUserProvider{}).
		Where("public_id IN ?", ids).
		Updates(map[string]any{"status": status, "updated_at": time.Now()})
	if result.Error != nil {
		return 0, dberror.Translate(result.Error)
	}
	return result.RowsAffected, nil
}

// SetStatusByHost 按主机名批量修改状态，用于管理员一键停用某个域名下的全部服务。
func (r *Repo) SetStatusByHost(ctx context.Context, host string, status string) (int64, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return 0, repository.ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Model(&models.LLMUserProvider{}).
		Where("host = ?", host).
		Updates(map[string]any{"status": status, "updated_at": time.Now()})
	if result.Error != nil {
		return 0, dberror.Translate(result.Error)
	}
	return result.RowsAffected, nil
}

// DeleteByPublicIDs 批量删除，返回实际删除条数。
func (r *Repo) DeleteByPublicIDs(ctx context.Context, publicIDs []string) (int64, error) {
	ids := normalizePublicIDs(publicIDs)
	if len(ids) == 0 {
		return 0, nil
	}
	if len(ids) > maxBulkPublicIDs {
		return 0, repository.ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Where("public_id IN ?", ids).Delete(&models.LLMUserProvider{})
	if result.Error != nil {
		return 0, dberror.Translate(result.Error)
	}
	return result.RowsAffected, nil
}

func normalizePublicIDs(publicIDs []string) []string {
	seen := make(map[string]struct{}, len(publicIDs))
	ids := make([]string, 0, len(publicIDs))
	for _, raw := range publicIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

// storedModel 是 models_json 的条目。早期版本只保存模型名字符串，读取时按服务协议补齐。
type storedModel struct {
	Name      string   `json:"name"`
	Protocols []string `json:"protocols"`
}

func encodeModels(items []domainpersonalprovider.Model) (string, error) {
	stored := make([]storedModel, 0, len(items))
	for _, item := range items {
		protocols := item.Protocols
		if protocols == nil {
			protocols = []string{}
		}
		stored = append(stored, storedModel{Name: item.Name, Protocols: protocols})
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func decodeModels(raw string, providerProtocol string) []domainpersonalprovider.Model {
	var entries []json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &entries); err != nil {
		return []domainpersonalprovider.Model{}
	}
	models := make([]domainpersonalprovider.Model, 0, len(entries))
	for _, entry := range entries {
		var name string
		if err := json.Unmarshal(entry, &name); err == nil {
			models = append(models, domainpersonalprovider.Model{Name: name, Protocols: []string{providerProtocol}})
			continue
		}
		var item storedModel
		if err := json.Unmarshal(entry, &item); err != nil || item.Name == "" {
			continue
		}
		if len(item.Protocols) == 0 {
			item.Protocols = []string{providerProtocol}
		}
		models = append(models, domainpersonalprovider.Model{Name: item.Name, Protocols: item.Protocols})
	}
	return models
}

func toDomainList(records []models.LLMUserProvider) []domainpersonalprovider.Provider {
	result := make([]domainpersonalprovider.Provider, 0, len(records))
	for _, record := range records {
		result = append(result, toDomain(record))
	}
	return result
}

func toDomain(record models.LLMUserProvider) domainpersonalprovider.Provider {
	return domainpersonalprovider.Provider{
		ID:            record.ID,
		PublicID:      record.PublicID,
		OwnerUserID:   record.OwnerUserID,
		Name:          record.Name,
		Icon:          record.Icon,
		Protocol:      record.Protocol,
		BaseURL:       record.BaseURL,
		Host:          record.Host,
		APIKeyEnc:     record.APIKeyEnc,
		KeyHint:       record.KeyHint,
		Models:        decodeModels(record.ModelsJSON, record.Protocol),
		Status:        record.Status,
		Source:        record.Source,
		LastError:     record.LastError,
		LastCheckedAt: record.LastCheckedAt,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}
}
