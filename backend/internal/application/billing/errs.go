package billing

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 用量计费哨兵随消息发送路径一起以 apperr 声明错误码与对外文案；
// 未携带对外契约的内部哨兵不得直接作为 HTTP 错误响应输出。
var (
	// ErrSubscribeFailed 订阅失败。
	ErrSubscribeFailed = apperr.New("billing.subscribe_failed", "subscription failed")
	// ErrPeriodCreditExceeded 周期套餐用量额度已用完。
	ErrPeriodCreditExceeded = apperr.New("billing.period_credit_exceeded", "period usage credit exceeded")
	// ErrModelPricingRequired 付费模型缺少有效单价。
	ErrModelPricingRequired = apperr.New("billing.pricing_required", "model pricing is required")
	// ErrInvalidModelPricing 表示模型定价输入非法或目标平台模型不存在。
	ErrInvalidModelPricing = apperr.New("billing.invalid_model_pricing", "invalid model pricing")
	// ErrPaymentRequired 付费套餐必须先完成支付。
	ErrPaymentRequired = apperr.New("billing.payment_required", "payment is required")
	// ErrPaymentProviderUnavailable 支付渠道未配置。
	ErrPaymentProviderUnavailable = apperr.New("payment.provider_unavailable", "payment provider is unavailable")
	// ErrEPayTypeUnsupported 表示请求的易支付类型未启用。
	ErrEPayTypeUnsupported = apperr.New("payment.epay_type_unsupported", "epay payment type is not supported")
	// ErrUsageBalanceInsufficient 按量余额不足。
	ErrUsageBalanceInsufficient = apperr.NewMasked("billing.insufficient_funds", "insufficient balance", "usage balance is insufficient")
	// ErrUsageReservationConflict 表示调用编号已被使用，不能重复消费同一预算。
	ErrUsageReservationConflict = apperr.NewMasked("billing.reservation_conflict", "usage request already exists", "usage reservation already exists")
	// ErrUsageConcurrencyLimitExceeded 表示用户同时运行的付费调用数量达到上限。
	ErrUsageConcurrencyLimitExceeded = apperr.NewMasked("billing.concurrency_limit_exceeded", "too many concurrent paid requests", "usage concurrency limit exceeded")
	// ErrInvalidSubscriptionTier 非法订阅套餐。
	ErrInvalidSubscriptionTier = apperr.New("billing.invalid_subscription_tier", "invalid subscription tier")
	// ErrSubscriptionExpiryRequired 付费订阅必须指定到期时间。
	ErrSubscriptionExpiryRequired = apperr.New("billing.subscription_expiry_required", "subscription expiry required")
	// ErrInvalidSubscriptionExpiry 非法订阅到期时间。
	ErrInvalidSubscriptionExpiry = apperr.New("billing.invalid_subscription_expiry", "invalid subscription expiry")
	// ErrInvalidBillingPlan 非法计费套餐。
	ErrInvalidBillingPlan = apperr.New("billing.invalid_plan", "invalid billing plan")
	// ErrBillingPlanNotFound 计费套餐不存在。
	ErrBillingPlanNotFound = apperr.New("billing.plan_not_found", "billing plan not found")
	// ErrInvalidPermissionGroup 非法权限组。
	ErrInvalidPermissionGroup = apperr.New("billing.invalid_permission_group", "invalid permission group")
	// ErrInvalidUsageStatisticsSubject 表示用量统计的用户与权限组筛选条件冲突。
	ErrInvalidUsageStatisticsSubject = apperr.New("usage_statistics.subject_conflict", "user and permission group filters are mutually exclusive")
	// ErrPermissionGroupReferenceCounterUnavailable 权限组套餐引用检查能力不可用。
	ErrPermissionGroupReferenceCounterUnavailable = apperr.NewMasked(
		"internal.error",
		"internal server error",
		"permission group reference counter unavailable",
	)
	// ErrSubscriptionEntitlementActive 当前仍存在有效付费订阅权益。
	ErrSubscriptionEntitlementActive = apperr.New("billing.subscription_entitlement_active", "subscription entitlement is active")
	// ErrRedemptionCodeHashUnavailable 兑换码哈希密钥不可用。
	ErrRedemptionCodeHashUnavailable = apperr.NewMasked("billing.redemption_secret_unavailable", "redemption code service is unavailable", "redemption code hash secret unavailable")
	// ErrInvalidRedemptionCode 兑换码格式或配置非法。
	ErrInvalidRedemptionCode = apperr.New("billing.invalid_redemption_code", "invalid redemption code")
	// ErrRedemptionCodeConflict 兑换码明文对应的哈希已存在。
	ErrRedemptionCodeConflict = apperr.New("billing.redemption_code_conflict", "redemption code already exists")
	// ErrRedemptionCodeUnavailable 兑换码不存在、停用、过期或与当前计费模式不匹配。
	ErrRedemptionCodeUnavailable = apperr.New("billing.redemption_code_unavailable", "redemption code is unavailable")
	// ErrRedemptionCodePlaintextUnavailable 兑换码未保存可解密密文，无法再次展示明文。
	ErrRedemptionCodePlaintextUnavailable = apperr.New("billing.redemption_code_plaintext_unavailable", "redemption code plaintext unavailable")
	// ErrRedemptionCodeExhausted 兑换码总次数已用完。
	ErrRedemptionCodeExhausted = apperr.New("billing.redemption_code_exhausted", "redemption code exhausted")
	// ErrRedemptionUserLimitExceeded 当前用户已达到兑换次数上限。
	ErrRedemptionUserLimitExceeded = apperr.New("billing.redemption_user_limit_exceeded", "redemption user limit exceeded")
	// ErrInvalidPaymentOrder 表示支付订单参数不符合业务约束。
	ErrInvalidPaymentOrder = apperr.New("payment.invalid_order", "invalid payment order")
	// ErrPaymentOrderNotFound 表示支付回调引用了不存在的订单。
	ErrPaymentOrderNotFound = apperr.New("payment.order_not_found", "payment order not found")
	// ErrPaymentOrderStateInvalid 表示支付订单当前状态不允许完成。
	ErrPaymentOrderStateInvalid = apperr.New("payment.order_state_invalid", "payment order state is invalid")
	// ErrInvalidBillingAccountBalance 表示管理员提交的余额无效。
	ErrInvalidBillingAccountBalance = apperr.New("billing.invalid_account_balance", "invalid billing account balance")
	// ErrInvalidNativeToolPricing 表示原生工具计费覆盖配置无效。
	ErrInvalidNativeToolPricing = apperr.New("billing.invalid_native_tool_pricing", "invalid native tool pricing")
)
