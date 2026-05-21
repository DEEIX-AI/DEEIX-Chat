package admin

import "errors"

var (
	// ErrInvalidUserEmail 非法用户邮箱。
	ErrInvalidUserEmail = errors.New("invalid user email")
	// ErrInvalidUserPhone 非法用户手机号。
	ErrInvalidUserPhone = errors.New("invalid user phone")
	// ErrInvalidUserLocale 非法用户语言。
	ErrInvalidUserLocale = errors.New("invalid user locale")
	// ErrInvalidUserStatus 非法用户状态。
	ErrInvalidUserStatus = errors.New("invalid user status")
	// ErrInvalidUserRole 非法用户角色。
	ErrInvalidUserRole = errors.New("invalid user role")
	// ErrInvalidUserTimeZone 非法用户时区。
	ErrInvalidUserTimeZone = errors.New("invalid user timezone")
	// ErrSuperAdminStatusChangeNotAllowed 不允许修改 superadmin 状态。
	ErrSuperAdminStatusChangeNotAllowed = errors.New("superadmin status change not allowed")
	// ErrSuperAdminRoleChangeNotAllowed 不允许通过管理接口变更 superadmin 角色。
	ErrSuperAdminRoleChangeNotAllowed = errors.New("superadmin role change not allowed")
	// ErrLastSuperAdminRoleChangeNotAllowed 不允许降级最后一个 superadmin。
	ErrLastSuperAdminRoleChangeNotAllowed = errors.New("last superadmin role change not allowed")
	// ErrSuperAdminAlreadyExists 不允许提升第二个 superadmin。
	ErrSuperAdminAlreadyExists = errors.New("superadmin already exists")
	// ErrSelfRoleChangeNotAllowed 不允许修改自己的角色。
	ErrSelfRoleChangeNotAllowed = errors.New("self role change not allowed")
	// ErrSelfStatusChangeNotAllowed 不允许修改自己的状态。
	ErrSelfStatusChangeNotAllowed = errors.New("self status change not allowed")
	// ErrEmptyAdminUserPatch 不允许空更新。
	ErrEmptyAdminUserPatch = errors.New("empty admin user patch")
	// ErrSuperAdminPasswordResetNotAllowed 不允许通过管理接口重置 superadmin 密码。
	ErrSuperAdminPasswordResetNotAllowed = errors.New("superadmin password reset not allowed")
	// ErrSuperAdminTwoFactorResetNotAllowed 不允许通过管理接口重置 superadmin 两步验证。
	ErrSuperAdminTwoFactorResetNotAllowed = errors.New("superadmin two factor reset not allowed")
	// ErrSuperAdminDeleteNotAllowed 不允许通过管理接口删除 superadmin。
	ErrSuperAdminDeleteNotAllowed = errors.New("superadmin delete not allowed")
	// ErrSelfDeleteNotAllowed 不允许通过管理接口删除自己。
	ErrSelfDeleteNotAllowed = errors.New("self delete not allowed")
)
