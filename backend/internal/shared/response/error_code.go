package response

import "net/http"

const (
	CodeRequestInvalidBody       = "request.invalid_body"
	CodeRequestInvalid           = "request.invalid"
	CodeRequestInvalidID         = "request.invalid_id"
	CodeRequestInvalidQuery      = "request.invalid_query"
	CodeRequestRequired          = "request.required"
	CodeAuthUnauthorized         = "auth.unauthorized"
	CodeAuthForbidden            = "auth.forbidden"
	CodeAuthInvalidToken         = "auth.invalid_token"
	CodeAuthInvalidCredentials   = "auth.invalid_credentials"
	CodeAuthInvalidCurrentPass   = "auth.invalid_current_password"
	CodeAuthInvalidRefreshToken  = "auth.invalid_refresh_token"
	CodeAuthInvalidTwoFactorCode = "auth.invalid_two_factor_code"
	CodeAuthTwoFactorExpired     = "auth.two_factor_expired"
	CodeAuthTwoFactorNotStarted  = "auth.two_factor_not_started"
	CodeAuthLastLoginRequired    = "auth.last_login_method_required"
	CodeAuthSessionInvalid       = "auth.session_invalid"
	CodeResourceNotFound         = "resource.not_found"
	CodeResourceConflict         = "resource.conflict"
	CodeBillingPaymentRequired   = "billing.payment_required"
	CodeBillingInsufficientFunds = "billing.insufficient_funds"
	CodeBillingPricingRequired   = "billing.pricing_required"
	CodeRateLimitExceeded        = "rate_limit.exceeded"
	CodeQuotaExceeded            = "quota.exceeded"
	CodeFileInUse                = "file.in_use"
	CodeFileTooLarge             = "file.too_large"
	CodeFileNotReady             = "file.not_ready"
	CodeFileTypeBlocked          = "file.type_blocked"
	CodeUpstreamUnavailable      = "upstream.unavailable"
	CodeUpstreamRateLimited      = "upstream.rate_limited"
	CodeServiceUnavailable       = "service.unavailable"
	CodeInternal                 = "internal.error"
)

