import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";

import { ChatFontProvider } from "@/features/layouts/components/providers/chat-font-provider";
import { AppVersionGuard } from "@/features/layouts/components/providers/app-version-guard";
import { FontSizeProvider } from "@/features/layouts/components/providers/font-size-provider";
import { WorkspaceShell } from "@/features/layouts/components/sections/workspace-shell";
import { AppI18nProvider } from "@/i18n/app-i18n-provider";
import { DevtoolsBrandBanner } from "@/shared/components/devtools-brand-banner";
import { ThemeProvider } from "@/shared/components/theme-provider";
import { PWAServiceWorkerRegister } from "@/shared/components/pwa-service-worker-register";
import { pwaAsset } from "@/shared/pwa/assets";
import { WebVitals } from "@/shared/observability/web-vitals";
import { Toaster } from "@/components/ui/sonner";

import "./globals.css";
import "katex/dist/katex.min.css";
import "streamdown/styles.css";

const geistSans = Geist({
  variable: "--font-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-mono",
  subsets: ["latin"],
});

const webVitalsEnabled = process.env.NEXT_PUBLIC_WEB_VITALS_DEBUG === "true";

export const metadata: Metadata = {
  applicationName: "DEEIX Chat",
  title: "DEEIX Chat",
  description: "DEEIX Chat is a multi-model AI conversation system.",
  manifest: "/manifest.webmanifest",
  appleWebApp: {
    capable: true,
    statusBarStyle: "default",
    title: "DEEIX Chat",
  },
  formatDetection: {
    telephone: false,
  },
  icons: {
    icon: [
      { url: pwaAsset("/pwa/icon.svg"), type: "image/svg+xml" },
      { url: pwaAsset("/pwa/icon-192.png"), sizes: "192x192", type: "image/png" },
      { url: pwaAsset("/pwa/icon-512.png"), sizes: "512x512", type: "image/png" },
    ],
    apple: [
      { url: pwaAsset("/pwa/apple-touch-icon.png"), sizes: "180x180", type: "image/png" },
    ],
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
    <html lang="en" className="h-full" suppressHydrationWarning>
      <body
        className={`${geistSans.variable} ${geistMono.variable} h-full min-h-svh overflow-hidden antialiased`}
      >
        <AppI18nProvider>
          <ThemeProvider>
            <FontSizeProvider>
              <ChatFontProvider>
                <WorkspaceShell>{children}</WorkspaceShell>
                <AppVersionGuard />
                <PWAServiceWorkerRegister />
                <Toaster />
                {webVitalsEnabled ? <WebVitals /> : null}
                <DevtoolsBrandBanner />
              </ChatFontProvider>
            </FontSizeProvider>
          </ThemeProvider>
        </AppI18nProvider>
      </body>
    </html>
  );
}
