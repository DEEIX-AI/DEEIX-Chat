package model

// UserMemory 记录用户长期个性化记忆。
type UserMemory struct {
	BaseModel
	UserID    uint   `gorm:"not null;index:idx_user_memories_user_id;comment:用户ID"`
	MemoryKey string `gorm:"size:128;not null;uniqueIndex:idx_user_memories_user_key;comment:记忆键"`
	Value     string `gorm:"type:text;not null;default:'';comment:记忆值"`
	Scope     string `gorm:"size:32;not null;default:'';index:idx_user_memories_scope;comment:记忆范围"`
	UpdatedBy string `gorm:"size:32;not null;default:'';comment:更新来源"`
}

// TableName 指定表名。
func (UserMemory) TableName() string {
	return "user_memories"
}
