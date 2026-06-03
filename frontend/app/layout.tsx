import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";

import { ChatFontProvider } from "@/features/layouts/components/providers/chat-font-provider";
import { AppVersionGuard } from "@/features/layouts/components/providers/app-version-guard";
import { FontSizeProvider } from "@/features/layouts/components/providers/font-size-provider";
import { WorkspaceShell } from "@/features/layouts/components/sections/workspace-shell";
import { AppI18nProvider } from "@/i18n/app-i18n-provider";
import { DevtoolsBrandBanner } from "@/shared/components/devtools-brand-banner";
import { ThemeProvider } from "@/shared/components/theme-provider";
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
  title: "DEEIX Chat",
  description: "DEEIX Chat is a multi-model AI conversation system.",
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  maximumScale: 1,
  userScalable: false,
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
