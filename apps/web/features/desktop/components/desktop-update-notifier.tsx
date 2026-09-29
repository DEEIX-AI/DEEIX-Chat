"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useDesktopDistribution } from "@/features/desktop/hooks/use-desktop-distribution";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { checkUpdateNotice } from "@/shared/platform/desktop-distribution";
import { openExternal } from "@/shared/platform/desktop-open";
import { checkForUpdate, relaunchApp } from "@/shared/platform/desktop-updater";

// Desktop only: checks the release endpoint on launch and every few hours. What
// happens next depends on the distribution's update mode (decided by the shell):
//   auto      offer the update in a toast; nothing is downloaded until the user accepts;
//   notify    (portable) link to the download page, never self-replace;
//   disabled  (MSI, policy, dev) no checks at all.

const CHECK_INTERVAL_MS = 4 * 60 * 60 * 1000;
const TOAST_ID = "desktop-update";

// Versions already announced in notify mode, kept for the lifetime of the page so
// a remount does not announce the same release again.
const noticedVersions = new Set<string>();

export function DesktopUpdateNotifier(): null {
  const t = useTranslations("desktopUpdate");
  const tRef = React.useRef(t);
  tRef.current = t;
  const resolveErrorMessage = useLocalizedErrorMessage();
  const resolveErrorMessageRef = React.useRef(resolveErrorMessage);
  resolveErrorMessageRef.current = resolveErrorMessage;
  const { status, distribution } = useDesktopDistribution();
  // Browsers report "unsupported" and a failed lookup falls back to "disabled",
  // so only a shell that answered can enable checks.
  const updateMode = status === "ready" ? distribution.updateMode : "disabled";
  const releasesUrl = distribution.releasesUrl;

  React.useEffect(() => {
    if (updateMode !== "auto") {
      return;
    }
    let offered = "";

    const install = async (run: () => Promise<void>, version: string) => {
      toast.loading(tRef.current("installing"), { id: TOAST_ID, duration: Infinity });
      try {
        await run();
        toast.success(tRef.current("installed", { version }), {
          id: TOAST_ID,
          duration: Infinity,
          action: { label: tRef.current("relaunch"), onClick: () => void relaunchApp() },
        });
      } catch (error) {
        // Tauri commands reject with plain strings; surface that text instead of "unknown error".
        const reason = typeof error === "string" && error.trim() ? error.trim() : undefined;
        toast.error(tRef.current("installFailed", { message: resolveErrorMessageRef.current(error, reason) }), {
          id: TOAST_ID,
        });
      }
    };

    const check = async () => {
      const result = await checkForUpdate();
      if (result.kind !== "available" || result.version === offered) {
        return;
      }
      offered = result.version;
      toast.info(tRef.current("available", { version: result.version }), {
        id: TOAST_ID,
        duration: Infinity,
        action: { label: tRef.current("install"), onClick: () => void install(result.install, result.version) },
      });
    };

    void check();
    const timer = window.setInterval(() => void check(), CHECK_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [updateMode]);

  React.useEffect(() => {
    if (updateMode !== "notify" || !isWebUrl(releasesUrl)) {
      return;
    }

    const openDownloadPage = async () => {
      try {
        await openExternal(releasesUrl);
      } catch (error) {
        const reason = typeof error === "string" && error.trim() ? error.trim() : undefined;
        toast.error(tRef.current("openFailed", { message: resolveErrorMessageRef.current(error, reason) }), {
          id: TOAST_ID,
        });
      }
    };

    const check = async () => {
      let notice: Awaited<ReturnType<typeof checkUpdateNotice>>;
      try {
        notice = await checkUpdateNotice();
      } catch {
        // A background check that fails (offline, endpoint down) stays silent,
        // like the auto mode check; the next interval tries again.
        return;
      }
      if (!notice || noticedVersions.has(notice.version)) {
        return;
      }
      noticedVersions.add(notice.version);
      toast.info(tRef.current("available", { version: notice.version }), {
        id: TOAST_ID,
        duration: Infinity,
        action: { label: tRef.current("download"), onClick: () => void openDownloadPage() },
      });
    };

    void check();
    const timer = window.setInterval(() => void check(), CHECK_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [updateMode, releasesUrl]);

  return null;
}

// The download page is opened in the system browser; never hand it anything but http(s).
function isWebUrl(value: string): boolean {
  try {
    const { protocol } = new URL(value);
    return protocol === "https:" || protocol === "http:";
  } catch {
    return false;
  }
}
