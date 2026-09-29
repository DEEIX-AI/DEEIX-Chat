import * as React from "react";

import { useAppLocale } from "@/i18n/app-i18n-provider";
import type { AppLocale } from "@/i18n/config";
import { logout, patchMe } from "@/shared/api/auth";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { clearSessionAndRedirectToLogin } from "@/shared/auth/session";
import { dispatchUserProfileUpdated } from "@/shared/auth/user-profile-events";

/**
 * Account actions behind the sidebar user menu: logout (always clears the local
 * session, even if the API call fails) and locale switching, which applies
 * locally first and then persists to the profile best-effort.
 */
export function useShellUserMenuActions() {
  const { locale, setLocale } = useAppLocale();
  const { accessToken, user: sessionUser } = useAuthSession();
  const [loggingOut, setLoggingOut] = React.useState(false);
  const [savingLocale, setSavingLocale] = React.useState<AppLocale | null>(null);

  const onLogout = React.useCallback(async () => {
    if (loggingOut) {
      return;
    }
    setLoggingOut(true);
    try {
      if (accessToken) {
        await logout(accessToken);
      }
    } catch {
      // Ignore logout API errors and clear local session to ensure exit.
    } finally {
      clearSessionAndRedirectToLogin();
      setLoggingOut(false);
    }
  }, [accessToken, loggingOut]);

  const onLocaleSelect = React.useCallback(
    async (nextLocale: AppLocale) => {
      if (nextLocale === locale && !savingLocale) {
        return;
      }

      if (sessionUser) {
        dispatchUserProfileUpdated({ ...sessionUser, locale: nextLocale });
      }
      void setLocale(nextLocale);

      if (!accessToken) {
        return;
      }

      setSavingLocale(nextLocale);
      try {
        const nextUser = await patchMe(accessToken, { locale: nextLocale });
        dispatchUserProfileUpdated(nextUser);
      } catch {
        // Keep the local language selection; a later profile refresh may retry or restore the server value.
      } finally {
        setSavingLocale((current) => (current === nextLocale ? null : current));
      }
    },
    [accessToken, locale, savingLocale, sessionUser, setLocale],
  );

  return { locale, loggingOut, savingLocale, onLogout, onLocaleSelect };
}
