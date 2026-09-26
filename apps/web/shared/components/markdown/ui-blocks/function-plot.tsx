"use client";

import { CircleAlert, Eye, EyeOff, Home, Minus, Plus, RotateCcw, X } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import katex from "katex";

import { type CompiledExpression, compileExpression, expressionToLatex } from "@/shared/lib/safe-expression";
import type { UIBlockDefinition, UIBlockRenderProps } from "./block";
import { s } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";

type FunctionPlotProps = {
  title?: string;
  functions: { expr: string }[];
  x?: number[];
  y?: number[];
};

type Viewport = { xMin: number; xMax: number; yMin: number; yMax: number };
type FunctionRow = { key: number; expr: string; hidden: boolean };
// y = f(x) and x = g(y) are sampled along one axis; any other equation in x
// and y is traced as the zero set of lhs - rhs with marching squares.
type Curve =
  | { kind: "explicit-y"; fn: CompiledExpression }
  | { kind: "explicit-x"; fn: CompiledExpression }
  | { kind: "implicit"; fn: CompiledExpression };
type RowError = { kind: "variable" | "function"; name: string } | { kind: "invalid" };
type CompiledRow = FunctionRow & { curve: Curve | null; error?: RowError };

const MAX_FUNCTIONS = 8;
const PLOT_HEIGHT = 280;
const DEFAULT_X: readonly [number, number] = [-5, 5];
// Line colours need more contrast than the chart fill palette; each pair reads
// on both themes.
const SERIES_COLORS = [
  { stroke: "stroke-[var(--chart-1)]", dot: "bg-[var(--chart-1)]" },
  { stroke: "stroke-[var(--chart-2)]", dot: "bg-[var(--chart-2)]" },
  { stroke: "stroke-emerald-600 dark:stroke-emerald-400", dot: "bg-emerald-600 dark:bg-emerald-400" },
  { stroke: "stroke-sky-600 dark:stroke-sky-400", dot: "bg-sky-600 dark:bg-sky-400" },
  { stroke: "stroke-amber-600 dark:stroke-amber-400", dot: "bg-amber-600 dark:bg-amber-400" },
  { stroke: "stroke-pink-600 dark:stroke-pink-400", dot: "bg-pink-600 dark:bg-pink-400" },
  { stroke: "stroke-violet-600 dark:stroke-violet-400", dot: "bg-violet-600 dark:bg-violet-400" },
  { stroke: "stroke-teal-600 dark:stroke-teal-400", dot: "bg-teal-600 dark:bg-teal-400" },
] as const;
const EDGE_LABEL_GAP = 10;
const ZOOM_STEP = 1.5;
// Trackpads emit many small deltas; scale them continuously instead of stepping.
const WHEEL_SENSITIVITY = 0.002;
const ZOOM_ANIMATION_MS = 180;
const MIN_SPAN = 1e-4;
const MAX_SPAN = 1e6;

function assertVariables(compiled: CompiledExpression, allowed: readonly string[]): void {
  const foreign = compiled.variables.find((name) => !allowed.includes(name));
  if (foreign) {
    throw new Error(`unknown variable ${foreign}`);
  }
}

// Accepts `expr` (in x), `y = expr`, `f(x) = expr`, `x = expr` (in y) and any
// `lhs = rhs` equation in x and y.
function parseCurve(text: string): Curve {
  const parts = text.split("=");
  if (parts.length > 2) {
    throw new Error("only one = allowed");
  }
  if (parts.length === 1) {
    const fn = compileExpression(text);
    assertVariables(fn, ["x"]);
    return { kind: "explicit-y", fn };
  }
  const lhs = (parts[0] ?? "").trim();
  const rhs = (parts[1] ?? "").trim();
  if (lhs === "" || rhs === "") {
    throw new Error("incomplete equation");
  }
  if (lhs === "y" || /^[a-zA-Z]\(x\)$/.test(lhs)) {
    const fn = compileExpression(rhs);
    assertVariables(fn, ["x"]);
    return { kind: "explicit-y", fn };
  }
  if (lhs === "x") {
    const fn = compileExpression(rhs);
    assertVariables(fn, ["y"]);
    return { kind: "explicit-x", fn };
  }
  const fn = compileExpression(`(${lhs}) - (${rhs})`);
  assertVariables(fn, ["x", "y"]);
  return { kind: "implicit", fn };
}

