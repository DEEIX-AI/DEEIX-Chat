package conversation

import (
	"context"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
)

// CreateContextArtifacts 批量写入本轮上下文证据。
func (r *Repo) CreateContextArtifacts(ctx context.Context, items []domainconversation.ContextArtifact) error {
	if len(items) == 0 {
		return nil
	}
	entities := make([]models.ChatContextRecord, 0, len(items))
	for _, item := range items {
		entities = append(entities, toContextArtifactModel(item))
	}
	if err := r.db.WithContext(ctx).Create(&entities).Error; err != nil {
		return translateError(err)
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
		return nil, translateError(err)
	}
	result := toContextArtifactDomain(item)
	return &result, nil
}

// ListContextArtifactsByMessage 查询单条用户消息对应的上下文证据。
func (r *Repo) ListContextArtifactsByMessage(ctx context.Context, conversationID uint, messageID uint) ([]domainconversation.ContextArtifact, error) {
	items := make([]models.ChatContextRecord, 0)
	if err := r.db.WithContext(ctx).
		Where("record_type = ? AND conversation_id = ? AND message_id = ?", chatContextRecordArtifact, conversationID, messageID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, translateError(err)
	}
	return toContextArtifactDomains(items), nil
}

// ListRecentContextArtifacts 按会话和类型查询最近的上下文证据。
func (r *Repo) ListRecentContextArtifacts(ctx context.Context, conversationID uint, kinds []domainconversation.ContextArtifactKind, limit int) ([]domainconversation.ContextArtifact, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	query := r.db.WithContext(ctx).
		Where("record_type = ? AND conversation_id = ?", chatContextRecordArtifact, conversationID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Order("id DESC").
		Limit(limit)
	if len(kinds) > 0 {
		values := make([]string, 0, len(kinds))
		for _, kind := range kinds {
			if kind != "" {
				values = append(values, string(kind))
			}
		}
		if len(values) > 0 {
			query = query.Where("kind IN ?", values)
		}
	}
	items := make([]models.ChatContextRecord, 0)
	if err := query.Find(&items).Error; err != nil {
		return nil, translateError(err)
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
		return 0, translateError(err)
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).
		Unscoped().
		Where("id IN ?", ids).
		Delete(&models.ChatContextRecord{})
	if result.Error != nil {
		return 0, translateError(result.Error)
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
