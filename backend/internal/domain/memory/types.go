package memory

import "time"

// UserMemory 表示用户长期记忆。
type UserMemory struct {
	ID        uint
	UserID    uint
	MemoryKey string
	Value     string
	Scope     string
	UpdatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}
