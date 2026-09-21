"use client";

import * as React from "react";
import {
  Area,
  Bar,
  CartesianGrid,
  Cell,
  ComposedChart,
  Funnel,
  FunnelChart,
  LabelList,
  Line,
  Pie,
  PieChart,
  PolarAngleAxis,
  PolarGrid,
  PolarRadiusAxis,
  Radar,
  RadarChart,
  RadialBar,
  RadialBarChart,
  Scatter,
  ScatterChart,
  XAxis,
  YAxis,
} from "recharts";

import {
  type ChartConfig,
  ChartContainer,
  ChartInteractiveLegend,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart";
import { Skeleton } from "@/components/ui/skeleton";
import type { UIBlockDefinition, UIBlockRenderProps } from "./block";
import { s } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";

// Grouped digits keep long axis labels readable (1200000 → 1,200,000).
const NUMBER_FORMAT = new Intl.NumberFormat("en-US", { maximumFractionDigits: 2 });

const CHART_TYPES = ["line", "bar", "area", "horizontal-bar", "scatter", "pie", "radar", "funnel", "radial"] as const;
type ChartType = (typeof CHART_TYPES)[number];
const SERIES_TYPES = ["line", "bar", "area"] as const;
type SeriesType = (typeof SERIES_TYPES)[number];

export type ChartProps = {
  title?: string;
  type: ChartType;
  x?: (string | number)[];
  // `type` lets one series differ from the chart (bars + a trend line);
  // `axis: "right"` puts it on a second y axis when units differ.
  series: { name: string; data: number[]; type?: SeriesType; axis?: "left" | "right" }[];
  unit?: string;
  stacked?: boolean;
  // With `stacked`, normalises every x to 100%.
  percent?: boolean;
};

const PALETTE = ["var(--chart-1)", "var(--chart-2)", "var(--chart-3)", "var(--chart-4)", "var(--chart-5)"];
const CARTESIAN_MARGIN = { top: 8, right: 8, left: 0, bottom: 0 };
const PERCENT_FORMAT = new Intl.NumberFormat("en-US", { style: "percent", maximumFractionDigits: 0 });

function seriesKey(index: number): string {
  return `s${index}`;
}

function Chart({ id, props }: UIBlockRenderProps<ChartProps>) {
  const [hidden, setHidden] = React.useState<ReadonlySet<string>>(() => new Set());
  const config = React.useMemo<ChartConfig>(
    () => Object.fromEntries(props.series.map((series, index) => [seriesKey(index), { label: series.name, color: PALETTE[index % PALETTE.length] }])),
    [props.series],
  );
  // Pie / funnel / radial show one series, so the legend toggles x entries instead.
  const perPoint = props.type === "pie" || props.type === "funnel" || props.type === "radial";
  const legend = React.useMemo(
    () =>
      perPoint
        ? (props.x ?? props.series[0]?.data.map((_, index) => index + 1) ?? []).map((label, index) => ({ id: seriesKey(index), label: String(label), color: PALETTE[index % PALETTE.length] }))
        : props.series.map((series, index) => ({ id: seriesKey(index), label: series.name, color: PALETTE[index % PALETTE.length] })),
    [perPoint, props.series, props.x],
  );
  const toggle = React.useCallback((key: string) => {
    setHidden((current) => {
      const next = new Set(current);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  }, []);

  const rows = React.useMemo(() => {
    const length = Math.max(props.x?.length ?? 0, ...props.series.map((series) => series.data.length));
    return Array.from({ length }, (_, pointIndex) => {
      const row: Record<string, string | number> = { x: props.x?.[pointIndex] ?? pointIndex + 1 };
      props.series.forEach((series, seriesIndex) => {
        const value = series.data[pointIndex];
        if (value !== undefined) {
          row[seriesKey(seriesIndex)] = value;
        }
      });
      return row;
    });
  }, [props.series, props.x]);

  const formatValue = React.useCallback(
    (value: number) => `${NUMBER_FORMAT.format(value)}${props.unit ?? ""}`,
    [props.unit],
  );
  const percentStack = Boolean(props.stacked && props.percent);
  const formatAxis = percentStack ? (value: number) => PERCENT_FORMAT.format(value) : formatValue;

  return (
    <div className="px-4 pb-4 pt-5">
      {props.title ? <h3 className="mb-3 text-lg font-semibold leading-7">{props.title}</h3> : null}
      <ChartContainer id={id} config={config} className="aspect-auto h-64 w-full">
        {props.type === "pie" ? (
          <PieChartBody rows={rows} series={props.series} hidden={hidden} />
        ) : props.type === "funnel" ? (
          <FunnelChartBody rows={rows} series={props.series} hidden={hidden} formatValue={formatValue} />
        ) : props.type === "radial" ? (
          <RadialChartBody rows={rows} series={props.series} hidden={hidden} formatValue={formatValue} />
        ) : props.type === "radar" ? (
          <RadarChart data={rows} outerRadius="75%">
            <PolarGrid />
            <PolarAngleAxis dataKey="x" fontSize={11} />
            <PolarRadiusAxis fontSize={10} tickFormatter={formatValue} />
            <ChartTooltip content={<ChartTooltipContent />} />
            {props.series.map((_, index) =>
              hidden.has(seriesKey(index)) ? null : (
                <Radar key={seriesKey(index)} dataKey={seriesKey(index)} stroke={`var(--color-${seriesKey(index)})`} fill={`var(--color-${seriesKey(index)})`} fillOpacity={0.18} isAnimationActive={false} />
              ),
            )}
          </RadarChart>
        ) : props.type === "scatter" ? (
          <ScatterChart accessibilityLayer margin={CARTESIAN_MARGIN}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="x" type="number" tickLine={false} axisLine={false} fontSize={11} tickMargin={6} domain={["auto", "auto"]} />
            <YAxis dataKey="y" type="number" tickLine={false} axisLine={false} fontSize={11} width="auto" tickMargin={6} tickFormatter={formatValue} domain={["auto", "auto"]} />
            <ChartTooltip content={<ChartTooltipContent />} cursor={{ strokeDasharray: "3 3" }} />
            {props.series.map((series, index) =>
              hidden.has(seriesKey(index)) ? null : (
                <Scatter
                  key={seriesKey(index)}
                  name={series.name}
                  data={rows.filter((row) => row[seriesKey(index)] !== undefined).map((row) => ({ x: Number(row.x), y: row[seriesKey(index)] }))}
                  fill={`var(--color-${seriesKey(index)})`}
                  fillOpacity={0.8}
                  isAnimationActive={false}
                />
              ),
            )}
          </ScatterChart>
        ) : (
          <CartesianBody type={props.type} rows={rows} series={props.series} hidden={hidden} stacked={Boolean(props.stacked)} percent={percentStack} formatAxis={formatAxis} />
        )}
      </ChartContainer>
      {legend.length > 1 ? (
        <div className="mt-2">
          <ChartInteractiveLegend items={legend} hiddenSeries={hidden} onToggle={toggle} />
        </div>
      ) : null}
    </div>
  );
}

type BodyProps = {
  rows: Record<string, string | number>[];
  series: ChartProps["series"];
  hidden: ReadonlySet<string>;
};

// Bars, lines and areas all live in one composed chart so a series can opt
// into a different mark (`type`) or a second axis (`axis`).
function CartesianBody({
  type,
  rows,
  series,
  hidden,
  stacked,
  percent,
  formatAxis,
}: BodyProps & { type: ChartType; stacked: boolean; percent: boolean; formatAxis: (value: number) => string }) {
  const horizontal = type === "horizontal-bar";
  const baseType: SeriesType = type === "line" || type === "area" ? type : "bar";
  const hasRightAxis = series.some((entry) => entry.axis === "right");
  const categoryAxis = horizontal ? (
    <YAxis dataKey="x" type="category" tickLine={false} axisLine={false} fontSize={11} width="auto" tickMargin={6} interval={0} />
  ) : (
    <XAxis dataKey="x" tickLine={false} axisLine={false} fontSize={11} tickMargin={6} interval="preserveStartEnd" minTickGap={16} />
  );
  const valueAxis = horizontal ? (
    <XAxis type="number" tickLine={false} axisLine={false} fontSize={11} tickMargin={6} tickFormatter={formatAxis} />
  ) : (
    <YAxis yAxisId="left" tickLine={false} axisLine={false} fontSize={11} width="auto" tickMargin={6} tickFormatter={formatAxis} />
  );
  return (
    <ComposedChart data={rows} layout={horizontal ? "vertical" : "horizontal"} accessibilityLayer margin={CARTESIAN_MARGIN} stackOffset={percent ? "expand" : "none"}>
      <CartesianGrid vertical={horizontal} horizontal={!horizontal} strokeDasharray="3 3" />
      {categoryAxis}
      {valueAxis}
      {hasRightAxis && !horizontal ? <YAxis yAxisId="right" orientation="right" tickLine={false} axisLine={false} fontSize={11} width="auto" tickMargin={6} tickFormatter={NUMBER_FORMAT.format} /> : null}
      <ChartTooltip content={<ChartTooltipContent />} />
      {series.map((entry, index) => {
        const key = seriesKey(index);
        if (hidden.has(key)) {
          return null;
        }
        const mark = horizontal ? "bar" : (entry.type ?? baseType);
        const axisId = horizontal ? undefined : entry.axis === "right" && hasRightAxis ? "right" : "left";
        const color = `var(--color-${key})`;
        if (mark === "line") {
          return <Line key={key} yAxisId={axisId} type="monotone" dataKey={key} stroke={color} strokeWidth={2} dot={false} isAnimationActive={false} />;
        }
        if (mark === "area") {
          return <Area key={key} yAxisId={axisId} type="monotone" dataKey={key} stackId={stacked ? "stack" : undefined} stroke={color} fill={color} fillOpacity={0.18} strokeWidth={2} isAnimationActive={false} />;
        }
        const radius: [number, number, number, number] | undefined = stacked ? undefined : horizontal ? [0, 3, 3, 0] : [3, 3, 0, 0];
        return <Bar key={key} yAxisId={axisId} dataKey={key} stackId={stacked ? "stack" : undefined} fill={color} radius={radius} isAnimationActive={false} />;
      })}
    </ComposedChart>
  );
}

function firstSeriesSlices(rows: BodyProps["rows"], series: ChartProps["series"], hidden: ReadonlySet<string>) {
  const first = series[0];
  if (!first) {
    return [];
  }
  return rows
    .map((row, index) => ({ name: String(row.x), value: first.data[index] ?? 0, fill: PALETTE[index % PALETTE.length], key: seriesKey(index) }))
    .filter((slice) => !hidden.has(slice.key));
}

function PieChartBody({ rows, series, hidden }: BodyProps) {
  const slices = firstSeriesSlices(rows, series, hidden);
  return (
    <PieChart>
      <ChartTooltip content={<ChartTooltipContent nameKey="name" />} />
      <Pie data={slices} dataKey="value" nameKey="name" innerRadius="45%" outerRadius="80%" isAnimationActive={false}>
        {slices.map((slice) => (
          <Cell key={slice.key} fill={slice.fill} />
        ))}
      </Pie>
    </PieChart>
  );
}

function FunnelChartBody({ rows, series, hidden, formatValue }: BodyProps & { formatValue: (value: number) => string }) {
  const slices = firstSeriesSlices(rows, series, hidden);
  return (
    <FunnelChart>
      <ChartTooltip content={<ChartTooltipContent nameKey="name" />} />
      <Funnel data={slices} dataKey="value" nameKey="name" isAnimationActive={false}>
        <LabelList dataKey="name" position="right" fill="var(--foreground)" fontSize={11} stroke="none" />
        <LabelList dataKey="value" position="left" fill="var(--muted-foreground)" fontSize={11} stroke="none" formatter={(value) => (typeof value === "number" ? formatValue(value) : String(value ?? ""))} />
        {slices.map((slice) => (
          <Cell key={slice.key} fill={slice.fill} />
        ))}
      </Funnel>
    </FunnelChart>
  );
}

// One ring per x, drawn against the largest value (or 100 when values look like
// percentages) so partial rings read as progress.
function RadialChartBody({ rows, series, hidden, formatValue }: BodyProps & { formatValue: (value: number) => string }) {
  const slices = firstSeriesSlices(rows, series, hidden);
  const max = Math.max(100, ...slices.map((slice) => slice.value));
  return (
    <RadialBarChart data={slices} innerRadius="30%" outerRadius="100%" startAngle={90} endAngle={-270}>
      <PolarAngleAxis type="number" domain={[0, max]} tick={false} />
      <ChartTooltip content={<ChartTooltipContent nameKey="name" />} />
      <RadialBar dataKey="value" background={{ fill: "var(--muted)" }} cornerRadius={4} isAnimationActive={false}>
        <LabelList dataKey="value" position="insideStart" fill="var(--background)" fontSize={10} formatter={(value) => (typeof value === "number" ? formatValue(value) : String(value ?? ""))} />
      </RadialBar>
    </RadialBarChart>
  );
}

function ChartSkeleton() {
  return (
    <UIBlockFrame className="p-4">
      <Skeleton className="h-5 w-1/3 rounded-full" />
      <div className="mt-3 flex h-64 items-end gap-2 px-2">
        {Array.from({ length: 8 }).map((_, index) => (
          <Skeleton key={`chart-bar-${index}`} className="flex-1 rounded-t-sm" style={{ height: `${25 + ((index * 23) % 70)}%` }} />
        ))}
      </div>
    </UIBlockFrame>
  );
}

export const chartDefinition: UIBlockDefinition<ChartProps> = {
  name: "chart",
  version: 1,
  schema: s.object(
    {
      title: s.string(),
      type: s.string({ enum: CHART_TYPES }),
      x: s.array(s.cell()),
      series: s.array(
        s.object({ name: s.string(), data: s.array(s.number(), { minItems: 1 }), type: s.string({ enum: SERIES_TYPES }), axis: s.string({ enum: ["left", "right"] }) }, ["name", "data"]),
        { minItems: 1 },
      ),
      unit: s.string(),
      stacked: s.boolean(),
      percent: s.boolean(),
    },
    ["type", "series"],
  ),
  Component: Chart,
  Skeleton: ChartSkeleton,
};
