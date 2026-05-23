package response

import (
	"net/http"
	"strings"
)

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
	CodeFileTooLarge             = "file.too_large"
	CodeFileNotReady             = "file.not_ready"
	CodeFileTypeBlocked          = "file.type_blocked"
	CodeUpstreamUnavailable      = "upstream.unavailable"
	CodeServiceUnavailable       = "service.unavailable"
	CodeInternal                 = "internal.error"
)

type errorSpec struct {
	Code    string
	Message string
}

var exactErrorSpecs = map[string]errorSpec{
	"unauthorized":                                               {Code: CodeAuthUnauthorized, Message: "unauthorized"},
	"forbidden":                                                  {Code: CodeAuthForbidden, Message: "forbidden"},
	"admin permission required":                                  {Code: "auth.admin_required", Message: "admin permission required"},
	"superadmin permission required":                             {Code: "auth.superadmin_required", Message: "superadmin permission required"},
	"missing authorization header":                               {Code: CodeAuthInvalidToken, Message: "authorization header is required"},
	"invalid authorization header":                               {Code: CodeAuthInvalidToken, Message: "invalid authorization header"},
	"invalid token":                                              {Code: CodeAuthInvalidToken, Message: "invalid token"},
	"invalid token type":                                         {Code: CodeAuthInvalidToken, Message: "invalid token type"},
	"session invalid":                                            {Code: CodeAuthSessionInvalid, Message: "session invalid"},
	"invalid username or password":                               {Code: CodeAuthInvalidCredentials, Message: "invalid username or password"},
	"invalid current password":                                   {Code: CodeAuthInvalidCurrentPass, Message: "invalid current password"},
	"invalid refresh token":                                      {Code: CodeAuthInvalidRefreshToken, Message: "invalid refresh token"},
	"session revoked":                                            {Code: CodeAuthSessionInvalid, Message: "session invalid"},
	"invalid two factor code":                                    {Code: CodeAuthInvalidTwoFactorCode, Message: "invalid two factor code"},
	"two factor challenge expired":                               {Code: CodeAuthTwoFactorExpired, Message: "two factor challenge expired"},
	"two factor setup expired":                                   {Code: CodeAuthTwoFactorExpired, Message: "two factor setup expired"},
	"two factor setup not started":                               {Code: CodeAuthTwoFactorNotStarted, Message: "two factor setup not started"},
	"two factor setup was not persisted":                         {Code: CodeInternal, Message: "internal server error"},
	"two factor authentication is already enabled":               {Code: "auth.two_factor_already_enabled", Message: "two factor authentication is already enabled"},
	"password reset required":                                    {Code: "auth.password_reset_required", Message: "password reset required"},
	"username change required":                                   {Code: "auth.username_change_required", Message: "username change required"},
	"password length must be between 6 and 128":                  {Code: "auth.invalid_password", Message: "password length must be between 6 and 128"},
	"invalid password":                                           {Code: "auth.invalid_password", Message: "password must be at least 8 characters and not digits only"},
	"new password must be different from the bootstrap password": {Code: "auth.password_reuse_not_allowed", Message: "new password must be different from the bootstrap password"},
	"account locked":                                             {Code: CodeAuthInvalidCredentials, Message: "invalid username or password"},
	"cannot unlink the last available login method":              {Code: CodeAuthLastLoginRequired, Message: "set a password or bind another identity provider first"},
	"configure the provider callback url to the frontend callback endpoint": {Code: "auth.provider_callback_misconfigured", Message: "configure the provider callback URL to the frontend callback endpoint"},
	"email registration is disabled":                                        {Code: "auth.email_registration_disabled", Message: "email registration is disabled"},
	"email verification is disabled":                                        {Code: "auth.email_verification_disabled", Message: "email verification is disabled"},
	"email already exists":                                                  {Code: "auth.email_already_exists", Message: "email already exists"},
	"email already exists; bind the provider before login":                  {Code: "auth.provider_email_conflict", Message: "email already exists; bind the provider before login"},
	"user email is invalid":                                                 {Code: "auth.invalid_email", Message: "invalid email"},
	"admin email is invalid":                                                {Code: "auth.invalid_email", Message: "invalid email"},
	"invalid email":                                                         {Code: "auth.invalid_email", Message: "invalid email"},
	"user email is not verified":                                            {Code: "auth.email_not_verified", Message: "email is not verified"},
	"admin email is not verified":                                           {Code: "auth.email_not_verified", Message: "email is not verified"},
	"current email is not verified":                                         {Code: "auth.email_not_verified", Message: "email is not verified"},
	"new email must be different":                                           {Code: "auth.email_unchanged", Message: "new email must be different"},
	"email aliases are not allowed":                                         {Code: "auth.email_alias_not_allowed", Message: "email aliases are not allowed"},
	"email domain is not allowed":                                           {Code: "auth.email_domain_not_allowed", Message: "email domain is not allowed"},
	"email bootstrap is not allowed":                                        {Code: "auth.email_bootstrap_not_allowed", Message: "email bootstrap is not allowed"},
	"third-party login is disabled":                                         {Code: "auth.provider_login_disabled", Message: "third-party login is disabled"},
	"authorization code is required":                                        {Code: "auth.authorization_code_required", Message: "authorization code is required"},
	"provider id is required":                                               {Code: "auth.provider_id_required", Message: "provider id is required"},
	"provider bind must use account binding endpoint":                       {Code: "auth.provider_bind_endpoint_required", Message: "provider bind must use account binding endpoint"},
	"provider email belongs to another account; sign in to that account or change its email before binding": {
		Code:    "auth.provider_email_conflict",
		Message: "provider email belongs to another account",
	},
	"invalid redirect uri":                                        {Code: "auth.invalid_redirect_uri", Message: "invalid redirect uri"},
	"redirect uri origin is not allowed":                          {Code: "auth.invalid_redirect_uri", Message: "redirect uri origin is not allowed"},
	"provider ids must be unique":                                 {Code: "auth.provider_order_invalid", Message: "provider ids must be unique"},
	"provider type must be oidc or oauth2":                        {Code: "auth.provider_type_invalid", Message: "provider type must be oidc or oauth2"},
	"provider name is required":                                   {Code: "auth.provider_name_required", Message: "provider name is required"},
	"provider slug is required":                                   {Code: "auth.provider_slug_required", Message: "provider slug is required"},
	"default role must be user, admin or superadmin":              {Code: "auth.provider_default_role_invalid", Message: "default role must be user, admin or superadmin"},
	"only superadmin can set superadmin default role":             {Code: "auth.provider_superadmin_default_role_protected", Message: "only superadmin can set superadmin default role"},
	"logo url must be a valid http(s) or absolute path":           {Code: "auth.provider_logo_url_invalid", Message: "logo url must be a valid http(s) or absolute path"},
	"provider registration requires provider login to be enabled": {Code: "auth.provider_registration_requires_login", Message: "provider registration requires provider login to be enabled"},
	"client id is required":                                       {Code: "auth.provider_client_id_required", Message: "client id is required"},
	"client secret is required":                                   {Code: "auth.provider_client_secret_required", Message: "client secret is required"},
	"oidc issuer url or discovery url is required":                {Code: "auth.provider_oidc_issuer_required", Message: "OIDC issuer url or discovery url is required"},
	"oauth2 auth url, token url and userinfo url are required":    {Code: "auth.provider_oauth_urls_required", Message: "OAuth2 auth url, token url and userinfo url are required"},
	"provider auth url is not configured":                         {Code: "auth.provider_auth_url_not_configured", Message: "provider auth url is not configured"},

	"invalid time zone":                       {Code: "user.invalid_time_zone", Message: "invalid time zone"},
	"invalid timezone":                        {Code: "user.invalid_time_zone", Message: "invalid time zone"},
	"invalid location":                        {Code: "user.invalid_location", Message: "invalid location"},
	"invalid avatar url":                      {Code: "user.invalid_avatar_url", Message: "invalid avatar url"},
	"invalid username":                        {Code: "user.invalid_username", Message: "invalid username"},
	"invalid display name":                    {Code: "user.invalid_display_name", Message: "invalid display name"},
	"invalid user email":                      {Code: "user.invalid_email", Message: "invalid user email"},
	"invalid user phone":                      {Code: "user.invalid_phone", Message: "invalid user phone"},
	"invalid user locale":                     {Code: "user.invalid_locale", Message: "invalid user locale"},
	"invalid user status":                     {Code: "user.invalid_status", Message: "invalid user status"},
	"invalid user role":                       {Code: "user.invalid_role", Message: "invalid user role"},
	"invalid user timezone":                   {Code: "user.invalid_time_zone", Message: "invalid time zone"},
	"username already exists":                 {Code: "user.username_already_exists", Message: "username already exists"},
	"username change already used":            {Code: "user.username_change_used", Message: "username change already used"},
	"superadmin account deletion not allowed": {Code: "user.superadmin_delete_protected", Message: "superadmin account deletion is not allowed"},
	"account deletion requires verification":  {Code: "user.account_delete_verification_required", Message: "account deletion requires verification"},
	"superadmin delete not allowed":           {Code: "user.superadmin_delete_protected", Message: "superadmin account deletion is not allowed"},
	"superadmin status change not allowed":    {Code: "user.superadmin_status_protected", Message: "superadmin status change is not allowed"},
	"superadmin password reset not allowed":   {Code: "user.superadmin_password_reset_protected", Message: "superadmin password reset is not allowed"},
	"superadmin two factor reset not allowed": {Code: "user.superadmin_two_factor_reset_protected", Message: "superadmin two factor reset is not allowed"},
	"superadmin management not allowed":       {Code: "user.superadmin_management_protected", Message: "superadmin management is not allowed"},
	"last superadmin role change not allowed": {Code: "user.last_superadmin_role_protected", Message: "last superadmin role change is not allowed"},
	"self role change not allowed":            {Code: "user.self_role_change_not_allowed", Message: "self role change is not allowed"},
	"self status change not allowed":          {Code: "user.self_status_change_not_allowed", Message: "self status change is not allowed"},
	"self delete not allowed":                 {Code: "user.self_delete_not_allowed", Message: "self delete is not allowed"},
	"empty admin user patch":                  {Code: "user.empty_patch", Message: "at least one user field is required"},

	"invalid conversation title":                              {Code: "conversation.invalid_title", Message: "invalid conversation title"},
	"invalid conversation share":                              {Code: "conversation_share.invalid", Message: "invalid conversation share"},
	"conversation share schema outdated":                      {Code: "conversation_share.schema_outdated", Message: "conversation share schema is outdated"},
	"conversation share schema is outdated, rebuild database": {Code: "conversation_share.schema_outdated", Message: "conversation share schema is outdated"},
	"message feedback target invalid":                         {Code: "message.feedback_target_invalid", Message: "message feedback target invalid"},
	"invalid message feedback":                                {Code: "message.invalid_feedback", Message: "invalid message feedback"},
	"invalid message branch":                                  {Code: "message.invalid_branch", Message: "invalid message branch"},
	"message generation canceled":                             {Code: "conversation_run.canceled", Message: "message generation canceled"},
	"too many files in one message":                           {Code: "message.too_many_files", Message: "too many files in one message"},
	"generation stream not found":                             {Code: "conversation_run.stream_not_found", Message: "generation stream not found"},

	"file is required":                                     {Code: "file.required", Message: "file is required"},
	"invalid file stream":                                  {Code: "file.invalid_stream", Message: "invalid file stream"},
	"invalid file reference":                               {Code: "file.invalid_reference", Message: "invalid file reference"},
	"invalid file name":                                    {Code: "file.invalid_name", Message: "invalid file name"},
	"storage quota exceeded":                               {Code: CodeQuotaExceeded, Message: "storage quota exceeded"},
	"dangerous file type not allowed":                      {Code: CodeFileTypeBlocked, Message: "file type is not allowed"},
	"mime blocked":                                         {Code: CodeFileTypeBlocked, Message: "file type is not allowed"},
	"embedding unavailable":                                {Code: "file.embedding_unavailable", Message: "embedding is unavailable"},
	"embedding unavailable for this file size":             {Code: "file.embedding_unavailable", Message: "embedding is unavailable for this file size"},
	"embedding unavailable for current file capability":    {Code: "file.embedding_unavailable", Message: "embedding is unavailable for current file capability"},
	"file too large":                                       {Code: CodeFileTooLarge, Message: "file too large"},
	"file processing not ready":                            {Code: CodeFileNotReady, Message: "file processing is not ready"},
	"file extract not ready":                               {Code: "file.extract_not_ready", Message: "file extract is not ready"},
	"file too large for full context":                      {Code: "file.too_large_for_context", Message: "file is too large for full context"},
	"at least one of file_name or rag_opt_out is required": {Code: CodeRequestRequired, Message: "at least one of file_name or rag_opt_out is required"},

	"model route not configured":                  {Code: "llm.model_route_not_configured", Message: "model route is not configured"},
	"model returned empty response":               {Code: "llm.empty_response", Message: "model returned empty response"},
	"upstream returned empty response":            {Code: "llm.empty_response", Message: "model returned empty response"},
	"remote models unavailable":                   {Code: "llm.remote_models_unavailable", Message: "remote models unavailable"},
	"no active api key":                           {Code: "llm.no_active_api_key", Message: "no active api key"},
	"invalid adapter":                             {Code: "llm.invalid_adapter", Message: "invalid adapter"},
	"invalid compatible":                          {Code: "llm.invalid_compatible", Message: "invalid compatible"},
	"invalid json config":                         {Code: "config.invalid_json", Message: "invalid json config"},
	"invalid headers config":                      {Code: "llm.invalid_headers_config", Message: "invalid headers json config"},
	"invalid headers json config":                 {Code: "llm.invalid_headers_config", Message: "invalid headers json config"},
	"invalid api keys config":                     {Code: "llm.invalid_api_keys_config", Message: "invalid api keys config"},
	"invalid protocol defaults config":            {Code: "llm.invalid_protocol_defaults_config", Message: "invalid protocol defaults config"},
	"invalid kinds":                               {Code: "llm.invalid_kinds", Message: "invalid model kinds"},
	"invalid platform model name":                 {Code: "llm.invalid_platform_model_name", Message: "invalid platform model name"},
	"system prompt too long":                      {Code: "llm.system_prompt_too_long", Message: "system prompt too long"},
	"platform model name is required":             {Code: "llm.platform_model_name_required", Message: "platform model name is required"},
	"protocol required":                           {Code: "llm.protocol_required", Message: "protocol is required"},
	"platform model name already exists":          {Code: "llm.platform_model_name_exists", Message: "platform model name already exists"},
	"target model already bound on this upstream": {Code: "llm.route_conflict", Message: "target model is already bound on this upstream"},
	"all routes unavailable":                      {Code: "llm.routes_unavailable", Message: "all model routes are unavailable"},
	"route not found":                             {Code: "route.not_found", Message: "route not found"},
	"api_keys is required":                        {Code: "llm.api_keys_required", Message: "api_keys is required"},

	"usage balance is insufficient":                                {Code: CodeBillingInsufficientFunds, Message: "insufficient balance"},
	"model pricing is required":                                    {Code: CodeBillingPricingRequired, Message: "model pricing is required"},
	"period usage credit exceeded":                                 {Code: "billing.period_credit_exceeded", Message: "period usage credit exceeded"},
	"invalid subscription tier":                                    {Code: "billing.invalid_subscription_tier", Message: "invalid subscription tier"},
	"subscription expiry required":                                 {Code: "billing.subscription_expiry_required", Message: "subscription expiry required"},
	"invalid subscription expiry":                                  {Code: "billing.invalid_subscription_expiry", Message: "invalid subscription expiry"},
	"invalid model pricing":                                        {Code: "billing.invalid_model_pricing", Message: "invalid model pricing"},
	"invalid daily usage date range":                               {Code: "billing.invalid_daily_usage_date_range", Message: "invalid daily usage date range"},
	"invalid daily usage days":                                     {Code: "billing.invalid_daily_usage_days", Message: "invalid daily usage days"},
	"payment is required":                                          {Code: CodeBillingPaymentRequired, Message: "payment is required"},
	"payment provider is unavailable":                              {Code: "payment.provider_unavailable", Message: "payment provider is unavailable"},
	"create checkout failed":                                       {Code: "payment.checkout_failed", Message: "create checkout failed"},
	"provider mismatch":                                            {Code: "payment.notification_mismatch", Message: "payment notification does not match the order"},
	"checkout id mismatch":                                         {Code: "payment.notification_mismatch", Message: "payment notification does not match the order"},
	"amount mismatch":                                              {Code: "payment.notification_mismatch", Message: "payment notification does not match the order"},
	"currency mismatch":                                            {Code: "payment.notification_mismatch", Message: "payment notification does not match the order"},
	"merchant mismatch":                                            {Code: "payment.notification_mismatch", Message: "payment notification does not match the order"},
	"epay payment type is not supported":                           {Code: "payment.epay_type_unsupported", Message: "epay payment type is not supported"},
	"payment return url is invalid":                                {Code: "payment.return_url_invalid", Message: "payment return url is invalid"},
	"payment return url must use the configured public web origin": {Code: "payment.return_url_cross_origin", Message: "payment return url must use the configured public web origin"},
	"stripe webhook is not configured":                             {Code: "payment.webhook_not_configured", Message: "stripe webhook is not configured"},
	"read webhook body failed":                                     {Code: "payment.invalid_webhook_body", Message: "invalid webhook body"},
	"webhook body too large":                                       {Code: "payment.webhook_body_too_large", Message: "webhook body too large"},
	"invalid stripe signature":                                     {Code: "payment.invalid_signature", Message: "invalid stripe signature"},
	"invalid stripe event":                                         {Code: "payment.invalid_event", Message: "invalid stripe event"},
	"missing order_no":                                             {Code: "payment.order_no_required", Message: "order_no is required"},

	"invalid namespace":                     {Code: "settings.invalid_namespace", Message: "invalid namespace"},
	"invalid setting key":                   {Code: "settings.invalid_key", Message: "invalid setting key"},
	"setting not found":                     {Code: "settings.not_found", Message: "setting not found"},
	"settings service unavailable":          {Code: "settings.service_unavailable", Message: "settings service unavailable"},
	"invalid id":                            {Code: CodeRequestInvalidID, Message: "invalid id"},
	"embedding service not available":       {Code: "embedding.service_unavailable", Message: "embedding service is not available"},
	"tika runtime service unavailable":      {Code: "runtime.tika_unavailable", Message: "tika runtime service unavailable"},
	"docling runtime service unavailable":   {Code: "runtime.docling_unavailable", Message: "docling runtime service unavailable"},
	"tesseract runtime service unavailable": {Code: "runtime.tesseract_unavailable", Message: "tesseract runtime service unavailable"},
	"rapidocr runtime service unavailable":  {Code: "runtime.rapidocr_unavailable", Message: "rapidocr runtime service unavailable"},
	"mineru runtime service unavailable":    {Code: "runtime.mineru_unavailable", Message: "mineru runtime service unavailable"},

	"memory_key is required":          {Code: "memory.key_required", Message: "memory_key is required"},
	"invalid mcp server id":           {Code: "mcp.server.invalid_id", Message: "invalid mcp server id"},
	"invalid mcp tool id":             {Code: "mcp.tool.invalid_id", Message: "invalid mcp tool id"},
	"invalid mcp server name":         {Code: "mcp.invalid_server_name", Message: "invalid mcp server name"},
	"invalid mcp server base url":     {Code: "mcp.invalid_server_base_url", Message: "invalid mcp server base url"},
	"invalid mcp server status":       {Code: "mcp.invalid_server_status", Message: "invalid mcp server status"},
	"invalid mcp server headers json": {Code: "mcp.invalid_server_headers", Message: "invalid mcp server headers json"},
	"invalid mcp tool status":         {Code: "mcp.invalid_tool_status", Message: "invalid mcp tool status"},
	"invalid mcp tool display name":   {Code: "mcp.invalid_tool_name", Message: "invalid mcp tool display name"},
	"invalid mcp tool description":    {Code: "mcp.invalid_tool_description", Message: "invalid mcp tool description"},
	"invalid mcp tool selection":      {Code: "mcp.invalid_tool_selection", Message: "invalid mcp tool selection"},
	"mcp client unavailable":          {Code: "mcp.client_unavailable", Message: "mcp client unavailable"},

	"rate limit exceeded":              {Code: CodeRateLimitExceeded, Message: "rate limit exceeded"},
	"too many refresh attempts":        {Code: "rate_limit.refresh_exceeded", Message: "too many refresh attempts"},
	"too many authentication attempts": {Code: "rate_limit.authentication_exceeded", Message: "too many authentication attempts"},

	"deleting this identity provider would remove the only login method for some users": {Code: "identity_provider.delete_conflict", Message: "deleting this identity provider would remove the only login method for some users"},
}

