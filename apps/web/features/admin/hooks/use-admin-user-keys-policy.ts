import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { listAdminSettingsByNamespace, patchAdminSettings } from "@/features/admin/api";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { invalidatePersonalProviderAccess } from "@/shared/hooks/use-personal-provider-access";

const USER_KEYS_SETTINGS_NAMESPACE = "personal_provider";

/** Setting keys in display order; values are kept as the raw strings the settings API stores. */
const USER_KEYS_SETTING_KEYS = ["enabled", "max_per_user", "blocked_hosts"] as const;

export type UserKeysSettingKey = (typeof USER_KEYS_SETTING_KEYS)[number];
type UserKeysSettings = Record<UserKeysSettingKey, string>;

const DEFAULT_SETTINGS: UserKeysSettings = {
  enabled: "false",
  max_per_user: "10",
  blocked_hosts: "",
};

/**
 * The admin policy for users' own API keys (off by default). Edits stay local
 * until saved, like the other settings sections; saving refreshes the cached
 * access flag so this tab's settings sidebar follows immediately.
 */
export function useAdminUserKeysPolicy() {
  const t = useTranslations("adminUserKeys");
  const [values, setValues] = React.useState<UserKeysSettings>(DEFAULT_SETTINGS);
  const [saved, setSaved] = React.useState<UserKeysSettings>(DEFAULT_SETTINGS);
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const items = await listAdminSettingsByNamespace(token, USER_KEYS_SETTINGS_NAMESPACE);
        if (cancelled) return;
        const next = { ...DEFAULT_SETTINGS };
        for (const key of USER_KEYS_SETTING_KEYS) {
          const item = items.find((entry) => entry.key === key);
          if (item) next[key] = item.value;
        }
        setValues(next);
        setSaved(next);
      } catch (error) {
        if (!cancelled) toast.error(resolveAdminErrorMessage(error, t("toasts.loadFailed")));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [t]);

  const isDirty = (key: UserKeysSettingKey) => values[key].trim() !== saved[key].trim();
  const dirty = USER_KEYS_SETTING_KEYS.some(isDirty);

  const setValue = React.useCallback((key: UserKeysSettingKey, value: string) => {
    setValues((current) => ({ ...current, [key]: value }));
  }, []);

  const save = React.useCallback(async () => {
    const changed = USER_KEYS_SETTING_KEYS.filter((key) => values[key].trim() !== saved[key].trim());
    if (changed.length === 0) return;
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error("missing access token");
      await patchAdminSettings(token, {
        items: changed.map((key) => ({ namespace: USER_KEYS_SETTINGS_NAMESPACE, key, value: values[key].trim() })),
      });
      setSaved(values);
      invalidatePersonalProviderAccess();
      toast.success(t("toasts.saved"));
    } catch (error) {
      toast.error(resolveAdminErrorMessage(error, t("toasts.saveFailed")));
    } finally {
      setSaving(false);
    }
  }, [saved, t, values]);

  return { values, setValue, isDirty, dirty, save, loading, saving, enabled: saved.enabled === "true" };
}
