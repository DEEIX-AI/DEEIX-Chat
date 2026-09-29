"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  bindAdminLLMModelUpstreamSource,
  listAdminLLMModelUpstreamSources,
  openAdminLLMUpstreamModelCircuit,
  resetAdminLLMUpstreamCircuit,
  resetAdminLLMUpstreamModelCircuit,
  updateAdminLLMModelUpstreamSource,
} from "@/features/admin/api";
import type {
  AdminLLMAdapter,
  AdminLLMModelDTO,
  AdminLLMModelUpstreamSourceDTO,
  AdminLLMStatus,
  UpdateAdminLLMModelUpstreamSourceRequest,
} from "@/features/admin/api/llm-types";
import { type ModelSourceBindDraft, resolveModelSourceBindDraft } from "@/features/admin/model/models-source-binding";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { isAdminLLMSourceAvailable } from "@/features/admin/utils/llm-source-availability";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export type ModelSourceRouteDraft = {
  protocol: AdminLLMAdapter | "";
  priority: string;
  weight: string;
};

export type ModelSourceRouteNumberField = "priority" | "weight";

function routeDraftFromSource(source: AdminLLMModelUpstreamSourceDTO): ModelSourceRouteDraft {
  return {
    protocol: source.protocol,
    priority: String(source.priority),
    weight: String(source.weight),
  };
}

type UseAdminModelsSourcesOptions = {
  model: AdminLLMModelDTO | null;
  circuitBreakerEnabled: boolean;
  onRefreshModel: () => void;
  onSourceAvailabilityChange?: (modelID: number, previousAvailable: boolean, nextAvailable: boolean) => void;
};

