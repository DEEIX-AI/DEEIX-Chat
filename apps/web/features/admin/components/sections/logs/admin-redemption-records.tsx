"use client";

import { X } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import * as React from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import type { AdminRedemptionRecordDTO } from "@/features/admin/api/admin.types";
import { AdminDateRangeFilter } from "@/features/admin/components/admin-date-range-filter";
import {
  REDEMPTION_SORT_OPTIONS,
  type RedemptionSortValue,
  useAdminRedemptions,
} from "@/features/admin/hooks/use-admin-logs";
import { formatDateTime, resolveUserDisplayName } from "@/features/admin/model/log-display";
import { formatTooltipUsageCost, formatUsageBalance } from "@/features/admin/model/usage-log-billing";
import type { BillingDisplayOptions } from "@/shared/lib/billing-display";

// nanousd 转 USD，供余额展示复用现有格式化函数。
function nanousdToUSD(value: number | null | undefined): number | null {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return null;
  }
  return value / 1_000_000_000;
}

function RedemptionIDFilter({
  value,
  placeholder,
  clearLabel,
  onValueChange,
}: {
  value: number;
  placeholder: string;
  clearLabel: string;
  onValueChange: (value: number) => void;
}) {
  const [draft, setDraft] = React.useState(value > 0 ? String(value) : "");

  React.useEffect(() => {
    setDraft(value > 0 ? String(value) : "");
  }, [value]);

  const commit = React.useCallback(() => {
    const parsed = /^\d+$/.test(draft) ? Number.parseInt(draft, 10) : 0;
    const nextValue = Number.isSafeInteger(parsed) && parsed > 0 ? parsed : 0;

    setDraft(nextValue > 0 ? String(nextValue) : "");
    if (nextValue !== value) {
      onValueChange(nextValue);
    }
  }, [draft, onValueChange, value]);

  const clear = React.useCallback(() => {
    setDraft("");
    if (value > 0) {
      onValueChange(0);
    }
  }, [onValueChange, value]);

  return (
    <div className="relative">
      <Input
        value={draft}
        inputMode="numeric"
        pattern="[0-9]*"
        placeholder={placeholder}
        className="h-7 pr-7 pl-2 text-[11px] tabular-nums"
        onChange={(event) => setDraft(event.target.value.replace(/\D/g, ""))}
        onBlur={commit}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            event.preventDefault();
            event.currentTarget.blur();
          }
        }}
      />
      {draft ? (
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          className="absolute top-1/2 right-1 h-5 w-5 -translate-y-1/2 text-muted-foreground shadow-none"
          onMouseDown={(event) => event.preventDefault()}
          onClick={clear}
          aria-label={clearLabel}
        >
          <X className="size-3 stroke-1" />
        </Button>
      ) : null}
    </div>
  );
}