// InferErrorCode provides a compatibility code for legacy response.Error calls.
// New code should prefer ErrorWithCode/ErrorWithDetails with an explicit code.
func InferErrorCode(status int, msg string) string {
	if spec, ok := resolveErrorSpec(status, msg); ok {
		return spec.Code
	}
	switch {
	case status == http.StatusBadGateway:
		return CodeUpstreamUnavailable
	case status == http.StatusServiceUnavailable:
		return CodeServiceUnavailable
	case status >= http.StatusInternalServerError:
		return CodeInternal
	}
	text := normalizeErrorText(msg)
	switch {
	case strings.Contains(text, "invalid request body"):
		return CodeRequestInvalidBody
	case strings.Contains(text, "invalid ") && strings.Contains(text, " id"):
		return invalidIDCode(text)
	case strings.Contains(text, "is required"):
		return CodeRequestRequired
	case strings.Contains(text, "not found"):
		return notFoundCode(text)
	case strings.Contains(text, "already exists") || strings.Contains(text, "conflict"):
		return CodeResourceConflict
	case strings.Contains(text, "quota exceeded") || strings.Contains(text, "exceeded"):
		return CodeQuotaExceeded
	case strings.Contains(text, "insufficient"):
		return CodeBillingInsufficientFunds
	case strings.Contains(text, "pricing"):
		return CodeBillingPricingRequired
	case strings.Contains(text, "payment required"):
		return CodeBillingPaymentRequired
	case strings.Contains(text, "file too large"):
		return CodeFileTooLarge
	case strings.Contains(text, "file processing not ready") || strings.Contains(text, "file extract not ready"):
		return CodeFileNotReady
	case strings.Contains(text, "mime blocked") || strings.Contains(text, "dangerous file type"):
		return CodeFileTypeBlocked
	case strings.Contains(text, "remote models unavailable") || strings.Contains(text, "model route not configured"):
		return CodeUpstreamUnavailable
	case strings.Contains(text, "verification code"):
		return "auth.verification_code_invalid"
	}

	switch status {
	case http.StatusBadRequest:
		return CodeRequestInvalid
	case http.StatusUnauthorized:
		return CodeAuthUnauthorized
	case http.StatusForbidden:
		return CodeAuthForbidden
	case http.StatusNotFound:
		return CodeResourceNotFound
	case http.StatusConflict:
		return CodeResourceConflict
	case http.StatusPaymentRequired:
		return CodeBillingPaymentRequired
	case http.StatusTooManyRequests:
		return CodeRateLimitExceeded
	case http.StatusBadGateway:
		return CodeUpstreamUnavailable
	case http.StatusServiceUnavailable:
		return CodeServiceUnavailable
	default:
		if status >= http.StatusInternalServerError {
			return CodeInternal
		}
		return CodeRequestInvalid
	}
}