// codeMessages 只登记由响应边界直接选择的错误码。应用层错误的文案与错误码由 apperr 自身声明，
// 不在这里重复维护。
var codeMessages = map[string]string{
	CodeRequestInvalidBody:       "invalid request body",
	CodeRequestInvalid:           "invalid request",
	CodeRequestInvalidID:         "invalid id",
	CodeRequestInvalidQuery:      "invalid query parameter",
	CodeRequestRequired:          "required field missing",
	CodeAuthUnauthorized:         "unauthorized",
	CodeAuthForbidden:            "forbidden",
	CodeAuthInvalidToken:         "invalid token",
	CodeAuthInvalidCredentials:   "invalid username or password",
	CodeAuthInvalidCurrentPass:   "invalid current password",
	CodeAuthInvalidRefreshToken:  "invalid refresh token",
	CodeAuthInvalidTwoFactorCode: "invalid two factor code",
	CodeAuthTwoFactorExpired:     "two factor challenge expired",
	CodeAuthTwoFactorNotStarted:  "two factor setup not started",
	CodeAuthLastLoginRequired:    "set a password or bind another identity provider first",
	CodeAuthSessionInvalid:       "session invalid",
	CodeResourceNotFound:         "resource not found",
	CodeResourceConflict:         "resource conflict",
	CodeBillingPaymentRequired:   "payment required",
	CodeBillingInsufficientFunds: "insufficient balance",
	CodeBillingPricingRequired:   "model pricing is required",
	CodeRateLimitExceeded:        "rate limit exceeded",
	CodeQuotaExceeded:            "quota exceeded",
	CodeFileInUse:                "file is in use",
	CodeFileTooLarge:             "file too large",
	CodeFileNotReady:             "file is not ready",
	CodeFileTypeBlocked:          "file type is not allowed",
	CodeUpstreamUnavailable:      "upstream service unavailable",
	CodeUpstreamRateLimited:      "upstream rate limited",
	CodeServiceUnavailable:       "service unavailable",
	CodeInternal:                 "internal server error",

	"auth.provider_email_conflict":                  "provider email belongs to another account",
	"billing.invalid_redemption_code":               "invalid redemption code",
	"content_moderation.config_required":            "content moderation service config and policy are required when enabled",
	"content_moderation.invalid_config":             "invalid content moderation config",
	"content_moderation.probe_failed":               "content moderation probe failed",
	"conversation.message_fork_history_incomplete":  "message history is too deep or incomplete",
	"conversation.message_fork_state_invalid":       "message is still generating",
	"conversation.message_fork_target_invalid":      "only assistant messages can be forked",
	"cors.origin_forbidden":                         "origin is not allowed",
	"embedding.service_not_configured":              "embedding service is not configured",
	"embedding.service_unavailable":                 "embedding service is not available",
	"embedding.submit_failed":                       "failed to submit embedding jobs",
	"embedding.too_many_files":                      "too many files for embedding",
	"file.not_found":                                "file not found",
	"identity_provider.delete_conflict":             "deleting this identity provider would remove the only login method for some users",
	"knowledge_base.conflict":                       "knowledge base conflict",
	"knowledge_base.disabled":                       "knowledge base feature is disabled",
	"knowledge_base.file_cleanup_unavailable":       "platform file cleanup unavailable",
	"knowledge_base.internal":                       "knowledge base operation failed",
	"knowledge_base.invalid":                        "invalid knowledge base request",
	"knowledge_base.not_found":                      "knowledge base not found",
	"knowledge_base.owner_file_reference":           "user owns files referenced by builtin knowledge bases",
	"knowledge_base.platform_file_in_use":           "platform file is in use",
	"llm.empty_response":                            "model returned empty response",
	"llm.model_icon_asset_in_use":                   "model icon asset is in use",
	"llm.model_vendor_builtin":                      "built-in model vendor cannot be deleted",
	"llm.model_vendor_in_use":                       "model vendor is in use",
	"llm.remote_models_empty_confirmation_required": "remote models snapshot is empty",
	"llm.remote_models_snapshot_changed":            "remote models snapshot changed",
	"llm.upstream_model_binding_changed":            "upstream model binding changed; reload and retry",
	"llm.upstream_model_conflict":                   "model upstream source conflict",
	"media.artifact_unavailable":                    "generated media artifact is temporarily unavailable",
	"media.image_stream_unsupported":                "upstream may not support image streaming; disable image.stream for this model",
	"payment.checkout_failed":                       "create checkout failed",
	"payment.epay_gateway_invalid":                  "epay gateway url is invalid",
	"payment.provider_unavailable":                  "payment provider is unavailable",
	"settings.invalid_namespace":                    "invalid setting namespace",
	"settings.invalid_key":                          "invalid setting key",
	"settings.invalid_value":                        "invalid setting value",
	"settings.smtp_invalid":                         "invalid SMTP settings",
	"settings.billing_payment_invalid":              "invalid billing payment settings",
	"settings.embedding_invalid":                    "invalid embedding settings",
	"settings.extract_invalid":                      "invalid file extraction settings",
	"settings.model_option_policy_invalid":          "invalid model option policy settings",
	"skill.too_many_ids":                            "too many skill ids",
	"usage_statistics.invalid_billing_scope":        "invalid billing scope",
	"usage_statistics.invalid_date_range":           "invalid usage statistics date range",
	"usage_statistics.invalid_rank_by":              "invalid usage ranking field",
	"usage_statistics.invalid_section":              "invalid usage statistics section",
	"usage_statistics.subject_conflict":             "user and permission group filters are mutually exclusive",
}

func canonicalMessage(code string) (string, bool) {
	message, ok := codeMessages[code]
	return message, ok
}

func defaultDescription(status int) Description {
	switch status {
	case http.StatusUnauthorized:
		return Description{Status: status, Code: CodeAuthUnauthorized, Message: codeMessages[CodeAuthUnauthorized]}
	case http.StatusForbidden:
		return Description{Status: status, Code: CodeAuthForbidden, Message: codeMessages[CodeAuthForbidden]}
	case http.StatusNotFound:
		return Description{Status: status, Code: CodeResourceNotFound, Message: codeMessages[CodeResourceNotFound]}
	case http.StatusConflict:
		return Description{Status: status, Code: CodeResourceConflict, Message: codeMessages[CodeResourceConflict]}
	case http.StatusPaymentRequired:
		return Description{Status: status, Code: CodeBillingPaymentRequired, Message: codeMessages[CodeBillingPaymentRequired]}
	case http.StatusTooManyRequests:
		return Description{Status: status, Code: CodeRateLimitExceeded, Message: codeMessages[CodeRateLimitExceeded]}
	case http.StatusBadGateway:
		return Description{Status: status, Code: CodeUpstreamUnavailable, Message: codeMessages[CodeUpstreamUnavailable]}
	case http.StatusServiceUnavailable:
		return Description{Status: status, Code: CodeServiceUnavailable, Message: codeMessages[CodeServiceUnavailable]}
	default:
		if status >= http.StatusInternalServerError {
			return Description{Status: status, Code: CodeInternal, Message: codeMessages[CodeInternal]}
		}
		return Description{Status: status, Code: CodeRequestInvalid, Message: codeMessages[CodeRequestInvalid]}
	}
}