// Paged upstream sources of one model with inline route edits (priority,
// weight, protocol, status, circuit) applied optimistically.
export function useAdminModelsSources({
  model,
  circuitBreakerEnabled,
  onRefreshModel,
  onSourceAvailabilityChange,
}: UseAdminModelsSourcesOptions) {
  const t = useTranslations("adminModels.sources");
  const toastT = useTranslations("adminModels.toast");
  const [sources, setSources] = React.useState<AdminLLMModelUpstreamSourceDTO[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(25);
  const [actionSourceID, setActionSourceID] = React.useState<number | null>(null);
  const [routeDrafts, setRouteDrafts] = React.useState<Record<number, ModelSourceRouteDraft>>({});
  const [bindPending, setBindPending] = React.useState(false);

  React.useEffect(() => {
    if (circuitBreakerEnabled) return;
    setSources((current) =>
      current.map((source) => ({
        ...source,
        circuitOpen: false,
        circuitUntil: "",
        circuitScope: "" as const,
      })),
    );
  }, [circuitBreakerEnabled]);

  const loadSources = React.useCallback(
    async (modelId: number, nextPage = 1, nextPageSize = pageSize) => {
      setLoading(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
          return;
        }
        const data = await listAdminLLMModelUpstreamSources(token, modelId, {
          page: nextPage,
          pageSize: nextPageSize,
        });
        setSources(data.results);
        setRouteDrafts(Object.fromEntries(data.results.map((item) => [item.id, routeDraftFromSource(item)])));
        setTotal(data.total);
        setPage(nextPage);
        setPageSize(nextPageSize);
      } catch (error) {
        toast.error(toastT("sourcesLoadFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        setLoading(false);
      }
    },
    [pageSize, toastT],
  );

  React.useEffect(() => {
    if (model) {
      setSources([]);
      setTotal(0);
      setPage(1);
      setActionSourceID(null);
      setRouteDrafts({});
      void loadSources(model.id, 1);
      return;
    }

    setActionSourceID(null);
  }, [loadSources, model]);

  const setRouteDraft = React.useCallback(
    <K extends keyof ModelSourceRouteDraft>(sourceID: number, field: K, value: ModelSourceRouteDraft[K]) => {
      setRouteDrafts((prev) => ({
        ...prev,
        [sourceID]: {
          protocol: prev[sourceID]?.protocol ?? "",
          priority: prev[sourceID]?.priority ?? "",
          weight: prev[sourceID]?.weight ?? "",
          [field]: value,
        },
      }));
    },
    [],
  );

  const handleRouteValueCommit = React.useCallback(
    async (source: AdminLLMModelUpstreamSourceDTO, field: ModelSourceRouteNumberField) => {
      if (!model) return;

      const raw = routeDrafts[source.id]?.[field] ?? String(source[field]);
      const value = Number(raw);
      if (!Number.isInteger(value) || value <= 0) {
        toast.error(field === "priority" ? t("priorityMustBePositive") : t("weightMustBePositive"));
        setRouteDraft(source.id, field, String(source[field]));
        return;
      }
      if (value === source[field]) {
        return;
      }

      const token = await resolveAccessToken();
      if (!token) {
        toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
        return;
      }

      const previousSource = source;
      const nextSource = { ...source, [field]: value };
      setActionSourceID(source.id);
      setSources((current) => current.map((item) => (item.id === source.id ? nextSource : item)));
      setRouteDraft(source.id, field, String(value));
      try {
        const data = await updateAdminLLMModelUpstreamSource(
          token,
          model.id,
          source.id,
          field === "priority" ? { priority: value } : { weight: value },
        );
        setSources((current) => current.map((item) => (item.id === source.id ? data.source : item)));
        setRouteDrafts((current) => ({
          ...current,
          [source.id]: routeDraftFromSource(data.source),
        }));
        toast.success(field === "priority" ? t("priorityUpdated") : t("weightUpdated"));
      } catch (error) {
        setSources((current) => current.map((item) => (item.id === source.id ? previousSource : item)));
        setRouteDraft(source.id, field, String(source[field]));
        toast.error(toastT("routeUpdateFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        setActionSourceID(null);
      }
    },
    [model, routeDrafts, setRouteDraft, t, toastT],
  );

  const handleProtocolChange = React.useCallback(
    async (source: AdminLLMModelUpstreamSourceDTO, protocol: AdminLLMAdapter) => {
      if (!model || protocol === source.protocol) return;

      const token = await resolveAccessToken();
      if (!token) {
        toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
        return;
      }

      const previousSource = source;
      const nextSource = { ...source, protocol };
      setActionSourceID(source.id);
      setSources((current) => current.map((item) => (item.id === source.id ? nextSource : item)));
      setRouteDraft(source.id, "protocol", protocol);
      try {
        const data = await updateAdminLLMModelUpstreamSource(token, model.id, source.id, { protocol });
        setSources((current) => current.map((item) => (item.id === source.id ? data.source : item)));
        setRouteDrafts((current) => ({
          ...current,
          [source.id]: routeDraftFromSource(data.source),
        }));
        toast.success(t("protocolUpdated"));
        onRefreshModel();
      } catch (error) {
        setSources((current) => current.map((item) => (item.id === source.id ? previousSource : item)));
        setRouteDraft(source.id, "protocol", source.protocol);
        toast.error(toastT("routeUpdateFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        setActionSourceID(null);
      }
    },
    [model, onRefreshModel, setRouteDraft, t, toastT],
  );

  const handleToggleStatus = React.useCallback(
    async (source: AdminLLMModelUpstreamSourceDTO, nextStatus: AdminLLMStatus) => {
      if (!model) return;

      const token = await resolveAccessToken();
      if (!token) {
        toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
        return;
      }

      const previousSource = source;
      const nextSource = { ...source, status: nextStatus };
      const previousAvailable = isAdminLLMSourceAvailable(source, model.status);
      const nextAvailable = isAdminLLMSourceAvailable(nextSource, model.status);
      setActionSourceID(source.id);
      setSources((current) => current.map((item) => (item.id === source.id ? nextSource : item)));
      onSourceAvailabilityChange?.(model.id, previousAvailable, nextAvailable);
      try {
        const data = await updateAdminLLMModelUpstreamSource(token, model.id, source.id, {
          status: nextStatus,
        });
        setSources((current) => current.map((item) => (item.id === source.id ? data.source : item)));
        toast.success(nextStatus === "active" ? toastT("sourceEnabled") : toastT("sourceDisabled"));
        onRefreshModel();
      } catch (error) {
        setSources((current) => current.map((item) => (item.id === source.id ? previousSource : item)));
        onSourceAvailabilityChange?.(model.id, nextAvailable, previousAvailable);
        toast.error(toastT("sourceStatusUpdateFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        setActionSourceID(null);
      }
    },
    [model, onRefreshModel, onSourceAvailabilityChange, toastT],
  );

  const handleCircuitAction = React.useCallback(
    async (source: AdminLLMModelUpstreamSourceDTO, action: "open" | "reset") => {
      if (!model) return;

      const token = await resolveAccessToken();
      if (!token) {
        toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
        return;
      }

      const previousSource = source;
      const nextSource =
        action === "open"
          ? {
              ...source,
              circuitOpen: true,
              circuitUntil: String(Math.floor(Date.now() / 1000) + 24 * 60 * 60),
              circuitScope: "source" as const,
            }
          : { ...source, circuitOpen: false, circuitUntil: "", circuitScope: "" as const };
      const previousAvailable = isAdminLLMSourceAvailable(source, model.status);
      const nextAvailable = isAdminLLMSourceAvailable(nextSource, model.status);
      setActionSourceID(source.id);
      setSources((current) => current.map((item) => (item.id === source.id ? nextSource : item)));
      onSourceAvailabilityChange?.(model.id, previousAvailable, nextAvailable);
      try {
        if (action === "open") {
          await openAdminLLMUpstreamModelCircuit(token, source.upstreamID, source.id);
          toast.success(toastT("circuitOpened"));
        } else if (source.circuitScope === "upstream") {
          await resetAdminLLMUpstreamCircuit(token, source.upstreamID);
          toast.success(toastT("circuitReset"));
        } else {
          await resetAdminLLMUpstreamModelCircuit(token, source.upstreamID, source.id);
          toast.success(toastT("circuitReset"));
        }
        onRefreshModel();
      } catch (error) {
        setSources((current) => current.map((item) => (item.id === source.id ? previousSource : item)));
        onSourceAvailabilityChange?.(model.id, nextAvailable, previousAvailable);
        toast.error(toastT("operationFailed"), { description: resolveAdminErrorMessage(error) });
      } finally {
        setActionSourceID(null);
      }
    },
    [model, onRefreshModel, onSourceAvailabilityChange, toastT],
  );

  /** Resolves `true` once saved so the caller can close the circuit dialog. */
  const saveCircuitSettings = React.useCallback(async (
    circuitSource: AdminLLMModelUpstreamSourceDTO,
    payload: UpdateAdminLLMModelUpstreamSourceRequest,
  ): Promise<boolean> => {
    if (!model) return false;

    const token = await resolveAccessToken();
    if (!token) {
      toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
      return false;
    }

    setActionSourceID(circuitSource.id);
    try {
      const data = await updateAdminLLMModelUpstreamSource(token, model.id, circuitSource.id, payload);
      setSources((current) => current.map((item) => (item.id === circuitSource.id ? data.source : item)));
      toast.success(t("circuitUpdated"));
      return true;
    } catch (error) {
      toast.error(toastT("routeUpdateFailed"), { description: resolveAdminErrorMessage(error) });
      return false;
    } finally {
      setActionSourceID(null);
    }
  }, [model, t, toastT]);

  /** Drops a route deleted elsewhere (e.g. from the probe dialog) from the current page. */
  const removeSource = React.useCallback((routeID: number) => {
    setSources((current) => current.filter((item) => item.id !== routeID));
    setTotal((current) => Math.max(0, current - 1));
  }, []);

  /** `onBound` runs right after the success toast, before the list reloads. */
  const submitBind = React.useCallback(async (bindForm: ModelSourceBindDraft, onBound: () => void) => {
    if (!model || bindPending) return;
    const resolvedDraft = resolveModelSourceBindDraft(bindForm);
    if (resolvedDraft.status !== "valid") {
      const error = resolvedDraft.status === "empty" ? "required" : resolvedDraft.error;
      const messageKey = {
        required: "bindRequired",
        protocolRequired: "bindProtocolRequired",
        priorityMustBePositive: "priorityMustBePositive",
        weightMustBePositive: "weightMustBePositive",
        duplicate: "bindDuplicateSource",
      }[error];
      toast.error(toastT(messageKey));
      return;
    }

    const token = await resolveAccessToken();
    if (!token) {
      toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
      return;
    }

    setBindPending(true);
    try {
      await bindAdminLLMModelUpstreamSource(token, model.id, resolvedDraft.payload);
      toast.success(toastT("sourceBound"));
      onBound();
      await loadSources(model.id, 1, pageSize);
      onRefreshModel();
    } catch (error) {
      toast.error(toastT("sourceBindFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setBindPending(false);
    }
  }, [bindPending, loadSources, model, onRefreshModel, pageSize, toastT]);

  return {
    sources,
    loading,
    total,
    page,
    pageSize,
    actionSourceID,
    routeDrafts,
    bindPending,
    loadSources,
    setRouteDraft,
    handleRouteValueCommit,
    handleProtocolChange,
    handleToggleStatus,
    handleCircuitAction,
    saveCircuitSettings,
    removeSource,
    submitBind,
  };
}
