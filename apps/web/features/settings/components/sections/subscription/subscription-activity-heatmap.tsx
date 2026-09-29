"use client";

import { useTranslations } from "next-intl";
import * as React from "react";

import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { type ActivityDay, useSettingsUserActivity } from "@/features/settings/hooks/use-settings-user-activity";
import { formatTokenCount } from "@/features/settings/model/subscription-format";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import { isOneOf } from "@/shared/lib/type-guards";
import { SubscriptionActivityHeatmapSkeleton } from "./subscription-activity-heatmap-skeleton";

const ACTIVITY_METRICS = ["tokens", "requests"] as const;
export type ActivityMetric = (typeof ACTIVITY_METRICS)[number];
const isActivityMetric = isOneOf(ACTIVITY_METRICS);

type ActivityStats = {
  peakDayTokens: number;
  currentStreak: number;
  longestStreak: number;
  activeDays: number;
  totalRequests: number;
};

type HeatmapWeek = Array<ActivityDay | null>;

const HEATMAP_MAX_LEVEL = 4;
const REQUESTS_PER_LEVEL = 5;

const HEATMAP_CELL_CLASS = [
  "bg-muted",
  "bg-green-600/25 dark:bg-green-500/25",
  "bg-green-600/45 dark:bg-green-500/45",
  "bg-green-600/70 dark:bg-green-500/70",
  "bg-green-600 dark:bg-green-500",
] as const;

// Same tiers as LobeHub: tokens normalized relative to the peak, message counts use fixed steps.
function resolveLevel(value: number, isTokenMetric: boolean, peakTokens: number): number {
  if (value <= 0) return 0;
  const level = isTokenMetric
    ? Math.ceil((value / peakTokens) * HEATMAP_MAX_LEVEL)
    : Math.ceil(value / REQUESTS_PER_LEVEL);
  return Math.min(HEATMAP_MAX_LEVEL, Math.max(1, level));
}

// GitHub-style week columns: the first column aligns to Sunday, with leading empty cells as padding.
function buildHeatmapWeeks(days: ActivityDay[]): HeatmapWeek[] {
  if (days.length === 0) return [];
  const leadingEmpty = new Date(`${days[0].date}T00:00:00`).getDay();
  const cells: Array<ActivityDay | null> = [...Array.from({ length: leadingEmpty }, () => null), ...days];
  const weeks: HeatmapWeek[] = [];
  for (let index = 0; index < cells.length; index += 7) {
    const week = cells.slice(index, index + 7);
    while (week.length < 7) week.push(null);
    weeks.push(week);
  }
  return weeks;
}

// Ticks for the past year are split evenly into 12 months, so varying month lengths don't make label spacing uneven.
function buildMonthLabels(days: ActivityDay[], locale: string): string[] {
  if (days.length === 0) return [];
  const formatter = new Intl.DateTimeFormat(locale, { month: "short" });
  const lastDay = new Date(`${days[days.length - 1].date}T00:00:00`);
  return Array.from({ length: 12 }, (_, index) => (
    formatter.format(new Date(lastDay.getFullYear(), lastDay.getMonth() - 11 + index, 1))
  ));
}

function computeActivityStats(days: ActivityDay[]): ActivityStats {
  let peakDayTokens = 0;
  let activeDays = 0;
  let totalRequests = 0;
  let longestStreak = 0;
  let runningStreak = 0;
  for (const day of days) {
    peakDayTokens = Math.max(peakDayTokens, day.tokens);
    totalRequests += day.requests;
    if (day.requests > 0) {
      activeDays += 1;
      runningStreak += 1;
      longestStreak = Math.max(longestStreak, runningStreak);
    } else {
      runningStreak = 0;
    }
  }
  // Today isn't over yet: if the last day is 0, count the streak from yesterday.
  let cursor = days.length - 1;
  if (cursor >= 0 && days[cursor].requests === 0) cursor -= 1;
  let currentStreak = 0;
  while (cursor >= 0 && days[cursor].requests > 0) {
    currentStreak += 1;
    cursor -= 1;
  }
  return { peakDayTokens, currentStreak, longestStreak, activeDays, totalRequests };
}

function formatActivityDate(date: string, formatter: Intl.DateTimeFormat): string {
  return formatter.format(new Date(`${date}T00:00:00`));
}

function ActivityMetricTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-md bg-muted/35 px-3 py-3.5 md:px-4">
      <p className="truncate text-xs text-muted-foreground">{label}</p>
      <p className="mt-2 truncate text-base font-semibold tabular-nums text-foreground md:text-lg">{value}</p>
    </div>
  );
}

function HeatmapTooltipContent({ day, formattedDate }: { day: ActivityDay; formattedDate: string }) {
  const t = useTranslations("settings.subscriptionPage.activity.tooltip");
  return (
    <div className="grid min-w-[8rem] gap-1.5">
      <p className="font-medium">{formattedDate}</p>
      <div className="grid gap-1">
        <div className="flex items-center justify-between gap-6">
          <span>{t("requests")}</span>
          <span className="tabular-nums">{day.requests.toLocaleString("en-US")}</span>
        </div>
        <div className="flex items-center justify-between gap-6">
          <span>{t("tokens")}</span>
          <span className="tabular-nums">{day.tokens > 0 ? formatTokenCount(day.tokens) : "0"}</span>
        </div>
      </div>
    </div>
  );
}

