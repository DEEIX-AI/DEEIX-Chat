"use client";

import * as React from "react";
import { Banknote, Check, Ticket } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ChartContainer, ChartTooltip } from "@/components/ui/chart";
import type { ChartConfig } from "@/components/ui/chart";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableRow, TableSkeletonRows } from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useProgressiveRows } from "@/hooks/use-progressive-rows";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { createBillingCheckout, getBillingConfig, getBillingOverview, listBillingDailyUsage, listBillingMonthlyUsage, listBillingPlans, listBillingUsage, redeemBillingCode, subscribeBillingPlan } from "@/shared/api/billing";
import type { BillingAccountData, BillingConfigData, BillingMode, BillingOverviewData, BillingSubscriptionEntitlementDTO, BillingUsageDailyDTO, BillingUsageLedgerDTO, BillingUsageMonthlyDTO } from "@/shared/api/billing.types";
import type { BillingPlanDTO, BillingPlanPriceDTO } from "@/shared/api/billing.types";
import {
  SettingsPage,
  SettingsSectionHeader,
} from "@/shared/components/settings-layout";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { billingRateMultiplierNote, cacheWriteBillingLabel, cacheWriteBillingNote } from "@/shared/lib/billing-display";
import type { BillingDisplayLabels } from "@/shared/lib/billing-display";
import type { UserDTO } from "@/shared/api/auth.types";
import { Bar, BarChart, CartesianGrid, Cell, XAxis, YAxis } from "recharts";

function resolveDefaultPrice(plan: BillingPlanDTO | null | undefined): BillingPlanPriceDTO | null {
  const prices = plan?.prices ?? [];
  if (prices.length === 0) {
    return null;
  }
  return prices.find((item) => item.isDefault) || prices[0] || null;
}

type BillingRuntimeConfig = BillingConfigData["config"];
type BillingAccount = BillingAccountData["account"];
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

function formatPlanPrice(price: BillingPlanPriceDTO | null, intervalLabels: { lifetime: string; year: string; month: string }): string {
  if (!price) return "-";
  const amount = new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: (price.currency || "USD").toUpperCase(),
  }).format((price.amountCents || 0) / 100);
  if (price.billingInterval === "lifetime") return `${amount} / ${intervalLabels.lifetime}`;
  if (price.billingInterval === "year") return `${amount} / ${intervalLabels.year}`;
  return `${amount} / ${intervalLabels.month}`;
}

function formatPlanCredit(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "$0";
  return `$${value.toLocaleString("en-US", { maximumFractionDigits: 2 })}`;
}

function formatAccountBalance(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "$0.000000";
  return `$${value.toLocaleString("en-US", {
    minimumFractionDigits: 6,
    maximumFractionDigits: 6,
  })}`;
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

function formatUsageSummaryCost(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "$0";
  if (value < 0.0001) return "< $0.0001";
  return `$${value.toLocaleString("en-US", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 4,
  })}`;
}

function formatUsageAxisTokens(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0";
  if (value >= 1_000_000) return `${(value / 1_000_000).toLocaleString("en-US", { maximumFractionDigits: 1 })}M`;
  if (value >= 1_000) return `${Math.round(value / 1_000).toLocaleString("en-US")}K`;
  return Math.round(value).toLocaleString("en-US");
}

function formatLatency(value: number | null | undefined): string {
  if (!Number.isFinite(value ?? NaN) || (value ?? 0) <= 0) return "-";
  const ms = value ?? 0;
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toLocaleString("en-US", { maximumFractionDigits: 2 })}s`;
}

function formatUsageTrendLatency(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0";
  return formatLatency(value);
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

function modelDisplayLabel(model: { platformModelName?: string }): string {
  return model.platformModelName?.trim() || "-";
}

function serviceItemModelDisplayLabel(item: BillingServiceItemSnapshot): string {
  return String(item.platform_model_name || "-").trim();
}

function buildServiceItemTooltipLines(item: BillingServiceItemSnapshot, labels: BillingTooltipLabels): BillingTooltipLine[] {
  const pricingMode = normalizePricingMode(item.pricing_mode);
  const serviceName = String(item.service_name || item.service_code || labels.baseService).trim();
  const lines: BillingTooltipLine[] = [{ type: "row", left: serviceName, right: serviceItemModelDisplayLabel(item) }];
  const cacheWriteLabel = cacheWriteBillingLabel(item, labels.display);
  const cacheWriteNote = cacheWriteBillingNote(item, labels.display);
  const rateMultiplierNote = billingRateMultiplierNote(item, labels.display);
  if (pricingMode === "call") {
    const callRate = readServiceItemNumber(item, "call_nanousd_per_call");
    const callCount = readServiceItemNumber(item, "call_count");
    const callBilled = readServiceItemNumber(item, "call_billed_nanousd") || Math.round(callCount * callRate);
    lines.push(formatCountBillingFormulaLine(labels.perCall, callCount, labels.callUnit, labels.callUnit, callRate, callBilled));
    lines.push(formatBillingTotalLine(labels.subtotal, formatTooltipUsageCost(nanousdToUSD(readServiceItemNumber(item, "billed_nanousd")))));
    return lines;
  }
  if (pricingMode === "duration") {
    const durationRate = readServiceItemNumber(item, "duration_nanousd_per_second");
    const durationSeconds = readServiceItemNumber(item, "duration_seconds");
    const durationBilled = readServiceItemNumber(item, "duration_billed_nanousd") || Math.round(durationSeconds * durationRate);
    lines.push(formatCountBillingFormulaLine(labels.perSecond, durationSeconds, labels.secondUnit, labels.secondUnit, durationRate, durationBilled));
    lines.push(formatBillingTotalLine(labels.subtotal, formatTooltipUsageCost(nanousdToUSD(readServiceItemNumber(item, "billed_nanousd")))));
    return lines;
  }
  if (pricingMode === "tiered") {
    const inputRate = readServiceItemNumber(item, "input_nanousd_per_m_tokens");
    const outputRate = readServiceItemNumber(item, "output_nanousd_per_m_tokens");
    const cacheReadRate = readServiceItemNumber(item, "cache_read_nanousd_per_m_tokens");
    const cacheWriteRate = readServiceItemNumber(item, "cache_write_nanousd_per_m_tokens");
    const outputTokens = readServiceItemNumber(item, "output_tokens");
    const reasoningTokens = readServiceItemNumber(item, "reasoning_tokens");
    const tieredRows = [
      formatTieredTableRow(labels.input, readServiceItemNumber(item, "input_tokens"), inputRate, readServiceItemNumber(item, "input_billed_nanousd")),
      formatTieredTableRow(labels.output, outputTokens + reasoningTokens, outputRate, readServiceItemNumber(item, "output_billed_nanousd")),
      formatTieredTableRow(labels.cacheRead, readServiceItemNumber(item, "cache_read_tokens"), cacheReadRate, readServiceItemNumber(item, "cache_read_billed_nanousd")),
      formatTieredTableRow(cacheWriteLabel, readServiceItemNumber(item, "cache_write_tokens"), cacheWriteRate, readServiceItemNumber(item, "cache_write_billed_nanousd")),
    ];
    if (tieredRows.length > 0) {
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
        rangeLabel: formatTieredRangeLabel(item.tiered_from_tokens, item.tiered_up_to_tokens, labels),
        rows: tieredRows,
        totalLabel: labels.subtotal,
        totalAmount: formatTooltipUsageCost(nanousdToUSD(readServiceItemNumber(item, "billed_nanousd"))),
      });
      return lines;
    }
  }
  const inputRate = readServiceItemNumber(item, "input_nanousd_per_m_tokens");
  const outputRate = readServiceItemNumber(item, "output_nanousd_per_m_tokens");
  const cacheReadRate = readServiceItemNumber(item, "cache_read_nanousd_per_m_tokens");
  const cacheWriteRate = readServiceItemNumber(item, "cache_write_nanousd_per_m_tokens");
  const inputTokens = readServiceItemNumber(item, "input_tokens");
  const cacheReadTokens = readServiceItemNumber(item, "cache_read_tokens");
  const cacheWriteTokens = readServiceItemNumber(item, "cache_write_tokens");
  const outputTokens = readServiceItemNumber(item, "output_tokens");
  const reasoningTokens = readServiceItemNumber(item, "reasoning_tokens");
  const billedOutputTokens = outputTokens + reasoningTokens;
  lines.push(formatBillingFormulaLine(labels.input, inputTokens, inputRate, readServiceItemNumber(item, "input_billed_nanousd") || calcTokenBilledNanousd(inputTokens, inputRate)));
  lines.push(formatBillingFormulaLine(labels.output, billedOutputTokens, outputRate, readServiceItemNumber(item, "output_billed_nanousd") || calcTokenBilledNanousd(billedOutputTokens, outputRate)));
  lines.push(formatBillingFormulaLine(labels.cacheRead, cacheReadTokens, cacheReadRate, readServiceItemNumber(item, "cache_read_billed_nanousd") || calcTokenBilledNanousd(cacheReadTokens, cacheReadRate)));
  lines.push(formatBillingFormulaLine(cacheWriteLabel, cacheWriteTokens, cacheWriteRate, readServiceItemNumber(item, "cache_write_billed_nanousd") || calcTokenBilledNanousd(cacheWriteTokens, cacheWriteRate)));
  if (rateMultiplierNote) {
    lines.push({ type: "row", left: labels.rateNote, right: rateMultiplierNote });
  }
  if (cacheWriteNote) {
    lines.push({ type: "row", left: labels.cacheNote, right: cacheWriteNote });
  }
  lines.push(formatBillingTotalLine(labels.subtotal, formatTooltipUsageCost(nanousdToUSD(readServiceItemNumber(item, "billed_nanousd")))));
  return lines;
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

function formatTokenCount(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "-";
  return value.toLocaleString("en-US");
}

function formatFormulaTokenCount(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0";
  return value.toLocaleString("en-US");
}

function formatDay(value: string | null | undefined): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${month}/${day}`;
}

