"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { Separator } from "@/components/ui/separator";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow, TableSkeletonRows } from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useProgressiveRows } from "@/hooks/use-progressive-rows";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import type { BillingUsageLedgerDTO } from "@/shared/api/billing.types";
import { billingRateMultiplierNote, cacheWriteBillingLabel, cacheWriteBillingNote } from "@/shared/lib/billing-display";
import type { BillingDisplayLabels } from "@/shared/lib/billing-display";
import {
  formatFormulaTokenCount,
  formatLatency,
  formatTooltipUnitPrice,
  formatTooltipUsageCost,
  formatUsageCost,
  formatUsageLogTime,
  modelDisplayLabel,
  nanousdToUSD,
} from "./subscription-format";

const USAGE_LOG_PAGE_SIZE_OPTIONS = [25, 50, 100, 200] as const;

type BillingTooltipLabels = {
  display: BillingDisplayLabels;
  baseService: string;
  input: string;
  output: string;
  cacheRead: string;
  rateNote: string;
  cacheNote: string;
  total: string;
  subtotal: string;
  freeModelNoBilling: string;
  perCall: string;
  perSecond: string;
  callUnit: string;
  secondUnit: string;
  tieredRange: (from: string, upTo: string | null) => string;
};

function useBillingTooltipLabels(): BillingTooltipLabels {
  const t = useTranslations("settings.subscriptionPage.billingTooltip");
  return React.useMemo(
    () => ({
      display: {
        cacheWrite: t("cacheWrite"),
        cacheWrite5m: t("cacheWrite5m"),
        cacheWrite1h: t("cacheWrite1h"),
        cacheWrite5m1h: t("cacheWrite5m1h"),
        claudeCacheWriteMixedNote: (multiplier) => t("claudeCacheWriteMixedNote", { multiplier }),
        claudeCacheWriteNote: (timeout, multiplier) => t("claudeCacheWriteNote", { timeout, multiplier }),
        claudeFastModeNote: (multiplier) => t("claudeFastModeNote", { multiplier }),
        openaiServiceTierNote: (tier, multiplier) => t("openaiServiceTierNote", { tier, multiplier }),
        cacheWritePricingLabel: t("cacheWritePricingLabel"),
        cacheWritePricingNote: t("cacheWritePricingNote"),
      },
      baseService: t("baseService"),
      input: t("input"),
      output: t("output"),
      cacheRead: t("cacheRead"),
      rateNote: t("rateNote"),
      cacheNote: t("cacheNote"),
      total: t("total"),
      subtotal: t("subtotal"),
      freeModelNoBilling: t("freeModelNoBilling"),
      perCall: t("perCall"),
      perSecond: t("perSecond"),
      callUnit: t("callUnit"),
      secondUnit: t("secondUnit"),
      tieredRange: (from, upTo) => upTo ? t("tieredRangeBounded", { from, upTo }) : t("tieredRangeOpen", { from }),
    }),
    [t],
  );
}

type BillingPricingSnapshot = {
  platform_model_name?: string;
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
  base_service_billed_nanousd?: number;
  tiered_from_tokens?: number;
  tiered_up_to_tokens?: number | null;
  service_items?: BillingServiceItemSnapshot[];
};

type BillingServiceItemSnapshot = {
  service_code?: string;
  service_name?: string;
  platform_model_name?: string;
  pricing_mode?: "token" | "call" | "duration" | "tiered" | string;
  provider_protocol?: string;
  cache_timeout?: string;
  fast_mode?: boolean;
  billing_speed?: string;
  billing_service_tier?: string;
  rate_multiplier?: number;
  cache_write_5m_tokens?: number;
  cache_write_1h_tokens?: number;
  input_tokens?: number;
  cache_read_tokens?: number;
  cache_write_tokens?: number;
  output_tokens?: number;
  reasoning_tokens?: number;
  call_count?: number;
  duration_seconds?: number;
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
  billed_nanousd?: number;
  tiered_from_tokens?: number;
  tiered_up_to_tokens?: number | null;
};

function parsePricingSnapshot(value: string): BillingPricingSnapshot {
  if (!value) return {};
  try {
    const parsed = JSON.parse(value) as unknown;
    return parsed && typeof parsed === "object" ? (parsed as BillingPricingSnapshot) : {};
  } catch {
    return {};
  }
}

