import { formatRateMultiplier as formatMultiplier } from "@/shared/lib/billing-display";

// Time-of-day rate multipliers ("峰谷") attached to a model pricing entry.
// Mirrors backend/internal/application/billing/schedule_pricing.go: periods use
// the server's local clock, may wrap past midnight, and must not overlap.

export type SchedulePeriodForm = {
  id: string;
  name: string;
  weekdays: number[];
  start: string;
  end: string;
  ratePercent: string;
};

export type SchedulePeriodPayload = {
  name: string;
  weekdays: number[];
  start: string;
  end: string;
  ratePercent: number;
};

export const SCHEDULE_PERIOD_LIMITS = { maxPeriods: 32, nameLength: 32, minRatePercent: 1, maxRatePercent: 10000 } as const;
export const WEEKDAY_ORDER = [1, 2, 3, 4, 5, 6, 0] as const;
export const ALL_WEEKDAYS = [0, 1, 2, 3, 4, 5, 6] as const;

const MINUTES_PER_DAY = 24 * 60;
const TIME_RE = /^([01]?\d|2[0-3]):([0-5]\d)$/;

let periodSequence = 0;
export function createSchedulePeriodForm(partial?: Partial<Omit<SchedulePeriodForm, "id">>): SchedulePeriodForm {
  periodSequence += 1;
  return {
    id: `period-${Date.now()}-${periodSequence}`,
    name: "",
    weekdays: [1, 2, 3, 4, 5],
    start: "09:00",
    end: "18:00",
    ratePercent: "100",
    ...partial,
  };
}

export function parseClockMinute(value: string): number | null {
  const match = TIME_RE.exec(value.trim());
  if (!match) {
    return null;
  }
  return Number(match[1]) * 60 + Number(match[2]);
}

// Tolerant reader for hand-typed times, used to canonicalise an input on blur:
// "830" → 08:30, "8:00" → 08:00, "13:5" → 13:05. Null when unreadable.
export function parseClockLoose(value: string): number | null {
  // CJK input can arrive as full-width digits and punctuation.
  const text = value
    .trim()
    .replace(/[０-９]/g, (digit) => String.fromCharCode(digit.charCodeAt(0) - 0xfee0))
    .replace(/[：.、]/g, ":")
    .replace(/\s+/g, "");
  if (text === "") {
    return null;
  }
  const [, hourText = "", minuteText = ""] = /^(\d{1,2}):(\d{1,2})$/.exec(text) ?? [];
  if (hourText) {
    const hour = Number(hourText);
    const minute = Number(minuteText);
    if (minute > 59) {
      return null;
    }
    // 24:00 is the usual way to write "until midnight"; store it as 00:00.
    if (hour === 24 && minute === 0) {
      return 0;
    }
    return hour <= 23 ? hour * 60 + minute : null;
  }
  if (!/^\d{1,4}$/.test(text)) {
    return null;
  }
  // Digits only: 1–2 digits are an hour, 3 digits are H:MM, 4 digits are HH:MM.
  const padded = text.padStart(4, "0");
  const hour = Number(text.length <= 2 ? text : padded.slice(0, 2));
  const minute = text.length <= 2 ? 0 : Number(padded.slice(2));
  return hour <= 23 && minute <= 59 ? hour * 60 + minute : null;
}

// Canonical HH:MM for an input field; unreadable text is kept as typed so the
// save-time validator can still complain about it.
export function canonicalClock(value: string): string {
  const minute = parseClockLoose(value);
  return minute === null ? value.trim() : formatClock(minute);
}

// An end not after the start means the period runs past midnight.
export function isOvernightPeriod(period: Pick<SchedulePeriodPayload, "start" | "end">): boolean {
  const start = parseClockLoose(period.start);
  const end = parseClockLoose(period.end);
  return start !== null && end !== null && end <= start;
}

function formatClock(minute: number): string {
  return `${String(Math.floor(minute / 60)).padStart(2, "0")}:${String(minute % 60).padStart(2, "0")}`;
}

function isSchedulePeriodPayload(value: unknown): value is SchedulePeriodPayload {
  if (!value || typeof value !== "object") {
    return false;
  }
  const record = value as Record<string, unknown>;
  return (
    typeof record.name === "string" &&
    Array.isArray(record.weekdays) &&
    record.weekdays.every((day) => Number.isInteger(day) && (day as number) >= 0 && (day as number) <= 6) &&
    typeof record.start === "string" &&
    typeof record.end === "string" &&
    typeof record.ratePercent === "number"
  );
}

