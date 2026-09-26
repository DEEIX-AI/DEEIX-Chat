"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { isDesktopApp } from "@/shared/platform";
import { checkForUpdate, relaunchApp } from "@/shared/platform/desktop-updater";

// Desktop only: checks the release endpoint on launch and every few hours and
// offers the update in a toast. Nothing is downloaded until the user accepts.

const CHECK_INTERVAL_MS = 4 * 60 * 60 * 1000;
const TOAST_ID = "desktop-update";

export function DesktopUpdateNotifier(): null {
  const t = useTranslations("desktopUpdate");
  const tRef = React.useRef(t);
  tRef.current = t;

  React.useEffect(() => {
    if (!isDesktopApp()) {
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
        toast.error(tRef.current("installFailed", { message: error instanceof Error ? error.message : String(error) }), { id: TOAST_ID });
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
  }, []);

  return null;
}
