package conversation

import (
	"context"
	"strings"
	"time"

	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateForkedConversation 在一个事务内创建 fork 会话、重建消息父链并复制仍有效的附件引用。
func (r *Repo) CreateForkedConversation(ctx context.Context, input repository.CreateForkedConversationInput) error {
	if input.Conversation == nil ||
		input.SourceConversationID == 0 ||
		input.Conversation.UserID == 0 ||
		strings.TrimSpace(input.Conversation.PublicID) == "" ||
		strings.TrimSpace(input.Conversation.SessionKey) == "" ||
		len(input.Messages) == 0 ||
		len(input.Messages) > maxAncestorQueryDepth {
		return repository.ErrInvalidInput
	}

	sourceMessageIDs := make([]uint, 0, len(input.Messages))
	seenSourceMessageIDs := make(map[uint]struct{}, len(input.Messages))
	for _, item := range input.Messages {
		if item.SourceMessageID == 0 || strings.TrimSpace(item.Message.PublicID) == "" {
			return repository.ErrInvalidInput
		}
		if _, exists := seenSourceMessageIDs[item.SourceMessageID]; exists {
			return repository.ErrInvalidInput
		}
		seenSourceMessageIDs[item.SourceMessageID] = struct{}{}
		sourceMessageIDs = append(sourceMessageIDs, item.SourceMessageID)
	}

	target := *input.Conversation
	target.MessageCount = len(input.Messages)
	var createdConversation models.Conversation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sourceConversation models.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ? AND user_id = ?", input.SourceConversationID, target.UserID).
			First(&sourceConversation).Error; err != nil {
			return err
		}

		lockedSourceMessages := make([]models.Message, 0, len(sourceMessageIDs))
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("conversation_id = ? AND user_id = ? AND id IN ?", input.SourceConversationID, target.UserID, sourceMessageIDs).
			Find(&lockedSourceMessages).Error; err != nil {
			return err
		}
		if len(lockedSourceMessages) != len(sourceMessageIDs) {
			return repository.ErrNotFound
		}

		createdConversation = toConversationModel(&target)
		if err := tx.Create(&createdConversation).Error; err != nil {
			return err
		}

		targetMessageIDs := make(map[uint]uint, len(input.Messages))
		for _, item := range input.Messages {
			message := item.Message
			message.ID = 0
			message.ConversationID = createdConversation.ID
			message.UserID = target.UserID
			message.ParentMessageID = nil
			if item.SourceParentMessageID != nil {
				parentID, exists := targetMessageIDs[*item.SourceParentMessageID]
				if !exists {
					return repository.ErrInvalidInput
				}
				message.ParentMessageID = &parentID
			}

			entity := toMessageModel(&message)
			if err := tx.Create(&entity).Error; err != nil {
				return err
			}
			targetMessageIDs[item.SourceMessageID] = entity.ID
		}

		return cloneForkedAttachments(tx, target.UserID, input.SourceConversationID, createdConversation.ID, targetMessageIDs)
	})
	if err != nil {
		return translateError(err)
	}

	*input.Conversation = toConversationDomain(createdConversation)
	return nil
}

func cloneForkedAttachments(
	tx *gorm.DB,
	userID uint,
	sourceConversationID uint,
	targetConversationID uint,
	targetMessageIDs map[uint]uint,
) error {
	sourceMessageIDs := make([]uint, 0, len(targetMessageIDs))
	for sourceMessageID := range targetMessageIDs {
		sourceMessageIDs = append(sourceMessageIDs, sourceMessageID)
	}

	sourceAttachments := make([]models.Attachment, 0)
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("conversation_id = ? AND user_id = ? AND message_id IN ? AND status <> ?", sourceConversationID, userID, sourceMessageIDs, "deleted").
		Order("id ASC").
		Find(&sourceAttachments).Error; err != nil {
		return err
	}
	if len(sourceAttachments) == 0 {
		return nil
	}

	fileIDs := make([]string, 0, len(sourceAttachments))
	seenFileIDs := make(map[string]struct{}, len(sourceAttachments))
	for _, item := range sourceAttachments {
		fileID := strings.TrimSpace(item.FileID)
		if fileID == "" {
			continue
		}
		if _, exists := seenFileIDs[fileID]; exists {
			continue
		}
		seenFileIDs[fileID] = struct{}{}
		fileIDs = append(fileIDs, fileID)
	}
	if len(fileIDs) == 0 {
		return nil
	}

	activeFiles := make([]models.FileObject, 0, len(fileIDs))
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND status = ? AND file_id IN ?", userID, "active", fileIDs).
		Find(&activeFiles).Error; err != nil {
		return err
	}
	filesByID := make(map[string]models.FileObject, len(activeFiles))
	for _, item := range activeFiles {
		filesByID[item.FileID] = item
	}

	now := time.Now().UTC()
	attachments := make([]models.Attachment, 0, len(sourceAttachments))
	for _, source := range sourceAttachments {
		file, exists := filesByID[strings.TrimSpace(source.FileID)]
		if !exists {
			continue
		}
		targetMessageID, exists := targetMessageIDs[source.MessageID]
		if !exists {
			return repository.ErrInvalidInput
		}
		kind := strings.TrimSpace(source.Kind)
		if kind == "" {
			kind = "file"
		}
		fileName := strings.TrimSpace(source.FileName)
		if fileName == "" {
			fileName = file.FileName
		}
		mimeType := strings.TrimSpace(source.MimeType)
		if mimeType == "" {
			mimeType = file.MimeType
		}
		fileSize := source.FileSize
		if fileSize <= 0 {
			fileSize = file.SizeBytes
		}
		attachments = append(attachments, models.Attachment{
			ConversationID: targetConversationID,
			MessageID:      targetMessageID,
			UserID:         userID,
			FileID:         file.FileID,
			Kind:           kind,
			FileName:       fileName,
			MimeType:       mimeType,
			FileSize:       fileSize,
			SHA256:         file.SHA256,
			StoragePath:    file.StoragePath,
			Status:         "active",
			MetaJSON:       source.MetaJSON,
			UploadedAt:     now,
		})
	}
	if len(attachments) == 0 {
		return nil
	}
	return tx.Create(&attachments).Error
}

var _ repository.ConversationForkRepository = (*Repo)(nil)
