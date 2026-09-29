"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  listAdminLLMModelUpstreamSources,
  listAdminLLMUpstreamModels,
  listAdminLLMUpstreams,
} from "@/features/admin/api";
import type {
  AdminLLMModelDTO,
  AdminLLMModelUpstreamSourceDTO,
  AdminLLMUpstreamModelDTO,
  AdminLLMUpstreamView,
} from "@/features/admin/api/llm-types";
import { listAllAdminPages } from "@/features/admin/api/shared";
import { uniqueUpstreamModels } from "@/features/admin/model/models-source-binding";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type UseAdminModelsSheetSourcesOptions = {
  open: boolean;
  mode: "create" | "edit";
  target: AdminLLMModelDTO | null;
};

// Route data of the model sheet: existing sources when editing, and the
// upstream / upstream-model choices for initial bindings when creating.
export function useAdminModelsSheetSources({ open, mode, target }: UseAdminModelsSheetSourcesOptions) {
  const t = useTranslations("adminModels");
  const [sources, setSources] = React.useState<AdminLLMModelUpstreamSourceDTO[]>([]);
  const [sourcesLoading, setSourcesLoading] = React.useState(false);
  const [upstreams, setUpstreams] = React.useState<AdminLLMUpstreamView[]>([]);
  const [upstreamsLoading, setUpstreamsLoading] = React.useState(false);
  const [upstreamsLoaded, setUpstreamsLoaded] = React.useState(false);
  const [upstreamModelsByID, setUpstreamModelsByID] = React.useState<Record<string, AdminLLMUpstreamModelDTO[]>>({});
  const [upstreamModelsLoadingByID, setUpstreamModelsLoadingByID] = React.useState<Record<string, boolean>>({});

  const loadUpstreams = React.useCallback(async () => {
    setUpstreamsLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }
      const results = await listAllAdminPages((options) =>
        listAdminLLMUpstreams(token, { ...options, status: "active", sort: "name_asc" }),
      );
      setUpstreams(results);
    } catch (error) {
      toast.error(t("toast.upstreamsLoadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setUpstreamsLoaded(true);
      setUpstreamsLoading(false);
    }
  }, [t]);

  const loadUpstreamModels = React.useCallback(async (upstreamID: string) => {
    const parsedUpstreamID = Number.parseInt(upstreamID, 10);
    if (!Number.isFinite(parsedUpstreamID) || parsedUpstreamID <= 0) {
      return;
    }
    if (upstreamModelsByID[upstreamID] || upstreamModelsLoadingByID[upstreamID]) {
      return;
    }
    setUpstreamModelsLoadingByID((current) => ({ ...current, [upstreamID]: true }));
    try {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }
      const results = await listAllAdminPages((options) =>
        listAdminLLMUpstreamModels(token, parsedUpstreamID, {
          ...options,
          upstreamStatus: "active",
          sort: "upstream_asc",
        }),
      );
      const items = uniqueUpstreamModels(results).filter((item) => item.upstreamModelStatus === "active");
      setUpstreamModelsByID((current) => ({ ...current, [upstreamID]: items }));
    } catch (error) {
      toast.error(t("toast.upstreamModelsLoadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setUpstreamModelsLoadingByID((current) => ({ ...current, [upstreamID]: false }));
    }
  }, [t, upstreamModelsByID, upstreamModelsLoadingByID]);

  React.useEffect(() => {
    if (!open) {
      setSources([]);
      setUpstreams([]);
      setUpstreamsLoaded(false);
      setUpstreamModelsByID({});
      setUpstreamModelsLoadingByID({});
      return;
    }

    if (mode === "create" || !target) {
      setSources([]);
      return;
    }

    setSourcesLoading(true);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const data = await listAdminLLMModelUpstreamSources(token, target.id, {
          page: 1,
          pageSize: 100,
        });
        setSources(data.results);
      } catch {
        setSources([]);
      } finally {
        setSourcesLoading(false);
      }
    })();
  }, [mode, open, target]);

  React.useEffect(() => {
    if (open && mode === "create" && !upstreamsLoaded && !upstreamsLoading) {
      void loadUpstreams();
    }
  }, [loadUpstreams, mode, open, upstreamsLoaded, upstreamsLoading]);

  return {
    sources,
    sourcesLoading,
    upstreams,
    upstreamsLoading,
    upstreamModelsByID,
    upstreamModelsLoadingByID,
    loadUpstreamModels,
  };
}
