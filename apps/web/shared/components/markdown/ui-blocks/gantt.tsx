"use client";

import { ChevronRight } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useLocale, useTranslations } from "next-intl";
import * as React from "react";

import { HeightTransition } from "@/components/ui/height-transition";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { UIBlockDefinition, UIBlockRenderProps, UIBlockSkeletonProps } from "./block";
import { s } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";

type GanttTask = {
  id?: string;
  name: string;
  start: string;
  // Either an end date or a duration in days.
  end?: string;
  days?: number;
  group?: string;
  progress?: number;
  milestone?: boolean;
  depends?: string[];
};

type GanttProps = {
  title?: string;
  tasks: GanttTask[];
};

type Resolved = {
  key: string;
  id: string;
  name: string;
  group?: string;
  start: number;
  end: number;
  progress: number;
  milestone: boolean;
  depends: string[];
  color: string;
  row: number;
};

const DAY = 86_400_000;
const ROW_HEIGHT = 28;
const GROUP_HEIGHT = 24;
const HEADER_HEIGHT = 28;
const PALETTE = ["var(--chart-1)", "var(--chart-2)", "var(--chart-3)", "var(--chart-4)", "var(--chart-5)"];
const MAX_TASKS = 60;
// Shared by row reflow, bar moves and scale changes so everything lands together.
const MOTION = { duration: 0.22, ease: [0.22, 1, 0.36, 1] } as const;

function parseDay(value: string | undefined): number | null {
  if (!value) {
    return null;
  }
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value.trim());
  if (!match) {
    return null;
  }
  const time = Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3]));
  return Number.isFinite(time) ? time : null;
}

function startOfTodayUTC(): number {
  const now = new Date();
  return Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
}

// Tick cadence follows the total span (sprint → days, roadmap → months); the
// column width stretches to fill the available width and only falls back to
// horizontal scrolling when days would get too narrow to read.
type ScaleUnit = "day" | "week" | "month";
const SCALE_UNITS: readonly ScaleUnit[] = ["day", "week", "month"];

function chooseScale(spanDays: number, available: number, forced?: ScaleUnit): { unit: ScaleUnit; pxPerDay: number } {
  const fit = available > 0 ? available / spanDays : 0;
  const unit = forced ?? (spanDays <= 45 ? "day" : spanDays <= 240 ? "week" : "month");
  if (unit === "day") {
    return { unit, pxPerDay: Math.min(36, Math.max(16, fit)) };
  }
  if (unit === "week") {
    return { unit, pxPerDay: Math.min(12, Math.max(4, fit)) };
  }
  return { unit, pxPerDay: Math.min(4, Math.max(1.5, fit)) };
}

// Transitive predecessors and successors of one task.
function dependencyChain(items: Resolved[], key: string): Set<string> {
  const byID = new Map(items.map((item) => [item.id, item]));
  const chain = new Set<string>([key]);
  const walkUp = (item: Resolved) => {
    for (const dependency of item.depends) {
      const from = byID.get(dependency);
      if (from && !chain.has(from.key)) {
        chain.add(from.key);
        walkUp(from);
      }
    }
  };
  const walkDown = (item: Resolved) => {
    for (const candidate of items) {
      if (candidate.depends.includes(item.id) && !chain.has(candidate.key)) {
        chain.add(candidate.key);
        walkDown(candidate);
      }
    }
  };
  const start = items.find((item) => item.key === key);
  if (start) {
    walkUp(start);
    walkDown(start);
  }
  return chain;
}

function resolveTasks(tasks: GanttTask[]): { items: Resolved[]; groups: { name: string | undefined; row: number }[]; min: number; max: number } {
  const groupIndex = new Map<string | undefined, number>();
  for (const task of tasks.slice(0, MAX_TASKS)) {
    if (!groupIndex.has(task.group)) {
      groupIndex.set(task.group, groupIndex.size);
    }
  }
  // Keep author order inside a group; cluster groups in first-seen order.
  const ordered = tasks.slice(0, MAX_TASKS).sort((a, b) => (groupIndex.get(a.group) ?? 0) - (groupIndex.get(b.group) ?? 0));
  const hasGroups = ordered.some((task) => task.group);
  const items: Resolved[] = [];
  const groups: { name: string | undefined; row: number }[] = [];
  let row = 0;
  let lastGroup: string | undefined | symbol = Symbol("none");
  ordered.forEach((task, index) => {
    const start = parseDay(task.start);
    if (start === null) {
      return;
    }
    const explicitEnd = parseDay(task.end);
    const end = task.milestone ? start : explicitEnd !== null ? Math.max(explicitEnd, start) : start + Math.max(1, Math.round(task.days ?? 1)) * DAY - DAY;
    if (hasGroups && task.group !== lastGroup) {
      groups.push({ name: task.group, row });
      lastGroup = task.group;
      row += 1;
    }
    items.push({
      key: `${index}`,
      id: task.id ?? task.name,
      name: task.name,
      group: task.group,
      start,
      end,
      progress: Math.min(100, Math.max(0, task.progress ?? 0)),
      milestone: Boolean(task.milestone),
      depends: task.depends ?? [],
      color: PALETTE[(groupIndex.get(task.group) ?? 0) % PALETTE.length] as string,
      row,
    });
    row += 1;
  });
  const min = Math.min(...items.map((item) => item.start));
  const max = Math.max(...items.map((item) => item.end));
  return { items, groups, min, max };
}

