package auth

import "errors"

var (
	// ErrInvalidCredentials 用户名或密码错误。
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrAccountLocked 账户已被锁定。
	ErrAccountLocked = errors.New("account locked")
	// ErrInvalidTimeZone 用户时区格式非法。
	ErrInvalidTimeZone = errors.New("invalid time zone")
	// ErrInvalidLocale 用户语言区域非法。
	ErrInvalidLocale = errors.New("invalid user locale")
	// ErrInvalidAppearancePreferences 外观偏好 JSON 非法。
	ErrInvalidAppearancePreferences = errors.New("invalid appearance preferences")
	// ErrInvalidAvatarURL 用户头像地址格式非法。
	ErrInvalidAvatarURL = errors.New("invalid avatar url")
	// ErrInvalidUsername 用户名格式非法。
	ErrInvalidUsername = errors.New("invalid username")
	// ErrUsernameTaken 用户名已被占用。
	ErrUsernameTaken = errors.New("username already exists")
	// ErrUsernameChangeUsed 用户名自主修改次数已用完。
	ErrUsernameChangeUsed = errors.New("username change already used")
	// ErrUsernameChangeRequired 初始化用户名必须修改。
	ErrUsernameChangeRequired = errors.New("username change required")
	// ErrInvalidLocation 当前会话位置数据非法。
	ErrInvalidLocation = errors.New("invalid location")
	// ErrDeleteSuperAdminNotAllowed 禁止自助删除超级管理员。
	ErrDeleteSuperAdminNotAllowed = errors.New("superadmin account deletion not allowed")
	// ErrAccountDeleteVerificationRequired 删除账号必须先完成安全验证。
	ErrAccountDeleteVerificationRequired = errors.New("account deletion requires verification")
	// ErrInvalidRefreshToken 无效刷新令牌。
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	// ErrSessionRevoked 会话已吊销。
	ErrSessionRevoked = errors.New("session revoked")
	// ErrLastLoginMethodNotAllowed 禁止移除账号最后一种可用登录方式。
	ErrLastLoginMethodNotAllowed = errors.New("cannot unlink the last available login method")
	// ErrIdentityNotFound 表示当前用户绑定身份不存在。
	ErrIdentityNotFound = errors.New("identity not found")
	// ErrIdentityProviderDeleteConflict 表示删除身份源会让用户失去最后一种登录方式。
	ErrIdentityProviderDeleteConflict = errors.New("identity provider delete conflict")
	// ErrIdentityProviderLogoUnavailable 表示身份源图标不可用或不符合代理安全策略。
	ErrIdentityProviderLogoUnavailable = errors.New("identity provider logo unavailable")
	// ErrIdentityProviderSuperAdminDefaultRoleNotAllowed 表示非 superadmin 不允许设置 superadmin 默认角色。
	ErrIdentityProviderSuperAdminDefaultRoleNotAllowed = errors.New("only superadmin can set superadmin default role")
	// ErrTwoFactorSetupExpired 两步验证设置已过期。
	ErrTwoFactorSetupExpired = errors.New("two factor setup expired")
	// ErrTwoFactorSetupNotStarted 当前没有待确认的两步验证设置。
	ErrTwoFactorSetupNotStarted = errors.New("two factor setup not started")
	// ErrTwoFactorSetupNotPersisted 两步验证确认后持久化状态未生效。
	ErrTwoFactorSetupNotPersisted = errors.New("two factor setup not persisted")
	// ErrTwoFactorChallengeExpired 登录二次验证挑战已过期。
	ErrTwoFactorChallengeExpired = errors.New("two factor challenge expired")
)

// IdentityProviderDeleteConflictError 携带身份源删除冲突的受影响用户数量。
type IdentityProviderDeleteConflictError struct {
	DependentUsers int
}

func (e *IdentityProviderDeleteConflictError) Error() string {
	return ErrIdentityProviderDeleteConflict.Error()
}

func (e *IdentityProviderDeleteConflictError) Unwrap() error {
	return ErrIdentityProviderDeleteConflict
}
