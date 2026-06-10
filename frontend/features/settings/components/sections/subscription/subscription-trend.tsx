"use client";

import * as React from "react";
import { Bar, BarChart, CartesianGrid, Cell, XAxis, YAxis } from "recharts";
import { useTranslations } from "next-intl";

import { ChartContainer, ChartTooltip } from "@/components/ui/chart";
import type { ChartConfig } from "@/components/ui/chart";
import { Skeleton } from "@/components/ui/skeleton";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import type { BillingUsageDailyDTO, BillingUsageMonthlyDTO } from "@/shared/api/billing.types";
import {
  formatDay,
  formatFormulaTokenCount,
  formatFullMonthLabel,
  formatMonthLabel,
  formatShortDate,
  formatTokenCount,
  formatUsageAxisTokens,
  formatUsageSummaryCost,
  formatUsageTrendLatency,
  modelDisplayLabel,
} from "./subscription-format";

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

export type UsageTrendView = "daily" | "monthly";

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
    [items, locale, modelColorByName, modelKeyByName],
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
    [items, locale],
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

export function SubscriptionTrend({
  dailyUsage,
  monthlyUsage,
  loading,
  view,
  onViewChange,
}: {
  dailyUsage: BillingUsageDailyDTO[];
  monthlyUsage: BillingUsageMonthlyDTO[];
  loading: boolean;
  view: UsageTrendView;
  onViewChange: (view: UsageTrendView) => void;
}) {
  const t = useTranslations("settings.subscriptionPage");
  const trendStats = React.useMemo(
    () => (view === "daily" ? calculateDailyTrendStats(dailyUsage) : calculateMonthlyTrendStats(monthlyUsage)),
    [dailyUsage, monthlyUsage, view],
  );

  return (
    <div className="space-y-4 md:space-y-5">
      <div className="flex h-9 items-center justify-between gap-3">
        <h3 className="text-sm font-semibold">{view === "daily" ? t("usageTrend.dailyTitle") : t("usageTrend.monthlyTitle")}</h3>
        <div className="inline-flex items-center gap-1 rounded-full bg-muted/40 p-1">
          <button
            type="button"
            className={`rounded-full px-2.5 py-1 text-[11px] font-medium transition-colors ${view === "daily" ? "bg-background text-foreground shadow-xs" : "text-foreground/60 hover:text-foreground"}`}
            onClick={() => onViewChange("daily")}
          >
            {t("usageTrend.daily")}
          </button>
          <button
            type="button"
            className={`rounded-full px-2.5 py-1 text-[11px] font-medium transition-colors ${view === "monthly" ? "bg-background text-foreground shadow-xs" : "text-foreground/60 hover:text-foreground"}`}
            onClick={() => onViewChange("monthly")}
          >
            {t("usageTrend.monthly")}
          </button>
        </div>
      </div>
      <UsageTrendMetricTiles stats={trendStats} />
      {view === "daily" ? <DailyUsageChart items={dailyUsage} loading={loading} /> : <MonthlyUsageChart items={monthlyUsage} loading={loading} />}
    </div>
  );
}
