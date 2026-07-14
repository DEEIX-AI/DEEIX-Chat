"use client";

import * as React from "react";

import { useAppLocale } from "@/i18n/app-i18n-provider";
import { normalizeAppLocale } from "@/i18n/config";
import { useAuthSession } from "@/shared/auth/auth-session-context";

export function UserLocaleSync() {
  const { setLocale } = useAppLocale();
  const { user, userStatus } = useAuthSession();
  const syncedUserLocaleRef = React.useRef("");

  React.useEffect(() => {
    if (userStatus !== "ready" || !user?.locale || user.initialSecurityRequired) {
      syncedUserLocaleRef.current = "";
      return;
    }
    const nextLocale = normalizeAppLocale(user.locale);
    const syncKey = `${user.publicID}:${nextLocale}`;
    if (syncKey === syncedUserLocaleRef.current) {
      return;
    }
    syncedUserLocaleRef.current = syncKey;
    void setLocale(nextLocale);
  }, [setLocale, user?.initialSecurityRequired, user?.locale, user?.publicID, userStatus]);

  return null;
}
