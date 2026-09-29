"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  deleteAdminLLMUpstreamModel,
  listAdminLLMModelUpstreamSources,
  openAdminLLMUpstreamModelCircuit,
  resetAdminLLMUpstreamCircuit,
  resetAdminLLMUpstreamModelCircuit,
  updateAdminLLMModelUpstreamSource,
} from "@/features/admin/api";
import type {
  AdminLLMModelDTO,
  AdminLLMModelUpstreamSourceDTO,
  AdminLLMStatus,
  UpdateAdminLLMModelUpstreamSourceRequest,
} from "@/features/admin/api/llm-types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { isAdminLLMSourceAvailable } from "@/features/admin/utils/llm-source-availability";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type InlineSourceEntry = {
  items: AdminLLMModelUpstreamSourceDTO[];
  loading: boolean;
};

export type InlineSourceTarget = {
  modelId: number;
  source: AdminLLMModelUpstreamSourceDTO;
};

type UseAdminModelsInlineSourcesOptions = {
  items: AdminLLMModelDTO[];
  circuitBreakerEnabled: boolean;
  onRefreshModels?: () => void;
  onSourceAvailabilityChange?: (modelID: number, previousAvailable: boolean, nextAvailable: boolean) => void;
  onSourceDeleteChange?: (modelID: number, source: AdminLLMModelUpstreamSourceDTO, deleted: boolean) => void;
};

