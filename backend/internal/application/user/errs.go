package user

import "errors"

var (
	// ErrUsernameTaken 用户名已存在。
	ErrUsernameTaken = errors.New("username already exists")
	// ErrInvalidUsername 用户名格式非法。
	ErrInvalidUsername = errors.New("invalid username")
	// ErrInvalidDisplayName 显示名称格式非法。
	ErrInvalidDisplayName = errors.New("invalid display name")
	// ErrInvalidPassword 密码不符合安全策略。
	ErrInvalidPassword = errors.New("invalid password")
	// ErrUserNotFound 用户不存在。
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidAvatarURL 非法头像地址。
	ErrInvalidAvatarURL = errors.New("invalid avatar url")
	// ErrInvalidEmail 非法邮箱。
	ErrInvalidEmail = errors.New("invalid user email")
	// ErrInvalidPhone 非法手机号。
	ErrInvalidPhone = errors.New("invalid user phone")
	// ErrInvalidTimeZone 非法时区。
	ErrInvalidTimeZone = errors.New("invalid timezone")
	// ErrInvalidLocale 非法语言区域。
	ErrInvalidLocale = errors.New("invalid user locale")
	// ErrInvalidSubscriptionTier 非法订阅等级。
	ErrInvalidSubscriptionTier = errors.New("invalid subscription tier")
	// ErrSubscriptionExpiryRequired 付费订阅必须指定到期时间。
	ErrSubscriptionExpiryRequired = errors.New("subscription expiry required")
	// ErrInvalidSubscriptionExpiry 非法订阅到期时间。
	ErrInvalidSubscriptionExpiry = errors.New("invalid subscription expiry")
)
