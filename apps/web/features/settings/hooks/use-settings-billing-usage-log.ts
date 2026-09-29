import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { listBillingUsage } from "@/shared/api/billing";
import type { BillingUsageLedgerDTO } from "@/shared/api/billing-types";

/**
 * Paginated usage ledger for the subscription page. Filter and page-size
 * changes jump back to page 1; every change reloads through one effect.
 */
export function useSettingsBillingUsageLog(accessToken: string) {
  const t = useTranslations("settings.subscriptionPage");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [usageLedgers, setUsageLedgers] = React.useState<BillingUsageLedgerDTO[]>([]);
  const [usageTotal, setUsageTotal] = React.useState(0);
  const [usagePage, setUsagePage] = React.useState(1);
  const [usagePageSize, setUsagePageSize] = React.useState(25);
  const [usageQuery, setUsageQuery] = React.useState("");
  const [usageStatus, setUsageStatus] = React.useState("");
  const [usageSort, setUsageSort] = React.useState("newest");
  const [usageLoading, setUsageLoading] = React.useState(true);

  const loadUsageLogs = React.useCallback(async (page: number, pageSize: number, query: string, status: string, sort: string) => {
    setUsageLoading(true);
    try {
      const usage = await listBillingUsage(accessToken, { page, pageSize, query, status, sort });
      setUsageLedgers(usage.results ?? []);
      setUsageTotal(usage.total ?? 0);
    } catch (error) {
      toast.error(t("toasts.usageLogLoadFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
    } finally {
      setUsageLoading(false);
    }
  }, [accessToken, resolveErrorMessage, t]);

  React.useEffect(() => {
    void loadUsageLogs(usagePage, usagePageSize, usageQuery, usageStatus, usageSort);
  }, [loadUsageLogs, usagePage, usagePageSize, usageQuery, usageStatus, usageSort]);

  const changeQuery = React.useCallback((value: string) => {
    setUsageQuery(value);
    setUsagePage(1);
  }, []);
  const changeStatus = React.useCallback((value: string) => {
    setUsageStatus(value);
    setUsagePage(1);
  }, []);
  const changeSort = React.useCallback((value: string) => {
    setUsageSort(value);
    setUsagePage(1);
  }, []);
  const changePageSize = React.useCallback((nextPageSize: number) => {
    setUsagePageSize(nextPageSize);
    setUsagePage(1);
  }, []);
  const refresh = React.useCallback(() => {
    void loadUsageLogs(usagePage, usagePageSize, usageQuery, usageStatus, usageSort);
  }, [loadUsageLogs, usagePage, usagePageSize, usageQuery, usageStatus, usageSort]);

  return {
    items: usageLedgers,
    total: usageTotal,
    loading: usageLoading,
    page: usagePage,
    pageSize: usagePageSize,
    query: usageQuery,
    status: usageStatus,
    sort: usageSort,
    setPage: setUsagePage,
    changeQuery,
    changeStatus,
    changeSort,
    changePageSize,
    refresh,
  };
}
