"use client";

import { format } from "date-fns";
import { enUS, zhCN } from "date-fns/locale";
import { CalendarIcon } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import type { Matcher } from "react-day-picker";

import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { ADMIN_DATE_PICKER_TRIGGER_CLASSNAME } from "@/features/admin/components/admin-date-range-filter";
import { cn } from "@/lib/utils";

type AdminDateTimePickerProps = {
  value: string;
  disabled?: boolean;
  label: string;
  placeholder: string;
  defaultTime?: string;
  granularity?: "date" | "datetime";
  disabledDate?: Matcher | Matcher[];
  onChange: (value: string) => void;
};

export function adminDateTimeFormValue(value: string | null | undefined): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return `${format(date, "yyyy-MM-dd")}T${format(date, "HH:mm:ss")}`;
}

export function adminDateTimeValueToISOString(value: string): string | null | undefined {
  const text = value.trim();
  if (!text) return null;
  const date = new Date(text);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}

function parseDatePart(value: string): Date | undefined {
  const [dateText] = value.trim().split("T");
  if (!dateText) return undefined;
  const [year, month, day] = dateText.split("-").map((part) => Number.parseInt(part, 10));
  if (!year || !month || !day) return undefined;
  const date = new Date(year, month - 1, day);
  return Number.isNaN(date.getTime()) ? undefined : date;
}

function normalizeTimeValue(value: string, fallback: string): string {
  const trimmed = value.trim();
  if (/^\d{2}:\d{2}:\d{2}$/.test(trimmed)) return trimmed;
  if (/^\d{2}:\d{2}$/.test(trimmed)) return `${trimmed}:00`;
  const [, timeText = ""] = trimmed.split("T");
  if (/^\d{2}:\d{2}:\d{2}$/.test(timeText)) return timeText;
  if (/^\d{2}:\d{2}$/.test(timeText)) return `${timeText}:00`;
  return fallback;
}

function buildDateTimeValue(date: Date, time: string, fallback: string): string {
  return `${format(date, "yyyy-MM-dd")}T${normalizeTimeValue(time, fallback)}`;
}

function timePart(value: string, index: number, fallback: string): string {
  return normalizeTimeValue(value, fallback).split(":")[index] || "00";
}

function updateTimePart(value: string, index: number, nextPart: string, fallback: string): string {
  const parts = normalizeTimeValue(value, fallback).split(":");
  const max = index === 0 ? 23 : 59;
  const parsed = Number.parseInt(nextPart, 10);
  const normalized = String(Number.isFinite(parsed) ? Math.min(Math.max(parsed, 0), max) : 0).padStart(2, "0");
  parts[index] = normalized;
  return parts.join(":");
}

export function AdminDateTimePicker({
  value,
  disabled = false,
  label,
  placeholder,
  defaultTime = "23:59:59",
  granularity = "datetime",
  disabledDate,
  onChange,
}: AdminDateTimePickerProps) {
  const locale = useLocale();
  const tDateRange = useTranslations("common.dateRange");
  const selectedDate = parseDatePart(value);
  const normalizedTime = selectedDate ? normalizeTimeValue(value, defaultTime) : "";
  const dateOnly = granularity === "date";

  const handleTimePartChange = (index: number, nextPart: string) => {
    if (!selectedDate) return;
    onChange(buildDateTimeValue(selectedDate, updateTimePart(normalizedTime, index, nextPart, defaultTime), defaultTime));
  };

  return (
    <div className="space-y-1">
      <p className="text-xs text-muted-foreground">{label}</p>
      <div className={cn("grid gap-5", !dateOnly && "md:grid-cols-2")}>
        <Popover>
          <PopoverTrigger asChild>
            <Button
              type="button"
              variant="outline"
              className={cn(
                ADMIN_DATE_PICKER_TRIGGER_CLASSNAME,
                "h-8 justify-between",
                !selectedDate && "text-muted-foreground",
              )}
              disabled={disabled}
            >
              <span className="min-w-0 truncate">
                {selectedDate ? format(selectedDate, "yyyy-MM-dd") : placeholder}
              </span>
              <CalendarIcon className="size-3.5 opacity-70" />
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-auto p-0" align="start">
            <Calendar
              mode="single"
              selected={selectedDate}
              locale={locale === "zh-CN" ? zhCN : enUS}
              onSelect={(date) => onChange(date ? (dateOnly ? format(date, "yyyy-MM-dd") : buildDateTimeValue(date, normalizeTimeValue(value, defaultTime), defaultTime)) : "")}
              disabled={disabledDate}
              autoFocus
            />
            {dateOnly && selectedDate ? (
              <div className="px-2 pb-2">
                <Button
                  type="button"
                  size="sm"
                  variant="secondary"
                  className="h-6 w-full text-xs"
                  disabled={disabled}
                  onClick={() => onChange("")}
                >
                  {tDateRange("clear")}
                </Button>
              </div>
            ) : null}
          </PopoverContent>
        </Popover>
        {dateOnly ? null : (
          <div className="grid grid-cols-4 gap-2">
            <Input
              type="number"
              min={0}
              max={23}
              value={selectedDate ? timePart(normalizedTime, 0, defaultTime) : ""}
              placeholder="HH"
              disabled={disabled || !selectedDate}
              className="px-2 text-center tabular-nums"
              onChange={(event) => handleTimePartChange(0, event.target.value)}
            />
            <Input
              type="number"
              min={0}
              max={59}
              value={selectedDate ? timePart(normalizedTime, 1, defaultTime) : ""}
              placeholder="MM"
              disabled={disabled || !selectedDate}
              className="px-2 text-center tabular-nums"
              onChange={(event) => handleTimePartChange(1, event.target.value)}
            />
            <Input
              type="number"
              min={0}
              max={59}
              value={selectedDate ? timePart(normalizedTime, 2, defaultTime) : ""}
              placeholder="SS"
              disabled={disabled || !selectedDate}
              className="px-2 text-center tabular-nums"
              onChange={(event) => handleTimePartChange(2, event.target.value)}
            />
            <Button
              type="button"
              variant="outline"
              className="h-8 w-full px-0 text-xs text-muted-foreground"
              disabled={disabled || !selectedDate}
              onClick={() => onChange("")}
              aria-label={tDateRange("clear")}
            >
              {tDateRange("clear")}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
