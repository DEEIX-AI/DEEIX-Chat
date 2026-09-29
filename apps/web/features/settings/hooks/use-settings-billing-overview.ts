import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import type { UserDTO } from "@/shared/api/auth-types";
import {
  getBillingConfig,
  getBillingOverview,
  listBillingDailyUsage,
  listBillingMonthlyUsage,
  listBillingPlans,
} from "@/shared/api/billing";
import type {
  BillingConfigData,
  BillingOverviewData,
  BillingPlanDTO,
  BillingUsageDailyDTO,
  BillingUsageMonthlyDTO,
} from "@/shared/api/billing-types";

export type BillingRuntimeConfig = BillingConfigData["config"];
export type BillingOverview = BillingOverviewData["overview"];

/**
 * Loads everything the subscription page summary and trend need in one batch
 * (config, plans, overview, daily and monthly usage). `viewer` is captured with
 * the batch so plan matching uses the user the data was loaded for.
 */
export function useSettingsBillingOverview(accessToken: string, user: UserDTO | null) {
  const t = useTranslations("settings.subscriptionPage");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [viewer, setViewer] = React.useState<UserDTO | null>(null);
  const [billingPlans, setBillingPlans] = React.useState<BillingPlanDTO[]>([]);
  const [billingConfig, setBillingConfig] = React.useState<BillingRuntimeConfig | null>(null);
  const [billingOverview, setBillingOverview] = React.useState<BillingOverview | null>(null);
  const [dailyUsage, setDailyUsage] = React.useState<BillingUsageDailyDTO[]>([]);
  const [monthlyUsage, setMonthlyUsage] = React.useState<BillingUsageMonthlyDTO[]>([]);
  const [billingLoading, setBillingLoading] = React.useState(true);

  React.useEffect(() => {
    let mounted = true;
    setBillingLoading(true);
    void Promise.all([
      getBillingConfig(accessToken),
      listBillingPlans(accessToken),
      getBillingOverview(accessToken),
      listBillingDailyUsage(accessToken),
      listBillingMonthlyUsage(accessToken, 12),
    ])
      .then(([configData, plans, overviewData, dailyUsageData, monthlyUsageData]) => ({
        viewer: user,
        config: configData.config,
        plans,
        overview: overviewData.overview,
        dailyUsage: dailyUsageData,
        monthlyUsage: monthlyUsageData,
      }))
      .then(({ viewer: nextViewer, config, plans, overview, dailyUsage: nextDailyUsage, monthlyUsage: nextMonthlyUsage }) => {
        if (!mounted) return;
        setViewer(nextViewer);
        setBillingConfig(config);
        setBillingPlans(plans);
        setBillingOverview(overview);
        setDailyUsage(nextDailyUsage ?? []);
        setMonthlyUsage(nextMonthlyUsage ?? []);
      })
      .catch((error) => {
        if (mounted) toast.error(t("toasts.subscriptionLoadFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
      })
      .finally(() => {
        if (mounted) setBillingLoading(false);
      });
    return () => {
      mounted = false;
    };
  }, [accessToken, resolveErrorMessage, t, user]);

  return {
    viewer,
    billingPlans,
    billingConfig,
    billingOverview,
    setBillingOverview,
    dailyUsage,
    monthlyUsage,
    billingLoading,
  };
}