// PublicErrorMessage normalizes legacy handler messages into a safe API fallback.
// It intentionally preserves client-side validation context while hiding 5xx
// internals behind requestId + server logs.
func PublicErrorMessage(status int, code string, msg string) string {
	msg = strings.TrimSpace(msg)
	if spec, ok := resolveErrorSpec(status, msg); ok {
		return spec.Message
	}
	if msg == "" {
		msg = fallbackMessage(status, code)
	}

	switch {
	case status >= http.StatusInternalServerError:
		return fallbackMessage(status, code)
	case status == http.StatusBadGateway:
		return fallbackMessage(status, code)
	case status == http.StatusServiceUnavailable:
		return fallbackMessage(status, code)
	}

	switch code {
	case CodeAuthUnauthorized:
		return "unauthorized"
	case CodeAuthForbidden:
		return "forbidden"
	case CodeRateLimitExceeded:
		return "rate limit exceeded"
	default:
		return msg
	}
}

func fallbackMessage(status int, code string) string {
	if msg, ok := fallbackMessages[code]; ok {
		return msg
	}
	switch code {
	case CodeRequestInvalidBody:
		return "invalid request body"
	case CodeRequestInvalidID:
		return "invalid id"
	case CodeRequestRequired:
		return "required field missing"
	case CodeAuthUnauthorized:
		return "unauthorized"
	case CodeAuthForbidden:
		return "forbidden"
	case CodeAuthInvalidToken:
		return "invalid token"
	case CodeAuthInvalidCredentials:
		return "invalid username or password"
	case CodeAuthInvalidCurrentPass:
		return "invalid current password"
	case CodeAuthInvalidRefreshToken:
		return "invalid refresh token"
	case CodeAuthInvalidTwoFactorCode:
		return "invalid two factor code"
	case CodeAuthTwoFactorExpired:
		return "two factor challenge expired"
	case CodeAuthTwoFactorNotStarted:
		return "two factor setup not started"
	case CodeAuthLastLoginRequired:
		return "set a password or bind another identity provider first"
	case CodeAuthSessionInvalid:
		return "session invalid"
	case CodeResourceNotFound:
		return "resource not found"
	case CodeResourceConflict:
		return "resource conflict"
	case CodeBillingInsufficientFunds:
		return "insufficient balance"
	case CodeBillingPricingRequired:
		return "model pricing is required"
	case CodeBillingPaymentRequired:
		return "payment required"
	case CodeQuotaExceeded:
		return "quota exceeded"
	case CodeFileTooLarge:
		return "file too large"
	case CodeFileNotReady:
		return "file is not ready"
	case CodeFileTypeBlocked:
		return "file type is not allowed"
	case CodeUpstreamUnavailable:
		return "upstream service unavailable"
	case CodeServiceUnavailable:
		return "service unavailable"
	}
	switch status {
	case http.StatusBadRequest:
		return "invalid request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "resource not found"
	case http.StatusConflict:
		return "resource conflict"
	case http.StatusPaymentRequired:
		return "payment required"
	case http.StatusTooManyRequests:
		return "rate limit exceeded"
	case http.StatusBadGateway:
		return "upstream service unavailable"
	case http.StatusServiceUnavailable:
		return "service unavailable"
	default:
		if status >= http.StatusInternalServerError {
			return "internal server error"
		}
		return "request failed"
	}
}

