import enErrors from "@/i18n/messages/en-US/errors.json";
import zhErrors from "@/i18n/messages/zh-CN/errors.json";
import { DEFAULT_LOCALE, LOCALE_COOKIE_NAME, normalizeAppLocale, resolveBrowserLocale, type AppLocale } from "@/i18n/config";
import { ApiError } from "@/shared/api/http-client";

const ERROR_MESSAGES: Record<AppLocale, unknown> = {
  "en-US": enErrors,
  "zh-CN": zhErrors,
};

const FALLBACK_MESSAGES: Record<AppLocale, string> = {
  "en-US": "Request failed. Please try again later.",
  "zh-CN": "请求失败，请稍后重试。",
};

type RequestBodyFieldError = {
  field?: unknown;
  rule?: unknown;
  param?: unknown;
};

type RequestBodyErrorDetails = {
  fieldErrors?: unknown;
};

type RedemptionCodeErrorDetails = {
  reason?: unknown;
};

type SettingsValidationDetails = {
  field?: unknown;
  fields?: unknown;
  rule?: unknown;
  param?: unknown;
};

const REQUEST_FIELD_LABELS: Record<AppLocale, Record<string, string>> = {
  "en-US": {
    apiKeys: "API keys",
    avatarURL: "Avatar URL",
    baseURL: "Base URL",
    cbDurationMin: "Circuit duration",
    cbFailureThreshold: "Failure threshold",
    cbModelThreshold: "Model threshold",
    cbThresholdLogic: "Threshold logic",
    cbWindowMin: "Circuit window",
    compatible: "Compatibility mode",
    connectTimeoutMS: "Connect timeout",
    displayName: "Display name",
    email: "Email",
    headersJSON: "Headers JSON",
    locale: "Language",
    maskFileID: "Mask file",
    name: "Name",
    password: "Password",
    phone: "Phone",
    protocolDefaultsJSON: "Protocol defaults JSON",
    readTimeoutMS: "Read timeout",
    status: "Status",
    systemPrompt: "System prompt",
    streamIdleTimeoutMS: "Stream idle timeout",
    subscriptionExpiresAt: "Subscription expiry",
    subscriptionTier: "Subscription plan",
    timezone: "Timezone",
    username: "Username",
  },
  "zh-CN": {
    apiKeys: "API Keys",
    avatarURL: "头像地址",
    baseURL: "Base URL",
    cbDurationMin: "熔断时长",
    cbFailureThreshold: "失败阈值",
    cbModelThreshold: "模型阈值",
    cbThresholdLogic: "阈值逻辑",
    cbWindowMin: "统计窗口",
    compatible: "兼容模式",
    connectTimeoutMS: "连接超时",
    displayName: "昵称",
    email: "邮箱",
    headersJSON: "请求头 JSON",
    locale: "语言",
    maskFileID: "蒙版文件",
    name: "名称",
    password: "密码",
    phone: "手机号",
    protocolDefaultsJSON: "协议默认配置 JSON",
    readTimeoutMS: "读取超时",
    status: "状态",
    systemPrompt: "系统提示词",
    streamIdleTimeoutMS: "流式空闲超时",
    subscriptionExpiresAt: "订阅到期时间",
    subscriptionTier: "订阅方案",
    timezone: "时区",
    username: "用户名",
  },
};