export function RedemptionRecordTable({
  billingDisplay,
  initialCodeID,
  onOpenDetail,
}: {
  billingDisplay: BillingDisplayOptions;
  initialCodeID?: number;
  onOpenDetail: (item: AdminRedemptionRecordDTO) => void;
}) {
  const locale = useLocale();
  const t = useTranslations("adminLogs");
  const logs = useAdminRedemptions(initialCodeID);
  const virtualRows = useVirtualTableRows(logs.records, {
    enabled: logs.records.length > 100,
    estimateSize: 40,
  });
  const rewardLabel = React.useCallback(
    (item: AdminRedemptionRecordDTO) => {
      if (item.rewardType === "subscription") {
        const planName = item.planName.trim() || t("redemptions.rewards.subscription");
        return item.durationDays > 0
          ? `${planName} · ${t("redemptions.rewards.durationDays", { count: item.durationDays })}`
          : planName;
      }
      return `${t("redemptions.rewards.balance")} +${formatTooltipUsageCost(item.creditUSD, billingDisplay)}`;
    },
    [billingDisplay, t],
  );

  return (
    <div className="space-y-3">
      <TableToolbar
        query={logs.query}
        onQueryChange={logs.setQuery}
        queryPlaceholder={t("redemptions.searchPlaceholder")}
        filters={[
          {
            key: "reward_type",
            label: t("redemptions.filters.rewardType"),
            value: logs.rewardTypeFilter,
            onValueChange: logs.setRewardTypeFilter,
            options: [
              { label: t("redemptions.filters.all"), value: "" },
              { label: t("redemptions.rewards.balance"), value: "balance" },
              { label: t("redemptions.rewards.subscription"), value: "subscription" },
            ],
          },
          {
            key: "code_id",
            label: t("redemptions.filters.codeID"),
            active: logs.codeIDFilter > 0,
            content: (
              <RedemptionIDFilter
                value={logs.codeIDFilter}
                placeholder={t("redemptions.filters.codeIDPlaceholder")}
                clearLabel={t("redemptions.filters.clearCodeID")}
                onValueChange={logs.setCodeIDFilter}
              />
            ),
          },
          {
            key: "user_id",
            label: t("redemptions.filters.userID"),
            active: logs.userIDFilter > 0,
            content: (
              <RedemptionIDFilter
                value={logs.userIDFilter}
                placeholder={t("redemptions.filters.userIDPlaceholder")}
                clearLabel={t("redemptions.filters.clearUserID")}
                onValueChange={logs.setUserIDFilter}
              />
            ),
          },
          {
            key: "created_range",
            label: t("filters.timeRange"),
            active: Boolean(logs.createdFromFilter || logs.createdToFilter),
            content: (
              <AdminDateRangeFilter
                fromValue={logs.createdFromFilter}
                toValue={logs.createdToFilter}
                onFromChange={logs.setCreatedFromFilter}
                onToChange={logs.setCreatedToFilter}
                disabled={logs.loading}
              />
            ),
          },
        ]}
        sort={{
          value: logs.sortValue,
          onValueChange: (value) => logs.setSortValue(value as RedemptionSortValue),
          options: REDEMPTION_SORT_OPTIONS.map((item) => ({ label: t(item.labelKey), value: item.value })),
        }}
        loading={logs.loading}
        onRefresh={() => void logs.loadRedemptions(logs.page, logs.pageSize)}
      />

      <Table
        viewportRef={virtualRows.viewportRef}
        viewportClassName={virtualRows.viewportClassName}
        viewportStyle={virtualRows.viewportStyle}
      >
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="w-[72px]">ID</TableHead>
            <TableHead>{t("columns.user")}</TableHead>
            <TableHead>{t("columns.code")}</TableHead>
            <TableHead>{t("columns.reward")}</TableHead>
            <TableHead>{t("columns.balanceAfterRedemption")}</TableHead>
            <TableHead>{t("columns.time")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.loading && logs.records.length === 0 ? <TableLoadingRow colSpan={6} /> : null}
          {logs.records.length > 0 ? <VirtualTablePaddingRow colSpan={6} height={virtualRows.paddingTop} /> : null}
          {logs.records.length > 0 ? virtualRows.rows.map(({ item }) => {
            const balanceAfterUSD = nanousdToUSD(item.balanceAfterNanousd);
            return (
              <TableRow key={item.id} className="cursor-pointer" onClick={() => onOpenDetail(item)}>
                <TableCell className="font-mono text-xs text-foreground">{item.id}</TableCell>
                <TableCell className="whitespace-nowrap text-muted-foreground">
                  {resolveUserDisplayName(item.userLabel, item.username, item.userID)}
                </TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">
                  <div
                    className="max-w-[13rem] truncate"
                    title={item.codeDescription ? `${item.codeHint} · ${item.codeDescription}` : item.codeHint || "-"}
                  >
                    {item.codeHint || "-"}
                  </div>
                </TableCell>
                <TableCell className="whitespace-nowrap">
                  {rewardLabel(item)}
                </TableCell>
                <TableCell className="whitespace-nowrap font-medium tabular-nums text-foreground">
                  {balanceAfterUSD === null ? "-" : formatUsageBalance(balanceAfterUSD, billingDisplay)}
                </TableCell>
                <TableCell className="whitespace-nowrap text-muted-foreground">{formatDateTime(item.createdAt, locale)}</TableCell>
              </TableRow>
            );
          }) : null}
          {logs.records.length > 0 ? <VirtualTablePaddingRow colSpan={6} height={virtualRows.paddingBottom} /> : null}
          {!logs.loading && logs.records.length === 0 ? <TableEmptyRow colSpan={6}>{t("redemptions.empty")}</TableEmptyRow> : null}
        </TableBody>
      </Table>

      <TablePagination
        loading={logs.loading}
        page={logs.page}
        pageCount={logs.pageCount}
        pageSize={logs.pageSize}
        total={logs.total}
        onPageChange={(nextPage) => void logs.loadRedemptions(nextPage, logs.pageSize)}
        onPageSizeChange={(nextPageSize) => void logs.loadRedemptions(1, nextPageSize)}
      />
    </div>
  );
}
