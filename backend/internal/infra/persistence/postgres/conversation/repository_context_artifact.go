package conversation

import (
	"context"
	"strings"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// CreateContextArtifacts 批量写入本轮上下文证据。
func (r *Repo) CreateContextArtifacts(ctx context.Context, items []domainconversation.ContextArtifact) error {
	if len(items) == 0 {
		return nil
	}
	entities := make([]models.ChatContextRecord, 0, len(items))
	for _, item := range items {
		item.RunID = strings.TrimSpace(item.RunID)
		if item.ConversationID == 0 || item.MessageID == 0 || item.UserID == 0 || item.RunID == "" {
			return repository.ErrInvalidInput
		}
		entities = append(entities, toContextArtifactModel(item))
	}
	if err := r.db.WithContext(ctx).Create(&entities).Error; err != nil {
		return dberror.Translate(err)
	}
	for index := range items {
		items[index] = toContextArtifactDomain(entities[index])
	}
	return nil
}

// GetContextArtifactByIDForUser 查询当前用户可访问的上下文证据。
func (r *Repo) GetContextArtifactByIDForUser(ctx context.Context, userID uint, artifactID uint) (*domainconversation.ContextArtifact, error) {
	var item models.ChatContextRecord
	if err := r.db.WithContext(ctx).
		Where("record_type = ? AND id = ? AND user_id = ?", chatContextRecordArtifact, artifactID, userID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		First(&item).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	result := toContextArtifactDomain(item)
	return &result, nil
}

// ListRecentContextArtifacts 在当前活跃分支内按类型查询最近的上下文证据。
func (r *Repo) ListRecentContextArtifacts(ctx context.Context, filter repository.ContextArtifactListFilter) ([]domainconversation.ContextArtifact, error) {
	if !filter.Scope.Valid() {
		return nil, nil
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	scopeQuery := historicalMessageScopeSubquery(r.db.WithContext(ctx), filter.Scope)
	query := r.db.WithContext(ctx).
		Model(&models.ChatContextRecord{}).
		Select("chat_context_records.*").
		Joins(`JOIN chat_messages AS artifact_owner
			ON artifact_owner.id = chat_context_records.message_id
			AND artifact_owner.conversation_id = chat_context_records.conversation_id
			AND artifact_owner.user_id = chat_context_records.user_id
			AND artifact_owner.run_id = chat_context_records.run_id
			AND artifact_owner.role = ?
			AND artifact_owner.deleted_at IS NULL`, "assistant").
		Where("chat_context_records.record_type = ? AND chat_context_records.conversation_id = ? AND chat_context_records.user_id = ?", chatContextRecordArtifact, filter.Scope.ConversationID, filter.Scope.UserID).
		Where("chat_context_records.message_id IN (?)", scopeQuery).
		Where("chat_context_records.expires_at IS NULL OR chat_context_records.expires_at > ?", time.Now()).
		Order("chat_context_records.id DESC").
		Limit(filter.Limit)
	if len(filter.Kinds) > 0 {
		values := make([]string, 0, len(filter.Kinds))
		for _, kind := range filter.Kinds {
			if kind != "" {
				values = append(values, string(kind))
			}
		}
		if len(values) > 0 {
			query = query.Where("chat_context_records.kind IN ?", values)
		}
	}
	items := make([]models.ChatContextRecord, 0)
	if err := query.Find(&items).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	return toContextArtifactDomains(items), nil
}

// DeleteExpiredContextArtifacts 硬删除已过期上下文证据，避免长期堆积用户证据文本。
func (r *Repo) DeleteExpiredContextArtifacts(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	ids := make([]uint, 0, limit)
	if err := r.db.WithContext(ctx).
		Model(&models.ChatContextRecord{}).
		Where("record_type = ? AND expires_at IS NOT NULL AND expires_at <= ?", chatContextRecordArtifact, before).
		Order("expires_at ASC").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return 0, dberror.Translate(err)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).
		Unscoped().
		Where("id IN ?", ids).
		Delete(&models.ChatContextRecord{})
	if result.Error != nil {
		return 0, dberror.Translate(result.Error)
	}
	return result.RowsAffected, nil
}

func toContextArtifactDomain(item models.ChatContextRecord) domainconversation.ContextArtifact {
	return domainconversation.ContextArtifact{
		ID:             item.ID,
		ConversationID: item.ConversationID,
		MessageID:      item.MessageID,
		UserID:         item.UserID,
		RunID:          item.RunID,
		Kind:           domainconversation.ContextArtifactKind(item.Kind),
		SourceType:     item.SourceType,
		SourceID:       item.SourceID,
		SourceTitle:    item.SourceTitle,
		Content:        item.Content,
		ContentHash:    item.ContentHash,
		TokenEstimate:  item.TokenEstimate,
		Score:          item.Score,
		MetadataJSON:   item.MetadataJSON,
		ExpiresAt:      item.ExpiresAt,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func toContextArtifactDomains(items []models.ChatContextRecord) []domainconversation.ContextArtifact {
	results := make([]domainconversation.ContextArtifact, 0, len(items))
	for _, item := range items {
		results = append(results, toContextArtifactDomain(item))
	}
	return results
}

func toContextArtifactModel(item domainconversation.ContextArtifact) models.ChatContextRecord {
	return models.ChatContextRecord{
		RecordType:     chatContextRecordArtifact,
		ConversationID: item.ConversationID,
		MessageID:      item.MessageID,
		UserID:         item.UserID,
		RunID:          item.RunID,
		Kind:           string(item.Kind),
		SourceType:     item.SourceType,
		SourceID:       item.SourceID,
		SourceTitle:    item.SourceTitle,
		Content:        item.Content,
		ContentHash:    item.ContentHash,
		TokenEstimate:  item.TokenEstimate,
		Score:          item.Score,
		MetadataJSON:   item.MetadataJSON,
		ExpiresAt:      item.ExpiresAt,
	}
}