var fallbackMessages = map[string]string{
	CodeRequestInvalidQuery:                           "invalid query parameter",
	"auth.admin_required":                             "admin permission required",
	"auth.superadmin_required":                        "superadmin permission required",
	"auth.password_reset_required":                    "password reset required",
	"auth.username_change_required":                   "username change required",
	"auth.invalid_password":                           "invalid password",
	"auth.password_reuse_not_allowed":                 "new password must be different",
	"auth.provider_callback_misconfigured":            "configure the provider callback URL to the frontend callback endpoint",
	"auth.verification_code_invalid":                  "verification code is invalid or expired",
	"auth.verification_code_recent":                   "verification code was sent recently",
	"auth.email_registration_disabled":                "email registration is disabled",
	"auth.email_verification_disabled":                "email verification is disabled",
	"auth.email_already_exists":                       "email already exists",
	"auth.email_not_verified":                         "email is not verified",
	"auth.email_unchanged":                            "new email must be different",
	"auth.email_alias_not_allowed":                    "email aliases are not allowed",
	"auth.email_domain_not_allowed":                   "email domain is not allowed",
	"auth.email_bootstrap_not_allowed":                "email bootstrap is not allowed",
	"auth.provider_login_disabled":                    "provider login is disabled",
	"auth.provider_registration_disabled":             "provider registration is disabled",
	"auth.authorization_code_required":                "authorization code is required",
	"auth.two_factor_already_enabled":                 "two factor authentication is already enabled",
	"auth.provider_bind_endpoint_required":            "provider bind must use account binding endpoint",
	"auth.provider_email_conflict":                    "provider email belongs to another account",
	"auth.provider_invalid":                           "provider authentication failed",
	"auth.provider_upstream_failed":                   "provider authentication failed",
	"auth.provider_subject_missing":                   "provider subject is missing",
	"auth.provider_identity_conflict":                 "provider identity is already bound to another account",
	"auth.provider_already_bound":                     "provider is already bound",
	"auth.provider_account_not_registered":            "provider account is not registered",
	"auth.oauth_intent_mismatch":                      "oauth intent mismatch",
	"auth.oauth_state_invalid":                        "invalid oauth state",
	"auth.oauth_state_expired":                        "oauth state expired",
	"auth.invalid_redirect_uri":                       "invalid redirect uri",
	"auth.invalid_pkce":                               "invalid pkce parameters",
	"auth.provider_id_required":                       "provider id is required",
	"auth.provider_order_invalid":                     "provider ids must be unique",
	"auth.provider_type_invalid":                      "provider type must be oidc or oauth2",
	"auth.provider_name_required":                     "provider name is required",
	"auth.provider_slug_required":                     "provider slug is required",
	"auth.provider_default_role_invalid":              "default role must be user, admin or superadmin",
	"auth.provider_superadmin_default_role_protected": "only superadmin can set superadmin default role",
	"auth.provider_logo_url_invalid":                  "logo url must be a valid http(s) or absolute path",
	"auth.provider_registration_requires_login":       "provider registration requires provider login to be enabled",
	"auth.provider_client_id_required":                "client id is required",
	"auth.provider_client_secret_required":            "client secret is required",
	"auth.provider_oidc_issuer_required":              "OIDC issuer url or discovery url is required",
	"auth.provider_oauth_urls_required":               "OAuth2 auth url, token url and userinfo url are required",
	"auth.provider_auth_url_not_configured":           "provider auth url is not configured",
	"user.invalid_time_zone":                          "invalid time zone",
	"user.invalid_avatar_url":                         "invalid avatar url",
	"user.invalid_username":                           "invalid username",
	"user.invalid_location":                           "invalid location",
	"user.invalid_email":                              "invalid user email",
	"user.invalid_phone":                              "invalid user phone",
	"user.invalid_locale":                             "invalid user locale",
	"user.invalid_status":                             "invalid user status",
	"user.invalid_role":                               "invalid user role",
	"user.username_already_exists":                    "username already exists",
	"user.username_change_used":                       "username change already used",
	"user.superadmin_management_protected":            "superadmin management is not allowed",
	"conversation.invalid_id":                         "invalid conversation id",
	"conversation.not_found":                          "conversation not found",
	"conversation.invalid_title":                      "invalid conversation title",
	"conversation_share.invalid":                      "invalid conversation share",
	"conversation_share.not_found":                    "conversation share not found",
	"conversation_share.invalid_id":                   "invalid share id",
	"message.invalid_id":                              "invalid message id",
	"message.not_found":                               "message not found",
	"file.invalid_id":                                 "invalid file id",
	"file.not_found":                                  "file not found",
	"file.required":                                   "file is required",
	"file.invalid_stream":                             "invalid file stream",
	"file.invalid_reference":                          "invalid file reference",
	"context_artifact.invalid_id":                     "invalid context artifact id",
	"context_artifact.not_found":                      "context artifact not found",
	"llm.model_route_not_configured":                  "model route is not configured",
	"llm.remote_models_unavailable":                   "remote models unavailable",
	"llm.no_active_api_key":                           "no active api key",
	"llm.invalid_adapter":                             "invalid adapter",
	"llm.invalid_compatible":                          "invalid compatible",
	"llm.invalid_platform_model_name":                 "invalid platform model name",
	"llm.system_prompt_too_long":                      "system prompt too long",
	"llm.platform_model_name_required":                "platform model name is required",
	"llm.protocol_required":                           "protocol is required",
	"billing.period_credit_exceeded":                  "period usage credit exceeded",
	"billing.invalid_subscription_tier":               "invalid subscription tier",
	"billing.subscription_expiry_required":            "subscription expiry required",
	"billing.invalid_subscription_expiry":             "invalid subscription expiry",
	"billing.invalid_model_pricing":                   "invalid model pricing",
	"billing.invalid_daily_usage_date_range":          "invalid daily usage date range",
	"billing.invalid_daily_usage_days":                "invalid daily usage days",
	"payment.provider_unavailable":                    "payment provider is unavailable",
	"payment.checkout_failed":                         "create checkout failed",
	"payment.notification_mismatch":                   "payment notification does not match the order",
	"payment.epay_type_unsupported":                   "epay payment type is not supported",
	"payment.return_url_invalid":                      "payment return url is invalid",
	"payment.return_url_cross_origin":                 "payment return url must use the configured public web origin",
	"payment.webhook_not_configured":                  "stripe webhook is not configured",
	"payment.invalid_webhook_body":                    "invalid webhook body",
	"payment.webhook_body_too_large":                  "webhook body too large",
	"payment.invalid_signature":                       "invalid stripe signature",
	"payment.invalid_event":                           "invalid stripe event",
	"payment.order_no_required":                       "order_no is required",
	"settings.invalid_namespace":                      "invalid namespace",
	"settings.invalid_key":                            "invalid setting key",
	"settings.not_found":                              "setting not found",
	"settings.invalid_value":                          "invalid setting value",
	"settings.smtp_invalid":                           "invalid smtp settings",
	"settings.billing_payment_invalid":                "invalid billing payment settings",
	"settings.model_option_policy_invalid":            "invalid model option policy settings",
	"settings.embedding_invalid":                      "invalid embedding settings",
	"settings.extract_invalid":                        "invalid file extraction settings",
	"embedding.service_unavailable":                   "embedding service is not available",
	"user_settings.unknown_key":                       "unknown setting key",
	"user_settings.invalid_value":                     "invalid user setting value",
	"memory.key_required":                             "memory_key is required",
	"rate_limit.refresh_exceeded":                     "too many refresh attempts",
	"rate_limit.authentication_exceeded":              "too many authentication attempts",
	"cors.origin_forbidden":                           "origin is not allowed",
}

