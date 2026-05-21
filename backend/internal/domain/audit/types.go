package audit

import "time"

// Log 表示审计日志记录。
type Log struct {
	ID          uint
	RequestID   string
	ActorUserID uint
	Action      string
	Resource    string
	ResourceID  string
	IP          string
	UserAgent   string
	DetailJSON  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
