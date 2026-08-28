"use client";

import * as React from "react";
import { Activity, AlertTriangle, CheckCircle2, RefreshCw, ShieldAlert, Timer } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Area, CartesianGrid, ComposedChart, Line, XAxis, YAxis } from "recharts";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  ChartContainer,
  ChartInteractiveLegend,
  ChartTooltip,
  type ChartConfig,
  type ChartInteractiveLegendItem,
} from "@/components/ui/chart";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { type DailyStat, getContentModerationStats } from "@/features/admin/api/content-moderation";
import { cn } from "@/lib/utils";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type TrendMetric = "checks" | "hits" | "failures";

type DayAggregate = {
  date: string;
  label: string;
  fullLabel: string;
  checks: number;
  hits: number;
  failures: number;
  latencySumMS: number;
  latencyCount: number;
};

function MetricCard({
  label,
  value,
  icon,
  loading,
}: {
  label: string;
  value: string;
  icon: React.ReactNode;
  loading: boolean;
}) {
  return (
    <div className="min-w-0 rounded-md bg-muted/35 px-3 py-3.5 md:px-4 md:py-4">
      <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <span className="text-foreground/55">{icon}</span>
        <span>{label}</span>
      </div>
      {loading ? (
        <Skeleton className="mt-3 h-7 w-28" />
      ) : (
        <p className="mt-2 truncate text-base font-semibold tabular-nums text-foreground md:text-lg">{value}</p>
      )}
    </div>
  );
}

