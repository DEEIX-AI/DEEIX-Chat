"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  deleteAdminLLMUpstreamModel,
  testAdminLLMModelAll,
  testAdminLLMUpstreamModelRoute,
} from "@/features/admin/api";
import type {
  AdminLLMModelDTO,
  AdminLLMModelProbeResult,
  AdminLLMModelUpstreamSourceDTO,
} from "@/features/admin/api/llm-types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type UseAdminModelsProbeOptions = {
  /** Runs after a probed route was deleted and the success toast was shown. */
  onRouteDeleted: (result: AdminLLMModelProbeResult) => void;
};

// Probe dialog state for model/route health checks, shared by the models page
// and the model sources sheet.
export function useAdminModelsProbe({ onRouteDeleted }: UseAdminModelsProbeOptions) {
  const t = useTranslations("adminModels");
  const [probeOpen, setProbeOpen] = React.useState(false);
  const [probeLoading, setProbeLoading] = React.useState(false);
  const [probeTargetName, setProbeTargetName] = React.useState("");
  const [probeResults, setProbeResults] = React.useState<AdminLLMModelProbeResult[]>([]);

  const runProbe = React.useCallback(async (
    targetName: string,
    loader: (token: string) => Promise<AdminLLMModelProbeResult | AdminLLMModelProbeResult[]>,
  ) => {
    setProbeTargetName(targetName);
    setProbeResults([]);
    setProbeOpen(true);
    setProbeLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        setProbeOpen(false);
        return;
      }
      const data = await loader(token);
      setProbeResults(Array.isArray(data) ? data : [data]);
    } catch (error) {
      toast.error(t("toast.operationFailed"), { description: resolveAdminErrorMessage(error) });
      setProbeOpen(false);
    } finally {
      setProbeLoading(false);
    }
  }, [t]);

  const testModel = React.useCallback((item: AdminLLMModelDTO) => {
    void runProbe(item.platformModelName, async (token) => (await testAdminLLMModelAll(token, item.id)).results);
  }, [runProbe]);

  const testSource = React.useCallback((source: AdminLLMModelUpstreamSourceDTO) => {
    const targetName = `${source.upstreamName} / ${source.upstreamModelName}`;
    void runProbe(targetName, (token) => testAdminLLMUpstreamModelRoute(token, source.upstreamID, source.id));
  }, [runProbe]);

  // Rethrows so the probe dialog can keep its per-row pending state accurate.
  const deleteProbeRoute = React.useCallback(async (result: AdminLLMModelProbeResult) => {
    const token = await resolveAccessToken();
    if (!token) {
      toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
      throw new Error("session expired");
    }
    try {
      await deleteAdminLLMUpstreamModel(token, result.upstreamID, result.routeID);
      const nextResults = probeResults.filter((item) => item.routeID !== result.routeID);
      setProbeResults(nextResults);
      if (nextResults.length === 0) {
        setProbeOpen(false);
      }
      toast.success(t("toast.sourceDeleted"));
      onRouteDeleted(result);
    } catch (error) {
      toast.error(t("toast.sourceDeleteFailed"), { description: resolveAdminErrorMessage(error) });
      throw error;
    }
  }, [onRouteDeleted, probeResults, t]);

  const clearProbeResults = React.useCallback(() => {
    setProbeResults([]);
  }, []);

  return {
    probeOpen,
    setProbeOpen,
    probeLoading,
    probeTargetName,
    probeResults,
    testModel,
    testSource,
    deleteProbeRoute,
    clearProbeResults,
  };
}
