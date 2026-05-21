package model

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

// SystemEvent 记录后台可检索的结构化系统事件。
type SystemEvent struct {
	BaseModel
	RequestID  string `gorm:"size:64;not null;default:'';index:idx_system_events_request_id;comment:请求ID"`
	TraceID    string `gorm:"size:64;not null;default:'';index:idx_system_events_trace_id;comment:链路追踪ID"`
	Level      string `gorm:"size:32;not null;default:'info';index:idx_system_events_level;comment:事件级别(info/warn/error)"`
	Source     string `gorm:"size:128;not null;default:'';index:idx_system_events_source;comment:事件来源模块"`
	Event      string `gorm:"size:128;not null;default:'';index:idx_system_events_event;comment:事件名称"`
	Resource   string `gorm:"size:128;not null;default:'';index:idx_system_events_resource;comment:资源类型"`
	ResourceID string `gorm:"size:128;not null;default:'';index:idx_system_events_resource_id;comment:资源ID"`
	Message    string `gorm:"size:512;not null;default:'';comment:事件消息"`
	DetailJSON string `gorm:"type:text;not null;default:'{}';comment:详情JSON"`
}

// TableName 指定表名。
func (SystemEvent) TableName() string {
	return "system_events"
}