func resolveErrorSpec(status int, msg string) (errorSpec, bool) {
	text := normalizeErrorText(msg)
	if text == "" {
		return errorSpec{}, false
	}
	if spec, ok := exactErrorSpecs[text]; ok {
		return spec, true
	}
	if strings.HasPrefix(text, "invalid setting: ") {
		detail := strings.TrimSpace(strings.TrimPrefix(text, "invalid setting: "))
		switch {
		case strings.HasPrefix(detail, "invalid namespace:"):
			return errorSpec{Code: "settings.invalid_namespace", Message: detail}, true
		case strings.HasPrefix(detail, "invalid setting key:"):
			return errorSpec{Code: "settings.invalid_key", Message: detail}, true
		case strings.Contains(detail, "smtp"):
			return errorSpec{Code: "settings.smtp_invalid", Message: detail}, true
		case strings.Contains(detail, "payment_providers"):
			return errorSpec{Code: "settings.billing_payment_invalid", Message: detail}, true
		case strings.Contains(detail, "model_option_"):
			return errorSpec{Code: "settings.model_option_policy_invalid", Message: detail}, true
		case strings.Contains(detail, "embedding") || strings.Contains(detail, "rag") || strings.Contains(detail, "semantic"):
			return errorSpec{Code: "settings.embedding_invalid", Message: detail}, true
		case strings.Contains(detail, "extract:"):
			return errorSpec{Code: "settings.extract_invalid", Message: detail}, true
		default:
			return errorSpec{Code: "settings.invalid_value", Message: detail}, true
		}
	}
	if strings.HasPrefix(text, "unknown setting key:") {
		return errorSpec{Code: "user_settings.unknown_key", Message: "unknown setting key"}, true
	}
	if strings.HasPrefix(text, "invalid value for ") {
		return errorSpec{Code: "user_settings.invalid_value", Message: text}, true
	}
	if status < http.StatusInternalServerError && status != http.StatusBadGateway && (strings.Contains(text, "provider") || strings.Contains(text, "oauth") || strings.Contains(text, "pkce")) {
		return providerErrorSpec(text)
	}
	if strings.HasPrefix(text, "invalid ") && strings.HasSuffix(text, " id") {
		return errorSpec{Code: invalidIDCode(text), Message: text}, true
	}
	if strings.HasPrefix(text, "invalid ") && strings.HasSuffix(text, "_id") {
		return errorSpec{Code: CodeRequestInvalidID, Message: text}, true
	}
	if strings.HasPrefix(text, "invalid ") {
		return errorSpec{Code: "request.invalid_" + slug(strings.TrimPrefix(text, "invalid ")), Message: text}, true
	}
	if strings.HasSuffix(text, " not found") {
		return errorSpec{Code: notFoundCode(text), Message: text}, true
	}
	if strings.HasSuffix(text, " already exists") {
		return errorSpec{Code: slug(strings.TrimSuffix(text, " already exists")) + ".already_exists", Message: text}, true
	}
	if strings.Contains(text, "verification code is invalid or expired") {
		return errorSpec{Code: "auth.verification_code_invalid", Message: "verification code is invalid or expired"}, true
	}
	if strings.Contains(text, "verification code was sent recently") {
		return errorSpec{Code: "auth.verification_code_recent", Message: "verification code was sent recently"}, true
	}
	if strings.Contains(text, "verification code attempts exceeded") {
		return errorSpec{Code: "auth.verification_code_attempts_exceeded", Message: "verification code attempts exceeded"}, true
	}
	if strings.Contains(text, "email already exists") {
		return errorSpec{Code: "auth.email_already_exists", Message: "email already exists"}, true
	}
	if strings.Contains(text, "invalid email") || strings.Contains(text, "user email is invalid") {
		return errorSpec{Code: "auth.invalid_email", Message: "invalid email"}, true
	}
	if strings.Contains(text, "email verification is disabled") {
		return errorSpec{Code: "auth.email_verification_disabled", Message: "email verification is disabled"}, true
	}
	if strings.Contains(text, "email bootstrap is not allowed") {
		return errorSpec{Code: "auth.email_bootstrap_not_allowed", Message: "email bootstrap is not allowed"}, true
	}
	if strings.Contains(text, "smtp") {
		return errorSpec{Code: "settings.smtp_invalid", Message: text}, true
	}
	if strings.Contains(text, "payment") || strings.Contains(text, "stripe") || strings.Contains(text, "checkout") {
		return errorSpec{Code: CodeBillingPaymentRequired, Message: fallbackMessage(status, CodeBillingPaymentRequired)}, true
	}
	if strings.Contains(text, "required") {
		return errorSpec{Code: CodeRequestRequired, Message: text}, true
	}
	return errorSpec{}, false
}

