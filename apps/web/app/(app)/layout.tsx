import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono, JetBrains_Mono } from "next/font/google";

import { DesktopBootstrap } from "@/features/platform/components/desktop-bootstrap";
import { DesktopUpdateNotifier } from "@/features/platform/components/desktop-update-notifier";
import { AppVersionGuard } from "@/features/layouts";
import { AppearancePreferencesProvider } from "@/features/settings";
import { AppI18nProvider } from "@/i18n/app-i18n-provider";
import { BrandingProvider } from "@/shared/config/branding-provider";
import { DevtoolsBrandBanner } from "@/shared/components/devtools-brand-banner";
import { ThemeBootstrapScript } from "@/shared/components/theme-bootstrap-script";
import { ThemeProvider } from "@/shared/components/theme-provider";
import { LegacyPWAServiceWorkerMigration } from "@/shared/pwa/migrations/legacy-service-worker-migration";
import { Toaster } from "@/components/ui/sonner";

import "../globals.css";
import "katex/dist/katex.min.css";
import "streamdown/styles.css";

// Font-face registration only; which family a surface uses is set by the theme
// tokens in globals.css (`--font-sans` etc.), hence the namespaced variables.
// `display: "optional"` avoids the metric-fallback swap frame on cold start —
// the desktop webview has no warm cache, so it would show on every launch.
// Options are written out per call: next/font rejects spreads.
const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
  display: "optional",
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
  display: "optional",
});

const jetBrainsMono = JetBrains_Mono({
  variable: "--font-jetbrains-mono",
  subsets: ["latin"],
  display: "optional",
});

export const metadata: Metadata = {
  appleWebApp: {
    capable: true,
    statusBarStyle: "default",
  },
  formatDetection: {
    telephone: false,
  },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  maximumScale: 1,
  userScalable: false,
  themeColor: "#0f172a",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className={`${geistSans.variable} ${geistMono.variable} ${jetBrainsMono.variable} h-full`}
      data-branding-pending="true"
      suppressHydrationWarning
    >
      <head>
        <ThemeBootstrapScript />
      </head>
      <body
        className="h-full min-h-svh overflow-hidden antialiased"
      >
        <BrandingProvider>
          <AppI18nProvider>
            <ThemeProvider>
              <AppearancePreferencesProvider>
                <DesktopBootstrap>
                  {children}
                  <AppVersionGuard />
                  <DesktopUpdateNotifier />
                  <LegacyPWAServiceWorkerMigration />
                  <DevtoolsBrandBanner />
                </DesktopBootstrap>
                <Toaster />
              </AppearancePreferencesProvider>
            </ThemeProvider>
          </AppI18nProvider>
        </BrandingProvider>
      </body>
    </html>
  );
}