// Upstream sources shown in the expanded rows of the models table, with
// optimistic status/circuit updates that roll back on failure.
export function useAdminModelsInlineSources({
  items,
  circuitBreakerEnabled,
  onRefreshModels,
  onSourceAvailabilityChange,
  onSourceDeleteChange,
}: UseAdminModelsInlineSourcesOptions) {
  const t = useTranslations("adminModels");
  const [inlineSources, setInlineSources] = React.useState<Record<number, InlineSourceEntry>>({});
  const [deleteSourcePending, setDeleteSourcePending] = React.useState(false);
  const [circuitPending, setCircuitPending] = React.useState(false);
  // Mirrors state synchronously so a rapid re-expand does not refetch a row
  // whose load is already in flight.
  const inlineSourcesRef = React.useRef(inlineSources);

  React.useEffect(() => {
    inlineSourcesRef.current = inlineSources;
  }, [inlineSources]);

  React.useEffect(() => {
    if (circuitBreakerEnabled) return;
    setInlineSources((current) =>
      Object.fromEntries(
        Object.entries(current).map(([modelID, entry]) => [
          modelID,
          {
            ...entry,
            items: entry.items.map((source) => ({
              ...source,
              circuitOpen: false,
              circuitUntil: "",
              circuitScope: "" as const,
            })),
          },
        ]),
      ),
    );
  }, [circuitBreakerEnabled]);

  const refreshInlineSources = React.useCallback(async (modelId: number) => {
    const token = await resolveAccessToken();
    if (!token) return;
    const data = await listAdminLLMModelUpstreamSources(token, modelId, {
      page: 1,
      pageSize: 100,
    });
    const nextEntry = { items: data.results, loading: false };
    inlineSourcesRef.current = {
      ...inlineSourcesRef.current,
      [modelId]: nextEntry,
    };
    setInlineSources((prev) => ({
      ...prev,
      [modelId]: nextEntry,
    }));
  }, []);

  /** Loads a row's sources the first time it is expanded; later expands reuse the cache. */
  const ensureInlineSources = React.useCallback(async (modelId: number) => {
    if (inlineSourcesRef.current[modelId]) {
      return;
    }
    const loadingEntry: InlineSourceEntry = { items: [], loading: true };
    inlineSourcesRef.current = {
      ...inlineSourcesRef.current,
      [modelId]: loadingEntry,
    };
    setInlineSources((prev) => ({
      ...prev,
      [modelId]: loadingEntry,
    }));
    try {
      await refreshInlineSources(modelId);
    } catch {
      const failedEntry: InlineSourceEntry = { items: [], loading: false };
      inlineSourcesRef.current = {
        ...inlineSourcesRef.current,
        [modelId]: failedEntry,
      };
      setInlineSources((prev) => ({
        ...prev,
        [modelId]: failedEntry,
      }));
    }
  }, [refreshInlineSources]);

  const handleInlineCircuit = React.useCallback(
    async (
      source: AdminLLMModelUpstreamSourceDTO,
      modelId: number,
      action: "open" | "reset",
    ) => {
      const token = await resolveAccessToken();
      if (!token) return;
      const nextSource =
        action === "open"
          ? {
              ...source,
              circuitOpen: true,
              circuitUntil: String(Math.floor(Date.now() / 1000) + 24 * 60 * 60),
              circuitScope: "source" as const,
            }
          : { ...source, circuitOpen: false, circuitUntil: "", circuitScope: "" as const };
      const modelStatus = items.find((item) => item.id === modelId)?.status ?? "inactive";
      const previousAvailable = isAdminLLMSourceAvailable(source, modelStatus);
      const nextAvailable = isAdminLLMSourceAvailable(nextSource, modelStatus);
      setInlineSources((prev) => ({
        ...prev,
        [modelId]: {
          ...(prev[modelId] ?? { items: [], loading: false }),
          items: (prev[modelId]?.items ?? []).map((item) => (item.id === source.id ? nextSource : item)),
        },
      }));
      onSourceAvailabilityChange?.(modelId, previousAvailable, nextAvailable);
      try {
        if (action === "open") {
          await openAdminLLMUpstreamModelCircuit(token, source.upstreamID, source.id);
          toast.success(t("toast.circuitOpened"));
        } else if (source.circuitScope === "upstream") {
          await resetAdminLLMUpstreamCircuit(token, source.upstreamID);
          toast.success(t("toast.circuitReset"));
        } else {
          await resetAdminLLMUpstreamModelCircuit(token, source.upstreamID, source.id);
          toast.success(t("toast.circuitReset"));
        }
        onRefreshModels?.();
      } catch (error) {
        setInlineSources((prev) => ({
          ...prev,
          [modelId]: {
            ...(prev[modelId] ?? { items: [], loading: false }),
            items: (prev[modelId]?.items ?? []).map((item) => (item.id === source.id ? source : item)),
          },
        }));
        onSourceAvailabilityChange?.(modelId, nextAvailable, previousAvailable);
        toast.error(t("toast.operationFailed"), { description: resolveAdminErrorMessage(error) });
      }
    },
    [items, onRefreshModels, onSourceAvailabilityChange, t],
  );

  const handleInlineStatusToggle = React.useCallback(
    async (source: AdminLLMModelUpstreamSourceDTO, modelId: number) => {
      const token = await resolveAccessToken();
      if (!token) return;

      const nextStatus: AdminLLMStatus = source.status === "active" ? "inactive" : "active";
      const modelStatus = items.find((item) => item.id === modelId)?.status ?? "inactive";
      const nextSource = { ...source, status: nextStatus };
      const previousAvailable = isAdminLLMSourceAvailable(source, modelStatus);
      const nextAvailable = isAdminLLMSourceAvailable(nextSource, modelStatus);
      setInlineSources((prev) => ({
        ...prev,
        [modelId]: {
          ...(prev[modelId] ?? { items: [], loading: false }),
          items: (prev[modelId]?.items ?? []).map((item) =>
            item.id === source.id ? nextSource : item,
          ),
        },
      }));
      onSourceAvailabilityChange?.(modelId, previousAvailable, nextAvailable);
      try {
        const data = await updateAdminLLMModelUpstreamSource(token, modelId, source.id, {
          status: nextStatus,
        });
        setInlineSources((prev) => ({
          ...prev,
          [modelId]: {
            ...(prev[modelId] ?? { items: [], loading: false }),
            items: (prev[modelId]?.items ?? []).map((item) => (item.id === source.id ? data.source : item)),
          },
        }));
        toast.success(nextStatus === "inactive" ? t("toast.sourceDisabled") : t("toast.sourceEnabled"));
      } catch (error) {
        setInlineSources((prev) => ({
          ...prev,
          [modelId]: {
            ...(prev[modelId] ?? { items: [], loading: false }),
            items: (prev[modelId]?.items ?? []).map((item) => (item.id === source.id ? source : item)),
          },
        }));
        onSourceAvailabilityChange?.(modelId, nextAvailable, previousAvailable);
        toast.error(t("toast.operationFailed"), { description: resolveAdminErrorMessage(error) });
      }
    },
    [items, onSourceAvailabilityChange, t],
  );

  /** Resolves `true` once saved so the caller can close the circuit dialog. */
  const saveCircuitSettings = React.useCallback(async (
    target: InlineSourceTarget,
    payload: UpdateAdminLLMModelUpstreamSourceRequest,
  ): Promise<boolean> => {
    if (circuitPending) {
      return false;
    }

    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
      return false;
    }

    const { modelId, source } = target;
    setCircuitPending(true);
    try {
      const data = await updateAdminLLMModelUpstreamSource(token, modelId, source.id, payload);
      setInlineSources((prev) => ({
        ...prev,
        [modelId]: {
          ...(prev[modelId] ?? { items: [], loading: false }),
          items: (prev[modelId]?.items ?? []).map((item) => (item.id === source.id ? data.source : item)),
        },
      }));
      toast.success(t("sources.circuitUpdated"));
      return true;
    } catch (error) {
      toast.error(t("toast.routeUpdateFailed"), { description: resolveAdminErrorMessage(error) });
      return false;
    } finally {
      setCircuitPending(false);
    }
  }, [circuitPending, t]);

  /** Resolves `true` once deleted so the caller can close the confirmation dialog. */
  const deleteInlineSource = React.useCallback(async (target: InlineSourceTarget): Promise<boolean> => {
    if (deleteSourcePending) {
      return false;
    }

    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
      return false;
    }

    const { modelId, source } = target;
    const previousEntry = inlineSourcesRef.current[modelId] ?? { items: [], loading: false };
    setDeleteSourcePending(true);
    setInlineSources((prev) => ({
      ...prev,
      [modelId]: {
        ...(prev[modelId] ?? { items: [], loading: false }),
        items: (prev[modelId]?.items ?? []).filter((item) => item.id !== source.id),
      },
    }));
    onSourceDeleteChange?.(modelId, source, true);

    try {
      await deleteAdminLLMUpstreamModel(token, source.upstreamID, source.id);
      toast.success(t("toast.sourceDeleted"));
      return true;
    } catch (error) {
      setInlineSources((prev) => ({
        ...prev,
        [modelId]: previousEntry,
      }));
      onSourceDeleteChange?.(modelId, source, false);
      toast.error(t("toast.sourceDeleteFailed"), { description: resolveAdminErrorMessage(error) });
      return false;
    } finally {
      setDeleteSourcePending(false);
    }
  }, [deleteSourcePending, onSourceDeleteChange, t]);

  return {
    inlineSources,
    deleteSourcePending,
    circuitPending,
    ensureInlineSources,
    handleInlineCircuit,
    handleInlineStatusToggle,
    saveCircuitSettings,
    deleteInlineSource,
  };
}
