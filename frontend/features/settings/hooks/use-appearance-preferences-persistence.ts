"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  readLocalAppearancePreferences,
  serializeAppearancePreferences,
  type AppearancePreferencePatch,
} from "@/features/settings/utils/appearance-preferences";
import { resolveLocalizedErrorMessage } from "@/i18n/resolve-error-message";
import { patchMe } from "@/shared/api/auth";
import { useAuthSession } from "@/shared/auth/auth-session-context";

export function useAppearancePreferencesPersistence() {
  const t = useTranslations("settings");
  const { accessToken } = useAuthSession();
  const appearanceSaveTimerRef = React.useRef<number | null>(null);
  const pendingAppearancePatchRef = React.useRef<AppearancePreferencePatch>({});

  React.useEffect(() => {
    return () => {
      if (appearanceSaveTimerRef.current !== null) {
        window.clearTimeout(appearanceSaveTimerRef.current);
      }
    };
  }, []);

  return React.useCallback(
    (patch: AppearancePreferencePatch) => {
      if (!accessToken) {
        return;
      }

      pendingAppearancePatchRef.current = {
        ...pendingAppearancePatchRef.current,
        ...patch,
      };
      if (appearanceSaveTimerRef.current !== null) {
        window.clearTimeout(appearanceSaveTimerRef.current);
      }

      appearanceSaveTimerRef.current = window.setTimeout(() => {
        void (async () => {
          const pendingPatch = pendingAppearancePatchRef.current;
          pendingAppearancePatchRef.current = {};
          appearanceSaveTimerRef.current = null;
          const appearancePreferences = serializeAppearancePreferences({
            ...readLocalAppearancePreferences(),
            ...pendingPatch,
          });
          try {
            await patchMe(accessToken, { appearancePreferences });
          } catch (error) {
            toast.error(t("generalPage.toast.saveProfileFailed"), {
              description: resolveLocalizedErrorMessage(error),
            });
          }
        })();
      }, 300);
    },
    [accessToken, t],
  );
}
