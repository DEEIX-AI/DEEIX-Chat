package models

// AuditLog 记录可追溯的业务审计日志。
type AuditLog struct {
	BaseModel
	RequestID   string `gorm:"size:64;not null;default:'';index:idx_audit_logs_request_id;comment:请求ID"`
	ActorUserID uint   `gorm:"not null;index:idx_audit_logs_actor_user_id;comment:操作人用户ID"`
	Action      string `gorm:"size:128;not null;default:'';index:idx_audit_logs_action;comment:动作"`
	Resource    string `gorm:"size:128;not null;default:'';index:idx_audit_logs_resource;comment:资源类型"`
	ResourceID  string `gorm:"size:128;not null;default:'';index:idx_audit_logs_resource_id;comment:资源ID"`
	IP          string `gorm:"size:64;not null;default:'';comment:请求IP"`
	UserAgent   string `gorm:"size:255;not null;default:'';comment:用户代理"`
	DetailJSON  string `gorm:"type:text;not null;default:'';comment:详情JSON"`
}

// TableName 指定表名。
func (AuditLog) TableName() string {
	return "audit_logs"
}
