package settings

import "time"

// SystemSetting 表示系统动态配置项。
type SystemSetting struct {
	ID          uint
	Namespace   string
	Key         string
	Value       string
	ValueType   string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
