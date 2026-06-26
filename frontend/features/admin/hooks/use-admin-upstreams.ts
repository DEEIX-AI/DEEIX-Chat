import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  listAdminLLMUpstreams,
  updateAdminLLMUpstream,
} from "@/features/admin/api";
import type { AdminBatchDeleteData, AdminLLMStatus, AdminLLMUpstreamView } from "@/features/admin/api/llm.types";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { replaceByID } from "@/shared/lib/optimistic-list";
import { runSettledBulkItems } from "@/shared/lib/bulk-action";

export const UPSTREAM_SORT_OPTIONS = [
  { labelKey: "sort.idDesc", value: "id_desc" },
  { labelKey: "sort.idAsc", value: "id_asc" },
  { labelKey: "sort.nameAsc", value: "name_asc" },
  { labelKey: "sort.updatedDesc", value: "updated_desc" },
] as const;

export type UpstreamSortValue = (typeof UPSTREAM_SORT_OPTIONS)[number]["value"];

type UpstreamSheetState =
  | { open: false }
  | { open: true; mode: "create" }
  | { open: true; mode: "edit"; target: AdminLLMUpstreamView };

type UpstreamCircuitState =
  | { open: false }
  | { open: true; target: AdminLLMUpstreamView; action: "open" | "reset" };

type UseAdminUpstreamsState = {
  loading: boolean;
  total: number;
  query: string;
  setQuery: (value: string) => void;
  statusFilter: string;
  setStatusFilter: (value: string) => void;
  compatibleFilter: string;
  setCompatibleFilter: (value: string) => void;
  sortValue: UpstreamSortValue;
  setSortValue: (value: UpstreamSortValue) => void;
  page: number;
  setPage: (value: number) => void;
  pageSize: number;
  setPageSize: (value: number) => void;
  pageCount: number;
  safePage: number;
  filteredItems: AdminLLMUpstreamView[];
  pagedItems: AdminLLMUpstreamView[];
  selected: Set<number>;
  togglingStatusIDs: Set<number>;
  selectedUpstreams: AdminLLMUpstreamView[];
  batchApplying: boolean;
  batchStatus: AdminLLMStatus | "";
  setBatchStatus: (value: AdminLLMStatus | "") => void;
  sheetState: UpstreamSheetState;
  deleteTarget: AdminLLMUpstreamView | null;
  bulkDeleteTargets: AdminLLMUpstreamView[];
  circuitState: UpstreamCircuitState;
  modelsTarget: AdminLLMUpstreamView | null;
  modelsOpen: boolean;
  setModelsOpen: (open: boolean) => void;
  load: () => Promise<void>;
  handleSelectAll: (checked: boolean) => void;
  handleSelectOne: (id: number, checked: boolean) => void;
  handleOpenCreate: () => void;
  closeSheet: () => void;
  handleEdit: (item: AdminLLMUpstreamView) => void;
  handleManageModels: (item: AdminLLMUpstreamView) => void;
  closeModels: () => void;
  handleCircuitAction: (
    item: AdminLLMUpstreamView,
    action: "open" | "reset",
  ) => void;
  closeCircuit: () => void;
  handleToggleStatus: (item: AdminLLMUpstreamView) => Promise<void>;
  handleBulkApplyStatus: () => Promise<void>;
  handleDelete: (item: AdminLLMUpstreamView) => void;
  closeDelete: () => void;
  handleRequestBulkDelete: () => void;
  closeBulkDelete: () => void;
  handleSheetSuccess: (item: AdminLLMUpstreamView) => void;
  handleDeleted: (id: number) => void;
  handleBulkDeleted: (result: AdminBatchDeleteData) => void;
  handleCircuitDone: (updated: AdminLLMUpstreamView) => void;
  handleUpstreamUpdated: (updated: AdminLLMUpstreamView) => void;
};

