package model

import "time"

// SystemSetting 系统动态配置项。
type SystemSetting struct {
	ID          uint      `gorm:"primaryKey;comment:主键ID"`
	Namespace   string    `gorm:"size:32;not null;uniqueIndex:uk_system_settings_ns_key;comment:配置命名空间"`
	Key         string    `gorm:"size:128;not null;uniqueIndex:uk_system_settings_ns_key;comment:配置键"`
	Value       string    `gorm:"type:text;not null;comment:配置值"`
	ValueType   string    `gorm:"size:16;not null;comment:值类型(string/int/bool/json)"`
	Description string    `gorm:"size:255;not null;default:'';comment:配置说明"`
	CreatedAt   time.Time `gorm:"comment:创建时间"`
	UpdatedAt   time.Time `gorm:"comment:更新时间"`
}

// TableName 指定表名。
func (SystemSetting) TableName() string {
	return "system_settings"
}

// UserSetting 用户个人偏好配置项。
// key 自带命名空间前缀（如 chat.file_mode），不设单独 namespace 列。
type UserSetting struct {
	ID        uint      `gorm:"primaryKey;comment:主键ID"`
	UserID    uint      `gorm:"not null;uniqueIndex:uk_user_settings_user_key;index:idx_user_settings_user_id;comment:用户ID"`
	Key       string    `gorm:"size:64;not null;uniqueIndex:uk_user_settings_user_key;comment:配置键(含前缀，如chat.file_mode)"`
	Value     string    `gorm:"type:text;not null;default:'';comment:配置值"`
	UpdatedAt time.Time `gorm:"comment:更新时间"`
}

// TableName 指定表名。
func (UserSetting) TableName() string {
	return "user_settings"
}