function rowToLatex(text: string): string | null {
  try {
    return text
      .split("=")
      .map((side) => expressionToLatex(side.trim()))
      .join(" = ");
  } catch {
    return null;
  }
}

function MathDisplay({ latex, className }: { latex: string; className?: string }) {
  const html = React.useMemo(() => katex.renderToString(latex, { throwOnError: false, output: "html" }), [latex]);
  // KaTeX output is generated from our own AST, never from raw model text.
  // biome-ignore lint/security/noDangerouslySetInnerHtml: KaTeX markup built from a parsed AST
  return <span className={className} dangerouslySetInnerHTML={{ __html: html }} />;
}

function classifyError(error: unknown): RowError {
  const message = error instanceof Error ? error.message : String(error);
  const variable = /^unknown variable (\S+)/.exec(message);
  if (variable?.[1]) {
    return { kind: "variable", name: variable[1] };
  }
  const fn = /^unknown function (\S+)/.exec(message);
  if (fn?.[1]) {
    return { kind: "function", name: fn[1] };
  }
  return { kind: "invalid" };
}

function compileRow(row: FunctionRow): CompiledRow {
  if (row.expr.trim() === "") {
    return { ...row, curve: null };
  }
  try {
    return { ...row, curve: parseCurve(row.expr) };
  } catch (error) {
    return { ...row, curve: null, error: classifyError(error) };
  }
}

// Row keys only grow so a row keeps its colour when others are removed.
function rowsFromProps(functions: FunctionPlotProps["functions"]): FunctionRow[] {
  return functions.slice(0, MAX_FUNCTIONS).map((fn, index) => ({ key: index, expr: fn.expr, hidden: false }));
}

function rangeOr(range: number[] | undefined, fallback: readonly [number, number]): [number, number] {
  if (range && range.length === 2 && Number.isFinite(range[0]) && Number.isFinite(range[1]) && range[1] > range[0]) {
    return [range[0], range[1]];
  }
  return [fallback[0], fallback[1]];
}

// Initial viewport keeps 1 unit on x the same length as 1 unit on y unless the
// author pinned a y range, so a circle looks like a circle.
function initialViewport(props: FunctionPlotProps, width: number): Viewport {
  const [xMin, xMax] = rangeOr(props.x, DEFAULT_X);
  if (props.y) {
    const [yMin, yMax] = rangeOr(props.y, DEFAULT_X);
    return { xMin, xMax, yMin, yMax };
  }
  const half = ((xMax - xMin) * PLOT_HEIGHT) / Math.max(width, 1) / 2;
  return { xMin, xMax, yMin: -half, yMax: half };
}

// 1 / 2 / 5 × 10^n step that yields roughly `target` grid lines across `span`.
function niceStep(span: number, target: number): number {
  const rough = span / target;
  const magnitude = 10 ** Math.floor(Math.log10(rough));
  const residual = rough / magnitude;
  const factor = residual >= 5 ? 5 : residual >= 2 ? 2 : 1;
  return factor * magnitude;
}

function ticks(min: number, max: number, step: number): number[] {
  const out: number[] = [];
  const start = Math.ceil(min / step) * step;
  for (let value = start; value <= max + step / 2; value += step) {
    out.push(Math.abs(value) < step / 1e6 ? 0 : value);
  }
  return out;
}

function formatTick(value: number, step: number): string {
  const digits = Math.max(0, -Math.floor(Math.log10(step)));
  return value.toFixed(Math.min(digits, 6)).replace(/\.?0+$/, "");
}

function safeEvaluate(fn: CompiledExpression, variables: Record<string, number>): number {
  try {
    return fn.evaluate(variables);
  } catch {
    return Number.NaN;
  }
}

