"use client";

import * as React from "react";
import { Activity, BadgeDollarSign, Braces, CalendarDays, Check, Funnel, RefreshCw, Timer } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { AdminDateRangeFilter } from "@/features/admin/components/admin-date-range-filter";
import type {
  AdminUsageStatisticsMetricsDTO,
  AdminUsageStatisticsRankBy,
} from "@/features/admin/api";
import { useAdminStatistics } from "@/features/admin/hooks/use-admin-statistics";
import { cn } from "@/lib/utils";
import { AdminStatisticsBillingFilter } from "./admin-statistics-billing-filter";
import { AdminStatisticsModelFilter } from "./admin-statistics-model-filter";
import { AdminStatisticsSubjectFilter } from "./admin-statistics-subject-filter";
import {
  formatStatisticsCost,
  formatStatisticsCount,
  formatStatisticsLatency,
  StatisticsModelRankingChart,
  StatisticsTrendChart,
  StatisticsUserRankingChart,
} from "./admin-statistics-charts";

const ALL_MODELS_VALUE = "__all_models__";

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

function emptyMetrics(): AdminUsageStatisticsMetricsDTO {
  return {
    recordCount: 0,
    inputTokens: 0,
    cacheReadTokens: 0,
    cacheWriteTokens: 0,
    outputTokens: 0,
    reasoningTokens: 0,
    totalTokens: 0,
    callCount: 0,
    avgLatencyMS: 0,
    billedNanousd: 0,
    billedUSD: 0,
  };
}