function readSnapshotNumber(snapshot: BillingPricingSnapshot, key: keyof BillingPricingSnapshot): number {
  const value = snapshot[key];
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function calcTokenBilledNanousd(tokens: number, rateNanousd: number): number {
  if (!Number.isFinite(tokens) || !Number.isFinite(rateNanousd) || tokens <= 0 || rateNanousd <= 0) return 0;
  return Math.round((tokens * rateNanousd) / 1_000_000);
}

function normalizePricingMode(value: string | null | undefined): "token" | "call" | "duration" | "tiered" {
  if (value === "call" || value === "duration" || value === "tiered") return value;
  return "token";
}

function resolveTokenBilledNanousd(snapshot: BillingPricingSnapshot, billedKey: keyof BillingPricingSnapshot, tokens: number, rateNanousd: number): number {
  const billed = readSnapshotNumber(snapshot, billedKey);
  return billed > 0 ? billed : calcTokenBilledNanousd(tokens, rateNanousd);
}

function resolveCountBilledNanousd(snapshot: BillingPricingSnapshot, billedKey: keyof BillingPricingSnapshot, count: number, rateNanousd: number): number {
  const billed = readSnapshotNumber(snapshot, billedKey);
  if (billed > 0) return billed;
  if (!Number.isFinite(count) || !Number.isFinite(rateNanousd) || count <= 0 || rateNanousd <= 0) return 0;
  return Math.round(count * rateNanousd);
}

type BillingTooltipLine =
  | { type: "row"; left: string; right: string }
  | { type: "divider" }
  | { type: "tiered-table"; rangeLabel: string; rows: BillingTieredTableRow[]; totalLabel: string; totalAmount: string };

type BillingTieredTableRow = {
  item: string;
  tokens: string;
  unitPrice: string;
  amount: string;
};

function formatBillingFormulaLine(label: string, tokens: number, rateNanousd: number, billedNanousd: number): BillingTooltipLine {
  return {
    type: "row",
    left: label,
    right: `${formatFormulaTokenCount(tokens)} tokens * ${formatTooltipUnitPrice(nanousdToUSD(rateNanousd))} / 1M = ${formatTooltipUsageCost(nanousdToUSD(billedNanousd))}`,
  };
}

function formatCountBillingFormulaLine(label: string, count: number, unit: string, rateUnit: string, rateNanousd: number, billedNanousd: number): BillingTooltipLine {
  const safeCount = Number.isFinite(count) && count > 0 ? count : 0;
  return {
    type: "row",
    left: label,
    right: `${safeCount.toLocaleString("en-US")} ${unit} * ${formatTooltipUnitPrice(nanousdToUSD(rateNanousd))} / ${rateUnit} = ${formatTooltipUsageCost(nanousdToUSD(billedNanousd))}`,
  };
}

function formatTieredRangeLabel(fromTokens: number | null | undefined, upToTokens: number | null | undefined, labels: BillingTooltipLabels): string {
  const from = Number.isFinite(fromTokens ?? NaN) && (fromTokens ?? 0) > 0 ? fromTokens ?? 0 : 0;
  const upTo = Number.isFinite(upToTokens ?? NaN) && (upToTokens ?? 0) > 0 ? upToTokens ?? 0 : null;
  return labels.tieredRange(formatFormulaTokenCount(from), upTo ? formatFormulaTokenCount(upTo) : null);
}

function formatTieredTableRow(item: string, tokens: number, rateNanousd: number, billedNanousd: number): BillingTieredTableRow {
  const safeTokens = Number.isFinite(tokens) && tokens > 0 ? tokens : 0;
  const safeBilled = Number.isFinite(billedNanousd) && billedNanousd > 0 ? billedNanousd : 0;
  return {
    item,
    tokens: formatFormulaTokenCount(safeTokens),
    unitPrice: `${formatTooltipUnitPrice(nanousdToUSD(rateNanousd))} / 1M`,
    amount: formatTooltipUsageCost(nanousdToUSD(safeBilled)),
  };
}

function formatBillingTotalLine(label: string, amount: string): BillingTooltipLine {
  return { type: "row", left: label, right: amount };
}

function readServiceItemNumber(item: BillingServiceItemSnapshot, key: keyof BillingServiceItemSnapshot): number {
  const value = item[key];
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function serviceItemModelDisplayLabel(item: BillingServiceItemSnapshot): string {
  return String(item.platform_model_name || "-").trim();
}

function readMainBilledNanousd(snapshot: BillingPricingSnapshot): number {
  return (
    readSnapshotNumber(snapshot, "input_billed_nanousd") +
    readSnapshotNumber(snapshot, "cache_read_billed_nanousd") +
    readSnapshotNumber(snapshot, "cache_write_billed_nanousd") +
    readSnapshotNumber(snapshot, "output_billed_nanousd") +
    readSnapshotNumber(snapshot, "call_billed_nanousd") +
    readSnapshotNumber(snapshot, "duration_billed_nanousd")
  );
}

function readServiceItems(snapshot: BillingPricingSnapshot): BillingServiceItemSnapshot[] {
  return Array.isArray(snapshot.service_items) ? snapshot.service_items : [];
}

function readServiceItemsBilledNanousd(items: BillingServiceItemSnapshot[]): number {
  return items.reduce((total, item) => total + readServiceItemNumber(item, "billed_nanousd"), 0);
}

function buildServiceItemsSummaryLines(serviceItems: BillingServiceItemSnapshot[], labels: BillingTooltipLabels): BillingTooltipLine[] {
  if (serviceItems.length === 0) {
    return [];
  }
  return serviceItems.map((serviceItem) => {
    const serviceName = String(serviceItem.service_name || serviceItem.service_code || labels.baseService).trim();
    const amount = formatTooltipUsageCost(nanousdToUSD(readServiceItemNumber(serviceItem, "billed_nanousd")));
    return { type: "row", left: serviceName, right: amount };
  });
}

function isBaseServiceLedger(item: BillingUsageLedgerDTO): boolean {
  const snapshot = parsePricingSnapshot(item.pricingSnapshotJSON);
  return readServiceItems(snapshot).length > 0 && readMainBilledNanousd(snapshot) <= 0;
}

type UsageLogDisplayRow = {
  item: BillingUsageLedgerDTO;
  baseServiceItems: BillingServiceItemSnapshot[];
};

function buildUsageLogDisplayRows(items: BillingUsageLedgerDTO[]): UsageLogDisplayRow[] {
  const chatRows = items.filter((item) => !isBaseServiceLedger(item));
  const rows = chatRows.map((item) => ({ item, baseServiceItems: [] as BillingServiceItemSnapshot[] }));
  const serviceLedgers = items.filter(isBaseServiceLedger);
  for (const serviceLedger of serviceLedgers) {
    const serviceSnapshot = parsePricingSnapshot(serviceLedger.pricingSnapshotJSON);
    const serviceItems = readServiceItems(serviceSnapshot);
    if (serviceItems.length === 0) continue;
    const serviceTime = new Date(serviceLedger.createdAt || serviceLedger.usageDate).getTime();
    let matchedIndex = -1;
    let matchedDistance = Number.POSITIVE_INFINITY;
    for (let index = 0; index < rows.length; index += 1) {
      const row = rows[index];
      if (row.item.conversationID !== serviceLedger.conversationID) continue;
      const rowTime = new Date(row.item.createdAt || row.item.usageDate).getTime();
      const distance = Math.abs((Number.isFinite(serviceTime) ? serviceTime : 0) - (Number.isFinite(rowTime) ? rowTime : 0));
      if (distance < matchedDistance) {
        matchedDistance = distance;
        matchedIndex = index;
      }
    }
    if (matchedIndex >= 0) {
      rows[matchedIndex].baseServiceItems.push(...serviceItems);
    }
  }
  return rows;
}

function buildBaseBillingTooltipLines(serviceItems: BillingServiceItemSnapshot[], labels: BillingTooltipLabels): BillingTooltipLine[] {
  if (serviceItems.length === 0) {
    return [formatBillingTotalLine(labels.total, "$0.000000")];
  }
  const lines: BillingTooltipLine[] = serviceItems.map((serviceItem) => {
    const serviceName = String(serviceItem.service_name || serviceItem.service_code || labels.baseService).trim();
    const modelLabel = serviceItemModelDisplayLabel(serviceItem);
    const amount = formatTooltipUsageCost(nanousdToUSD(readServiceItemNumber(serviceItem, "billed_nanousd")));
    return { type: "row", left: `${serviceName} (${modelLabel})`, right: amount };
  });
  return [
    ...lines,
    { type: "divider" },
    formatBillingTotalLine(labels.total, formatTooltipUsageCost(nanousdToUSD(readServiceItemsBilledNanousd(serviceItems)))),
  ];
}

function buildServiceBillingTooltipLines(item: BillingUsageLedgerDTO, labels: BillingTooltipLabels): BillingTooltipLine[] {
  const snapshot = parsePricingSnapshot(item.pricingSnapshotJSON);
  const mainBilledNanousd = readMainBilledNanousd(snapshot);
  const currentServiceItems = readServiceItems(snapshot);
  const currentServiceBilledNanousd = readServiceItemsBilledNanousd(currentServiceItems);
  const pricingMode = normalizePricingMode(snapshot.pricing_mode);
  const inputRate = readSnapshotNumber(snapshot, "input_nanousd_per_m_tokens");
  const outputRate = readSnapshotNumber(snapshot, "output_nanousd_per_m_tokens");
  const cacheReadRate = readSnapshotNumber(snapshot, "cache_read_nanousd_per_m_tokens");
  const cacheWriteRate = readSnapshotNumber(snapshot, "cache_write_nanousd_per_m_tokens");
  const billedOutputTokens = item.outputTokens + item.reasoningTokens;
  const totalBilledNanousd = mainBilledNanousd + currentServiceBilledNanousd;
  const total = item.isFreeModel ? 0 : nanousdToUSD(totalBilledNanousd);
  const totalLine = formatBillingTotalLine(labels.total, item.isFreeModel ? `$0.000000 (${labels.freeModelNoBilling})` : formatTooltipUsageCost(total));
  const cacheWriteLabel = cacheWriteBillingLabel(snapshot, labels.display);
  const cacheWriteNote = cacheWriteBillingNote(snapshot, labels.display);
  const rateMultiplierNote = billingRateMultiplierNote(snapshot, labels.display);
  const appendCurrentServiceItems = (lines: BillingTooltipLine[]) => {
    const serviceLines = buildServiceItemsSummaryLines(currentServiceItems, labels);
    if (serviceLines.length === 0) {
      return lines;
    }
    return [...lines, { type: "divider" as const }, ...serviceLines];
  };
  if (pricingMode === "call") {
    const callRate = readSnapshotNumber(snapshot, "call_nanousd_per_call");
    const callBilled = resolveCountBilledNanousd(snapshot, "call_billed_nanousd", item.callCount, callRate);
    const lines = [
      formatCountBillingFormulaLine(labels.perCall, item.callCount, labels.callUnit, labels.callUnit, callRate, callBilled),
      ...appendCurrentServiceItems([]),
      { type: "divider" as const },
      totalLine,
    ];
    return lines;
  }
  if (pricingMode === "duration") {
    const durationRate = readSnapshotNumber(snapshot, "duration_nanousd_per_second");
    const durationBilled = resolveCountBilledNanousd(snapshot, "duration_billed_nanousd", item.durationSeconds, durationRate);
    const lines = [
      formatCountBillingFormulaLine(labels.perSecond, item.durationSeconds, labels.secondUnit, labels.secondUnit, durationRate, durationBilled),
      ...appendCurrentServiceItems([]),
      { type: "divider" as const },
      totalLine,
    ];
    return lines;
  }
  if (pricingMode === "tiered") {
    const tieredRows = [
      formatTieredTableRow(labels.input, item.inputTokens, inputRate, readSnapshotNumber(snapshot, "input_billed_nanousd")),
      formatTieredTableRow(labels.output, billedOutputTokens, outputRate, readSnapshotNumber(snapshot, "output_billed_nanousd")),
      formatTieredTableRow(labels.cacheRead, item.cacheReadTokens, cacheReadRate, readSnapshotNumber(snapshot, "cache_read_billed_nanousd")),
      formatTieredTableRow(cacheWriteLabel, item.cacheWriteTokens, cacheWriteRate, readSnapshotNumber(snapshot, "cache_write_billed_nanousd")),
    ];
    if (tieredRows.length > 0) {
      const lines: BillingTooltipLine[] = [];
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
        type: "tiered-table" as const,
        rangeLabel: formatTieredRangeLabel(snapshot.tiered_from_tokens, snapshot.tiered_up_to_tokens, labels),
        rows: tieredRows,
        totalLabel: currentServiceItems.length > 0 ? labels.subtotal : labels.total,
        totalAmount: item.isFreeModel ? `$0.000000 (${labels.freeModelNoBilling})` : formatTooltipUsageCost(nanousdToUSD(mainBilledNanousd)),
      });
      if (currentServiceItems.length > 0) {
        lines.push(...appendCurrentServiceItems([]), { type: "divider" as const }, totalLine);
      }
      return lines;
    }
  }
  const inputBilled = resolveTokenBilledNanousd(snapshot, "input_billed_nanousd", item.inputTokens, inputRate);
  const cacheReadBilled = resolveTokenBilledNanousd(snapshot, "cache_read_billed_nanousd", item.cacheReadTokens, cacheReadRate);
  const cacheWriteBilled = resolveTokenBilledNanousd(snapshot, "cache_write_billed_nanousd", item.cacheWriteTokens, cacheWriteRate);
  const outputBilled = resolveTokenBilledNanousd(snapshot, "output_billed_nanousd", billedOutputTokens, outputRate);
  const lines = [
    formatBillingFormulaLine(labels.input, item.inputTokens, inputRate, inputBilled),
    formatBillingFormulaLine(labels.output, billedOutputTokens, outputRate, outputBilled),
    formatBillingFormulaLine(labels.cacheRead, item.cacheReadTokens, cacheReadRate, cacheReadBilled),
    formatBillingFormulaLine(cacheWriteLabel, item.cacheWriteTokens, cacheWriteRate, cacheWriteBilled),
    ...appendCurrentServiceItems([]),
    { type: "divider" as const },
    totalLine,
  ];
  const noteLines: BillingTooltipLine[] = [];
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

function TooltipLines({ lines }: { lines: BillingTooltipLine[] }) {
  return (
    <div className="min-w-72 max-w-[min(92vw,44rem)] space-y-1 text-left text-xs leading-relaxed">
      {lines.map((line, index) =>
        line.type === "divider" ? (
          <Separator key={`divider-${index}`} />
        ) : line.type === "tiered-table" ? (
          <TieredBillingTable key={`tiered-table-${index}`} line={line} />
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

function TieredBillingTable({ line }: { line: Extract<BillingTooltipLine, { type: "tiered-table" }> }) {
  const t = useTranslations("settings.subscriptionPage.billingTooltip.table");
  return (
    <div className="max-w-[min(92vw,34rem)] overflow-x-auto">
      <div className="mb-1 text-[11px] font-medium text-background/80">{line.rangeLabel}</div>
      <table className="w-full border-collapse text-left tabular-nums">
        <thead>
          <tr className="border-b border-background/20 text-[11px] text-background/65">
            <th className="whitespace-nowrap px-2 pb-1 font-medium first:pl-0" aria-label={t("item")} />
            <th className="whitespace-nowrap px-2 pb-1 text-right font-medium">{t("usage")}</th>
            <th className="whitespace-nowrap px-2 pb-1 text-right font-medium">{t("unitPrice")}</th>
            <th className="whitespace-nowrap px-2 pb-1 text-right font-medium last:pr-0">{t("amount")}</th>
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

function BaseBillingSummary({ items }: { items: BillingServiceItemSnapshot[] }) {
  const labels = useBillingTooltipLabels();
  const total = readServiceItemsBilledNanousd(items);
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex cursor-default items-center font-medium tabular-nums text-foreground">{formatUsageCost(nanousdToUSD(total))}</span>
      </TooltipTrigger>
      <TooltipContent>
        <TooltipLines lines={buildBaseBillingTooltipLines(items, labels)} />
      </TooltipContent>
    </Tooltip>
  );
}

function ServiceBillingSummary({ item }: { item: BillingUsageLedgerDTO }) {
  const labels = useBillingTooltipLabels();
  const snapshot = parsePricingSnapshot(item.pricingSnapshotJSON);
  const currentServiceItems = readServiceItems(snapshot);
  const total = item.isFreeModel ? 0 : readMainBilledNanousd(snapshot) + readServiceItemsBilledNanousd(currentServiceItems);
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex cursor-default items-center font-medium tabular-nums text-foreground">{formatUsageCost(nanousdToUSD(total))}</span>
      </TooltipTrigger>
      <TooltipContent>
        <TooltipLines lines={buildServiceBillingTooltipLines(item, labels)} />
      </TooltipContent>
    </Tooltip>
  );
}

export function SubscriptionUsageLog({
  items,
  total,
  loading,
  page,
  pageSize,
  query,
  status,
  sort,
  onQueryChange,
  onStatusChange,
  onSortChange,
  onRefresh,
  onPageChange,
  onPageSizeChange,
}: {
  items: BillingUsageLedgerDTO[];
  total: number;
  loading: boolean;
  page: number;
  pageSize: number;
  query: string;
  status: string;
  sort: string;
  onQueryChange: (value: string) => void;
  onStatusChange: (value: string) => void;
  onSortChange: (value: string) => void;
  onRefresh: () => void;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
}) {
  const t = useTranslations("settings.subscriptionPage.usageLog");
  const { locale } = useAppLocale();
  const rows = React.useMemo(() => buildUsageLogDisplayRows(items), [items]);
  const statusOptions = React.useMemo(
    () => [
      { label: t("filters.all"), value: "" },
      { label: t("filters.free"), value: "free" },
      { label: t("filters.billable"), value: "billable" },
    ],
    [t],
  );
  const sortOptions = React.useMemo(
    () => [
      { label: t("sort.newest"), value: "newest" },
      { label: t("sort.oldest"), value: "oldest" },
      { label: t("sort.tokensDesc"), value: "tokens_desc" },
      { label: t("sort.costDesc"), value: "cost_desc" },
      { label: t("sort.latencyDesc"), value: "latency_desc" },
    ],
    [t],
  );
  const { visibleRows: renderedRows } = useProgressiveRows(rows, {
    initialCount: 12,
    step: 16,
    disabled: loading,
  });
  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  return (
    <div className="space-y-3">
      <div className="flex h-9 items-center">
        <h3 className="text-sm font-semibold">{t("title")}</h3>
      </div>

      <TableToolbar
        query={query}
        onQueryChange={onQueryChange}
        queryPlaceholder={t("searchModel")}
        filters={[
          {
            key: "status",
            label: t("type"),
            value: status,
            onValueChange: onStatusChange,
            options: statusOptions,
          },
        ]}
        sort={{
          value: sort,
          onValueChange: onSortChange,
          options: sortOptions,
        }}
        loading={loading}
        onRefresh={onRefresh}
      />

      <Table className="table-fixed">
        <TableHeader>
          <TableRow>
            <TableHead className="w-[10.5rem]">{t("columns.time")}</TableHead>
            <TableHead className="w-[10rem]">{t("columns.model")}</TableHead>
            <TableHead className="w-[7rem]">{t("columns.baseBilling")}</TableHead>
            <TableHead className="w-[7rem]">{t("columns.serviceBilling")}</TableHead>
            <TableHead className="w-[5rem] text-right">{t("columns.latency")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading && rows.length === 0 ? <TableSkeletonRows colSpan={5} rowCount={8} /> : null}
          {!loading && rows.length === 0 ? <TableEmptyRow colSpan={5}>{t("empty")}</TableEmptyRow> : null}
          {renderedRows.map(({ item, baseServiceItems }) => (
            <TableRow key={item.id}>
              <TableCell className="text-xs text-muted-foreground">{formatUsageLogTime(item.createdAt || item.usageDate, locale)}</TableCell>
              <TableCell className="w-[10rem] max-w-[10rem] text-xs font-medium">
                <div className="truncate" title={modelDisplayLabel(item)}>
                  {modelDisplayLabel(item)}
                </div>
              </TableCell>
              <TableCell className="text-xs">
                <BaseBillingSummary items={baseServiceItems} />
              </TableCell>
              <TableCell className="text-xs">
                <ServiceBillingSummary item={item} />
              </TableCell>
              <TableCell className="text-right text-xs text-muted-foreground">{formatLatency(item.latencyMS)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <TablePagination
        total={total}
        page={page}
        pageCount={pageCount}
        pageSize={pageSize}
        pageSizeOptions={USAGE_LOG_PAGE_SIZE_OPTIONS}
        onPageChange={onPageChange}
        onPageSizeChange={onPageSizeChange}
        loading={loading}
      />
    </div>
  );
}
