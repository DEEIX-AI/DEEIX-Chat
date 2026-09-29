"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { listAdminLLMUpstreamModels, listAdminLLMUpstreams } from "@/features/admin/api";
import type { AdminLLMUpstreamModelDTO, AdminLLMUpstreamView } from "@/features/admin/api/llm-types";
import { listAllAdminPages } from "@/features/admin/api/shared";
import { uniqueUpstreamModels } from "@/features/admin/model/models-source-binding";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type UseAdminModelsSourceBindOptionsOptions = {
  bindOpen: boolean;
  upstreamID: string;
};

// Upstream and upstream-model choices for the "bind source" form of the
// model sources sheet. Upstreams load once per sheet; models reload whenever
// the selected upstream changes while the form is open.
export function useAdminModelsSourceBindOptions({ bindOpen, upstreamID }: UseAdminModelsSourceBindOptionsOptions) {
  const toastT = useTranslations("adminModels.toast");
  const [upstreams, setUpstreams] = React.useState<AdminLLMUpstreamView[]>([]);
  const [upstreamsLoading, setUpstreamsLoading] = React.useState(false);
  const [upstreamsLoaded, setUpstreamsLoaded] = React.useState(false);
  const [upstreamModels, setUpstreamModels] = React.useState<AdminLLMUpstreamModelDTO[]>([]);
  const [upstreamModelsLoading, setUpstreamModelsLoading] = React.useState(false);

  const loadUpstreams = React.useCallback(async () => {
    setUpstreamsLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
        return;
      }
      const results = await listAllAdminPages((options) =>
        listAdminLLMUpstreams(token, { ...options, status: "active", sort: "name_asc" }),
      );
      setUpstreams(results);
    } catch (error) {
      toast.error(toastT("upstreamsLoadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setUpstreamsLoaded(true);
      setUpstreamsLoading(false);
    }
  }, [toastT]);

  const loadUpstreamModels = React.useCallback(async (nextUpstreamID: string) => {
    const parsedUpstreamID = Number.parseInt(nextUpstreamID, 10);
    if (!Number.isFinite(parsedUpstreamID) || parsedUpstreamID <= 0) {
      setUpstreamModels([]);
      return;
    }
    setUpstreamModelsLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(toastT("sessionExpired"), { description: toastT("signInAgain") });
        return;
      }
      const results = await listAllAdminPages((options) =>
        listAdminLLMUpstreamModels(token, parsedUpstreamID, {
          ...options,
          upstreamStatus: "active",
          sort: "upstream_asc",
        }),
      );
      setUpstreamModels(uniqueUpstreamModels(results).filter((item) => item.upstreamModelStatus === "active"));
    } catch (error) {
      toast.error(toastT("upstreamModelsLoadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setUpstreamModelsLoading(false);
    }
  }, [toastT]);

  React.useEffect(() => {
    if (bindOpen && !upstreamsLoaded && !upstreamsLoading) {
      void loadUpstreams();
    }
  }, [bindOpen, loadUpstreams, upstreamsLoaded, upstreamsLoading]);

  React.useEffect(() => {
    if (!bindOpen) return;
    void loadUpstreamModels(upstreamID);
  }, [bindOpen, loadUpstreamModels, upstreamID]);

  const clearUpstreamModels = React.useCallback(() => {
    setUpstreamModels([]);
  }, []);

  return {
    upstreams,
    upstreamsLoading,
    upstreamModels,
    upstreamModelsLoading,
    clearUpstreamModels,
  };
}
