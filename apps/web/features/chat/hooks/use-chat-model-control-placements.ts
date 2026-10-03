"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import {
  MODEL_CONTROL_PLACEMENTS_SETTING_KEY,
  type ModelControlPlacements,
  parseModelControlPlacements,
} from "@/features/chat/model/chat-model-control-placements";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { updateUserSettings, useUserSettings } from "@/entities/user-settings";

/** Where the user placed model controls (toolbar vs "…" menu), synced to the account settings. */
export function useChatModelControlPlacements() {
  const t = useTranslations("chat.modelControls");
  const { settings } = useUserSettings();
  const saved = React.useMemo(
    () => parseModelControlPlacements(settings[MODEL_CONTROL_PLACEMENTS_SETTING_KEY]),
    [settings],
  );
  const [optimistic, setOptimistic] = React.useState<ModelControlPlacements | null>(null);
  const placements = optimistic ?? saved;

  const setPlacements = React.useCallback(async (next: ModelControlPlacements) => {
    setOptimistic(next);
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error("session expired");
      await updateUserSettings(token, { [MODEL_CONTROL_PLACEMENTS_SETTING_KEY]: JSON.stringify(next) });
    } catch {
      toast.error(t("pinSaveFailed"));
    } finally {
      setOptimistic(null);
    }
  }, [t]);

  return { placements, setPlacements };
}
