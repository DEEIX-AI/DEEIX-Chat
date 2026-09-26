import { DEFAULT_LOCALE, LOCALE_COOKIE_NAME, normalizeAppLocale, resolveBrowserLocale, type AppLocale } from "@/i18n/config";

export function readLocaleCookie(): AppLocale | null {
  if (typeof document === "undefined") {
    return null;
  }
  const raw = document.cookie
    .split(";")
    .map((part) => part.trim())
    .find((part) => part.startsWith(`${LOCALE_COOKIE_NAME}=`));
  if (!raw) {
    return null;
  }
  return normalizeAppLocale(decodeURIComponent(raw.slice(LOCALE_COOKIE_NAME.length + 1)));
}

export function readBrowserLocale(): AppLocale {
  if (typeof navigator === "undefined") {
    return DEFAULT_LOCALE;
  }
  return resolveBrowserLocale(navigator.languages?.length ? navigator.languages : [navigator.language]);
}

/** Saved preference, else the browser's language. */
export function detectLocale(): AppLocale {
  return readLocaleCookie() ?? readBrowserLocale();
}