const SETTINGS_FIELD_LABELS: Record<AppLocale, Record<string, string>> = {
  "en-US": {
    "auth:auto_link_verified_email": "Auto-link same email",
    "auth:email_login_enabled": "Email sign-in",
    "auth:email_registration_allowed_domains": "Allowed email domains",
    "auth:email_registration_block_plus_alias": "Block plus aliases",
    "auth:email_registration_enabled": "Email registration",
    "auth:email_verification_enabled": "Email verification",
    "auth:password_reset_enabled": "Password reset",
    "auth:login_default_next_path": "Default redirect path",
    "auth:login_lock_minutes": "Lock duration",
    "auth:login_max_failures": "Login failure limit",
    "auth:rate_limit_enabled": "Platform rate limit",
    "auth:rate_limit_rpm": "User API rate limit",
    "auth:public_auth_rate_limit_rpm": "Public auth rate limit",
    "auth:refresh_token_ttl_hours": "Refresh token TTL",
    "auth:smtp_from": "SMTP sender",
    "auth:smtp_host": "SMTP host",
    "auth:smtp_password": "SMTP password",
    "auth:smtp_port": "SMTP port",
    "auth:smtp_username": "SMTP username",
    "auth:third_party_login_enabled": "Third-party sign-in",
    "auth:token_ttl_hours": "Access token TTL",
    "auth:turnstile_registration_enabled": "Turnstile registration verification",
    "auth:turnstile_secret_key": "Turnstile Secret Key",
    "auth:turnstile_site_key": "Turnstile Site Key",
    "auth:username_login_enabled": "Username sign-in",
    "billing:epay_gateway_url": "EPay gateway URL",
    "billing:epay_key": "EPay key",
    "billing:epay_pid": "EPay merchant ID",
    "billing:epay_types": "EPay payment types",
    "billing:mode": "Billing mode",
    "billing:payment_providers": "Payment providers",
    "billing:prepaid_amount_usd": "Per-request reservation",
    "billing:stripe_publishable_key": "Stripe publishable key",
    "billing:stripe_secret_key": "Stripe secret key",
    "billing:stripe_webhook_secret": "Stripe webhook secret",
    "billing:usd_to_cny_rate": "USD to CNY rate",
    "chat:model_option_allowed_paths": "Model option allowlist",
    "chat:default_system_prompt": "Global default system prompt",
    "chat:model_option_denied_paths": "Model option denylist",
    "chat:model_option_policy_mode": "Model option policy",
    "file:embedding_enabled": "Embedding",
    "file:full_context_limit_enabled": "Full-text injection limits",
    "file:file_full_context_max_bytes": "Full-text size limit",
    "file:full_context_max_tokens": "Full-text token limit",
    "file:full_context_pdf_max_pages": "Full-text page limit",
    "mcp:mcp_enable": "MCP",
  },
  "zh-CN": {
    "auth:auto_link_verified_email": "同邮箱自动绑定",
    "auth:email_login_enabled": "邮箱登录",
    "auth:email_registration_allowed_domains": "邮箱注册域名白名单",
    "auth:email_registration_block_plus_alias": "禁止邮箱 + 别名",
    "auth:email_registration_enabled": "邮箱注册",
    "auth:email_verification_enabled": "邮箱验证",
    "auth:password_reset_enabled": "重置密码",
    "auth:login_default_next_path": "登录后默认跳转路径",
    "auth:login_lock_minutes": "锁定时长",
    "auth:login_max_failures": "登录失败阈值",
    "auth:rate_limit_enabled": "平台限流",
    "auth:rate_limit_rpm": "用户接口限流",
    "auth:public_auth_rate_limit_rpm": "公开鉴权限流",
    "auth:refresh_token_ttl_hours": "刷新令牌有效期",
    "auth:smtp_from": "SMTP 发件人",
    "auth:smtp_host": "SMTP 主机",
    "auth:smtp_password": "SMTP 密码",
    "auth:smtp_port": "SMTP 端口",
    "auth:smtp_username": "SMTP 用户名",
    "auth:third_party_login_enabled": "第三方登录",
    "auth:token_ttl_hours": "访问令牌有效期",
    "auth:turnstile_registration_enabled": "注册人机验证",
    "auth:turnstile_secret_key": "Turnstile Secret Key",
    "auth:turnstile_site_key": "Turnstile Site Key",
    "auth:username_login_enabled": "用户名登录",
    "billing:epay_gateway_url": "易支付网关地址",
    "billing:epay_key": "易支付商户密钥",
    "billing:epay_pid": "易支付商户 ID",
    "billing:epay_types": "易支付支付方式",
    "billing:mode": "计费模式",
    "billing:payment_providers": "支付渠道",
    "billing:prepaid_amount_usd": "单次预留金额",
    "billing:stripe_publishable_key": "Stripe Publishable Key",
    "billing:stripe_secret_key": "Stripe Secret Key",
    "billing:stripe_webhook_secret": "Stripe Webhook Secret",
    "billing:usd_to_cny_rate": "美元人民币汇率",
    "chat:model_option_allowed_paths": "模型参数白名单",
    "chat:default_system_prompt": "全局默认系统提示词",
    "chat:model_option_denied_paths": "模型参数黑名单",
    "chat:model_option_policy_mode": "模型参数透传策略",
    "file:embedding_enabled": "向量服务",
    "file:full_context_limit_enabled": "全文注入限制",
    "file:file_full_context_max_bytes": "全文大小上限",
    "file:full_context_max_tokens": "全文 Token 上限",
    "file:full_context_pdf_max_pages": "全文页数上限",
    "mcp:mcp_enable": "MCP",
  },
};

