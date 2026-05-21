package usersettings

import "time"

// UserSetting 表示用户个人配置项。
type UserSetting struct {
	ID        uint
	UserID    uint
	Key       string
	Value     string
	UpdatedAt time.Time
}
