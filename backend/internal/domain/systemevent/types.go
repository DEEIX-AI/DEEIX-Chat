package systemevent

import "time"

// Event 表示后台可检索的系统事件。
type Event struct {
	ID         uint
	RequestID  string
	TraceID    string
	Level      string
	Source     string
	Event      string
	Resource   string
	ResourceID string
	Message    string
	DetailJSON string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
