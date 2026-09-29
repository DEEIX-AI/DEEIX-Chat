"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  batchDeleteAdminLLMUpstreamModels,
  deleteAdminLLMUpstreamModel,
  listAdminLLMUpstreamModels,
  upsertAdminLLMUpstreamModel,
} from "@/features/admin/api";
import type {
  AdminLLMAdapter,
  AdminLLMUpstreamView,
  UpsertAdminLLMUpstreamModelRequest,
} from "@/features/admin/api/llm-types";
import {
  buildRowDrafts,
  displayToKindsJson,
  type RowDraft,
  summarizeBatchDeleteResult,
  validateRowDrafts,
} from "@/features/admin/model/upstreams-models";
import { PAGE_SIZE_DEFAULT } from "@/features/admin/types/llm";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { mergeBatchResultData, runBulkActionInChunks } from "@/shared/lib/bulk-action";

export const ROUTE_STATUS_FILTERS = ["bound", "active", "inactive"] as const;
export type RouteStatusFilter = (typeof ROUTE_STATUS_FILTERS)[number];
export const UPSTREAM_STATUS_FILTERS = ["all", "active", "inactive"] as const;
export type UpstreamStatusFilter = (typeof UPSTREAM_STATUS_FILTERS)[number];
export const ROUTE_SORT_VALUES = ["upstream_asc", "upstream_desc", "platform_asc", "platform_desc", "status_asc", "protocol_asc"] as const;
export type RouteSortValue = (typeof ROUTE_SORT_VALUES)[number];

export type RouteListParams = {
  upstreamID: number | null;
  page: number;
  pageSize: number;
  query: string;
  routeStatusFilter: RouteStatusFilter;
  upstreamStatusFilter: UpstreamStatusFilter;
  protocolFilter: string;
  sortValue: RouteSortValue;
};

export type RowDraftPatch = Partial<Omit<RowDraft, "draftKey" | "isDirty" | "routeStatusOverridden">>;

const DEFAULT_ROUTE_LIST_PARAMS: RouteListParams = {
  upstreamID: null,
  page: 1,
  pageSize: PAGE_SIZE_DEFAULT,
  query: "",
  routeStatusFilter: "bound",
  upstreamStatusFilter: "all",
  protocolFilter: "",
  sortValue: "upstream_asc",
};

export function routeIDsForRow(row: RowDraft): number[] {
  return Object.values(row.routeIDsByProtocol).filter((id) => id > 0);
}

function removeRouteIDFromRows(rows: RowDraft[], routeID: number): RowDraft[] {
  return rows.flatMap((row) => {
    if (!routeIDsForRow(row).includes(routeID)) {
      return [row];
    }
    const nextRouteIDsByProtocol = Object.fromEntries(
      Object.entries(row.routeIDsByProtocol).filter(([, id]) => id !== routeID),
    );
    const nextRouteIDs = Object.values(nextRouteIDsByProtocol).filter((id) => id > 0);
    if (nextRouteIDs.length === 0) {
      return [];
    }
    const nextProtocols = row.protocols.filter((protocol) => nextRouteIDsByProtocol[protocol] > 0);
    return [
      {
        ...row,
        protocol: nextProtocols[0] ?? row.protocol,
        protocols: nextProtocols,
        routeID: Math.min(...nextRouteIDs),
        routeIDsByProtocol: nextRouteIDsByProtocol,
      },
    ];
  });
}

function selectedProtocolsForSave(row: RowDraft): AdminLLMAdapter[] {
  const protocols = row.protocols.length > 0 ? row.protocols : [];
  return Array.from(new Set(protocols));
}

async function runOperationsInOrder(operations: Array<() => Promise<unknown>>): Promise<void> {
  for (const operation of operations) {
    await operation();
  }
}

type UseAdminUpstreamsModelsOptions = {
  open: boolean;
  /** Dialog snapshot of the upstream; stays set while the dialog animates closed. */
  upstream: AdminLLMUpstreamView | null;
  onUpstreamUpdated: (updated: AdminLLMUpstreamView) => void;
};