function Gantt({ id, props }: UIBlockRenderProps<GanttProps>) {
  const t = useTranslations("chat.markdown.uiBlock");
  const locale = useLocale();
  const { items, groups, min, max } = React.useMemo(() => resolveTasks(props.tasks), [props.tasks]);
  const today = React.useMemo(() => startOfTodayUTC(), []);
  const [hovered, setHovered] = React.useState<string | null>(null);
  const [selected, setSelected] = React.useState<string | null>(null);
  const [collapsed, setCollapsed] = React.useState<ReadonlySet<string | undefined>>(() => new Set());
  const [unit, setUnit] = React.useState<ScaleUnit | null>(null);
  const timelineRef = React.useRef<HTMLDivElement>(null);
  const [available, setAvailable] = React.useState(0);

  React.useLayoutEffect(() => {
    const element = timelineRef.current;
    if (!element) {
      return;
    }
    const measure = () => setAvailable(element.clientWidth);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  if (items.length === 0) {
    return (
      <div className="px-4 pb-4 pt-5">
        {props.title ? <h3 className="text-lg font-semibold leading-7">{props.title}</h3> : null}
        <p className="py-6 text-center text-xs text-muted-foreground">{t("empty")}</p>
      </div>
    );
  }

  // Pad the range by a couple of days so the first bar does not touch the edge.
  const rangeStart = min - 2 * DAY;
  const baseSpanDays = Math.round((max + 3 * DAY - rangeStart) / DAY);
  const baseScale = chooseScale(baseSpanDays, available, unit ?? undefined);
  // Keep at least 20px after the last bar so an end-of-range milestone has room.
  const rangeEnd = max + Math.max(3, Math.ceil(20 / baseScale.pxPerDay)) * DAY;
  const spanDays = Math.round((rangeEnd - rangeStart) / DAY);
  const scale = chooseScale(spanDays, available, unit ?? undefined);
  const width = Math.ceil(spanDays * scale.pxPerDay);
  const toX = (time: number) => ((time - rangeStart) / DAY) * scale.pxPerDay;
  const byID = new Map(items.map((item) => [item.id, item]));
  const tracksProgress = items.some((item) => item.progress > 0);

  // Rows in display order; tasks inside a collapsed group are skipped.
  const ordered = [
    ...groups.map((group) => ({ kind: "group" as const, row: group.row, name: group.name })),
    ...items.map((item) => ({ kind: "task" as const, row: item.row, item })),
  ].sort((a, b) => a.row - b.row);
  const rowsInOrder: ({ kind: "group"; name: string | undefined; top: number } | { kind: "task"; item: Resolved; top: number })[] = [];
  const topOf = new Map<string, number>();
  let cursor = 0;
  for (const entry of ordered) {
    if (entry.kind === "group") {
      rowsInOrder.push({ kind: "group", name: entry.name, top: cursor });
      cursor += GROUP_HEIGHT;
    } else if (!collapsed.has(entry.item.group)) {
      rowsInOrder.push({ kind: "task", item: entry.item, top: cursor });
      topOf.set(entry.item.key, cursor);
      cursor += ROW_HEIGHT;
    }
  }
  const bodyHeight = cursor;
  const visibleItems = items.filter((item) => topOf.has(item.key));
  const layoutSignature = `${scale.unit}:${[...collapsed].join("|")}`;
  const chain = selected ? dependencyChain(items, selected) : null;
  const toggleGroup = (name: string | undefined) =>
    setCollapsed((current) => {
      const next = new Set(current);
      if (next.has(name)) {
        next.delete(name);
      } else {
        next.add(name);
      }
      return next;
    });

  const dayFormat = new Intl.DateTimeFormat(locale, { month: "numeric", day: "numeric", timeZone: "UTC" });
  const monthFormat = new Intl.DateTimeFormat(locale, { year: "numeric", month: "short", timeZone: "UTC" });
  const fullFormat = new Intl.DateTimeFormat(locale, { year: "numeric", month: "numeric", day: "numeric", timeZone: "UTC" });

  const ticks: { time: number; label: string; major: boolean }[] = [];
  // Narrow day columns label every other day; a month/day label also claims the next slot.
  const sparseDays = scale.unit === "day" && scale.pxPerDay < 22;
  let labelBlockedUntil = 0;
  for (let time = rangeStart; time <= rangeEnd; time += DAY) {
    const date = new Date(time);
    if (scale.unit === "day") {
      const monthStart = date.getUTCDate() === 1 || time === rangeStart;
      const dayIndex = Math.round((time - rangeStart) / DAY);
      let label = "";
      if (monthStart) {
        label = dayFormat.format(date);
        labelBlockedUntil = time + DAY;
      } else if (time > labelBlockedUntil && (!sparseDays || dayIndex % 2 === 0)) {
        label = String(date.getUTCDate());
      }
      ticks.push({ time, label, major: date.getUTCDay() === 1 });
    } else if (scale.unit === "week" && date.getUTCDay() === 1) {
      ticks.push({ time, label: dayFormat.format(date), major: true });
    } else if (scale.unit === "month" && date.getUTCDate() === 1) {
      ticks.push({ time, label: monthFormat.format(date), major: true });
    }
  }
  const weekendBands: { from: number; to: number }[] = [];
  if (scale.unit === "day") {
    for (let time = rangeStart; time <= rangeEnd; time += DAY) {
      const day = new Date(time).getUTCDay();
      if (day === 6) {
        weekendBands.push({ from: time, to: time + 2 * DAY });
      }
    }
  }
  const todayVisible = today >= rangeStart && today <= rangeEnd;

  return (
    <div className="px-4 pb-4 pt-5">
      <div className="mb-3 flex items-baseline justify-between gap-3">
        {props.title ? <h3 className="text-lg font-semibold leading-7">{props.title}</h3> : <span />}
        <div className="flex shrink-0 items-center gap-0.5 text-[11px] text-muted-foreground" role="group" aria-label={t("ganttScale")}>
          {SCALE_UNITS.map((option) => (
            <button
              key={option}
              type="button"
              aria-pressed={scale.unit === option}
              onClick={() => setUnit(option)}
              className={cn("rounded-md px-1.5 py-0.5 transition-colors hover:text-foreground", scale.unit === option && "bg-muted text-foreground")}
            >
              {t(`ganttUnit_${option}`)}
            </button>
          ))}
        </div>
      </div>
      <HeightTransition contentClassName="flex">
        <div className="w-max max-w-[45%] shrink-0 pr-3">
          <div className="border-b-[0.5px] border-border" style={{ height: HEADER_HEIGHT }} />
          <AnimatePresence initial={false}>
          {rowsInOrder.map((entry) =>
            entry.kind === "group" ? (
              <motion.button
                key={`${id}-g-${entry.name ?? ""}`}
                layout="position"
                transition={MOTION}
                type="button"
                aria-expanded={!collapsed.has(entry.name)}
                onClick={() => toggleGroup(entry.name)}
                className="flex w-full items-center gap-1 px-3 text-left text-[11px] font-medium tracking-[0.04em] text-muted-foreground transition-colors hover:text-foreground"
                style={{ height: GROUP_HEIGHT }}
              >
                <ChevronRight className={cn("size-3 shrink-0 transition-transform", !collapsed.has(entry.name) && "rotate-90")} strokeWidth={2} />
                <span className="truncate">{entry.name}</span>
              </motion.button>
            ) : (
              <motion.button
                key={`${id}-n-${entry.item.key}`}
                layout="position"
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: ROW_HEIGHT, opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                transition={MOTION}
                type="button"
                aria-pressed={selected === entry.item.key}
                onClick={() => setSelected((current) => (current === entry.item.key ? null : entry.item.key))}
                className={cn(
                  "flex w-full items-center gap-3 overflow-hidden text-left text-xs transition-colors",
                  groups.length > 0 ? "pl-7 pr-3" : "px-3",
                  hovered === entry.item.key && "bg-foreground/[0.03]",
                  selected === entry.item.key && "font-medium",
                  chain && !chain.has(entry.item.key) && "text-muted-foreground/60",
                )}
                onMouseEnter={() => setHovered(entry.item.key)}
                onMouseLeave={() => setHovered(null)}
              >
                <span className="truncate">{entry.item.name}</span>
                {tracksProgress && !entry.item.milestone ? <span className="ml-auto shrink-0 text-[10px] tabular-nums text-muted-foreground">{entry.item.progress}%</span> : null}
              </motion.button>
            ),
          )}
          </AnimatePresence>
        </div>

        <div ref={timelineRef} className="min-w-0 flex-1 overflow-x-auto">
          <svg width={width} height={HEADER_HEIGHT + bodyHeight} className="block" role="img" aria-label={props.title ?? "gantt"}>
            <AnimatePresence initial={false}>
            <motion.g key={`labels-${scale.unit}`} className="fill-muted-foreground" fontSize={10} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={MOTION}>
              {ticks
                .filter((tick) => tick.label)
                .map((tick) => (
                  <text key={`t-${tick.time}`} x={toX(tick.time) + 3} y={HEADER_HEIGHT - 10} className={cn(new Date(tick.time).getUTCDate() <= (scale.unit === "day" ? 0 : 7) && "fill-foreground font-medium")}>
                    {tick.label}
                  </text>
                ))}
            </motion.g>
            </AnimatePresence>
            <line x1={0} x2={width} y1={HEADER_HEIGHT - 0.25} y2={HEADER_HEIGHT - 0.25} className="stroke-border" strokeWidth={0.5} />
            <g transform={`translate(0 ${HEADER_HEIGHT})`}>
              <AnimatePresence initial={false}>
                <motion.g key={`grid-${scale.unit}`} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={MOTION}>
                  {weekendBands.map((band) => (
                    <rect key={`w-${band.from}`} x={toX(band.from)} y={0} width={toX(band.to) - toX(band.from)} height={bodyHeight} className="fill-foreground/[0.02]" />
                  ))}
                  <g className="stroke-border" strokeWidth={0.5}>
                    {ticks
                      .filter((tick) => tick.major)
                      .map((tick) => (
                        <line key={`v-${tick.time}`} x1={toX(tick.time)} x2={toX(tick.time)} y1={0} y2={bodyHeight} />
                      ))}
                  </g>
                </motion.g>
              </AnimatePresence>
              <AnimatePresence initial={false}>
              <motion.g key={`arrows-${layoutSignature}`} className="stroke-muted-foreground/55" fill="none" strokeWidth={0.75} initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} transition={MOTION}>
                {visibleItems.flatMap((item) =>
                  item.depends.map((dependency) => {
                    const from = byID.get(dependency);
                    const fromTop = from ? topOf.get(from.key) : undefined;
                    const itemTop = topOf.get(item.key);
                    if (!from || from === item || fromTop === undefined || itemTop === undefined) {
                      return null;
                    }
                    const inChain = chain ? chain.has(from.key) && chain.has(item.key) : true;
                    const x1 = from.milestone ? toX(from.start) + scale.pxPerDay / 2 : toX(from.end + DAY);
                    const y1 = fromTop + ROW_HEIGHT / 2;
                    const x2 = item.milestone ? toX(item.start) + scale.pxPerDay / 2 : toX(item.start);
                    const y2 = itemTop + ROW_HEIGHT / 2;
                    // Straight elbow when the successor starts later; otherwise route
                    // around through the gap between the two rows.
                    const path =
                      x2 >= x1 + 12
                        ? `M${x1} ${y1}H${x1 + 6}V${y2}H${x2 - 2}`
                        : `M${x1} ${y1}H${x1 + 6}V${y1 + (y2 > y1 ? ROW_HEIGHT / 2 : -ROW_HEIGHT / 2)}H${x2 - 6}V${y2}H${x2 - 2}`;
                    return (
                      <path
                        key={`d-${item.key}-${dependency}`}
                        d={path}
                        markerEnd={`url(#${id}-arrow${inChain && chain ? "-active" : ""})`}
                        className={cn(chain && (inChain ? "stroke-primary" : "opacity-30"))}
                        strokeWidth={inChain && chain ? 1.25 : undefined}
                      />
                    );
                  }),
                )}
              </motion.g>
              </AnimatePresence>
              <AnimatePresence initial={false}>
              {visibleItems.map((item) => {
                const top = topOf.get(item.key) ?? 0;
                const y = top + (ROW_HEIGHT - 14) / 2;
                const hover = hovered === item.key || selected === item.key;
                const dimmed = chain ? !chain.has(item.key) : false;
                // Bars mirror the row buttons on the left, which carry the keyboard path.
                const select = () => setSelected((current) => (current === item.key ? null : item.key));
                const title = `${item.name} · ${fullFormat.format(new Date(item.start))}${item.milestone ? "" : ` – ${fullFormat.format(new Date(item.end))} · ${t("ganttDays", { count: Math.round((item.end - item.start) / DAY) + 1 })}`}${item.progress > 0 ? ` · ${item.progress}%` : ""}`;
                // The row group only moves vertically; horizontal geometry lives on
                // the rects so a scale change slides bars instead of redrawing them.
                const rowProps = {
                  className: cn("cursor-pointer", dimmed && "opacity-25"),
                  initial: { opacity: 0, y: top },
                  animate: { opacity: 1, y: top },
                  exit: { opacity: 0 },
                  transition: MOTION,
                  onMouseEnter: () => setHovered(item.key),
                  onMouseLeave: () => setHovered(null),
                  onClick: select,
                } as const;
                if (item.milestone) {
                  const cx = toX(item.start) + scale.pxPerDay / 2;
                  return (
                    <motion.g key={`b-${item.key}`} {...rowProps}>
                      <title>{title}</title>
                      <motion.g animate={{ x: cx }} transition={MOTION}>
                        <rect x={-6} y={y - top + 1} width={12} height={12} transform={`rotate(45 0 ${y - top + 7})`} fill={item.color} opacity={hover ? 1 : 0.9} />
                      </motion.g>
                    </motion.g>
                  );
                }
                const x = toX(item.start);
                const barWidth = Math.max(scale.pxPerDay, toX(item.end + DAY) - x);
                return (
                  <motion.g key={`b-${item.key}`} {...rowProps}>
                    <title>{title}</title>
                    <motion.rect animate={{ x, width: barWidth }} transition={MOTION} y={y - top} height={14} rx={3} fill={item.color} opacity={tracksProgress ? (hover ? 0.6 : 0.4) : hover ? 1 : 0.85} />
                    {item.progress > 0 ? <motion.rect animate={{ x, width: (barWidth * item.progress) / 100 }} transition={MOTION} y={y - top} height={14} rx={3} fill={item.color} /> : null}
                  </motion.g>
                );
              })}
              </AnimatePresence>
              {todayVisible ? (
                <motion.g animate={{ x: toX(today) + scale.pxPerDay / 2 }} transition={MOTION}>
                  <line x1={0} x2={0} y1={0} y2={bodyHeight} className="stroke-primary" strokeWidth={1} strokeDasharray="3 3" />
                  <text x={4} y={10} fontSize={9} className="fill-primary">
                    {t("ganttToday")}
                  </text>
                </motion.g>
              ) : null}
            </g>
            <defs>
              <marker id={`${id}-arrow`} viewBox="0 0 6 6" refX="5" refY="3" markerWidth="6" markerHeight="6" orient="auto">
                <path d="M0 0L6 3L0 6z" className="fill-muted-foreground/55" />
              </marker>
              <marker id={`${id}-arrow-active`} viewBox="0 0 6 6" refX="5" refY="3" markerWidth="6" markerHeight="6" orient="auto">
                <path d="M0 0L6 3L0 6z" className="fill-primary" />
              </marker>
            </defs>
          </svg>
        </div>
      </HeightTransition>
    </div>
  );
}

// Same header and row heights as the chart so the swap only fills in detail.
function GanttSkeleton({ items = 0 }: UIBlockSkeletonProps) {
  return (
    <UIBlockFrame className="px-4 pb-4 pt-5">
      <Skeleton className="mb-3 h-7 w-1/3 rounded-md" />
      <div style={{ height: HEADER_HEIGHT }} className="border-b-[0.5px] border-border" />
      <div>
        {Array.from({ length: Math.max(2, items) }).map((_, index) => (
          <div key={`gantt-row-${index}`} className="flex items-center gap-3" style={{ height: ROW_HEIGHT }}>
            <Skeleton className="h-3.5 w-32 rounded-sm" />
            <Skeleton className="h-3.5 rounded-sm" style={{ marginLeft: `${(index * 9) % 40}%`, width: `${18 + ((index * 17) % 30)}%` }} />
          </div>
        ))}
      </div>
    </UIBlockFrame>
  );
}

export const ganttDefinition: UIBlockDefinition<GanttProps> = {
  name: "gantt",
  version: 1,
  schema: s.object(
    {
      title: s.string(),
      tasks: s.array(
        s.object(
          {
            id: s.string(),
            name: s.string(),
            start: s.string(),
            end: s.string(),
            days: s.number(),
            group: s.string(),
            progress: s.number(),
            milestone: s.boolean(),
            depends: s.array(s.string()),
          },
          ["name", "start"],
        ),
        { minItems: 1 },
      ),
    },
    ["tasks"],
  ),
  Component: Gantt,
  Skeleton: GanttSkeleton,
};