function compactNumber(value: number, locale: string): string {
  if (!Number.isFinite(value) || value === 0) return "0";
  return new Intl.NumberFormat(locale, {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}

function formatLatency(value: number, locale: string): string {
  if (!Number.isFinite(value) || value <= 0) return "0";
  if (value < 1000) return `${Math.round(value).toLocaleString(locale)}ms`;
  return `${(value / 1000).toLocaleString(locale, { maximumFractionDigits: 2 })}s`;
}

function formatPercent(value: number, locale: string): string {
  if (!Number.isFinite(value) || value <= 0) return "0%";
  return `${value.toLocaleString(locale, { maximumFractionDigits: 1 })}%`;
}

function parseDateKey(value: string): string {
  return String(value ?? "").slice(0, 10);
}

function aggregateDailyStats(items: DailyStat[], locale: string): DayAggregate[] {
  const byDate = new Map<string, DayAggregate>();
  for (const item of items) {
    // Category rows are breakdown counters. Their hitCount is in addition to the
    // category="" summary row, so including them would double-count hits and can
    // produce a hit rate above 100%.
    if (item.category?.trim()) continue;
    const date = parseDateKey(item.statDate);
    if (!date) continue;
    const current = byDate.get(date) ?? {
      date,
      label: date,
      fullLabel: date,
      checks: 0,
      hits: 0,
      failures: 0,
      latencySumMS: 0,
      latencyCount: 0,
    };
    current.checks += item.checkCount || 0;
    current.hits += item.hitCount || 0;
    current.failures += item.failureCount || 0;
    current.latencySumMS += item.latencySumMS || 0;
    current.latencyCount += item.latencyCount || 0;
    byDate.set(date, current);
  }

  const sorted = Array.from(byDate.values()).sort((a, b) => a.date.localeCompare(b.date));
  return sorted.map((item) => {
    const date = new Date(`${item.date}T00:00:00`);
    const valid = !Number.isNaN(date.getTime());
    return {
      ...item,
      label: valid
        ? new Intl.DateTimeFormat(locale, { month: "2-digit", day: "2-digit" }).format(date)
        : item.date,
      fullLabel: valid
        ? new Intl.DateTimeFormat(locale, { year: "numeric", month: "2-digit", day: "2-digit" }).format(date)
        : item.date,
    };
  });
}

function ModerationTooltipContent({
  active,
  payload,
}: {
  active?: boolean;
  payload?: Array<{ payload?: DayAggregate & { avgLatencyMS?: number } }>;
}) {
  const t = useTranslations("adminStatistics.moderation");
  const locale = useLocale();
  const item = payload?.[0]?.payload;
  if (!active || !item) return null;
  return (
    <div className="grid min-w-[12rem] gap-2 rounded-md border border-border/60 bg-background px-3 py-2 text-xs shadow-md">
      <p className="font-medium">{item.fullLabel}</p>
      <div className="grid gap-1 text-muted-foreground">
        <div className="flex items-center justify-between gap-6">
          <span>{t("metrics.checks")}</span>
          <span className="font-medium tabular-nums text-foreground">
            {new Intl.NumberFormat(locale).format(item.checks)}
          </span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>{t("metrics.hits")}</span>
          <span className="font-medium tabular-nums text-foreground">
            {new Intl.NumberFormat(locale).format(item.hits)}
          </span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>{t("metrics.failures")}</span>
          <span className="font-medium tabular-nums text-foreground">
            {new Intl.NumberFormat(locale).format(item.failures)}
          </span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>{t("metrics.latency")}</span>
          <span className="font-medium tabular-nums text-foreground">
            {formatLatency(item.avgLatencyMS ?? 0, locale)}
          </span>
        </div>
      </div>
    </div>
  );
}

export function AdminModerationStatisticsSection() {
  const t = useTranslations("adminStatistics.moderation");
  const tRoot = useTranslations("adminStatistics");
  const locale = useLocale();
  const [loading, setLoading] = React.useState(true);
  const [items, setItems] = React.useState<DailyStat[]>([]);
  const [trendMetric, setTrendMetric] = React.useState<TrendMetric>("checks");
  const [hiddenSeries, setHiddenSeries] = React.useState<Set<string>>(() => new Set());

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const res = await getContentModerationStats(token);
      setItems(res.items ?? []);
    } catch {
      toast.error(t("loadFailed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  React.useEffect(() => {
    void load();
  }, [load]);

  const days = React.useMemo(() => aggregateDailyStats(items, locale), [items, locale]);
  const totals = React.useMemo(() => {
    return days.reduce(
      (acc, day) => {
        acc.checks += day.checks;
        acc.hits += day.hits;
        acc.failures += day.failures;
        acc.latencySumMS += day.latencySumMS;
        acc.latencyCount += day.latencyCount;
        return acc;
      },
      { checks: 0, hits: 0, failures: 0, latencySumMS: 0, latencyCount: 0 },
    );
  }, [days]);

  const hitRate = totals.checks > 0 ? (totals.hits / totals.checks) * 100 : 0;
  const avgLatencyMS = totals.latencyCount > 0 ? totals.latencySumMS / totals.latencyCount : 0;

  const chartData = React.useMemo(
    () =>
      days.map((day) => ({
        ...day,
        metricValue: day[trendMetric],
        avgLatencyMS: day.latencyCount > 0 ? day.latencySumMS / day.latencyCount : 0,
      })),
    [days, trendMetric],
  );

  const chartConfig = React.useMemo<ChartConfig>(
    () => ({
      metricValue: {
        label: t(`metrics.${trendMetric}`),
        color: "var(--chart-1)",
      },
      avgLatencyMS: {
        label: t("metrics.latency"),
        color: "var(--chart-2)",
      },
    }),
    [t, trendMetric],
  );

  const legendItems = React.useMemo<ChartInteractiveLegendItem[]>(
    () => [
      { id: "metricValue", label: t(`metrics.${trendMetric}`), color: "var(--chart-1)" },
      { id: "avgLatencyMS", label: t("metrics.latency"), color: "var(--chart-2)" },
    ],
    [t, trendMetric],
  );

  const hasData = days.some((day) => day.checks > 0 || day.hits > 0 || day.failures > 0);

  const toggleSeries = React.useCallback((id: string) => {
    setHiddenSeries((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex min-h-10 flex-wrap items-center justify-between gap-2 px-1">
        <h3 className="text-sm font-semibold">{t("title")}</h3>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-8 shrink-0 gap-1.5 px-2 text-xs font-normal text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
          onClick={() => void load()}
          disabled={loading}
          aria-label={tRoot("refresh")}
          title={tRoot("refresh")}
        >
          <span className="flex size-3.5 shrink-0 items-center justify-center">
            <RefreshCw className={cn("size-3.5 stroke-1", loading && "animate-spin")} />
          </span>
          <span>{tRoot("refresh")}</span>
        </Button>
      </div>

      <section className="grid grid-cols-2 gap-2 md:grid-cols-4">
        <MetricCard
          label={t("metrics.checks")}
          value={new Intl.NumberFormat(locale).format(totals.checks)}
          icon={<Activity className="size-4" />}
          loading={loading}
        />
        <MetricCard
          label={t("metrics.hits")}
          value={new Intl.NumberFormat(locale).format(totals.hits)}
          icon={<ShieldAlert className="size-4" />}
          loading={loading}
        />
        <MetricCard
          label={t("metrics.hitRate")}
          value={formatPercent(hitRate, locale)}
          icon={<CheckCircle2 className="size-4" />}
          loading={loading}
        />
        <MetricCard
          label={t("metrics.failures")}
          value={new Intl.NumberFormat(locale).format(totals.failures)}
          icon={<AlertTriangle className="size-4" />}
          loading={loading}
        />
      </section>

      <section className="space-y-6 px-1">
        <div className="flex min-h-10 flex-wrap items-center justify-between gap-3">
          <h3 className="text-sm font-semibold">{t("trendTitle")}</h3>
          <Tabs value={trendMetric} onValueChange={(value) => setTrendMetric(value as TrendMetric)}>
            <TabsList>
              {(["checks", "hits", "failures"] as const).map((metric) => (
                <TabsTrigger key={metric} value={metric} disabled={loading}>
                  {t(`metrics.${metric}`)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>
        <div className="rounded-md bg-muted/30 p-2 md:p-3">
          {loading ? (
            <div className="flex h-[300px] items-end gap-2 px-5 pb-8 pt-10">
              {Array.from({ length: 10 }).map((_, index) => (
                <Skeleton
                  key={`moderation-chart-skeleton-${index}`}
                  className="flex-1 rounded-t-sm"
                  style={{ height: `${25 + ((index * 19) % 68)}%` }}
                />
              ))}
            </div>
          ) : !hasData ? (
            <div className="flex h-[300px] items-center justify-center text-xs text-muted-foreground">
              {t("empty")}
            </div>
          ) : (
            <div className="space-y-3">
              <ChartContainer config={chartConfig} className="aspect-auto h-[320px] w-full">
                <ComposedChart
                  accessibilityLayer
                  data={chartData}
                  margin={{ top: 12, right: 12, left: 4, bottom: 0 }}
                  onMouseDown={(_, event) => event.preventDefault()}
                >
                  <defs>
                    <linearGradient id="fillModerationTrendMetric" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor="var(--color-metricValue)" stopOpacity={0.18} />
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
                    tickFormatter={(value: number) => compactNumber(value, locale)}
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
                    content={<ModerationTooltipContent />}
                  />
                  <Area
                    yAxisId="metric"
                    dataKey="metricValue"
                    type="monotone"
                    fill="url(#fillModerationTrendMetric)"
                    fillOpacity={1}
                    stroke="var(--color-metricValue)"
                    strokeWidth={1.5}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    dot={false}
                    activeDot={{ r: 2.5, strokeWidth: 1.5 }}
                    isAnimationActive
                    animationDuration={240}
                    animationEasing="ease-out"
                    hide={hiddenSeries.has("metricValue")}
                  />
                  <Line
                    yAxisId="latency"
                    dataKey="avgLatencyMS"
                    type="monotone"
                    stroke="var(--color-avgLatencyMS)"
                    strokeWidth={1}
                    strokeDasharray="3 3"
                    strokeLinecap="round"
                    dot={false}
                    activeDot={{ r: 2, strokeWidth: 1 }}
                    isAnimationActive
                    animationDuration={240}
                    animationEasing="ease-out"
                    hide={hiddenSeries.has("avgLatencyMS")}
                  />
                </ComposedChart>
              </ChartContainer>
              <ChartInteractiveLegend items={legendItems} hiddenSeries={hiddenSeries} onToggle={toggleSeries} />
            </div>
          )}
        </div>
        {!loading && hasData ? (
          <div className="flex items-center gap-1.5 px-1 text-xs text-muted-foreground">
            <Timer className="size-3.5" />
            <span>
              {t("avgLatencyLabel")}: {formatLatency(avgLatencyMS, locale)}
            </span>
          </div>
        ) : null}
      </section>
    </div>
  );
}
