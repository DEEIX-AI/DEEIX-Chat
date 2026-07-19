"use client";

import { differenceInCalendarDays, format } from "date-fns";
import { enUS, zhCN } from "date-fns/locale";
import { CalendarIcon } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import type { DateRange, Matcher } from "react-day-picker";

import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";

export const ADMIN_DATE_PICKER_TRIGGER_CLASSNAME =
  "w-full justify-start border-input/40 bg-transparent px-3 py-1 text-left text-xs font-normal shadow-none transition-[color,box-shadow] hover:bg-transparent focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40 dark:bg-input/30 dark:hover:bg-input/30";

type AdminDateRangeFilterProps = {
  fromValue: string;
  toValue: string;
  onFromChange: (value: string) => void;
  onToChange: (value: string) => void;
  disabled?: boolean;
  maxRangeDays?: number;
  placeholder?: string;
  triggerClassName?: string;
};

function parseDateValue(value: string): Date | undefined {
  const trimmed = value.trim();
  if (!trimmed) {
    return undefined;
  }
  const [year, month, day] = trimmed.split("-").map((part) => Number.parseInt(part, 10));
  if (!year || !month || !day) {
    return undefined;
  }
  const date = new Date(year, month - 1, day);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

export function AdminDateRangeFilter({
  fromValue,
  toValue,
  onFromChange,
  onToChange,
  disabled = false,
  maxRangeDays,
  placeholder,
  triggerClassName,
}: AdminDateRangeFilterProps) {
  const locale = useLocale();
  const t = useTranslations("common.dateRange");
  const hasValue = fromValue.trim() !== "" || toValue.trim() !== "";
  const selectedRange: DateRange | undefined = hasValue
    ? {
        from: parseDateValue(fromValue),
        to: parseDateValue(toValue),
      }
    : undefined;
  const rangeStart = selectedRange?.from;
  const disabledDates: Matcher | undefined = maxRangeDays && rangeStart && !selectedRange?.to
    ? (date) => Math.abs(differenceInCalendarDays(date, rangeStart)) >= maxRangeDays
    : undefined;
  const triggerLabel = selectedRange?.from
    ? selectedRange.to
      ? `${format(selectedRange.from, "yyyy-MM-dd")} - ${format(selectedRange.to, "yyyy-MM-dd")}`
      : format(selectedRange.from, "yyyy-MM-dd")
    : (placeholder ?? t("placeholder"));

  return (
    <div className="w-full min-w-0 space-y-2">
      <Popover>
        <PopoverTrigger asChild>
          <Button
            type="button"
            variant="outline"
            className={cn(
              ADMIN_DATE_PICKER_TRIGGER_CLASSNAME,
              "px-2.5 h-7",
              triggerClassName,
              !selectedRange?.from && "text-muted-foreground",
            )}
            disabled={disabled}
          >
            <CalendarIcon className="size-3 opacity-70" />
            <span className="min-w-0 truncate text-[11px]">{triggerLabel}</span>
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-auto p-0" align="start">
          <Calendar
            mode="range"
            defaultMonth={selectedRange?.from}
            selected={selectedRange}
            disabled={disabledDates}
            max={maxRangeDays ? maxRangeDays - 1 : undefined}
            resetOnSelect={Boolean(maxRangeDays)}
            locale={locale === "zh-CN" ? zhCN : enUS}
            onSelect={(range) => {
              onFromChange(range?.from ? format(range.from, "yyyy-MM-dd") : "");
              onToChange(range?.to ? format(range.to, "yyyy-MM-dd") : "");
            }}
            autoFocus
          />
          {hasValue ? (
            <div className="px-2 pb-2">
              <Button
                type="button"
                size="sm"
                variant="secondary"
                className="h-6 w-full text-xs"
                disabled={disabled}
                onClick={() => {
                  onFromChange("");
                  onToChange("");
                }}
              >
                {t("clear")}
              </Button>
            </div>
          ) : null}
        </PopoverContent>
      </Popover>
    </div>
  );
}
