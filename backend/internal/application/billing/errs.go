package billing

import "errors"

var (
	// ErrSubscribeFailed 订阅失败。
	ErrSubscribeFailed = errors.New("subscribe failed")
	// ErrPeriodCreditExceeded 周期套餐用量额度已用完。
	ErrPeriodCreditExceeded = errors.New("period usage credit exceeded")
	// ErrModelPricingRequired 付费模型缺少有效单价。
	ErrModelPricingRequired = errors.New("model pricing is required")
	// ErrInvalidModelPricing 表示模型定价输入非法或目标平台模型不存在。
	ErrInvalidModelPricing = errors.New("invalid model pricing")
	// ErrPaymentRequired 付费套餐必须先完成支付。
	ErrPaymentRequired = errors.New("payment is required")
	// ErrPaymentProviderUnavailable 支付渠道未配置。
	ErrPaymentProviderUnavailable = errors.New("payment provider is unavailable")
	// ErrUsageBalanceInsufficient 按量余额不足。
	ErrUsageBalanceInsufficient = errors.New("usage balance is insufficient")
	// ErrInvalidSubscriptionTier 非法订阅套餐。
	ErrInvalidSubscriptionTier = errors.New("invalid subscription tier")
	// ErrSubscriptionExpiryRequired 付费订阅必须指定到期时间。
	ErrSubscriptionExpiryRequired = errors.New("subscription expiry required")
	// ErrInvalidSubscriptionExpiry 非法订阅到期时间。
	ErrInvalidSubscriptionExpiry = errors.New("invalid subscription expiry")
)
