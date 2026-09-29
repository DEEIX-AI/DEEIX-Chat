import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { deleteUserMemory, listUserMemories, upsertUserMemory } from "@/shared/api/memory";
import type { UserMemoryDTO } from "@/shared/api/memory-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

/**
 * The user's "preference" scoped memories. Add/edit reuse the upsert endpoint
 * and patch the local list instead of reloading; adding an existing key
 * updates that row in place.
 */
export function useSettingsPreferenceMemories() {
  const t = useTranslations("settings.chatPage.memory");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [items, setItems] = React.useState<UserMemoryDTO[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [adding, setAdding] = React.useState(false);

  React.useEffect(() => {
    void (async () => {
      setLoading(true);
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const all = await listUserMemories(token);
        setItems(all.filter((m) => m.scope === "preference"));
      } catch {
        // ignore
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  // `onAdded` runs inside the request flow (before `adding` resets) so the
  // caller's draft reset and dialog close land in the same update as the list change.
  const addPreference = React.useCallback(async (rawKey: string, rawValue: string, onAdded: () => void) => {
    const key = rawKey.trim();
    const value = rawValue.trim();
    if (!key || !value) return;
    setAdding(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      await upsertUserMemory(token, key, value, "preference");
      setItems((prev) => {
        const exists = prev.find((m) => m.memoryKey === key);
        if (exists) {
          return prev.map((m) => m.memoryKey === key ? { ...m, value } : m);
        }
        return [{
          id: Date.now(),
          userID: 0,
          memoryKey: key,
          value,
          scope: "preference",
          updatedBy: "user",
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }, ...prev];
      });
      onAdded();
      toast.success(t("added"));
    } catch (error) {
      toast.error(t("addFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setAdding(false);
    }
  }, [resolveErrorMessage, t]);

  const editPreference = React.useCallback(async (memoryKey: string, value: string) => {
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      await upsertUserMemory(token, memoryKey, value, "preference");
      setItems((prev) => prev.map((m) => m.memoryKey === memoryKey ? { ...m, value } : m));
      toast.success(t("updated"));
    } catch (error) {
      toast.error(t("updateFailed"), { description: resolveErrorMessage(error) });
    }
  }, [resolveErrorMessage, t]);

  const deletePreference = React.useCallback(async (memoryKey: string) => {
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      await deleteUserMemory(token, memoryKey);
      setItems((prev) => prev.filter((m) => m.memoryKey !== memoryKey));
      toast.success(t("deleted"));
    } catch (error) {
      toast.error(t("deleteFailed"), { description: resolveErrorMessage(error) });
    }
  }, [resolveErrorMessage, t]);

  return { items, loading, adding, addPreference, editPreference, deletePreference };
}