function formatMonthLabel(value: string | null | undefined, locale: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, { month: "short" }).format(date);
}

function formatFullMonthLabel(value: string | null | undefined, locale: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, { year: "numeric", month: "long" }).format(date);
}

function formatShortDate(value: string | null | undefined, locale: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, {
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}

function formatMediumDate(value: string | null | undefined, locale: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}

function formatUsageLogTime(value: string | null | undefined, locale: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(date);
}

function isFreePlan(plan: BillingPlanDTO | null | undefined): boolean {
  return plan?.code?.trim() === "free";
}

function isCurrentBillingPlan(
  plan: BillingPlanDTO,
  currentPlan: BillingPlanDTO | null,
  viewer: UserDTO | null,
): boolean {
  return currentPlan?.id === plan.id || viewer?.subscriptionPlanID === plan.id || viewer?.subscriptionTier === plan.code;
}

function planRank(plan: BillingPlanDTO | null | undefined): number {
  if (!plan) return 0;
  if (Number.isFinite(plan.sortOrder) && plan.sortOrder > 0) {
    return plan.sortOrder;
  }
  if (plan.code === "ultra") return 40;
  if (plan.code === "max") return 30;
  if (plan.code === "pro") return 20;
  if (plan.code === "free") return 10;
  return plan.periodCreditUSD;
}

type PlanActionKind = "current" | "renew" | "upgrade" | "subscribe" | "switch" | "freeBlocked" | "unavailable";

type PlanActionLabels = {
  current: string;
  unavailable: string;
  renew: string;
  subscribe: string;
  switch: string;
  upgrade: string;
  freeBlocked: string;
};

function resolvePlanActionKind(
  plan: BillingPlanDTO,
  price: BillingPlanPriceDTO | null,
  isCurrent: boolean,
  currentPlan: BillingPlanDTO | null,
  protectedPaidPlanRank: number,
): PlanActionKind {
  const targetRank = planRank(plan);
  if (isCurrent) {
    if (isFreePlan(plan)) return "current";
    return price ? "renew" : "current";
  }
  if (isFreePlan(plan) && protectedPaidPlanRank > 0) return "freeBlocked";
  if (!price) return "unavailable";
  if (!price.amountCents) return "switch";
  if (!currentPlan || isFreePlan(currentPlan)) return "subscribe";

  if (protectedPaidPlanRank > targetRank) return "renew";
  const comparison = targetRank - planRank(currentPlan);
  if (comparison > 0) return "upgrade";
  return "renew";
}

function resolvePlanActionLabel(action: PlanActionKind, labels: PlanActionLabels): string {
  return labels[action];
}

function resolvePlanButtonVariant(action: PlanActionKind): "default" | "outline" | "secondary" {
  if (action === "current") return "secondary";
  if (action === "freeBlocked" || action === "unavailable" || action === "switch") return "outline";
  return "default";
}

function resolvePaymentProviderLabel(provider: string | undefined, fallback: string): string {
  if (provider === "stripe") return "Stripe";
  if (provider === "epay") return "EPay";
  return fallback;
}

function resolveEPayTypeLabel(type: string, labels: { alipay: string; wxpay: string; qqpay: string; custom: (type: string) => string }): string {
  if (type === "alipay") return labels.alipay;
  if (type === "wxpay") return labels.wxpay;
  if (type === "qqpay") return labels.qqpay;
  return labels.custom(type);
}

function resolvePlanFeatures(plan: BillingPlanDTO, labels: { monthlyCredit: (credit: string) => string; freeModelsNotIncluded: string }): string[] {
  const fallback = [
    labels.monthlyCredit(formatPlanCredit(plan.periodCreditUSD)),
    labels.freeModelsNotIncluded,
  ];
  try {
    const parsed = JSON.parse(plan.featureJSON || "null") as unknown;
    if (Array.isArray(parsed)) {
      const features = parsed.filter((item): item is string => typeof item === "string" && item.trim().length > 0);
      return features.length > 0 ? features : fallback;
    }
    if (parsed && typeof parsed === "object" && Array.isArray((parsed as { features?: unknown }).features)) {
      const features = ((parsed as { features: unknown[] }).features).filter((item): item is string => typeof item === "string" && item.trim().length > 0);
      return features.length > 0 ? features : fallback;
    }
  } catch {
    // ignore invalid admin-entered feature JSON
  }
  return plan.description ? [plan.description, ...fallback] : fallback;
}

function ActionRow({
  title,
  value,
  action,
}: {
  title: string;
  value?: string;
  action: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
      <div className="flex min-w-0 items-baseline gap-2">
        <p className="shrink-0 text-xs font-medium">{title}</p>
        {value ? <p className="max-w-[min(60vw,24rem)] truncate text-xs text-muted-foreground">{value}</p> : null}
      </div>
      <div className="self-start sm:self-auto sm:justify-self-end">{action}</div>
    </div>
  );
}

function ValueRow({
  title,
  value,
  action,
}: {
  title: string;
  value: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <p className="text-xs font-medium">{title}</p>
      <div className="flex min-w-0 max-w-full items-center gap-2 self-start rounded-lg bg-muted/35 px-2 py-1 text-xs text-muted-foreground sm:self-auto">
        <span className="max-w-[min(75vw,26rem)] truncate">{value}</span>
        {action}
      </div>
    </div>
  );
}

type DailyUsageChartPoint = {
  dayLabel: string;
  fullDayLabel: string;
  billedUsd: number;
  totalTokens: number;
  callCount: number;
  recordCount: number;
  avgLatencyMS: number;
  models: Array<BillingUsageDailyDTO["models"][number] & { color?: string }>;
  [key: string]: string | number | Array<BillingUsageDailyDTO["models"][number] & { color?: string }>;
};

type MonthlyUsageChartPoint = {
  monthLabel: string;
  fullMonthLabel: string;
  billedUsd: number;
  totalTokens: number;
  callCount: number;
  recordCount: number;
  avgLatencyMS: number;
};

type ModelSeries = {
  key: string;
  platformModelName: string;
  modelLabel: string;
  color: string;
};

type UsageTrendStats = {
  totalBilled: number;
  totalTokens: number;
  totalCalls: number;
  avgLatencyMS: number;
};

const usageTokenChartConfig = {
  totalTokens: {
    label: "Tokens",
    color: "var(--chart-1)",
  },
} satisfies ChartConfig;

const STACK_COLORS = ["var(--chart-1)", "var(--chart-2)", "var(--chart-3)", "var(--chart-4)", "var(--chart-5)"];

function MetricTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-md bg-background/55 px-3 py-2.5">
      <p className="truncate text-muted-foreground">{label}</p>
      <p className="mt-1 truncate text-sm font-medium tabular-nums text-foreground">{value}</p>
    </div>
  );
}

