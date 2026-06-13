import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  createAdminPromptPreset,
  deleteAdminPromptPreset,
  listAdminPromptPresets,
  updateAdminPromptPreset,
} from "@/shared/api/prompt-presets";
import type {
  PatchPromptPresetRequest,
  PromptPresetDTO,
  WritePromptPresetRequest,
} from "@/shared/api/prompt-presets.types";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { removeByID, replaceByID } from "@/shared/lib/optimistic-list";
import { PROMPT_PRESET_LIMITS, normalizePromptPresetName } from "@/shared/model/prompt-presets";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";

export type AdminPromptPresetForm = {
  id?: number;
  name: string;
  description: string;
  content: string;
  enabled: boolean;
};

const emptyForm: AdminPromptPresetForm = {
  name: "",
  description: "",
  content: "",
  enabled: true,
};

function formFromPromptPreset(item: PromptPresetDTO): AdminPromptPresetForm {
  return {
    id: item.id,
    name: item.trigger || item.title,
    description: item.description,
    content: item.content,
    enabled: item.enabled,
  };
}

function payloadFromForm(form: AdminPromptPresetForm): WritePromptPresetRequest {
  const name = normalizePromptPresetName(form.name);
  return {
    title: name,
    trigger: name,
    description: form.description.trim(),
    content: form.content.trim(),
    enabled: form.enabled,
    sortOrder: 0,
  };
}

function formExceedsLimits(form: AdminPromptPresetForm): boolean {
  return (
    normalizePromptPresetName(form.name).length > PROMPT_PRESET_LIMITS.name ||
    form.description.trim().length > PROMPT_PRESET_LIMITS.description ||
    form.content.trim().length > PROMPT_PRESET_LIMITS.content
  );
}

export function useAdminPromptPresets() {
  const t = useTranslations("adminPrompts");
  const { accessToken } = useAuthSession();
  const [items, setItems] = React.useState<PromptPresetDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSizeState] = React.useState(25);
  const [query, setQueryState] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [form, setForm] = React.useState<AdminPromptPresetForm>(emptyForm);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [deleteTarget, setDeleteTarget] = React.useState<PromptPresetDTO | null>(null);
  const [, startTableTransition] = React.useTransition();
  const requestSeqRef = React.useRef(0);

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedQuery(query.trim());
    }, 250);
    return () => window.clearTimeout(timer);
  }, [query]);

  const load = React.useCallback(async () => {
    const requestSeq = requestSeqRef.current + 1;
    requestSeqRef.current = requestSeq;
    setLoading(true);
    try {
      const data = await listAdminPromptPresets(accessToken, { page, pageSize, query: debouncedQuery });
      if (requestSeq !== requestSeqRef.current) {
        return;
      }
      startTableTransition(() => {
        setItems(data.results);
        setTotal(data.total);
      });
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      if (requestSeq === requestSeqRef.current) {
        setLoading(false);
      }
    }
  }, [accessToken, debouncedQuery, page, pageSize, startTableTransition, t]);

  React.useEffect(() => {
    void load();
  }, [load]);

  const pageCount = Math.max(1, Math.ceil(total / pageSize));

  const setQuery = React.useCallback((value: string) => {
    setQueryState(value);
    setPage(1);
  }, []);

  const setPageSize = React.useCallback((value: number) => {
    setPageSizeState(value);
    setPage(1);
  }, []);

  const openCreate = React.useCallback(() => {
    setForm(emptyForm);
    setDialogOpen(true);
  }, []);

  const openEdit = React.useCallback((item: PromptPresetDTO) => {
    setForm(formFromPromptPreset(item));
    setDialogOpen(true);
  }, []);

  const save = React.useCallback(async () => {
    const payload = payloadFromForm(form);
    if (!payload.title || !payload.trigger || !payload.content) {
      toast.error(t("toast.invalid"));
      return;
    }
    if (formExceedsLimits(form)) {
      toast.error(t("toast.tooLong"));
      return;
    }
    setSaving(true);
    try {
      if (form.id) {
        const updatePayload: PatchPromptPresetRequest = payload;
        await updateAdminPromptPreset(accessToken, form.id, updatePayload);
        await load();
        toast.success(t("toast.updated"));
      } else {
        await createAdminPromptPreset(accessToken, payload);
        await load();
        toast.success(t("toast.created"));
      }
      setDialogOpen(false);
    } catch (error) {
      toast.error(form.id ? t("toast.updateFailed") : t("toast.createFailed"), {
        description: resolveAdminErrorMessage(error),
      });
    } finally {
      setSaving(false);
    }
  }, [accessToken, form, load, t]);

  const toggleEnabled = React.useCallback(
    async (item: PromptPresetDTO, checked: boolean) => {
      setItems((current) => current.map((row) => (row.id === item.id ? { ...row, enabled: checked } : row)));
      try {
        await updateAdminPromptPreset(accessToken, item.id, { enabled: checked });
        await load();
      } catch (error) {
        setItems((current) => replaceByID(current, item.id, (row) => row.id, item));
        toast.error(t("toast.updateFailed"), { description: resolveAdminErrorMessage(error) });
      }
    },
    [accessToken, load, t],
  );

  const confirmDelete = React.useCallback(async () => {
    if (!deleteTarget) {
      return;
    }
    const target = deleteTarget;
    setDeleteTarget(null);
    try {
      await deleteAdminPromptPreset(accessToken, target.id);
      setItems((current) => removeByID(current, target.id, (item) => item.id));
      setTotal((current) => Math.max(0, current - 1));
      await load();
      toast.success(t("toast.deleted"));
    } catch (error) {
      toast.error(t("toast.deleteFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }, [accessToken, deleteTarget, load, t]);

  return {
    items,
    total,
    page,
    pageSize,
    pageCount,
    query,
    loading,
    saving,
    form,
    dialogOpen,
    deleteTarget,
    setPage,
    setPageSize,
    setQuery,
    setForm,
    setDialogOpen,
    setDeleteTarget,
    load,
    openCreate,
    openEdit,
    save,
    toggleEnabled,
    confirmDelete,
  };
}