export function toErrorMessagePath(errorCode: string): string[] {
  return errorCode
    .trim()
    .split(".")
    .filter(Boolean)
    .map((segment) => segment.replace(/_([a-z])/g, (_, char: string) => char.toUpperCase()));
}

function isInternalErrorKey(message: string): boolean {
  return /^errors\.[a-zA-Z0-9_.]+$/.test(message.trim());
}

function readClientLocale(): AppLocale {
  if (typeof document === "undefined") {
    return DEFAULT_LOCALE;
  }
  const cookieValue = document.cookie
    .split(";")
    .map((item) => item.trim())
    .find((item) => item.startsWith(`${LOCALE_COOKIE_NAME}=`))
    ?.slice(LOCALE_COOKIE_NAME.length + 1);
  if (cookieValue) {
    return normalizeAppLocale(decodeURIComponent(cookieValue));
  }
  return typeof navigator === "undefined"
    ? DEFAULT_LOCALE
    : resolveBrowserLocale(navigator.languages?.length ? navigator.languages : [navigator.language]);
}

function lookupErrorMessage(locale: AppLocale, errorCode: string): string | undefined {
  let current: unknown = ERROR_MESSAGES[locale];
  for (const segment of toErrorMessagePath(errorCode)) {
    if (!current || typeof current !== "object" || !Object.hasOwn(current, segment)) {
      return undefined;
    }
    current = (current as Record<string, unknown>)[segment];
  }
  return typeof current === "string" ? current : undefined;
}

function isRequestBodyErrorDetails(details: unknown): details is RequestBodyErrorDetails {
  return Boolean(details && typeof details === "object" && "fieldErrors" in details);
}

function isRequestBodyFieldError(item: unknown): item is RequestBodyFieldError {
  return Boolean(item && typeof item === "object" && "field" in item && "rule" in item);
}

function resolveRequestFieldLabel(locale: AppLocale, field: string): string {
  return REQUEST_FIELD_LABELS[locale][field] ?? field;
}

