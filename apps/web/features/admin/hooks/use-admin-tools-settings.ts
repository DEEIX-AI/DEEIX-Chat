import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { listAdminSettings, patchAdminSettings } from "@/features/admin/api";
import {
  TOOL_SETTINGS_FIELDS,
  applyToolSettingsDefaults,
  flattenToolSettings,
  toolFieldID,
} from "@/features/admin/model/tool-settings";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import type { PatchSettingItem } from "@/shared/api/settings-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

/** Loads and saves the MCP tool settings (enable switch, prompt, runtime limits). */
export function useAdminToolsSettings() {
  const t = useTranslations("adminTools");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [settingsMap, setSettingsMap] = React.useState<Record<string, string>>(() => applyToolSettingsDefaults({}));
  const [savedMap, setSavedMap] = React.useState<Record<string, string>>(() => applyToolSettingsDefaults({}));

  const loadSettings = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const grouped = await listAdminSettings(token);
      const flattened = flattenToolSettings(grouped);
      setSettingsMap(flattened);
      setSavedMap(flattened);
    } catch (error) {
      toast.error(t("toast.settingsLoadFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  const dirtyFieldIDs = React.useMemo(() => {
    const result = new Set<string>();
    for (const field of TOOL_SETTINGS_FIELDS) {
      const id = toolFieldID(field);
      if ((settingsMap[id] ?? "") !== (savedMap[id] ?? "")) {
        result.add(id);
      }
    }
    return result;
  }, [savedMap, settingsMap]);
  const handleSaveMCPSettings = React.useCallback(async () => {
    const items: PatchSettingItem[] = TOOL_SETTINGS_FIELDS
      .filter((field) => dirtyFieldIDs.has(toolFieldID(field)))
      .map((field) => ({
        namespace: field.namespace,
        key: field.key,
        value: settingsMap[toolFieldID(field)] ?? "",
      }));
    if (items.length === 0) {
      return;
    }

    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const grouped = await patchAdminSettings(token, { items });
      const flattened = flattenToolSettings(grouped);
      setSettingsMap(flattened);
      setSavedMap(flattened);
      toast.success(t("toast.settingsUpdated"));
    } catch (error) {
      toast.error(t("toast.saveFailed"), { description: resolveAdminErrorMessage(error, t("toast.unknownError")) });
    } finally {
      setSaving(false);
    }
  }, [dirtyFieldIDs, settingsMap, t]);

  return {
    loading,
    saving,
    settingsMap,
    setSettingsMap,
    savedMap,
    dirtyFieldIDs,
    handleSaveMCPSettings,
  };
}
