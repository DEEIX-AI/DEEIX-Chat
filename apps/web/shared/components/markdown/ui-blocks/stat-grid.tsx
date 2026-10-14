"use client";

import { Minus, TrendingDown, TrendingUp } from "lucide-react";
import * as React from "react";

import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { UIBlockDefinition, UIBlockRenderProps, UIBlockSkeletonProps } from "./block";
import { joinTextBlocks, textPair, textTitle, textValue } from "./block-text";
import { s } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";

const TRENDS = ["up", "down", "flat"] as const;
const TONES = ["positive", "negative", "neutral"] as const;
const NUMBER_FORMAT = new Intl.NumberFormat("en-US", { maximumFractionDigits: 2 });
const SPARK_HEIGHT = 32;
// Card width floor and grid gap (gap-3), in rem. The floor still fits two
// cards side by side on a phone; it matches the grid classes below.
const MIN_CARD_REM = 8.5;
const GAP_REM = 0.75;
// More columns squeeze labels and notes into slivers on wide screens.
const MAX_COLUMNS = 4;

type Tone = (typeof TONES)[number];

// Same tints as the live block's badge.
const TONE_BADGE: Record<Tone, string> = {
  positive: "bg-green-600/10 text-green-700 dark:text-green-400",
  negative: "bg-red-600/10 text-red-700 dark:text-red-400",
  neutral: "bg-muted text-muted-foreground",
};
const TONE_STROKE: Record<Tone, string> = {
  positive: "stroke-green-600 dark:stroke-green-400",
  negative: "stroke-red-600 dark:stroke-red-400",
  neutral: "stroke-foreground/60",
};
const TONE_FILL: Record<Tone, string> = {
  positive: "fill-green-600 dark:fill-green-400",
  negative: "fill-red-600 dark:fill-red-400",
  neutral: "fill-foreground/60",
};