// Samples one value per pixel along the independent axis and splits the path
// wherever the value is non-finite or jumps by more than twice the visible
// span (poles, tan, 1/x).
function buildExplicitPath(fn: CompiledExpression, view: Viewport, width: number, height: number, alongX: boolean): string {
  const parts: string[] = [];
  const steps = alongX ? width : height;
  const inMin = alongX ? view.xMin : view.yMin;
  const inSpan = alongX ? view.xMax - view.xMin : view.yMax - view.yMin;
  const outMin = alongX ? view.yMin : view.xMin;
  const outSpan = alongX ? view.yMax - view.yMin : view.xMax - view.xMin;
  const outPixels = alongX ? height : width;
  const jumpLimit = outSpan * 2;
  let open = false;
  let previous: number | null = null;
  for (let step = 0; step <= steps; step += 1) {
    const input = inMin + (step / steps) * inSpan;
    const value = safeEvaluate(fn, alongX ? { x: input } : { y: input });
    if (!Number.isFinite(value) || (previous !== null && Math.abs(value - previous) > jumpLimit)) {
      open = false;
      previous = Number.isFinite(value) ? value : null;
      continue;
    }
    // Clamp far-off-screen points so the SVG path stays numerically sane.
    const clamped = Math.max(outMin - jumpLimit, Math.min(outMin + outSpan + jumpLimit, value));
    const outPx = ((clamped - outMin) / outSpan) * outPixels;
    const px = alongX ? step : outPx;
    const py = alongX ? height - outPx : height - step;
    parts.push(`${open ? "L" : "M"}${px.toFixed(1)} ${py.toFixed(1)}`);
    open = true;
    previous = value;
  }
  return parts.join("");
}

const IMPLICIT_CELL = 4;

// Marching squares over a pixel grid: every cell whose corners change sign
// contributes the linearly interpolated crossing segment(s).
function buildImplicitPath(fn: CompiledExpression, view: Viewport, width: number, height: number): string {
  const cols = Math.ceil(width / IMPLICIT_CELL);
  const rows = Math.ceil(height / IMPLICIT_CELL);
  const values = new Float64Array((cols + 1) * (rows + 1));
  for (let row = 0; row <= rows; row += 1) {
    const y = view.yMax - (row / rows) * (view.yMax - view.yMin);
    for (let col = 0; col <= cols; col += 1) {
      const x = view.xMin + (col / cols) * (view.xMax - view.xMin);
      values[row * (cols + 1) + col] = safeEvaluate(fn, { x, y });
    }
  }
  const parts: string[] = [];
  const cellW = width / cols;
  const cellH = height / rows;
  const crossing = (a: number, b: number) => (a === b ? 0.5 : a / (a - b));
  for (let row = 0; row < rows; row += 1) {
    for (let col = 0; col < cols; col += 1) {
      const v0 = values[row * (cols + 1) + col] as number;
      const v1 = values[row * (cols + 1) + col + 1] as number;
      const v2 = values[(row + 1) * (cols + 1) + col + 1] as number;
      const v3 = values[(row + 1) * (cols + 1) + col] as number;
      if (!Number.isFinite(v0) || !Number.isFinite(v1) || !Number.isFinite(v2) || !Number.isFinite(v3)) {
        continue;
      }
      const x0 = col * cellW;
      const y0 = row * cellH;
      const points: [number, number][] = [];
      if (v0 < 0 !== v1 < 0) points.push([x0 + crossing(v0, v1) * cellW, y0]);
      if (v1 < 0 !== v2 < 0) points.push([x0 + cellW, y0 + crossing(v1, v2) * cellH]);
      if (v3 < 0 !== v2 < 0) points.push([x0 + crossing(v3, v2) * cellW, y0 + cellH]);
      if (v0 < 0 !== v3 < 0) points.push([x0, y0 + crossing(v0, v3) * cellH]);
      for (let index = 0; index + 1 < points.length; index += 2) {
        const [ax, ay] = points[index] as [number, number];
        const [bx, by] = points[index + 1] as [number, number];
        parts.push(`M${ax.toFixed(1)} ${ay.toFixed(1)}L${bx.toFixed(1)} ${by.toFixed(1)}`);
      }
    }
  }
  return parts.join("");
}

function buildPath(curve: Curve, view: Viewport, width: number, height: number): string {
  switch (curve.kind) {
    case "explicit-y":
      return buildExplicitPath(curve.fn, view, width, height, true);
    case "explicit-x":
      return buildExplicitPath(curve.fn, view, width, height, false);
    case "implicit":
      return buildImplicitPath(curve.fn, view, width, height);
  }
}

