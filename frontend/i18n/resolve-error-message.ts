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
    "auth:login_default_next_path": "Default redirect path",
    "auth:login_lock_minutes": "Lock duration",
    "auth:login_max_failures": "Login failure limit",
    "auth:login_page_title": "Login page title",
    "auth:public_auth_rate_limit_rpm": "Public auth rate limit",
    "auth:refresh_token_ttl_hours": "Refresh token TTL",
    "auth:smtp_from": "SMTP sender",
    "auth:smtp_host": "SMTP host",
    "auth:smtp_password": "SMTP password",
    "auth:smtp_port": "SMTP port",
    "auth:smtp_username": "SMTP username",
    "auth:third_party_login_enabled": "Third-party sign-in",
    "auth:token_ttl_hours": "Access token TTL",
    "auth:username_login_enabled": "Username sign-in",
    "billing:epay_gateway_url": "EPay gateway URL",
    "billing:epay_key": "EPay key",
    "billing:epay_pid": "EPay merchant ID",
    "billing:epay_types": "EPay payment types",
    "billing:mode": "Billing mode",
    "billing:payment_providers": "Payment providers",
    "billing:prepaid_amount_usd": "Prepaid amount",
    "billing:stripe_publishable_key": "Stripe publishable key",
    "billing:stripe_secret_key": "Stripe secret key",
    "billing:stripe_webhook_secret": "Stripe webhook secret",
    "billing:usd_to_cny_rate": "USD to CNY rate",
    "chat:model_option_allowed_paths": "Model option allowlist",
    "chat:default_system_prompt": "Global default system prompt",
    "chat:model_option_denied_paths": "Model option denylist",
    "chat:model_option_native_tool_types": "Native tool types",
    "chat:model_option_policy_mode": "Model option policy",
    "file:embedding_enabled": "Embedding",
    "file:full_context_limit_enabled": "Full-text injection limits",
    "file:file_full_context_max_bytes": "Full-text byte limit",
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
    "auth:login_default_next_path": "登录后默认跳转路径",
    "auth:login_lock_minutes": "锁定时长",
    "auth:login_max_failures": "登录失败阈值",
    "auth:login_page_title": "登录页标题",
    "auth:public_auth_rate_limit_rpm": "公开鉴权限流",
    "auth:refresh_token_ttl_hours": "刷新令牌有效期",
    "auth:smtp_from": "SMTP 发件人",
    "auth:smtp_host": "SMTP 主机",
    "auth:smtp_password": "SMTP 密码",
    "auth:smtp_port": "SMTP 端口",
    "auth:smtp_username": "SMTP 用户名",
    "auth:third_party_login_enabled": "第三方登录",
    "auth:token_ttl_hours": "访问令牌有效期",
    "auth:username_login_enabled": "用户名登录",
    "billing:epay_gateway_url": "易支付网关地址",
    "billing:epay_key": "易支付商户密钥",
    "billing:epay_pid": "易支付商户 ID",
    "billing:epay_types": "易支付支付方式",
    "billing:mode": "计费模式",
    "billing:payment_providers": "支付渠道",
    "billing:prepaid_amount_usd": "预付费金额",
    "billing:stripe_publishable_key": "Stripe Publishable Key",
    "billing:stripe_secret_key": "Stripe Secret Key",
    "billing:stripe_webhook_secret": "Stripe Webhook Secret",
    "billing:usd_to_cny_rate": "美元人民币汇率",
    "chat:model_option_allowed_paths": "模型参数白名单",
    "chat:default_system_prompt": "全局默认系统提示词",
    "chat:model_option_denied_paths": "模型参数黑名单",
    "chat:model_option_native_tool_types": "官方原生工具类型",
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
    if (!current || typeof current !== "object" || !Object.prototype.hasOwnProperty.call(current, segment)) {
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

function resolveSettingsReason(locale: AppLocale, label: string, reason: string): string {
  const normalized = reason.trim();
  if (!normalized) return "";
  if (locale === "zh-CN") {
    const integerRange = normalized.match(/^must be an integer between (.+) and (.+)$/);
    if (integerRange) return `${label}必须是 ${integerRange[1]} 到 ${integerRange[2]} 之间的整数。`;
    const optionalZeroRange = normalized.match(/^must be empty, 0, or between (.+) and (.+)$/);
    if (optionalZeroRange) return `${label}必须留空、填 0，或在 ${optionalZeroRange[1]} 到 ${optionalZeroRange[2]} 之间。`;
    const range = normalized.match(/^must be between (.+) and (.+)$/);
    if (range) return `${label}必须在 ${range[1]} 到 ${range[2]} 之间。`;
    const optionalMin = normalized.match(/^must be empty or >= (.+)$/);
    if (optionalMin) return `${label}必须留空，或大于等于 ${optionalMin[1]}。`;
    const min = normalized.match(/^must be >= (.+)$/);
    if (min) return `${label}必须大于等于 ${min[1]}。`;
    const maxLength = normalized.match(/^length must be <= (.+)$/);
    if (maxLength) return `${label}长度不能超过 ${maxLength[1]} 个字符。`;
    const oneOf = normalized.match(/^must be one of: (.+)$/);
    if (oneOf) return `${label}必须是以下值之一：${oneOf[1]}。`;
    const only = normalized.match(/^must contain only: (.+)$/);
    if (only) return `${label}只能包含：${only[1]}。`;
    const invalidDomain = normalized.match(/^contains invalid domain: (.+)$/);
    if (invalidDomain) return `${label}包含无效域名：${invalidDomain[1]}。`;
    const invalidMime = normalized.match(/^contains invalid mime: (.+)$/);
    if (invalidMime) return `${label}包含无效 MIME 类型：${invalidMime[1]}。`;
    switch (normalized) {
      case "cannot be empty":
      case "is required":
        return `${label}不能为空。`;
      case "must be a local path":
        return `${label}必须是站内路径，例如 /chat。`;
      case "must be bool":
        return `${label}必须是 true 或 false。`;
      case "must start with http:// or https://":
        return `${label}必须以 http:// 或 https:// 开头。`;
      case "must be a json array":
        return `${label}必须是 JSON 数组。`;
      case "must contain 1-10 payment types":
        return `${label}必须包含 1 到 10 个支付方式。`;
      case "items require name and type":
        return `${label}每一项都必须包含 name 和 type。`;
      case "item is too long":
        return `${label}单项内容过长。`;
      case "type contains invalid characters":
        return `${label}的 type 包含无效字符。`;
      case "type must be unique":
        return `${label}的 type 不能重复。`;
      default:
        return `${label}：${normalized}`;
    }
  }

  const optionalZeroRange = normalized.match(/^must be empty, 0, or between (.+) and (.+)$/);
  if (optionalZeroRange) {
    return `${label} must be empty, 0, or between ${optionalZeroRange[1]} and ${optionalZeroRange[2]}.`;
  }
  const optionalMin = normalized.match(/^must be empty or >= (.+)$/);
  if (optionalMin) {
    return `${label} must be empty or at least ${optionalMin[1]}.`;
  }

  return `${label}: ${normalized}.`;
}

function resolveSettingsValidationMessage(error: ApiError, locale: AppLocale): string | undefined {
  if (!error.errorCode?.startsWith("settings.")) return undefined;
  const raw = (error.rawMessage || error.message || "").trim();
  if (!raw || /^invalid .+ settings?\.?$/i.test(raw) || /^invalid setting value\.?$/i.test(raw)) {
    return undefined;
  }
  const detail = raw.replace(/^invalid setting:\s*/i, "").trim();
  const dependencyMessages: Record<AppLocale, Record<string, string>> = {
    "en-US": {
      "auth:third_party_login_enabled must be enabled before disabling username and email login": "Enable third-party sign-in before disabling both username and email sign-in.",
      "embedding service must be enabled and configured before enabling rag or semantic enhancement": "Enable and configure embedding before enabling RAG or semantic context.",
    },
    "zh-CN": {
      "auth:third_party_login_enabled must be enabled before disabling username and email login": "关闭用户名和邮箱登录前，必须先启用第三方登录。",
      "embedding service must be enabled and configured before enabling rag or semantic enhancement": "启用 RAG 或语义增强前，必须先启用并配置向量服务。",
    },
  };
  const dependencyMessage = dependencyMessages[locale][detail.toLowerCase()];
  if (dependencyMessage) return dependencyMessage;

  const match = detail.match(/^([a-z]+:[a-z0-9_]+)\s+(.+)$/);
  if (!match) return detail;
  return resolveSettingsReason(locale, resolveSettingsFieldLabel(locale, match[1]), match[2]);
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
