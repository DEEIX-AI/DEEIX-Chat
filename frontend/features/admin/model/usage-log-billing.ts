// 用量日志计价：pricing snapshot 解析、金额换算与账单 tooltip 行构建。

import type { AdminUsageLogDTO } from "@/features/admin/api/admin.types";
import { parseJSONRecord } from "@/features/admin/model/log-display";
import { formatBillingBalance } from "@/features/admin/utils/account-display";
import {
  type BillingDisplayLabels,
  type BillingDisplayOptions,
  billingRateMultiplierNote,
  cacheWriteBillingLabel,
  cacheWriteBillingNote,
  formatBillingDisplayCompactAmountFromUSD,
  formatBillingDisplayPreciseAmountFromUSD,
  formatBillingDisplayUnitPriceFromUSD,
} from "@/shared/lib/billing-display";

export type UsagePricingSnapshot = {
  pricing_mode?: "token" | "call" | "duration" | "tiered" | string;
  provider_protocol?: string;
  duration_billable?: boolean;
  media_type?: string;
  input_image_count?: number;
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

export type UsageBillingLabels = {
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

export type UsageBillingTieredTableRow = {
  item: string;
  tokens: string;
  unitPrice: string;
  amount: string;
};

export type UsageBillingTooltipLine =
  | { type: "row"; left: string; right: string }
  | { type: "divider" }
  | { type: "tiered-table"; rangeLabel: string; rows: UsageBillingTieredTableRow[]; totalLabel: string; totalAmount: string };

export function usageBillableOutputTokens(item: AdminUsageLogDTO): number {
  return item.outputTokens + item.reasoningTokens;
}

export function usageTotalTokens(item: AdminUsageLogDTO): number {
  return item.inputTokens + item.cacheReadTokens + item.cacheWriteTokens + usageBillableOutputTokens(item);
}

export function usageLogRawUsageJSON(item: AdminUsageLogDTO): string {
  const upstreamUsage = parseJSONRecord(item.pricingSnapshotJSON)?.upstream_usage;
  if (upstreamUsage && typeof upstreamUsage === "object") {
    return JSON.stringify(upstreamUsage, null, 2);
  }
  return "{}";
}

export function formatUsageBalance(value: number | null | undefined, billingDisplay: BillingDisplayOptions): string {
  return value === null || value === undefined ? "-" : formatBillingBalance(value, billingDisplay);
}

export function formatUsageCost(value: number, billingDisplay: BillingDisplayOptions): string {
  return formatBillingDisplayCompactAmountFromUSD(value, billingDisplay);
}

export function formatTooltipUsageCost(value: number, billingDisplay: BillingDisplayOptions): string {
  return formatBillingDisplayPreciseAmountFromUSD(value, billingDisplay);
}

export function parseUsagePricingSnapshot(raw: string): UsagePricingSnapshot {
  try {
    const parsed = JSON.parse(raw) as unknown;
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed as UsagePricingSnapshot : {};
  } catch {
    return {};
  }
}

function formatTooltipUnitPrice(value: number, billingDisplay: BillingDisplayOptions): string {
  return formatBillingDisplayUnitPriceFromUSD(value, billingDisplay);
}

function nanousdToUSD(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 0;
  return value / 1_000_000_000;
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

function usageFormulaLine(
  label: string,
  tokens: number,
  rateNanousd: number,
  billedNanousd: number,
  billingDisplay: BillingDisplayOptions,
): UsageBillingTooltipLine {
  return {
    type: "row",
    left: label,
    right: `${formatFormulaTokenCount(tokens)} tokens * ${formatTooltipUnitPrice(nanousdToUSD(rateNanousd), billingDisplay)} / 1M = ${formatTooltipUsageCost(nanousdToUSD(billedNanousd), billingDisplay)}`,
  };
}

function usageCountFormulaLine(
  label: string,
  count: number,
  unit: string,
  rateUnit: string,
  rateNanousd: number,
  billedNanousd: number,
  billingDisplay: BillingDisplayOptions,
): UsageBillingTooltipLine {
  const safeCount = Number.isFinite(count) && count > 0 ? count : 0;
  return {
    type: "row",
    left: label,
    right: `${safeCount.toLocaleString("en-US")} ${unit} * ${formatTooltipUnitPrice(nanousdToUSD(rateNanousd), billingDisplay)} / ${rateUnit} = ${formatTooltipUsageCost(nanousdToUSD(billedNanousd), billingDisplay)}`,
  };
}

function usageTieredTableRow(
  item: string,
  tokens: number,
  rateNanousd: number,
  billedNanousd: number,
  billingDisplay: BillingDisplayOptions,
): UsageBillingTieredTableRow {
  const safeTokens = Number.isFinite(tokens) && tokens > 0 ? tokens : 0;
  const safeBilled = Number.isFinite(billedNanousd) && billedNanousd > 0 ? billedNanousd : 0;
  return {
    item,
    tokens: formatFormulaTokenCount(safeTokens),
    unitPrice: `${formatTooltipUnitPrice(nanousdToUSD(rateNanousd), billingDisplay)} / 1M`,
    amount: formatTooltipUsageCost(nanousdToUSD(safeBilled), billingDisplay),
  };
}

function usageTotalLine(item: AdminUsageLogDTO, labels: UsageBillingLabels, billingDisplay: BillingDisplayOptions): UsageBillingTooltipLine {
  return {
    type: "row",
    left: labels.total,
    right: item.isFreeModel
      ? `${formatTooltipUsageCost(0, billingDisplay)} (${labels.freeModelNoBilling})`
      : formatTooltipUsageCost(nanousdToUSD(item.billedNanousd), billingDisplay),
  };
}

export function buildUsageBillingTooltipLines(
  item: AdminUsageLogDTO,
  labels: UsageBillingLabels,
  billingDisplay: BillingDisplayOptions,
): UsageBillingTooltipLine[] {
  const snapshot = parseUsagePricingSnapshot(item.pricingSnapshotJSON);
  const pricingMode = normalizePricingMode(snapshot.pricing_mode);
  const inputRate = readUsageSnapshotNumber(snapshot, "input_nanousd_per_m_tokens");
  const outputRate = readUsageSnapshotNumber(snapshot, "output_nanousd_per_m_tokens");
  const cacheReadRate = readUsageSnapshotNumber(snapshot, "cache_read_nanousd_per_m_tokens");
  const cacheWriteRate = readUsageSnapshotNumber(snapshot, "cache_write_nanousd_per_m_tokens");
  const billedOutputTokens = usageBillableOutputTokens(item);
  const totalLine = usageTotalLine(item, labels, billingDisplay);
  const cacheWriteLabel = cacheWriteBillingLabel(snapshot, labels.billingDisplay);
  const cacheWriteNote = cacheWriteBillingNote(snapshot, labels.billingDisplay);
  const rateMultiplierNote = billingRateMultiplierNote(snapshot, labels.billingDisplay);

  if (pricingMode === "call") {
    const callRate = readUsageSnapshotNumber(snapshot, "call_nanousd_per_call");
    const callBilled = resolveCountBilledNanousd(snapshot, "call_billed_nanousd", item.callCount, callRate);
    return [
      usageCountFormulaLine(labels.perCall, item.callCount, labels.callUnit, labels.callUnit, callRate, callBilled, billingDisplay),
      { type: "divider" },
      totalLine,
    ];
  }

  if (pricingMode === "duration") {
    const durationRate = readUsageSnapshotNumber(snapshot, "duration_nanousd_per_second");
    const durationBilled = resolveCountBilledNanousd(snapshot, "duration_billed_nanousd", item.durationSeconds, durationRate);
    return [
      usageCountFormulaLine(labels.perSecond, item.durationSeconds, labels.secondUnit, labels.secondUnit, durationRate, durationBilled, billingDisplay),
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
        usageTieredTableRow(labels.input, item.inputTokens, inputRate, readUsageSnapshotNumber(snapshot, "input_billed_nanousd"), billingDisplay),
        usageTieredTableRow(labels.output, billedOutputTokens, outputRate, readUsageSnapshotNumber(snapshot, "output_billed_nanousd"), billingDisplay),
        usageTieredTableRow(labels.cacheRead, item.cacheReadTokens, cacheReadRate, readUsageSnapshotNumber(snapshot, "cache_read_billed_nanousd"), billingDisplay),
        usageTieredTableRow(cacheWriteLabel, item.cacheWriteTokens, cacheWriteRate, readUsageSnapshotNumber(snapshot, "cache_write_billed_nanousd"), billingDisplay),
      ],
      totalLabel: labels.total,
      totalAmount: item.isFreeModel
        ? `${formatTooltipUsageCost(0, billingDisplay)} (${labels.freeModelNoBilling})`
        : formatTooltipUsageCost(nanousdToUSD(item.billedNanousd), billingDisplay),
    });
    return lines;
  }

  const inputBilled = resolveTokenBilledNanousd(snapshot, "input_billed_nanousd", item.inputTokens, inputRate);
  const cacheReadBilled = resolveTokenBilledNanousd(snapshot, "cache_read_billed_nanousd", item.cacheReadTokens, cacheReadRate);
  const cacheWriteBilled = resolveTokenBilledNanousd(snapshot, "cache_write_billed_nanousd", item.cacheWriteTokens, cacheWriteRate);
  const outputBilled = resolveTokenBilledNanousd(snapshot, "output_billed_nanousd", billedOutputTokens, outputRate);
  const lines: UsageBillingTooltipLine[] = [
    usageFormulaLine(labels.input, item.inputTokens, inputRate, inputBilled, billingDisplay),
    usageFormulaLine(labels.output, billedOutputTokens, outputRate, outputBilled, billingDisplay),
    usageFormulaLine(labels.cacheRead, item.cacheReadTokens, cacheReadRate, cacheReadBilled, billingDisplay),
    usageFormulaLine(cacheWriteLabel, item.cacheWriteTokens, cacheWriteRate, cacheWriteBilled, billingDisplay),
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
