import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  createAdminAnnouncement,
  deleteAdminAnnouncement,
  listAdminAnnouncements,
  updateAdminAnnouncement,
} from "@/features/admin/api";
import type {
  AdminAnnouncementDTO,
  CreateAdminAnnouncementRequest,
  UpdateAdminAnnouncementRequest,
} from "@/features/admin/api/announcements-types";
import { normalizeAdminAnnouncementType } from "@/features/admin/model/announcements";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { useAuthSession } from "@/shared/auth/auth-session-context";

/**
 * Announcement list with filters plus create/update/delete. Inline row edits
 * (status, pinned, type, priority) are optimistic and roll the row back to
 * its previous value when the request fails.
 */
export function useAdminAnnouncements() {
  const t = useTranslations("adminAnnouncements");
  const { accessToken } = useAuthSession();
  const [items, setItems] = React.useState<AdminAnnouncementDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(25);
  const [query, setQuery] = React.useState("");
  const [status, setStatus] = React.useState("");
  const [typeFilter, setTypeFilter] = React.useState("");
  const [pinnedFilter, setPinnedFilter] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const data = await listAdminAnnouncements(accessToken, { page, pageSize, query, status, type: typeFilter, pinned: pinnedFilter });
      setItems(data.results);
      setTotal(data.total);
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [accessToken, page, pageSize, pinnedFilter, query, status, t, typeFilter]);

  React.useEffect(() => {
    void load();
  }, [load]);

  // `onSuccess` runs in the same tick as the success toast so the dialog
  // closes together with `saving` resetting, as before the hook extraction.
  async function saveAnnouncement(
    { id, payload }: { id?: number; payload: CreateAdminAnnouncementRequest },
    onSuccess: () => void,
  ) {
    if (!payload.title || !payload.contentMarkdown) {
      toast.error(t("toast.invalid"));
      return;
    }
    setSaving(true);
    try {
      if (id) {
        const updatePayload: UpdateAdminAnnouncementRequest = payload;
        const data = await updateAdminAnnouncement(accessToken, id, updatePayload);
        setItems((current) => current.map((item) => item.id === data.announcement.id ? data.announcement : item));
        toast.success(t("toast.updated"));
      } else {
        const data = await createAdminAnnouncement(accessToken, payload);
        setItems((current) => [data.announcement, ...current].slice(0, pageSize));
        setTotal((current) => current + 1);
        toast.success(t("toast.created"));
      }
      onSuccess();
    } catch (error) {
      toast.error(id ? t("toast.updateFailed") : t("toast.createFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  async function toggleStatus(item: AdminAnnouncementDTO, checked: boolean) {
    const nextStatus = checked ? "active" : "inactive";
    setItems((current) => current.map((row) => row.id === item.id ? { ...row, status: nextStatus } : row));
    try {
      const data = await updateAdminAnnouncement(accessToken, item.id, { status: nextStatus });
      setItems((current) => current.map((row) => row.id === item.id ? data.announcement : row));
    } catch (error) {
      setItems((current) => current.map((row) => row.id === item.id ? item : row));
      toast.error(t("toast.statusFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }

  async function togglePinned(item: AdminAnnouncementDTO, checked: boolean) {
    setItems((current) => current.map((row) => row.id === item.id ? { ...row, pinned: checked } : row));
    try {
      const data = await updateAdminAnnouncement(accessToken, item.id, { pinned: checked });
      setItems((current) => current.map((row) => row.id === item.id ? data.announcement : row));
    } catch (error) {
      setItems((current) => current.map((row) => row.id === item.id ? item : row));
      toast.error(t("toast.updateFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }

  async function updateType(item: AdminAnnouncementDTO, value: string) {
    const nextType = normalizeAdminAnnouncementType(value);
    if (nextType === normalizeAdminAnnouncementType(item.type)) {
      return;
    }
    setItems((current) => current.map((row) => row.id === item.id ? { ...row, type: nextType } : row));
    try {
      const data = await updateAdminAnnouncement(accessToken, item.id, { type: nextType });
      setItems((current) => current.map((row) => row.id === item.id ? data.announcement : row));
    } catch (error) {
      setItems((current) => current.map((row) => row.id === item.id ? item : row));
      toast.error(t("toast.updateFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }

  async function updatePriority(item: AdminAnnouncementDTO, nextPriority: number) {
    setItems((current) => current.map((row) => row.id === item.id ? { ...row, priority: nextPriority } : row));
    try {
      const data = await updateAdminAnnouncement(accessToken, item.id, { priority: nextPriority });
      setItems((current) => current.map((row) => row.id === item.id ? data.announcement : row));
    } catch (error) {
      setItems((current) => current.map((row) => row.id === item.id ? item : row));
      toast.error(t("toast.updateFailed"), { description: resolveAdminErrorMessage(error) });
    }
  }

  function notifyInvalidPriority() {
    toast.error(t("toast.priorityInvalid"));
  }

  async function deleteAnnouncement(target: AdminAnnouncementDTO, onSuccess: () => void) {
    setSaving(true);
    try {
      await deleteAdminAnnouncement(accessToken, target.id);
      setItems((current) => current.filter((item) => item.id !== target.id));
      setTotal((current) => Math.max(0, current - 1));
      onSuccess();
      toast.success(t("toast.deleted"));
    } catch (error) {
      toast.error(t("toast.deleteFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }

  return {
    items,
    total,
    page,
    setPage,
    pageSize,
    setPageSize,
    query,
    setQuery,
    status,
    setStatus,
    typeFilter,
    setTypeFilter,
    pinnedFilter,
    setPinnedFilter,
    loading,
    saving,
    load,
    saveAnnouncement,
    toggleStatus,
    togglePinned,
    updateType,
    updatePriority,
    notifyInvalidPriority,
    deleteAnnouncement,
  };
}
