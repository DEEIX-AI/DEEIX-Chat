import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  createAdminSkill,
  deleteAdminSkill,
  listAdminSkills,
  updateAdminSkill,
} from "@/shared/api/skills";
import type {
  PatchSkillRequest,
  SkillDTO,
} from "@/shared/api/skills.types";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { removeByID, replaceByID } from "@/shared/lib/optimistic-list";
import {
  EMPTY_SKILL_FORM,
  skillFormFromDTO,
  skillFormIsWithinLimits,
  skillPayloadFromForm,
  skillPayloadIsComplete,
  type SkillFormValue,
} from "@/shared/model/skills";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";

export type AdminSkillForm = SkillFormValue;

export function useAdminSkills() {
  const t = useTranslations("adminPrompts");
  const { accessToken } = useAuthSession();
  const [items, setItems] = React.useState<SkillDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSizeState] = React.useState(25);
  const [query, setQueryState] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [form, setForm] = React.useState<AdminSkillForm>(EMPTY_SKILL_FORM);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [deleteTarget, setDeleteTarget] = React.useState<SkillDTO | null>(null);
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
      const data = await listAdminSkills(accessToken, { page, pageSize, query: debouncedQuery });
      if (requestSeq !== requestSeqRef.current) {
        return;
      }
      startTableTransition(() => {
        setItems(data.results);
        setTotal(data.total);
      });
    } catch (error) {
      toast.error(t("toast.skillsLoadFailed"), { description: resolveAdminErrorMessage(error) });
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
    setForm(EMPTY_SKILL_FORM);
    setDialogOpen(true);
  }, []);

  const openEdit = React.useCallback((item: SkillDTO) => {
    setForm(skillFormFromDTO(item));
    setDialogOpen(true);
  }, []);

  const save = React.useCallback(async () => {
    const payload = skillPayloadFromForm(form);
    if (!skillPayloadIsComplete(payload)) {
      toast.error(t("toast.skillInvalid"));
      return;
    }
    if (!skillFormIsWithinLimits(form)) {
      toast.error(t("toast.skillTooLong"));
      return;
    }
    setSaving(true);
    try {
      if (form.id) {
        const updatePayload: PatchSkillRequest = payload;
        await updateAdminSkill(accessToken, form.id, updatePayload);
        await load();
        toast.success(t("toast.skillUpdated"));
      } else {
        await createAdminSkill(accessToken, payload);
        await load();
        toast.success(t("toast.skillCreated"));
      }
      setDialogOpen(false);
    } catch (error) {
      toast.error(form.id ? t("toast.skillUpdateFailed") : t("toast.skillCreateFailed"), {
        description: resolveAdminErrorMessage(error),
      });
    } finally {
      setSaving(false);
    }
  }, [accessToken, form, load, t]);

  const toggleEnabled = React.useCallback(
    async (item: SkillDTO, checked: boolean) => {
      setItems((current) => current.map((row) => (row.id === item.id ? { ...row, enabled: checked } : row)));
      try {
        await updateAdminSkill(accessToken, item.id, { enabled: checked });
        await load();
      } catch (error) {
        setItems((current) => replaceByID(current, item.id, (row) => row.id, item));
        toast.error(t("toast.skillUpdateFailed"), { description: resolveAdminErrorMessage(error) });
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
      await deleteAdminSkill(accessToken, target.id);
      setItems((current) => removeByID(current, target.id, (item) => item.id));
      setTotal((current) => Math.max(0, current - 1));
      await load();
      toast.success(t("toast.skillDeleted"));
    } catch (error) {
      toast.error(t("toast.skillDeleteFailed"), { description: resolveAdminErrorMessage(error) });
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
