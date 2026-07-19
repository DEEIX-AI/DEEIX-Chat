"use client";

import * as React from "react";
import { useLocale, useTranslations } from "next-intl";
import { Area, Bar, BarChart, CartesianGrid, ComposedChart, Line, XAxis, YAxis } from "recharts";

import {
  ChartContainer,
  ChartInteractiveLegend,
  ChartTooltip,
  type ChartConfig,
  type ChartInteractiveLegendItem,
} from "@/components/ui/chart";
import { Skeleton } from "@/components/ui/skeleton";
import type {
  AdminUsageStatisticsMetricsDTO,
  AdminUsageStatisticsModelRankDTO,
  AdminUsageStatisticsRankBy,
  AdminUsageStatisticsTrendDTO,
  AdminUsageStatisticsUserRankDTO,
} from "@/features/admin/api";
import {
  formatBillingDisplayAmountFromUSD,
  type BillingDisplayOptions,
} from "@/shared/lib/billing-display";

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

function displayCost(value: number, billingDisplay: BillingDisplayOptions): string {
  return formatBillingDisplayAmountFromUSD(value, billingDisplay, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}

function chartMetricValue(
  metrics: AdminUsageStatisticsMetricsDTO,
  rankBy: AdminUsageStatisticsRankBy,
  billingDisplay: BillingDisplayOptions,
): number {
  const value = rawMetricValue(metrics, rankBy);
  if (rankBy !== "cost") return value;
  if (billingDisplay.currency === "CNY" && Number(billingDisplay.usdToCnyRate) > 0) {
    return value * Number(billingDisplay.usdToCnyRate);
  }
  return value;
}

function rawMetricValue(
  metrics: AdminUsageStatisticsMetricsDTO,
  rankBy: AdminUsageStatisticsRankBy,
): number {
  if (rankBy === "tokens") return metrics.totalTokens;
  if (rankBy === "calls") return metrics.callCount;
  return metrics.billedUSD;
}

function compactNumber(value: number, locale: string): string {
  if (!Number.isFinite(value) || value === 0) return "0";
  return new Intl.NumberFormat(locale, {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}

function chartAxisValue(
  value: number,
  rankBy: AdminUsageStatisticsRankBy,
  billingDisplay: BillingDisplayOptions,
  locale: string,
): string {
  if (rankBy !== "cost") return compactNumber(value, locale);
  const symbol = billingDisplay.currency === "CNY" && Number(billingDisplay.usdToCnyRate) > 0 ? "¥" : "$";
  return `${symbol}${value.toLocaleString("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`;
}

function formatLatency(value: number, locale: string): string {
  if (!Number.isFinite(value) || value <= 0) return "0";
  if (value < 1000) return `${Math.round(value).toLocaleString(locale)}ms`;
  return `${(value / 1000).toLocaleString(locale, { maximumFractionDigits: 2 })}s`;
}

function parsePeriodDate(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  if (!match) return null;
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isNaN(date.getTime()) ? null : date;
}

function periodLabels(
  value: string,
  granularity: string,
  locale: string,
  rangeStartDate?: string,
  rangeEndDate?: string,
): { short: string; full: string } {
  const date = parsePeriodDate(value);
  if (!date) return { short: "-", full: "-" };
  if (granularity === "week") {
    const selectedStart = rangeStartDate ? parsePeriodDate(rangeStartDate) : null;
    const selectedEnd = rangeEndDate ? parsePeriodDate(rangeEndDate) : null;
    const weekStart = selectedStart && selectedStart > date ? selectedStart : date;
    const weekEnd = new Date(date);
    weekEnd.setDate(weekEnd.getDate() + 6);
    const visibleWeekEnd = selectedEnd && selectedEnd < weekEnd ? selectedEnd : weekEnd;
    const shortFormatter = new Intl.DateTimeFormat(locale, { month: "2-digit", day: "2-digit" });
    const fullFormatter = new Intl.DateTimeFormat(locale, { year: "numeric", month: "2-digit", day: "2-digit" });
    return {
      short: shortFormatter.format(weekStart),
      full: `${fullFormatter.format(weekStart)} - ${fullFormatter.format(visibleWeekEnd)}`,
    };
  }
  if (granularity === "month") {
    return {
      short: new Intl.DateTimeFormat(locale, { month: "short" }).format(date),
      full: new Intl.DateTimeFormat(locale, { year: "numeric", month: "long" }).format(date),
    };
  }
  return {
    short: new Intl.DateTimeFormat(locale, { month: "2-digit", day: "2-digit" }).format(date),
    full: new Intl.DateTimeFormat(locale, { year: "numeric", month: "2-digit", day: "2-digit" }).format(date),
  };
}

function StatisticsTooltipContent({
  active,
  payload,
  label,
  billingDisplay,
}: {
  active?: boolean;
  payload?: Array<{ payload?: { metrics?: AdminUsageStatisticsMetricsDTO; fullLabel?: string } }>;
  label?: string;
  billingDisplay: BillingDisplayOptions;
}) {
  const t = useTranslations("adminStatistics");
  const locale = useLocale();
  const item = payload?.[0]?.payload;
  const metrics = item?.metrics;
  if (!active || !metrics) return null;
  return (
    <div className="grid min-w-[12rem] gap-2 rounded-md border border-border/60 bg-background px-3 py-2 text-xs shadow-md">
      <p className="font-medium">{item.fullLabel || label}</p>
      <div className="grid gap-1 text-muted-foreground">
        <TooltipRow label={t("metrics.tokens")} value={new Intl.NumberFormat(locale).format(metrics.totalTokens)} />
        <TooltipRow label={t("metrics.cost")} value={displayCost(metrics.billedUSD, billingDisplay)} />
        <TooltipRow label={t("metrics.calls")} value={new Intl.NumberFormat(locale).format(metrics.callCount)} />
        <TooltipRow label={t("metrics.latency")} value={formatLatency(metrics.avgLatencyMS, locale)} />
      </div>
    </div>
  );
}

function TooltipRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-6">
      <span>{label}</span>
      <span className="font-medium tabular-nums text-foreground">{value}</span>
    </div>
  );
}

function ChartLoadingSkeleton({ horizontal = false }: { horizontal?: boolean }) {
  return (
    <div className={horizontal ? "grid h-[300px] content-center gap-3 px-5" : "flex h-[300px] items-end gap-2 px-5 pb-8 pt-10"}>
      {Array.from({ length: 10 }).map((_, index) => (
        <Skeleton
          key={`statistics-chart-skeleton-${index}`}
          className={horizontal ? "h-4 rounded-sm" : "flex-1 rounded-t-sm"}
          style={horizontal ? { width: `${38 + ((index * 13) % 58)}%` } : { height: `${25 + ((index * 19) % 68)}%` }}
        />
      ))}
    </div>
  );
}

function useHiddenChartSeries() {
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

export const StatisticsTrendChart = React.memo(function StatisticsTrendChart({
  items,
  granularity,
  rangeStartDate,
  rangeEndDate,
  rankBy,
  billingDisplay,
  loading,
}: {
  items: AdminUsageStatisticsTrendDTO[];
  granularity: string;
  rangeStartDate?: string;
  rangeEndDate?: string;
  rankBy: AdminUsageStatisticsRankBy;
  billingDisplay: BillingDisplayOptions;
  loading: boolean;
}) {
  const t = useTranslations("adminStatistics");
  const locale = useLocale();
  const { hiddenSeries, toggleSeries } = useHiddenChartSeries();
  const chartConfig = React.useMemo<ChartConfig>(() => ({
    metricValue: {
      label: t(`rankBy.${rankBy}`),
      color: "var(--chart-1)",
    },
    avgLatencyMS: {
      label: t("metrics.latency"),
      color: "var(--chart-2)",
    },
  }), [rankBy, t]);
  const data = React.useMemo(
    () =>
      items.map((item) => {
        const labels = periodLabels(item.periodStart, granularity, locale, rangeStartDate, rangeEndDate);
        return {
          label: labels.short,
          fullLabel: labels.full,
          metricValue: chartMetricValue(item, rankBy, billingDisplay),
          avgLatencyMS: item.avgLatencyMS,
          metrics: item,
        };
      }),
    [billingDisplay, granularity, items, locale, rangeEndDate, rangeStartDate, rankBy],
  );
  const hasData = items.some((item) => item.recordCount > 0 || item.totalTokens > 0 || item.callCount > 0 || item.billedUSD > 0);
  const legendItems = React.useMemo<ChartInteractiveLegendItem[]>(() => [
    { id: "metricValue", label: t(`rankBy.${rankBy}`), color: "var(--chart-1)" },
    { id: "avgLatencyMS", label: t("metrics.latency"), color: "var(--chart-2)" },
  ], [rankBy, t]);

  if (loading) return <ChartLoadingSkeleton />;
  if (!loading && !hasData) {
    return <div className="flex h-[300px] items-center justify-center text-xs text-muted-foreground">{t("empty")}</div>;
  }
  return (
    <div className="space-y-3">
      <ChartContainer config={chartConfig} className="h-[320px] w-full aspect-auto">
        <ComposedChart
          accessibilityLayer
          data={data}
          margin={{ top: 12, right: 12, left: 4, bottom: 0 }}
          onMouseDown={(_, event) => event.preventDefault()}
        >
        <defs>
          <linearGradient id="fillUsageTrendMetric" x1="0" y1="0" x2="0" y2="1">
            <stop offset="5%" stopColor="var(--color-metricValue)" stopOpacity={0.3} />
            <stop offset="95%" stopColor="var(--color-metricValue)" stopOpacity={0.02} />
          </linearGradient>
        </defs>
        <CartesianGrid vertical={false} strokeDasharray="3 3" />
        <XAxis
          dataKey="label"
          axisLine={false}
          tickLine={false}
          tickMargin={8}
          minTickGap={24}
          interval="equidistantPreserveStart"
        />
        <YAxis
          yAxisId="metric"
          width={64}
          axisLine={false}
          tickLine={false}
          tickMargin={6}
          tickFormatter={(value: number) => chartAxisValue(value, rankBy, billingDisplay, locale)}
        />
        <YAxis
          yAxisId="latency"
          orientation="right"
          width={52}
          axisLine={false}
          tickLine={false}
          tickMargin={6}
          tickFormatter={(value: number) => formatLatency(value, locale)}
        />
        <ChartTooltip
          cursor={{ stroke: "var(--border)", strokeDasharray: "3 3" }}
          content={<StatisticsTooltipContent billingDisplay={billingDisplay} />}
        />
        <Area
          yAxisId="metric"
          dataKey="metricValue"
          type="monotone"
          fill="url(#fillUsageTrendMetric)"
          fillOpacity={1}
          stroke="var(--color-metricValue)"
          strokeWidth={2}
          dot={false}
          activeDot={{ r: 3, strokeWidth: 2 }}
          isAnimationActive
          animationDuration={CHART_ANIMATION_DURATION_MS}
          animationEasing="ease-out"
          hide={hiddenSeries.has("metricValue")}
        />
        <Line
          yAxisId="latency"
          dataKey="avgLatencyMS"
          type="monotone"
          stroke="var(--color-avgLatencyMS)"
          strokeWidth={1.5}
          strokeDasharray="4 4"
          dot={false}
          activeDot={{ r: 2.5, strokeWidth: 1.5 }}
          isAnimationActive
          animationDuration={CHART_ANIMATION_DURATION_MS}
          animationEasing="ease-out"
          hide={hiddenSeries.has("avgLatencyMS")}
        />
        </ComposedChart>
      </ChartContainer>
      <ChartInteractiveLegend items={legendItems} hiddenSeries={hiddenSeries} onToggle={toggleSeries} />
    </div>
  );
});

type StatisticsStackedSeries = {
  id: string;
  key: string;
  label: string;
  fullLabel: string;
  color: string;
  trend: AdminUsageStatisticsTrendDTO[];
};

type StatisticsStackedSegment = {
  id: string;
  key: string;
  label: string;
  fullLabel: string;
  color: string;
  rawValue: number;
  chartValue: number;
};

type StatisticsStackedChartPoint = Record<string, unknown> & {
  label: string;
  fullLabel: string;
  totalRawValue: number;
  segments: StatisticsStackedSegment[];
};

function statisticsPeriodKey(value: string): string {
  return value.slice(0, 10);
}

function formatStackedMetricValue(
  value: number,
  rankBy: AdminUsageStatisticsRankBy,
  billingDisplay: BillingDisplayOptions,
  locale: string,
): string {
  if (rankBy === "cost") return displayCost(value, billingDisplay);
  return new Intl.NumberFormat(locale).format(value);
}

function StatisticsStackedTooltipContent({
  active,
  payload,
  rankBy,
  billingDisplay,
  hiddenSeries,
}: {
  active?: boolean;
  payload?: Array<{ payload?: StatisticsStackedChartPoint }>;
  rankBy: AdminUsageStatisticsRankBy;
  billingDisplay: BillingDisplayOptions;
  hiddenSeries: ReadonlySet<string>;
}) {
  const t = useTranslations("adminStatistics");
  const locale = useLocale();
  const item = payload?.[0]?.payload;
  if (!active || !item) return null;
  const segments = item.segments.filter((segment) => segment.rawValue > 0 && !hiddenSeries.has(segment.id));
  if (segments.length === 0) return null;
  return (
    <div className="grid min-w-[15rem] max-w-[22rem] gap-2 rounded-md border border-border/60 bg-background px-3 py-2.5 text-xs shadow-md">
      <p className="font-medium">{item.fullLabel}</p>
      <div className="grid gap-1.5 text-muted-foreground">
        {segments.map((segment) => (
          <div key={segment.key} className="flex items-center justify-between gap-6">
            <span className="flex min-w-0 items-center gap-1.5" title={segment.fullLabel}>
              <span className="size-2 shrink-0 rounded-[2px]" style={{ backgroundColor: segment.color }} />
              <span className="truncate">{segment.label}</span>
            </span>
            <span className="shrink-0 font-medium tabular-nums text-foreground">
              {formatStackedMetricValue(segment.rawValue, rankBy, billingDisplay, locale)}
            </span>
          </div>
        ))}
      </div>
      <div className="border-t border-border/50 pt-2">
        <TooltipRow
          label={t("rankings.total")}
          value={formatStackedMetricValue(
            segments.reduce((total, segment) => total + segment.rawValue, 0),
            rankBy,
            billingDisplay,
            locale,
          )}
        />
      </div>
    </div>
  );
}

function StatisticsStackedTrendChart({
  periods,
  granularity,
  rangeStartDate,
  rangeEndDate,
  series,
  rankBy,
  billingDisplay,
  loading,
}: {
  periods: AdminUsageStatisticsTrendDTO[];
  granularity: string;
  rangeStartDate?: string;
  rangeEndDate?: string;
  series: StatisticsStackedSeries[];
  rankBy: AdminUsageStatisticsRankBy;
  billingDisplay: BillingDisplayOptions;
  loading: boolean;
}) {
  const t = useTranslations("adminStatistics");
  const locale = useLocale();
  const { hiddenSeries, toggleSeries } = useHiddenChartSeries();
  const config = React.useMemo<ChartConfig>(
    () => Object.fromEntries(series.map((item) => [item.key, { label: item.label, color: item.color }])),
    [series],
  );
  const data = React.useMemo<StatisticsStackedChartPoint[]>(
    () => {
      const metricsBySeries = series.map((item) =>
        new Map(item.trend.map((point) => [statisticsPeriodKey(point.periodStart), point])),
      );
      const points = periods.map((period) => {
        const labels = periodLabels(period.periodStart, granularity, locale, rangeStartDate, rangeEndDate);
        const segments = series.map((item, index) => {
          const metrics = metricsBySeries[index]?.get(statisticsPeriodKey(period.periodStart));
          return {
            id: item.id,
            key: item.key,
            label: item.label,
            fullLabel: item.fullLabel,
            color: item.color,
            rawValue: metrics ? rawMetricValue(metrics, rankBy) : 0,
            chartValue: metrics ? chartMetricValue(metrics, rankBy, billingDisplay) : 0,
          };
        });
        const point: StatisticsStackedChartPoint = {
          label: labels.short,
          fullLabel: labels.full,
          totalRawValue: segments.reduce((total, segment) => total + segment.rawValue, 0),
          segments,
        };
        for (const segment of segments) point[segment.key] = segment.chartValue;
        return point;
      });
      return points;
    },
    [billingDisplay, granularity, locale, periods, rangeEndDate, rangeStartDate, rankBy, series],
  );
  const hasData = data.some((point) => point.totalRawValue > 0);
  const legendItems = React.useMemo<ChartInteractiveLegendItem[]>(
    () => series.map((item) => ({ id: item.id, label: item.label, title: item.fullLabel, color: item.color })),
    [series],
  );
  const topVisibleSeriesID = React.useMemo(
    () => [...series].reverse().find((item) => !hiddenSeries.has(item.id))?.id,
    [hiddenSeries, series],
  );
  if (loading) return <ChartLoadingSkeleton />;
  if (!loading && !hasData) {
    return <div className="flex h-[300px] items-center justify-center text-xs text-muted-foreground">{t("empty")}</div>;
  }
  return (
    <div className="space-y-3">
      <ChartContainer config={config} className="h-[320px] w-full aspect-auto">
        <BarChart
          accessibilityLayer
          data={data}
          margin={{ top: 12, right: 12, left: 4, bottom: 0 }}
          onMouseDown={(_, event) => event.preventDefault()}
        >
          <CartesianGrid vertical={false} strokeDasharray="3 3" />
          <XAxis
            dataKey="label"
            axisLine={false}
            tickLine={false}
            tickMargin={8}
            minTickGap={24}
            interval="equidistantPreserveStart"
          />
          <YAxis
            width={64}
            axisLine={false}
            tickLine={false}
            tickMargin={6}
            tickFormatter={(value: number) => chartAxisValue(value, rankBy, billingDisplay, locale)}
          />
          <ChartTooltip
            cursor={{ fill: "var(--muted)", opacity: 0.45 }}
            content={
              <StatisticsStackedTooltipContent
                rankBy={rankBy}
                billingDisplay={billingDisplay}
                hiddenSeries={hiddenSeries}
              />
            }
          />
          {series.map((item) => (
            <Bar
              key={item.key}
              dataKey={item.key}
              stackId="usage"
              fill={item.color}
              maxBarSize={44}
              isAnimationActive
              animationDuration={CHART_ANIMATION_DURATION_MS}
              animationEasing="ease-out"
              hide={hiddenSeries.has(item.id)}
              radius={item.id === topVisibleSeriesID ? [4, 4, 0, 0] : 0}
            />
          ))}
        </BarChart>
      </ChartContainer>
      <ChartInteractiveLegend items={legendItems} hiddenSeries={hiddenSeries} onToggle={toggleSeries} />
    </div>
  );
}

export const StatisticsModelRankingChart = React.memo(function StatisticsModelRankingChart({
  items,
  periods,
  granularity,
  rangeStartDate,
  rangeEndDate,
  rankBy,
  billingDisplay,
  loading,
}: {
  items: AdminUsageStatisticsModelRankDTO[];
  periods: AdminUsageStatisticsTrendDTO[];
  granularity: string;
  rangeStartDate?: string;
  rangeEndDate?: string;
  rankBy: AdminUsageStatisticsRankBy;
  billingDisplay: BillingDisplayOptions;
  loading: boolean;
}) {
  const t = useTranslations("adminStatistics");
  const series = React.useMemo<StatisticsStackedSeries[]>(
    () =>
      items.map((item, index) => ({
        id: item.platformModelName.trim() || `unknown-model-${index}`,
        key: `model_${index}`,
        label: item.platformModelName.trim() || t("unknownModel"),
        fullLabel: item.platformModelName.trim() || t("unknownModel"),
        color: STACK_COLORS[index % STACK_COLORS.length],
        trend: item.trend ?? [],
      })),
    [items, t],
  );
  return (
    <StatisticsStackedTrendChart
      periods={periods}
      granularity={granularity}
      rangeStartDate={rangeStartDate}
      rangeEndDate={rangeEndDate}
      series={series}
      rankBy={rankBy}
      billingDisplay={billingDisplay}
      loading={loading}
    />
  );
});

export const StatisticsUserRankingChart = React.memo(function StatisticsUserRankingChart({
  items,
  periods,
  granularity,
  rangeStartDate,
  rangeEndDate,
  rankBy,
  billingDisplay,
  loading,
}: {
  items: AdminUsageStatisticsUserRankDTO[];
  periods: AdminUsageStatisticsTrendDTO[];
  granularity: string;
  rangeStartDate?: string;
  rangeEndDate?: string;
  rankBy: AdminUsageStatisticsRankBy;
  billingDisplay: BillingDisplayOptions;
  loading: boolean;
}) {
  const t = useTranslations("adminStatistics");
  const series = React.useMemo<StatisticsStackedSeries[]>(
    () =>
      items.map((item, index) => {
        const label = item.userLabel.trim() || item.userDisplayName.trim() || item.username.trim() || t("unknownUser", { id: item.userID });
        return {
          id: String(item.userID),
          key: `user_${index}`,
          label,
          fullLabel: `${label} (#${item.userID})`,
          color: STACK_COLORS[index % STACK_COLORS.length],
          trend: item.trend ?? [],
        };
      }),
    [items, t],
  );
  return (
    <StatisticsStackedTrendChart
      periods={periods}
      granularity={granularity}
      rangeStartDate={rangeStartDate}
      rangeEndDate={rangeEndDate}
      series={series}
      rankBy={rankBy}
      billingDisplay={billingDisplay}
      loading={loading}
    />
  );
});

export function formatStatisticsCost(value: number, billingDisplay: BillingDisplayOptions): string {
  return displayCost(value, billingDisplay);
}

export function formatStatisticsCount(value: number, locale: string): string {
  return new Intl.NumberFormat(locale).format(value);
}

export function formatStatisticsLatency(value: number, locale: string): string {
  return formatLatency(value, locale);
}
