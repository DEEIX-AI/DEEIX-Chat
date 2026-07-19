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
	// ErrUsageReservationConflict 表示调用编号已被使用，不能重复消费同一预算。
	ErrUsageReservationConflict = errors.New("usage reservation already exists")
	// ErrUsageConcurrencyLimitExceeded 表示用户同时运行的付费调用数量达到上限。
	ErrUsageConcurrencyLimitExceeded = errors.New("usage concurrency limit exceeded")
	// ErrInvalidSubscriptionTier 非法订阅套餐。
	ErrInvalidSubscriptionTier = errors.New("invalid subscription tier")
	// ErrSubscriptionExpiryRequired 付费订阅必须指定到期时间。
	ErrSubscriptionExpiryRequired = errors.New("subscription expiry required")
	// ErrInvalidSubscriptionExpiry 非法订阅到期时间。
	ErrInvalidSubscriptionExpiry = errors.New("invalid subscription expiry")
	// ErrInvalidBillingPlan 非法计费套餐。
	ErrInvalidBillingPlan = errors.New("invalid billing plan")
	// ErrBillingPlanNotFound 计费套餐不存在。
	ErrBillingPlanNotFound = errors.New("billing plan not found")
	// ErrInvalidPermissionGroup 非法权限组。
	ErrInvalidPermissionGroup = errors.New("invalid permission group")
	// ErrInvalidUsageStatisticsSubject 表示用量统计的用户与权限组筛选条件冲突。
	ErrInvalidUsageStatisticsSubject = errors.New("user and permission group filters are mutually exclusive")
	// ErrPermissionGroupReferenceCounterUnavailable 权限组套餐引用检查能力不可用。
	ErrPermissionGroupReferenceCounterUnavailable = errors.New("permission group reference counter unavailable")
	// ErrSubscriptionEntitlementActive 当前仍存在有效付费订阅权益。
	ErrSubscriptionEntitlementActive = errors.New("subscription entitlement is active")
	// ErrRedemptionCodeHashUnavailable 兑换码哈希密钥不可用。
	ErrRedemptionCodeHashUnavailable = errors.New("redemption code hash secret unavailable")
	// ErrInvalidRedemptionCode 兑换码格式或配置非法。
	ErrInvalidRedemptionCode = errors.New("invalid redemption code")
	// ErrRedemptionCodeConflict 兑换码明文对应的哈希已存在。
	ErrRedemptionCodeConflict = errors.New("redemption code already exists")
	// ErrRedemptionCodeUnavailable 兑换码不存在、停用、过期或与当前计费模式不匹配。
	ErrRedemptionCodeUnavailable = errors.New("redemption code is unavailable")
	// ErrRedemptionCodePlaintextUnavailable 兑换码未保存可解密密文，无法再次展示明文。
	ErrRedemptionCodePlaintextUnavailable = errors.New("redemption code plaintext unavailable")
	// ErrRedemptionCodeExhausted 兑换码总次数已用完。
	ErrRedemptionCodeExhausted = errors.New("redemption code exhausted")
	// ErrRedemptionUserLimitExceeded 当前用户已达到兑换次数上限。
	ErrRedemptionUserLimitExceeded = errors.New("redemption user limit exceeded")
)