function resolveRequestFieldError(locale: AppLocale, item: RequestBodyFieldError): string | undefined {
  const field = typeof item.field === "string" ? item.field.trim() : "";
  const rule = typeof item.rule === "string" ? item.rule.trim() : "";
  const param = typeof item.param === "string" ? item.param.trim() : "";
  if (!field || !rule) return undefined;

  const label = resolveRequestFieldLabel(locale, field);
  if (locale === "zh-CN") {
    switch (rule) {
      case "required":
      case "required_without":
        return `${label}不能为空。`;
      case "min":
        return `${label}至少 ${param} 个字符。`;
      case "max":
        return `${label}不能超过 ${param} 个字符。`;
      case "len":
        return `${label}长度必须是 ${param} 个字符。`;
      case "email":
        return `${label}格式不正确。`;
      case "url":
        return `${label}必须是完整 URL，例如 https://api.example.com。`;
      case "oneof":
        return `${label}必须是以下值之一：${param}。`;
      default:
        return `${label}参数无效。`;
    }
  }

  switch (rule) {
    case "required":
    case "required_without":
      return `${label} is required.`;
    case "min":
      return `${label} must be at least ${param} characters.`;
    case "max":
      return `${label} must be at most ${param} characters.`;
    case "len":
      return `${label} must be ${param} characters.`;
    case "email":
      return `${label} must be a valid email address.`;
    case "url":
      return `${label} must be a full URL, for example https://api.example.com.`;
    case "oneof":
      return `${label} must be one of: ${param}.`;
    default:
      return `${label} is invalid.`;
  }
}

function resolveRequestBodyValidationMessage(error: ApiError, locale: AppLocale): string | undefined {
  if (error.errorCode !== "request.invalid_body") return undefined;
  if (!isRequestBodyErrorDetails(error.details) || !Array.isArray(error.details.fieldErrors)) return undefined;

  const messages = error.details.fieldErrors
    .filter(isRequestBodyFieldError)
    .map((item) => resolveRequestFieldError(locale, item))
    .filter((item): item is string => Boolean(item));

  return messages.length > 0 ? messages.join(locale === "zh-CN" ? "" : " ") : undefined;
}

function resolveSettingsFieldLabel(locale: AppLocale, key: string): string {
  return SETTINGS_FIELD_LABELS[locale][key] ?? key;
}

function isSettingsValidationDetails(details: unknown): details is SettingsValidationDetails {
  return Boolean(details && typeof details === "object" && "rule" in details);
}

function splitRuleParam(param: string): string[] {
  return param.split(",").map((value) => value.trim()).filter(Boolean);
}