export function SubscriptionActivityHeatmap({ accessToken }: { accessToken: string }) {
  const t = useTranslations("settings.subscriptionPage.activity");
  const { locale } = useAppLocale();
  const [metric, setMetric] = React.useState<ActivityMetric>("tokens");
  const days = useSettingsUserActivity(accessToken);

  const weeks = React.useMemo(() => buildHeatmapWeeks(days ?? []), [days]);
  const monthLabels = React.useMemo(() => buildMonthLabels(days ?? [], locale), [days, locale]);
  const stats = React.useMemo(() => computeActivityStats(days ?? []), [days]);
  const dateFormatter = React.useMemo(() => new Intl.DateTimeFormat(locale, { dateStyle: "medium" }), [locale]);

  if (days === null) {
    return <SubscriptionActivityHeatmapSkeleton />;
  }

  const metricValue = (day: ActivityDay) => (metric === "tokens" ? day.tokens : day.requests);
  const hasAnyActivity = stats.totalRequests > 0 || stats.peakDayTokens > 0;

  return (
    <div className="space-y-4 md:space-y-5">
      <div className="flex h-9 items-center justify-between gap-3 px-1">
        <h3 className="truncate text-sm font-semibold">{t("title")}</h3>
        <Tabs value={metric} onValueChange={(value) => {
          if (isActivityMetric(value)) setMetric(value);
        }}>
          <TabsList>
            <TabsTrigger value="tokens">{t("tokens")}</TabsTrigger>
            <TabsTrigger value="requests">{t("requests")}</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>

      <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
        <ActivityMetricTile label={t("peakDayTokens")} value={formatTokenCount(stats.peakDayTokens)} />
        <ActivityMetricTile label={t("currentStreak")} value={t("dayCount", { count: stats.currentStreak })} />
        <ActivityMetricTile label={t("longestStreak")} value={t("dayCount", { count: stats.longestStreak })} />
        <ActivityMetricTile label={t("totalActiveDays")} value={t("dayCount", { count: stats.activeDays })} />
      </div>

      <div className="space-y-3 rounded-md bg-muted/35 p-3">
        {!hasAnyActivity ? (
          <div className="flex h-[104px] items-center justify-center text-xs text-muted-foreground">{t("empty")}</div>
        ) : (
          <div className="overflow-x-auto pb-1">
            {/* GitHub style: week columns are flex-1 equal width filling the card, cells are aspect-square and scale with width; narrow screens keep a minimum readable width and scroll horizontally.
                Note: grid-auto-flow:column + implicit rows can't be used (Chromium resolves the cells' percentage/aspect-ratio sizes to 0). */}
            <div className="min-w-[640px] space-y-1">
              <div className="grid grid-cols-12 text-[10px] leading-none text-muted-foreground">
                {monthLabels.map((label, index) => (
                  <span key={`activity-month-${index}`} className="min-w-0 whitespace-nowrap">
                    {label}
                  </span>
                ))}
              </div>
              <TooltipProvider>
                <div className="flex gap-[3px]">
                  {weeks.map((week, weekIndex) => (
                    <div key={`activity-week-${weekIndex}`} className="flex min-w-0 flex-1 flex-col gap-[3px]">
                      {week.map((day, dayIndex) => {
                        if (!day) {
                          return <div key={`activity-empty-${weekIndex}-${dayIndex}`} className="aspect-square w-full rounded-[2px] bg-transparent" />;
                        }
                        const value = metricValue(day);
                        const level = resolveLevel(value, metric === "tokens", stats.peakDayTokens);
                        const formattedDate = formatActivityDate(day.date, dateFormatter);
                        return (
                          <Tooltip key={`activity-cell-${day.date}`}>
                            <TooltipTrigger asChild>
                              <span
                                role="img"
                                aria-label={formattedDate}
                                className={`aspect-square w-full cursor-default rounded-[2px] ${HEATMAP_CELL_CLASS[level]}`}
                              />
                            </TooltipTrigger>
                            <TooltipContent>
                              <HeatmapTooltipContent day={day} formattedDate={formattedDate} />
                            </TooltipContent>
                          </Tooltip>
                        );
                      })}
                    </div>
                  ))}
                </div>
              </TooltipProvider>
            </div>
          </div>
        )}

        <div className="flex flex-wrap items-center justify-between gap-2 px-1 text-xs text-muted-foreground">
          <span>{t("totalRequests", { count: stats.totalRequests })}</span>
          <span className="flex items-center gap-1">
            {t("less")}
            {HEATMAP_CELL_CLASS.map((cellClass, level) => (
              <span key={`activity-legend-${level}`} className={`size-2.5 rounded-[2px] ${cellClass}`} />
            ))}
            {t("more")}
          </span>
        </div>
      </div>
    </div>
  );
}
