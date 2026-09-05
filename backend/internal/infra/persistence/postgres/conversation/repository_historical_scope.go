package conversation

import (
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
)

// historicalMessageScopeCTE 从当前叶消息沿不可变 parent_message_id 向上解析分支。
// UNION 在异常循环父指针下去重终止；ExcludeThroughMessageID 命中后停止继续向上遍历。
const historicalMessageScopeCTE = `
WITH RECURSIVE historical_message_scope(id, parent_message_id) AS (
    SELECT id, parent_message_id
    FROM chat_messages
    WHERE id = ? AND conversation_id = ? AND user_id = ? AND deleted_at IS NULL
    UNION
    SELECT messages.id, messages.parent_message_id
    FROM chat_messages AS messages
    INNER JOIN historical_message_scope AS scope ON messages.id = scope.parent_message_id
    WHERE scope.id <> ?
      AND messages.conversation_id = ?
      AND messages.user_id = ?
      AND messages.deleted_at IS NULL
), valid_historical_message_scope(id) AS (
    SELECT scope.id
    FROM historical_message_scope AS scope
    WHERE scope.id <> ?
      AND scope.id <> ?
      AND (
          ? = 0 OR EXISTS (
              SELECT 1
              FROM historical_message_scope AS boundary
              WHERE boundary.id = ?
          )
      )
)`

const historicalMessageScopeSubquerySQL = historicalMessageScopeCTE + `
SELECT id
FROM valid_historical_message_scope`

func historicalMessageScopeArgs(scope repository.HistoricalMessageScope) []any {
	return []any{
		scope.LeafMessageID,
		scope.ConversationID,
		scope.UserID,
		scope.ExcludeThroughMessageID,
		scope.ConversationID,
		scope.UserID,
		scope.LeafMessageID,
		scope.ExcludeThroughMessageID,
		scope.ExcludeThroughMessageID,
		scope.ExcludeThroughMessageID,
	}
}

func historicalMessageScopeSubquery(db *gorm.DB, scope repository.HistoricalMessageScope) *gorm.DB {
	return db.Raw(historicalMessageScopeSubquerySQL, historicalMessageScopeArgs(scope)...)
}
