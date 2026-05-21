import enErrors from "@/i18n/messages/en-US/errors.json";
import zhErrors from "@/i18n/messages/zh-CN/errors.json";
import { DEFAULT_LOCALE, LOCALE_COOKIE_NAME, normalizeAppLocale, type AppLocale } from "@/i18n/config";
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
    avatarURL: "Avatar URL",
    displayName: "Display name",
    email: "Email",
    locale: "Language",
    password: "Password",
    phone: "Phone",
    subscriptionExpiresAt: "Subscription expiry",
    subscriptionTier: "Subscription plan",
    timezone: "Timezone",
    username: "Username",
  },
  "zh-CN": {
    avatarURL: "头像地址",
    displayName: "昵称",
    email: "邮箱",
    locale: "语言",
    password: "密码",
    phone: "手机号",
    subscriptionExpiresAt: "订阅到期时间",
    subscriptionTier: "订阅方案",
    timezone: "时区",
    username: "用户名",
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
  return normalizeAppLocale(cookieValue ? decodeURIComponent(cookieValue) : undefined);
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

export function resolveLocalizedErrorMessage(error: unknown, fallback?: string): string {
  const locale = readClientLocale();
  if (error instanceof ApiError && error.errorCode) {
    const validationMessage = resolveRequestBodyValidationMessage(error, locale);
    if (validationMessage) {
      return validationMessage;
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
