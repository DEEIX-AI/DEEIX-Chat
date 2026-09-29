package models

// PermissionGroup 权限组主数据，控制平台模型的访问范围与计费倍率。
type PermissionGroup struct {
	ControlPlaneModel
	Name                  string `gorm:"not null;size:128;comment:权限组名称"`
	Description           string `gorm:"size:512;comment:权限组说明"`
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

// PermissionGroupModelRule 权限组动态模型访问规则。
type PermissionGroupModelRule struct {
	GroupID  uint   `gorm:"primaryKey;comment:权限组ID"`
	RuleType string `gorm:"primaryKey;size:32;comment:规则类型(all/vendor/protocol/upstream)"`
	Value    string `gorm:"primaryKey;size:128;comment:规则值"`
}

func (PermissionGroupModelRule) TableName() string {
	return "permission_group_model_rules"
}

// PermissionGroupUserAccess 权限组与用户的多对多关联。
type PermissionGroupUserAccess struct {
	GroupID uint `gorm:"primaryKey;comment:权限组ID"`
	UserID  uint `gorm:"primaryKey;index:idx_pgua_user_id;comment:用户ID"`
}

func (PermissionGroupUserAccess) TableName() string {
	return "permission_group_user_access"
}
