"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { getAdminOpenRouterOfficialPricing } from "@/features/admin/api";
import type { OfficialPricingCatalogItem } from "@/features/admin/model/official-pricing";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type RefreshOfficialPricingCatalogOptions = {
  quiet?: boolean;
  refresh?: boolean;
};

export function useAdminBillingOfficialPricingCatalog() {
  const t = useTranslations("adminBilling");
  const [catalog, setCatalog] = React.useState<OfficialPricingCatalogItem[]>([]);
  const [stale, setStale] = React.useState(true);
  const [loading, setLoading] = React.useState(false);
  const [hasError, setHasError] = React.useState(false);

  async function refresh(options: RefreshOfficialPricingCatalogOptions = {}) {
    setLoading(true);
    setHasError(false);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.sessionExpiredDescription") });
        return;
      }
      const data = await getAdminOpenRouterOfficialPricing(token, { refresh: options.refresh });
      const items = data.items ?? [];
      if (items.length === 0) {
        throw new Error(t("toast.officialPricingRemoteEmpty"));
      }
      setCatalog(items);
      setStale(Boolean(data.stale));
      if (data.stale) {
        setHasError(true);
        if (!options.quiet) {
          toast.error(t("toast.officialPricingRemoteFailed"));
        }
      }
      if (!options.quiet && !data.stale) {
        toast.success(t("toast.officialPricingRemoteLoaded", { count: items.length }));
      }
    } catch (error) {
      setStale(true);
      setHasError(true);
      if (!options.quiet) {
        toast.error(t("toast.officialPricingRemoteFailed"), { description: resolveAdminErrorMessage(error) });
      }
    } finally {
      setLoading(false);
    }
  }

  return { catalog, stale, loading, hasError, refresh };
}