function resolveSettingsValidationMessage(error: ApiError, locale: AppLocale): string | undefined {
  if (!error.errorCode?.startsWith("settings.") || !isSettingsValidationDetails(error.details)) return undefined;
  const rule = typeof error.details.rule === "string" ? error.details.rule.trim() : "";
  if (!rule) return undefined;
  const field = typeof error.details.field === "string" ? error.details.field.trim() : "";
  const fields = Array.isArray(error.details.fields)
    ? error.details.fields.filter((value): value is string => typeof value === "string" && value.trim().length > 0).map((value) => value.trim())
    : [];
  const param = typeof error.details.param === "string" ? error.details.param.trim() : "";
  const label = field ? resolveSettingsFieldLabel(locale, field) : fields.map((value) => resolveSettingsFieldLabel(locale, value)).join(locale === "zh-CN" ? "、" : ", ");
  const displayLabel = label || (locale === "zh-CN" ? "设置项" : "Setting");
  const [first, second] = splitRuleParam(param);

  if (locale === "zh-CN") {
    switch (rule) {
      case "required":
        return `${displayLabel}不能为空。`;
      case "required_when":
        return `${displayLabel}在 ${param} 时不能为空。`;
      case "required_together":
        return `${displayLabel}不能为空。`;
      case "bool":
        return `${displayLabel}必须是 true 或 false。`;
      case "integer":
        return `${displayLabel}必须是整数。`;
      case "integer_range":
        return `${displayLabel}必须在 ${first} 到 ${second} 之间。`;
      case "optional_integer_range":
        return `${displayLabel}必须留空、填 0，或在 ${first} 到 ${second} 之间。`;
      case "integer_min":
        return `${displayLabel}必须大于等于 ${param}。`;
      case "optional_integer_min":
        return `${displayLabel}必须留空，或大于等于 ${param}。`;
      case "float_range":
        return `${displayLabel}必须在 ${first} 到 ${second} 之间。`;
      case "max_length":
        return `${displayLabel}长度不能超过 ${param} 个字符。`;
      case "enum":
        return `${displayLabel}必须是以下值之一：${splitRuleParam(param).join("、")}。`;
      case "payment_provider":
        return `${displayLabel}只能包含：${splitRuleParam(param).join("、")}。`;
      case "http_url":
        return `${displayLabel}必须以 http:// 或 https:// 开头。`;
      case "trusted_http_url":
        return `${displayLabel}必须是受信任的 HTTP 地址。`;
      case "local_path":
        return `${displayLabel}必须是站内路径，例如 /chat。`;
      case "json_object":
        return `${displayLabel}必须是 JSON 对象。`;
      case "json_array":
        return `${displayLabel}必须是 JSON 数组。`;
      case "payment_count":
        return `${displayLabel}必须包含 ${first} 到 ${second} 个支付方式。`;
      case "payment_fields":
        return `${displayLabel}每一项都必须包含 ${param}。`;
      case "payment_item_length":
        return `${displayLabel}单项内容过长。`;
      case "payment_type_chars":
        return `${displayLabel}的 type 包含无效字符。`;
      case "payment_type_unique":
        return `${displayLabel}的 type 不能重复。`;
      case "model_option_protocol":
        return `${displayLabel}包含不支持的协议。`;
      case "model_option_path":
        return `${displayLabel}包含无效的参数路径。`;
      case "native_tool_pricing":
        return `${displayLabel}包含无效的原生工具计费配置。`;
      case "mime":
        return `${displayLabel}包含无效的 MIME 类型。`;
      case "file_type":
        return `${displayLabel}只能包含：${splitRuleParam(param).join("、")}。`;
      case "domain":
        return `${displayLabel}包含无效域名。`;
      case "epay_url":
        return `${displayLabel}必须是有效的 HTTP(S) 易支付地址。`;
      case "dependency":
        if (param === "username_or_email_login") {
          return "关闭用户名和邮箱登录前，必须先启用第三方登录。";
        }
        if (param === "superadmin_identity") {
          return "启用第三方登录前，必须先绑定管理员身份。";
        }
        if (param === "embedding_service_ready") {
          return "启用向量化前，必须先配置并启用向量服务。";
        }
        if (param === "vector_store_available") {
          return "启用向量化前，必须先配置可用的向量存储。";
        }
        return `${displayLabel}依赖条件未满足。`;
      case "clear_not_allowed":
        return `${displayLabel}不支持清空。`;
      case "invalid_namespace":
        return "设置命名空间无效。";
      case "invalid_key":
        return "设置项无效。";
      case "invalid_value":
        return "设置值无效。";
      default:
        return `${displayLabel}无效。`;
    }
  }

  switch (rule) {
    case "required":
      return `${displayLabel} is required.`;
    case "required_when":
      return `${displayLabel} is required when ${param}.`;
    case "required_together":
      return `${displayLabel} is required.`;
    case "bool":
      return `${displayLabel} must be true or false.`;
    case "integer":
      return `${displayLabel} must be an integer.`;
    case "integer_range":
      return `${displayLabel} must be between ${first} and ${second}.`;
    case "optional_integer_range":
      return `${displayLabel} must be empty, 0, or between ${first} and ${second}.`;
    case "integer_min":
      return `${displayLabel} must be at least ${param}.`;
    case "optional_integer_min":
      return `${displayLabel} must be empty or at least ${param}.`;
    case "float_range":
      return `${displayLabel} must be between ${first} and ${second}.`;
    case "max_length":
      return `${displayLabel} length must be at most ${param} characters.`;
    case "enum":
      return `${displayLabel} must be one of: ${splitRuleParam(param).join(", ")}.`;
    case "payment_provider":
      return `${displayLabel} may contain only: ${splitRuleParam(param).join(", ")}.`;
    case "http_url":
      return `${displayLabel} must start with http:// or https://.`;
    case "trusted_http_url":
      return `${displayLabel} must be a trusted HTTP endpoint.`;
    case "local_path":
      return `${displayLabel} must be a local path, such as /chat.`;
    case "json_object":
      return `${displayLabel} must be a JSON object.`;
    case "json_array":
      return `${displayLabel} must be a JSON array.`;
    case "payment_count":
      return `${displayLabel} must contain ${first} to ${second} payment types.`;
    case "payment_fields":
      return `Each ${displayLabel} item must include ${param}.`;
    case "payment_item_length":
      return `${displayLabel} contains an item that is too long.`;
    case "payment_type_chars":
      return `${displayLabel} contains invalid type characters.`;
    case "payment_type_unique":
      return `${displayLabel} type values must be unique.`;
    case "model_option_protocol":
      return `${displayLabel} contains an unsupported protocol.`;
    case "model_option_path":
      return `${displayLabel} contains an invalid option path.`;
    case "native_tool_pricing":
      return `${displayLabel} contains invalid native tool pricing configuration.`;
    case "mime":
      return `${displayLabel} contains an invalid MIME type.`;
    case "file_type":
      return `${displayLabel} may contain only: ${splitRuleParam(param).join(", ")}.`;
    case "domain":
      return `${displayLabel} contains an invalid domain.`;
    case "epay_url":
      return `${displayLabel} must be a valid HTTP(S) EPay URL.`;
    case "dependency":
      if (param === "username_or_email_login") {
        return "Enable third-party sign-in before disabling both username and email sign-in.";
      }
      if (param === "superadmin_identity") {
        return "Bind a super administrator identity before enabling third-party sign-in.";
      }
      if (param === "embedding_service_ready") {
        return "Configure and enable the embedding service before enabling vectorization.";
      }
      if (param === "vector_store_available") {
        return "Configure an available vector store before enabling vectorization.";
      }
      return `${displayLabel} has an unmet dependency.`;
    case "clear_not_allowed":
      return `${displayLabel} cannot be cleared.`;
    case "invalid_namespace":
      return "Invalid setting namespace.";
    case "invalid_key":
      return "Invalid setting key.";
    case "invalid_value":
      return "Invalid setting value.";
    default:
      return `${displayLabel} is invalid.`;
  }
}

