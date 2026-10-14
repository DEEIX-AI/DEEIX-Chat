package conversation

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/dberror"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// maxUIStateBlocksPerMessage 是单条消息可保存的组件块数量上限，与应用层的同名常量一致。
const maxUIStateBlocksPerMessage = 32

// maxUIStateBlockIDLength 是组件块 ID 的长度上限，与提示词要求的“简短英文 id”一致。
const maxUIStateBlockIDLength = 64

// uiStateBlockIDPattern 与提示词要求的“简短英文 id”一致；应用层已按同一规则校验，
// 这里作为写库前的最后一道防线，避免脏块 ID 影响后续按块合并与注入。
var uiStateBlockIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// UpsertMessageUIStates 按块写入消息的交互式组件状态；StateJSON 为空的块表示删除。
// 块数量上限在事务内复核，避免并发写入越过上限。
func (r *Repo) UpsertMessageUIStates(
	ctx context.Context,
	userID uint,
	conversationID uint,
	messageID uint,
	blocks map[string]string,
) error {
	if userID == 0 || conversationID == 0 || messageID == 0 || len(blocks) == 0 {
		return repository.ErrInvalidInput
	}

	removals := make([]string, 0, len(blocks))
	writes := make(map[string]string, len(blocks))
	for blockID, stateJSON := range blocks {
		normalizedBlockID := strings.TrimSpace(blockID)
		if normalizedBlockID == "" || len(normalizedBlockID) > maxUIStateBlockIDLength || !uiStateBlockIDPattern.MatchString(normalizedBlockID) {
			return repository.ErrMessageUIStateInvalid
		}
		if strings.TrimSpace(stateJSON) == "" {
			removals = append(removals, normalizedBlockID)
			continue
		}
		writes[normalizedBlockID] = stateJSON
	}

	// 本次请求涉及的块（写入与删除）都不计入“已有块数”：被覆盖的块不会新增行，被删除的块会腾出名额。
	touched := make([]string, 0, len(blocks))
	touched = append(touched, removals...)
	for blockID := range writes {
		touched = append(touched, blockID)
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先锁消息行再计数：同一条消息的并发写入在此串行化，块数上限不会被并发请求越过；
		// 同时确认消息仍然存在且归属当前用户与会话（消息可能在请求途中被删除）。
		var message models.Message
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ? AND conversation_id = ? AND user_id = ?", messageID, conversationID, userID).
			First(&message).Error; err != nil {
			return err
		}

		var untouched int64
		if err := tx.Model(&models.ConversationMessageUIState{}).
			Where("message_id = ? AND block_id NOT IN ?", messageID, touched).
			Count(&untouched).Error; err != nil {
			return err
		}
		if untouched+int64(len(writes)) > maxUIStateBlocksPerMessage {
			return repository.ErrMessageUIStateLimitExceeded
		}

		if len(removals) > 0 {
			if err := tx.
				Where("user_id = ? AND message_id = ? AND block_id IN ?", userID, messageID, removals).
				Delete(&models.ConversationMessageUIState{}).Error; err != nil {
				return err
			}
		}
		if len(writes) == 0 {
			return nil
		}

		rows := make([]models.ConversationMessageUIState, 0, len(writes))
		for blockID, stateJSON := range writes {
			rows = append(rows, models.ConversationMessageUIState{
				UserID:         userID,
				ConversationID: conversationID,
				MessageID:      messageID,
				BlockID:        blockID,
				StateJSON:      stateJSON,
			})
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "message_id"},
				{Name: "block_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"user_id",
				"conversation_id",
				"state_json",
				"updated_at",
			}),
		}).Create(&rows).Error
	})
	if err != nil {
		if errors.Is(err, repository.ErrMessageUIStateLimitExceeded) {
			return err
		}
		return dberror.Translate(err)
	}
	return nil
}

// GetMessageUIStateMap 批量读取消息的组件状态，按消息 ID 聚合块状态。
func (r *Repo) GetMessageUIStateMap(
	ctx context.Context,
	userID uint,
	messageIDs []uint,
) (map[uint]map[string]string, error) {
	result := make(map[uint]map[string]string, len(messageIDs))
	if len(messageIDs) == 0 {
		return result, nil
	}

	items := make([]models.ConversationMessageUIState, 0, len(messageIDs))
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND message_id IN ?", userID, messageIDs).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, dberror.Translate(err)
	}
	for _, item := range items {
		blockID := strings.TrimSpace(item.BlockID)
		stateJSON := strings.TrimSpace(item.StateJSON)
		if blockID == "" || stateJSON == "" {
			continue
		}
		blocks := result[item.MessageID]
		if blocks == nil {
			blocks = make(map[string]string, 1)
			result[item.MessageID] = blocks
		}
		blocks[blockID] = stateJSON
	}
	return result, nil
}

// cloneForkedMessageUIStates 在 fork 事务内把源消息的组件状态复制到新消息上。
func cloneForkedMessageUIStates(
	tx *gorm.DB,
	userID uint,
	sourceConversationID uint,
	targetConversationID uint,
	targetMessageIDs map[uint]uint,
) error {
	if len(targetMessageIDs) == 0 {
		return nil
	}
	sourceMessageIDs := make([]uint, 0, len(targetMessageIDs))
	for sourceMessageID := range targetMessageIDs {
		sourceMessageIDs = append(sourceMessageIDs, sourceMessageID)
	}

	rows := make([]models.ConversationMessageUIState, 0, len(sourceMessageIDs))
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("conversation_id = ? AND user_id = ? AND message_id IN ?", sourceConversationID, userID, sourceMessageIDs).
		Order("id ASC").
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	clones := make([]models.ConversationMessageUIState, 0, len(rows))
	for _, row := range rows {
		targetMessageID, exists := targetMessageIDs[row.MessageID]
		if !exists || targetMessageID == 0 {
			continue
		}
		clones = append(clones, models.ConversationMessageUIState{
			UserID:         userID,
			ConversationID: targetConversationID,
			MessageID:      targetMessageID,
			BlockID:        row.BlockID,
			StateJSON:      row.StateJSON,
		})
	}
	if len(clones) == 0 {
		return nil
	}
	return tx.Create(&clones).Error
}
