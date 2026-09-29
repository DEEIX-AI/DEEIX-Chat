"use client";

import * as React from "react";
import { NextIntlClientProvider } from "next-intl";

import { detectLocale } from "@/i18n/detect-locale";
import enDesktopTabs from "@/i18n/messages/en-US/desktop-tabs.json";
import zhDesktopTabs from "@/i18n/messages/zh-CN/desktop-tabs.json";
import { ThemeProvider } from "@/shared/components/theme-provider";
import type { AppLocale } from "@/i18n/config";

const MESSAGES: Record<AppLocale, { desktopTabs: typeof enDesktopTabs }> = {
  "en-US": { desktopTabs: enDesktopTabs },
  "zh-CN": { desktopTabs: zhDesktopTabs },
};

/** Theme + the strip's own strings; follows the app's saved locale and theme. */
export function ShellProviders({ children }: { children: React.ReactNode }) {
  const [locale, setLocale] = React.useState<AppLocale>("en-US");
  React.useEffect(() => {
    setLocale(detectLocale());
  }, []);
  return (
    <ThemeProvider>
      <NextIntlClientProvider locale={locale} messages={MESSAGES[locale]} timeZone="UTC">
        {children}
      </NextIntlClientProvider>
    </ThemeProvider>
  );
}
