"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { getAppVersion, resolveAppBuildID } from "@/shared/api/app-version";

const APP_VERSION_STORAGE_KEY = "deeix-chat:app-build-id";
const APP_VERSION_TOAST_ID = "deeix-chat:app-version-refresh";
const APP_VERSION_CHECK_INTERVAL_MS = 10 * 60 * 1000;

type CheckReason = "initial" | "interval" | "focus" | "visible";

function readLocalStorage(key: string): string {
  try {
    return window.localStorage.getItem(key)?.trim() ?? "";
  } catch {
    return "";
  }
}

function writeLocalStorage(key: string, value: string): void {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // Storage can be unavailable in private browsing or hardened environments.
  }
}

export function AppVersionGuard() {
  const t = useTranslations("common.appVersion");
  const tActions = useTranslations("common.actions");
  const checkingRef = React.useRef(false);
  const lastCheckAtRef = React.useRef(0);

  const showRefreshToast = React.useCallback(
    (nextBuildID: string) => {
      toast.info(t("title"), {
        id: APP_VERSION_TOAST_ID,
        description: t("description"),
        duration: Infinity,
        action: {
          label: tActions("refresh"),
          onClick: () => {
            writeLocalStorage(APP_VERSION_STORAGE_KEY, nextBuildID);
            window.location.reload();
          },
        },
      });
    },
    [t, tActions],
  );

  const checkVersion = React.useCallback(
    async (reason: CheckReason) => {
      const now = Date.now();
      if (checkingRef.current) {
        return;
      }
      if (reason !== "initial" && now - lastCheckAtRef.current < APP_VERSION_CHECK_INTERVAL_MS) {
        return;
      }

      checkingRef.current = true;
      lastCheckAtRef.current = now;
      try {
        const nextBuildID = resolveAppBuildID(await getAppVersion());
        if (!nextBuildID) {
          return;
        }

        const currentBuildID = readLocalStorage(APP_VERSION_STORAGE_KEY);
        if (!currentBuildID) {
          writeLocalStorage(APP_VERSION_STORAGE_KEY, nextBuildID);
          return;
        }
        if (currentBuildID === nextBuildID) {
          return;
        }

        showRefreshToast(nextBuildID);
      } catch {
        // Version checking is opportunistic and must not affect normal app usage.
      } finally {
        checkingRef.current = false;
      }
    },
    [showRefreshToast],
  );

  React.useEffect(() => {
    void checkVersion("initial");

    const intervalID = window.setInterval(() => {
      void checkVersion("interval");
    }, APP_VERSION_CHECK_INTERVAL_MS);

    const handleFocus = () => {
      void checkVersion("focus");
    };
    const handleVisibilityChange = () => {
      if (document.visibilityState === "visible") {
        void checkVersion("visible");
      }
    };

    window.addEventListener("focus", handleFocus);
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      window.clearInterval(intervalID);
      window.removeEventListener("focus", handleFocus);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [checkVersion]);

  return null;
}