// Scales the viewport by `factor` around `anchor` (defaults to the centre).
function zoomed(base: Viewport, factor: number, anchor?: { x: number; y: number }): Viewport {
  const cx = anchor?.x ?? (base.xMin + base.xMax) / 2;
  const cy = anchor?.y ?? (base.yMin + base.yMax) / 2;
  const xSpan = (base.xMax - base.xMin) * factor;
  const ySpan = (base.yMax - base.yMin) * factor;
  if (xSpan < MIN_SPAN || xSpan > MAX_SPAN) {
    return base;
  }
  const fx = (cx - base.xMin) / (base.xMax - base.xMin);
  const fy = (cy - base.yMin) / (base.yMax - base.yMin);
  return { xMin: cx - xSpan * fx, xMax: cx + xSpan * (1 - fx), yMin: cy - ySpan * fy, yMax: cy + ySpan * (1 - fy) };
}

function FunctionPlot({ id, props }: UIBlockRenderProps<FunctionPlotProps>) {
  const t = useTranslations("chat.markdown.uiBlock");
  const containerRef = React.useRef<HTMLDivElement>(null);
  const [width, setWidth] = React.useState(0);
  const [rows, setRows] = React.useState<FunctionRow[]>(() => rowsFromProps(props.functions));
  const nextKeyRef = React.useRef(rows.length);
  const compiledRows = React.useMemo(() => rows.map(compileRow), [rows]);
  const [view, setView] = React.useState<Viewport | null>(null);
  const dragRef = React.useRef<{ x: number; y: number; view: Viewport } | null>(null);
  const inputRefs = React.useRef(new Map<number, HTMLInputElement>());
  const [pendingFocus, setPendingFocus] = React.useState<number | null>(null);
  const [editingKey, setEditingKey] = React.useState<number | null>(null);
  const rowErrorText = (error: RowError) =>
    error.kind === "variable" ? t("exprUnknownVariable", { name: error.name }) : error.kind === "function" ? t("exprUnknownFunction", { name: error.name }) : t("exprInvalid");

  React.useLayoutEffect(() => {
    const element = containerRef.current;
    if (!element) {
      return;
    }
    const measure = () => setWidth(element.clientWidth);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  React.useEffect(() => {
    if (pendingFocus === null) {
      return;
    }
    setEditingKey(pendingFocus);
    inputRefs.current.get(pendingFocus)?.focus();
    setPendingFocus(null);
  }, [pendingFocus]);

  const defaultView = React.useMemo(() => initialViewport(props, width || 600), [props, width]);
  const current = view ?? defaultView;
  const height = PLOT_HEIGHT;

  const updateRow = (key: number, patch: Partial<FunctionRow>) => setRows((list) => list.map((row) => (row.key === key ? { ...row, ...patch } : row)));
  const removeRow = (key: number) => setRows((list) => list.filter((row) => row.key !== key));
  const addRow = () => {
    if (rows.length >= MAX_FUNCTIONS) {
      return;
    }
    const key = nextKeyRef.current++;
    setRows((list) => [...list, { key, expr: "", hidden: false }]);
    setPendingFocus(key);
  };
  const reset = () => {
    stopAnimation();
    setRows(rowsFromProps(props.functions));
    nextKeyRef.current = Math.min(props.functions.length, MAX_FUNCTIONS);
    setView(null);
  };
  const dirty = view !== null || rows.length !== Math.min(props.functions.length, MAX_FUNCTIONS) || rows.some((row, index) => row.hidden || row.expr !== props.functions[index]?.expr);

  // Latest viewport for event handlers registered once (wheel, animation frames).
  const viewRef = React.useRef(current);
  viewRef.current = current;
  const animationRef = React.useRef<number | null>(null);

  const stopAnimation = React.useCallback(() => {
    if (animationRef.current !== null) {
      cancelAnimationFrame(animationRef.current);
      animationRef.current = null;
    }
  }, []);
  React.useEffect(() => stopAnimation, [stopAnimation]);

  // Eases the viewport to `target`; `settle` runs when it arrives (used to
  // collapse "back at default" into view === null so reset state stays exact).
  const animateTo = React.useCallback(
    (target: Viewport, settle?: () => void) => {
      stopAnimation();
      const from = viewRef.current;
      const startedAt = performance.now();
      const frame = (now: number) => {
        const progress = Math.min(1, (now - startedAt) / ZOOM_ANIMATION_MS);
        const eased = 1 - (1 - progress) ** 3;
        setView({
          xMin: from.xMin + (target.xMin - from.xMin) * eased,
          xMax: from.xMax + (target.xMax - from.xMax) * eased,
          yMin: from.yMin + (target.yMin - from.yMin) * eased,
          yMax: from.yMax + (target.yMax - from.yMax) * eased,
        });
        if (progress < 1) {
          animationRef.current = requestAnimationFrame(frame);
        } else {
          animationRef.current = null;
          settle?.();
        }
      };
      animationRef.current = requestAnimationFrame(frame);
    },
    [stopAnimation],
  );

  const zoomBy = (factor: number) => animateTo(zoomed(viewRef.current, factor));
  const resetView = () => animateTo(defaultView, () => setView(null));

  // Wheel zoom must be a non-passive listener to stop the page from scrolling.
  React.useEffect(() => {
    const element = containerRef.current;
    if (!element) {
      return;
    }
    const onWheel = (event: WheelEvent) => {
      event.preventDefault();
      stopAnimation();
      const rect = element.getBoundingClientRect();
      const base = viewRef.current;
      const anchor = {
        x: base.xMin + ((event.clientX - rect.left) / rect.width) * (base.xMax - base.xMin),
        y: base.yMax - ((event.clientY - rect.top) / rect.height) * (base.yMax - base.yMin),
      };
      const factor = Math.min(ZOOM_STEP, Math.max(1 / ZOOM_STEP, Math.exp(event.deltaY * WHEEL_SENSITIVITY)));
      setView(zoomed(base, factor, anchor));
    };
    element.addEventListener("wheel", onWheel, { passive: false });
    return () => element.removeEventListener("wheel", onWheel);
  }, [stopAnimation]);

  const onPointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    // Pointer capture would retarget the click away from the zoom controls.
    if (event.button !== 0 || (event.target as HTMLElement).closest("button")) {
      return;
    }
    stopAnimation();
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = { x: event.clientX, y: event.clientY, view: current };
  };
  const onPointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    const rect = containerRef.current?.getBoundingClientRect();
    if (!drag || !rect) {
      return;
    }
    if (event.clientX === drag.x && event.clientY === drag.y) {
      return;
    }
    const dx = ((event.clientX - drag.x) / rect.width) * (drag.view.xMax - drag.view.xMin);
    const dy = ((event.clientY - drag.y) / rect.height) * (drag.view.yMax - drag.view.yMin);
    setView({ xMin: drag.view.xMin - dx, xMax: drag.view.xMax - dx, yMin: drag.view.yMin + dy, yMax: drag.view.yMax + dy });
  };
  const onPointerUp = (event: React.PointerEvent<HTMLDivElement>) => {
    if (!dragRef.current) {
      return;
    }
    dragRef.current = null;
    event.currentTarget.releasePointerCapture(event.pointerId);
  };

  const xStep = niceStep(current.xMax - current.xMin, 10);
  const yStep = niceStep(current.yMax - current.yMin, 6);
  const xTicks = ticks(current.xMin, current.xMax, xStep);
  const yTicks = ticks(current.yMin, current.yMax, yStep);
  const toPx = (x: number) => ((x - current.xMin) / (current.xMax - current.xMin)) * width;
  const toPy = (y: number) => ((current.yMax - y) / (current.yMax - current.yMin)) * height;
  const axisX = Math.min(Math.max(toPy(0), 0), height);
  const axisY = Math.min(Math.max(toPx(0), 0), width);

  return (
    <div className="px-4 pb-4 pt-5">
      <div className="flex items-baseline justify-between gap-3">
        {props.title ? <h3 className="text-lg font-semibold leading-7">{props.title}</h3> : <span />}
        {dirty ? (
          <button type="button" onClick={reset} className="inline-flex shrink-0 items-center gap-1 text-xs text-muted-foreground hover:text-foreground">
            <RotateCcw className="size-3" strokeWidth={1.8} />
            {t("reset")}
          </button>
        ) : null}
      </div>

      <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
        {/* The list never grows past the plot: 8 rows fit exactly, anything taller scrolls. */}
        <div className="min-w-0 md:max-h-(--plot-height) md:overflow-y-auto" style={{ "--plot-height": `${height}px` } as React.CSSProperties}>
          <ul className="[&>li+li]:shadow-[inset_0_0.5px_0_var(--border)]">
            {compiledRows.map((row) => {
              const color = SERIES_COLORS[row.key % SERIES_COLORS.length];
              const latex = row.error || editingKey === row.key ? null : rowToLatex(row.expr);
              return (
                <li key={`${id}-fn-${row.key}`} className="px-2.5">
                  <div className="flex h-[35px] items-center gap-1.5">
                    <span className={cn("mr-0.5 size-2.5 shrink-0 rounded-full", color.dot, row.hidden && "opacity-35")} aria-hidden="true" />
                    {latex ? (
                      <button
                        type="button"
                        onClick={() => {
                          setEditingKey(row.key);
                          setPendingFocus(row.key);
                        }}
                        className={cn("flex h-[27px] min-w-0 flex-1 items-center overflow-x-auto text-left text-sm [&_.katex]:text-[1em]", row.hidden && "text-muted-foreground")}
                      >
                        <MathDisplay latex={latex} />
                      </button>
                    ) : null}
                    <input
                      ref={(element) => {
                        if (element) {
                          inputRefs.current.set(row.key, element);
                        } else {
                          inputRefs.current.delete(row.key);
                        }
                      }}
                      value={row.expr}
                      onChange={(event) => updateRow(row.key, { expr: event.target.value })}
                      onFocus={() => setEditingKey(row.key)}
                      onBlur={() => setEditingKey((key) => (key === row.key ? null : key))}
                      onKeyDown={(event) => {
                        if (event.key === "Enter" && !event.nativeEvent.isComposing) {
                          event.preventDefault();
                          addRow();
                        }
                        if (event.key === "Escape") {
                          event.currentTarget.blur();
                        }
                      }}
                      placeholder="y = sin(x)"
                      spellCheck={false}
                      autoComplete="off"
                      aria-label={t("functionExpr")}
                      aria-invalid={row.error ? true : undefined}
                      className={cn(
                        "h-[27px] min-w-0 flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground/50",
                        row.hidden && "text-muted-foreground",
                        latex && "sr-only",
                      )}
                    />
                    <div className="flex shrink-0 items-center">
                      {row.error ? (
                        <span
                          role="img"
                          title={rowErrorText(row.error)}
                          aria-label={rowErrorText(row.error)}
                          className="flex size-6 items-center justify-center text-destructive"
                        >
                          <CircleAlert className="size-3.5" strokeWidth={1.8} />
                        </span>
                      ) : null}
                      <button
                        type="button"
                        onClick={() => updateRow(row.key, { hidden: !row.hidden })}
                        aria-pressed={!row.hidden}
                        aria-label={t("toggleVisibility")}
                        className="flex size-6 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                      >
                        {row.hidden ? <EyeOff className="size-3.5" strokeWidth={1.8} /> : <Eye className="size-3.5" strokeWidth={1.8} />}
                      </button>
                      <button
                        type="button"
                        onClick={() => removeRow(row.key)}
                        aria-label={t("removeFunction")}
                        className="flex size-6 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                      >
                        <X className="size-3.5" strokeWidth={1.8} />
                      </button>
                    </div>
                  </div>
                </li>
              );
            })}
            {rows.length < MAX_FUNCTIONS ? (
              <li>
                <button type="button" onClick={addRow} className="flex h-[35px] w-full items-center gap-2 px-2.5 text-xs text-muted-foreground hover:text-foreground">
                  <Plus className="size-3.5" strokeWidth={1.8} />
                  {t("addFunction")}
                </button>
              </li>
            ) : null}
          </ul>
        </div>

        <div
          ref={containerRef}
          className="relative cursor-grab select-none overflow-hidden rounded-lg border-[0.5px] border-border bg-background touch-none active:cursor-grabbing"
          style={{ height }}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={onPointerUp}
          role="img"
          aria-label={props.title ?? rows.map((row) => row.expr).join(", ")}
        >
          {width > 0 ? (
            <svg width={width} height={height} className="block">
              <g className="stroke-border" strokeWidth={0.5}>
                {xTicks.map((x) => (
                  <line key={`gx-${x}`} x1={toPx(x)} x2={toPx(x)} y1={0} y2={height} />
                ))}
                {yTicks.map((y) => (
                  <line key={`gy-${y}`} x1={0} x2={width} y1={toPy(y)} y2={toPy(y)} />
                ))}
              </g>
              <g className="stroke-foreground/45" strokeWidth={1}>
                <line x1={0} x2={width} y1={axisX} y2={axisX} />
                <line x1={axisY} x2={axisY} y1={0} y2={height} />
              </g>
              <g className="fill-muted-foreground" fontSize={10}>
                {xTicks
                  .filter((x) => x !== 0 && toPx(x) > EDGE_LABEL_GAP && toPx(x) < width - EDGE_LABEL_GAP)
                  .map((x) => (
                    <text key={`tx-${x}`} x={toPx(x)} y={Math.min(axisX + 12, height - 3)} textAnchor="middle">
                      {formatTick(x, xStep)}
                    </text>
                  ))}
                {yTicks
                  .filter((y) => y !== 0 && toPy(y) > EDGE_LABEL_GAP && toPy(y) < height - EDGE_LABEL_GAP)
                  .map((y) => (
                    <text key={`ty-${y}`} x={Math.max(axisY - 4, 22)} y={toPy(y) + 3} textAnchor="end">
                      {formatTick(y, yStep)}
                    </text>
                  ))}
              </g>
              <g fill="none" strokeWidth={1.75} strokeLinejoin="round" strokeLinecap="round">
                {compiledRows.map((row) =>
                  row.curve && !row.hidden ? (
                    <path key={`${id}-path-${row.key}`} d={buildPath(row.curve, current, width, height)} className={SERIES_COLORS[row.key % SERIES_COLORS.length].stroke} />
                  ) : null,
                )}
              </g>
            </svg>
          ) : null}

          <div className="absolute right-2 bottom-2 flex overflow-hidden rounded-md border-[0.5px] border-border bg-card text-muted-foreground">
            <button type="button" aria-label={t("zoomIn")} onClick={() => zoomBy(1 / ZOOM_STEP)} className="flex size-7 items-center justify-center hover:bg-accent hover:text-foreground">
              <Plus className="size-3.5" strokeWidth={1.8} />
            </button>
            <button type="button" aria-label={t("zoomOut")} onClick={() => zoomBy(ZOOM_STEP)} className="flex size-7 items-center justify-center border-l-[0.5px] border-border hover:bg-accent hover:text-foreground">
              <Minus className="size-3.5" strokeWidth={1.8} />
            </button>
            <button type="button" aria-label={t("resetView")} onClick={resetView} disabled={view === null} className="flex size-7 items-center justify-center border-l-[0.5px] border-border hover:bg-accent hover:text-foreground disabled:opacity-40 disabled:hover:bg-transparent">
              <Home className="size-3.5" strokeWidth={1.8} />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

function FunctionPlotSkeleton() {
  return (
    <UIBlockFrame className="p-4">
      <Skeleton className="h-5 w-1/3 rounded-full" />
      <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
        <Skeleton className="h-24 rounded-lg" />
        <Skeleton className="rounded-lg" style={{ height: PLOT_HEIGHT }} />
      </div>
    </UIBlockFrame>
  );
}

export const functionPlotDefinition: UIBlockDefinition<FunctionPlotProps> = {
  name: "function-plot",
  version: 1,
  schema: s.object(
    {
      title: s.string(),
      functions: s.array(s.object({ expr: s.string() }, ["expr"]), { minItems: 1 }),
      x: s.array(s.number()),
      y: s.array(s.number()),
    },
    ["functions"],
  ),
  Component: FunctionPlot,
  Skeleton: FunctionPlotSkeleton,
};