type StatItem = {
  label: string;
  value: string | number | boolean;
  unit?: string;
  delta?: string;
  trend?: (typeof TRENDS)[number];
  // Whether the change is good or bad. A falling weight or cost is good news,
  // so the arrow (trend) and the color cannot be the same field.
  tone?: Tone;
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
  const { ref, columns } = useBalancedColumns(props.items.length);
  return (
    <div className="px-4 pb-4 pt-5">
      {props.title ? <h3 className="mb-3 text-lg font-semibold leading-7">{props.title}</h3> : null}
      <div
        ref={ref}
        className="grid grid-cols-[repeat(auto-fit,minmax(8.5rem,1fr))] gap-3"
        style={columns ? { gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` } : undefined}
      >
        {props.items.map((item, index) => (
          <StatCard key={`${id}-${index}-${item.label}`} item={item} periods={props.periods} />
        ))}
      </div>
    </div>
  );
}

/**
 * Column count that fills rows evenly: six cards become 3 + 3 rather than
 * 5 + 1, and seven become 4 + 3. Plain auto-fit packs the first rows and
 * strands the remainder. Until the grid is measured the CSS auto-fit applies.
 */
function useBalancedColumns(count: number) {
  const ref = React.useRef<HTMLDivElement>(null);
  const [columns, setColumns] = React.useState<number | null>(null);
  React.useLayoutEffect(() => {
    const element = ref.current;
    if (!element) {
      return;
    }
    const items = Math.max(1, count);
    // The grid classes below size cards in rem, so the threshold has to be read
    // from the root; the element's own font size is not what rem resolves to.
    const rem = Number.parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
    const measure = () => {
      const fit = Math.max(1, Math.floor((element.clientWidth + GAP_REM * rem) / ((MIN_CARD_REM + GAP_REM) * rem)));
      const rows = Math.ceil(items / Math.min(fit, MAX_COLUMNS, items));
      setColumns(Math.ceil(items / rows));
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [count]);
  return { ref, columns };
}

function StatCard({ item, periods }: { item: StatItem; periods?: string[] }) {
  const history = React.useMemo(() => (item.history ?? []).filter((value) => Number.isFinite(value)), [item.history]);
  const [hoverIndex, setHoverIndex] = React.useState<number | null>(null);
  const hovered = hoverIndex !== null ? history[hoverIndex] : undefined;
  const Trend = item.trend === "up" ? TrendingUp : item.trend === "down" ? TrendingDown : Minus;
  const tone = item.tone ?? (item.trend === "up" ? "positive" : item.trend === "down" ? "negative" : "neutral");

  return (
    <div className="@container flex min-w-0 flex-col rounded-lg border-[0.5px] border-border bg-background p-3" onMouseLeave={() => setHoverIndex(null)}>
      {/* The delta sits in the corner beside the label, where it reads with the value and every
          card in a row lines up. A card too narrow for both moves it under the value instead
          of cutting the label short; cards in a row share a width, so they still line up.
          While scrubbing the sparkline, the point's period takes its place: the value then
          shows that point, which the delta does not describe. */}
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center @[9rem]:gap-x-2">
        <p className="col-start-1 row-start-1 min-h-5 truncate text-[11px] leading-5 text-muted-foreground" title={item.label}>
          {item.label}
        </p>
        {hoverIndex !== null || item.delta ? (
          <div className="col-start-1 row-start-3 mt-1 flex min-w-0 @[9rem]:col-start-2 @[9rem]:row-start-1 @[9rem]:mt-0 @[9rem]:max-w-[9rem] @[9rem]:justify-end">
            {hoverIndex !== null ? (
              <span className="truncate text-[11px] leading-5 tabular-nums text-muted-foreground">{periods?.[hoverIndex] ?? `#${hoverIndex + 1}`}</span>
            ) : (
              <span className={cn("inline-flex min-w-0 items-center gap-1 rounded-full px-1.5 py-0.5 text-[11px] font-medium leading-4 tabular-nums", TONE_BADGE[tone])} title={item.delta}>
                <Trend className="size-3 shrink-0" strokeWidth={2} />
                <span className="truncate">{item.delta}</span>
              </span>
            )}
          </div>
        ) : null}
        <p className="col-span-2 row-start-2 mt-1 flex min-w-0 items-baseline gap-1 tabular-nums">
          <span className={cn("truncate text-xl font-semibold leading-7", hovered !== undefined && "text-foreground/80")}>{hovered !== undefined ? NUMBER_FORMAT.format(hovered) : String(item.value)}</span>
          {item.unit ? <span className="shrink-0 text-xs text-muted-foreground">{item.unit}</span> : null}
        </p>
      </div>
      {history.length > 1 ? <Sparkline values={history} hoverIndex={hoverIndex} onHover={setHoverIndex} tone={tone} /> : null}
      {item.note ? (
        <p className="mt-1.5 line-clamp-2 break-words text-[11px] leading-4 text-muted-foreground" title={item.note}>
          {item.note}
        </p>
      ) : null}
    </div>
  );
}

function Sparkline({
  values,
  hoverIndex,
  onHover,
  tone,
}: {
  values: number[];
  hoverIndex: number | null;
  onHover: (index: number | null) => void;
  tone: Tone;
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
  const stroke = TONE_STROKE[tone];
  const fill = TONE_FILL[tone];

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
      <div className="grid grid-cols-[repeat(auto-fit,minmax(8.5rem,1fr))] gap-3">
        {Array.from({ length: Math.max(2, items) }).map((_, index) => (
          <Skeleton key={`stat-grid-${index}`} className="h-[5.75rem] rounded-lg" />
        ))}
      </div>
    </UIBlockFrame>
  );
}

function statGridText(props: StatGridProps): string {
  const items = props.items.map((item) => {
    const delta = item.delta?.trim() ? ` (${item.delta.trim()})` : "";
    return `- ${textPair(item.label, textValue(item.value), item.unit)}${delta}`;
  });
  return joinTextBlocks([...textTitle(props.title), items.join("\n")]);
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
            tone: s.string({ enum: TONES }),
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
  toText: statGridText,
};
