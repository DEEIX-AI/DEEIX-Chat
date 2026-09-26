"use client";

import * as React from "react";
import { toast } from "sonner";

import type {
  PatchUIComponentRequest,
  UIComponentDTO,
  UIComponentData,
  UIComponentDeleteData,
  UIComponentPage,
  WriteUIComponentRequest,
} from "@/shared/api/ui-components.types";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { useDebouncedValue } from "@/shared/hooks/use-debounced-value";
import { removeByID, replaceByID } from "@/shared/lib/optimistic-list";
import {
  EMPTY_UI_COMPONENT_FORM,
  type UIComponentFormValue,
  uiComponentFormFromDTO,
  uiComponentFormIsComplete,
  uiComponentFormIsWithinLimits,
  uiComponentPayloadFromForm,
  uiComponentSchemaIsValid,
} from "@/shared/model/ui-components";

export type UIComponentLibraryAPI = {
  list: (accessToken: string, options: { page: number; pageSize: number; query: string }, signal: AbortSignal) => Promise<UIComponentPage>;
  create: (accessToken: string, payload: WriteUIComponentRequest) => Promise<UIComponentData>;
  update: (accessToken: string, id: number, payload: PatchUIComponentRequest) => Promise<UIComponentData>;
  remove: (accessToken: string, id: number) => Promise<UIComponentDeleteData>;
};

export type UIComponentLibraryMessages = {
  loadFailed: string;
  invalid: string;
  invalidSchema: string;
  tooLong: string;
  created: string;
  updated: string;
  deleted: string;
  createFailed: string;
  updateFailed: string;
  deleteFailed: string;
};

// Shared list/edit/delete state for the admin and user component libraries;
// the two differ only in endpoints and copy.
export function useUIComponentLibrary(api: UIComponentLibraryAPI, messages: UIComponentLibraryMessages, resolveError: (error: unknown) => string) {
  const { accessToken } = useAuthSession();
  const [items, setItems] = React.useState<UIComponentDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSizeState] = React.useState(25);
  const [query, setQueryState] = React.useState("");
  const debouncedQuery = useDebouncedValue(query.trim());
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [form, setForm] = React.useState<UIComponentFormValue>(EMPTY_UI_COMPONENT_FORM);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [deleteTarget, setDeleteTarget] = React.useState<UIComponentDTO | null>(null);
  const [, startTableTransition] = React.useTransition();
  const requestSeqRef = React.useRef(0);
  const requestControllerRef = React.useRef<AbortController | null>(null);

  React.useEffect(
    () => () => {
      requestSeqRef.current += 1;
      requestControllerRef.current?.abort();
      requestControllerRef.current = null;
    },
    [],
  );

  const load = React.useCallback(async () => {
    const requestSeq = requestSeqRef.current + 1;
    requestSeqRef.current = requestSeq;
    requestControllerRef.current?.abort();
    const requestController = new AbortController();
    requestControllerRef.current = requestController;
    setLoading(true);
    try {
      const data = await api.list(accessToken, { page, pageSize, query: debouncedQuery }, requestController.signal);
      if (requestSeq !== requestSeqRef.current) {
        return;
      }
      startTableTransition(() => {
        setItems(data.results);
        setTotal(data.total);
      });
    } catch (error) {
      if (!requestController.signal.aborted) {
        toast.error(messages.loadFailed, { description: resolveError(error) });
      }
    } finally {
      if (requestControllerRef.current === requestController) {
        requestControllerRef.current = null;
      }
      if (!requestController.signal.aborted && requestSeq === requestSeqRef.current) {
        setLoading(false);
      }
    }
  }, [accessToken, api, debouncedQuery, messages.loadFailed, page, pageSize, resolveError, startTableTransition]);

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
    setForm(EMPTY_UI_COMPONENT_FORM);
    setDialogOpen(true);
  }, []);

  const openEdit = React.useCallback((item: UIComponentDTO) => {
    setForm(uiComponentFormFromDTO(item));
    setDialogOpen(true);
  }, []);

  const save = React.useCallback(async () => {
    if (!uiComponentFormIsComplete(form)) {
      toast.error(messages.invalid);
      return;
    }
    if (!uiComponentSchemaIsValid(form.propsSchema)) {
      toast.error(messages.invalidSchema);
      return;
    }
    if (!uiComponentFormIsWithinLimits(form)) {
      toast.error(messages.tooLong);
      return;
    }
    setSaving(true);
    try {
      if (form.id) {
        const payload: PatchUIComponentRequest =
          // Builtin catalog text is code-owned; only the switch is sent.
          form.scope === "builtin" ? { enabled: form.enabled } : uiComponentPayloadFromForm(form);
        await api.update(accessToken, form.id, payload);
        await load();
        toast.success(messages.updated);
      } else {
        await api.create(accessToken, uiComponentPayloadFromForm(form));
        await load();
        toast.success(messages.created);
      }
      setDialogOpen(false);
    } catch (error) {
      toast.error(form.id ? messages.updateFailed : messages.createFailed, { description: resolveError(error) });
    } finally {
      setSaving(false);
    }
  }, [accessToken, api, form, load, messages, resolveError]);

  const toggleEnabled = React.useCallback(
    async (item: UIComponentDTO, checked: boolean) => {
      setItems((current) => current.map((row) => (row.id === item.id ? { ...row, enabled: checked } : row)));
      try {
        await api.update(accessToken, item.id, { enabled: checked });
        await load();
      } catch (error) {
        setItems((current) => replaceByID(current, item.id, (row) => row.id, item));
        toast.error(messages.updateFailed, { description: resolveError(error) });
      }
    },
    [accessToken, api, load, messages.updateFailed, resolveError],
  );

  const confirmDelete = React.useCallback(async () => {
    if (!deleteTarget) {
      return;
    }
    const target = deleteTarget;
    setDeleteTarget(null);
    try {
      await api.remove(accessToken, target.id);
      setItems((current) => removeByID(current, target.id, (item) => item.id));
      const nextTotal = Math.max(0, total - 1);
      const nextPage = Math.min(page, Math.max(1, Math.ceil(nextTotal / pageSize)));
      setTotal(nextTotal);
      if (nextPage !== page) {
        setPage(nextPage);
      } else {
        await load();
      }
      toast.success(messages.deleted);
    } catch (error) {
      toast.error(messages.deleteFailed, { description: resolveError(error) });
    }
  }, [accessToken, api, deleteTarget, load, messages.deleteFailed, messages.deleted, page, pageSize, resolveError, total]);

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
