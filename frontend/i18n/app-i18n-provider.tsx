"use client";

import * as React from "react";
import { NextIntlClientProvider } from "next-intl";

import { DEFAULT_LOCALE, LOCALE_COOKIE_NAME, normalizeAppLocale, resolveBrowserLocale, type AppLocale } from "@/i18n/config";
import { applyBrandingToMessages, DEFAULT_MESSAGES, loadLocaleMessages, type AppMessages } from "@/i18n/messages";
import { useBranding } from "@/shared/config/branding-provider";

type AppI18nContextValue = {
  locale: AppLocale;
  setLocale: (locale: AppLocale) => Promise<void>;
};

const AppI18nContext = React.createContext<AppI18nContextValue | null>(null);

function readLocaleCookie(): AppLocale | null {
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

function readBrowserLocale(): AppLocale {
  if (typeof navigator === "undefined") {
    return DEFAULT_LOCALE;
  }
  return resolveBrowserLocale(navigator.languages?.length ? navigator.languages : [navigator.language]);
}

function writeLocaleCookie(locale: AppLocale): void {
  if (typeof document === "undefined") {
    return;
  }
  document.cookie = `${LOCALE_COOKIE_NAME}=${encodeURIComponent(locale)}; path=/; max-age=31536000; samesite=lax`;
}

function applyDocumentLocale(locale: AppLocale): void {
  if (typeof document === "undefined") {
    return;
  }
  document.documentElement.lang = locale;
}

export function AppI18nProvider({ children }: { children: React.ReactNode }) {
  const branding = useBranding();
  const [locale, setLocaleState] = React.useState<AppLocale>(DEFAULT_LOCALE);
  const [localeMessages, setLocaleMessages] = React.useState<AppMessages>(DEFAULT_MESSAGES);
  const localeRef = React.useRef<AppLocale>(DEFAULT_LOCALE);

  const applyLocale = React.useCallback(async (nextLocale: AppLocale, persist: boolean) => {
    const normalized = normalizeAppLocale(nextLocale);
    if (normalized === localeRef.current) {
      if (persist) {
        writeLocaleCookie(normalized);
      }
      applyDocumentLocale(normalized);
      return;
    }

    const nextMessages = await loadLocaleMessages(normalized);
    localeRef.current = normalized;
    setLocaleState(normalized);
    setLocaleMessages(nextMessages);
    if (persist) {
      writeLocaleCookie(normalized);
    }
    applyDocumentLocale(normalized);
  }, []);

  const setLocale = React.useCallback(async (nextLocale: AppLocale) => {
    await applyLocale(nextLocale, true);
  }, [applyLocale]);

  React.useEffect(() => {
    const cookieLocale = readLocaleCookie();
    void applyLocale(cookieLocale ?? readBrowserLocale(), cookieLocale !== null);
  }, [applyLocale]);

  const value = React.useMemo<AppI18nContextValue>(
    () => ({
      locale,
      setLocale,
    }),
    [locale, setLocale],
  );
  const messages = React.useMemo(
    () => applyBrandingToMessages(localeMessages, branding.title),
    [branding.title, localeMessages],
  );

  return (
    <AppI18nContext.Provider value={value}>
      <NextIntlClientProvider locale={locale} messages={messages} timeZone="UTC">
        {children}
      </NextIntlClientProvider>
    </AppI18nContext.Provider>
  );
}

export function useAppLocale() {
  const context = React.useContext(AppI18nContext);
  if (!context) {
    throw new Error("useAppLocale must be used within AppI18nProvider");
  }
  return context;
}
