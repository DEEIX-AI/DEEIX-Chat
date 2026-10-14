package models

import "time"

// ConversationMessageUIState 记录一条助手消息内单个交互式组件的状态快照。
// 每个 (message_id, block_id) 一行，只保存与块初始值不同的键；恢复与注入都按消息整块读取。
// 该表是纯旁路数据：硬删除，不做软删除，避免旧行占用唯一索引导致同块再次写入冲突。
type ConversationMessageUIState struct {
	ID             uint      `gorm:"primaryKey;comment:主键ID"`
	UserID         uint      `gorm:"not null;default:0;index:idx_chat_message_ui_state_user_id;comment:用户ID"`
	ConversationID uint      `gorm:"not null;default:0;index:idx_chat_message_ui_state_conversation_id;comment:会话ID"`
	MessageID      uint      `gorm:"not null;default:0;uniqueIndex:idx_chat_message_ui_state_message_block,priority:1;index:idx_chat_message_ui_state_message_id;comment:助手消息ID"`
	BlockID        string    `gorm:"size:64;not null;default:'';uniqueIndex:idx_chat_message_ui_state_message_block,priority:2;comment:组件块ID"`
	StateJSON      string    `gorm:"type:text;not null;default:'{}';comment:组件状态快照JSON(仅存与初始值不同的键)"`
	CreatedAt      time.Time `gorm:"comment:创建时间"`
	UpdatedAt      time.Time `gorm:"comment:更新时间"`
}

// TableName 指定表名。
func (ConversationMessageUIState) TableName() string {
	return "chat_message_ui_states"
}
