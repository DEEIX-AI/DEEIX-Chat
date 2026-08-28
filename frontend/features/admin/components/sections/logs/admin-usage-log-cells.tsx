"use client";

import { CornerDownRight } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Separator } from "@/components/ui/separator";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { AdminUsageLogDTO } from "@/features/admin/api/admin.types";
import { ADMIN_DATE_PICKER_TRIGGER_CLASSNAME } from "@/features/admin/components/admin-date-range-filter";
import { formatCount } from "@/features/admin/model/log-display";
import {
  buildUsageBillingTooltipLines,
  formatUsageCost,
  parseUsagePricingSnapshot,
  type UsageBillingLabels,
  type UsageBillingTooltipLine,
  usageBillableOutputTokens,
} from "@/features/admin/model/usage-log-billing";
import { cn } from "@/lib/utils";
import { ModelSelect, type ModelSelectOption } from "@/shared/components/model-select";
import type { BillingDisplayOptions } from "@/shared/lib/billing-display";

const ALL_MODELS_VALUE = "__all__";

export function useUsageBillingLabels(): UsageBillingLabels {
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

export function UsageBillingTooltipLines({ lines, labels }: { lines: UsageBillingTooltipLine[]; labels: UsageBillingLabels }) {
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

export function UsageLogModelCell({ item, labels }: { item: AdminUsageLogDTO; labels: UsageBillingLabels }) {
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

export function UsageLogUsageCell({ item, locale }: { item: AdminUsageLogDTO; locale: string }) {
  const t = useTranslations("adminLogs.usage.tokens");
  const snapshot = parseUsagePricingSnapshot(item.pricingSnapshotJSON);
  const isVideoUsage = snapshot.media_type === "video" || snapshot.duration_billable === true;
  if (isVideoUsage) {
    const inputImageCount = typeof snapshot.input_image_count === "number" && Number.isFinite(snapshot.input_image_count) && snapshot.input_image_count >= 0
      ? Math.trunc(snapshot.input_image_count)
      : null;
    const mediaUsage = [
      { label: t("input"), value: inputImageCount === null ? "—" : t("imageCount", { count: inputImageCount }) },
      { label: t("output"), value: t("secondCount", { count: item.durationSeconds }) },
    ];
    return (
      <div className="grid min-w-[10.5rem] gap-1">
        {mediaUsage.map((entry) => (
          <span
            key={entry.label}
            className="inline-flex h-5 items-center justify-between gap-2 rounded-md bg-muted/45 px-1.5 text-[11px] leading-none text-muted-foreground"
          >
            <span>{entry.label}</span>
            <span className="font-mono tabular-nums">{entry.value}</span>
          </span>
        ))}
      </div>
    );
  }
  const tokens = [
    { label: t("inputShort"), value: item.inputTokens },
    {
      label: t("outputShort"),
      value: usageBillableOutputTokens(item),
      breakdown: {
        visible: item.outputTokens,
        reasoning: item.reasoningTokens,
      },
    },
    { label: t("cacheReadShort"), value: item.cacheReadTokens },
    { label: t("cacheWriteShort"), value: item.cacheWriteTokens },
  ];

  return (
    <div className="grid min-w-[10.5rem] grid-cols-2 gap-1">
      {tokens.map((token) => {
        const badge = (
          <span className={cn(
            "inline-flex h-5 items-center justify-between gap-1 rounded-md bg-muted/45 px-1.5 font-mono text-[11px] leading-none text-muted-foreground",
            token.breakdown && "cursor-help",
          )}>
            <span>{token.label}</span>
            <span className="tabular-nums">{formatCount(token.value, locale)}</span>
          </span>
        );
        if (!token.breakdown) {
          return <React.Fragment key={token.label}>{badge}</React.Fragment>;
        }
        return (
          <Tooltip key={token.label}>
            <TooltipTrigger asChild>{badge}</TooltipTrigger>
            <TooltipContent side="top" className="min-w-40">
              <div className="grid gap-1.5 text-xs">
                <div className="flex items-center justify-between gap-5">
                  <span>{t("output")}</span>
                  <span className="font-mono tabular-nums">{formatCount(token.breakdown.visible, locale)}</span>
                </div>
                <div className="flex items-center justify-between gap-5">
                  <span>{t("reasoning")}</span>
                  <span className="font-mono tabular-nums">{formatCount(token.breakdown.reasoning, locale)}</span>
                </div>
                <Separator className="bg-background/20" />
                <div className="flex items-center justify-between gap-5 font-medium">
                  <span>{t("outputTotal")}</span>
                  <span className="font-mono tabular-nums">{formatCount(token.value, locale)}</span>
                </div>
              </div>
            </TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
}

export function UsageLogCostCell({
  item,
  labels,
  billingDisplay,
}: {
  item: AdminUsageLogDTO;
  labels: UsageBillingLabels;
  billingDisplay: BillingDisplayOptions;
}) {
  const lines = buildUsageBillingTooltipLines(item, labels, billingDisplay);

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className={cn("inline-flex cursor-default items-center font-medium tabular-nums", item.isFreeModel ? "text-muted-foreground" : "text-foreground")}>
          {item.isFreeModel ? labels.freeModelNoBilling : formatUsageCost(item.billedUSD, billingDisplay)}
        </span>
      </TooltipTrigger>
      <TooltipContent side="top">
        <UsageBillingTooltipLines lines={lines} labels={labels} />
      </TooltipContent>
    </Tooltip>
  );
}

export function UsageLogModelFilter({
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
