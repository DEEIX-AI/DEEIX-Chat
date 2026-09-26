"use client";

import { Minus, TrendingDown, TrendingUp } from "lucide-react";
import * as React from "react";

import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { UIBlockDefinition, UIBlockRenderProps, UIBlockSkeletonProps } from "./block";
import { s } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";

const TRENDS = ["up", "down", "flat"] as const;
const NUMBER_FORMAT = new Intl.NumberFormat("en-US", { maximumFractionDigits: 2 });
const SPARK_HEIGHT = 32;

type StatItem = {
  label: string;
  value: string | number | boolean;
  unit?: string;
  delta?: string;
  trend?: (typeof TRENDS)[number];
  note?: string;
  // Recent values, oldest first; drawn as a sparkline the reader can scrub.
  history?: number[];
};

export type StatGridProps = {
  title?: string;
  items: StatItem[];
  // Labels for history points, shared by every item (e.g. dates).
  periods?: string[];
};

function StatGrid({ id, props }: UIBlockRenderProps<StatGridProps>) {
  return (
    <div className="px-4 pb-4 pt-5">
      {props.title ? <h3 className="mb-3 text-lg font-semibold leading-7">{props.title}</h3> : null}
      <div className="grid grid-cols-[repeat(auto-fit,minmax(10rem,1fr))] gap-3">
        {props.items.map((item, index) => (
          <StatCard key={`${id}-${index}-${item.label}`} item={item} periods={props.periods} />
        ))}
      </div>
    </div>
  );
}

function StatCard({ item, periods }: { item: StatItem; periods?: string[] }) {
  const history = React.useMemo(() => (item.history ?? []).filter((value) => Number.isFinite(value)), [item.history]);
  const [hoverIndex, setHoverIndex] = React.useState<number | null>(null);
  const hovered = hoverIndex !== null ? history[hoverIndex] : undefined;
  const Trend = item.trend === "up" ? TrendingUp : item.trend === "down" ? TrendingDown : Minus;
  const trendClass = item.trend === "up" ? "text-green-600 dark:text-green-400" : item.trend === "down" ? "text-red-600 dark:text-red-400" : "text-muted-foreground";

  return (
    <div className="flex min-w-0 flex-col rounded-lg border-[0.5px] border-border bg-background p-3" onMouseLeave={() => setHoverIndex(null)}>
      <p className="flex items-baseline justify-between gap-2 text-[11px] text-muted-foreground">
        <span className="truncate">{item.label}</span>
        {hoverIndex !== null ? <span className="shrink-0 tabular-nums">{periods?.[hoverIndex] ?? `#${hoverIndex + 1}`}</span> : null}
      </p>
      <p className="mt-1 flex items-baseline gap-1 tabular-nums">
        <span className={cn("truncate text-xl font-semibold leading-7", hovered !== undefined && "text-foreground/80")}>{hovered !== undefined ? NUMBER_FORMAT.format(hovered) : String(item.value)}</span>
        {item.unit ? <span className="shrink-0 text-xs text-muted-foreground">{item.unit}</span> : null}
      </p>
      {history.length > 1 ? <Sparkline values={history} hoverIndex={hoverIndex} onHover={setHoverIndex} trend={item.trend} /> : null}
      {item.delta || item.note ? (
        <div className="mt-auto flex items-center justify-between gap-2 pt-1.5">
          {item.delta ? (
            <span className={cn("inline-flex min-w-0 items-center gap-1 truncate text-xs tabular-nums", trendClass)}>
              <Trend className="size-3 shrink-0" strokeWidth={2} />
              {item.delta}
            </span>
          ) : (
            <span />
          )}
          {item.note ? <span className="min-w-0 truncate text-[11px] text-muted-foreground">{item.note}</span> : null}
        </div>
      ) : null}
    </div>
  );
}

function Sparkline({
  values,
  hoverIndex,
  onHover,
  trend,
}: {
  values: number[];
  hoverIndex: number | null;
  onHover: (index: number | null) => void;
  trend?: StatItem["trend"];
}) {
  const ref = React.useRef<HTMLDivElement>(null);
  const [width, setWidth] = React.useState(0);
  React.useLayoutEffect(() => {
    const element = ref.current;
    if (!element) {
      return;
    }
    const measure = () => setWidth(element.clientWidth);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const stepX = values.length > 1 ? width / (values.length - 1) : 0;
  const toX = (index: number) => index * stepX;
  const toY = (value: number) => 3 + (1 - (value - min) / span) * (SPARK_HEIGHT - 6);
  const points = values.map((value, index) => `${toX(index).toFixed(1)},${toY(value).toFixed(1)}`).join(" ");
  const stroke = trend === "up" ? "stroke-green-600 dark:stroke-green-400" : trend === "down" ? "stroke-red-600 dark:stroke-red-400" : "stroke-foreground/60";
  const fill = trend === "up" ? "fill-green-600 dark:fill-green-400" : trend === "down" ? "fill-red-600 dark:fill-red-400" : "fill-foreground/60";

  const onMove = (event: React.PointerEvent<HTMLDivElement>) => {
    const rect = event.currentTarget.getBoundingClientRect();
    const ratio = Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width));
    onHover(Math.round(ratio * (values.length - 1)));
  };

  return (
    <div ref={ref} className="mt-2 cursor-crosshair touch-none" style={{ height: SPARK_HEIGHT }} onPointerMove={onMove} onPointerDown={onMove} onPointerLeave={() => onHover(null)}>
      {width > 0 ? (
        <svg width={width} height={SPARK_HEIGHT} className="block overflow-visible" role="img" aria-hidden="true">
          <polyline points={points} fill="none" strokeWidth={1.5} strokeLinejoin="round" strokeLinecap="round" className={stroke} />
          {hoverIndex !== null && values[hoverIndex] !== undefined ? (
            <g>
              <line x1={toX(hoverIndex)} x2={toX(hoverIndex)} y1={0} y2={SPARK_HEIGHT} className="stroke-border" strokeWidth={1} />
              <circle cx={toX(hoverIndex)} cy={toY(values[hoverIndex])} r={3} className={cn(fill, "stroke-background")} strokeWidth={1.5} />
            </g>
          ) : (
            <circle cx={toX(values.length - 1)} cy={toY(values[values.length - 1] ?? 0)} r={2.5} className={fill} />
          )}
        </svg>
      ) : null}
    </div>
  );
}

function StatGridSkeleton({ items = 0 }: UIBlockSkeletonProps) {
  return (
    <UIBlockFrame className="px-4 pb-4 pt-5">
      <Skeleton className="mb-3 h-7 w-1/3 rounded-md" />
      <div className="grid grid-cols-[repeat(auto-fit,minmax(10rem,1fr))] gap-3">
        {Array.from({ length: Math.max(2, items) }).map((_, index) => (
          <Skeleton key={`stat-grid-${index}`} className="h-[5.75rem] rounded-lg" />
        ))}
      </div>
    </UIBlockFrame>
  );
}

export const statGridDefinition: UIBlockDefinition<StatGridProps> = {
  name: "stat-grid",
  version: 1,
  schema: s.object(
    {
      title: s.string(),
      periods: s.array(s.string()),
      items: s.array(
        s.object(
          {
            label: s.string(),
            value: s.cell(),
            unit: s.string(),
            delta: s.string(),
            trend: s.string({ enum: TRENDS }),
            note: s.string(),
            history: s.array(s.number()),
          },
          ["label", "value"],
        ),
        { minItems: 1 },
      ),
    },
    ["items"],
  ),
  Component: StatGrid,
  Skeleton: StatGridSkeleton,
};
