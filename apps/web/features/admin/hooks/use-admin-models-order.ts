"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { listAdminLLMModels, reorderAdminLLMModels } from "@/features/admin/api";
import type { AdminLLMModelDTO } from "@/features/admin/api/llm-types";
import { invalidateAdminReferenceDataCache } from "@/features/admin/api/reference-data";
import { listAllAdminPages } from "@/features/admin/api/shared";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { resolveModelPresentationGroup } from "@/entities/model";

export type ModelOrderGroup = {
  groupKey: string;
  label: string;
  icon: string;
  items: AdminLLMModelDTO[];
};

function buildModelOrderGroups(models: AdminLLMModelDTO[]): ModelOrderGroup[] {
  const groups = new Map<string, ModelOrderGroup>();
  for (const model of models) {
    const presentation = resolveModelPresentationGroup(model);
    const current = groups.get(presentation.key);
    if (current) {
      current.items.push(model);
      continue;
    }
    groups.set(presentation.key, {
      groupKey: presentation.key,
      label: presentation.label,
      icon: presentation.icon,
      items: [model],
    });
  }
  return Array.from(groups.values());
}

function flattenGroups(groups: ModelOrderGroup[]): AdminLLMModelDTO[] {
  return groups.flatMap((group) => group.items);
}

type UseAdminModelsOrderOptions = {
  open: boolean;
  onSaved: () => void;
  onClose: () => void;
};

export function useAdminModelsOrder({ open, onSaved, onClose }: UseAdminModelsOrderOptions) {
  const t = useTranslations("adminModels.order");
  const toastT = useTranslations("adminModels.toast");
  const [models, setModels] = React.useState<AdminLLMModelDTO[]>([]);
  const [selectedGroupKey, setSelectedGroupKey] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [dirty, setDirty] = React.useState(false);
  const initialOrderRef = React.useRef<string>("");

  const groups = React.useMemo(() => buildModelOrderGroups(models), [models]);
  const selectedGroup = groups.find((group) => group.groupKey === selectedGroupKey) ?? groups[0] ?? null;

  const loadModels = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
        return;
      }
      const results = await listAllAdminPages((options) =>
        listAdminLLMModels(token, {
          ...options,
          onlyAvailable: true,
          sort: "sortOrder_asc",
        }),
      );
      setModels(results);
      initialOrderRef.current = results.map((item) => item.id).join(",");
      setDirty(false);
      const nextGroups = buildModelOrderGroups(results);
      setSelectedGroupKey((current) =>
        nextGroups.some((group) => group.groupKey === current)
          ? current
          : nextGroups[0]?.groupKey ?? "",
      );
    } catch (error) {
      toast.error(t("loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [t, toastT]);

  React.useEffect(() => {
    if (!open) {
      return;
    }
    void loadModels();
  }, [loadModels, open]);

  const commitGroups = React.useCallback((nextGroups: ModelOrderGroup[]) => {
    const nextModels = flattenGroups(nextGroups);
    setModels(nextModels);
    setDirty(nextModels.map((item) => item.id).join(",") !== initialOrderRef.current);
  }, []);

  const handleSave = React.useCallback(async () => {
    if (!dirty || saving || models.length === 0) {
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
        return;
      }
      await reorderAdminLLMModels(token, { modelIDs: models.map((model) => model.id) });
      initialOrderRef.current = models.map((model) => model.id).join(",");
      setDirty(false);
      invalidateAdminReferenceDataCache();
      toast.success(t("saveSuccess"));
      onSaved();
      onClose();
    } catch (error) {
      toast.error(t("saveFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [dirty, models, onClose, onSaved, saving, t, toastT]);

  return {
    models,
    groups,
    selectedGroup,
    setSelectedGroupKey,
    loading,
    saving,
    dirty,
    commitGroups,
    handleSave,
  };
}
