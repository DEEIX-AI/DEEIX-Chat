"use client";

import { Trash2 } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import * as React from "react";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { SpinnerLabel } from "@/components/ui/spinner";
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import type {
  AdminAuditLogDTO,
  AdminConversationEventDTO,
  AdminPaymentOrderDTO,
  AdminUsageLogDTO,
  AdminUserAuthEventDTO,
} from "@/features/admin/api/admin.types";
import { type AdminLogCleanupType } from "@/features/admin/api/audit";
import { AdminDateRangeFilter } from "@/features/admin/components/admin-date-range-filter";
import { AdminDateTimePicker } from "@/features/admin/components/admin-date-time-picker";
import { LogDetailSheet } from "@/features/admin/components/sections/logs/admin-log-detail-sheet";
import { ModerationEventTable } from "@/features/admin/components/sections/logs/admin-moderation-events";
import { RedemptionRecordTable } from "@/features/admin/components/sections/logs/admin-redemption-records";
import {
  UsageLogCostCell,
  UsageLogModelCell,
  UsageLogModelFilter,
  UsageLogUsageCell,
  useUsageBillingLabels,
} from "@/features/admin/components/sections/logs/admin-usage-log-cells";
import {
  AUDIT_LOG_SORT_OPTIONS,
  type AuditLogSortValue,
  CONVERSATION_EVENT_SORT_OPTIONS,
  type ConversationEventSortValue,
  PAYMENT_ORDER_SORT_OPTIONS,
  type PaymentOrderSortValue,
  SECURITY_LOG_SORT_OPTIONS,
  type SecurityLogSortValue,
  USAGE_LOG_SORT_OPTIONS,
  type UsageLogSortValue,
  useAdminConversationEvents,
  useAdminLogs,
  useAdminPaymentOrders,
  useAdminSecurityLogs,
  useAdminUsageLogs,
} from "@/features/admin/hooks/use-admin-logs";
import {
  cleanupDateToISOString,
  useAdminBillingDisplayOptions,
  useAdminConversationRunsCleanup,
  useAdminLogCleanupDialog,
  useAdminLogDetail,
} from "@/features/admin/hooks/use-admin-logs-actions";
import {
  formatCount,
  formatDateTime,
  formatMoneyCents,
  resolveUserDisplayName,
} from "@/features/admin/model/log-display";
import { formatUsageBalance } from "@/features/admin/model/usage-log-billing";
import { cn } from "@/lib/utils";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import type { BillingDisplayOptions } from "@/shared/lib/billing-display";