export function AdminStatisticsPage() {
  const t = useTranslations("adminStatistics");
  const tTable = useTranslations("common.table");
  const locale = useLocale();
  const statistics = useAdminStatistics();
  const [dateFilterOpen, setDateFilterOpen] = React.useState(false);
  const [trendMetric, setTrendMetric] = React.useState<AdminUsageStatisticsRankBy>("tokens");
  const totals = statistics.statistics?.totals ?? emptyMetrics();
  const trendItems = statistics.statistics?.trend ?? [];
  const granularity = statistics.statistics?.range.granularity ?? "day";
  const chartRangeStartDate = statistics.statistics?.range.startDate ?? statistics.startDate;
  const chartRangeEndDate = statistics.statistics?.range.endDate ?? statistics.endDate;
  const initialLoading = statistics.loading && !statistics.statistics;
  const modelOptions = React.useMemo(
    () => [{ label: t("filters.allModels"), value: ALL_MODELS_VALUE }, ...statistics.modelOptions],
    [statistics.modelOptions, t],
  );
  const rangeErrorText = statistics.rangeError ? t(`filters.rangeErrors.${statistics.rangeError}`) : "";
  const rangeLabel = statistics.rangePreset === "custom"
    ? statistics.startDate && statistics.endDate
      ? `${statistics.startDate} - ${statistics.endDate}`
      : t("filters.custom")
    : t(`filters.last${statistics.rangePreset}Days`);
  const activeFilterCount = Number(statistics.subject.type !== "all") + Number(Boolean(statistics.platformModelName)) + Number(statistics.billingScope !== "all");

  return (
    <div className="pb-10">
      <div className="space-y-3">
        <div className="flex min-h-10 flex-wrap items-center justify-between gap-2 px-1">
          <h3 className="text-sm font-semibold">{t("title")}</h3>
          <div className="flex flex-wrap items-center justify-end gap-1.5">
            <Popover open={dateFilterOpen} onOpenChange={setDateFilterOpen}>
              <PopoverTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-8 max-w-[220px] gap-1.5 px-2 text-xs font-normal text-muted-foreground shadow-none hover:bg-muted hover:text-foreground data-[state=open]:bg-muted data-[state=open]:text-foreground"
                  disabled={initialLoading}
                >
                  <span className="flex size-3.5 shrink-0 items-center justify-center">
                    <CalendarDays className="size-3.5 stroke-1" />
                  </span>
                  <span className="truncate">{rangeLabel}</span>
                </Button>
              </PopoverTrigger>
              <PopoverContent align="end" className="w-[280px] p-2">
                <div className="space-y-0.5">
                  {(["7", "30", "90"] as const).map((preset) => (
                    <Button
                      key={preset}
                      type="button"
                      variant="ghost"
                      size="sm"
                      className={cn(
                        "h-8 w-full justify-start px-2 text-xs font-normal shadow-none",
                        statistics.rangePreset === preset && "bg-accent/50",
                      )}
                      onClick={() => {
                        statistics.setRangePreset(preset);
                        setDateFilterOpen(false);
                      }}
                    >
                      {t(`filters.last${preset}Days`)}
                      {statistics.rangePreset === preset ? <Check className="ml-auto size-3.5" /> : null}
                    </Button>
                  ))}
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className={cn(
                      "h-8 w-full justify-start px-2 text-xs font-normal shadow-none",
                      statistics.rangePreset === "custom" && "bg-accent/50",
                    )}
                    onClick={() => statistics.setRangePreset("custom")}
                  >
                    {t("filters.custom")}
                    {statistics.rangePreset === "custom" ? <Check className="ml-auto size-3.5" /> : null}
                  </Button>
                </div>
                {statistics.rangePreset === "custom" ? (
                  <div className="mt-2 border-t border-border/60 pt-2">
                    <AdminDateRangeFilter
                      fromValue={statistics.startDate}
                      toValue={statistics.endDate}
                      onFromChange={statistics.setStartDate}
                      onToChange={(value) => {
                        statistics.setEndDate(value);
                        if (value) setDateFilterOpen(false);
                      }}
                      maxRangeDays={366}
                      disabled={initialLoading}
                      triggerClassName="h-8 bg-background dark:bg-input/30"
                    />
                  </div>
                ) : null}
              </PopoverContent>
            </Popover>
            <Popover>
              <PopoverTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className={cn(
                    "h-8 gap-1.5 px-2 text-xs font-normal text-muted-foreground shadow-none hover:bg-muted hover:text-foreground data-[state=open]:bg-muted data-[state=open]:text-foreground",
                    activeFilterCount > 0 && "bg-muted text-foreground",
                  )}
                  disabled={initialLoading}
                >
                  <span className="flex size-3.5 shrink-0 items-center justify-center">
                    <Funnel className="size-3.5 stroke-1" />
                  </span>
                  <span className="flex items-center gap-1">
                    <span>{tTable("filter")}</span>
                    {activeFilterCount > 0 ? <span className="text-[10px] tabular-nums">{activeFilterCount}</span> : null}
                  </span>
                </Button>
              </PopoverTrigger>
              <PopoverContent align="end" className="w-[280px] p-2">
                <div className="space-y-0.5">
                  <AdminStatisticsSubjectFilter
                    value={statistics.subject}
                    permissionGroups={statistics.permissionGroups}
                    disabled={statistics.referenceLoading || initialLoading}
                    label={t("filters.subject")}
                    triggerClassName={cn(statistics.subject.type !== "all" && "bg-accent/50")}
                    onChange={statistics.setSubject}
                  />
                  <AdminStatisticsModelFilter
                    value={statistics.platformModelName || ALL_MODELS_VALUE}
                    fallbackValue={ALL_MODELS_VALUE}
                    options={modelOptions}
                    disabled={statistics.referenceLoading || initialLoading}
                    label={t("filters.model")}
                    triggerClassName={cn(statistics.platformModelName && "bg-accent/50")}
                    onChange={(value) => statistics.setPlatformModelName(value === ALL_MODELS_VALUE ? "" : value)}
                  />
                  <AdminStatisticsBillingFilter
                    value={statistics.billingScope}
                    disabled={initialLoading}
                    label={t("filters.billingScope")}
                    triggerClassName={cn(statistics.billingScope !== "all" && "bg-accent/50")}
                    options={[
                      { value: "all", label: t("filters.billingAll") },
                      { value: "free", label: t("filters.billingFree") },
                      { value: "billable", label: t("filters.billingBillable") },
                    ]}
                    onChange={statistics.setBillingScope}
                  />
                </div>
                {activeFilterCount > 0 ? (
                  <div className="mt-2 flex justify-end border-t border-border/60 pt-2">
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="h-7 px-2 text-xs font-normal shadow-none"
                      onClick={() => {
                        statistics.setSubject({ type: "all" });
                        statistics.setPlatformModelName("");
                        statistics.setBillingScope("all");
                      }}
                    >
                      {t("filters.clear")}
                    </Button>
                  </div>
                ) : null}
              </PopoverContent>
            </Popover>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-8 shrink-0 gap-1.5 px-2 text-xs font-normal text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
              onClick={statistics.refresh}
              disabled={statistics.loading || Boolean(statistics.rangeError)}
              aria-label={t("refresh")}
              title={t("refresh")}
            >
              <span className="flex size-3.5 shrink-0 items-center justify-center">
                <RefreshCw className={cn("size-3.5 stroke-1", statistics.loading && "animate-spin")} />
              </span>
              <span>{t("refresh")}</span>
            </Button>
          </div>
        </div>
        {rangeErrorText ? <p className="px-1 text-xs text-destructive">{rangeErrorText}</p> : null}

        <section className="grid grid-cols-2 gap-2 md:grid-cols-4">
          <MetricCard
            label={t("metrics.cost")}
            value={formatStatisticsCost(totals.billedUSD, statistics.billingDisplay)}
            icon={<BadgeDollarSign className="size-4" />}
            loading={initialLoading}
          />
          <MetricCard
            label={t("metrics.tokens")}
            value={formatStatisticsCount(totals.totalTokens, locale)}
            icon={<Braces className="size-4" />}
            loading={initialLoading}
          />
          <MetricCard
            label={t("metrics.calls")}
            value={formatStatisticsCount(totals.callCount, locale)}
            icon={<Activity className="size-4" />}
            loading={initialLoading}
          />
          <MetricCard
            label={t("metrics.latency")}
            value={formatStatisticsLatency(totals.avgLatencyMS, locale)}
            icon={<Timer className="size-4" />}
            loading={initialLoading}
          />
        </section>
      </div>

      <Separator className="mx-1 my-10" />

      <section className="space-y-6 px-1">
        <div className="flex min-h-10 flex-wrap items-center justify-between gap-3">
          <h3 className="text-sm font-semibold">{t("trend.title")}</h3>
          <Tabs
            value={trendMetric}
            onValueChange={(value) => setTrendMetric(value as AdminUsageStatisticsRankBy)}
          >
            <TabsList>
              {(["tokens", "cost", "calls"] as const).map((metric) => (
                <TabsTrigger key={metric} value={metric} disabled={initialLoading}>
                  {t(`rankBy.${metric}`)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>
        <div className="rounded-md bg-muted/30 p-2 md:p-3">
          <StatisticsTrendChart
            items={trendItems}
            granularity={granularity}
            rangeStartDate={chartRangeStartDate}
            rangeEndDate={chartRangeEndDate}
            rankBy={trendMetric}
            billingDisplay={statistics.billingDisplay}
            loading={initialLoading}
          />
        </div>
      </section>

      <Separator className="mx-1 my-10" />

      <section className="space-y-6 px-1">
        <div className="flex min-h-10 flex-wrap items-center justify-between gap-3">
          <h3 className="text-sm font-semibold">{t("rankings.models")}</h3>
          <Tabs
            value={statistics.modelRankingMetric}
            onValueChange={(value) => statistics.setModelRankingMetric(value as AdminUsageStatisticsRankBy)}
          >
            <TabsList>
              {(["tokens", "cost", "calls"] as const).map((metric) => (
                <TabsTrigger key={metric} value={metric} disabled={initialLoading}>
                  {t(`rankBy.${metric}`)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>
        <div className="rounded-md bg-muted/30 p-2 md:p-3">
          <StatisticsModelRankingChart
            items={statistics.statistics?.topModels ?? []}
            periods={trendItems}
            granularity={granularity}
            rangeStartDate={chartRangeStartDate}
            rangeEndDate={chartRangeEndDate}
            rankBy={statistics.appliedModelRankingMetric}
            billingDisplay={statistics.billingDisplay}
            loading={initialLoading}
          />
        </div>
      </section>

      <Separator className="mx-1 my-10" />

      <section className="space-y-6 px-1">
        <div className="flex min-h-10 flex-wrap items-center justify-between gap-3">
          <h3 className="text-sm font-semibold">{t("rankings.users")}</h3>
          <Tabs
            value={statistics.userRankingMetric}
            onValueChange={(value) => statistics.setUserRankingMetric(value as AdminUsageStatisticsRankBy)}
          >
            <TabsList>
              {(["tokens", "cost", "calls"] as const).map((metric) => (
                <TabsTrigger key={metric} value={metric} disabled={initialLoading}>
                  {t(`rankBy.${metric}`)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        </div>
        <div className="rounded-md bg-muted/30 p-2 md:p-3">
          <StatisticsUserRankingChart
            items={statistics.statistics?.topUsers ?? []}
            periods={trendItems}
            granularity={granularity}
            rangeStartDate={chartRangeStartDate}
            rangeEndDate={chartRangeEndDate}
            rankBy={statistics.appliedUserRankingMetric}
            billingDisplay={statistics.billingDisplay}
            loading={initialLoading}
          />
        </div>
      </section>
    </div>
  );
}
