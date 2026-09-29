"use client";

import { ArrowUpRight } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { DropdownMenuItem, DropdownMenuItemIcon } from "@/components/ui/dropdown-menu";
import { resolveDesktopDownloadURL } from "@/features/shell/model/desktop-download";
import { useFeaturePolicy } from "@/shared/hooks/use-feature-policy";
import { isDesktopApp } from "@/shared/platform";

function subscribeNothing(): () => void {
  return () => {};
}

// Server and first client render agree on "not a browser build"; the real value is
// read after hydration, so the static export never mismatches.
function getIsBrowserBuild(): boolean {
  return !isDesktopApp();
}

function getServerIsBrowserBuild(): boolean {
  return false;
}

/**
 * "Download desktop app" entry of the user menu: opens the download page in a new tab.
 *
 * The platform check lives here on purpose (this file must not import
 * shared/capabilities): offering the desktop app to someone already inside it is
 * meaningless, so this is a platform-inherent entry, not a server feature. Whether
 * the entry exists at all is still decided by the admin feature policy.
 */
export function NavDesktopDownload({ onOpenPage }: { onOpenPage?: () => void }) {
  const t = useTranslations("common.navigation.desktopDownload");
  const { desktopDownloadEnabled, desktopDownloadURL, loaded } = useFeaturePolicy();
  const isBrowserBuild = React.useSyncExternalStore(subscribeNothing, getIsBrowserBuild, getServerIsBrowserBuild);
  const downloadURL = loaded ? resolveDesktopDownloadURL(desktopDownloadEnabled, desktopDownloadURL) : null;

  if (!isBrowserBuild || !downloadURL) return null;

  return (
    <DropdownMenuItem asChild onSelect={() => onOpenPage?.()}>
      <a href={downloadURL} target="_blank" rel="noopener noreferrer">
        <span className="min-w-0 flex-1 truncate">{t("label")}</span>
        <DropdownMenuItemIcon icon={ArrowUpRight} className="ml-auto text-muted-foreground" />
      </a>
    </DropdownMenuItem>
  );
}
