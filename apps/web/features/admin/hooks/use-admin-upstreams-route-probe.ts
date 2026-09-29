"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { deleteAdminLLMUpstreamModel, testAdminLLMUpstreamModelRoute } from "@/features/admin/api";
import type { AdminLLMModelProbeResult, AdminLLMUpstreamView } from "@/features/admin/api/llm-types";
import type { RowDraft } from "@/features/admin/model/upstreams-models";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type UseAdminUpstreamsRouteProbeOptions = {
  upstream: AdminLLMUpstreamView | null;
  /** Runs after a probed route was deleted and the success toast was shown. */
  onRouteDeleted: (result: AdminLLMModelProbeResult, upstream: AdminLLMUpstreamView) => void;
};

// Probe dialog state for single routes in the upstream models dialog.
export function useAdminUpstreamsRouteProbe({ upstream, onRouteDeleted }: UseAdminUpstreamsRouteProbeOptions) {
  const t = useTranslations("adminUpstreams");
  const modelT = useTranslations("adminModels");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [probeOpen, setProbeOpen] = React.useState(false);
  const [probeLoading, setProbeLoading] = React.useState(false);
  const [probeTargetName, setProbeTargetName] = React.useState("");
  const [probeResults, setProbeResults] = React.useState<AdminLLMModelProbeResult[]>([]);
  const upstreamID = upstream?.id ?? null;
  const upstreamStatus = upstream?.status;

  const testRoute = React.useCallback(
    async (row: RowDraft, routeID: number) => {
      if (!upstreamID || routeID <= 0 || row.upstreamModelStatus === "inactive" || upstreamStatus === "inactive") return;
      setProbeTargetName(`${row.platformModelNameDraft || row.platformModelName} / ${row.upstreamModelName}`);
      setProbeResults([]);
      setProbeOpen(true);
      setProbeLoading(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(modelT("toast.sessionExpired"), { description: modelT("toast.signInAgain") });
          setProbeOpen(false);
          return;
        }
        setProbeResults([await testAdminLLMUpstreamModelRoute(token, upstreamID, routeID)]);
      } catch (error) {
        toast.error(t("toast.operationFailed"), { description: resolveErrorMessage(error) });
        setProbeOpen(false);
      } finally {
        setProbeLoading(false);
      }
    },
    [modelT, resolveErrorMessage, t, upstreamID, upstreamStatus],
  );

  // Rethrows so the probe dialog can keep its per-row pending state accurate.
  const deleteProbeRoute = React.useCallback(
    async (result: AdminLLMModelProbeResult) => {
      if (!upstream) {
        return;
      }
      try {
        const token = await resolveAccessToken();
        await deleteAdminLLMUpstreamModel(token, result.upstreamID, result.routeID);
        const nextResults = probeResults.filter((item) => item.routeID !== result.routeID);
        setProbeResults(nextResults);
        if (nextResults.length === 0) {
          setProbeOpen(false);
        }
        toast.success(modelT("toast.sourceDeleted"));
        onRouteDeleted(result, upstream);
      } catch (error) {
        toast.error(modelT("toast.sourceDeleteFailed"), { description: resolveErrorMessage(error) });
        throw error;
      }
    },
    [modelT, onRouteDeleted, probeResults, resolveErrorMessage, upstream],
  );

  return {
    probeOpen,
    setProbeOpen,
    probeLoading,
    probeTargetName,
    probeResults,
    testRoute,
    deleteProbeRoute,
  };
}
