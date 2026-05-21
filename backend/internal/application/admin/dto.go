package admin

import (
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/userview"
	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
	domainsystemevent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/systemevent"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
)

// PatchUserInput 管理员局部更新用户输入。
type PatchUserInput struct {
	AvatarURL             *string
	DisplayName           *string
	Email                 *string
	Phone                 *string
	Role                  *string
	Status                *string
	Timezone              *string
	Locale                *string
	ProfilePreferences    *string
	SubscriptionTier      *string
	SubscriptionExpiresAt *time.Time
	Reason                string
}

// UserResult 用户响应数据（内部传输，不携带序列化标记）。
type UserResult struct {
	User userview.UserView
}

// RevokeUserSessionsResult 管理员吊销用户会话响应数据（内部传输，不携带序列化标记）。
type RevokeUserSessionsResult struct {
	Revoked bool
}

// UpdateUserStatusResult 管理员更新用户状态响应数据（内部传输，不携带序列化标记）。
type UpdateUserStatusResult struct {
	User userview.UserView
}

// ResetUserPasswordResult 管理员重置密码响应数据（内部传输，不携带序列化标记）。
type ResetUserPasswordResult struct {
	Reset bool
}

// DeleteUserResult 管理员删除用户响应数据（内部传输，不携带序列化标记）。
type DeleteUserResult struct {
	Deleted bool
}

// UserAuthEventsResult 用户认证事件分页数据（内部传输，不携带序列化标记）。
type UserAuthEventsResult struct {
	Total   int64
	Results []domainuser.AuthEvent
}

// AuditLogsResult 审计日志分页数据（内部传输，不携带序列化标记）。
type AuditLogsResult struct {
	Total   int64
	Results []domainaudit.Log
}

// SystemEventsResult 系统事件分页数据（内部传输，不携带序列化标记）。
type SystemEventsResult struct {
	Total   int64
	Results []domainsystemevent.Event
}
