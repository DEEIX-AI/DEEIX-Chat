"use client";

import * as React from "react";
import { CornerDownRight, Trash2 } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { toast } from "sonner";

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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
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
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import { AdminDateRangeFilter, ADMIN_DATE_PICKER_TRIGGER_CLASSNAME } from "@/features/admin/components/admin-date-range-filter";
import { AdminDateTimePicker } from "@/features/admin/components/admin-date-time-picker";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { CopyActionButton } from "@/shared/components/copy-action";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import type {
  AdminAuditLogDTO,
  AdminConversationEventDTO,
  AdminPaymentOrderDTO,
  AdminSystemEventDTO,
  AdminUsageLogDTO,
  AdminUserAuthEventDTO,
} from "@/features/admin/api/admin.types";
import {
  cleanupAdminLogs,
  type AdminLogCleanupType,
} from "@/features/admin/api/audit";
import {
  AUDIT_LOG_SORT_OPTIONS,
  CONVERSATION_EVENT_SORT_OPTIONS,
  PAYMENT_ORDER_SORT_OPTIONS,
  SECURITY_LOG_SORT_OPTIONS,
  USAGE_LOG_SORT_OPTIONS,
  useAdminConversationEvents,
  useAdminLogs,
  useAdminPaymentOrders,
  useAdminSecurityLogs,
  useAdminUsageLogs,
  type AuditLogSortValue,
  type ConversationEventSortValue,
  type PaymentOrderSortValue,
  type SecurityLogSortValue,
  type UsageLogSortValue,
} from "@/features/admin/hooks/use-admin-logs";
import { cn } from "@/lib/utils";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { resolveAdminErrorMessage } from "@/features/admin/utils/admin-error";
import { billingRateMultiplierNote, cacheWriteBillingLabel, cacheWriteBillingNote, type BillingDisplayLabels } from "@/shared/lib/billing-display";
import { ModelSelect, type ModelSelectOption } from "@/shared/components/model-select";

type LogDetail =
  | { kind: "audit"; item: AdminAuditLogDTO }
  | { kind: "auth"; item: AdminUserAuthEventDTO }
  | { kind: "usage"; item: AdminUsageLogDTO }
  | { kind: "system"; item: AdminSystemEventDTO }
  | { kind: "order"; item: AdminPaymentOrderDTO }
  | { kind: "conversation"; item: AdminConversationEventDTO };

const ALL_MODELS_VALUE = "__all__";

function formatDateTime(value: string | null | undefined, locale: string): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function resolveUserDisplayName(label: string, username: string, fallbackID: number): string {
  const name = label.trim() || username.trim();
  return name || String(fallbackID);
}

function formatJSON(raw: string | null | undefined): string {
  const value = raw?.trim();
  if (!value) {
    return "{}";
  }
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

function parseJSONRecord(raw: string | null | undefined): Record<string, unknown> | null {
  const value = raw?.trim();
  if (!value) {
    return null;
  }
  try {
    const parsed = JSON.parse(value) as unknown;
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : null;
  } catch {
    return null;
  }
}

function formatCount(value: number | null | undefined, locale: string): string {
  return new Intl.NumberFormat(locale).format(value ?? 0);
}

function usageTotalTokens(item: AdminUsageLogDTO): number {
  return item.inputTokens + item.cacheReadTokens + item.cacheWriteTokens + item.outputTokens + item.reasoningTokens;
}

type UsagePricingSnapshot = {
  pricing_mode?: "token" | "call" | "duration" | "tiered" | string;
  provider_protocol?: string;
  cache_timeout?: string;
  fast_mode?: boolean;
  billing_speed?: string;
  billing_service_tier?: string;
  rate_multiplier?: number;
  cache_write_5m_tokens?: number;
  cache_write_1h_tokens?: number;
  input_nanousd_per_m_tokens?: number;
  cache_read_nanousd_per_m_tokens?: number;
  cache_write_nanousd_per_m_tokens?: number;
  output_nanousd_per_m_tokens?: number;
  call_nanousd_per_call?: number;
  duration_nanousd_per_second?: number;
  input_billed_nanousd?: number;
  cache_read_billed_nanousd?: number;
  cache_write_billed_nanousd?: number;
  output_billed_nanousd?: number;
  call_billed_nanousd?: number;
  duration_billed_nanousd?: number;
  tiered_from_tokens?: number;
  tiered_up_to_tokens?: number | null;
  upstream_usage?: unknown;
};

function usageLogRawUsageJSON(item: AdminUsageLogDTO): string {
  const upstreamUsage = parseJSONRecord(item.pricingSnapshotJSON)?.upstream_usage;
  if (upstreamUsage && typeof upstreamUsage === "object") {
    return JSON.stringify(upstreamUsage, null, 2);
  }
  return "{}";
}

type UsageBillingLabels = {
  input: string;
  output: string;
  cacheRead: string;
  total: string;
  freeModelNoBilling: string;
  perCall: string;
  perSecond: string;
  callUnit: string;
  secondUnit: string;
  rateNote: string;
  cacheNote: string;
  tieredRangeBounded: (from: string, upTo: string) => string;
  tieredRangeOpen: (from: string) => string;
  table: {
    item: string;
    usage: string;
    unitPrice: string;
    amount: string;
  };
  billingDisplay: BillingDisplayLabels;
};

function useUsageBillingLabels(): UsageBillingLabels {
  const t = useTranslations("adminLogs.usage.billing");

  return React.useMemo(
    () => ({
      input: t("input"),
      output: t("output"),
      cacheRead: t("cacheRead"),
      total: t("total"),
      freeModelNoBilling: t("freeModelNoBilling"),
      perCall: t("perCall"),
      perSecond: t("perSecond"),
      callUnit: t("callUnit"),
      secondUnit: t("secondUnit"),
      rateNote: t("rateNote"),
      cacheNote: t("cacheNote"),
      tieredRangeBounded: (from: string, upTo: string) => t("tieredRangeBounded", { from, upTo }),
      tieredRangeOpen: (from: string) => t("tieredRangeOpen", { from }),
      table: {
        item: t("table.item"),
        usage: t("table.usage"),
        unitPrice: t("table.unitPrice"),
        amount: t("table.amount"),
      },
      billingDisplay: {
        cacheWrite: t("cacheWrite"),
        cacheWrite5m: t("cacheWrite5m"),
        cacheWrite1h: t("cacheWrite1h"),
        cacheWrite5m1h: t("cacheWrite5m1h"),
        cacheWritePricingLabel: t("cacheWritePricingLabel"),
        cacheWritePricingNote: t("cacheWritePricingNote"),
        claudeCacheWriteMixedNote: (multiplier: string) => t("claudeCacheWriteMixedNote", { multiplier }),
        claudeCacheWriteNote: (timeout: "5m" | "1h", multiplier: string) => t("claudeCacheWriteNote", { timeout, multiplier }),
        claudeFastModeNote: (multiplier: string) => t("claudeFastModeNote", { multiplier }),
        openaiServiceTierNote: (tier: string, multiplier: string) => t("openaiServiceTierNote", { tier, multiplier }),
      },
    }),
    [t],
  );
}

function formatUsageCost(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "$0";
  if (value < 0.000001) return "< $0.000001";
  return `$${value.toLocaleString("en-US", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 6,
  })}`;
}

function formatTooltipUsageCost(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "$0.000000";
  return `$${value.toLocaleString("en-US", {
    minimumFractionDigits: 6,
    maximumFractionDigits: 6,
  })}`;
}

function formatMoneyCents(value: number | null | undefined, currency: string): string {
  const amount = (value ?? 0) / 100;
  const normalizedCurrency = currency.trim().toUpperCase();
  if (!normalizedCurrency) {
    return amount.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }
  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: normalizedCurrency,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(amount);
  } catch {
    return `${amount.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${normalizedCurrency}`;
  }
}

