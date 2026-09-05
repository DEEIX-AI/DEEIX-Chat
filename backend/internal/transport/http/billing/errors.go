package billing

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/apperr"

// 由 transport 层直接判定的请求错误（路径参数、必填校验、运行时依赖缺失等）。
// 错误码与文案是前端依赖的 API 契约，改动需同步 frontend/i18n/messages/*/errors.json。
var (
	errInvalidDailyUsageDateRange  = apperr.New("billing.invalid_daily_usage_date_range", "invalid daily usage date range")
	errInvalidDailyUsageDays       = apperr.New("billing.invalid_daily_usage_days", "invalid daily usage days")
	errInvalidPlanID               = apperr.New("plan.invalid_id", "invalid plan id")
	errInvalidRedemptionCodeID     = apperr.New("redemption_code.invalid_id", "invalid redemption code id")
	errInvalidStripeEvent          = apperr.New("payment.invalid_event", "invalid stripe event")
	errInvalidStripeSignature      = apperr.New("payment.invalid_signature", "invalid stripe signature")
	errInvalidUserID               = apperr.New("user.invalid_id", "invalid user id")
	errInvalidWebhookBody          = apperr.New("payment.invalid_webhook_body", "invalid webhook body")
	errOrderNoRequired             = apperr.New("payment.order_no_required", "order_no is required")
	errPaymentNotificationMismatch = apperr.New("payment.notification_mismatch", "payment notification does not match the order")
	errPaymentProviderUnavailable  = apperr.New("payment.provider_unavailable", "payment provider is unavailable")
	errPlatformModelNameRequired   = apperr.New("llm.platform_model_name_required", "platform model name is required")
	errSettingsServiceUnavailable  = apperr.New("settings.service_unavailable", "settings service unavailable")
	errStripeWebhookNotConfigured  = apperr.New("payment.webhook_not_configured", "stripe webhook is not configured")
	errUpstreamServiceUnavailable  = apperr.New("upstream.unavailable", "upstream service unavailable")
	errWebhookBodyTooLarge         = apperr.New("payment.webhook_body_too_large", "webhook body too large")
)
