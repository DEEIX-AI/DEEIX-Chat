import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { getAdminReferenceData, listAdminSettings, patchAdminSettings } from "@/features/admin/api";
import {
  CONVERSATION_DEFAULT_MODEL_SYSTEM,
  CONVERSATION_TASK_MODEL_FOLLOW,
  type ConversationSettingsField,
  fieldID,
  flattenConversationSettings,
} from "@/features/admin/model/conversation-settings";
import { buildTaskModelOptions } from "@/features/admin/model/task-model-options";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import type { PatchSettingItem } from "@/shared/api/settings-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { ModelSelectOption } from "@/entities/model";
import { overrideFeaturePolicy } from "@/shared/hooks/use-feature-policy";

/**
 * Loads the conversation settings together with the model list used by the
 * task/default model pickers, tracks per-field dirty state and saves one
 * section's dirty fields at a time.
 */
export function useAdminConversationSettings(conversationSettingsFields: ConversationSettingsField[]) {
  const t = useTranslations("adminConversation");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [settingsMap, setSettingsMap] = React.useState<Record<string, string>>({});
  const [savedMap, setSavedMap] = React.useState<Record<string, string>>({});
  const [taskModelOptions, setTaskModelOptions] = React.useState<ModelSelectOption[]>(() =>
    buildTaskModelOptions({
      models: [],
      followLabel: t("taskModel.follow"),
      followValue: CONVERSATION_TASK_MODEL_FOLLOW,
    }),
  );
  const [defaultModelOptions, setDefaultModelOptions] = React.useState<ModelSelectOption[]>(() =>
    buildTaskModelOptions({
      models: [],
      followLabel: t("defaultModel.systemRecommended"),
      followValue: CONVERSATION_DEFAULT_MODEL_SYSTEM,
    }),
  );
  const loadSettings = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const [grouped, referenceData] = await Promise.all([
        listAdminSettings(token),
        getAdminReferenceData(token).catch((): null => null),
      ]);
      const nextModelOptions = buildTaskModelOptions({
        models: referenceData?.models ?? [],
        followLabel: t("taskModel.follow"),
        followValue: CONVERSATION_TASK_MODEL_FOLLOW,
      });
      const nextDefaultModelOptions = buildTaskModelOptions({
        models: referenceData?.models ?? [],
        followLabel: t("defaultModel.systemRecommended"),
        followValue: CONVERSATION_DEFAULT_MODEL_SYSTEM,
      });
      const flattened = flattenConversationSettings(grouped);
      setTaskModelOptions(nextModelOptions);
      setDefaultModelOptions(nextDefaultModelOptions);
      setSettingsMap(flattened);
      setSavedMap(flattened);
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  const dirtyFieldIDs = React.useMemo(() => {
    const result = new Set<string>();
    for (const field of conversationSettingsFields) {
      const id = fieldID(field);
      if ((settingsMap[id] ?? "") !== (savedMap[id] ?? "")) {
        result.add(id);
      }
    }
    return result;
  }, [conversationSettingsFields, savedMap, settingsMap]);

  const handleSave = React.useCallback(async (fields: ConversationSettingsField[]) => {
    const items: PatchSettingItem[] = fields
      .filter((field) => dirtyFieldIDs.has(fieldID(field)))
      .map((field) => ({
        namespace: field.namespace,
        key: field.key,
        value: settingsMap[fieldID(field)] ?? "",
      }));
    if (items.length === 0) {
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const grouped = await patchAdminSettings(token, { items });
      const flattened = flattenConversationSettings(grouped);
      setSettingsMap(flattened);
      setSavedMap(flattened);
      overrideFeaturePolicy({ processTraceEnabled: flattened["chat.process_trace_enabled"] !== "false" });
      toast.success(t("toast.updated"));
    } catch (error) {
      toast.error(t("toast.saveFailed"), { description: resolveAdminErrorMessage(error) });
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
    taskModelOptions,
    defaultModelOptions,
    dirtyFieldIDs,
    handleSave,
  };
}
