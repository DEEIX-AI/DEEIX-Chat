package repository

import "errors"

var (
	// ErrNotFound 表示记录不存在。
	ErrNotFound = errors.New("record not found")
	// ErrDuplicate 表示违反唯一约束。
	ErrDuplicate = errors.New("duplicate record")
	// ErrDuplicateUsername 表示用户名唯一约束冲突。
	ErrDuplicateUsername = errors.New("duplicate username")
	// ErrDuplicateUserIdentity 表示第三方身份唯一约束冲突。
	ErrDuplicateUserIdentity = errors.New("duplicate user identity")
	// ErrConflict 表示资源状态冲突，操作无法执行。
	ErrConflict = errors.New("resource conflict")
	// ErrInvalidInput 表示输入数据非法。
	ErrInvalidInput = errors.New("invalid input")
	// ErrInsufficientBalance 表示余额不足，无法完成扣费。
	ErrInsufficientBalance = errors.New("insufficient balance")
	// ErrUsageReservationLimitExceeded 表示用户活跃付费调用数量达到上限。
	ErrUsageReservationLimitExceeded = errors.New("usage reservation limit exceeded")
	// ErrRedemptionUnavailable 表示兑换码不可用。
	ErrRedemptionUnavailable = errors.New("redemption unavailable")
	// ErrRedemptionExhausted 表示兑换码总次数耗尽。
	ErrRedemptionExhausted = errors.New("redemption exhausted")
	// ErrRedemptionUserLimitExceeded 表示用户达到兑换次数上限。
	ErrRedemptionUserLimitExceeded = errors.New("redemption user limit exceeded")
	// ErrLastSuperAdminRoleChange 表示操作会移除最后一个超级管理员。
	ErrLastSuperAdminRoleChange = errors.New("last superadmin role change not allowed")
	// ErrFileProcessingQueueFull 表示文件处理队列已达容量上限，暂时无法接收新任务。
	ErrFileProcessingQueueFull = errors.New("file processing queue full")
	// ErrUserMemoryLimitExceeded 表示用户长期记忆条目数量达到上限，无法再新增。
	ErrUserMemoryLimitExceeded = errors.New("user memory limit exceeded")
	// ErrMCPServerLimitExceeded 表示 MCP 服务数量达到上限，无法再新增。
	ErrMCPServerLimitExceeded = errors.New("mcp server limit exceeded")
	// ErrConversationProjectLimitExceeded 表示单用户会话项目数量达到上限，无法再新增。
	ErrConversationProjectLimitExceeded = errors.New("conversation project limit exceeded")
	// ErrFileNotFound 文件对象不存在或无权限。
	ErrFileNotFound = errors.New("file not found")
	// ErrStorageQuotaExceeded 用户存储配额超限。
	ErrStorageQuotaExceeded = errors.New("storage quota exceeded")
	// ErrConversationShareSchemaOutdated 会话分享存储结构未更新。
	ErrConversationShareSchemaOutdated = errors.New("conversation share schema outdated")

	// 上游与模型仓储语义错误。
	ErrUpstreamNotFound           = errors.New("upstream not found")
	ErrModelNotFound              = errors.New("model not found")
	ErrModelVendorNotFound        = errors.New("model vendor not found")
	ErrModelDisplayGroupNotFound  = errors.New("model display group not found")
	ErrDuplicatePlatformModelName = errors.New("duplicate platform model name")
	ErrUpstreamModelNotFound      = errors.New("upstream model not found")
	ErrUpstreamModelConflict      = errors.New("upstream model conflict")
	ErrLLMSettingNotFound         = errors.New("llm setting not found")
)

// IdentityProviderDeleteConflictError 表示删除身份源会导致部分账号失去最后一种登录方式。
type IdentityProviderDeleteConflictError struct {
	DependentUsers int
}

func (e *IdentityProviderDeleteConflictError) Error() string {
	return "identity provider has dependent users"
}

func (e *IdentityProviderDeleteConflictError) Unwrap() error {
	return ErrConflict
}
