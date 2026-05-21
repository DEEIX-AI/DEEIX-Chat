export const APP_LOCALES = ["en-US", "zh-CN"] as const;

export type AppLocale = (typeof APP_LOCALES)[number];

export const DEFAULT_LOCALE: AppLocale = "en-US";
export const LOCALE_COOKIE_NAME = "deeix_chat_locale";

export const APP_LOCALE_LABELS: Record<AppLocale, string> = {
  "en-US": "English",
  "zh-CN": "简体中文",
};

export function normalizeAppLocale(value: string | null | undefined): AppLocale {
  const normalized = String(value ?? "").trim();
  return APP_LOCALES.includes(normalized as AppLocale) ? (normalized as AppLocale) : DEFAULT_LOCALE;
}
