import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono, JetBrains_Mono } from "next/font/google";

import { AppVersionGuard } from "@/features/layouts";
import { AppearancePreferencesProvider } from "@/features/settings";
import { AppI18nProvider } from "@/i18n/app-i18n-provider";
import { DevtoolsBrandBanner } from "@/shared/components/devtools-brand-banner";
import { ThemeProvider } from "@/shared/components/theme-provider";
import { PWAServiceWorkerRegister } from "@/shared/components/pwa-service-worker-register";
import { brandAssets, brandText } from "@/shared/lib/branding";
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

const jetBrainsMono = JetBrains_Mono({
  variable: "--font-jetbrains-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  applicationName: brandText.title,
  title: brandText.title,
  description: brandText.description,
  manifest: "/manifest.webmanifest",
  appleWebApp: {
    capable: true,
    statusBarStyle: "default",
    title: brandText.title,
  },
  formatDetection: {
    telephone: false,
  },
  icons: {
    icon: [
      { url: brandAssets.favicon },
      { url: brandAssets.pwaIcon192, sizes: "192x192", type: "image/png" },
      { url: brandAssets.pwaIcon512, sizes: "512x512", type: "image/png" },
    ],
    apple: [
      { url: brandAssets.appleTouchIcon180, sizes: "180x180", type: "image/png" },
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
    <html
      lang="en"
      className={`${geistSans.variable} ${geistMono.variable} ${jetBrainsMono.variable} h-full`}
      suppressHydrationWarning
    >
      <body
        className="h-full min-h-svh overflow-hidden antialiased"
      >
        <AppI18nProvider>
          <ThemeProvider>
            <AppearancePreferencesProvider>
              {children}
              <AppVersionGuard />
              <PWAServiceWorkerRegister />
              <Toaster />
              <DevtoolsBrandBanner />
            </AppearancePreferencesProvider>
          </ThemeProvider>
        </AppI18nProvider>
      </body>
    </html>
  );
}
