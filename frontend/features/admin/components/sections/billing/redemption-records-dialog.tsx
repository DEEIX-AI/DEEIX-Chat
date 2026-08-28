"use client";

import { useLocale, useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableEmptyRow,
  TableHead,
  TableHeader,
  TableLoadingRow,
  TableRow,
} from "@/components/ui/table";
import { TablePagination } from "@/components/ui/table-tools";
import { listAdminRedemptions } from "@/features/admin/api";
import type { AdminRedemptionRecordDTO } from "@/features/admin/api/admin.types";
import type { AdminRedemptionCodeDTO } from "@/features/admin/api/billing.types";
import { formatCreditUSD, formatDateTime } from "@/features/admin/model/billing-settings";
import { resolveUserDisplayName } from "@/features/admin/model/log-display";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const RECORDS_PAGE_SIZE = 10;

function RedemptionRecordsDialogBody({ code }: { code: AdminRedemptionCodeDTO }) {
  const locale = useLocale();
  const tLogs = useTranslations("adminLogs");
  const [records, setRecords] = React.useState<AdminRedemptionRecordDTO[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(RECORDS_PAGE_SIZE);
  const [loading, setLoading] = React.useState(true);

  const loadRecords = React.useCallback(async (nextPage = 1, nextPageSize = RECORDS_PAGE_SIZE) => {
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
        codeID: code.id,
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
  }, [code.id, tLogs]);

  React.useEffect(() => {
    void loadRecords(1);
  }, [loadRecords]);

  const rewardLabel = (item: AdminRedemptionRecordDTO): string => {
    if (item.rewardType === "subscription") {
      const planName = item.planName.trim() || tLogs("redemptions.rewards.subscription");
      return item.durationDays > 0
        ? `${planName} · ${tLogs("redemptions.rewards.durationDays", { count: item.durationDays })}`
        : planName;
    }
    return `${tLogs("redemptions.rewards.balance")} +${formatCreditUSD(item.creditUSD)}`;
  };

  return (
    <div className="space-y-3">
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead>{tLogs("columns.user")}</TableHead>
            <TableHead>{tLogs("columns.reward")}</TableHead>
            <TableHead>{tLogs("columns.balanceAfterRedemption")}</TableHead>
            <TableHead>{tLogs("columns.time")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading && records.length === 0 ? <TableLoadingRow colSpan={4} /> : null}
          {records.map((item) => (
            <TableRow key={item.id}>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {resolveUserDisplayName(item.userLabel, item.username, item.userID)}
              </TableCell>
              <TableCell className="whitespace-nowrap">{rewardLabel(item)}</TableCell>
              <TableCell className="whitespace-nowrap font-medium tabular-nums text-foreground">
                {typeof item.balanceAfterNanousd === "number"
                  ? formatCreditUSD(item.balanceAfterNanousd / 1_000_000_000)
                  : "-"}
              </TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {formatDateTime(item.createdAt, locale)}
              </TableCell>
            </TableRow>
          ))}
          {!loading && records.length === 0 ? (
            <TableEmptyRow colSpan={4}>{tLogs("redemptions.empty")}</TableEmptyRow>
          ) : null}
        </TableBody>
      </Table>
      {total > pageSize ? (
        <TablePagination
          loading={loading}
          page={page}
          pageCount={Math.max(1, Math.ceil(total / pageSize))}
          pageSize={pageSize}
          total={total}
          onPageChange={(nextPage) => void loadRecords(nextPage, pageSize)}
          onPageSizeChange={(nextPageSize) => void loadRecords(1, nextPageSize)}
        />
      ) : null}
    </div>
  );
}

export function RedemptionRecordsDialog({
  code,
  onClose,
}: {
  code: AdminRedemptionCodeDTO | null;
  onClose: () => void;
}) {
  const tLogs = useTranslations("adminLogs");

  return (
    <Dialog open={Boolean(code)} onOpenChange={(open) => !open && onClose()}>
      {code ? (
        <DialogContent className="flex max-h-[min(86vh,720px)] max-w-2xl flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
          <DialogHeader className="shrink-0 px-4 py-4">
            <DialogTitle>{tLogs("tabs.redemptions")}</DialogTitle>
            <DialogDescription className="flex flex-wrap items-center gap-x-2 gap-y-1">
              <span className="font-mono text-foreground/80">{code.codeHint}</span>
              {code.description ? <span className="truncate">· {code.description}</span> : null}
            </DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-4">
            <RedemptionRecordsDialogBody key={code.id} code={code} />
          </div>
        </DialogContent>
      ) : null}
    </Dialog>
  );
}
