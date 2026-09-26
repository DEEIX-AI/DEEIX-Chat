import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  listAdminLLMModels,
  setAdminLLMModelProtocols,
  setAdminLLMModelsDisplayGroup,
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
  type ModelSortValue,
} from "@/features/admin/types/llm";
import {
  isValidModelContextWindow,
  modelContextWindowOverride,
  modelMaxOutputTokensOverride,
  setModelContextWindowInCapabilities,
} from "@/features/admin/model/model-context-window";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveKindsDisplayForProtocols } from "@/features/admin/utils/llm-display";
import {
  applySourceAvailabilityDelta,
  isAdminLLMSourceAvailable,
} from "@/features/admin/utils/llm-source-availability";
import { patchByID, removeByID, removeManyByID, replaceByID } from "@/shared/lib/optimistic-list";
import { runSettledBulkItems } from "@/shared/lib/bulk-action";

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
  batchDisplayGroupID: string;
  setBatchDisplayGroupID: (value: string) => void;
  batchContextWindow: string;
  setBatchContextWindow: (value: string) => void;
  batchStatus: AdminLLMStatus | "";
  setBatchStatus: (value: AdminLLMStatus | "") => void;
  pendingModelIDs: ReadonlySet<number>;
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
  handleBulkApplyDisplayGroup: () => Promise<void>;
  handleBulkApplyContextWindow: () => Promise<void>;
  handleBulkApplyStatus: () => Promise<void>;
  handleSourceAvailabilityChange: (modelID: number, previousAvailable: boolean, nextAvailable: boolean) => void;
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
  const [batchDisplayGroupID, setBatchDisplayGroupID] = React.useState("");
  const [batchContextWindow, setBatchContextWindow] = React.useState("");
  const [batchStatus, setBatchStatus] = React.useState<AdminLLMStatus | "">("");
  const [pendingModelIDs, setPendingModelIDs] = React.useState<Set<number>>(new Set());
  const [, startTableTransition] = React.useTransition();
  const requestSeqRef = React.useRef(0);
  const pageSizeRef = React.useRef(PAGE_SIZE_DEFAULT);
  const pendingModelIDsRef = React.useRef(new Set<number>());

  const beginModelUpdate = React.useCallback((modelID: number): boolean => {
    if (pendingModelIDsRef.current.has(modelID)) {
      return false;
    }
    pendingModelIDsRef.current.add(modelID);
    setPendingModelIDs(new Set(pendingModelIDsRef.current));
    setSelectedModelIDs((current) => {
      if (!current.has(modelID)) {
        return current;
      }
      const next = new Set(current);
      next.delete(modelID);
      return next;
    });
    return true;
  }, []);

  const finishModelUpdate = React.useCallback((modelID: number) => {
    pendingModelIDsRef.current.delete(modelID);
    setPendingModelIDs(new Set(pendingModelIDsRef.current));
  }, []);

  React.useEffect(() => {
    pageSizeRef.current = pageSize;
  }, [pageSize]);

  const loadModels = React.useCallback(
    async (nextPage = 1, nextPageSize = pageSizeRef.current) => {
      const requestSeq = requestSeqRef.current + 1;
      requestSeqRef.current = requestSeq;
      setLoading(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          if (requestSeq !== requestSeqRef.current) {
            return;
          }
          toast.error(t("sessionExpired"), { description: t("signInAgain") });
          return;
        }
        const data = await listAdminLLMModels(token, {
          page: nextPage,
          pageSize: nextPageSize,
          onlyActive: false,
          query: query.trim(),
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
        if (requestSeq !== requestSeqRef.current) {
          return;
        }
        toast.error(t("modelsLoadFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        if (requestSeq === requestSeqRef.current) {
          setLoading(false);
        }
      }
    },
    [protocolFilter, query, sortValue, startTableTransition, statusFilter, t, vendorFilter],
  );

  React.useEffect(() => {
    void loadModels(1, pageSizeRef.current);
  }, [loadModels]);

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
      if (!beginModelUpdate(item.id)) return;
      const previousItem = items.find((model) => model.id === item.id) ?? item;
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("sessionExpired"), { description: t("signInAgain") });
          return;
        }
        setItems((current) =>
          patchByID(current, item.id, (model) => model.id, {
            status: nextStatus,
            ...(nextStatus === "inactive" ? { activeSourceCount: 0 } : {}),
          }),
        );
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
          await loadModels(nextPage, pageSize);
        }
      } catch (error) {
        setItems((current) => replaceByID(current, item.id, (model) => model.id, previousItem));
        toast.error(t("modelStatusUpdateFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        finishModelUpdate(item.id);
      }
    },
    [beginModelUpdate, finishModelUpdate, items, loadModels, page, pageSize, sortValue, statusFilter, t],
  );

  const handleToggleAccessScope = React.useCallback(
    async (item: AdminLLMModelDTO, nextScope: AdminLLMModelAccessScope) => {
      if (!beginModelUpdate(item.id)) return;
      const previousItem = items.find((model) => model.id === item.id) ?? item;
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("sessionExpired"), { description: t("signInAgain") });
          return;
        }
        setItems((current) => patchByID(current, item.id, (model) => model.id, { accessScope: nextScope }));
        const data = await updateAdminLLMModel(token, item.id, { accessScope: nextScope });
        setItems((current) => replaceByID(current, item.id, (model) => model.id, data.model));
        toast.success(nextScope === "public" ? t("modelScopePublic") : t("modelScopeInternal"));
        if (sortValue === "updated_desc") {
          await loadModels(page, pageSize);
        }
      } catch (error) {
        setItems((current) => replaceByID(current, item.id, (model) => model.id, previousItem));
        toast.error(t("modelScopeUpdateFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        finishModelUpdate(item.id);
      }
    },
    [beginModelUpdate, finishModelUpdate, items, loadModels, page, pageSize, sortValue, t],
  );

  const handleSourceAvailabilityChange = React.useCallback((modelID: number, previousAvailable: boolean, nextAvailable: boolean) => {
    setItems((current) =>
      current.map((item) =>
        item.id === modelID
          ? {
              ...item,
              activeSourceCount: applySourceAvailabilityDelta(
                item.activeSourceCount,
                item.sourceCount,
                previousAvailable,
                nextAvailable,
              ),
            }
          : item,
      ),
    );
  }, []);

  const handleSourceDeleteChange = React.useCallback((modelID: number, source: AdminLLMModelUpstreamSourceDTO, deleted: boolean) => {
    const sourceDelta = deleted ? -1 : 1;
    setItems((current) =>
      current.map((item) => {
        if (item.id !== modelID) {
          return item;
        }
        const nextSourceCount = Math.max(0, item.sourceCount + sourceDelta);
        const wasAvailable = isAdminLLMSourceAvailable(source, item.status);
        return {
          ...item,
          sourceCount: nextSourceCount,
          activeSourceCount: applySourceAvailabilityDelta(
            item.activeSourceCount,
            nextSourceCount,
            deleted ? wasAvailable : false,
            deleted ? false : wasAvailable,
          ),
        };
      }),
    );
  }, []);

  const runBulkModelUpdates = React.useCallback(async (options: {
    targets: AdminLLMModelDTO[];
    optimisticPatch: (item: AdminLLMModelDTO) => AdminLLMModelDTO;
    successMessage: string;
    partialFailureMessage: string;
    failureMessage: string;
    runItem: (token: string, item: AdminLLMModelDTO) => Promise<{ model: AdminLLMModelDTO }>;
    onSuccess: () => void;
  }) => {
    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("sessionExpired"), { description: t("signInAgain") });
      return;
    }

    const rollbackModels = options.targets.map((item) => items.find((current) => current.id === item.id) ?? item);
    const targetIDs = new Set(options.targets.map((item) => item.id));
    setBatchApplying(true);
    setItems((current) =>
      current.map((item) => (targetIDs.has(item.id) ? options.optimisticPatch(item) : item)),
    );
    try {
      const results = await runSettledBulkItems({
        items: options.targets,
        title: options.successMessage,
        runItem: (item) => options.runItem(token, item),
      });
      const failedModels = results.filter((result) => result.status === "rejected").map((result) => result.item);
      const successModels = results.filter((result) => result.status === "fulfilled").map((result) => result.item);
      const successResponses = results
        .filter((result): result is Extract<typeof result, { status: "fulfilled" }> => result.status === "fulfilled")
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
        toast.error(options.partialFailureMessage, {
          description: t("bulkPartialDescription", { success: successModels.length, failed: failedModels.length }),
        });
        return;
      }

      toast.success(options.successMessage);
      setSelectedModelIDs(new Set());
      options.onSuccess();
    } catch (error) {
      setItems((current) =>
        rollbackModels.reduce((next, model) => replaceByID(next, model.id, (item) => item.id, model), current),
      );
      toast.error(options.failureMessage, { description: resolveAdminErrorMessage(error) });
    } finally {
      setBatchApplying(false);
    }
  }, [items, t]);

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

    await runBulkModelUpdates({
      targets,
      optimisticPatch: (item) => ({ ...item, kindsJSON: nextKindsJSON }),
      successMessage: t("bulkKindsUpdated", { count: targets.length }),
      partialFailureMessage: t("bulkKindsPartialFailed"),
      failureMessage: t("bulkKindsFailed"),
      runItem: (token, item) => updateAdminLLMModel(token, item.id, { kindsJSON: nextKindsJSON }),
      onSuccess: () => setBatchKindsDisplay(""),
    });
  }, [batchApplying, batchKindsDisplay, runBulkModelUpdates, selectedModels, t]);

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

    await runBulkModelUpdates({
      targets,
      optimisticPatch: (item) => ({ ...item, vendor: nextVendor }),
      successMessage: t("bulkVendorUpdated", { count: targets.length }),
      partialFailureMessage: t("bulkVendorPartialFailed"),
      failureMessage: t("bulkVendorFailed"),
      runItem: (token, item) => updateAdminLLMModel(token, item.id, { vendor: nextVendor }),
      onSuccess: () => setBatchVendor(""),
    });
  }, [batchApplying, batchVendor, runBulkModelUpdates, selectedModels, t]);

  const handleBulkApplyDisplayGroup = React.useCallback(async () => {
    if (!selectedModels.length || batchDisplayGroupID === "" || batchApplying) {
      return;
    }
    const nextDisplayGroupID = Number(batchDisplayGroupID);
    if (!Number.isInteger(nextDisplayGroupID) || nextDisplayGroupID < 0) {
      return;
    }

    const targets = selectedModels.filter((item) => (item.displayGroupID ?? 0) !== nextDisplayGroupID);
    if (!targets.length) {
      toast.info(t("bulkDisplayGroupAlreadyApplied"));
      return;
    }

    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("sessionExpired"), { description: t("signInAgain") });
      return;
    }

    setBatchApplying(true);
    try {
      await setAdminLLMModelsDisplayGroup(token, {
        modelIDs: targets.map((item) => item.id),
        displayGroupID: nextDisplayGroupID,
      });
      await loadModels(page, pageSize);
      toast.success(t("bulkDisplayGroupUpdated", { count: targets.length }));
      setBatchDisplayGroupID("");
    } catch (error) {
      toast.error(t("bulkDisplayGroupFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setBatchApplying(false);
    }
  }, [batchApplying, batchDisplayGroupID, loadModels, page, pageSize, selectedModels, t]);

  const handleBulkApplyContextWindow = React.useCallback(async () => {
    const nextContextWindow = Number(batchContextWindow.trim());
    if (!selectedModels.length || !batchContextWindow.trim() || batchApplying) {
      return;
    }
    if (!isValidModelContextWindow(nextContextWindow)) {
      toast.error(t("bulkContextWindowInvalid"));
      return;
    }

    const capabilitiesByID = new Map<number, string>();
    const targets: AdminLLMModelDTO[] = [];
    for (const item of selectedModels) {
      if (modelContextWindowOverride(item.capabilitiesJSON) === nextContextWindow) {
        continue;
      }
      const maxOutputTokens = modelMaxOutputTokensOverride(item.capabilitiesJSON);
      if (maxOutputTokens !== null && maxOutputTokens >= nextContextWindow) {
        toast.error(t("bulkContextWindowOutputConflict", {
          max: maxOutputTokens,
          name: item.platformModelName,
        }));
        return;
      }
      const capabilitiesJSON = setModelContextWindowInCapabilities(
        item.capabilitiesJSON,
        nextContextWindow,
      );
      if (capabilitiesJSON === null) {
        toast.error(t("bulkContextWindowInvalidCapabilities", { name: item.platformModelName }));
        return;
      }
      capabilitiesByID.set(item.id, capabilitiesJSON);
      targets.push(item);
    }

    if (!targets.length) {
      toast.info(t("bulkContextWindowAlreadyApplied"));
      return;
    }

    await runBulkModelUpdates({
      targets,
      optimisticPatch: (item) => ({
        ...item,
        capabilitiesJSON: capabilitiesByID.get(item.id) ?? item.capabilitiesJSON,
        contextWindow: nextContextWindow,
      }),
      successMessage: t("bulkContextWindowUpdated", { count: targets.length }),
      partialFailureMessage: t("bulkContextWindowPartialFailed"),
      failureMessage: t("bulkContextWindowFailed"),
      runItem: (token, item) => updateAdminLLMModel(token, item.id, {
        capabilitiesJSON: capabilitiesByID.get(item.id) ?? item.capabilitiesJSON,
      }),
      onSuccess: () => setBatchContextWindow(""),
    });
  }, [batchApplying, batchContextWindow, runBulkModelUpdates, selectedModels, t]);

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

    await runBulkModelUpdates({
      targets,
      optimisticPatch: (item) => ({ ...item, status: nextStatus }),
      successMessage: t("bulkStatusUpdated", { count: targets.length }),
      partialFailureMessage: t("bulkStatusPartialFailed"),
      failureMessage: t("bulkStatusFailed"),
      runItem: (token, item) => updateAdminLLMModel(token, item.id, { status: nextStatus }),
      onSuccess: () => setBatchStatus(""),
    });
  }, [batchApplying, batchStatus, runBulkModelUpdates, selectedModels, t]);

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

    const nextProtocolsJSON = JSON.stringify([nextProtocol]);
    const nextKindsJSON = displayToKindsJson(resolveKindsDisplayForProtocols([nextProtocol]));
    await runBulkModelUpdates({
      targets,
      optimisticPatch: (item) => ({ ...item, protocolsJSON: nextProtocolsJSON, kindsJSON: nextKindsJSON }),
      successMessage: t("bulkProtocolUpdated", { count: targets.length }),
      partialFailureMessage: t("bulkProtocolPartialFailed"),
      failureMessage: t("bulkProtocolFailed"),
      runItem: (token, item) =>
        setAdminLLMModelProtocols(token, item.id, {
          protocols: [nextProtocol],
          kindsJSON: nextKindsJSON,
        }),
      onSuccess: () => setBatchProtocol(""),
    });
  }, [batchApplying, batchProtocol, runBulkModelUpdates, selectedModels, t]);

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
    batchDisplayGroupID,
    setBatchDisplayGroupID,
    batchContextWindow,
    setBatchContextWindow,
    batchStatus,
    setBatchStatus,
    pendingModelIDs,
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
    handleBulkApplyDisplayGroup,
    handleBulkApplyContextWindow,
    handleBulkApplyStatus,
    handleSourceAvailabilityChange,
    handleSourceDeleteChange,
    handleRequestBulkDelete,
    handleDeleted,
    handleBulkDeleted,
  };
}