func normalizeErrorText(msg string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(msg)), " "))
}

func invalidIDCode(text string) string {
	resource := strings.TrimPrefix(text, "invalid ")
	resource = strings.TrimSuffix(resource, " id")
	resource = strings.TrimSpace(resource)
	if resource == "" {
		return CodeRequestInvalidID
	}
	return slug(resource) + ".invalid_id"
}

func notFoundCode(text string) string {
	resource := strings.TrimSuffix(text, " not found")
	resource = strings.TrimSpace(resource)
	if resource == "" || resource == "resource" || resource == "record" {
		return CodeResourceNotFound
	}
	return slug(resource) + ".not_found"
}

func providerErrorSpec(text string) (errorSpec, bool) {
	switch {
	case strings.Contains(text, "third-party login is disabled"):
		return errorSpec{Code: "auth.provider_login_disabled", Message: "third-party login is disabled"}, true
	case strings.Contains(text, "provider login is disabled"):
		return errorSpec{Code: "auth.provider_login_disabled", Message: "provider login is disabled"}, true
	case strings.Contains(text, "provider registration is disabled"):
		return errorSpec{Code: "auth.provider_registration_disabled", Message: "provider registration is disabled"}, true
	case strings.Contains(text, "authorization code is required"):
		return errorSpec{Code: CodeRequestRequired, Message: "authorization code is required"}, true
	case strings.Contains(text, "oauth intent mismatch"):
		return errorSpec{Code: "auth.oauth_intent_mismatch", Message: "oauth intent mismatch"}, true
	case strings.Contains(text, "provider subject is missing"):
		return errorSpec{Code: "auth.provider_subject_missing", Message: "provider subject is missing"}, true
	case strings.Contains(text, "provider identity is already bound"):
		return errorSpec{Code: "auth.provider_identity_conflict", Message: "provider identity is already bound to another account"}, true
	case strings.Contains(text, "provider is already bound"):
		return errorSpec{Code: "auth.provider_already_bound", Message: "provider is already bound"}, true
	case strings.Contains(text, "provider account is not registered"):
		return errorSpec{Code: "auth.provider_account_not_registered", Message: "provider account is not registered"}, true
	case strings.Contains(text, "invalid oauth state") || strings.Contains(text, "oauth state mismatch"):
		return errorSpec{Code: "auth.oauth_state_invalid", Message: "invalid oauth state"}, true
	case strings.Contains(text, "oauth state expired"):
		return errorSpec{Code: "auth.oauth_state_expired", Message: "oauth state expired"}, true
	case strings.Contains(text, "redirect uri"):
		return errorSpec{Code: "auth.invalid_redirect_uri", Message: "invalid redirect uri"}, true
	case strings.Contains(text, "pkce"):
		return errorSpec{Code: "auth.invalid_pkce", Message: text}, true
	case strings.Contains(text, "provider token") || strings.Contains(text, "provider userinfo") || strings.Contains(text, "provider discovery"):
		return errorSpec{Code: "auth.provider_upstream_failed", Message: "provider authentication failed"}, true
	default:
		return errorSpec{Code: "auth.provider_invalid", Message: text}, true
	}
}

func slug(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "resource"
	}
	replacer := strings.NewReplacer(
		" ", "_",
		"-", "_",
		".", "_",
		":", "_",
		"/", "_",
		"\\", "_",
	)
	return replacer.Replace(value)
}
