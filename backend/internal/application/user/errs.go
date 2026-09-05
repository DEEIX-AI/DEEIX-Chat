package user

import (
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"
)

var (
	// ErrUsernameTaken 用户名已存在。
	ErrUsernameTaken = apperr.New("user.username_already_exists", "username already exists")
	// ErrInvalidUsername 用户名格式非法。
	ErrInvalidUsername = apperr.New("user.invalid_username", "invalid username")
	// ErrInvalidDisplayName 显示名称格式非法。
	ErrInvalidDisplayName = apperr.New("user.invalid_display_name", "invalid display name")
	// ErrInvalidPassword 密码不符合安全策略。
	ErrInvalidPassword = apperr.NewMasked("auth.invalid_password", "password must be at least 8 characters and not digits only", "invalid password")
	// ErrUserNotFound 用户不存在。
	ErrUserNotFound = apperr.New("user.not_found", "user not found")
	// ErrInvalidAvatarURL 非法头像地址。
	ErrInvalidAvatarURL = apperr.New("user.invalid_avatar_url", "invalid avatar url")
	// ErrAvatarNotFound 头像不存在。
	ErrAvatarNotFound = apperr.New("avatar.not_found", "avatar not found")
	// ErrInvalidEmail 非法邮箱。
	ErrInvalidEmail = apperr.New("user.invalid_email", "invalid user email")
	// ErrInvalidPhone 非法手机号。
	ErrInvalidPhone = apperr.New("user.invalid_phone", "invalid user phone")
	// ErrInvalidTimeZone 非法时区。
	ErrInvalidTimeZone = apperr.NewMasked("user.invalid_time_zone", "invalid time zone", "invalid timezone")
	// ErrInvalidLocale 非法语言区域。
	ErrInvalidLocale = apperr.New("user.invalid_locale", "invalid user locale")
	// ErrInvalidSubscriptionTier 非法订阅等级。
	ErrInvalidSubscriptionTier = apperr.New("billing.invalid_subscription_tier", "invalid subscription tier")
	// ErrSubscriptionExpiryRequired 付费订阅必须指定到期时间。
	ErrSubscriptionExpiryRequired = apperr.New("billing.subscription_expiry_required", "subscription expiry required")
	// ErrInvalidSubscriptionExpiry 非法订阅到期时间。
	ErrInvalidSubscriptionExpiry = apperr.New("billing.invalid_subscription_expiry", "invalid subscription expiry")
)
