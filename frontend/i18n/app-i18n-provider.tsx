"use client";

import * as React from "react";
import { NextIntlClientProvider } from "next-intl";

import { DEFAULT_LOCALE, LOCALE_COOKIE_NAME, normalizeAppLocale, type AppLocale } from "@/i18n/config";
import { DEFAULT_MESSAGES, loadLocaleMessages, type AppMessages } from "@/i18n/messages";

type AppI18nContextValue = {
  locale: AppLocale;
  setLocale: (locale: AppLocale) => Promise<void>;
};

const AppI18nContext = React.createContext<AppI18nContextValue | null>(null);

function readLocaleCookie(): AppLocale {
  if (typeof document === "undefined") {
    return DEFAULT_LOCALE;
  }
  const raw = document.cookie
    .split(";")
    .map((part) => part.trim())
    .find((part) => part.startsWith(`${LOCALE_COOKIE_NAME}=`));
  if (!raw) {
    return DEFAULT_LOCALE;
  }
  return normalizeAppLocale(decodeURIComponent(raw.slice(LOCALE_COOKIE_NAME.length + 1)));
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
  const [locale, setLocaleState] = React.useState<AppLocale>(DEFAULT_LOCALE);
  const [messages, setMessages] = React.useState<AppMessages>(DEFAULT_MESSAGES);
  const localeRef = React.useRef<AppLocale>(DEFAULT_LOCALE);

  const setLocale = React.useCallback(async (nextLocale: AppLocale) => {
    const normalized = normalizeAppLocale(nextLocale);
    if (normalized === localeRef.current) {
      writeLocaleCookie(normalized);
      applyDocumentLocale(normalized);
      return;
    }

    const nextMessages = await loadLocaleMessages(normalized);
    localeRef.current = normalized;
    setLocaleState(normalized);
    setMessages(nextMessages);
    writeLocaleCookie(normalized);
    applyDocumentLocale(normalized);
  }, []);

  React.useEffect(() => {
    void setLocale(readLocaleCookie());
  }, [setLocale]);

  const value = React.useMemo<AppI18nContextValue>(
    () => ({
      locale,
      setLocale,
    }),
    [locale, setLocale],
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
