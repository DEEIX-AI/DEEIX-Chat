package conversation

import (
	"context"
	"strings"

	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DeleteMessageAndReparentChildren 在一个事务内软删除指定消息并把其子消息重接到其父
// 消息上（splice 语义），后续消息保持可见；同时递减会话消息数，并失效覆盖边界落在
// 被删消息子树内的压缩快照。返回被重接的子消息数量。
//
// 锁顺序与 fork、会话删除保持一致（先会话行再消息行），发送路径同样以会话行为
// 首把锁，两侧不会形成 AB-BA 死锁。
func (r *Repo) DeleteMessageAndReparentChildren(ctx context.Context, userID uint, conversationID uint, messageID uint) (int64, error) {
	var reparented int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation models.Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ? AND user_id = ?", conversationID, userID).
			First(&conversation).Error; err != nil {
			return err
		}

		var target models.Message
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND conversation_id = ? AND user_id = ?", messageID, conversationID, userID).
			First(&target).Error; err != nil {
			return err
		}
		// 服务层已预检，这里在持锁后复核：pending 消息可能正被进行中的运行回写，
		// 根消息的重挂会让历史以助手消息开头，二者都不允许删除。
		if strings.EqualFold(strings.TrimSpace(target.Status), "pending") {
			return repository.ErrMessageDeleteStateInvalid
		}
		if target.ParentMessageID == nil {
			return repository.ErrMessageDeleteRootInvalid
		}

		// 子树必须在重挂之前收集：重挂会改写直接子消息的父指针，之后按 parent 链
		// 向下遍历就只能拿到被删消息自己了。
		subtreeIDs, err := collectMessageSubtreeIDs(tx, conversationID, target.ID)
		if err != nil {
			return err
		}

		// splice：子消息重接到被删消息的父节点。已软删的子消息保持原样（本就不可见）。
		result := tx.Model(&models.Message{}).
			Where("parent_message_id = ? AND deleted_at IS NULL", target.ID).
			Update("parent_message_id", *target.ParentMessageID)
		if result.Error != nil {
			return result.Error
		}
		reparented = result.RowsAffected

		if err := tx.Delete(&target).Error; err != nil {
			return err
		}

		result = tx.Model(&models.Conversation{}).
			Where("id = ?", conversationID).
			Update("message_count", gorm.Expr("message_count - ?", 1))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return repository.ErrNotFound
		}

		return invalidateSnapshotsCoveringMessages(tx, conversationID, subtreeIDs)
	})
	if err != nil {
		return 0, translateError(err)
	}
	return reparented, nil
}

// collectMessageSubtreeIDs 收集指定消息及其全部未删除后代的消息 ID。
func collectMessageSubtreeIDs(tx *gorm.DB, conversationID uint, messageID uint) ([]uint, error) {
	const subtreeSQL = `
WITH RECURSIVE subtree AS (
    SELECT id
    FROM chat_messages
    WHERE id = ? AND conversation_id = ? AND deleted_at IS NULL
    UNION ALL
    SELECT m.id
    FROM chat_messages m
    INNER JOIN subtree s ON m.parent_message_id = s.id
    WHERE m.conversation_id = ? AND m.deleted_at IS NULL
)
SELECT id FROM subtree`
	subtreeIDs := make([]uint, 0, 8)
	if err := tx.Raw(subtreeSQL, messageID, conversationID, conversationID).Scan(&subtreeIDs).Error; err != nil {
		return nil, err
	}
	return subtreeIDs, nil
}

// invalidateSnapshotsCoveringMessages 删除覆盖边界（covered_until_message_id）落在指定
// 消息集合内的压缩快照。深层快照本会因 CoveragePathHash 校验失败被自然拒绝，但
// SnapshotBoundaryAncestorIndex 依赖「parent 链不可变」假设判定边界归属，splice 改写
// parent 链后该假设不再成立，必须主动失效。
func invalidateSnapshotsCoveringMessages(tx *gorm.DB, conversationID uint, messageIDs []uint) error {
	if len(messageIDs) == 0 {
		return nil
	}
	return tx.Unscoped().
		Where("conversation_id = ? AND record_type = ? AND covered_until_message_id IN ?", conversationID, chatContextRecordSnapshot, messageIDs).
		Delete(&models.ChatContextRecord{}).Error
}
