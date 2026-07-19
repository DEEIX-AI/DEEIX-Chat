"use client";

import * as React from "react";
import { Activity, BadgeDollarSign, Braces, Timer } from "lucide-react";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { useTranslations } from "next-intl";

import { ChartContainer, ChartInteractiveLegend, ChartTooltip } from "@/components/ui/chart";
import type { ChartConfig, ChartInteractiveLegendItem } from "@/components/ui/chart";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
} from "@/features/settings/model/subscription-format";
import type { BillingDisplayOptions } from "@/shared/lib/billing-display";

type DailyUsageChartModel = BillingUsageDailyDTO["models"][number] & {
  color?: string;
  modelLabel?: string;
};

type DailyUsageChartPoint = {
  dayLabel: string;
  fullDayLabel: string;
  billedUsd: number;
  totalTokens: number;
  callCount: number;
  recordCount: number;
  avgLatencyMS: number;
  models: DailyUsageChartModel[];
  [key: string]: string | number | DailyUsageChartModel[];
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

const STACK_COLORS = [
  "#2563eb",
  "#06b6d4",
  "#f97316",
  "#7c3aed",
  "#22c55e",
  "#eab308",
  "#ec4899",
  "#4f46e5",
  "#14b8a6",
  "#ef4444",
];
const CHART_ANIMATION_DURATION_MS = 240;
const MAX_DAILY_MODEL_SERIES = 10;
const OTHER_MODEL_KEY = "__other_models__";
const OTHER_MODEL_COLOR = "#64748b";

function useHiddenUsageSeries() {
  const [hiddenSeries, setHiddenSeries] = React.useState<Set<string>>(() => new Set());
  const toggleSeries = React.useCallback((id: string) => {
    setHiddenSeries((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);
  return { hiddenSeries, toggleSeries };
}

function MetricTile({ label, value, icon }: { label: string; value: string; icon: React.ReactNode }) {
  return (
    <div className="min-w-0 rounded-md bg-muted/35 px-3 py-3.5 md:px-4 md:py-4">
      <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <span className="text-foreground/55">{icon}</span>
        <span>{label}</span>
      </div>
      <p className="mt-2 truncate text-base font-semibold tabular-nums text-foreground md:text-lg">{value}</p>
    </div>
  );
}

function UsageTrendMetricTiles({ stats, billingDisplay }: { stats: UsageTrendStats; billingDisplay: BillingDisplayOptions }) {
  const t = useTranslations("settings.subscriptionPage.usageTrend.metrics");
  return (
    <div className="grid grid-cols-2 gap-2 text-xs md:grid-cols-4">
      <MetricTile
        label={t("totalCost")}
        value={formatUsageSummaryCost(stats.totalBilled, billingDisplay)}
        icon={<BadgeDollarSign className="size-4" />}
      />
      <MetricTile
        label={t("totalTokens")}
        value={formatFormulaTokenCount(stats.totalTokens)}
        icon={<Braces className="size-4" />}
      />
      <MetricTile
        label={t("totalCalls")}
        value={stats.totalCalls.toLocaleString("en-US")}
        icon={<Activity className="size-4" />}
      />
      <MetricTile
        label={t("averageLatency")}
        value={formatUsageTrendLatency(stats.avgLatencyMS)}
        icon={<Timer className="size-4" />}
      />
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

function aggregateDailyModels(models: BillingUsageDailyDTO["models"], modelLabel: string): DailyUsageChartModel {
  const totals = models.reduce(
    (result, model) => ({
      recordCount: result.recordCount + model.recordCount,
      inputTokens: result.inputTokens + model.inputTokens,
      cacheReadTokens: result.cacheReadTokens + model.cacheReadTokens,
      cacheWriteTokens: result.cacheWriteTokens + model.cacheWriteTokens,
      outputTokens: result.outputTokens + model.outputTokens,
      reasoningTokens: result.reasoningTokens + model.reasoningTokens,
      totalTokens: result.totalTokens + model.totalTokens,
      callCount: result.callCount + model.callCount,
      durationSeconds: result.durationSeconds + model.durationSeconds,
      latency: result.latency + model.avgLatencyMS * model.recordCount,
      billedNanousd: result.billedNanousd + model.billedNanousd,
      billedUSD: result.billedUSD + model.billedUSD,
    }),
    {
      recordCount: 0,
      inputTokens: 0,
      cacheReadTokens: 0,
      cacheWriteTokens: 0,
      outputTokens: 0,
      reasoningTokens: 0,
      totalTokens: 0,
      callCount: 0,
      durationSeconds: 0,
      latency: 0,
      billedNanousd: 0,
      billedUSD: 0,
    },
  );
  return {
    platformModelName: OTHER_MODEL_KEY,
    recordCount: totals.recordCount,
    inputTokens: totals.inputTokens,
    cacheReadTokens: totals.cacheReadTokens,
    cacheWriteTokens: totals.cacheWriteTokens,
    outputTokens: totals.outputTokens,
    reasoningTokens: totals.reasoningTokens,
    totalTokens: totals.totalTokens,
    callCount: totals.callCount,
    durationSeconds: totals.durationSeconds,
    avgLatencyMS: totals.recordCount > 0 ? totals.latency / totals.recordCount : 0,
    billedNanousd: totals.billedNanousd,
    billedUSD: totals.billedUSD,
    modelLabel,
  };
}

function DailyUsageChartTooltip({
  active,
  payload,
  billingDisplay,
  hiddenSeries,
}: {
  active?: boolean;
  payload?: Array<{
    payload?: DailyUsageChartPoint;
  }>;
  billingDisplay: BillingDisplayOptions;
  hiddenSeries: ReadonlySet<string>;
}) {
  const t = useTranslations("settings.subscriptionPage.usageTrend.tooltip");
  const item = payload?.[0]?.payload;
  if (!active || !item) {
    return null;
  }
  const visibleModels = item.models.filter((model) => !hiddenSeries.has(model.platformModelName || "-"));
  const visibleMetrics = item.models.length > 0
    ? visibleModels.reduce(
        (totals, model) => ({
          tokens: totals.tokens + model.totalTokens,
          billedUsd: totals.billedUsd + model.billedUSD,
          calls: totals.calls + model.callCount,
          latency: totals.latency + model.avgLatencyMS * model.recordCount,
          records: totals.records + model.recordCount,
        }),
        { tokens: 0, billedUsd: 0, calls: 0, latency: 0, records: 0 },
      )
    : {
        tokens: item.totalTokens,
        billedUsd: item.billedUsd,
        calls: item.callCount,
        latency: item.avgLatencyMS * item.recordCount,
        records: item.recordCount,
      };
  const visibleLatency = visibleMetrics.records > 0 ? visibleMetrics.latency / visibleMetrics.records : 0;

  return (
    <div className="grid min-w-[9rem] gap-1.5 rounded-md border border-border/50 bg-background px-2.5 py-2 text-xs shadow-md">
      <p className="font-medium">{item.fullDayLabel}</p>
      <div className="grid gap-1 text-muted-foreground">
        <div className="flex items-center justify-between gap-6">
          <span>Tokens</span>
          <span className="font-medium text-foreground tabular-nums">{formatTokenCount(visibleMetrics.tokens)}</span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>{t("cost")}</span>
          <span className="font-medium text-foreground tabular-nums">{formatUsageSummaryCost(visibleMetrics.billedUsd, billingDisplay)}</span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>{t("calls")}</span>
          <span className="font-medium text-foreground tabular-nums">{visibleMetrics.calls.toLocaleString("en-US")}</span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>{t("latency")}</span>
          <span className="font-medium text-foreground tabular-nums">{formatUsageTrendLatency(visibleLatency)}</span>
        </div>
        {visibleModels.length > 0 ? (
          <div className="mt-1 grid gap-1 border-t border-border/50 pt-1">
            {visibleModels.slice(0, 10).map((model) => (
              <div key={model.platformModelName || "unknown"} className="flex items-center justify-between gap-6">
                <span className="flex min-w-0 items-center gap-1.5">
                  <span className="size-2 shrink-0 rounded-full" style={{ backgroundColor: model.color || "var(--foreground)" }} />
                  <span className="max-w-[8rem] truncate">{model.modelLabel ?? modelDisplayLabel(model)}</span>
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

function DailyUsageChart({
  items,
  loading,
  billingDisplay,
}: {
  items: BillingUsageDailyDTO[];
  loading: boolean;
  billingDisplay: BillingDisplayOptions;
}) {
  const t = useTranslations("settings.subscriptionPage.usageTrend");
  const { locale } = useAppLocale();
  const { hiddenSeries, toggleSeries } = useHiddenUsageSeries();
  const otherModelLabel = t("otherModels");
  const [todayEndMS] = React.useState(() => {
    const today = new Date();
    today.setHours(23, 59, 59, 999);
    return today.getTime();
  });
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
    const orderedModels = Array.from(totals.entries())
      .sort((left, right) => right[1].totalTokens - left[1].totalTokens || left[0].localeCompare(right[0]));
    const topModels = orderedModels.slice(0, MAX_DAILY_MODEL_SERIES)
      .map(([platformModelName, summary], index) => ({
        key: `model_${index}`,
        platformModelName,
        modelLabel: summary.label,
        color: STACK_COLORS[index],
      }));
    if (orderedModels.length > MAX_DAILY_MODEL_SERIES) {
      topModels.push({
        key: "model_other",
        platformModelName: OTHER_MODEL_KEY,
        modelLabel: otherModelLabel,
        color: OTHER_MODEL_COLOR,
      });
    }
    return topModels;
  }, [items, otherModelLabel]);
  const topModelNames = React.useMemo(
    () => new Set(modelSeries.filter((item) => item.platformModelName !== OTHER_MODEL_KEY).map((item) => item.platformModelName)),
    [modelSeries],
  );
  const modelKeyByName = React.useMemo(() => new Map(modelSeries.map((item) => [item.platformModelName, item.key])), [modelSeries]);
  const modelColorByName = React.useMemo(() => new Map(modelSeries.map((item) => [item.platformModelName, item.color])), [modelSeries]);
  const chartData = React.useMemo<DailyUsageChartPoint[]>(
    () =>
      [...items]
        .filter((item) => new Date(item.usageDate).getTime() <= todayEndMS)
        .sort((left, right) => new Date(left.usageDate).getTime() - new Date(right.usageDate).getTime())
        .map((item) => {
          const models: DailyUsageChartModel[] = (item.models ?? []).filter((model) => topModelNames.has(model.platformModelName || "-"));
          const otherModels = (item.models ?? []).filter((model) => !topModelNames.has(model.platformModelName || "-"));
          if (otherModels.length > 0) models.push(aggregateDailyModels(otherModels, otherModelLabel));
          const point: DailyUsageChartPoint = {
            dayLabel: formatDay(item.usageDate),
            fullDayLabel: formatShortDate(item.usageDate, locale),
            billedUsd: item.billedUSD,
            totalTokens: item.totalTokens,
            callCount: item.callCount,
            recordCount: item.recordCount,
            avgLatencyMS: item.avgLatencyMS,
            models: models.map((model) => ({
              ...model,
              color: modelColorByName.get(model.platformModelName || "-"),
              modelLabel: model.modelLabel,
            })),
          };
          for (const model of models) {
            const key = modelKeyByName.get(model.platformModelName || "-");
            if (key) {
              point[key] = model.totalTokens;
            }
          }
          return point;
        }),
    [items, locale, modelColorByName, modelKeyByName, otherModelLabel, todayEndMS, topModelNames],
  );
  const chartConfig = React.useMemo<ChartConfig>(() => {
    if (modelSeries.length === 0) return usageTokenChartConfig;
    return Object.fromEntries(modelSeries.map((item) => [item.key, { label: item.modelLabel, color: item.color }])) satisfies ChartConfig;
  }, [modelSeries]);
  const rangeLabel = chartData.length > 0 ? `${chartData[0].fullDayLabel} - ${chartData[chartData.length - 1].fullDayLabel}` : "";
  const hasUsageData = chartData.some((item) => item.billedUsd > 0 || item.totalTokens > 0 || item.callCount > 0 || item.recordCount > 0);
  const legendItems = React.useMemo<ChartInteractiveLegendItem[]>(
    () => modelSeries.length > 0
      ? modelSeries.map((item) => ({
          id: item.platformModelName,
          label: item.modelLabel,
          color: item.color,
        }))
      : [{ id: "totalTokens", label: "Tokens", color: "var(--chart-1)" }],
    [modelSeries],
  );
  const topVisibleModelName = React.useMemo(
    () => [...modelSeries].reverse().find((item) => !hiddenSeries.has(item.platformModelName))?.platformModelName,
    [hiddenSeries, modelSeries],
  );

  return (
    <div className="space-y-3 rounded-md bg-muted/35 p-3">
      <div className="flex h-7 items-center justify-between gap-3 px-1">
        <p className="text-xs font-medium text-foreground">{t("dailyUsage")}</p>
        {rangeLabel ? <p className="truncate text-xs text-muted-foreground">{rangeLabel}</p> : null}
      </div>
      {loading ? <UsageChartSkeleton /> : null}
      {!loading && !hasUsageData ? <div className="flex h-[220px] items-center justify-center text-xs text-muted-foreground">{t("empty")}</div> : null}
      {!loading && hasUsageData ? (
        <div className="space-y-3">
          <ChartContainer config={chartConfig} className="h-[260px] w-full aspect-auto">
            <BarChart data={chartData} margin={{ top: 8, right: 8, left: 8, bottom: 0 }}>
              <CartesianGrid vertical={false} strokeDasharray="3 3" />
              <XAxis
                dataKey="dayLabel"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                minTickGap={24}
                interval="equidistantPreserveStart"
              />
              <YAxis
                width={64}
                tickLine={false}
                axisLine={false}
                tickMargin={6}
                tickFormatter={(value: number) => formatUsageAxisTokens(value)}
              />
              <ChartTooltip
                cursor={false}
                content={<DailyUsageChartTooltip billingDisplay={billingDisplay} hiddenSeries={hiddenSeries} />}
              />
              {modelSeries.length > 0 ? (
                modelSeries.map((model) => (
                  <Bar
                    key={model.key}
                    dataKey={model.key}
                    stackId="usage"
                    fill={model.color}
                    maxBarSize={42}
                    hide={hiddenSeries.has(model.platformModelName)}
                    radius={model.platformModelName === topVisibleModelName ? [4, 4, 0, 0] : 0}
                    isAnimationActive
                    animationDuration={CHART_ANIMATION_DURATION_MS}
                    animationEasing="ease-out"
                  />
                ))
              ) : (
                <Bar
                  dataKey="totalTokens"
                  fill="var(--color-totalTokens)"
                  radius={[4, 4, 0, 0]}
                  maxBarSize={42}
                  hide={hiddenSeries.has("totalTokens")}
                  isAnimationActive
                  animationDuration={CHART_ANIMATION_DURATION_MS}
                  animationEasing="ease-out"
                />
              )}
            </BarChart>
          </ChartContainer>
          <ChartInteractiveLegend items={legendItems} hiddenSeries={hiddenSeries} onToggle={toggleSeries} />
        </div>
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
  billingDisplay,
}: {
  active?: boolean;
  payload?: Array<{
    payload?: MonthlyUsageChartPoint;
  }>;
  billingDisplay: BillingDisplayOptions;
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
          <span>Tokens</span>
          <span className="font-medium text-foreground tabular-nums">{formatTokenCount(item.totalTokens)}</span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>{t("cost")}</span>
          <span className="font-medium text-foreground tabular-nums">{formatUsageSummaryCost(item.billedUsd, billingDisplay)}</span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>{t("calls")}</span>
          <span className="font-medium text-foreground tabular-nums">{item.callCount.toLocaleString("en-US")}</span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>{t("latency")}</span>
          <span className="font-medium text-foreground tabular-nums">{formatUsageTrendLatency(item.avgLatencyMS)}</span>
        </div>
      </div>
    </div>
  );
}

function MonthlyUsageChart({
  items,
  loading,
  billingDisplay,
}: {
  items: BillingUsageMonthlyDTO[];
  loading: boolean;
  billingDisplay: BillingDisplayOptions;
}) {
  const t = useTranslations("settings.subscriptionPage.usageTrend");
  const { locale } = useAppLocale();
  const { hiddenSeries, toggleSeries } = useHiddenUsageSeries();
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
  const legendItems = React.useMemo<ChartInteractiveLegendItem[]>(
    () => [{ id: "totalTokens", label: "Tokens", color: "var(--chart-1)" }],
    [],
  );

  return (
    <div className="space-y-3 rounded-md bg-muted/35 p-3">
      <div className="flex h-7 items-center justify-between gap-3 px-1">
        <p className="text-xs font-medium text-foreground">{t("monthlyUsage")}</p>
        {rangeLabel ? <p className="truncate text-xs text-muted-foreground">{rangeLabel}</p> : null}
      </div>
      {loading ? <UsageChartSkeleton /> : null}
      {!loading && !hasUsageData ? <div className="flex h-[220px] items-center justify-center text-xs text-muted-foreground">{t("empty")}</div> : null}
      {!loading && hasUsageData ? (
        <div className="space-y-3">
          <ChartContainer config={usageTokenChartConfig} className="h-[260px] w-full aspect-auto">
            <BarChart data={chartData} margin={{ top: 8, right: 8, left: 8, bottom: 0 }}>
              <CartesianGrid vertical={false} strokeDasharray="3 3" />
              <XAxis
                dataKey="monthLabel"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                minTickGap={24}
                interval="equidistantPreserveStart"
              />
              <YAxis width={64} tickLine={false} axisLine={false} tickMargin={6} tickFormatter={(value: number) => formatUsageAxisTokens(value)} />
              <ChartTooltip cursor={false} content={<MonthlyUsageChartTooltip billingDisplay={billingDisplay} />} />
              <Bar
                dataKey="totalTokens"
                fill="var(--color-totalTokens)"
                radius={[4, 4, 2, 2]}
                maxBarSize={42}
                hide={hiddenSeries.has("totalTokens")}
                isAnimationActive
                animationDuration={CHART_ANIMATION_DURATION_MS}
                animationEasing="ease-out"
              />
            </BarChart>
          </ChartContainer>
          <ChartInteractiveLegend items={legendItems} hiddenSeries={hiddenSeries} onToggle={toggleSeries} />
        </div>
      ) : null}
    </div>
  );
}

export function SubscriptionTrend({
  dailyUsage,
  monthlyUsage,
  loading,
  view,
  billingDisplay,
  onViewChange,
}: {
  dailyUsage: BillingUsageDailyDTO[];
  monthlyUsage: BillingUsageMonthlyDTO[];
  loading: boolean;
  view: UsageTrendView;
  billingDisplay: BillingDisplayOptions;
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
        <Tabs value={view} onValueChange={(value) => onViewChange(value as UsageTrendView)}>
          <TabsList>
            <TabsTrigger value="daily">{t("usageTrend.daily")}</TabsTrigger>
            <TabsTrigger value="monthly">{t("usageTrend.monthly")}</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>
      <UsageTrendMetricTiles stats={trendStats} billingDisplay={billingDisplay} />
      {view === "daily" ? (
        <DailyUsageChart items={dailyUsage} loading={loading} billingDisplay={billingDisplay} />
      ) : (
        <MonthlyUsageChart items={monthlyUsage} loading={loading} billingDisplay={billingDisplay} />
      )}
    </div>
  );
}
