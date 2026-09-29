"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { upsertUserMemory } from "@/shared/api/memory";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

/** Saves a quick "remember this preference" memory from the message actions. */
export function useChatMemoryPin() {
  const t = useTranslations("chat.messages");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [saving, setSaving] = React.useState(false);

  // `onSaved` runs before `saving` resets so the cleared draft and the idle button land together.
  const savePreference = React.useCallback(
    async (key: string, value: string, onSaved: () => void) => {
      const trimmedKey = key.trim();
      const trimmedValue = value.trim();
      if (!trimmedKey || !trimmedValue) return;
      setSaving(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("authTokenMissing"));
          return;
        }
        await upsertUserMemory(token, trimmedKey, trimmedValue, "preference");
        toast.success(t("memorySaved"), { description: t("memorySavedDescription") });
        onSaved();
      } catch (error) {
        toast.error(t("memorySaveFailed"), { description: resolveErrorMessage(error) });
      } finally {
        setSaving(false);
      }
    },
    [resolveErrorMessage, t],
  );

  return { saving, savePreference };
}