function UsageTrendMetricTiles({ stats }: { stats: UsageTrendStats }) {
  const t = useTranslations("settings.subscriptionPage.usageTrend.metrics");
  return (
    <div className="grid grid-cols-2 gap-2 text-xs md:grid-cols-4">
      <MetricTile label={t("totalCost")} value={formatUsageSummaryCost(stats.totalBilled)} />
      <MetricTile label={t("totalTokens")} value={formatFormulaTokenCount(stats.totalTokens)} />
      <MetricTile label={t("totalCalls")} value={stats.totalCalls.toLocaleString("en-US")} />
      <MetricTile label={t("averageLatency")} value={formatUsageTrendLatency(stats.avgLatencyMS)} />
    </div>
  );
}

function calculateDailyTrendStats(items: BillingUsageDailyDTO[]): UsageTrendStats {
  const totals = items.reduce(
    (acc, item) => {
      acc.totalBilled += item.billedUSD;
      acc.totalTokens += item.totalTokens;
      acc.totalCalls += item.callCount;
      if (item.avgLatencyMS > 0 && item.recordCount > 0) {
        acc.latency += item.avgLatencyMS * item.recordCount;
        acc.records += item.recordCount;
      }
      return acc;
    },
    { totalBilled: 0, totalTokens: 0, totalCalls: 0, latency: 0, records: 0 },
  );
  return {
    totalBilled: totals.totalBilled,
    totalTokens: totals.totalTokens,
    totalCalls: totals.totalCalls,
    avgLatencyMS: totals.records > 0 ? totals.latency / totals.records : 0,
  };
}

function calculateMonthlyTrendStats(items: BillingUsageMonthlyDTO[]): UsageTrendStats {
  const totals = items.reduce(
    (acc, item) => {
      acc.totalBilled += item.billedUSD;
      acc.totalTokens += item.totalTokens;
      acc.totalCalls += item.callCount;
      if (item.avgLatencyMS > 0 && item.recordCount > 0) {
        acc.latency += item.avgLatencyMS * item.recordCount;
        acc.records += item.recordCount;
      }
      return acc;
    },
    { totalBilled: 0, totalTokens: 0, totalCalls: 0, latency: 0, records: 0 },
  );
  return {
    totalBilled: totals.totalBilled,
    totalTokens: totals.totalTokens,
    totalCalls: totals.totalCalls,
    avgLatencyMS: totals.records > 0 ? totals.latency / totals.records : 0,
  };
}

