import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { listAdminSettingsByNamespace, patchAdminSettings } from "@/features/admin/api";
import {
  DESKTOP_DOWNLOAD_NAMESPACE,
  type DesktopDownloadSettingsField,
  desktopDownloadFieldID,
  flattenDesktopDownloadSettings,
} from "@/features/admin/model/desktop-download-settings";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import type { PatchSettingItem } from "@/shared/api/settings-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { overrideFeaturePolicy } from "@/shared/hooks/use-feature-policy";

/**
 * Loads and saves the desktop download entry settings (`desktop` namespace).
 * After a save the current tab's feature policy is updated so the user menu
 * reflects the change without a reload.
 */
export function useAdminAboutDesktopDownload(fields: DesktopDownloadSettingsField[]) {
  const t = useTranslations("adminUsers.aboutPage.desktopDownload");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [settingsMap, setSettingsMap] = React.useState<Record<string, string>>(() => flattenDesktopDownloadSettings([]));
  const [savedMap, setSavedMap] = React.useState<Record<string, string>>(() => flattenDesktopDownloadSettings([]));

  React.useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
          return;
        }
        const items = await listAdminSettingsByNamespace(token, DESKTOP_DOWNLOAD_NAMESPACE);
        if (cancelled) return;
        const flattened = flattenDesktopDownloadSettings(items);
        setSettingsMap(flattened);
        setSavedMap(flattened);
      } catch (error) {
        if (!cancelled) toast.error(t("toast.loadFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [t]);

  const dirtyFieldIDs = React.useMemo(() => {
    const result = new Set<string>();
    for (const field of fields) {
      const id = desktopDownloadFieldID(field);
      if ((settingsMap[id] ?? "") !== (savedMap[id] ?? "")) result.add(id);
    }
    return result;
  }, [fields, savedMap, settingsMap]);

  const updateValue = React.useCallback((field: DesktopDownloadSettingsField, value: string) => {
    setSettingsMap((prev) => ({ ...prev, [desktopDownloadFieldID(field)]: value }));
  }, []);

  const handleSave = React.useCallback(async () => {
    const items: PatchSettingItem[] = fields
      .filter((field) => dirtyFieldIDs.has(desktopDownloadFieldID(field)))
      .map((field) => ({
        namespace: field.namespace,
        key: field.key,
        value: (settingsMap[desktopDownloadFieldID(field)] ?? "").trim(),
      }));
    if (items.length === 0) return;
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const grouped = await patchAdminSettings(token, { items });
      const flattened = flattenDesktopDownloadSettings(grouped[DESKTOP_DOWNLOAD_NAMESPACE]);
      setSettingsMap(flattened);
      setSavedMap(flattened);
      overrideFeaturePolicy({
        desktopDownloadEnabled: flattened["desktop.download_enabled"] === "true",
        desktopDownloadURL: flattened["desktop.download_url"] ?? "",
      });
      toast.success(t("toast.updated"));
    } catch (error) {
      toast.error(t("toast.saveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [dirtyFieldIDs, fields, settingsMap, t]);

  return { loading, saving, settingsMap, savedMap, dirtyFieldIDs, updateValue, handleSave };
}
