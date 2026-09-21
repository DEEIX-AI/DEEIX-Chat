"use client";

import { Info, Plus, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import {
  canonicalClock,
  createSchedulePeriodForm,
  isOvernightPeriod,
  normalizeSchedulePeriods,
  SCHEDULE_PERIOD_LIMITS,
  type SchedulePeriodForm,
  type SchedulePeriodIssue,
  WEEKDAY_ORDER,
} from "@/shared/model/schedule-pricing";

const WEEKDAY_KEYS = ["sun", "mon", "tue", "wed", "thu", "fri", "sat"] as const;

// Typing is left alone (digits, colon or not); on blur the text is read
// tolerantly and rewritten in the canonical 24h HH:MM shape.
function ClockInput({ value, disabled, onChange, onSettled }: { value: string; disabled?: boolean; onChange: (value: string) => void; onSettled?: () => void }) {
  return (
    <Input
      value={value}
      inputMode="numeric"
      placeholder="HH:MM"
      maxLength={5}
      className="text-center tabular-nums"
      disabled={disabled}
      onChange={(event) => onChange(event.target.value.replace(/[^\d:]/g, "").slice(0, 5))}
      onBlur={(event) => {
        onChange(canonicalClock(event.target.value));
        onSettled?.();
      }}
    />
  );
}

// Time-of-day rate multipliers of one model; one block per period, following the
// label-over-field grammar of the rest of the pricing dialog.
export function BillingScheduleEditor({
  periods,
  onChange,
  disabled,
  showErrors,
}: {
  periods: SchedulePeriodForm[];
  onChange: (periods: SchedulePeriodForm[]) => void;
  disabled?: boolean;
  // Validation runs on save; the parent turns this on when that attempt failed.
  showErrors?: boolean;
}) {
  const t = useTranslations("adminBilling.modelPricing.schedule");
  const tWeekday = useTranslations("adminBilling.modelPricing.schedule.weekdays");
  const issues = React.useMemo(() => (showErrors ? normalizeSchedulePeriods(periods).issues : null), [periods, showErrors]);

  const update = (id: string, patch: Partial<SchedulePeriodForm>) => onChange(periods.map((period) => (period.id === id ? { ...period, ...patch } : period)));
  const toggleWeekday = (period: SchedulePeriodForm, weekday: number) => {
    const next = period.weekdays.includes(weekday) ? period.weekdays.filter((day) => day !== weekday) : [...period.weekdays, weekday];
    update(period.id, { weekdays: next });
  };
  const issueText = (issue: SchedulePeriodIssue | undefined) => (issue ? t(`issues.${issue}`) : null);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-1.5">
          <p className="text-xs font-medium">{t("title")}</p>
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="inline-flex text-muted-foreground" aria-label={t("hint")}>
                <Info className="size-3.5" strokeWidth={1.5} />
              </span>
            </TooltipTrigger>
            <TooltipContent side="top" sideOffset={6} className="max-w-[320px] text-xs leading-5">
              {t("hint")}
            </TooltipContent>
          </Tooltip>
        </div>
        <Button type="button" variant="ghost" size="xs" disabled={disabled || periods.length >= SCHEDULE_PERIOD_LIMITS.maxPeriods} onClick={() => onChange([...periods, createSchedulePeriodForm()])}>
          <Plus className="size-3.5" />
          {t("add")}
        </Button>
      </div>

      {periods.map((period, index) => {
        const issue = issues?.get(period.id);
        const overnight = isOvernightPeriod(period);
        return (
          <div key={period.id} className={cn("space-y-3", index > 0 && "border-t border-dashed pt-4")}>
            <div className="grid grid-cols-2 gap-5">
              <div className="space-y-1">
                <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  {t("name")}
                  {overnight ? <Badge variant="secondary" className="h-3.5 rounded-sm px-1 py-0 text-[10px] font-normal leading-none">{t("nextDay")}</Badge> : null}
                </p>
                <Input value={period.name} maxLength={SCHEDULE_PERIOD_LIMITS.nameLength} disabled={disabled} onChange={(event) => update(period.id, { name: event.target.value })} />
              </div>
              <div className="grid grid-cols-[1fr_auto_1fr_5.5rem] items-end gap-2">
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">{t("start")}</p>
                  <ClockInput value={period.start} disabled={disabled} onChange={(start) => update(period.id, { start })} />
                </div>
                <span className="pb-2 text-xs text-muted-foreground">–</span>
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">{t("end")}</p>
                  <ClockInput value={period.end} disabled={disabled} onChange={(end) => update(period.id, { end })} />
                </div>
                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">{t("ratePercent")}</p>
                  <div className="relative">
                    <Input
                      value={period.ratePercent}
                      type="number"
                      min={SCHEDULE_PERIOD_LIMITS.minRatePercent}
                      max={SCHEDULE_PERIOD_LIMITS.maxRatePercent}
                      step="1"
                      className="pr-6 text-right tabular-nums [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                      disabled={disabled}
                      onChange={(event) => update(period.id, { ratePercent: event.target.value })}
                    />
                    <span className="pointer-events-none absolute inset-y-0 right-2.5 flex items-center text-xs text-muted-foreground">%</span>
                  </div>
                </div>
              </div>
            </div>
            <div className="grid grid-cols-2 items-center gap-5">
              <div className="grid h-8 grid-cols-7 overflow-hidden rounded-md border border-input/40" role="group" aria-label={t("weekdaysLabel")}>
                {WEEKDAY_ORDER.map((weekday) => {
                  const active = period.weekdays.includes(weekday);
                  return (
                    <button
                      key={weekday}
                      type="button"
                      aria-pressed={active}
                      disabled={disabled}
                      onClick={() => toggleWeekday(period, weekday)}
                      className={cn(
                        "text-xs transition-colors not-first:border-l not-first:border-input/40",
                        active ? "bg-primary/10 font-medium text-primary" : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
                      )}
                    >
                      {tWeekday(WEEKDAY_KEYS[weekday])}
                    </button>
                  );
                })}
              </div>
              <div className="flex min-w-0 items-center gap-3">
                {issue ? <p className="min-w-0 truncate text-xs text-destructive">{issueText(issue)}</p> : null}
                <Button type="button" variant="ghost" size="xs" className="ml-auto shrink-0 text-muted-foreground" disabled={disabled} onClick={() => onChange(periods.filter((item) => item.id !== period.id))}>
                  <Trash2 className="size-3.5" />
                  {t("remove")}
                </Button>
              </div>
            </div>
          </div>
        );
      })}
      {issues?.get("") ? <p className="text-xs text-destructive">{issueText(issues.get(""))}</p> : null}
    </div>
  );
}
