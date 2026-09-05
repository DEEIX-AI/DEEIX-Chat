import * as React from "react";
import {
  importAdminLLMUpstreamModels,
  listAdminLLMRemoteModels,
  syncAdminLLMUpstreamModels,
} from "@/features/admin/api";
import {
  listPermissionGroups,
  type PermissionGroup,
} from "@/features/admin/api/permission-groups";
import type {
  ImportAdminLLMUpstreamModelsData,
  ImportAdminLLMUpstreamModelsRequest,
  ListAdminLLMRemoteModelsData,
  SyncAdminLLMUpstreamModelsData,
} from "@/features/admin/api/llm.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type ApplyUpstreamModelSyncInput = {
  allowEmpty: boolean;
  expectedSnapshot: string;
  items: ImportAdminLLMUpstreamModelsRequest["items"];
  permissionGroupIDs?: number[];
};

export type ApplyUpstreamModelSyncResult = {
  catalog: SyncAdminLLMUpstreamModelsData;
  bindings: ImportAdminLLMUpstreamModelsData | null;
};

export class UpstreamModelBindingsApplyError extends Error {
  readonly catalog: SyncAdminLLMUpstreamModelsData;
  readonly originalError: unknown;

  constructor(catalog: SyncAdminLLMUpstreamModelsData, originalError: unknown) {
    super("upstream model catalog applied before route import failed");
    this.name = "UpstreamModelBindingsApplyError";
    this.catalog = catalog;
    this.originalError = originalError;
  }
}

export function isUpstreamModelSyncAbort(error: unknown): boolean {
  return error instanceof Error && error.name === "AbortError";
}

function throwIfAborted(signal: AbortSignal) {
  if (!signal.aborted) return;
  if (signal.reason instanceof Error && signal.reason.name === "AbortError") {
    throw signal.reason;
  }
  const error = new Error("The operation was aborted");
  error.name = "AbortError";
  throw error;
}

export function useUpstreamModelSync(open: boolean, upstreamID: number | null) {
  const [catalog, setCatalog] = React.useState<ListAdminLLMRemoteModelsData | null>(null);
  const [catalogLoading, setCatalogLoading] = React.useState(false);
  const [catalogError, setCatalogError] = React.useState<unknown>(null);
  const [permissionGroups, setPermissionGroups] = React.useState<PermissionGroup[]>([]);
  const [permissionGroupsLoading, setPermissionGroupsLoading] = React.useState(false);
  const [permissionGroupsError, setPermissionGroupsError] = React.useState<unknown>(null);
  const catalogRequestRef = React.useRef<{ controller: AbortController; version: number } | null>(null);
  const catalogRequestVersionRef = React.useRef(0);
  const applyControllerRef = React.useRef<AbortController | null>(null);

  const cancelCatalogRequest = React.useCallback(() => {
    catalogRequestVersionRef.current += 1;
    catalogRequestRef.current?.controller.abort();
    catalogRequestRef.current = null;
  }, []);

  const reloadCatalog = React.useCallback(async () => {
    if (!upstreamID) return;
    cancelCatalogRequest();
    const controller = new AbortController();
    const version = catalogRequestVersionRef.current;
    catalogRequestRef.current = { controller, version };
    setCatalogError(null);
    setCatalogLoading(true);
    try {
      const token = await resolveAccessToken();
      throwIfAborted(controller.signal);
      const data = await listAdminLLMRemoteModels(token, upstreamID, controller.signal);
      if (catalogRequestRef.current?.version === version) {
        setCatalog(data);
      }
    } catch (error) {
      if (catalogRequestRef.current?.version === version && !isUpstreamModelSyncAbort(error)) {
        setCatalogError(error);
      }
    } finally {
      if (catalogRequestRef.current?.version === version) {
        catalogRequestRef.current = null;
        setCatalogLoading(false);
      }
    }
  }, [cancelCatalogRequest, upstreamID]);

  React.useEffect(() => {
    if (!open || !upstreamID) {
      cancelCatalogRequest();
      applyControllerRef.current?.abort();
      applyControllerRef.current = null;
      setCatalog(null);
      setCatalogError(null);
      setCatalogLoading(false);
      return;
    }
    setCatalog(null);
    void reloadCatalog();
    return () => {
      cancelCatalogRequest();
      applyControllerRef.current?.abort();
      applyControllerRef.current = null;
    };
  }, [cancelCatalogRequest, open, reloadCatalog, upstreamID]);

  React.useEffect(() => {
    if (!open) {
      setPermissionGroups([]);
      setPermissionGroupsError(null);
      setPermissionGroupsLoading(false);
      return;
    }
    const controller = new AbortController();
    setPermissionGroups([]);
    setPermissionGroupsError(null);
    setPermissionGroupsLoading(true);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        throwIfAborted(controller.signal);
        const groups = await listPermissionGroups(token, controller.signal);
        if (!controller.signal.aborted) {
          setPermissionGroups(groups);
        }
      } catch (error) {
        if (!controller.signal.aborted && !isUpstreamModelSyncAbort(error)) {
          setPermissionGroupsError(error);
        }
      } finally {
        if (!controller.signal.aborted) {
          setPermissionGroupsLoading(false);
        }
      }
    })();
    return () => controller.abort();
  }, [open]);

  const applySync = React.useCallback(async (input: ApplyUpstreamModelSyncInput): Promise<ApplyUpstreamModelSyncResult> => {
    if (!upstreamID) {
      throw new Error("upstream is required");
    }
    applyControllerRef.current?.abort();
    const controller = new AbortController();
    applyControllerRef.current = controller;
    try {
      const token = await resolveAccessToken();
      throwIfAborted(controller.signal);
      const catalogResult = await syncAdminLLMUpstreamModels(token, upstreamID, {
        allowEmpty: input.allowEmpty,
        expectedSnapshot: input.expectedSnapshot,
        signal: controller.signal,
      });
      throwIfAborted(controller.signal);
      const currentRemoteNames = new Set(catalogResult.syncedModels.map((item) => item.upstreamModelName));
      const items = input.items.filter((item) => currentRemoteNames.has(item.upstreamModelName));
      if (items.length === 0) {
        return { catalog: catalogResult, bindings: null };
      }
      try {
        const bindings = await importAdminLLMUpstreamModels(token, upstreamID, {
          items,
          permissionGroupIDs: input.permissionGroupIDs,
        }, controller.signal);
        throwIfAborted(controller.signal);
        return { catalog: catalogResult, bindings };
      } catch (error) {
        if (isUpstreamModelSyncAbort(error)) throw error;
        throw new UpstreamModelBindingsApplyError(catalogResult, error);
      }
    } finally {
      if (applyControllerRef.current === controller) {
        applyControllerRef.current = null;
      }
    }
  }, [upstreamID]);

  return {
    catalog,
    catalogError,
    catalogLoading,
    permissionGroups,
    permissionGroupsError,
    permissionGroupsLoading,
    reloadCatalog,
    applySync,
  };
}