function isRedemptionCodeErrorDetails(details: unknown): details is RedemptionCodeErrorDetails {
  return Boolean(details && typeof details === "object" && "reason" in details);
}

function resolveRedemptionCodeValidationMessage(error: ApiError, locale: AppLocale): string | undefined {
  if (error.errorCode !== "billing.invalid_redemption_code") return undefined;
  if (!isRedemptionCodeErrorDetails(error.details) || typeof error.details.reason !== "string") return undefined;
  const reason = error.details.reason.trim();
  if (!reason) return undefined;
  return lookupErrorMessage(locale, `billing.redemption_validation.${reason}`);
}

export function resolveLocalizedErrorMessage(error: unknown, fallback?: string): string {
  const locale = readClientLocale();
  if (error instanceof ApiError && error.errorCode) {
    const validationMessage = resolveRequestBodyValidationMessage(error, locale);
    if (validationMessage) {
      return validationMessage;
    }

    const settingsValidationMessage = resolveSettingsValidationMessage(error, locale);
    if (settingsValidationMessage) {
      return settingsValidationMessage;
    }

    const redemptionCodeValidationMessage = resolveRedemptionCodeValidationMessage(error, locale);
    if (redemptionCodeValidationMessage) {
      return redemptionCodeValidationMessage;
    }

    const translated = lookupErrorMessage(locale, error.errorCode);
    if (translated) {
      return translated;
    }
  }

  if (error instanceof Error) {
    const message = error.message.trim();
    if (isInternalErrorKey(message)) {
      const translated = lookupErrorMessage(locale, message.replace(/^errors\./, ""));
      if (translated) {
        return translated;
      }
    }
    if (message && !isInternalErrorKey(message)) {
      return message;
    }
  }

  return fallback || FALLBACK_MESSAGES[locale];
}