function AuditLogTable({ onOpenDetail }: { onOpenDetail: (item: AdminAuditLogDTO) => void }) {
  const locale = useLocale();
  const t = useTranslations("adminLogs");
  const logs = useAdminLogs();
  const virtualRows = useVirtualTableRows(logs.auditLogs, {
    enabled: logs.auditLogs.length > 100,
    estimateSize: 40,
  });

  return (
    <div className="space-y-3">
      <TableToolbar
        query={logs.query}
        onQueryChange={logs.setQuery}
        queryPlaceholder={t("audit.searchPlaceholder")}
        filters={[
          {
            key: "resource",
            label: t("columns.resource"),
            value: logs.resourceFilter,
            onValueChange: logs.setResourceFilter,
            options: logs.resourceOptions,
          },
          {
            key: "action",
            label: t("columns.action"),
            value: logs.actionFilter,
            onValueChange: logs.setActionFilter,
            options: logs.actionOptions,
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
          onValueChange: (value) => logs.setSortValue(value as AuditLogSortValue),
          options: AUDIT_LOG_SORT_OPTIONS.map((item) => ({ label: t(item.labelKey), value: item.value })),
        }}
        loading={logs.loading}
        onRefresh={() => void logs.loadAuditLogs(logs.page, logs.pageSize)}
      />

      <Table
        viewportRef={virtualRows.viewportRef}
        viewportClassName={virtualRows.viewportClassName}
        viewportStyle={virtualRows.viewportStyle}
      >
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="w-[72px]">ID</TableHead>
            <TableHead>{t("columns.actor")}</TableHead>
            <TableHead>{t("columns.action")}</TableHead>
            <TableHead>{t("columns.resource")}</TableHead>
            <TableHead>IP</TableHead>
            <TableHead>{t("columns.time")}</TableHead>
            <TableHead>{t("columns.requestID")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.loading && logs.auditLogs.length === 0 ? <TableLoadingRow colSpan={7} /> : null}
          {logs.auditLogs.length > 0 ? <VirtualTablePaddingRow colSpan={7} height={virtualRows.paddingTop} /> : null}
          {logs.auditLogs.length > 0 ? virtualRows.rows.map(({ item }) => (
            <TableRow key={item.id} className="cursor-pointer" onClick={() => onOpenDetail(item)}>
              <TableCell className="font-mono text-xs text-foreground">{item.id}</TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {resolveUserDisplayName(item.actorLabel, item.actorUsername, item.actorUserID)}
              </TableCell>
              <TableCell>
                <div className="max-w-[12rem] truncate" title={item.action || "-"}>{item.action || "-"}</div>
              </TableCell>
              <TableCell>
                <div className="max-w-[14rem] truncate" title={item.resource || "-"}>{item.resource || "-"}</div>
              </TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">{item.ip || "-"}</TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">{formatDateTime(item.createdAt, locale)}</TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">
                <div className="max-w-[14rem] truncate" title={item.requestID || "-"}>{item.requestID || "-"}</div>
              </TableCell>
            </TableRow>
          )) : null}
          {logs.auditLogs.length > 0 ? <VirtualTablePaddingRow colSpan={7} height={virtualRows.paddingBottom} /> : null}
          {!logs.loading && logs.auditLogs.length === 0 ? <TableEmptyRow colSpan={7}>{t("audit.empty")}</TableEmptyRow> : null}
        </TableBody>
      </Table>

      <TablePagination
        loading={logs.loading}
        page={logs.page}
        pageCount={logs.pageCount}
        pageSize={logs.pageSize}
        total={logs.total}
        onPageChange={(nextPage) => void logs.loadAuditLogs(nextPage, logs.pageSize)}
        onPageSizeChange={(nextPageSize) => void logs.loadAuditLogs(1, nextPageSize)}
      />
    </div>
  );
}

function AuthLogTable({ onOpenDetail }: { onOpenDetail: (item: AdminUserAuthEventDTO) => void }) {
  const locale = useLocale();
  const t = useTranslations("adminLogs");
  const logs = useAdminSecurityLogs();
  const virtualRows = useVirtualTableRows(logs.sortedEvents, {
    enabled: logs.sortedEvents.length > 100,
    estimateSize: 40,
  });
  const resultLabel = React.useCallback(
    (value: string) => {
      switch (value) {
        case "success":
          return t("detail.result.success");
        case "failure":
          return t("detail.result.failure");
        case "blocked":
          return t("detail.result.blocked");
        default:
          return value || "-";
      }
    },
    [t],
  );

  return (
    <div className="space-y-3">
      <TableToolbar
        query={logs.query}
        onQueryChange={logs.setQuery}
        queryPlaceholder={t("auth.searchPlaceholder")}
        filters={[
          {
            key: "result",
            label: t("columns.result"),
            value: logs.resultFilter,
            onValueChange: logs.setResultFilter,
            options: [
              { label: t("filters.allResults"), value: "" },
              { label: t("detail.result.success"), value: "success" },
              { label: t("detail.result.failure"), value: "failure" },
              { label: t("detail.result.blocked"), value: "blocked" },
            ],
          },
        ]}
        sort={{
          value: logs.sortValue,
          onValueChange: (value) => logs.setSortValue(value as SecurityLogSortValue),
          options: SECURITY_LOG_SORT_OPTIONS.map((item) => ({ label: t(item.labelKey), value: item.value })),
        }}
        loading={logs.loading}
        onRefresh={() => void logs.loadSecurityLogs(logs.page, logs.pageSize)}
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
            <TableHead>{t("columns.event")}</TableHead>
            <TableHead>{t("columns.result")}</TableHead>
            <TableHead>{t("columns.reason")}</TableHead>
            <TableHead>IP</TableHead>
            <TableHead>{t("columns.time")}</TableHead>
            <TableHead>{t("columns.requestID")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.loading && logs.sortedEvents.length === 0 ? <TableLoadingRow colSpan={8} /> : null}
          {logs.sortedEvents.length > 0 ? <VirtualTablePaddingRow colSpan={8} height={virtualRows.paddingTop} /> : null}
          {logs.sortedEvents.length > 0 ? virtualRows.rows.map(({ item }) => (
            <TableRow key={item.id} className="cursor-pointer" onClick={() => onOpenDetail(item)}>
              <TableCell className="font-mono text-xs text-foreground">{item.id}</TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {resolveUserDisplayName(item.userLabel, item.username, item.userID)}
              </TableCell>
              <TableCell>
                <div className="max-w-[14rem] truncate" title={item.eventType}>{item.eventType || "-"}</div>
              </TableCell>
              <TableCell className="whitespace-nowrap">{resultLabel(item.result)}</TableCell>
              <TableCell className="text-muted-foreground">
                <div className="max-w-[14rem] truncate" title={item.reason || "-"}>{item.reason || "-"}</div>
              </TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">{item.clientIP || "-"}</TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">{formatDateTime(item.occurredAt, locale)}</TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">
                <div className="max-w-[14rem] truncate" title={item.requestID || "-"}>{item.requestID || "-"}</div>
              </TableCell>
            </TableRow>
          )) : null}
          {logs.sortedEvents.length > 0 ? <VirtualTablePaddingRow colSpan={8} height={virtualRows.paddingBottom} /> : null}
          {!logs.loading && logs.sortedEvents.length === 0 ? <TableEmptyRow colSpan={8}>{t("auth.empty")}</TableEmptyRow> : null}
        </TableBody>
      </Table>

      <TablePagination
        loading={logs.loading}
        page={logs.page}
        pageCount={logs.pageCount}
        pageSize={logs.pageSize}
        total={logs.total}
        onPageChange={(nextPage) => void logs.loadSecurityLogs(nextPage, logs.pageSize)}
        onPageSizeChange={(nextPageSize) => void logs.loadSecurityLogs(1, nextPageSize)}
      />
    </div>
  );
}

function UsageLogTable({
  billingDisplay,
  onOpenDetail,
}: {
  billingDisplay: BillingDisplayOptions;
  onOpenDetail: (item: AdminUsageLogDTO) => void;
}) {
  const locale = useLocale();
  const t = useTranslations("adminLogs");
  const usageLabels = useUsageBillingLabels();
  const logs = useAdminUsageLogs();
  const virtualRows = useVirtualTableRows(logs.logs, {
    enabled: logs.logs.length > 100,
    estimateSize: 40,
  });

  return (
    <div className="space-y-3">
      <TableToolbar
        query={logs.query}
        onQueryChange={logs.setQuery}
        queryPlaceholder={t("usage.searchPlaceholder")}
        filters={[
          {
            key: "billing_mode",
            label: t("usage.filters.billingMode"),
            value: logs.billingModeFilter,
            onValueChange: logs.setBillingModeFilter,
            options: [
              { label: t("usage.filters.all"), value: "" },
              { label: usageLabels.freeModelNoBilling, value: "free" },
              { label: t("usage.billingModes.token"), value: "token" },
              { label: t("usage.billingModes.call"), value: "call" },
              { label: t("usage.billingModes.duration"), value: "duration" },
              { label: t("usage.billingModes.tiered"), value: "tiered" },
            ],
          },
          {
            key: "platform_model",
            label: t("usage.filters.model"),
            active: Boolean(logs.platformModelFilter),
            content: (
              <UsageLogModelFilter
                value={logs.platformModelFilter}
                options={logs.platformModelOptions}
                disabled={logs.loading}
                onChange={logs.setPlatformModelFilter}
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
          onValueChange: (value) => logs.setSortValue(value as UsageLogSortValue),
          options: USAGE_LOG_SORT_OPTIONS.map((item) => ({ label: t(item.labelKey), value: item.value })),
        }}
        loading={logs.loading}
        onRefresh={() => void logs.loadUsageLogs(logs.page, logs.pageSize)}
      />

      <Table
        viewportRef={virtualRows.viewportRef}
        viewportClassName={virtualRows.viewportClassName}
        viewportStyle={virtualRows.viewportStyle}
      >
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="w-[72px]">ID</TableHead>
            <TableHead>{t("columns.caller")}</TableHead>
            <TableHead>{t("columns.model")}</TableHead>
            <TableHead>{t("columns.usage")}</TableHead>
            <TableHead>{t("columns.billing")}</TableHead>
            <TableHead>{t("columns.balanceAfter")}</TableHead>
            <TableHead>{t("columns.latency")}</TableHead>
            <TableHead>{t("columns.time")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.loading && logs.logs.length === 0 ? <TableLoadingRow colSpan={8} /> : null}
          {logs.logs.length > 0 ? <VirtualTablePaddingRow colSpan={8} height={virtualRows.paddingTop} /> : null}
          {logs.logs.length > 0 ? virtualRows.rows.map(({ item }) => (
            <TableRow key={item.id} className="cursor-pointer" onClick={() => onOpenDetail(item)}>
              <TableCell className="font-mono text-xs text-foreground">{item.id}</TableCell>
              <TableCell>
                <span className="block max-w-[10rem] truncate whitespace-nowrap text-muted-foreground" title={`${resolveUserDisplayName(item.userLabel, item.username, item.userID)} (#${item.userID})`}>
                  {resolveUserDisplayName(item.userLabel, item.username, item.userID)}
                </span>
              </TableCell>
              <TableCell>
                <UsageLogModelCell item={item} labels={usageLabels} />
              </TableCell>
              <TableCell>
                <UsageLogUsageCell item={item} locale={locale} />
              </TableCell>
              <TableCell><UsageLogCostCell item={item} labels={usageLabels} billingDisplay={billingDisplay} /></TableCell>
              <TableCell className="whitespace-nowrap font-medium tabular-nums text-foreground">
                {formatUsageBalance(item.balanceAfterUSD, billingDisplay)}
              </TableCell>
              <TableCell className="whitespace-nowrap font-mono text-muted-foreground">{formatCount(item.latencyMS, locale)} ms</TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">{formatDateTime(item.createdAt, locale)}</TableCell>
            </TableRow>
          )) : null}
          {logs.logs.length > 0 ? <VirtualTablePaddingRow colSpan={8} height={virtualRows.paddingBottom} /> : null}
          {!logs.loading && logs.logs.length === 0 ? <TableEmptyRow colSpan={8}>{t("usage.empty")}</TableEmptyRow> : null}
        </TableBody>
      </Table>

      <TablePagination
        loading={logs.loading}
        page={logs.page}
        pageCount={logs.pageCount}
        pageSize={logs.pageSize}
        total={logs.total}
        onPageChange={(nextPage) => void logs.loadUsageLogs(nextPage, logs.pageSize)}
        onPageSizeChange={(nextPageSize) => void logs.loadUsageLogs(1, nextPageSize)}
      />
    </div>
  );
}

function PaymentOrderTable({ onOpenDetail }: { onOpenDetail: (item: AdminPaymentOrderDTO) => void }) {
  const locale = useLocale();
  const t = useTranslations("adminLogs");
  const logs = useAdminPaymentOrders();
  const virtualRows = useVirtualTableRows(logs.orders, {
    enabled: logs.orders.length > 100,
    estimateSize: 40,
  });
  const orderTypeLabel = React.useCallback((value: string) => {
    switch (value) {
      case "subscription":
        return t("orders.types.subscription");
      case "topup":
        return t("orders.types.topup");
      default:
        return value || "-";
    }
  }, [t]);
  const orderStatusLabel = React.useCallback((value: string) => {
    switch (value) {
      case "pending":
        return t("orders.status.pending");
      case "paid":
        return t("orders.status.paid");
      case "expired":
        return t("orders.status.expired");
      case "failed":
        return t("orders.status.failed");
      default:
        return value || "-";
    }
  }, [t]);

  return (
    <div className="space-y-3">
      <TableToolbar
        query={logs.query}
        onQueryChange={logs.setQuery}
        queryPlaceholder={t("orders.searchPlaceholder")}
        filters={[
          {
            key: "order_type",
            label: t("orders.filters.orderType"),
            value: logs.orderTypeFilter,
            onValueChange: logs.setOrderTypeFilter,
            options: [
              { label: t("orders.filters.all"), value: "" },
              { label: t("orders.types.subscription"), value: "subscription" },
              { label: t("orders.types.topup"), value: "topup" },
            ],
          },
          {
            key: "provider",
            label: t("orders.filters.provider"),
            value: logs.providerFilter,
            onValueChange: logs.setProviderFilter,
            options: [
              { label: t("orders.filters.all"), value: "" },
              { label: "Stripe", value: "stripe" },
              { label: "EPay", value: "epay" },
            ],
          },
          {
            key: "status",
            label: t("orders.filters.status"),
            value: logs.statusFilter,
            onValueChange: logs.setStatusFilter,
            options: [
              { label: t("orders.filters.all"), value: "" },
              { label: t("orders.status.pending"), value: "pending" },
              { label: t("orders.status.paid"), value: "paid" },
              { label: t("orders.status.expired"), value: "expired" },
              { label: t("orders.status.failed"), value: "failed" },
            ],
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
          onValueChange: (value) => logs.setSortValue(value as PaymentOrderSortValue),
          options: PAYMENT_ORDER_SORT_OPTIONS.map((item) => ({ label: t(item.labelKey), value: item.value })),
        }}
        loading={logs.loading}
        onRefresh={() => void logs.loadPaymentOrders(logs.page, logs.pageSize)}
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
            <TableHead>{t("columns.orderNo")}</TableHead>
            <TableHead>{t("columns.type")}</TableHead>
            <TableHead>{t("columns.provider")}</TableHead>
            <TableHead>{t("columns.status")}</TableHead>
            <TableHead>{t("columns.amount")}</TableHead>
            <TableHead>{t("columns.time")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.loading && logs.orders.length === 0 ? <TableLoadingRow colSpan={8} /> : null}
          {logs.orders.length > 0 ? <VirtualTablePaddingRow colSpan={8} height={virtualRows.paddingTop} /> : null}
          {logs.orders.length > 0 ? virtualRows.rows.map(({ item }) => (
            <TableRow key={item.id} className="cursor-pointer" onClick={() => onOpenDetail(item)}>
              <TableCell className="font-mono text-xs text-foreground">{item.id}</TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {resolveUserDisplayName(item.userLabel, item.username, item.userID)}
              </TableCell>
              <TableCell className="font-mono text-xs text-muted-foreground">
                <div className="max-w-[13rem] truncate" title={item.orderNo || "-"}>{item.orderNo || "-"}</div>
              </TableCell>
              <TableCell className="whitespace-nowrap">{orderTypeLabel(item.orderType)}</TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">{item.provider || "-"}</TableCell>
              <TableCell className="whitespace-nowrap">{orderStatusLabel(item.status)}</TableCell>
              <TableCell className="whitespace-nowrap font-mono text-muted-foreground">{formatMoneyCents(item.payAmountCents, item.payCurrency)}</TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">{formatDateTime(item.createdAt, locale)}</TableCell>
            </TableRow>
          )) : null}
          {logs.orders.length > 0 ? <VirtualTablePaddingRow colSpan={8} height={virtualRows.paddingBottom} /> : null}
          {!logs.loading && logs.orders.length === 0 ? <TableEmptyRow colSpan={8}>{t("orders.empty")}</TableEmptyRow> : null}
        </TableBody>
      </Table>

      <TablePagination
        loading={logs.loading}
        page={logs.page}
        pageCount={logs.pageCount}
        pageSize={logs.pageSize}
        total={logs.total}
        onPageChange={(nextPage) => void logs.loadPaymentOrders(nextPage, logs.pageSize)}
        onPageSizeChange={(nextPageSize) => void logs.loadPaymentOrders(1, nextPageSize)}
      />
    </div>
  );
}

function ConversationEventTable({ onOpenDetail }: { onOpenDetail: (item: AdminConversationEventDTO) => void }) {
  const locale = useLocale();
  const t = useTranslations("adminLogs");
  const commonT = useTranslations("common.actions");
  const logs = useAdminConversationEvents();
  const {
    selectedRunIDs,
    visibleRunIDs,
    allVisibleSelected,
    someVisibleSelected,
    cleanupOpen,
    setCleanupOpen,
    cleanupPending,
    toggleRun,
    toggleVisibleRuns,
    cleanupSelectedRuns,
  } = useAdminConversationRunsCleanup(logs);
  const virtualRows = useVirtualTableRows(logs.events, {
    enabled: logs.events.length > 100,
    estimateSize: 40,
  });
  const scopeLabel = React.useCallback((value: string) => {
    switch (value) {
      case "trace_block":
        return t("conversation.scopes.trace_block");
      case "trace_event":
        return t("conversation.scopes.trace_event");
      case "tool_call":
        return t("conversation.scopes.tool_call");
      default:
        return value || "-";
    }
  }, [t]);
  const eventStatusLabel = React.useCallback((value: string) => {
    switch (value) {
      case "streaming":
        return t("conversation.status.streaming");
      case "completed":
        return t("conversation.status.completed");
      case "error":
        return t("conversation.status.error");
      default:
        return value || "-";
    }
  }, [t]);

  return (
    <div className="space-y-3">
      <TableToolbar
        query={logs.query}
        onQueryChange={logs.setQuery}
        queryPlaceholder={t("conversation.searchPlaceholder")}
        filters={[
          {
            key: "event_scope",
            label: t("conversation.filters.scope"),
            value: logs.eventScopeFilter,
            onValueChange: logs.setEventScopeFilter,
            options: [
              { label: t("conversation.filters.all"), value: "" },
              { label: t("conversation.scopes.trace_block"), value: "trace_block" },
              { label: t("conversation.scopes.trace_event"), value: "trace_event" },
              { label: t("conversation.scopes.tool_call"), value: "tool_call" },
            ],
          },
          {
            key: "event_type",
            label: t("conversation.filters.eventType"),
            value: logs.eventTypeFilter,
            onValueChange: logs.setEventTypeFilter,
            options: [{ label: t("conversation.filters.all"), value: "" }, ...logs.eventTypeOptions],
          },
          {
            key: "status",
            label: t("conversation.filters.status"),
            value: logs.statusFilter,
            onValueChange: logs.setStatusFilter,
            options: [
              { label: t("conversation.filters.all"), value: "" },
              { label: t("conversation.status.streaming"), value: "streaming" },
              { label: t("conversation.status.completed"), value: "completed" },
              { label: t("conversation.status.error"), value: "error" },
            ],
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
          onValueChange: (value) => logs.setSortValue(value as ConversationEventSortValue),
          options: CONVERSATION_EVENT_SORT_OPTIONS.map((item) => ({ label: t(item.labelKey), value: item.value })),
        }}
        selectedCount={selectedRunIDs.size}
        bulkActions={[
          {
            key: "delete-runs",
            label: t("conversation.cleanup.action"),
            icon: <Trash2 />,
            onClick: () => setCleanupOpen(true),
          },
        ]}
        loading={logs.loading}
        onRefresh={() => void logs.loadConversationEvents(logs.page, logs.pageSize)}
      />

      <Table
        viewportRef={virtualRows.viewportRef}
        viewportClassName={virtualRows.viewportClassName}
        viewportStyle={virtualRows.viewportStyle}
      >
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="w-[44px] py-1.5 text-center">
              <div className="flex h-7 items-center justify-center">
                <Checkbox
                  checked={allVisibleSelected ? true : someVisibleSelected ? "indeterminate" : false}
                  disabled={visibleRunIDs.length === 0}
                  onCheckedChange={(checked) => toggleVisibleRuns(checked === true)}
                  aria-label={t("conversation.cleanup.selectAll")}
                />
              </div>
            </TableHead>
            <TableHead className="w-[72px]">ID</TableHead>
            <TableHead>{t("columns.user")}</TableHead>
            <TableHead>{t("columns.scope")}</TableHead>
            <TableHead>{t("columns.event")}</TableHead>
            <TableHead>{t("columns.status")}</TableHead>
            <TableHead>{t("columns.upstream")}</TableHead>
            <TableHead>{t("columns.tool")}</TableHead>
            <TableHead>{t("columns.runID")}</TableHead>
            <TableHead>{t("columns.time")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.loading && logs.events.length === 0 ? <TableLoadingRow colSpan={10} /> : null}
          {logs.events.length > 0 ? <VirtualTablePaddingRow colSpan={10} height={virtualRows.paddingTop} /> : null}
          {logs.events.length > 0 ? virtualRows.rows.map(({ item }) => {
            const runID = item.runID.trim();
            return (
              <TableRow
                key={item.id}
                className="cursor-pointer"
                selected={selectedRunIDs.has(runID)}
                onClick={() => onOpenDetail(item)}
              >
                <TableCell className="w-[44px] py-1.5 text-center">
                  <div className="flex h-7 items-center justify-center">
                    <Checkbox
                      checked={selectedRunIDs.has(runID)}
                      disabled={!runID}
                      onClick={(event) => event.stopPropagation()}
                      onCheckedChange={(checked) => toggleRun(runID, checked === true)}
                      aria-label={t("conversation.cleanup.selectRun", { runID: runID || "-" })}
                    />
                  </div>
                </TableCell>
                <TableCell className="font-mono text-xs text-foreground">{item.id}</TableCell>
                <TableCell className="whitespace-nowrap text-muted-foreground">
                  {resolveUserDisplayName(item.userLabel, item.username, item.userID)}
                </TableCell>
                <TableCell className="whitespace-nowrap text-muted-foreground">{scopeLabel(item.eventScope)}</TableCell>
                <TableCell>
                  <div className="max-w-[12rem] truncate" title={item.eventType || item.title || "-"}>{item.eventType || item.title || "-"}</div>
                </TableCell>
                <TableCell className="whitespace-nowrap">{eventStatusLabel(item.status)}</TableCell>
                <TableCell>
                  <div className="max-w-[12rem] truncate text-muted-foreground" title={item.upstreamName || "-"}>{item.upstreamName || "-"}</div>
                </TableCell>
                <TableCell>
                  <div className="max-w-[10rem] truncate text-muted-foreground" title={item.toolName || "-"}>{item.toolName || "-"}</div>
                </TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">
                  <div className="max-w-[13rem] truncate" title={runID || "-"}>{runID || "-"}</div>
                </TableCell>
                <TableCell className="whitespace-nowrap text-muted-foreground">{formatDateTime(item.createdAt, locale)}</TableCell>
              </TableRow>
            );
          }) : null}
          {logs.events.length > 0 ? <VirtualTablePaddingRow colSpan={10} height={virtualRows.paddingBottom} /> : null}
          {!logs.loading && logs.events.length === 0 ? <TableEmptyRow colSpan={10}>{t("conversation.empty")}</TableEmptyRow> : null}
        </TableBody>
      </Table>

      <TablePagination
        loading={logs.loading}
        page={logs.page}
        pageCount={logs.pageCount}
        pageSize={logs.pageSize}
        total={logs.total}
        onPageChange={(nextPage) => void logs.loadConversationEvents(nextPage, logs.pageSize)}
        onPageSizeChange={(nextPageSize) => void logs.loadConversationEvents(1, nextPageSize)}
      />

      <AlertDialog open={cleanupOpen} onOpenChange={(open) => !cleanupPending && setCleanupOpen(open)}>
        <AlertDialogContent className="sm:max-w-[440px]">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("conversation.cleanup.title")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("conversation.cleanup.description", { count: selectedRunIDs.size })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={cleanupPending}>{commonT("cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={cleanupPending || selectedRunIDs.size === 0}
              onClick={(event) => {
                event.preventDefault();
                void cleanupSelectedRuns();
              }}
            >
              {cleanupPending ? <SpinnerLabel>{t("conversation.cleanup.deleting")}</SpinnerLabel> : t("conversation.cleanup.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

const LOG_CLEANUP_TYPES: AdminLogCleanupType[] = [
  "audit",
  "auth",
  "usage",
  "orders",
  "conversation",
  "system",
];

function LogCleanupDialog({
  open,
  onOpenChange,
  onSuccess,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSuccess: (type: AdminLogCleanupType) => void;
}) {
  const t = useTranslations("adminLogs.cleanup");
  const commonT = useTranslations("common.actions");
  const { logType, setLogType, date, setDate, pending, handleOpenChange, submit } = useAdminLogCleanupDialog({
    onOpenChange,
    onSuccess,
  });
  const highRisk = logType === "usage" || logType === "orders";

  return (
    <AlertDialog open={open} onOpenChange={handleOpenChange}>
      <AlertDialogContent className="sm:max-w-[520px]">
        <AlertDialogHeader>
          <AlertDialogTitle>{t("title")}</AlertDialogTitle>
          <AlertDialogDescription>{t("description")}</AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-4">
          <div className="space-y-1.5">
            <p className="text-xs text-muted-foreground">{t("typeLabel")}</p>
            <Select
              value={logType}
              disabled={pending}
              onValueChange={(value) => setLogType(value as AdminLogCleanupType)}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {LOG_CLEANUP_TYPES.map((value) => (
                  <SelectItem key={value} value={value}>
                    {t(`types.${value}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <AdminDateTimePicker
            value={date}
            disabled={pending}
            label={t("dateLabel")}
            placeholder={t("datePlaceholder")}
            granularity="date"
            disabledDate={{ after: new Date() }}
            onChange={setDate}
          />

          <div className="rounded-md bg-muted/35 px-3 py-2.5 text-xs leading-5">
            <div className="flex items-center gap-2">
              <p className={cn("font-medium", highRisk ? "text-destructive" : "text-foreground/80")}>
                {t("impactTitle")}
              </p>
              {highRisk ? (
                <Badge variant="secondary" className="h-5 rounded-md px-1.5 text-[10px] font-normal text-destructive shadow-none">
                  {t("highRisk")}
                </Badge>
              ) : null}
            </div>
            <p className={cn("mt-1", highRisk ? "text-destructive/85" : "text-muted-foreground")}>
              {t(`impacts.${logType}`)}
            </p>
          </div>

          {date ? (
            <p className="text-xs text-muted-foreground">
              {t("boundary", { date })}
            </p>
          ) : null}
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>{commonT("cancel")}</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={pending || !cleanupDateToISOString(date)}
            onClick={(event) => {
              event.preventDefault();
              void submit();
            }}
          >
            {pending ? <SpinnerLabel>{t("deleting")}</SpinnerLabel> : t("confirm")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

const LOG_TAB_VALUES = new Set(["audit", "usage", "auth", "orders", "redemptions", "conversation"]);

export function AdminLogsPage() {
  const t = useTranslations("adminLogs");
  const { user } = useAuthSession();
  const isSuperAdmin = user?.role === "superadmin";
  const searchParams = useSearchParams();
  // 支持从其他管理页深链跳转（如兑换码管理的“查看兑换记录”）预选 tab 与兑换码筛选。
  const [initialTab] = React.useState(() => {
    const tabParam = searchParams.get("tab") ?? "";
    return LOG_TAB_VALUES.has(tabParam) ? tabParam : "audit";
  });
  const [initialRedemptionCodeID] = React.useState(() => {
    const parsed = Number.parseInt(searchParams.get("code_id") ?? "", 10);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
  });
  const { detail, setDetail, conversationDetailLoading, openConversationDetail, closeDetail } = useAdminLogDetail();
  const [cleanupOpen, setCleanupOpen] = React.useState(false);
  const billingDisplay = useAdminBillingDisplayOptions();
  const [cleanupRevisions, setCleanupRevisions] = React.useState<Record<AdminLogCleanupType, number>>({
    audit: 0,
    auth: 0,
    usage: 0,
    orders: 0,
    conversation: 0,
    system: 0,
  });

  const handleCleanupSuccess = React.useCallback((type: AdminLogCleanupType) => {
    setCleanupRevisions((current) => ({
      ...current,
      [type]: current[type] + 1,
    }));
  }, []);

  return (
    <div className="space-y-5 pb-10">
      <div className="flex h-10 items-center justify-between gap-4 px-1">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold">{t("centerTitle")}</h3>
        </div>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="h-8 gap-1.5 px-2 text-xs text-muted-foreground shadow-none hover:bg-destructive/10 hover:text-destructive"
          onClick={() => setCleanupOpen(true)}
        >
          <Trash2 className="size-3.5 stroke-1" />
          {t("cleanup.trigger")}
        </Button>
      </div>

      <Tabs defaultValue={initialTab} className="space-y-3">
        <TabsList variant="line">
          <TabsTrigger value="audit">{t("tabs.audit")}</TabsTrigger>
          <TabsTrigger value="usage">{t("tabs.usage")}</TabsTrigger>
          <TabsTrigger value="auth">{t("tabs.auth")}</TabsTrigger>
          <TabsTrigger value="orders">{t("tabs.orders")}</TabsTrigger>
          <TabsTrigger value="redemptions">{t("tabs.redemptions")}</TabsTrigger>
          <TabsTrigger value="conversation">{t("tabs.conversation")}</TabsTrigger>
          {isSuperAdmin ? <TabsTrigger value="moderation">{t("tabs.moderation")}</TabsTrigger> : null}
        </TabsList>
        <TabsContent value="audit">
          <AuditLogTable key={cleanupRevisions.audit} onOpenDetail={(item) => setDetail({ kind: "audit", item })} />
        </TabsContent>
        <TabsContent value="auth">
          <AuthLogTable key={cleanupRevisions.auth} onOpenDetail={(item) => setDetail({ kind: "auth", item })} />
        </TabsContent>
        <TabsContent value="usage">
          <UsageLogTable
            key={cleanupRevisions.usage}
            billingDisplay={billingDisplay}
            onOpenDetail={(item) => setDetail({ kind: "usage", item })}
          />
        </TabsContent>
        <TabsContent value="orders">
          <PaymentOrderTable key={cleanupRevisions.orders} onOpenDetail={(item) => setDetail({ kind: "order", item })} />
        </TabsContent>
        <TabsContent value="redemptions">
          <RedemptionRecordTable
            billingDisplay={billingDisplay}
            initialCodeID={initialRedemptionCodeID}
            onOpenDetail={(item) => setDetail({ kind: "redemption", item })}
          />
        </TabsContent>
        <TabsContent value="conversation">
          <ConversationEventTable key={cleanupRevisions.conversation} onOpenDetail={(item) => void openConversationDetail(item)} />
        </TabsContent>
        {isSuperAdmin ? (
          <TabsContent value="moderation">
            <ModerationEventTable />
          </TabsContent>
        ) : null}
      </Tabs>

      <LogDetailSheet
        detail={detail}
        billingDisplay={billingDisplay}
        conversationDetailLoading={conversationDetailLoading}
        onClose={closeDetail}
      />
      <LogCleanupDialog
        open={cleanupOpen}
        onOpenChange={setCleanupOpen}
        onSuccess={handleCleanupSuccess}
      />
    </div>
  );
}
