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
	// ErrAdminPermissionRequired 需要管理员权限。
	ErrAdminPermissionRequired = errors.New("admin permission required")
	// ErrSuperAdminStatusChangeNotAllowed 不允许修改 superadmin 状态。
	ErrSuperAdminStatusChangeNotAllowed = errors.New("superadmin status change not allowed")
	// ErrSuperAdminManagementNotAllowed 不允许 admin 管理 superadmin。
	ErrSuperAdminManagementNotAllowed = errors.New("superadmin management not allowed")
	// ErrLastSuperAdminRoleChangeNotAllowed 不允许降级最后一个 superadmin。
	ErrLastSuperAdminRoleChangeNotAllowed = errors.New("last superadmin role change not allowed")
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
	// ErrInvalidImportDSN 表示导入数据源地址不合法。
	ErrInvalidImportDSN = errors.New("invalid import dsn")
	// ErrInvalidImportMultiplier 表示导入积分转换比例不合法。
	ErrInvalidImportMultiplier = errors.New("invalid import credit multiplier")
	// ErrOpenWebUIImportFailed 表示 OpenWebUI 导入失败。
	ErrOpenWebUIImportFailed = errors.New("openwebui import failed")
	// ErrPermissionGroupRepoUnavailable 表示权限组仓储未注入。
	ErrPermissionGroupRepoUnavailable = errors.New("permission group repo unavailable")
	// ErrInvalidPermissionGroupCode 表示权限组编码不合法。
	ErrInvalidPermissionGroupCode = errors.New("invalid permission group code")
	// ErrInvalidPermissionGroupName 表示权限组名称不合法。
	ErrInvalidPermissionGroupName = errors.New("invalid permission group name")
	// ErrDefaultPermissionGroupDeleteNotAllowed 表示默认权限组不可删除。
	ErrDefaultPermissionGroupDeleteNotAllowed = errors.New("default permission group delete not allowed")
)
