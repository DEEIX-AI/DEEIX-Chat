package model

// PermissionGroup 分组主数据，控制平台模型的访问范围与计费倍率。
type PermissionGroup struct {
	ControlPlaneModel
	Code                  string `gorm:"uniqueIndex;not null;size:64;comment:分组编码"`
	Name                  string `gorm:"not null;size:128;comment:分组名称"`
	Description           string `gorm:"size:512;comment:分组说明"`
	IsDefault             bool   `gorm:"default:false;comment:是否内置默认组(所有用户隐式归属)"`
	RateMultiplierPercent int    `gorm:"not null;default:100;comment:计费倍率百分比(100=1.0x)"`
}

func (PermissionGroup) TableName() string {
	return "permission_groups"
}

// PermissionGroupModelAccess 权限组与平台模型的多对多关联。
type PermissionGroupModelAccess struct {
	GroupID         uint `gorm:"primaryKey;comment:权限组ID"`
	PlatformModelID uint `gorm:"primaryKey;index:idx_pgma_platform_model_id;comment:平台模型ID"`
}

func (PermissionGroupModelAccess) TableName() string {
	return "permission_group_model_access"
}

// PermissionGroupUserAccess 权限组与用户的多对多关联。
type PermissionGroupUserAccess struct {
	GroupID uint `gorm:"primaryKey;comment:权限组ID"`
	UserID  uint `gorm:"primaryKey;index:idx_pgua_user_id;comment:用户ID"`
}

func (PermissionGroupUserAccess) TableName() string {
	return "permission_group_user_access"
}
