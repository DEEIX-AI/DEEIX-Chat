"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { listAdminRedemptions } from "@/features/admin/api";
import type { AdminRedemptionRecordDTO } from "@/features/admin/api/admin-types";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

export const REDEMPTION_RECORDS_PAGE_SIZE = 25;

export function useAdminBillingRedemptionRecords(codeID: number) {
  const tLogs = useTranslations("adminLogs");
  const [records, setRecords] = React.useState<AdminRedemptionRecordDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(REDEMPTION_RECORDS_PAGE_SIZE);
  const [loading, setLoading] = React.useState(true);

  const loadRecords = React.useCallback(async (nextPage = 1, nextPageSize = REDEMPTION_RECORDS_PAGE_SIZE) => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(tLogs("toast.sessionExpired"), { description: tLogs("toast.signInAgain") });
        return;
      }
      const data = await listAdminRedemptions(token, {
        page: nextPage,
        pageSize: nextPageSize,
        codeID,
      });
      setRecords(data.results);
      setTotal(data.total);
      setPage(nextPage);
      setPageSize(nextPageSize);
    } catch (error) {
      toast.error(tLogs("toast.redemptionsLoadFailed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [codeID, tLogs]);

  React.useEffect(() => {
    void loadRecords(1);
  }, [loadRecords]);

  return { records, total, page, pageSize, loading, loadRecords };
}