// Accepts the stored JSON string or an already-parsed object; null when absent or malformed.
export function parseSchedulePricing(raw: unknown): SchedulePeriodPayload[] | null {
  try {
    const parsed: unknown = typeof raw === "string" ? (raw.trim() ? JSON.parse(raw) : null) : raw;
    if (!parsed || typeof parsed !== "object" || !("periods" in parsed) || !Array.isArray(parsed.periods)) {
      return null;
    }
    const periods = parsed.periods.filter(isSchedulePeriodPayload);
    return periods.length === parsed.periods.length ? periods : null;
  } catch {
    return null;
  }
}

export function schedulePeriodsToForm(periods: SchedulePeriodPayload[] | null | undefined): SchedulePeriodForm[] {
  return (periods ?? []).map((period) =>
    createSchedulePeriodForm({
      name: period.name,
      weekdays: [...period.weekdays],
      start: period.start,
      end: period.end,
      ratePercent: String(period.ratePercent),
    }),
  );
}

// Week-minute spans [from, to) for overlap checks; a wrap-around period yields two spans.
function weekSpans(period: SchedulePeriodPayload): [number, number][] {
  const start = parseClockMinute(period.start) ?? 0;
  const end = parseClockMinute(period.end) ?? 0;
  const spans: [number, number][] = [];
  for (const weekday of period.weekdays) {
    const base = weekday * MINUTES_PER_DAY;
    if (start < end) {
      spans.push([base + start, base + end]);
      continue;
    }
    spans.push([base + start, base + MINUTES_PER_DAY]);
    const next = ((weekday + 1) % 7) * MINUTES_PER_DAY;
    if (end > 0) {
      spans.push([next, next + end]);
    }
  }
  return spans;
}

export type SchedulePeriodIssue = "name" | "weekdays" | "time" | "rate" | "overlap" | "count";

// Validates and normalises the editor rows into payloads; issues are keyed by row id ("" for list-level).
export function normalizeSchedulePeriods(rows: SchedulePeriodForm[]): { periods: SchedulePeriodPayload[]; issues: Map<string, SchedulePeriodIssue> } {
  const issues = new Map<string, SchedulePeriodIssue>();
  const periods: SchedulePeriodPayload[] = [];
  if (rows.length > SCHEDULE_PERIOD_LIMITS.maxPeriods) {
    issues.set("", "count");
  }
  for (const row of rows) {
    const name = row.name.trim();
    const start = parseClockMinute(row.start);
    const end = parseClockMinute(row.end);
    const ratePercent = Number(row.ratePercent);
    if (!name || name.length > SCHEDULE_PERIOD_LIMITS.nameLength) {
      issues.set(row.id, "name");
    } else if (row.weekdays.length === 0) {
      issues.set(row.id, "weekdays");
    } else if (start === null || end === null || start === end) {
      issues.set(row.id, "time");
    } else if (!Number.isInteger(ratePercent) || ratePercent < SCHEDULE_PERIOD_LIMITS.minRatePercent || ratePercent > SCHEDULE_PERIOD_LIMITS.maxRatePercent) {
      issues.set(row.id, "rate");
    } else {
      periods.push({ name, weekdays: [...row.weekdays].sort((a, b) => a - b), start: formatClock(start), end: formatClock(end), ratePercent });
    }
  }
  // periods[i] came from validRows[i]; resolve before marking overlaps so indices stay aligned.
  const validRows = rows.filter((item) => !issues.has(item.id));
  const spans = periods.flatMap((period, index) => weekSpans(period).map((span) => ({ span, index })));
  spans.sort((a, b) => a.span[0] - b.span[0]);
  for (let index = 1; index < spans.length; index += 1) {
    const previous = spans[index - 1];
    const current = spans[index];
    if (previous && current && current.span[0] < previous.span[1]) {
      const row = validRows[current.index];
      if (row) {
        issues.set(row.id, "overlap");
      }
    }
  }
  return { periods, issues };
}

export function stringifySchedulePricing(periods: SchedulePeriodPayload[]): string {
  return periods.length === 0 ? "" : JSON.stringify({ periods });
}

// Which period applies right now on the server clock, expressed via its UTC offset.
export function resolveCurrentSchedulePeriod(periods: readonly SchedulePeriodPayload[], serverUTCOffsetMinutes: number, now = new Date()): SchedulePeriodPayload | null {
  if (periods.length === 0) {
    return null;
  }
  const serverNow = new Date(now.getTime() + (serverUTCOffsetMinutes + now.getTimezoneOffset()) * 60_000);
  const weekMinute = serverNow.getDay() * MINUTES_PER_DAY + serverNow.getHours() * 60 + serverNow.getMinutes();
  for (const period of periods) {
    for (const [from, to] of weekSpans(period)) {
      if (weekMinute >= from && weekMinute < to) {
        return period;
      }
    }
  }
  return null;
}

export function formatRateMultiplier(ratePercent: number): string {
  return formatMultiplier(ratePercent / 100);
}
