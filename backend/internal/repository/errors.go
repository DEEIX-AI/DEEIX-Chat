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

	// 上游与模型仓储语义错误。
	ErrUpstreamNotFound           = errors.New("upstream not found")
	ErrModelNotFound              = errors.New("model not found")
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