// Route bindings of one upstream: server-paged list with local row drafts,
// selection, batch delete and ordered save (upserts before deletes).
export function useAdminUpstreamsModels({ open, upstream, onUpstreamUpdated }: UseAdminUpstreamsModelsOptions) {
  const t = useTranslations("adminUpstreams");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [rows, setRows] = React.useState<RowDraft[]>([]);
  const [loadedUpstreamID, setLoadedUpstreamID] = React.useState<number | null>(null);
  const [loadingList, setLoadingList] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [deleting, setDeleting] = React.useState(false);
  const [selected, setSelected] = React.useState<Set<string>>(new Set());
  const [query, setQuery] = React.useState("");
  const [listParams, setListParams] = React.useState<RouteListParams>(DEFAULT_ROUTE_LIST_PARAMS);
  const [total, setTotal] = React.useState(0);
  // Only the latest list request may write state; older responses are dropped.
  const requestSeqRef = React.useRef(0);
  const upstreamID = upstream?.id ?? null;

  const loadBindings = React.useCallback(async (params: RouteListParams = listParams) => {
    if (!upstreamID || params.upstreamID !== upstreamID) return;
    const requestSeq = requestSeqRef.current + 1;
    requestSeqRef.current = requestSeq;
    setLoadingList(true);
    try {
      const token = await resolveAccessToken();
      const result = await listAdminLLMUpstreamModels(token, upstreamID, {
        page: params.page,
        pageSize: params.pageSize,
        query: params.query,
        routeStatus: params.routeStatusFilter,
        upstreamStatus: params.upstreamStatusFilter === "all" ? "" : params.upstreamStatusFilter,
        protocol: params.protocolFilter,
        sort: params.sortValue,
      });
      if (requestSeq !== requestSeqRef.current) {
        return;
      }
      setRows(buildRowDrafts(result.results));
      setTotal(result.total);
      setLoadedUpstreamID(upstreamID);
      setSelected(new Set());
    } catch (err) {
      if (requestSeq !== requestSeqRef.current) {
        return;
      }
      setRows([]);
      setTotal(0);
      setLoadedUpstreamID(upstreamID);
      toast.error(t("modelsDialog.loadFailed"), { description: resolveErrorMessage(err) });
    } finally {
      if (requestSeq === requestSeqRef.current) {
        setLoadingList(false);
      }
    }
  }, [listParams, resolveErrorMessage, t, upstreamID]);

  React.useEffect(() => {
    if (!open || !upstreamID) return;
    requestSeqRef.current += 1;
    setRows([]);
    setTotal(0);
    setLoadedUpstreamID(null);
    setSelected(new Set());
    setQuery("");
    setListParams({ ...DEFAULT_ROUTE_LIST_PARAMS, upstreamID });
    return () => {
      requestSeqRef.current += 1;
    };
  }, [open, upstreamID]);

  React.useEffect(() => {
    if (!open || !upstreamID || listParams.upstreamID !== upstreamID) {
      return;
    }
    void loadBindings(listParams);
  }, [listParams, loadBindings, open, upstreamID]);

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      const nextQuery = query.trim();
      setListParams((prev) => {
        if (!open || !upstreamID || prev.upstreamID !== upstreamID) {
          return prev;
        }
        if (prev.query === nextQuery && prev.page === 1) {
          return prev;
        }
        return { ...prev, query: nextQuery, page: 1 };
      });
    }, 250);
    return () => window.clearTimeout(timer);
  }, [open, query, upstreamID]);

  const updateListParams = React.useCallback((patch: Partial<RouteListParams>) => {
    setListParams((prev) => ({ ...prev, ...patch, page: patch.page ?? 1 }));
  }, []);

  const updateRow = React.useCallback((
    draftKey: string,
    patch: RowDraftPatch,
  ) => {
    setRows((prev) =>
      prev.map((r) =>
        r.draftKey === draftKey
          ? {
              ...r,
              ...patch,
              isDirty: true,
              routeStatusOverridden: r.routeStatusOverridden || patch.routeStatus !== undefined,
            }
          : r,
      ),
    );
  }, []);

  const applyBulkPatch = React.useCallback((patch: RowDraftPatch) => {
    if (selected.size === 0) return;
    setRows((prev) =>
      prev.map((row) => {
        if (routeIDsForRow(row).length === 0 || !selected.has(row.draftKey)) return row;
        // Route toggles for models delisted upstream are not operable; skip them in bulk route-status changes too, to avoid silently rewriting disabled toggles.
        const { routeStatus: _routeStatus, ...rest } = patch;
        const rowPatch: RowDraftPatch = row.upstreamModelStatus === "inactive" ? rest : patch;
        if (Object.keys(rowPatch).length === 0) return row;
        return {
          ...row,
          ...rowPatch,
          isDirty: true,
          routeStatusOverridden: row.routeStatusOverridden || rowPatch.routeStatus !== undefined,
        };
      }),
    );
  }, [selected]);

  /** Drops a route deleted elsewhere (e.g. from the probe dialog) from rows and selection. */
  const removeRouteLocally = React.useCallback((routeID: number) => {
    setRows((prev) => removeRouteIDFromRows(prev, routeID));
    setSelected((prev) => {
      const next = new Set(prev);
      rows.forEach((row) => {
        if (routeIDsForRow(row).includes(routeID)) {
          next.delete(row.draftKey);
        }
      });
      return next;
    });
  }, [rows]);

  /** Resolves `true` when the delete request ran (successfully or not), `false` when there was nothing to delete. */
  async function deleteSelected(): Promise<boolean> {
    if (!upstream || selected.size === 0) return false;
    const routeIDs = rows
      .filter((row) => selected.has(row.draftKey))
      .flatMap(routeIDsForRow);
    if (routeIDs.length === 0) return false;
    setDeleting(true);
    try {
      const token = await resolveAccessToken();
      const result = mergeBatchResultData(await runBulkActionInChunks({
        items: routeIDs,
        title: t("modelsDialog.batchDeleteTitle"),
        runChunk: (ids) => batchDeleteAdminLLMUpstreamModels(token, upstream.id, { ids }),
      }));
      const deletedIDs = new Set(
        result.results
          .filter((item) => item.status === "deleted" || item.status === "not_found")
          .map((item) => item.id),
      );
      setRows((prev) =>
        prev.filter((row) => routeIDsForRow(row).some((routeID) => !deletedIDs.has(routeID))),
      );
      setSelected(new Set());
      if (result.failedCount > 0) {
        toast.error(t("modelsDialog.batchDeletePartialFailed"), {
          description: summarizeBatchDeleteResult(result, {
            batchDeleteSummary: (successCount, notFoundCount, failedCount) =>
              t("modelsDialog.batchDeleteSummary", { successCount, notFoundCount, failedCount }),
          }),
        });
      } else {
        toast.success(t("modelsDialog.batchDeleteDone"), {
          description: summarizeBatchDeleteResult(result, {
            batchDeleteSummary: (successCount, notFoundCount, failedCount) =>
              t("modelsDialog.batchDeleteSummary", { successCount, notFoundCount, failedCount }),
          }),
        });
      }
      void loadBindings();
      onUpstreamUpdated({ ...upstream });
    } catch (err) {
      toast.error(t("toast.deleteFailed"), { description: resolveErrorMessage(err) });
    } finally {
      setDeleting(false);
    }
    return true;
  }

  async function save() {
    if (!upstream) return;
    const dirty = rows.filter((r) => r.isDirty);
    if (dirty.length === 0) {
      toast.info(t("modelsDialog.noPendingChanges"));
      return;
    }
    const validationError = validateRowDrafts(rows, {
      upstreamModelRequired: t("modelsDialog.upstreamModelRequired"),
      activeRouteRequiresPlatformModel: t("modelsDialog.activeRouteRequiresPlatformModel"),
      duplicateBinding: (upstreamModelName, platformModelName) =>
        t("modelsDialog.duplicateBinding", { upstreamModelName, platformModelName }),
    });
    if (validationError) {
      toast.error(validationError);
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      const deleteOperations: Array<() => Promise<unknown>> = [];
      const upsertOperations: Array<() => Promise<unknown>> = [];
      let savedCount = 0;
      let deletedCount = 0;

      for (const row of dirty) {
        const platformModelName = row.platformModelNameDraft.trim();
        const existingRouteIDs = routeIDsForRow(row);
        const shouldDeleteRoute =
          existingRouteIDs.length > 0 &&
          row.routeStatus === "inactive" &&
          platformModelName.length === 0;

        if (shouldDeleteRoute) {
          for (const routeID of existingRouteIDs) {
            deleteOperations.push(() => deleteAdminLLMUpstreamModel(token, upstream.id, routeID));
            deletedCount += 1;
          }
          continue;
        }
        if (!platformModelName) {
          continue;
        }

        const basePayload: Omit<UpsertAdminLLMUpstreamModelRequest, "protocols"> = {
          platformModelName,
          upstreamModelName: row.upstreamModelName.trim(),
          kindsJSON: displayToKindsJson(row.kindsDisplay),
          ...(row.routeStatusOverridden ? { status: row.routeStatus || "active" } : {}),
        };
        const desiredProtocols = selectedProtocolsForSave(row);
        upsertOperations.push(() =>
          upsertAdminLLMUpstreamModel(token, upstream.id, {
            ...basePayload,
            routeIDs: existingRouteIDs,
            protocols: desiredProtocols,
          }),
        );
        savedCount += 1;
      }

      if (deleteOperations.length === 0 && upsertOperations.length === 0) {
        toast.info(t("modelsDialog.noSavableChanges"));
        await loadBindings();
        return;
      }

      await runOperationsInOrder(upsertOperations);
      await runOperationsInOrder(deleteOperations);
      if (savedCount > 0 && deletedCount > 0) {
        toast.success(t("modelsDialog.savedAndDeleted", { savedCount, deletedCount }));
      } else if (deletedCount > 0) {
        toast.success(t("modelsDialog.deletedBindings", { deletedCount }), {
          description: t("modelsDialog.deleteBindingDescription"),
        });
      } else {
        toast.success(t("modelsDialog.savedChanges", { savedCount }));
      }
      await loadBindings();
      onUpstreamUpdated({ ...upstream });
    } catch (err) {
      toast.error(t("toast.updateFailed"), { description: resolveErrorMessage(err) });
    } finally {
      setSaving(false);
    }
  }

  return {
    rows,
    loadedUpstreamID,
    loadingList,
    saving,
    deleting,
    selected,
    setSelected,
    query,
    setQuery,
    listParams,
    total,
    loadBindings,
    updateListParams,
    updateRow,
    applyBulkPatch,
    removeRouteLocally,
    deleteSelected,
    save,
  };
}