function formatTooltipUnitPrice(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "$0.00";
  return `$${value.toLocaleString("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

function nanousdToUSD(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 0;
  return value / 1_000_000_000;
}

function parseUsagePricingSnapshot(raw: string): UsagePricingSnapshot {
  try {
    const parsed = JSON.parse(raw) as unknown;
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed as UsagePricingSnapshot : {};
  } catch {
    return {};
  }
}

function readUsageSnapshotNumber(snapshot: UsagePricingSnapshot, key: keyof UsagePricingSnapshot): number {
  const value = snapshot[key];
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function normalizePricingMode(value: string | null | undefined): "token" | "call" | "duration" | "tiered" {
  if (value === "call" || value === "duration" || value === "tiered") return value;
  return "token";
}

function calcTokenBilledNanousd(tokens: number, rateNanousd: number): number {
  if (!Number.isFinite(tokens) || !Number.isFinite(rateNanousd) || tokens <= 0 || rateNanousd <= 0) return 0;
  return Math.round((tokens * rateNanousd) / 1_000_000);
}

function resolveTokenBilledNanousd(snapshot: UsagePricingSnapshot, billedKey: keyof UsagePricingSnapshot, tokens: number, rateNanousd: number): number {
  const billed = readUsageSnapshotNumber(snapshot, billedKey);
  return billed > 0 ? billed : calcTokenBilledNanousd(tokens, rateNanousd);
}

function resolveCountBilledNanousd(snapshot: UsagePricingSnapshot, billedKey: keyof UsagePricingSnapshot, count: number, rateNanousd: number): number {
  const billed = readUsageSnapshotNumber(snapshot, billedKey);
  if (billed > 0) return billed;
  if (!Number.isFinite(count) || !Number.isFinite(rateNanousd) || count <= 0 || rateNanousd <= 0) return 0;
  return Math.round(count * rateNanousd);
}

function formatFormulaTokenCount(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0";
  return value.toLocaleString("en-US");
}

function formatTieredRangeLabel(fromTokens: number | null | undefined, upToTokens: number | null | undefined, labels: UsageBillingLabels): string {
  const from = Number.isFinite(fromTokens ?? NaN) && (fromTokens ?? 0) > 0 ? fromTokens ?? 0 : 0;
  const upTo = Number.isFinite(upToTokens ?? NaN) && (upToTokens ?? 0) > 0 ? upToTokens ?? 0 : null;
  return upTo
    ? labels.tieredRangeBounded(formatFormulaTokenCount(from), formatFormulaTokenCount(upTo))
    : labels.tieredRangeOpen(formatFormulaTokenCount(from));
}

type UsageBillingTooltipLine =
  | { type: "row"; left: string; right: string }
  | { type: "divider" }
  | { type: "tiered-table"; rangeLabel: string; rows: UsageBillingTieredTableRow[]; totalLabel: string; totalAmount: string };

type UsageBillingTieredTableRow = {
  item: string;
  tokens: string;
  unitPrice: string;
  amount: string;
};

function UsageBillingTooltipLines({ lines, labels }: { lines: UsageBillingTooltipLine[]; labels: UsageBillingLabels }) {
  return (
    <div className="min-w-72 max-w-[min(92vw,44rem)] space-y-1 text-left text-xs leading-relaxed">
      {lines.map((line, index) =>
        line.type === "divider" ? (
          <Separator key={`divider-${index}`} />
        ) : line.type === "tiered-table" ? (
          <UsageBillingTieredTable key={`tiered-table-${index}`} line={line} labels={labels} />
        ) : (
          <div key={`${line.left}-${index}`} className="grid grid-cols-[minmax(0,1fr)_auto] items-baseline gap-8">
            <span className="min-w-0 text-left">{line.left}</span>
            <span className="whitespace-nowrap text-right tabular-nums">{line.right}</span>
          </div>
        ),
      )}
    </div>
  );
}

function UsageBillingTieredTable({
  line,
  labels,
}: {
  line: Extract<UsageBillingTooltipLine, { type: "tiered-table" }>;
  labels: UsageBillingLabels;
}) {
  return (
    <div className="max-w-[min(92vw,34rem)] overflow-x-auto">
      <div className="mb-1 text-[11px] font-medium text-background/80">{line.rangeLabel}</div>
      <table className="w-full border-collapse text-left tabular-nums">
        <thead>
          <tr className="border-b border-background/20 text-[11px] text-background/65">
            <th className="whitespace-nowrap px-2 pb-1 font-medium first:pl-0" aria-label={labels.table.item} />
            <th className="whitespace-nowrap px-2 pb-1 text-right font-medium">{labels.table.usage}</th>
            <th className="whitespace-nowrap px-2 pb-1 text-right font-medium">{labels.table.unitPrice}</th>
            <th className="whitespace-nowrap px-2 pb-1 text-right font-medium last:pr-0">{labels.table.amount}</th>
          </tr>
        </thead>
        <tbody>
          {line.rows.map((row, rowIndex) => (
            <tr key={`${row.item}-${rowIndex}`} className="border-b border-background/10 last:border-0">
              <td className="whitespace-nowrap px-2 py-1 first:pl-0">{row.item}</td>
              <td className="whitespace-nowrap px-2 py-1 text-right">{row.tokens}</td>
              <td className="whitespace-nowrap px-2 py-1 text-right">{row.unitPrice}</td>
              <td className="whitespace-nowrap px-2 py-1 text-right last:pr-0">{row.amount}</td>
            </tr>
          ))}
        </tbody>
        <tfoot>
          <tr className="border-t border-background/20">
            <td className="px-2 pt-1.5 font-medium first:pl-0" colSpan={3}>{line.totalLabel}</td>
            <td className="whitespace-nowrap px-2 pt-1.5 text-right font-medium last:pr-0">{line.totalAmount}</td>
          </tr>
        </tfoot>
      </table>
    </div>
  );
}

function usageFormulaLine(label: string, tokens: number, rateNanousd: number, billedNanousd: number): UsageBillingTooltipLine {
  return {
    type: "row",
    left: label,
    right: `${formatFormulaTokenCount(tokens)} tokens * ${formatTooltipUnitPrice(nanousdToUSD(rateNanousd))} / 1M = ${formatTooltipUsageCost(nanousdToUSD(billedNanousd))}`,
  };
}

function usageCountFormulaLine(label: string, count: number, unit: string, rateUnit: string, rateNanousd: number, billedNanousd: number): UsageBillingTooltipLine {
  const safeCount = Number.isFinite(count) && count > 0 ? count : 0;
  return {
    type: "row",
    left: label,
    right: `${safeCount.toLocaleString("en-US")} ${unit} * ${formatTooltipUnitPrice(nanousdToUSD(rateNanousd))} / ${rateUnit} = ${formatTooltipUsageCost(nanousdToUSD(billedNanousd))}`,
  };
}

function usageTieredTableRow(item: string, tokens: number, rateNanousd: number, billedNanousd: number): UsageBillingTieredTableRow {
  const safeTokens = Number.isFinite(tokens) && tokens > 0 ? tokens : 0;
  const safeBilled = Number.isFinite(billedNanousd) && billedNanousd > 0 ? billedNanousd : 0;
  return {
    item,
    tokens: formatFormulaTokenCount(safeTokens),
    unitPrice: `${formatTooltipUnitPrice(nanousdToUSD(rateNanousd))} / 1M`,
    amount: formatTooltipUsageCost(nanousdToUSD(safeBilled)),
  };
}

function usageTotalLine(item: AdminUsageLogDTO, labels: UsageBillingLabels): UsageBillingTooltipLine {
  return {
    type: "row",
    left: labels.total,
    right: item.isFreeModel ? `$0.000000 (${labels.freeModelNoBilling})` : formatTooltipUsageCost(nanousdToUSD(item.billedNanousd)),
  };
}

function buildUsageBillingTooltipLines(item: AdminUsageLogDTO, labels: UsageBillingLabels): UsageBillingTooltipLine[] {
  const snapshot = parseUsagePricingSnapshot(item.pricingSnapshotJSON);
  const pricingMode = normalizePricingMode(snapshot.pricing_mode);
  const inputRate = readUsageSnapshotNumber(snapshot, "input_nanousd_per_m_tokens");
  const outputRate = readUsageSnapshotNumber(snapshot, "output_nanousd_per_m_tokens");
  const cacheReadRate = readUsageSnapshotNumber(snapshot, "cache_read_nanousd_per_m_tokens");
  const cacheWriteRate = readUsageSnapshotNumber(snapshot, "cache_write_nanousd_per_m_tokens");
  const billedOutputTokens = item.outputTokens + item.reasoningTokens;
  const totalLine = usageTotalLine(item, labels);
  const cacheWriteLabel = cacheWriteBillingLabel(snapshot, labels.billingDisplay);
  const cacheWriteNote = cacheWriteBillingNote(snapshot, labels.billingDisplay);
  const rateMultiplierNote = billingRateMultiplierNote(snapshot, labels.billingDisplay);

  if (pricingMode === "call") {
    const callRate = readUsageSnapshotNumber(snapshot, "call_nanousd_per_call");
    const callBilled = resolveCountBilledNanousd(snapshot, "call_billed_nanousd", item.callCount, callRate);
    return [
      usageCountFormulaLine(labels.perCall, item.callCount, labels.callUnit, labels.callUnit, callRate, callBilled),
      { type: "divider" },
      totalLine,
    ];
  }

  if (pricingMode === "duration") {
    const durationRate = readUsageSnapshotNumber(snapshot, "duration_nanousd_per_second");
    const durationBilled = resolveCountBilledNanousd(snapshot, "duration_billed_nanousd", item.durationSeconds, durationRate);
    return [
      usageCountFormulaLine(labels.perSecond, item.durationSeconds, labels.secondUnit, labels.secondUnit, durationRate, durationBilled),
      { type: "divider" },
      totalLine,
    ];
  }

  if (pricingMode === "tiered") {
    const lines: UsageBillingTooltipLine[] = [];
    if (rateMultiplierNote || cacheWriteNote) {
      if (rateMultiplierNote) {
        lines.push({ type: "row", left: labels.rateNote, right: rateMultiplierNote });
      }
      if (cacheWriteNote) {
        lines.push({ type: "row", left: labels.cacheNote, right: cacheWriteNote });
      }
      lines.push({ type: "divider" });
    }
    lines.push({
      type: "tiered-table",
      rangeLabel: formatTieredRangeLabel(snapshot.tiered_from_tokens, snapshot.tiered_up_to_tokens, labels),
      rows: [
        usageTieredTableRow(labels.input, item.inputTokens, inputRate, readUsageSnapshotNumber(snapshot, "input_billed_nanousd")),
        usageTieredTableRow(labels.output, billedOutputTokens, outputRate, readUsageSnapshotNumber(snapshot, "output_billed_nanousd")),
        usageTieredTableRow(labels.cacheRead, item.cacheReadTokens, cacheReadRate, readUsageSnapshotNumber(snapshot, "cache_read_billed_nanousd")),
        usageTieredTableRow(cacheWriteLabel, item.cacheWriteTokens, cacheWriteRate, readUsageSnapshotNumber(snapshot, "cache_write_billed_nanousd")),
      ],
      totalLabel: labels.total,
      totalAmount: item.isFreeModel ? `$0.000000 (${labels.freeModelNoBilling})` : formatTooltipUsageCost(nanousdToUSD(item.billedNanousd)),
    });
    return lines;
  }

  const inputBilled = resolveTokenBilledNanousd(snapshot, "input_billed_nanousd", item.inputTokens, inputRate);
  const cacheReadBilled = resolveTokenBilledNanousd(snapshot, "cache_read_billed_nanousd", item.cacheReadTokens, cacheReadRate);
  const cacheWriteBilled = resolveTokenBilledNanousd(snapshot, "cache_write_billed_nanousd", item.cacheWriteTokens, cacheWriteRate);
  const outputBilled = resolveTokenBilledNanousd(snapshot, "output_billed_nanousd", billedOutputTokens, outputRate);
  const lines: UsageBillingTooltipLine[] = [
    usageFormulaLine(labels.input, item.inputTokens, inputRate, inputBilled),
    usageFormulaLine(labels.output, billedOutputTokens, outputRate, outputBilled),
    usageFormulaLine(labels.cacheRead, item.cacheReadTokens, cacheReadRate, cacheReadBilled),
    usageFormulaLine(cacheWriteLabel, item.cacheWriteTokens, cacheWriteRate, cacheWriteBilled),
    { type: "divider" },
    totalLine,
  ];
  const noteLines: UsageBillingTooltipLine[] = [];
  if (rateMultiplierNote) {
    noteLines.push({ type: "row", left: labels.rateNote, right: rateMultiplierNote });
  }
  if (cacheWriteNote) {
    noteLines.push({ type: "row", left: labels.cacheNote, right: cacheWriteNote });
  }
  if (noteLines.length > 0) {
    lines.splice(4, 0, ...noteLines);
  }
  return lines;
}

function UsageLogModelCell({ item, labels }: { item: AdminUsageLogDTO; labels: UsageBillingLabels }) {
  const t = useTranslations("adminLogs.usage.modelTooltip");
  const lines: UsageBillingTooltipLine[] = [
    { type: "row", left: t("upstreamName"), right: item.upstreamName || "-" },
    { type: "row", left: t("upstreamModel"), right: item.upstreamModelName || "-" },
    { type: "row", left: t("bindingCode"), right: item.routedBindingCode || "-" },
    { type: "row", left: t("protocol"), right: item.providerProtocol || "-" },
  ];

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className="grid min-w-0 cursor-default gap-px">
          <div className="max-w-[15rem] truncate font-medium leading-4" title={item.platformModelName || "-"}>
            {item.platformModelName || "-"}
          </div>
          <div className="flex min-w-0 items-center gap-1 font-mono leading-4 text-muted-foreground">
            <CornerDownRight className="size-3 shrink-0 stroke-1" />
            <span className="max-w-[14rem] truncate" title={item.upstreamModelName || "-"}>
              {item.upstreamModelName || "-"}
            </span>
          </div>
        </div>
      </TooltipTrigger>
      <TooltipContent side="top">
        <UsageBillingTooltipLines lines={lines} labels={labels} />
      </TooltipContent>
    </Tooltip>
  );
}

function UsageLogTokenCell({ item, locale }: { item: AdminUsageLogDTO; locale: string }) {
  const t = useTranslations("adminLogs.usage.tokens");
  const tokens = [
    { label: t("inputShort"), value: item.inputTokens },
    { label: t("outputShort"), value: item.outputTokens },
    { label: t("cacheReadShort"), value: item.cacheReadTokens },
    { label: t("cacheWriteShort"), value: item.cacheWriteTokens },
  ];

  return (
    <div className="grid min-w-[10.5rem] grid-cols-2 gap-1">
      {tokens.map((token) => (
        <span
          key={token.label}
          className="inline-flex h-5 items-center justify-between gap-1 rounded-md bg-muted/45 px-1.5 font-mono text-[11px] leading-none text-muted-foreground"
        >
          <span>{token.label}</span>
          <span className="tabular-nums">{formatCount(token.value, locale)}</span>
        </span>
      ))}
    </div>
  );
}

function UsageLogCostCell({ item, labels }: { item: AdminUsageLogDTO; labels: UsageBillingLabels }) {
  const lines = buildUsageBillingTooltipLines(item, labels);

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className={cn("inline-flex cursor-default items-center font-medium tabular-nums", item.isFreeModel ? "text-muted-foreground" : "text-foreground")}>
          {item.isFreeModel ? labels.freeModelNoBilling : formatUsageCost(item.billedUSD)}
        </span>
      </TooltipTrigger>
      <TooltipContent side="top">
        <UsageBillingTooltipLines lines={lines} labels={labels} />
      </TooltipContent>
    </Tooltip>
  );
}

function UsageLogModelFilter({
  value,
  options,
  disabled,
  onChange,
}: {
  value: string;
  options: ModelSelectOption[];
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  const t = useTranslations("adminLogs.usage.filters");
  const allOption = React.useMemo<ModelSelectOption>(() => ({ label: t("allModels"), value: ALL_MODELS_VALUE, iconUrl: null }), [t]);
  const modelOptions = React.useMemo(() => [allOption, ...options], [allOption, options]);

  return (
    <ModelSelect
      value={value.trim() || ALL_MODELS_VALUE}
      fallbackValue={ALL_MODELS_VALUE}
      disabled={disabled}
      options={modelOptions}
      align="start"
      valueAlign="start"
      itemAlign="start"
      contentClassName="min-w-[320px]"
      triggerClassName={cn(ADMIN_DATE_PICKER_TRIGGER_CLASSNAME, "h-7 px-2.5 text-[11px]")}
      valueClassName={!value.trim() ? "text-muted-foreground" : undefined}
      onChange={(nextValue) => onChange(nextValue === ALL_MODELS_VALUE ? "" : nextValue)}
    />
  );
}

function DetailRow({ label, value, mono = false }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="grid grid-cols-[88px_minmax(0,1fr)] gap-3 border-b border-border/50 py-2.5 last:border-b-0">
      <p className="text-xs text-muted-foreground">{label}</p>
      <div className={cn("min-w-0 break-words text-xs leading-5 text-foreground/86", mono && "font-mono")}>{value ?? "-"}</div>
    </div>
  );
}

function DetailBlock({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-2">
      <h4 className="px-1 text-xs font-medium text-foreground/88">{title}</h4>
      <div className="rounded-lg border border-border/60 bg-background px-3">{children}</div>
    </section>
  );
}

function LogDetailSheet({ detail: rawDetail, onClose }: { detail: LogDetail | null; onClose: () => void }) {
  const locale = useLocale();
  const t = useTranslations("adminLogs.detail");
  const usageLabels = useUsageBillingLabels();
  const detail = useDialogSnapshot(rawDetail);
  const copyMessages = React.useMemo(() => ({
    copied: t("copied", { label: "" }).trim(),
    failed: t("copyFailed"),
  }), [t]);
  const resultLabel = React.useCallback(
    (value: string) => {
      switch (value) {
        case "success":
          return t("result.success");
        case "failure":
          return t("result.failure");
        case "blocked":
          return t("result.blocked");
        default:
          return value || "-";
      }
    },
    [t],
  );
  const title =
    detail?.kind === "auth"
      ? t("titles.auth")
      : detail?.kind === "usage"
        ? t("titles.usage")
        : detail?.kind === "order"
          ? t("titles.order")
          : detail?.kind === "conversation"
            ? t("titles.conversation")
        : detail?.kind === "system"
          ? t("titles.system")
          : t("titles.audit");
  const description =
    detail?.kind === "auth"
      ? `${detail.item.eventType || t("fallbacks.authEvent")} · ${formatDateTime(detail.item.occurredAt, locale)}`
      : detail?.kind === "usage"
        ? `${detail.item.platformModelName || t("fallbacks.modelCall")} · ${formatDateTime(detail.item.createdAt, locale)}`
        : detail?.kind === "order"
          ? `${detail.item.orderNo || t("fallbacks.order")} · ${formatDateTime(detail.item.createdAt, locale)}`
          : detail?.kind === "conversation"
            ? `${detail.item.eventType || detail.item.eventScope || t("fallbacks.conversationEvent")} · ${formatDateTime(detail.item.createdAt, locale)}`
      : detail?.kind === "system"
        ? `${detail.item.event || t("fallbacks.systemEvent")} · ${formatDateTime(detail.item.createdAt, locale)}`
        : `${detail?.item.action || t("fallbacks.auditEvent")} · ${formatDateTime(detail?.item.createdAt, locale)}`;
  const requestID = detail && detail.kind !== "usage" && detail.kind !== "order" && detail.kind !== "conversation" ? detail.item.requestID : "";
  const detailJSON =
    detail?.kind === "usage"
      ? detail.item.pricingSnapshotJSON
      : detail?.kind === "order"
        ? detail.item.snapshotJSON
        : detail?.kind === "conversation"
          ? detail.item.payloadJSON || detail.item.inputJSON || detail.item.outputJSON || detail.item.errorJSON
          : detail?.item.detailJSON;
  const rawUsageJSON = detail?.kind === "usage" ? usageLogRawUsageJSON(detail.item) : "";
  const formattedJSON = formatJSON(detailJSON);

  return (
    <Sheet open={Boolean(rawDetail)} onOpenChange={(open) => !open && onClose()}>
      <SheetContent className="sm:max-w-[480px]">
        <SheetHeader>
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription>{description}</SheetDescription>
        </SheetHeader>

        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-6 pb-6">
          {detail?.kind === "audit" ? (
            <>
              <DetailBlock title={t("blocks.event")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.action")} value={detail.item.action} />
                <DetailRow label={t("fields.resource")} value={detail.item.resource} />
                <DetailRow label={t("fields.resourceID")} value={detail.item.resourceID} mono />
                <DetailRow label={t("fields.createdAt")} value={formatDateTime(detail.item.createdAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.actor")}>
                <DetailRow label={t("fields.user")} value={resolveUserDisplayName(detail.item.actorLabel, detail.item.actorUsername, detail.item.actorUserID)} />
                <DetailRow label={t("fields.userID")} value={detail.item.actorUserID} mono />
              </DetailBlock>
              <DetailBlock title={t("blocks.request")}>
                <DetailRow label={t("fields.requestID")} value={detail.item.requestID} mono />
                <DetailRow label="IP" value={detail.item.ip} mono />
                <DetailRow label="User Agent" value={detail.item.userAgent} />
              </DetailBlock>
            </>
          ) : null}

          {detail?.kind === "auth" ? (
            <>
              <DetailBlock title={t("blocks.event")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.event")} value={detail.item.eventType} />
                <DetailRow label={t("fields.result")} value={resultLabel(detail.item.result)} />
                <DetailRow label={t("fields.reason")} value={detail.item.reason} />
                <DetailRow label={t("fields.occurredAt")} value={formatDateTime(detail.item.occurredAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.user")}>
                <DetailRow label={t("fields.user")} value={resolveUserDisplayName(detail.item.userLabel, detail.item.username, detail.item.userID)} />
                <DetailRow label={t("fields.userID")} value={detail.item.userID} mono />
              </DetailBlock>
              <DetailBlock title={t("blocks.request")}>
                <DetailRow label={t("fields.requestID")} value={detail.item.requestID} mono />
                <DetailRow label="IP" value={detail.item.clientIP} mono />
                <DetailRow label="User Agent" value={detail.item.userAgent} />
              </DetailBlock>
            </>
          ) : null}

          {detail?.kind === "system" ? (
            <>
              <DetailBlock title={t("blocks.event")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.level")} value={detail.item.level} />
                <DetailRow label={t("fields.source")} value={detail.item.source} />
                <DetailRow label={t("fields.event")} value={detail.item.event} />
                <DetailRow label={t("fields.message")} value={detail.item.message} />
                <DetailRow label={t("fields.createdAt")} value={formatDateTime(detail.item.createdAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.resource")}>
                <DetailRow label={t("fields.resource")} value={detail.item.resource} />
                <DetailRow label={t("fields.resourceID")} value={detail.item.resourceID} mono />
              </DetailBlock>
              <DetailBlock title={t("blocks.request")}>
                <DetailRow label={t("fields.requestID")} value={detail.item.requestID} mono />
                <DetailRow label="Trace ID" value={detail.item.traceID} mono />
              </DetailBlock>
            </>
          ) : null}

          {detail?.kind === "usage" ? (
            <>
              <DetailBlock title={t("blocks.call")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.caller")} value={resolveUserDisplayName(detail.item.userLabel, detail.item.username, detail.item.userID)} />
                <DetailRow label={t("fields.userID")} value={detail.item.userID} mono />
                <DetailRow label={t("fields.conversationID")} value={detail.item.conversationID} mono />
                <DetailRow label={t("fields.callTime")} value={formatDateTime(detail.item.createdAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.modelRoute")}>
                <DetailRow label={t("fields.platformModel")} value={detail.item.platformModelName} mono />
                <DetailRow label={t("fields.upstreamName")} value={detail.item.upstreamName} />
                <DetailRow label={t("fields.upstreamModel")} value={detail.item.upstreamModelName} mono />
                <DetailRow label={t("fields.bindingCode")} value={detail.item.routedBindingCode} mono />
                <DetailRow label={t("fields.protocol")} value={detail.item.providerProtocol} />
              </DetailBlock>
              <DetailBlock title={t("blocks.usageBilling")}>
                <DetailRow label={t("fields.billing")} value={`${formatTooltipUsageCost(detail.item.billedUSD)} ${detail.item.isFreeModel ? `(${usageLabels.freeModelNoBilling})` : ""}`} />
                <DetailRow label={t("fields.totalTokens")} value={formatCount(usageTotalTokens(detail.item), locale)} mono />
                <DetailRow label={usageLabels.input} value={formatCount(detail.item.inputTokens, locale)} mono />
                <DetailRow label={usageLabels.cacheRead} value={formatCount(detail.item.cacheReadTokens, locale)} mono />
                <DetailRow label={usageLabels.billingDisplay.cacheWrite} value={formatCount(detail.item.cacheWriteTokens, locale)} mono />
                <DetailRow label={usageLabels.output} value={formatCount(detail.item.outputTokens, locale)} mono />
                <DetailRow label={t("fields.reasoning")} value={formatCount(detail.item.reasoningTokens, locale)} mono />
                <DetailRow label={t("fields.callCount")} value={formatCount(detail.item.callCount, locale)} mono />
                <DetailRow label={t("fields.latency")} value={`${formatCount(detail.item.latencyMS, locale)} ms`} mono />
              </DetailBlock>
            </>
          ) : null}

          {detail?.kind === "usage" ? (
            <section className="space-y-2">
              <div className="flex items-center justify-between gap-3 px-1">
                <h4 className="text-xs font-medium text-foreground/88">{t("rawUsageJsonTitle")}</h4>
                <CopyActionButton
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs shadow-none"
                  value={rawUsageJSON}
                  messages={copyMessages}
                  copyOptions={{ copied: t("copied", { label: t("rawUsageJsonTitle") }) }}
                >
                  JSON
                </CopyActionButton>
              </div>
              <pre className="max-h-[240px] overflow-auto rounded-lg border border-border/60 bg-muted/35 p-3 text-xs leading-5 text-foreground/86">
                <code>{rawUsageJSON}</code>
              </pre>
            </section>
          ) : null}

          {detail?.kind === "order" ? (
            <>
              <DetailBlock title={t("blocks.order")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.orderNo")} value={detail.item.orderNo} mono />
                <DetailRow label={t("fields.orderType")} value={detail.item.orderType} />
                <DetailRow label={t("fields.provider")} value={detail.item.provider} />
                <DetailRow label={t("fields.status")} value={detail.item.status} />
                <DetailRow label={t("fields.createdAt")} value={formatDateTime(detail.item.createdAt, locale)} />
                <DetailRow label={t("fields.paidAt")} value={formatDateTime(detail.item.paidAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.user")}>
                <DetailRow label={t("fields.user")} value={resolveUserDisplayName(detail.item.userLabel, detail.item.username, detail.item.userID)} />
                <DetailRow label={t("fields.userID")} value={detail.item.userID} mono />
              </DetailBlock>
              <DetailBlock title={t("blocks.payment")}>
                <DetailRow label={t("fields.amount")} value={`${formatMoneyCents(detail.item.payAmountCents, detail.item.payCurrency)} / ${formatMoneyCents(detail.item.baseAmountCents, detail.item.baseCurrency)}`} mono />
                <DetailRow label={t("fields.credit")} value={formatTooltipUsageCost(detail.item.creditUSD)} mono />
                <DetailRow label={t("fields.interval")} value={`${detail.item.billingInterval || "-"} x ${detail.item.cycles || 0}`} />
                <DetailRow label={t("fields.externalPaymentID")} value={detail.item.externalPaymentID || "-"} mono />
                <DetailRow label={t("fields.externalCheckoutID")} value={detail.item.externalCheckoutID || "-"} mono />
              </DetailBlock>
            </>
          ) : null}

          {detail?.kind === "conversation" ? (
            <>
              <DetailBlock title={t("blocks.conversationEvent")}>
                <DetailRow label="ID" value={detail.item.id} mono />
                <DetailRow label={t("fields.runID")} value={detail.item.runID} mono />
                <DetailRow label={t("fields.eventScope")} value={detail.item.eventScope} />
                <DetailRow label={t("fields.event")} value={detail.item.eventType} />
                <DetailRow label={t("fields.status")} value={detail.item.status} />
                <DetailRow label={t("fields.stage")} value={detail.item.stage || detail.item.phase || "-"} />
                <DetailRow label={t("fields.seq")} value={detail.item.seq} mono />
                <DetailRow label={t("fields.createdAt")} value={formatDateTime(detail.item.createdAt, locale)} />
              </DetailBlock>
              <DetailBlock title={t("blocks.user")}>
                <DetailRow label={t("fields.user")} value={resolveUserDisplayName(detail.item.userLabel, detail.item.username, detail.item.userID)} />
                <DetailRow label={t("fields.userID")} value={detail.item.userID} mono />
                <DetailRow label={t("fields.conversationID")} value={detail.item.conversationID} mono />
                <DetailRow label={t("fields.messageID")} value={detail.item.messageID} mono />
              </DetailBlock>
              <DetailBlock title={t("blocks.modelRoute")}>
                <DetailRow label={t("fields.platformModel")} value={detail.item.platformModelName || "-"} mono />
                <DetailRow label={t("fields.upstreamName")} value={detail.item.upstreamName || "-"} />
                <DetailRow label={t("fields.upstreamModel")} value={detail.item.upstreamModelName || "-"} mono />
                <DetailRow label={t("fields.bindingCode")} value={detail.item.routedBindingCode || "-"} mono />
                <DetailRow label={t("fields.protocol")} value={detail.item.providerProtocol || "-"} />
              </DetailBlock>
              <DetailBlock title={t("blocks.tool")}>
                <DetailRow label={t("fields.toolName")} value={detail.item.toolName || "-"} />
                <DetailRow label={t("fields.toolCallID")} value={detail.item.toolCallID || "-"} mono />
                <DetailRow label={t("fields.latency")} value={`${formatCount(detail.item.latencyMS, locale)} ms`} mono />
                <DetailRow label={t("fields.title")} value={detail.item.title || "-"} />
                <DetailRow label={t("fields.summary")} value={detail.item.summary || "-"} />
              </DetailBlock>
            </>
          ) : null}

          <section className="space-y-2">
            <div className="flex items-center justify-between gap-3 px-1">
              <h4 className="text-xs font-medium text-foreground/88">{t("jsonTitle")}</h4>
              <div className="flex items-center gap-1">
                {requestID ? (
                  <CopyActionButton
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2 text-xs shadow-none"
                    value={requestID}
                    messages={copyMessages}
                    copyOptions={{ copied: t("copied", { label: t("fields.requestID") }) }}
                  >
                    {t("fields.requestID")}
                  </CopyActionButton>
                ) : null}
                <CopyActionButton
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs shadow-none"
                  value={formattedJSON}
                  messages={copyMessages}
                  copyOptions={{ copied: t("copied", { label: t("jsonTitle") }) }}
                >
                  JSON
                </CopyActionButton>
              </div>
            </div>
            <pre className="max-h-[320px] overflow-auto rounded-lg border border-border/60 bg-muted/35 p-3 text-xs leading-5 text-foreground/86">
              <code>{formattedJSON}</code>
            </pre>
          </section>
        </div>
      </SheetContent>
    </Sheet>
  );
}

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

function UsageLogTable({ onOpenDetail }: { onOpenDetail: (item: AdminUsageLogDTO) => void }) {
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
            <TableHead>Token</TableHead>
            <TableHead>{t("columns.billing")}</TableHead>
            <TableHead>{t("columns.latency")}</TableHead>
            <TableHead>{t("columns.time")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.loading && logs.logs.length === 0 ? <TableLoadingRow colSpan={7} /> : null}
          {logs.logs.length > 0 ? <VirtualTablePaddingRow colSpan={7} height={virtualRows.paddingTop} /> : null}
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
                <UsageLogTokenCell item={item} locale={locale} />
              </TableCell>
              <TableCell><UsageLogCostCell item={item} labels={usageLabels} /></TableCell>
              <TableCell className="whitespace-nowrap font-mono text-muted-foreground">{formatCount(item.latencyMS, locale)} ms</TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">{formatDateTime(item.createdAt, locale)}</TableCell>
            </TableRow>
          )) : null}
          {logs.logs.length > 0 ? <VirtualTablePaddingRow colSpan={7} height={virtualRows.paddingBottom} /> : null}
          {!logs.loading && logs.logs.length === 0 ? <TableEmptyRow colSpan={7}>{t("usage.empty")}</TableEmptyRow> : null}
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
  const logs = useAdminConversationEvents();
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
          {logs.loading && logs.events.length === 0 ? <TableLoadingRow colSpan={9} /> : null}
          {logs.events.length > 0 ? <VirtualTablePaddingRow colSpan={9} height={virtualRows.paddingTop} /> : null}
          {logs.events.length > 0 ? virtualRows.rows.map(({ item }) => (
            <TableRow key={item.id} className="cursor-pointer" onClick={() => onOpenDetail(item)}>
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
                <div className="max-w-[13rem] truncate" title={item.runID || "-"}>{item.runID || "-"}</div>
              </TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">{formatDateTime(item.createdAt, locale)}</TableCell>
            </TableRow>
          )) : null}
          {logs.events.length > 0 ? <VirtualTablePaddingRow colSpan={9} height={virtualRows.paddingBottom} /> : null}
          {!logs.loading && logs.events.length === 0 ? <TableEmptyRow colSpan={9}>{t("conversation.empty")}</TableEmptyRow> : null}
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

function cleanupDateToISOString(value: string): string | null {
  const [yearText, monthText, dayText] = value.trim().split("-");
  const year = Number.parseInt(yearText ?? "", 10);
  const month = Number.parseInt(monthText ?? "", 10);
  const day = Number.parseInt(dayText ?? "", 10);
  if (!year || !month || !day) {
    return null;
  }
  const date = new Date(year, month - 1, day, 0, 0, 0, 0);
  if (
    Number.isNaN(date.getTime()) ||
    date.getFullYear() !== year ||
    date.getMonth() !== month - 1 ||
    date.getDate() !== day
  ) {
    return null;
  }
  return date.toISOString();
}

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
  const [logType, setLogType] = React.useState<AdminLogCleanupType>("audit");
  const [date, setDate] = React.useState("");
  const [pending, setPending] = React.useState(false);
  const highRisk = logType === "usage" || logType === "orders";

  const handleOpenChange = React.useCallback((nextOpen: boolean) => {
    if (pending) {
      return;
    }
    onOpenChange(nextOpen);
    if (!nextOpen) {
      setLogType("audit");
      setDate("");
    }
  }, [onOpenChange, pending]);

  const submit = React.useCallback(async () => {
    const before = cleanupDateToISOString(date);
    if (!before) {
      return;
    }

    setPending(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("toast.sessionExpired"), { description: t("toast.signInAgain") });
        return;
      }
      const result = await cleanupAdminLogs(token, { type: logType, before });
      toast.success(t("toast.success", { count: result.deletedCount }));
      onSuccess(logType);
      onOpenChange(false);
      setLogType("audit");
      setDate("");
    } catch (error) {
      toast.error(t("toast.failed"), { description: resolveAdminErrorMessage(error) });
    } finally {
      setPending(false);
    }
  }, [date, logType, onOpenChange, onSuccess, t]);

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

export function AdminLogsPage() {
  const t = useTranslations("adminLogs");
  const [detail, setDetail] = React.useState<LogDetail | null>(null);
  const [cleanupOpen, setCleanupOpen] = React.useState(false);
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

      <Tabs defaultValue="audit" className="space-y-3">
        <TabsList variant="line">
          <TabsTrigger value="audit">{t("tabs.audit")}</TabsTrigger>
          <TabsTrigger value="usage">{t("tabs.usage")}</TabsTrigger>
          <TabsTrigger value="auth">{t("tabs.auth")}</TabsTrigger>
          <TabsTrigger value="orders">{t("tabs.orders")}</TabsTrigger>
          <TabsTrigger value="conversation">{t("tabs.conversation")}</TabsTrigger>
        </TabsList>
        <TabsContent value="audit">
          <AuditLogTable key={cleanupRevisions.audit} onOpenDetail={(item) => setDetail({ kind: "audit", item })} />
        </TabsContent>
        <TabsContent value="auth">
          <AuthLogTable key={cleanupRevisions.auth} onOpenDetail={(item) => setDetail({ kind: "auth", item })} />
        </TabsContent>
        <TabsContent value="usage">
          <UsageLogTable key={cleanupRevisions.usage} onOpenDetail={(item) => setDetail({ kind: "usage", item })} />
        </TabsContent>
        <TabsContent value="orders">
          <PaymentOrderTable key={cleanupRevisions.orders} onOpenDetail={(item) => setDetail({ kind: "order", item })} />
        </TabsContent>
        <TabsContent value="conversation">
          <ConversationEventTable key={cleanupRevisions.conversation} onOpenDetail={(item) => setDetail({ kind: "conversation", item })} />
        </TabsContent>
      </Tabs>

      <LogDetailSheet detail={detail} onClose={() => setDetail(null)} />
      <LogCleanupDialog
        open={cleanupOpen}
        onOpenChange={setCleanupOpen}
        onSuccess={handleCleanupSuccess}
      />
    </div>
  );
}