function mergeUpstreamListView(current: AdminLLMUpstreamView, updated: AdminLLMUpstreamView): AdminLLMUpstreamView {
  return {
    ...current,
    ...updated,
  };
}

function normalizeUpstreamListAvailability(item: AdminLLMUpstreamView): AdminLLMUpstreamView {
  if (item.status === "active" && !item.circuitOpen) {
    return item;
  }
  return {
    ...item,
    activeModelsCount: 0,
  };
}

export function useAdminUpstreams(): UseAdminUpstreamsState {
  const t = useTranslations("adminUpstreams.toast");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [items, setItems] = React.useState<AdminLLMUpstreamView[]>([]);
  const [total, setTotal] = React.useState(0);
  const [loading, setLoading] = React.useState(true);
  const [selected, setSelected] = React.useState<Set<number>>(new Set());
  const [togglingStatusIDs, setTogglingStatusIDs] = React.useState<Set<number>>(new Set());
  const [batchApplying, setBatchApplying] = React.useState(false);
  const [batchStatus, setBatchStatus] = React.useState<AdminLLMStatus | "">("");

  const [query, setQuery] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [statusFilter, setStatusFilter] = React.useState("");
  const [compatibleFilter, setCompatibleFilter] = React.useState("");
  const [sortValue, setSortValue] = React.useState<UpstreamSortValue>("id_desc");
  const [, startTableTransition] = React.useTransition();

  const [page, setPageState] = React.useState(1);
  const [pageSize, setPageSizeState] = React.useState(25);

  const [sheetState, setSheetState] = React.useState<UpstreamSheetState>({ open: false });
  const [deleteTarget, setDeleteTarget] = React.useState<AdminLLMUpstreamView | null>(null);
  const [bulkDeleteTargets, setBulkDeleteTargets] = React.useState<AdminLLMUpstreamView[]>([]);
  const [circuitState, setCircuitState] = React.useState<UpstreamCircuitState>({ open: false });
  const [modelsTarget, setModelsTarget] = React.useState<AdminLLMUpstreamView | null>(null);
  const [modelsOpen, setModelsOpen] = React.useState(false);
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
      const token = await resolveAccessToken();
      const result = await listAdminLLMUpstreams(token, {
        page,
        pageSize,
        query: debouncedQuery,
        status: statusFilter,
        compatible: compatibleFilter,
        sort: sortValue,
      });
      if (requestSeq !== requestSeqRef.current) {
        return;
      }
      startTableTransition(() => {
        setItems(result.results);
        setTotal(result.total);
        setSelected(new Set());
      });
    } catch (error) {
      toast.error(t("upstreamsLoadFailed"), { description: resolveErrorMessage(error) });
    } finally {
      if (requestSeq === requestSeqRef.current) {
        setLoading(false);
      }
    }
  }, [compatibleFilter, debouncedQuery, page, pageSize, resolveErrorMessage, sortValue, startTableTransition, statusFilter, t]);

  React.useEffect(() => {
    void load();
  }, [load]);

  const filteredItems = items;

  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const safePage = Math.min(page, pageCount);
  const pagedItems = filteredItems;

  const setPage = React.useCallback((value: number) => {
    setPageState(value);
  }, []);

  const setPageSize = React.useCallback((value: number) => {
    setPageSizeState(value);
    setPageState(1);
  }, []);

  React.useEffect(() => {
    setPageState(1);
  }, [compatibleFilter, debouncedQuery, sortValue, statusFilter]);

  const selectedUpstreams = React.useMemo(
    () => filteredItems.filter((item) => selected.has(item.id)),
    [filteredItems, selected],
  );

  function handleSelectAll(checked: boolean) {
    setSelected(checked ? new Set(pagedItems.map((item) => item.id)) : new Set());
  }

  function handleSelectOne(id: number, checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(id);
      } else {
        next.delete(id);
      }
      return next;
    });
  }

  function handleOpenCreate() {
    setSheetState({ open: true, mode: "create" });
  }

  function closeSheet() {
    setSheetState({ open: false });
  }

  function handleEdit(item: AdminLLMUpstreamView) {
    setSheetState({ open: true, mode: "edit", target: item });
  }

  function handleManageModels(item: AdminLLMUpstreamView) {
    setModelsTarget(item);
    setModelsOpen(true);
  }

  function closeModels() {
    setModelsOpen(false);
    setModelsTarget(null);
  }

  function handleCircuitAction(
    item: AdminLLMUpstreamView,
    action: "open" | "reset",
  ) {
    setCircuitState({ open: true, target: item, action });
  }

  function closeCircuit() {
    setCircuitState({ open: false });
  }

  async function handleToggleStatus(item: AdminLLMUpstreamView) {
    if (togglingStatusIDs.has(item.id)) {
      return;
    }
    const newStatus = item.status === "active" ? "inactive" : "active";
    const previousItem = items.find((current) => current.id === item.id) ?? item;
    setTogglingStatusIDs((prev) => new Set(prev).add(item.id));
    setItems((prev) =>
      prev.map((current) =>
        current.id === item.id
          ? normalizeUpstreamListAvailability({ ...current, status: newStatus })
          : current,
      ),
    );
    try {
      const token = await resolveAccessToken();
      const data = await updateAdminLLMUpstream(token, item.id, {
        status: newStatus,
      });
      setItems((prev) => prev.map((current) => (current.id === item.id ? mergeUpstreamListView(current, data.upstream) : current)));
      toast.success(newStatus === "active" ? t("upstreamEnabled") : t("upstreamDisabled"));
    } catch (error) {
      setItems((prev) => replaceByID(prev, item.id, (current) => current.id, previousItem));
      toast.error(t("operationFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setTogglingStatusIDs((prev) => {
        const next = new Set(prev);
        next.delete(item.id);
        return next;
      });
    }
  }

  async function handleBulkApplyStatus() {
    const nextStatus = batchStatus;
    if (!selectedUpstreams.length || !nextStatus || batchApplying) {
      return;
    }

    const targets = selectedUpstreams.filter((item) => item.status !== nextStatus);
    if (!targets.length) {
      toast.info(t("bulkStatusAlreadyApplied"));
      return;
    }

    const token = await resolveAccessToken();
    const rollbackUpstreams = targets.map((item) => items.find((current) => current.id === item.id) ?? item);
    const targetIDs = new Set(targets.map((item) => item.id));
    setBatchApplying(true);
    setItems((prev) =>
      prev.map((item) =>
        targetIDs.has(item.id)
          ? normalizeUpstreamListAvailability({ ...item, status: nextStatus })
          : item,
      ),
    );
    try {
      const results = await runSettledBulkItems({
        items: targets,
        title: t("bulkStatusUpdated", { count: targets.length }),
        runItem: (item) => updateAdminLLMUpstream(token, item.id, { status: nextStatus }),
      });
      const failedUpstreams = results.filter((result) => result.status === "rejected").map((result) => result.item);
      const successUpstreams = results.filter((result) => result.status === "fulfilled").map((result) => result.item);
      const successResponses = results
        .filter((result): result is Extract<typeof result, { status: "fulfilled" }> => result.status === "fulfilled")
        .map((result) => result.value.upstream);

      setItems((prev) =>
        successResponses.reduce(
          (next, upstream) => next.map((item) => (item.id === upstream.id ? mergeUpstreamListView(item, upstream) : item)),
          prev,
        ),
      );
      if (failedUpstreams.length > 0) {
        const failedIDs = new Set(failedUpstreams.map((item) => item.id));
        setItems((prev) =>
          rollbackUpstreams.reduce(
            (next, upstream) => (failedIDs.has(upstream.id) ? replaceByID(next, upstream.id, (item) => item.id, upstream) : next),
            prev,
          ),
        );
        setSelected(new Set(failedUpstreams.map((item) => item.id)));
        toast.error(t("bulkStatusPartialFailed"), {
          description: t("bulkPartialDescription", { success: successUpstreams.length, failed: failedUpstreams.length }),
        });
        return;
      }

      toast.success(t("bulkStatusUpdated", { count: targets.length }));
      setSelected(new Set());
      setBatchStatus("");
    } catch (error) {
      setItems((prev) =>
        rollbackUpstreams.reduce((next, upstream) => replaceByID(next, upstream.id, (item) => item.id, upstream), prev),
      );
      toast.error(t("bulkStatusFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setBatchApplying(false);
    }
  }

  function handleDelete(item: AdminLLMUpstreamView) {
    setDeleteTarget(item);
  }

  function closeDelete() {
    setDeleteTarget(null);
  }

  function handleRequestBulkDelete() {
    if (selectedUpstreams.length === 0) {
      return;
    }
    setBulkDeleteTargets(selectedUpstreams);
  }

  function closeBulkDelete() {
    setBulkDeleteTargets([]);
  }

  function handleSheetSuccess(item: AdminLLMUpstreamView) {
    setItems((prev) => {
      const index = prev.findIndex((current) => current.id === item.id);
      if (index >= 0) {
        const next = [...prev];
        next[index] = mergeUpstreamListView(next[index], item);
        return next;
      }
      setTotal((current) => current + 1);
      return [item, ...prev];
    });
  }

  function handleDeleted(id: number) {
    setItems((prev) => prev.filter((item) => item.id !== id));
    setTotal((current) => Math.max(0, current - 1));
    setSelected((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  }

  function handleBulkDeleted(result: AdminBatchDeleteData) {
    const removedIDs = new Set(
      result.results
        .filter((item) => item.status === "deleted" || item.status === "not_found")
        .map((item) => item.id),
    );
    setItems((prev) => prev.filter((item) => !removedIDs.has(item.id)));
    setTotal((prev) => Math.max(0, prev - removedIDs.size));
    setSelected((prev) => {
      const next = new Set(prev);
      removedIDs.forEach((id) => next.delete(id));
      return next;
    });
    setBulkDeleteTargets([]);
  }

  function handleCircuitDone(updated: AdminLLMUpstreamView) {
    setItems((prev) =>
      prev.map((item) => (item.id === updated.id ? normalizeUpstreamListAvailability(mergeUpstreamListView(item, updated)) : item)),
    );
    void load();
  }

  function handleUpstreamUpdated(updated: AdminLLMUpstreamView) {
    setItems((prev) =>
      prev.map((item) => (item.id === updated.id ? mergeUpstreamListView(item, updated) : item)),
    );
    if (modelsTarget?.id === updated.id) {
      setModelsTarget(updated);
    }
    void load();
  }

  return {
    loading,
    total,
    query,
    setQuery,
    statusFilter,
    setStatusFilter,
    compatibleFilter,
    setCompatibleFilter,
    sortValue,
    setSortValue,
    page,
    setPage,
    pageSize,
    setPageSize,
    pageCount,
    safePage,
    filteredItems,
    pagedItems,
    selected,
    togglingStatusIDs,
    selectedUpstreams,
    batchApplying,
    batchStatus,
    setBatchStatus,
    sheetState,
    deleteTarget,
    bulkDeleteTargets,
    circuitState,
    modelsTarget,
    modelsOpen,
    setModelsOpen,
    load,
    handleSelectAll,
    handleSelectOne,
    handleOpenCreate,
    closeSheet,
    handleEdit,
    handleManageModels,
    closeModels,
    handleCircuitAction,
    closeCircuit,
    handleToggleStatus,
    handleBulkApplyStatus,
    handleDelete,
    closeDelete,
    handleRequestBulkDelete,
    closeBulkDelete,
    handleSheetSuccess,
    handleDeleted,
    handleBulkDeleted,
    handleCircuitDone,
    handleUpstreamUpdated,
  };
}