function DailyUsageChartTooltip({
  active,
  payload,
}: {
  active?: boolean;
  payload?: Array<{
    payload?: DailyUsageChartPoint;
  }>;
}) {
  const t = useTranslations("settings.subscriptionPage.usageTrend.tooltip");
  const item = payload?.[0]?.payload;
  if (!active || !item) {
    return null;
  }

  return (
    <div className="grid min-w-[9rem] gap-1.5 rounded-md border border-border/50 bg-background px-2.5 py-2 text-xs shadow-md">
      <p className="font-medium">{item.fullDayLabel}</p>
      <div className="grid gap-1 text-muted-foreground">
        <div className="flex items-center justify-between gap-6">
          <span>{t("cost")}</span>
          <span className="font-medium text-foreground tabular-nums">{formatUsageSummaryCost(item.billedUsd)}</span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>{t("calls")}</span>
          <span className="font-medium text-foreground tabular-nums">{item.callCount.toLocaleString("en-US")}</span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>Tokens</span>
          <span className="font-medium text-foreground tabular-nums">{formatTokenCount(item.totalTokens)}</span>
        </div>
        {item.models.length > 0 ? (
          <div className="mt-1 grid gap-1 border-t border-border/50 pt-1">
            {item.models.slice(0, 6).map((model) => (
              <div key={model.platformModelName || "unknown"} className="flex items-center justify-between gap-6">
                <span className="flex min-w-0 items-center gap-1.5">
                  <span className="size-2 shrink-0 rounded-full" style={{ backgroundColor: model.color || "var(--foreground)" }} />
                  <span className="max-w-[8rem] truncate">{modelDisplayLabel(model)}</span>
                </span>
                <span className="font-medium text-foreground tabular-nums">{formatTokenCount(model.totalTokens)}</span>
              </div>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}

function isTopStackSegment(point: DailyUsageChartPoint, modelSeries: ModelSeries[], modelIndex: number): boolean {
  const current = Number(point[modelSeries[modelIndex]?.key] ?? 0);
  if (current <= 0) return false;
  for (let index = modelIndex + 1; index < modelSeries.length; index += 1) {
    if (Number(point[modelSeries[index].key] ?? 0) > 0) {
      return false;
    }
  }
  return true;
}

function DailyUsageChart({
  items,
  loading,
}: {
  items: BillingUsageDailyDTO[];
  loading: boolean;
}) {
  const t = useTranslations("settings.subscriptionPage.usageTrend");
  const { locale } = useAppLocale();
  const modelSeries = React.useMemo<ModelSeries[]>(() => {
    const totals = new Map<string, { totalTokens: number; label: string }>();
    for (const item of items) {
      for (const model of item.models ?? []) {
        const platformModelName = model.platformModelName || "-";
        const label = modelDisplayLabel(model);
        const current = totals.get(platformModelName);
        totals.set(platformModelName, {
          totalTokens: (current?.totalTokens ?? 0) + model.totalTokens,
          label: current?.label && current.label !== "-" ? current.label : label,
        });
      }
    }
    return Array.from(totals.entries())
      .sort((left, right) => right[1].totalTokens - left[1].totalTokens || left[0].localeCompare(right[0]))
      .map(([platformModelName, summary], index) => ({
        key: `model_${index}`,
        platformModelName,
        modelLabel: summary.label,
        color: STACK_COLORS[index % STACK_COLORS.length],
      }));
  }, [items]);
  const modelKeyByName = React.useMemo(() => new Map(modelSeries.map((item) => [item.platformModelName, item.key])), [modelSeries]);
  const modelColorByName = React.useMemo(() => new Map(modelSeries.map((item) => [item.platformModelName, item.color])), [modelSeries]);
  const chartData = React.useMemo<DailyUsageChartPoint[]>(
    () =>
      [...items]
        .sort((left, right) => new Date(left.usageDate).getTime() - new Date(right.usageDate).getTime())
        .map((item) => {
          const point: DailyUsageChartPoint = {
            dayLabel: formatDay(item.usageDate),
            fullDayLabel: formatShortDate(item.usageDate, locale),
            billedUsd: item.billedUSD,
            totalTokens: item.totalTokens,
            callCount: item.callCount,
            recordCount: item.recordCount,
            avgLatencyMS: item.avgLatencyMS,
            models: (item.models ?? []).map((model) => ({
              ...model,
              color: modelColorByName.get(model.platformModelName || "-"),
            })),
          };
          for (const model of item.models ?? []) {
            const key = modelKeyByName.get(model.platformModelName || "-");
            if (key) {
              point[key] = model.totalTokens;
            }
          }
          return point;
        }),
    [items, locale, modelColorByName, modelKeyByName]
  );
  const chartConfig = React.useMemo<ChartConfig>(() => {
    if (modelSeries.length === 0) return usageTokenChartConfig;
    return Object.fromEntries(modelSeries.map((item) => [item.key, { label: item.modelLabel, color: item.color }])) satisfies ChartConfig;
  }, [modelSeries]);
  const rangeLabel = chartData.length > 0 ? `${chartData[0].fullDayLabel} - ${chartData[chartData.length - 1].fullDayLabel}` : "";
  const hasUsageData = chartData.some((item) => item.billedUsd > 0 || item.totalTokens > 0 || item.callCount > 0 || item.recordCount > 0);

  return (
    <div className="space-y-3 rounded-md bg-muted/35 p-3">
      <div className="flex h-7 items-center justify-between gap-3 px-1">
        <p className="text-xs font-medium text-foreground">{t("dailyUsage")}</p>
        {rangeLabel ? <p className="truncate text-xs text-muted-foreground">{rangeLabel}</p> : null}
      </div>
      {loading ? <UsageChartSkeleton /> : null}
      {!loading && !hasUsageData ? <div className="flex h-[220px] items-center justify-center text-xs text-muted-foreground">{t("empty")}</div> : null}
      {!loading && hasUsageData ? (
        <>
          <ChartContainer config={chartConfig} className="h-[260px] w-full aspect-auto">
            <BarChart data={chartData} margin={{ top: 8, right: 8, left: 8, bottom: 0 }}>
              <CartesianGrid vertical={false} strokeDasharray="3 3" />
              <XAxis dataKey="dayLabel" tickLine={false} axisLine={false} tickMargin={8} interval="preserveStartEnd" />
              <YAxis
                width={64}
                tickLine={false}
                axisLine={false}
                tickMargin={6}
                tickFormatter={(value: number) => formatUsageAxisTokens(value)}
              />
              <ChartTooltip cursor={false} content={<DailyUsageChartTooltip />} />
              {modelSeries.length > 0 ? (
                modelSeries.map((model, modelIndex) => (
                  <Bar key={model.key} dataKey={model.key} stackId="usage" fill={model.color} maxBarSize={42}>
                    {chartData.map((point) => (
                      <Cell
                        key={`${model.key}-${point.fullDayLabel}`}
                        radius={(isTopStackSegment(point, modelSeries, modelIndex) ? [4, 4, 0, 0] : [0, 0, 0, 0]) as unknown as number}
                      />
                    ))}
                  </Bar>
                ))
              ) : (
                <Bar dataKey="totalTokens" fill="var(--color-totalTokens)" radius={[4, 4, 0, 0]} maxBarSize={42} />
              )}
            </BarChart>
          </ChartContainer>
        </>
      ) : null}
    </div>
  );
}

function UsageChartSkeleton() {
  return (
    <div className="flex h-[260px] items-end gap-2 px-2 pb-8 pt-8">
      {Array.from({ length: 12 }).map((_, index) => (
        <Skeleton
          key={`usage-chart-skeleton-${index}`}
          className="flex-1 rounded-t-sm"
          style={{ height: `${28 + ((index * 17) % 58)}%` }}
        />
      ))}
    </div>
  );
}

function MonthlyUsageChartTooltip({
  active,
  payload,
}: {
  active?: boolean;
  payload?: Array<{
    payload?: MonthlyUsageChartPoint;
  }>;
}) {
  const t = useTranslations("settings.subscriptionPage.usageTrend.tooltip");
  const item = payload?.[0]?.payload;
  if (!active || !item) {
    return null;
  }

  return (
    <div className="grid min-w-[9rem] gap-1.5 rounded-md border border-border/50 bg-background px-2.5 py-2 text-xs shadow-md">
      <p className="font-medium">{item.fullMonthLabel}</p>
      <div className="grid gap-1 text-muted-foreground">
        <div className="flex items-center justify-between gap-6">
          <span>{t("cost")}</span>
          <span className="font-medium text-foreground tabular-nums">{formatUsageSummaryCost(item.billedUsd)}</span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>{t("calls")}</span>
          <span className="font-medium text-foreground tabular-nums">{item.callCount.toLocaleString("en-US")}</span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>Tokens</span>
          <span className="font-medium text-foreground tabular-nums">{formatTokenCount(item.totalTokens)}</span>
        </div>
      </div>
    </div>
  );
}

function MonthlyUsageChart({
  items,
  loading,
}: {
  items: BillingUsageMonthlyDTO[];
  loading: boolean;
}) {
  const t = useTranslations("settings.subscriptionPage.usageTrend");
  const { locale } = useAppLocale();
  const chartData = React.useMemo<MonthlyUsageChartPoint[]>(
    () =>
      [...items]
        .sort((left, right) => new Date(left.monthStartAt).getTime() - new Date(right.monthStartAt).getTime())
        .map((item) => ({
          monthLabel: formatMonthLabel(item.monthStartAt, locale),
          fullMonthLabel: formatFullMonthLabel(item.monthStartAt, locale),
          billedUsd: item.billedUSD,
          totalTokens: item.totalTokens,
          callCount: item.callCount,
          recordCount: item.recordCount,
          avgLatencyMS: item.avgLatencyMS,
        })),
    [items, locale]
  );
  const rangeLabel = chartData.length > 0 ? `${chartData[0].fullMonthLabel} - ${chartData[chartData.length - 1].fullMonthLabel}` : "";
  const hasUsageData = chartData.some((item) => item.billedUsd > 0 || item.totalTokens > 0 || item.callCount > 0 || item.recordCount > 0);

  return (
    <div className="space-y-3 rounded-md bg-muted/35 p-3">
      <div className="flex h-7 items-center justify-between gap-3 px-1">
        <p className="text-xs font-medium text-foreground">{t("monthlyUsage")}</p>
        {rangeLabel ? <p className="truncate text-xs text-muted-foreground">{rangeLabel}</p> : null}
      </div>
      {loading ? <UsageChartSkeleton /> : null}
      {!loading && !hasUsageData ? <div className="flex h-[220px] items-center justify-center text-xs text-muted-foreground">{t("empty")}</div> : null}
      {!loading && hasUsageData ? (
        <ChartContainer config={usageTokenChartConfig} className="h-[260px] w-full aspect-auto">
          <BarChart data={chartData} margin={{ top: 8, right: 8, left: 8, bottom: 0 }}>
            <CartesianGrid vertical={false} strokeDasharray="3 3" />
            <XAxis dataKey="monthLabel" tickLine={false} axisLine={false} tickMargin={8} />
            <YAxis width={64} tickLine={false} axisLine={false} tickMargin={6} tickFormatter={(value: number) => formatUsageAxisTokens(value)} />
            <ChartTooltip cursor={false} content={<MonthlyUsageChartTooltip />} />
            <Bar dataKey="totalTokens" fill="var(--color-totalTokens)" radius={[4, 4, 2, 2]} maxBarSize={42} />
          </BarChart>
        </ChartContainer>
      ) : null}
    </div>
  );
}

function UsageLogTable({
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

type SubscriptionEntitlementQueueLabels = {
  title: string;
  count: (count: number) => string;
  current: string;
  upcoming: string;
  range: (start: string, end: string) => string;
  credit: (credit: string) => string;
};

function entitlementTimeMS(value: string | null | undefined): number | null {
  if (!value) return null;
  const time = new Date(value).getTime();
  return Number.isFinite(time) ? time : null;
}

function SubscriptionEntitlementQueue({
  items,
  labels,
  locale,
}: {
  items: BillingSubscriptionEntitlementDTO[];
  labels: SubscriptionEntitlementQueueLabels;
  locale: string;
}) {
  if (items.length === 0) {
    return null;
  }
  const orderedItems = [...items].sort((left, right) => {
    const leftStart = entitlementTimeMS(left.currentPeriodStartAt || left.startAt) ?? 0;
    const rightStart = entitlementTimeMS(right.currentPeriodStartAt || right.startAt) ?? 0;
    if (leftStart !== rightStart) return leftStart - rightStart;
    return left.id - right.id;
  });

  return (
    <div className="px-1 text-xs text-muted-foreground">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <span className="font-medium text-foreground">{labels.title}</span>
        <span>{labels.count(items.length)}</span>
        <span className="text-muted-foreground/50">/</span>
        {orderedItems.map((item, index) => {
          const start = formatMediumDate(item.currentPeriodStartAt || item.startAt, locale);
          const end = item.currentPeriodEndAt ? formatMediumDate(item.currentPeriodEndAt, locale) : "-";
          return (
            <React.Fragment key={`${item.id}-${item.currentPeriodStartAt}`}>
              {index > 0 ? <span className="text-muted-foreground/50">/</span> : null}
              <span
                className={item.isCurrent ? "font-medium text-foreground" : undefined}
                title={`${labels.range(start, end)} · ${labels.credit(formatPlanCredit(item.plan.periodCreditUSD))}`}
              >
                {item.plan.name || item.plan.code}
              </span>
              <span className="tabular-nums">{labels.range(start, end)}</span>
            </React.Fragment>
          );
        })}
      </div>
    </div>
  );
}

export function SettingsSubscription() {
  const t = useTranslations("settings.subscriptionPage");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const { locale } = useAppLocale();
  const { accessToken, user } = useAuthSession();
  const [viewer, setViewer] = React.useState<UserDTO | null>(null);
  const [billingPlans, setBillingPlans] = React.useState<BillingPlanDTO[]>([]);
  const [billingConfig, setBillingConfig] = React.useState<BillingRuntimeConfig | null>(null);
  const [billingOverview, setBillingOverview] = React.useState<BillingOverviewData["overview"] | null>(null);
  const [usageLedgers, setUsageLedgers] = React.useState<BillingUsageLedgerDTO[]>([]);
  const [dailyUsage, setDailyUsage] = React.useState<BillingUsageDailyDTO[]>([]);
  const [monthlyUsage, setMonthlyUsage] = React.useState<BillingUsageMonthlyDTO[]>([]);
  const [usageTotal, setUsageTotal] = React.useState(0);
  const [usagePage, setUsagePage] = React.useState(1);
  const [usagePageSize, setUsagePageSize] = React.useState(25);
  const [usageQuery, setUsageQuery] = React.useState("");
  const [usageStatus, setUsageStatus] = React.useState("");
  const [usageSort, setUsageSort] = React.useState("newest");
  const [usageView, setUsageView] = React.useState<"daily" | "monthly">("daily");
  const billingMode = billingConfig?.mode ?? "self";
  const [billingLoading, setBillingLoading] = React.useState(true);
  const [usageLoading, setUsageLoading] = React.useState(true);
  const [checkoutPriceID, setCheckoutPriceID] = React.useState<number | null>(null);
  const [topUpAmount, setTopUpAmount] = React.useState("20");
  const [topUpLoading, setTopUpLoading] = React.useState(false);
  const [pricingDialogOpen, setPricingDialogOpen] = React.useState(false);
  const [paymentDialogOpen, setPaymentDialogOpen] = React.useState(false);
  const [selectedPlan, setSelectedPlan] = React.useState<BillingPlanDTO | null>(null);
  const [selectedPrice, setSelectedPrice] = React.useState<BillingPlanPriceDTO | null>(null);
  const [selectedPaymentProvider, setSelectedPaymentProvider] = React.useState<"stripe" | "epay">("stripe");
  const [selectedEPayType, setSelectedEPayType] = React.useState("alipay");
  const [topUpDialogOpen, setTopUpDialogOpen] = React.useState(false);
  const [redemptionDialogOpen, setRedemptionDialogOpen] = React.useState(false);
  const [redemptionCode, setRedemptionCode] = React.useState("");
  const [redemptionLoading, setRedemptionLoading] = React.useState(false);
  const intervalLabels = React.useMemo(
    () => ({
      lifetime: t("interval.lifetime"),
      year: t("interval.year"),
      month: t("interval.month"),
    }),
    [t],
  );
  const planActionLabels = React.useMemo(
    () => ({
      current: t("plans.actions.current"),
      unavailable: t("plans.actions.unavailable"),
      renew: t("plans.actions.renew"),
      subscribe: t("plans.actions.subscribe"),
      switch: t("plans.actions.switch"),
      upgrade: t("plans.actions.upgrade"),
      freeBlocked: t("plans.actions.freeBlocked"),
    }),
    [t],
  );
  const planFeatureLabels = React.useMemo(
    () => ({
      monthlyCredit: (credit: string) => t("plans.features.monthlyCredit", { credit }),
      freeModelsNotIncluded: t("plans.features.freeModelsNotIncluded"),
    }),
    [t],
  );
  const entitlementLabels = React.useMemo<SubscriptionEntitlementQueueLabels>(
    () => ({
      title: t("entitlements.title"),
      count: (count) => t("entitlements.count", { count }),
      current: t("entitlements.current"),
      upcoming: t("entitlements.upcoming"),
      range: (start, end) => t("entitlements.range", { start, end }),
      credit: (credit) => t("entitlements.credit", { credit }),
    }),
    [t],
  );
  const epayLabels = React.useMemo(
    () => ({
      alipay: t("payment.epay.alipay"),
      wxpay: t("payment.epay.wxpay"),
      qqpay: t("payment.epay.qqpay"),
      custom: (type: string) => t("payment.epay.custom", { type }),
    }),
    [t],
  );

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
      .then(({ viewer: nextViewer, config, plans, overview, dailyUsage, monthlyUsage }) => {
        if (mounted) {
          setViewer(nextViewer);
          setBillingConfig(config);
          setBillingPlans(plans);
          setBillingOverview(overview);
          setDailyUsage(dailyUsage ?? []);
          setMonthlyUsage(monthlyUsage ?? []);
        }
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

  const epayTypes = React.useMemo(() => {
    const values = billingConfig?.epayTypes?.filter((item) => item.type.trim()) ?? [];
    return values.length > 0 ? values : [{ name: epayLabels.alipay, type: "alipay" }, { name: epayLabels.wxpay, type: "wxpay" }];
  }, [billingConfig?.epayTypes, epayLabels.alipay, epayLabels.wxpay]);
  const paymentProviders = React.useMemo(() => billingConfig?.paymentProviders?.filter((item) => item === "stripe" || item === "epay") ?? [], [billingConfig?.paymentProviders]);

  React.useEffect(() => {
    if (paymentProviders.length > 0 && !paymentProviders.includes(selectedPaymentProvider)) {
      setSelectedPaymentProvider(paymentProviders[0] ?? "stripe");
    }
  }, [paymentProviders, selectedPaymentProvider]);

  React.useEffect(() => {
    if (selectedPaymentProvider !== "epay") return;
    if (!epayTypes.some((item) => item.type === selectedEPayType)) {
      setSelectedEPayType(epayTypes[0]?.type ?? "alipay");
    }
  }, [epayTypes, selectedEPayType, selectedPaymentProvider]);

  const handleCheckout = React.useCallback(async (price: BillingPlanPriceDTO, paymentProvider: "stripe" | "epay", epayType?: string) => {
    setCheckoutPriceID(price.id);
    try {
      const data = await createBillingCheckout(accessToken, {
        orderType: "subscription",
        priceID: price.id,
        cycles: 1,
        paymentProvider,
        epayType: paymentProvider === "epay" ? epayType : undefined,
        successURL: `${window.location.origin}/setting/subscription?payment=success`,
        cancelURL: `${window.location.origin}/setting/subscription?payment=cancel`,
      });
      if (!data.checkout.checkoutURL) {
        toast.error(t("toasts.checkoutCreateFailed"), { description: t("toasts.checkoutURLMissing") });
        return;
      }
      window.open(data.checkout.checkoutURL, "_blank", "noopener,noreferrer");
    } catch (error) {
      toast.error(t("toasts.checkoutCreateFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
    } finally {
      setCheckoutPriceID(null);
    }
  }, [accessToken, resolveErrorMessage, t]);

  const handleSubscribeFreePlan = React.useCallback(async (price: BillingPlanPriceDTO) => {
    setCheckoutPriceID(price.id);
    try {
      await subscribeBillingPlan(accessToken, price.id);
      toast.success(t("toasts.planUpdated"));
      window.location.reload();
    } catch (error) {
      toast.error(t("toasts.subscribeFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
    } finally {
      setCheckoutPriceID(null);
    }
  }, [accessToken, resolveErrorMessage, t]);

  const handleTopUp = React.useCallback(async () => {
    const amount = Number(topUpAmount);
    if (!Number.isFinite(amount) || amount <= 0) {
      toast.error(t("toasts.invalidTopUpAmount"), { description: t("toasts.invalidTopUpAmountDescription") });
      return;
    }
    setTopUpLoading(true);
    try {
      const data = await createBillingCheckout(accessToken, {
        orderType: "topup",
        amountUSD: amount,
        cycles: 1,
        paymentProvider: selectedPaymentProvider,
        epayType: selectedPaymentProvider === "epay" ? selectedEPayType : undefined,
        successURL: `${window.location.origin}/setting/subscription?payment=success`,
        cancelURL: `${window.location.origin}/setting/subscription?payment=cancel`,
      });
      if (!data.checkout.checkoutURL) {
        toast.error(t("toasts.checkoutCreateFailed"), { description: t("toasts.checkoutURLMissing") });
        return;
      }
      window.open(data.checkout.checkoutURL, "_blank", "noopener,noreferrer");
    } catch (error) {
      toast.error(t("toasts.checkoutCreateFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
    } finally {
      setTopUpLoading(false);
    }
  }, [accessToken, resolveErrorMessage, selectedEPayType, selectedPaymentProvider, t, topUpAmount]);

  const handleRedeemCode = React.useCallback(async () => {
    const code = redemptionCode.trim();
    if (!code) {
      toast.error(t("toasts.invalidRedemptionCode"));
      return;
    }
    setRedemptionLoading(true);
    try {
      const data = await redeemBillingCode(accessToken, { code });
      setBillingOverview(data.overview);
      setRedemptionDialogOpen(false);
      setRedemptionCode("");
      toast.success(t("toasts.redemptionSucceeded"));
    } catch (error) {
      toast.error(t("toasts.redemptionFailed"), { description: resolveErrorMessage(error, t("toasts.retryLater")) });
    } finally {
      setRedemptionLoading(false);
    }
  }, [accessToken, redemptionCode, resolveErrorMessage, t]);

  const subscriptionEntitlements = React.useMemo(
    () => billingOverview?.subscriptionEntitlements ?? [],
    [billingOverview?.subscriptionEntitlements],
  );
  const paymentDisabled = paymentProviders.length === 0;
  const currentPlan = React.useMemo(() => {
    if (billingOverview?.plan) return billingOverview.plan;
    return billingPlans.find((plan) => viewer?.subscriptionPlanID === plan.id || viewer?.subscriptionTier === plan.code) ?? null;
  }, [billingOverview?.plan, billingPlans, viewer?.subscriptionPlanID, viewer?.subscriptionTier]);
  const currentPrice = React.useMemo(() => resolveDefaultPrice(currentPlan), [currentPlan]);
  const protectedPaidPlanRank = React.useMemo(
    () => Math.max(
      currentPlan && !isFreePlan(currentPlan) ? planRank(currentPlan) : 0,
      ...subscriptionEntitlements.map((item) => isFreePlan(item.plan) ? 0 : planRank(item.plan)),
    ),
    [currentPlan, subscriptionEntitlements],
  );

  const handleSelectPlan = React.useCallback(
    async (plan: BillingPlanDTO, price: BillingPlanPriceDTO | null, isCurrent: boolean) => {
      if (isCurrent && isFreePlan(plan)) {
        return;
      }
      if (!price) {
        toast.error(t("toasts.planUnavailable"), { description: t("toasts.planUnavailableDescription") });
        return;
      }
      const actionKind = resolvePlanActionKind(
        plan,
        price,
        isCurrent,
        currentPlan,
        protectedPaidPlanRank,
      );
      if (actionKind === "freeBlocked") {
        toast.error(t("toasts.freeSwitchBlocked"), { description: t("toasts.freeSwitchBlockedDescription") });
        return;
      }
      if (price.amountCents > 0) {
        if (paymentDisabled) {
          toast.error(t("toasts.paymentDisabled"), { description: t("toasts.paymentDisabledDescription") });
          return;
        }
        setSelectedPlan(plan);
        setSelectedPrice(price);
        setPricingDialogOpen(false);
        setPaymentDialogOpen(true);
        return;
      }
      await handleSubscribeFreePlan(price);
    },
    [currentPlan, handleSubscribeFreePlan, paymentDisabled, protectedPaidPlanRank, t],
  );

  const handleConfirmPayment = React.useCallback(async () => {
    if (!selectedPrice) {
      toast.error(t("toasts.noPlanSelected"), { description: t("toasts.noPlanSelectedDescription") });
      return;
    }
    await handleCheckout(selectedPrice, selectedPaymentProvider, selectedEPayType);
  }, [handleCheckout, selectedEPayType, selectedPaymentProvider, selectedPrice, t]);

  const periodCredit = billingOverview?.periodCreditUSD ?? currentPlan?.periodCreditUSD ?? 0;
  const periodUsed = billingOverview?.periodUsedUSD ?? 0;
  const periodRemaining = billingOverview?.periodRemainingUSD ?? Math.max(0, periodCredit - periodUsed);
  const periodPercent = periodCredit > 0 ? Math.min(100, Math.max(0, (periodUsed / periodCredit) * 100)) : 0;
  const billingAccount: BillingAccount | null = billingOverview?.account ?? null;
  const selectedPlanActionKind = selectedPlan
    ? resolvePlanActionKind(
      selectedPlan,
      selectedPrice,
      isCurrentBillingPlan(selectedPlan, currentPlan, viewer),
      currentPlan,
      protectedPaidPlanRank,
    )
    : "subscribe";
  const selectedRenewStartsAfterHigher = Boolean(
    selectedPlan
      && selectedPlanActionKind === "renew"
      && protectedPaidPlanRank > planRank(selectedPlan),
  );
  const paymentTitle = selectedPlanActionKind === "renew"
    ? t("payment.renewTitle")
    : selectedPlanActionKind === "upgrade"
      ? t("payment.upgradeTitle")
      : t("payment.title");
  const paymentImpactDescription = selectedPlanActionKind === "renew"
    ? selectedRenewStartsAfterHigher
      ? t("payment.renewAfterHigherDescription")
      : t("payment.renewDescription")
    : selectedPlanActionKind === "upgrade"
      ? t("payment.upgradeDescription")
      : null;
  const trendStats = React.useMemo(
    () => (usageView === "daily" ? calculateDailyTrendStats(dailyUsage) : calculateMonthlyTrendStats(monthlyUsage)),
    [dailyUsage, monthlyUsage, usageView],
  );

  return (
    <SettingsPage className="space-y-6">
      <SettingsSectionHeader title={t("title")} className="px-1" />

      {billingMode === "period" ? (
        <section className="space-y-6 px-0.5 md:space-y-7 xl:space-y-8 xl:px-1">
          <div className="space-y-4 md:space-y-5">
            <div className="flex items-start justify-between gap-3 md:gap-4">
              <div className="min-w-0 space-y-1">
                <p className="text-xs font-medium">{t("currentSubscription.title")}</p>
                <p className="truncate text-sm font-semibold">{currentPlan?.name ?? t("currentSubscription.none")}</p>
                <p className="text-xs text-muted-foreground">
                  {currentPlan ? `${formatPlanPrice(currentPrice, intervalLabels)} · ${t("plans.features.monthlyCredit", { credit: formatPlanCredit(periodCredit) })}` : t("currentSubscription.empty")}
                </p>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <Button type="button" variant="outline" disabled={billingLoading || redemptionLoading} onClick={() => setRedemptionDialogOpen(true)}>
                  <Ticket className="size-3.5" />
                  {t("redemption.open")}
                </Button>
                <Button type="button" variant="outline" disabled={billingLoading || billingPlans.length === 0} onClick={() => setPricingDialogOpen(true)}>
                  <Banknote className="size-3.5" />
                  {t("currentSubscription.subscribe")}
                </Button>
              </div>
            </div>
          </div>

          <SubscriptionEntitlementQueue
            items={subscriptionEntitlements}
            labels={entitlementLabels}
            locale={locale}
          />

          <Separator />

          <div className="space-y-3 rounded-md bg-muted/35 p-3 md:space-y-4">
            <div className="flex items-start justify-between gap-3 md:gap-4">
              <div className="space-y-1">
                <p className="text-xs font-medium">{t("periodUsage.title")}</p>
                <p className="text-xs text-muted-foreground">
                  {billingOverview?.periodStartAt && billingOverview?.periodEndAt
                    ? `${formatShortDate(billingOverview.periodStartAt, locale)} - ${formatShortDate(billingOverview.periodEndAt, locale)}`
                    : t("periodUsage.currentPeriod")}
                </p>
              </div>
              <p className="shrink-0 text-xs font-medium text-muted-foreground">{Math.round(periodPercent)}%</p>
            </div>
            <div className="space-y-2">
              <div className="flex items-center justify-between gap-4 text-xs">
                <span className="text-muted-foreground">{t("periodUsage.used", { value: formatPlanCredit(periodUsed) })}</span>
                <span className="text-muted-foreground">{t("periodUsage.total", { value: formatPlanCredit(periodCredit) })}</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-muted">
                <div className="h-full rounded-full bg-foreground/70" style={{ width: `${periodPercent}%` }} />
              </div>
            </div>
          </div>
        </section>
      ) : null}

      {billingMode === "usage" ? (
        <section className="space-y-6 px-0.5 md:space-y-7 xl:space-y-8 xl:px-1">
          <ActionRow
            title={t("usageBilling.title")}
            value={t("usageBilling.balance", { value: formatAccountBalance(billingAccount?.balanceUSD ?? 0) })}
            action={
              <div className="flex items-center gap-2">
                <Button type="button" variant="outline" disabled={billingLoading || redemptionLoading} onClick={() => setRedemptionDialogOpen(true)}>
                  <Ticket className="size-3.5" />
                  {t("redemption.open")}
                </Button>
                <Button type="button" variant="outline" disabled={billingLoading || topUpLoading || paymentDisabled} onClick={() => setTopUpDialogOpen(true)}>
                  <Banknote className="size-3.5" />
                  {t("usageBilling.topUp")}
                </Button>
              </div>
            }
          />
        </section>
      ) : null}

      {billingMode === "self" ? (
        <section className="space-y-6 px-0.5 md:space-y-7 xl:space-y-8 xl:px-1">
          <ValueRow title={t("selfMode.title")} value={t("selfMode.value")} />
        </section>
      ) : null}

      <section className="space-y-6 px-0.5 md:space-y-7 xl:space-y-8 xl:px-1">
        <Separator />
        <div className="space-y-4 md:space-y-5">
          <div className="flex h-9 items-center justify-between gap-3">
            <h3 className="text-sm font-semibold">{usageView === "daily" ? t("usageTrend.dailyTitle") : t("usageTrend.monthlyTitle")}</h3>
            <div className="inline-flex items-center gap-1 rounded-full bg-muted/40 p-1">
              <button
                type="button"
                className={`rounded-full px-2.5 py-1 text-[11px] font-medium transition-colors ${usageView === "daily" ? "bg-background text-foreground shadow-xs" : "text-foreground/60 hover:text-foreground"}`}
                onClick={() => setUsageView("daily")}
              >
                {t("usageTrend.daily")}
              </button>
              <button
                type="button"
                className={`rounded-full px-2.5 py-1 text-[11px] font-medium transition-colors ${usageView === "monthly" ? "bg-background text-foreground shadow-xs" : "text-foreground/60 hover:text-foreground"}`}
                onClick={() => setUsageView("monthly")}
              >
                {t("usageTrend.monthly")}
              </button>
            </div>
          </div>
          <UsageTrendMetricTiles stats={trendStats} />
          {usageView === "daily" ? <DailyUsageChart items={dailyUsage} loading={billingLoading} /> : <MonthlyUsageChart items={monthlyUsage} loading={billingLoading} />}
        </div>
        <Separator />
        <UsageLogTable
          items={usageLedgers}
          total={usageTotal}
          loading={usageLoading}
          page={usagePage}
          pageSize={usagePageSize}
          query={usageQuery}
          status={usageStatus}
          sort={usageSort}
          onQueryChange={(value) => {
            setUsageQuery(value);
            setUsagePage(1);
          }}
          onStatusChange={(value) => {
            setUsageStatus(value);
            setUsagePage(1);
          }}
          onSortChange={(value) => {
            setUsageSort(value);
            setUsagePage(1);
          }}
          onRefresh={() => void loadUsageLogs(usagePage, usagePageSize, usageQuery, usageStatus, usageSort)}
          onPageChange={setUsagePage}
          onPageSizeChange={(nextPageSize) => {
            setUsagePageSize(nextPageSize);
            setUsagePage(1);
          }}
        />
      </section>

      <Dialog open={pricingDialogOpen} onOpenChange={setPricingDialogOpen}>
        <DialogContent className="xl:max-w-[1040px] xl:p-6">
          <DialogHeader>
            <DialogTitle>{t("plans.title")}</DialogTitle>
          </DialogHeader>

          <div className="space-y-2 xl:hidden">
            {billingPlans.map((plan) => {
              const price = resolveDefaultPrice(plan);
              const isCurrent = isCurrentBillingPlan(plan, currentPlan, viewer);
              const actionKind = resolvePlanActionKind(plan, price, isCurrent, currentPlan, protectedPaidPlanRank);
              const actionLabel = resolvePlanActionLabel(actionKind, planActionLabels);
              const disabled = billingLoading || actionKind === "current" || actionKind === "freeBlocked" || actionKind === "unavailable" || checkoutPriceID === price?.id;
              const isSelected = selectedPlan?.id === plan.id;
              const isHighlighted = isCurrent || isSelected;
              const buttonVariant = resolvePlanButtonVariant(actionKind);
              const actionButton = (
                <Button
                  type="button"
                  size="sm"
                  className="h-8 shrink-0 px-3 shadow-none"
                  variant={buttonVariant}
                  disabled={disabled}
                  onClick={() => void handleSelectPlan(plan, price, isCurrent)}
                >
                  {checkoutPriceID === price?.id ? <SpinnerLabel>{t("actions.processing")}</SpinnerLabel> : actionLabel}
                </Button>
              );
              return (
                <div
                  key={plan.id}
                  className={[
                    "flex items-center justify-between gap-3 rounded-md bg-muted/30 px-3 py-3 transition-colors",
                    isHighlighted ? "ring-1 ring-foreground" : "hover:bg-muted/45",
                  ].join(" ")}
                >
                  <div className="min-w-0 space-y-1">
                    <div className="flex min-w-0 items-center gap-2">
                      <p className="truncate text-sm font-medium">{plan.name}</p>
                    </div>
                    <p className="text-xs text-muted-foreground">{formatPlanPrice(price, intervalLabels)}</p>
                  </div>
                  {actionButton}
                </div>
              );
            })}
          </div>

          <div className="hidden gap-4 pt-4 xl:grid xl:grid-cols-4">
            {billingPlans.map((plan) => {
              const price = resolveDefaultPrice(plan);
              const isCurrent = isCurrentBillingPlan(plan, currentPlan, viewer);
              const actionKind = resolvePlanActionKind(plan, price, isCurrent, currentPlan, protectedPaidPlanRank);
              const actionLabel = resolvePlanActionLabel(actionKind, planActionLabels);
              const disabled = billingLoading || actionKind === "current" || actionKind === "freeBlocked" || actionKind === "unavailable" || checkoutPriceID === price?.id;
              const features = resolvePlanFeatures(plan, planFeatureLabels).slice(0, 6);
              const isSelected = selectedPlan?.id === plan.id;
              const isHighlighted = isCurrent || isSelected;
              const buttonVariant = resolvePlanButtonVariant(actionKind);
              const actionButton = (
                <Button
                  type="button"
                  className="mt-6 w-full shadow-none"
                  variant={buttonVariant}
                  disabled={disabled}
                  onClick={() => void handleSelectPlan(plan, price, isCurrent)}
                >
                  {checkoutPriceID === price?.id ? <SpinnerLabel>{t("actions.processing")}</SpinnerLabel> : actionLabel}
                </Button>
              );
              return (
                <div
                  key={plan.id}
                  className={[
                    "flex min-h-[26rem] flex-col rounded-lg border border-transparent bg-muted/30 p-5 transition-colors",
                    isHighlighted ? "ring-2 ring-foreground" : "hover:bg-muted/45",
                  ].join(" ")}
                >
                  <div className="space-y-3">
                    <div className="flex items-start justify-between gap-3">
                      <h3 className="truncate text-lg font-semibold">{plan.name}</h3>
                    </div>
                    <div className="space-y-1">
                      <p className="text-2xl font-semibold">{formatPlanPrice(price, intervalLabels)}</p>
                    </div>
                  </div>

                  {actionButton}

                  <div className="mt-6 hidden space-y-3 sm:block">
                    {features.map((feature) => (
                      <div key={feature} className="flex items-start gap-3 text-xs text-muted-foreground">
                        <Check className="mt-0.5 size-3.5 shrink-0 text-foreground" />
                        <span className="leading-5">{feature}</span>
                      </div>
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={paymentDialogOpen} onOpenChange={setPaymentDialogOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>{paymentTitle}</DialogTitle>
            <DialogDescription>
              <span className="block">
                {selectedPlan && selectedPrice
                  ? `${selectedPlan.name} · ${formatPlanPrice(selectedPrice, intervalLabels)}`
                  : t("payment.description")}
              </span>
              {paymentImpactDescription ? <span className="mt-1 block">{paymentImpactDescription}</span> : null}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            {paymentProviders.includes("stripe") ? (
              <button
                type="button"
                className={`flex w-full items-center justify-between rounded-lg border px-3 py-3 text-left ${
                  selectedPaymentProvider === "stripe" ? "border-foreground bg-muted/25" : "border-border bg-transparent"
                }`}
                disabled={paymentDisabled}
                onClick={() => setSelectedPaymentProvider("stripe")}
              >
                <span className="space-y-1">
                  <span className="block text-xs font-medium">Stripe</span>
                  <span className="block text-xs text-muted-foreground">{t("payment.card")}</span>
                </span>
                {selectedPaymentProvider === "stripe" ? <Check className="size-4" /> : null}
              </button>
            ) : null}
            {paymentProviders.includes("epay")
              ? epayTypes.map((item) => {
                const selected = selectedPaymentProvider === "epay" && selectedEPayType === item.type;
                return (
                  <button
                    key={item.type}
                    type="button"
                    className={`flex w-full items-center justify-between rounded-lg border px-3 py-3 text-left ${
                      selected ? "border-foreground bg-muted/25" : "border-border bg-transparent"
                    }`}
                    disabled={paymentDisabled}
                    onClick={() => {
                      setSelectedPaymentProvider("epay");
                      setSelectedEPayType(item.type);
                    }}
                  >
                    <span className="space-y-1">
                      <span className="block text-xs font-medium">{item.name || resolveEPayTypeLabel(item.type, epayLabels)}</span>
                      <span className="block text-xs text-muted-foreground">{resolvePaymentProviderLabel("epay", t("payment.disabled"))}</span>
                    </span>
                    {selected ? <Check className="size-4" /> : null}
                  </button>
                );
              })
              : null}
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setPaymentDialogOpen(false)} disabled={checkoutPriceID === selectedPrice?.id}>
              {t("actions.cancel")}
            </Button>
            <Button type="button" disabled={paymentDisabled || !selectedPrice || checkoutPriceID === selectedPrice.id} onClick={() => void handleConfirmPayment()}>
              {checkoutPriceID === selectedPrice?.id ? <SpinnerLabel>{t("actions.processing")}</SpinnerLabel> : t("payment.continue")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={topUpDialogOpen} onOpenChange={setTopUpDialogOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>{t("topUp.title")}</DialogTitle>
            <DialogDescription>{t("topUp.description")}</DialogDescription>
          </DialogHeader>

          <div className="space-y-1.5">
            <div className="flex items-center justify-between gap-3">
              <p className="text-xs text-muted-foreground">{t("topUp.amount")}</p>
              <p className="truncate text-xs text-muted-foreground tabular-nums">
                {t("topUp.currentBalance", { value: formatAccountBalance(billingAccount?.balanceUSD ?? 0) })}
              </p>
            </div>
            <div className="relative">
              <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">$</span>
              <Input
                value={topUpAmount}
                type="number"
                min="0"
                step="0.01"
                className="pl-7"
                onChange={(event) => setTopUpAmount(event.target.value)}
                disabled={billingLoading || topUpLoading || paymentDisabled}
                aria-label={t("topUp.amountAria")}
              />
            </div>
          </div>

          {!paymentDisabled ? (
            <div className="space-y-2">
              <p className="text-xs text-muted-foreground">{t("payment.method")}</p>
              <div className="grid grid-cols-2 gap-2">
                {paymentProviders.includes("stripe") ? (
                  <button
                    type="button"
                    className={`flex h-9 items-center justify-center rounded-md border px-2 text-xs ${
                      selectedPaymentProvider === "stripe" ? "border-foreground bg-muted/25 font-medium" : "border-border bg-transparent text-muted-foreground"
                    }`}
                    disabled={billingLoading || topUpLoading || paymentDisabled}
                    onClick={() => setSelectedPaymentProvider("stripe")}
                  >
                    Stripe
                  </button>
                ) : null}
                {paymentProviders.includes("epay")
                  ? epayTypes.map((item) => {
                    const selected = selectedPaymentProvider === "epay" && selectedEPayType === item.type;
                    return (
                      <button
                        key={item.type}
                        type="button"
                        className={`flex h-9 items-center justify-center rounded-md border px-2 text-xs ${
                          selected ? "border-foreground bg-muted/25 font-medium" : "border-border bg-transparent text-muted-foreground"
                        }`}
                        disabled={billingLoading || topUpLoading || paymentDisabled}
                        onClick={() => {
                          setSelectedPaymentProvider("epay");
                          setSelectedEPayType(item.type);
                        }}
                      >
                        {item.name || resolveEPayTypeLabel(item.type, epayLabels)}
                      </button>
                    );
                  })
                  : null}
              </div>
            </div>
          ) : null}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setTopUpDialogOpen(false)} disabled={topUpLoading}>
              {t("actions.cancel")}
            </Button>
            <Button type="button" disabled={billingLoading || topUpLoading || paymentDisabled} onClick={() => void handleTopUp()}>
              {topUpLoading ? <SpinnerLabel>{t("actions.processing")}</SpinnerLabel> : t("topUp.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={redemptionDialogOpen} onOpenChange={setRedemptionDialogOpen}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>{t("redemption.title")}</DialogTitle>
            <DialogDescription>{t("redemption.description")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">{t("redemption.code")}</p>
            <Input
              value={redemptionCode}
              autoComplete="off"
              className="font-mono"
              disabled={billingLoading || redemptionLoading}
              onChange={(event) => setRedemptionCode(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  void handleRedeemCode();
                }
              }}
              aria-label={t("redemption.code")}
            />
          </div>
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={() => setRedemptionDialogOpen(false)} disabled={redemptionLoading}>
              {t("actions.cancel")}
            </Button>
            <Button type="button" disabled={billingLoading || redemptionLoading} onClick={() => void handleRedeemCode()}>
              {redemptionLoading ? <SpinnerLabel>{t("actions.processing")}</SpinnerLabel> : t("redemption.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SettingsPage>
  );
}
