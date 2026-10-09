package models

import "time"

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

// ErrorLog 记录失败请求的原始原因（已脱敏、截断），按 request_id 供管理员检索。
// 只追加、按保留期物理清理，不做软删除，因此不嵌入 BaseModel。
type ErrorLog struct {
	ID         uint      `gorm:"primaryKey;comment:主键ID"`
	RequestID  string    `gorm:"size:64;not null;default:'';index:idx_system_error_logs_request_id;comment:请求ID"`
	TraceID    string    `gorm:"size:64;not null;default:'';comment:链路追踪ID"`
	UserID     uint      `gorm:"not null;default:0;index:idx_system_error_logs_user_id;comment:用户ID"`
	Method     string    `gorm:"size:16;not null;default:'';comment:HTTP方法"`
	Route      string    `gorm:"size:255;not null;default:'';comment:路由模板"`
	Path       string    `gorm:"size:512;not null;default:'';comment:请求路径(已脱敏)"`
	StatusCode int       `gorm:"not null;default:0;index:idx_system_error_logs_status_code;comment:HTTP状态码"`
	ErrorCode  string    `gorm:"size:128;not null;default:'';index:idx_system_error_logs_error_code;comment:对外错误码"`
	Cause      string    `gorm:"type:text;not null;default:'';comment:原始错误原因(已脱敏)"`
	Detail     string    `gorm:"type:text;not null;default:'';comment:原始诊断内容(如上游响应体,已遮蔽凭证)"`
	LatencyMS  int64     `gorm:"not null;default:0;comment:耗时毫秒"`
	ClientIP   string    `gorm:"size:64;not null;default:'';comment:客户端IP"`
	UserAgent  string    `gorm:"size:255;not null;default:'';comment:用户代理"`
	CreatedAt  time.Time `gorm:"not null;index:idx_system_error_logs_created_at;comment:创建时间"`
}

// TableName 指定表名。
func (ErrorLog) TableName() string {
	return "system_error_logs"
}
