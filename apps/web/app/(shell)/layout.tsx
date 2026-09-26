import { Geist } from "next/font/google";

import { ShellProviders } from "@/features/platform/components/shell-providers";
import { ThemeBootstrapScript } from "@/shared/components/theme-bootstrap-script";

import "../globals.css";

// Root layout for the desktop shell's own pages (the tab strip). Deliberately
// not the app layout: no branding fetch, no i18n bundles, no guards — these
// pages have no server and must stay cheap, one per window.

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
  display: "optional",
});

export default function ShellLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${geistSans.variable} h-full`} suppressHydrationWarning>
      <head>
        <ThemeBootstrapScript />
      </head>
      <body className="h-full overflow-hidden antialiased">
        <ShellProviders>{children}</ShellProviders>
      </body>
    </html>
  );
}
