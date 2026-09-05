package admin

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

var (
	// ErrInvalidUserEmail 非法用户邮箱。
	ErrInvalidUserEmail = apperr.New("user.invalid_email", "invalid user email")
	// ErrInvalidUserPhone 非法用户手机号。
	ErrInvalidUserPhone = apperr.New("user.invalid_phone", "invalid user phone")
	// ErrInvalidUserLocale 非法用户语言。
	ErrInvalidUserLocale = apperr.New("user.invalid_locale", "invalid user locale")
	// ErrInvalidUserStatus 非法用户状态。
	ErrInvalidUserStatus = apperr.New("user.invalid_status", "invalid user status")
	// ErrInvalidUserRole 非法用户角色。
	ErrInvalidUserRole = apperr.New("user.invalid_role", "invalid user role")
	// ErrInvalidUserTimeZone 非法用户时区。
	ErrInvalidUserTimeZone = apperr.NewMasked("user.invalid_time_zone", "invalid time zone", "invalid user timezone")
	// ErrAdminPermissionRequired 需要管理员权限。
	ErrAdminPermissionRequired = apperr.New("auth.admin_required", "admin permission required")
	// ErrSuperAdminStatusChangeNotAllowed 不允许修改 superadmin 状态。
	ErrSuperAdminStatusChangeNotAllowed = apperr.NewMasked("user.superadmin_status_protected", "superadmin status change is not allowed", "superadmin status change not allowed")
	// ErrSuperAdminManagementNotAllowed 不允许 admin 管理 superadmin。
	ErrSuperAdminManagementNotAllowed = apperr.NewMasked("user.superadmin_management_protected", "superadmin management is not allowed", "superadmin management not allowed")
	// ErrLastSuperAdminRoleChangeNotAllowed 不允许降级最后一个 superadmin。
	ErrLastSuperAdminRoleChangeNotAllowed = apperr.NewMasked("user.last_superadmin_role_protected", "last superadmin role change is not allowed", "last superadmin role change not allowed")
	// ErrSelfRoleChangeNotAllowed 不允许修改自己的角色。
	ErrSelfRoleChangeNotAllowed = apperr.NewMasked("user.self_role_change_not_allowed", "self role change is not allowed", "self role change not allowed")
	// ErrSelfStatusChangeNotAllowed 不允许修改自己的状态。
	ErrSelfStatusChangeNotAllowed = apperr.NewMasked("user.self_status_change_not_allowed", "self status change is not allowed", "self status change not allowed")
	// ErrEmptyAdminUserPatch 不允许空更新。
	ErrEmptyAdminUserPatch = apperr.NewMasked("user.empty_patch", "at least one user field is required", "empty admin user patch")
	// ErrSuperAdminPasswordResetNotAllowed 不允许通过管理接口重置 superadmin 密码。
	ErrSuperAdminPasswordResetNotAllowed = apperr.NewMasked("user.superadmin_password_reset_protected", "superadmin password reset is not allowed", "superadmin password reset not allowed")
	// ErrSuperAdminTwoFactorResetNotAllowed 不允许通过管理接口重置 superadmin 两步验证。
	ErrSuperAdminTwoFactorResetNotAllowed = apperr.NewMasked("user.superadmin_two_factor_reset_protected", "superadmin two factor reset is not allowed", "superadmin two factor reset not allowed")
	// ErrSuperAdminDeleteNotAllowed 不允许通过管理接口删除 superadmin。
	ErrSuperAdminDeleteNotAllowed = apperr.NewMasked("user.superadmin_delete_protected", "superadmin account deletion is not allowed", "superadmin delete not allowed")
	// ErrSelfDeleteNotAllowed 不允许通过管理接口删除自己。
	ErrSelfDeleteNotAllowed = apperr.NewMasked("user.self_delete_not_allowed", "self delete is not allowed", "self delete not allowed")
	// ErrInvalidImportDSN 表示导入数据源地址不合法。
	ErrInvalidImportDSN = apperr.New("request.invalid_import_dsn", "invalid import dsn")
	// ErrInvalidImportMultiplier 表示导入积分转换比例不合法。
	ErrInvalidImportMultiplier = apperr.New("request.invalid_import_credit_multiplier", "invalid import credit multiplier")
	// ErrOpenWebUIImportFailed 表示 OpenWebUI 导入失败。
	ErrOpenWebUIImportFailed = apperr.NewMasked("internal.error", "internal server error", "openwebui import failed")
	// ErrPermissionGroupRepoUnavailable 表示权限组仓储未注入。
	ErrPermissionGroupRepoUnavailable = apperr.NewMasked("internal.error", "internal server error", "permission group repo unavailable")
	// ErrPermissionGroupNotFound 表示权限组不存在。
	ErrPermissionGroupNotFound = apperr.New("admin.permission_group_not_found", "permission group not found")
	// ErrInvalidPermissionGroupName 表示权限组名称不合法。
	ErrInvalidPermissionGroupName = apperr.New("admin.invalid_permission_group_name", "invalid permission group name")
	// ErrDefaultPermissionGroupDeleteNotAllowed 表示默认权限组不可删除。
	ErrDefaultPermissionGroupDeleteNotAllowed = apperr.NewMasked("admin.default_permission_group_delete_not_allowed", "default permission group cannot be deleted", "default permission group delete not allowed")
	// ErrDefaultPermissionGroupUsersImmutable 表示默认权限组成员由系统隐式维护，不可手动配置。
	ErrDefaultPermissionGroupUsersImmutable = apperr.New("admin.default_permission_group_users_implicit", "default permission group users are implicit")
	// ErrInvalidPermissionGroupRateMultiplier 表示权限组计费倍率不合法。
	ErrInvalidPermissionGroupRateMultiplier = apperr.New("admin.invalid_permission_group_rate_multiplier", "invalid permission group rate multiplier")
	// ErrPermissionGroupReferencedByPlan 表示权限组被计费套餐引用，不可删除。
	ErrPermissionGroupReferencedByPlan = apperr.NewMasked("admin.permission_group_referenced_by_plan", "permission group is referenced by a billing plan", "permission group is referenced by billing plan")
	// ErrInvalidPermissionGroupModels 表示权限组绑定的平台模型集合非法。
	ErrInvalidPermissionGroupModels = apperr.New("admin.invalid_permission_group_models", "invalid permission group models")
	// ErrInvalidPermissionGroupUsers 表示权限组绑定的用户集合非法。
	ErrInvalidPermissionGroupUsers = apperr.New("admin.invalid_permission_group_users", "invalid permission group users")
)
