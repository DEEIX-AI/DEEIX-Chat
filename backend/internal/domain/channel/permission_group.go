package channel

import "time"

// PermissionGroup 定义模型访问权限组。
type PermissionGroup struct {
	ID          uint
	Code        string
	Name        string
	Description string
	IsDefault   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PermissionGroupModelAccess 关联权限组与平台模型。
type PermissionGroupModelAccess struct {
	GroupID         uint
	PlatformModelID uint
}

// PermissionGroupUserAccess 关联权限组与用户。
type PermissionGroupUserAccess struct {
	GroupID uint
	UserID  uint
}
