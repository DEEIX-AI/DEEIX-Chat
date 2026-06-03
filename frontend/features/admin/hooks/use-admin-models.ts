import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  listAdminLLMModelUpstreamSources,
  listAdminLLMModels,
  upsertAdminLLMUpstreamModel,
  updateAdminLLMModel,
} from "@/features/admin/api";
import type {
  AdminLLMAdapter,
  AdminBatchDeleteData,
  AdminLLMModelAccessScope,
  AdminLLMModelDTO,
  AdminLLMModelUpstreamSourceDTO,
  AdminLLMStatus,
} from "@/features/admin/api/llm.types";
import {
  PAGE_SIZE_DEFAULT,
  displayToKindsJson,
  resolveErrorMessage,
  type ModelSortValue,
} from "@/features/admin/types/llm";
import { resolveKindsDisplayForProtocols } from "@/features/admin/utils/llm-display";
import { patchByID, removeByID, removeManyByID, replaceByID } from "@/shared/lib/optimistic-list";

type UseAdminModelsState = {
  items: AdminLLMModelDTO[];
  total: number;
  page: number;
  pageSize: number;
  pageCount: number;
  loading: boolean;
  query: string;
  setQuery: (value: string) => void;
  statusFilter: string;
  setStatusFilter: (value: string) => void;
  vendorFilter: string;
  setVendorFilter: (value: string) => void;
  protocolFilter: string;
  setProtocolFilter: (value: string) => void;
  sortValue: ModelSortValue;
  setSortValue: (value: ModelSortValue) => void;
  filteredItems: AdminLLMModelDTO[];
  selectedModelIDs: Set<number>;
  setSelectedModelIDs: React.Dispatch<React.SetStateAction<Set<number>>>;
  selectedModels: AdminLLMModelDTO[];
  batchApplying: boolean;
  batchKindsDisplay: string;
  setBatchKindsDisplay: (value: string) => void;
  batchProtocol: AdminLLMAdapter | "";
  setBatchProtocol: (value: AdminLLMAdapter | "") => void;
  batchVendor: string;
  setBatchVendor: (value: string) => void;
  batchStatus: AdminLLMStatus | "";
  setBatchStatus: (value: AdminLLMStatus | "") => void;
  editTarget: AdminLLMModelDTO | null;
  setEditTarget: (target: AdminLLMModelDTO | null) => void;
  deleteTarget: AdminLLMModelDTO | null;
  setDeleteTarget: (target: AdminLLMModelDTO | null) => void;
  bulkDeleteTargets: AdminLLMModelDTO[];
  closeBulkDelete: () => void;
  sourcesModel: AdminLLMModelDTO | null;
  setSourcesModel: (target: AdminLLMModelDTO | null) => void;
  loadModels: (page?: number, pageSize?: number) => Promise<void>;
  handleToggleStatus: (item: AdminLLMModelDTO, nextStatus: AdminLLMStatus) => Promise<void>;
  handleToggleAccessScope: (item: AdminLLMModelDTO, nextScope: AdminLLMModelAccessScope) => Promise<void>;
  handleBulkApplyKinds: () => Promise<void>;
  handleBulkApplyProtocol: () => Promise<void>;
  handleBulkApplyVendor: () => Promise<void>;
  handleBulkApplyStatus: () => Promise<void>;
  handleSourceStatusChange: (modelID: number, previous: AdminLLMStatus, next: AdminLLMStatus) => void;
  handleSourceDeleteChange: (modelID: number, source: AdminLLMModelUpstreamSourceDTO, deleted: boolean) => void;
  handleRequestBulkDelete: () => void;
  handleDeleted: () => void;
  handleBulkDeleted: (result: AdminBatchDeleteData) => void;
};

