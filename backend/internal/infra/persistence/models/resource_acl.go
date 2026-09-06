package model

// ResourceACLEntry 存储会话/知识库等资源的用户级 ACL（硬删除，便于重新授权）。
type ResourceACLEntry struct {
	ControlPlaneModel
	ResourceType     string `gorm:"size:32;not null;default:'';uniqueIndex:idx_resource_acl_unique;comment:资源类型"`
	ResourcePublicID string `gorm:"size:64;not null;default:'';uniqueIndex:idx_resource_acl_unique;index:idx_resource_acl_resource;comment:资源公开ID"`
	GranteeUserID    uint   `gorm:"not null;default:0;uniqueIndex:idx_resource_acl_unique;index:idx_resource_acl_grantee;comment:被授权用户ID"`
	Role             string `gorm:"size:16;not null;default:'viewer';comment:角色(viewer/editor)"`
}

// TableName 指定表名。
func (ResourceACLEntry) TableName() string {
	return "resource_acl_entries"
}