export function useAdminModels(): UseAdminModelsState {
  const t = useTranslations("adminModels.toast");
  const [items, setItems] = React.useState<AdminLLMModelDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(PAGE_SIZE_DEFAULT);
  const [loading, setLoading] = React.useState(true);

  const [query, setQuery] = React.useState("");
  const [debouncedQuery, setDebouncedQuery] = React.useState("");
  const [statusFilter, setStatusFilter] = React.useState("");
  const [vendorFilter, setVendorFilter] = React.useState("");
  const [protocolFilter, setProtocolFilter] = React.useState("");
  const [sortValue, setSortValue] = React.useState<ModelSortValue>("sortOrder_asc");

  const [editTarget, setEditTarget] = React.useState<AdminLLMModelDTO | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<AdminLLMModelDTO | null>(null);
  const [bulkDeleteTargets, setBulkDeleteTargets] = React.useState<AdminLLMModelDTO[]>([]);
  const [selectedModelIDs, setSelectedModelIDs] = React.useState<Set<number>>(new Set());
  const [sourcesModel, setSourcesModel] = React.useState<AdminLLMModelDTO | null>(null);
  const [batchApplying, setBatchApplying] = React.useState(false);
  const [batchKindsDisplay, setBatchKindsDisplay] = React.useState("");
  const [batchProtocol, setBatchProtocol] = React.useState<AdminLLMAdapter | "">("");
  const [batchVendor, setBatchVendor] = React.useState("");
  const [batchStatus, setBatchStatus] = React.useState<AdminLLMStatus | "">("");
  const [, startTableTransition] = React.useTransition();
  const requestSeqRef = React.useRef(0);

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedQuery(query.trim());
    }, 250);
    return () => window.clearTimeout(timer);
  }, [query]);

  const loadModels = React.useCallback(
    async (nextPage = 1, nextPageSize = pageSize) => {
      const requestSeq = requestSeqRef.current + 1;
      requestSeqRef.current = requestSeq;
      setLoading(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("sessionExpired"), { description: t("signInAgain") });
          return;
        }
        const data = await listAdminLLMModels(token, {
          page: nextPage,
          pageSize: nextPageSize,
          onlyActive: false,
          query: debouncedQuery,
          status: statusFilter,
          vendor: vendorFilter,
          protocol: protocolFilter,
          sort: sortValue,
        });
        if (requestSeq !== requestSeqRef.current) {
          return;
        }
        startTableTransition(() => {
          setItems(data.results);
          setTotal(data.total);
          setPage(nextPage);
          setPageSize(nextPageSize);
          setSelectedModelIDs(new Set());
        });
      } catch (error) {
        toast.error(t("modelsLoadFailed"), { description: resolveErrorMessage(error) });
      } finally {
        if (requestSeq === requestSeqRef.current) {
          setLoading(false);
        }
      }
    },
    [debouncedQuery, pageSize, protocolFilter, sortValue, startTableTransition, statusFilter, t, vendorFilter],
  );

  React.useEffect(() => {
    void loadModels(1, pageSize);
  }, [loadModels, pageSize]);

  const pageCount = Math.max(1, Math.ceil(total / pageSize));

  const filteredItems = items;

  React.useEffect(() => {
    const visibleIDs = new Set(filteredItems.map((item) => item.id));
    setSelectedModelIDs((prev) => {
      const next = new Set<number>();
      prev.forEach((id) => {
        if (visibleIDs.has(id)) {
          next.add(id);
        }
      });
      return next.size === prev.size ? prev : next;
    });
  }, [filteredItems]);

  const selectedModels = React.useMemo(
    () => filteredItems.filter((item) => selectedModelIDs.has(item.id)),
    [filteredItems, selectedModelIDs],
  );

  const handleToggleStatus = React.useCallback(
    async (item: AdminLLMModelDTO, nextStatus: AdminLLMStatus) => {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("sessionExpired"), { description: t("signInAgain") });
        return;
      }
      const previousItem = items.find((model) => model.id === item.id) ?? item;
      setItems((current) => patchByID(current, item.id, (model) => model.id, { status: nextStatus }));
      try {
        const data = await updateAdminLLMModel(token, item.id, { status: nextStatus });
        const leavesCurrentStatusFilter = statusFilter !== "" && statusFilter !== nextStatus;
        if (leavesCurrentStatusFilter) {
          setItems((current) => removeByID(current, item.id, (model) => model.id));
          setTotal((current) => Math.max(0, current - 1));
        } else {
          setItems((current) => replaceByID(current, item.id, (model) => model.id, data.model));
        }
        toast.success(nextStatus === "active" ? t("modelEnabled") : t("modelDisabled"));
        if (statusFilter || sortValue === "updated_desc") {
          const nextPage = leavesCurrentStatusFilter && items.length === 1 && page > 1 ? page - 1 : page;
          void loadModels(nextPage, pageSize);
        }
      } catch (error) {
        setItems((current) => replaceByID(current, item.id, (model) => model.id, previousItem));
        toast.error(t("modelStatusUpdateFailed"), { description: resolveErrorMessage(error) });
      }
    },
    [items, loadModels, page, pageSize, sortValue, statusFilter, t],
  );

  const handleToggleAccessScope = React.useCallback(
    async (item: AdminLLMModelDTO, nextScope: AdminLLMModelAccessScope) => {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("sessionExpired"), { description: t("signInAgain") });
        return;
      }
      const previousItem = items.find((model) => model.id === item.id) ?? item;
      setItems((current) => patchByID(current, item.id, (model) => model.id, { accessScope: nextScope }));
      try {
        const data = await updateAdminLLMModel(token, item.id, { accessScope: nextScope });
        setItems((current) => replaceByID(current, item.id, (model) => model.id, data.model));
        toast.success(nextScope === "public" ? t("modelScopePublic") : t("modelScopeInternal"));
        if (sortValue === "updated_desc") {
          void loadModels(page, pageSize);
        }
      } catch (error) {
        setItems((current) => replaceByID(current, item.id, (model) => model.id, previousItem));
        toast.error(t("modelScopeUpdateFailed"), { description: resolveErrorMessage(error) });
      }
    },
    [items, loadModels, page, pageSize, sortValue, t],
  );

  const handleSourceStatusChange = React.useCallback((modelID: number, previous: AdminLLMStatus, next: AdminLLMStatus) => {
    if (previous === next) {
      return;
    }
    const delta = next === "active" ? 1 : -1;
    setItems((current) =>
      current.map((item) =>
        item.id === modelID
          ? {
              ...item,
              activeSourceCount: Math.max(0, item.activeSourceCount + delta),
            }
          : item,
      ),
    );
  }, []);

  const handleSourceDeleteChange = React.useCallback((modelID: number, source: AdminLLMModelUpstreamSourceDTO, deleted: boolean) => {
    const sourceDelta = deleted ? -1 : 1;
    const activeDelta = source.status === "active" ? sourceDelta : 0;
    setItems((current) =>
      current.map((item) =>
        item.id === modelID
          ? {
              ...item,
              sourceCount: Math.max(0, item.sourceCount + sourceDelta),
              activeSourceCount: Math.max(0, item.activeSourceCount + activeDelta),
            }
          : item,
      ),
    );
  }, []);

  const handleBulkApplyKinds = React.useCallback(async () => {
    const nextKindsJSON = displayToKindsJson(batchKindsDisplay);
    if (!selectedModels.length || !nextKindsJSON || batchApplying) {
      return;
    }

    const targets = selectedModels.filter((item) => item.kindsJSON !== nextKindsJSON);
    if (!targets.length) {
      toast.info(t("bulkKindsAlreadyApplied"));
      return;
    }

    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("sessionExpired"), { description: t("signInAgain") });
      return;
    }

    const rollbackModels = targets.map((item) => items.find((current) => current.id === item.id) ?? item);
    const targetIDs = new Set(targets.map((item) => item.id));
    setBatchApplying(true);
    setItems((current) =>
      current.map((item) => (targetIDs.has(item.id) ? { ...item, kindsJSON: nextKindsJSON } : item)),
    );
    try {
      const results = await Promise.allSettled(
        targets.map((item) => updateAdminLLMModel(token, item.id, { kindsJSON: nextKindsJSON })),
      );
      const failedModels = targets.filter((_, index) => results[index]?.status === "rejected");
      const successModels = targets.filter((_, index) => results[index]?.status === "fulfilled");
      const successResponses = results
        .filter((result): result is PromiseFulfilledResult<{ model: AdminLLMModelDTO }> => result.status === "fulfilled")
        .map((result) => result.value.model);

      setItems((current) =>
        successResponses.reduce((next, model) => replaceByID(next, model.id, (item) => item.id, model), current),
      );
      if (failedModels.length > 0) {
        const failedIDs = new Set(failedModels.map((item) => item.id));
        setItems((current) =>
          rollbackModels.reduce(
            (next, model) => (failedIDs.has(model.id) ? replaceByID(next, model.id, (item) => item.id, model) : next),
            current,
          ),
        );
        setSelectedModelIDs(new Set(failedModels.map((item) => item.id)));
        toast.error(t("bulkKindsPartialFailed"), {
          description: t("bulkPartialDescription", { success: successModels.length, failed: failedModels.length }),
        });
        return;
      }

      toast.success(t("bulkKindsUpdated", { count: targets.length }));
      setSelectedModelIDs(new Set());
      setBatchKindsDisplay("");
    } catch (error) {
      setItems((current) =>
        rollbackModels.reduce((next, model) => replaceByID(next, model.id, (item) => item.id, model), current),
      );
      toast.error(t("bulkKindsFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setBatchApplying(false);
    }
  }, [batchApplying, batchKindsDisplay, items, selectedModels, t]);

  const handleBulkApplyVendor = React.useCallback(async () => {
    const nextVendor = batchVendor.trim();
    if (!selectedModels.length || !nextVendor || batchApplying) {
      return;
    }

    const targets = selectedModels.filter((item) => item.vendor !== nextVendor);
    if (!targets.length) {
      toast.info(t("bulkVendorAlreadyApplied"));
      return;
    }

    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("sessionExpired"), { description: t("signInAgain") });
      return;
    }

    const rollbackModels = targets.map((item) => items.find((current) => current.id === item.id) ?? item);
    const targetIDs = new Set(targets.map((item) => item.id));
    setBatchApplying(true);
    setItems((current) =>
      current.map((item) => (targetIDs.has(item.id) ? { ...item, vendor: nextVendor } : item)),
    );
    try {
      const results = await Promise.allSettled(
        targets.map((item) => updateAdminLLMModel(token, item.id, { vendor: nextVendor })),
      );
      const failedModels = targets.filter((_, index) => results[index]?.status === "rejected");
      const successModels = targets.filter((_, index) => results[index]?.status === "fulfilled");
      const successResponses = results
        .filter((result): result is PromiseFulfilledResult<{ model: AdminLLMModelDTO }> => result.status === "fulfilled")
        .map((result) => result.value.model);

      setItems((current) =>
        successResponses.reduce((next, model) => replaceByID(next, model.id, (item) => item.id, model), current),
      );
      if (failedModels.length > 0) {
        const failedIDs = new Set(failedModels.map((item) => item.id));
        setItems((current) =>
          rollbackModels.reduce(
            (next, model) => (failedIDs.has(model.id) ? replaceByID(next, model.id, (item) => item.id, model) : next),
            current,
          ),
        );
        setSelectedModelIDs(new Set(failedModels.map((item) => item.id)));
        toast.error(t("bulkVendorPartialFailed"), {
          description: t("bulkPartialDescription", { success: successModels.length, failed: failedModels.length }),
        });
        return;
      }

      toast.success(t("bulkVendorUpdated", { count: targets.length }));
      setSelectedModelIDs(new Set());
      setBatchVendor("");
    } catch (error) {
      setItems((current) =>
        rollbackModels.reduce((next, model) => replaceByID(next, model.id, (item) => item.id, model), current),
      );
      toast.error(t("bulkVendorFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setBatchApplying(false);
    }
  }, [batchApplying, batchVendor, items, selectedModels, t]);

  const handleBulkApplyStatus = React.useCallback(async () => {
    const nextStatus = batchStatus;
    if (!selectedModels.length || !nextStatus || batchApplying) {
      return;
    }

    const targets = selectedModels.filter((item) => item.status !== nextStatus);
    if (!targets.length) {
      toast.info(t("bulkStatusAlreadyApplied"));
      return;
    }

    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("sessionExpired"), { description: t("signInAgain") });
      return;
    }

    const rollbackModels = targets.map((item) => items.find((current) => current.id === item.id) ?? item);
    const targetIDs = new Set(targets.map((item) => item.id));
    setBatchApplying(true);
    setItems((current) =>
      current.map((item) => (targetIDs.has(item.id) ? { ...item, status: nextStatus } : item)),
    );
    try {
      const results = await Promise.allSettled(
        targets.map((item) => updateAdminLLMModel(token, item.id, { status: nextStatus })),
      );
      const failedModels = targets.filter((_, index) => results[index]?.status === "rejected");
      const successModels = targets.filter((_, index) => results[index]?.status === "fulfilled");
      const successResponses = results
        .filter((result): result is PromiseFulfilledResult<{ model: AdminLLMModelDTO }> => result.status === "fulfilled")
        .map((result) => result.value.model);

      setItems((current) =>
        successResponses.reduce((next, model) => replaceByID(next, model.id, (item) => item.id, model), current),
      );
      if (failedModels.length > 0) {
        const failedIDs = new Set(failedModels.map((item) => item.id));
        setItems((current) =>
          rollbackModels.reduce(
            (next, model) => (failedIDs.has(model.id) ? replaceByID(next, model.id, (item) => item.id, model) : next),
            current,
          ),
        );
        setSelectedModelIDs(new Set(failedModels.map((item) => item.id)));
        toast.error(t("bulkStatusPartialFailed"), {
          description: t("bulkPartialDescription", { success: successModels.length, failed: failedModels.length }),
        });
        return;
      }

      toast.success(t("bulkStatusUpdated", { count: targets.length }));
      setSelectedModelIDs(new Set());
      setBatchStatus("");
    } catch (error) {
      setItems((current) =>
        rollbackModels.reduce((next, model) => replaceByID(next, model.id, (item) => item.id, model), current),
      );
      toast.error(t("bulkStatusFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setBatchApplying(false);
    }
  }, [batchApplying, batchStatus, items, selectedModels, t]);

  const handleBulkApplyProtocol = React.useCallback(async () => {
    const nextProtocol = batchProtocol;
    if (!selectedModels.length || !nextProtocol || batchApplying) {
      return;
    }

    const targets = selectedModels.filter((item) => item.sourceCount > 0);
    if (!targets.length) {
      toast.info(t("bulkProtocolNoSources"));
      return;
    }

    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("sessionExpired"), { description: t("signInAgain") });
      return;
    }

    const rollbackModels = targets.map((item) => items.find((current) => current.id === item.id) ?? item);
    const targetIDs = new Set(targets.map((item) => item.id));
    const nextProtocolsJSON = JSON.stringify([nextProtocol]);
    const nextKindsJSON = displayToKindsJson(resolveKindsDisplayForProtocols([nextProtocol]));
    setBatchApplying(true);
    setItems((current) =>
      current.map((item) => (targetIDs.has(item.id) ? { ...item, protocolsJSON: nextProtocolsJSON, kindsJSON: nextKindsJSON } : item)),
    );
    try {
      const results = await Promise.allSettled(
        targets.map(async (model) => {
          const sources = await listAdminLLMModelUpstreamSources(token, model.id, { page: 1, pageSize: 2000 });
          if (sources.results.length === 0) {
            throw new Error("model upstream sources not found");
          }
          for (const source of sources.results) {
            await upsertAdminLLMUpstreamModel(token, source.upstreamID, {
              routeID: source.id,
              platformModelName: model.platformModelName,
              upstreamModelName: source.upstreamModelName,
              protocol: nextProtocol,
              kindsJSON: nextKindsJSON,
              status: source.status,
              priority: source.priority,
              weight: source.weight,
            });
          }
          return { ...model, kindsJSON: nextKindsJSON, protocolsJSON: nextProtocolsJSON };
        }),
      );
      const failedModels = targets.filter((_, index) => results[index]?.status === "rejected");
      const successModels = targets.filter((_, index) => results[index]?.status === "fulfilled");
      const successResponses = results
        .filter((result): result is PromiseFulfilledResult<AdminLLMModelDTO> => result.status === "fulfilled")
        .map((result) => result.value);
      setItems((current) =>
        successResponses.reduce((next, model) => replaceByID(next, model.id, (item) => item.id, model), current),
      );
      if (failedModels.length > 0) {
        const failedIDs = new Set(failedModels.map((item) => item.id));
        setItems((current) =>
          rollbackModels.reduce(
            (next, model) => (failedIDs.has(model.id) ? replaceByID(next, model.id, (item) => item.id, model) : next),
            current,
          ),
        );
        setSelectedModelIDs(new Set(failedModels.map((item) => item.id)));
        toast.error(t("bulkProtocolPartialFailed"), {
          description: t("bulkPartialDescription", { success: successModels.length, failed: failedModels.length }),
        });
        return;
      }

      toast.success(t("bulkProtocolUpdated", { count: targets.length }));
      setSelectedModelIDs(new Set());
      setBatchProtocol("");
    } catch (error) {
      setItems((current) =>
        rollbackModels.reduce((next, model) => replaceByID(next, model.id, (item) => item.id, model), current),
      );
      toast.error(t("bulkProtocolFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setBatchApplying(false);
    }
  }, [batchApplying, batchProtocol, items, selectedModels, t]);

  const handleRequestBulkDelete = React.useCallback(() => {
    if (selectedModels.length === 0) {
      return;
    }
    setBulkDeleteTargets(selectedModels);
  }, [selectedModels]);

  function closeBulkDelete() {
    setBulkDeleteTargets([]);
  }

  function handleDeleted() {
    if (deleteTarget) {
      setItems((current) => removeByID(current, deleteTarget.id, (item) => item.id));
      setTotal((current) => Math.max(0, current - 1));
      if (items.length === 1 && page > 1) {
        void loadModels(page - 1, pageSize);
      }
    }
    setDeleteTarget(null);
    setSelectedModelIDs((prev) => {
      if (!deleteTarget) {
        return prev;
      }
      const next = new Set(prev);
      next.delete(deleteTarget.id);
      return next;
    });
  }

  function handleBulkDeleted(result: AdminBatchDeleteData) {
    const removedIDs = result.results
      .filter((item) => item.status === "deleted" || item.status === "not_found")
      .map((item) => item.id);
    setBulkDeleteTargets([]);
    setItems((current) => removeManyByID(current, removedIDs, (item) => item.id));
    setTotal((current) => Math.max(0, current - removedIDs.length));
    setSelectedModelIDs((current) => {
      const removed = new Set(removedIDs);
      return new Set([...current].filter((id) => !removed.has(id)));
    });
    if (removedIDs.length >= items.length && page > 1) {
      void loadModels(page - 1, pageSize);
    }
  }

  return {
    items,
    total,
    page,
    pageSize,
    pageCount,
    loading,
    query,
    setQuery,
    statusFilter,
    setStatusFilter,
    vendorFilter,
    setVendorFilter,
    protocolFilter,
    setProtocolFilter,
    sortValue,
    setSortValue,
    filteredItems,
    selectedModelIDs,
    setSelectedModelIDs,
    selectedModels,
    batchApplying,
    batchKindsDisplay,
    setBatchKindsDisplay,
    batchProtocol,
    setBatchProtocol,
    batchVendor,
    setBatchVendor,
    batchStatus,
    setBatchStatus,
    editTarget,
    setEditTarget,
    deleteTarget,
    setDeleteTarget,
    bulkDeleteTargets,
    closeBulkDelete,
    sourcesModel,
    setSourcesModel,
    loadModels,
    handleToggleStatus,
    handleToggleAccessScope,
    handleBulkApplyKinds,
    handleBulkApplyProtocol,
    handleBulkApplyVendor,
    handleBulkApplyStatus,
    handleSourceStatusChange,
    handleSourceDeleteChange,
    handleRequestBulkDelete,
    handleDeleted,
    handleBulkDeleted,
  };
}
