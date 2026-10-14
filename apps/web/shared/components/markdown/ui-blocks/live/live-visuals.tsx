"use client";

import type { ResolvedNode } from "@deeix/core";
import { formulaToLatex } from "@deeix/core";
import katex from "katex";
import * as React from "react";

import { cn } from "@/lib/utils";
import { isRecord } from "@/shared/lib/type-guards";
import { LIVE_TONES, type LiveTone, readNumber, readText, useLiveBlock } from "./live-context";

// Two primitives for explaining a relationship rather than listing data:
// `formula` writes a formula out with the current values, and `scene` draws a
// small diagram whose geometry follows the state. Neither takes markup from the
// model: the formula is LaTeX built from the parsed expression, the scene is a
// fixed set of SVG shapes.

// ---- formula ----

// `F_g = m g` large, then the worked line `= (9)(9.8) = 88.2 N` that follows
// every change of the inputs.
export function LiveFormula({ node }: { node: ResolvedNode }) {
  const { values } = useLiveBlock();
  const expr = readText(node.props, "expr");
  const latex = React.useMemo(() => {
    if (!expr) {
      return null;
    }
    try {
      return formulaToLatex(expr, {
        name: readText(node.props, "name"),
        unit: readText(node.props, "unit"),
        precision: readNumber(node.props, "precision"),
        values,
        with: node.props.with,
      });
    } catch {
      return null;
    }
  }, [expr, node.props, values]);
  if (!latex) {
    return null;
  }
  const label = readText(node.props, "label");
  return (
    <div className="min-w-0 space-y-1.5 py-1 text-center">
      {label ? <p className="text-xs text-muted-foreground">{label}</p> : null}
      <KatexLine latex={latex.symbolic} className="block overflow-x-auto text-xl" />
      {latex.worked ? <KatexLine latex={latex.worked} className="block overflow-x-auto text-sm text-muted-foreground" /> : null}
    </div>
  );
}

function KatexLine({ latex, className }: { latex: string; className?: string }) {
  const html = React.useMemo(() => katex.renderToString(latex, { throwOnError: false, output: "html", strict: "ignore" }), [latex]);
  // The LaTeX comes from formulaToLatex: built from a parsed AST, with names
  // restricted to identifiers and units escaped.
  // biome-ignore lint/security/noDangerouslySetInnerHtml: KaTeX markup built from a parsed AST
  return <span className={cn("[&_.katex]:text-[1em]", className)} dangerouslySetInnerHTML={{ __html: html }} />;
}

// ---- scene ----

const MAX_SHAPES = 60;
const MAX_SIZE = 1000;
const LABEL_MAX_LENGTH = 40;

// Strokes and fills per tone; only semantic colours, the model never picks one.
// Kept soft: a diagram sits next to text and controls and should not shout.
const TONE_STROKE: Record<LiveTone, string> = {
  default: "stroke-foreground/55",
  muted: "stroke-muted-foreground/40",
  positive: "stroke-emerald-500/85 dark:stroke-emerald-400/85",
  negative: "stroke-rose-500/85 dark:stroke-rose-400/85",
  warning: "stroke-amber-500/85 dark:stroke-amber-400/85",
  info: "stroke-sky-500/85 dark:stroke-sky-400/85",
};
const TONE_FILL: Record<LiveTone, string> = {
  default: "fill-foreground/[0.04]",
  muted: "fill-muted/60",
  positive: "fill-emerald-500/10",
  negative: "fill-rose-500/10",
  warning: "fill-amber-500/10",
  info: "fill-sky-500/10",
};
const TONE_TEXT: Record<LiveTone, string> = {
  default: "fill-foreground/80",
  muted: "fill-muted-foreground",
  positive: "fill-emerald-700 dark:fill-emerald-300",
  negative: "fill-rose-700 dark:fill-rose-300",
  warning: "fill-amber-700 dark:fill-amber-300",
  info: "fill-sky-700 dark:fill-sky-300",
};
const TONE_MARKER: Record<LiveTone, string> = {
  default: "fill-foreground/55",
  muted: "fill-muted-foreground/40",
  positive: "fill-emerald-500/85 dark:fill-emerald-400/85",
  negative: "fill-rose-500/85 dark:fill-rose-400/85",
  warning: "fill-amber-500/85 dark:fill-amber-400/85",
  info: "fill-sky-500/85 dark:fill-sky-400/85",
};

const LABEL_SIZE = 12;
const LABEL_GAP = 8;

type Shape = Record<string, unknown>;

// A diagram in its own coordinates (`width` × `height`, origin top left), drawn
// at that size and only ever scaled down: stretched to a wide column, labels and
// strokes would grow with it. Shapes: rect {x, y, w, h}, circle {cx, cy, r},
// line and arrow {x1, y1, x2, y2}, text {x, y, text}; each may have `label` and
// `tone`. A shape with a missing or non-finite coordinate is skipped, not drawn at 0.
export function LiveScene({ node }: { node: ResolvedNode }) {
  const markerID = `scene-arrow-${React.useId().replace(/[^a-zA-Z0-9_-]/g, "")}`;
  const width = clampSize(readNumber(node.props, "width") ?? 400);
  const height = clampSize(readNumber(node.props, "height") ?? 240);
  const shapes = Array.isArray(node.props.shapes) ? node.props.shapes.filter(isRecord).slice(0, MAX_SHAPES) : [];
  const usedTones = new Set(shapes.filter((shape) => shape.type === "arrow").map(toneOf));
  return (
    <div className="flex min-w-0 items-center justify-center">
      <svg
        width={width}
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        className="h-auto max-w-full overflow-visible"
        role="img"
        aria-label={readText(node.props, "label")}
      >
        <defs>
          {Array.from(usedTones, (tone) => (
            // In user units, so the head does not grow with the stroke.
            <marker
              key={tone}
              id={`${markerID}-${tone}`}
              viewBox="0 0 10 10"
              refX="9"
              refY="5"
              markerWidth="9"
              markerHeight="9"
              markerUnits="userSpaceOnUse"
              orient="auto-start-reverse"
            >
              <path d="M 0 1 L 10 5 L 0 9 z" className={TONE_MARKER[tone]} />
            </marker>
          ))}
        </defs>
        {shapes.map((shape, index) => (
          <SceneShape key={`${node.path}-${index}`} shape={shape} markerID={markerID} />
        ))}
      </svg>
    </div>
  );
}

function SceneShape({ shape, markerID }: { shape: Shape; markerID: string }) {
  const tone = toneOf(shape);
  const label = shapeLabel(shape);
  const n = (key: string) => finite(shape[key]);
  const labelClass = TONE_TEXT[tone];
  switch (shape.type) {
    case "rect": {
      const [x, y, w, h] = [n("x"), n("y"), n("w") ?? n("width"), n("h") ?? n("height")];
      if (x === undefined || y === undefined || w === undefined || h === undefined) {
        return null;
      }
      const left = Math.min(x, x + w);
      const top = Math.min(y, y + h);
      return (
        <g>
          <rect x={left} y={top} width={Math.abs(w)} height={Math.abs(h)} rx={8} className={cn(TONE_FILL[tone], TONE_STROKE[tone])} strokeWidth={1.25} />
          {label ? (
            <text x={left + Math.abs(w) / 2} y={top + Math.abs(h) / 2} textAnchor="middle" dominantBaseline="central" fontSize={LABEL_SIZE} className={labelClass}>
              {label}
            </text>
          ) : null}
        </g>
      );
    }
    case "circle": {
      const [cx, cy, r] = [n("cx") ?? n("x"), n("cy") ?? n("y"), n("r")];
      if (cx === undefined || cy === undefined || r === undefined) {
        return null;
      }
      return (
        <g>
          <circle cx={cx} cy={cy} r={Math.abs(r)} className={cn(TONE_FILL[tone], TONE_STROKE[tone])} strokeWidth={1.25} />
          {label ? (
            <text x={cx} y={cy} textAnchor="middle" dominantBaseline="central" fontSize={LABEL_SIZE} className={labelClass}>
              {label}
            </text>
          ) : null}
        </g>
      );
    }
    case "line":
    case "arrow": {
      const [x1, y1, x2, y2] = [n("x1"), n("y1"), n("x2"), n("y2")];
      if (x1 === undefined || y1 === undefined || x2 === undefined || y2 === undefined) {
        return null;
      }
      const arrow = shape.type === "arrow";
      const placement = lineLabelPlacement(x1, y1, x2, y2);
      return (
        <g>
          <line
            x1={x1}
            y1={y1}
            x2={x2}
            y2={y2}
            className={TONE_STROKE[tone]}
            strokeWidth={arrow ? 2 : 1.25}
            strokeLinecap="round"
            markerEnd={arrow ? `url(#${markerID}-${tone})` : undefined}
          />
          {label ? (
            <text x={placement.x} y={placement.y} textAnchor={placement.anchor} dominantBaseline={placement.baseline} fontSize={LABEL_SIZE} className={labelClass}>
              {label}
            </text>
          ) : null}
        </g>
      );
    }
    case "text": {
      const [x, y] = [n("x"), n("y")];
      const text = label || shapeText(shape.text);
      if (x === undefined || y === undefined || !text) {
        return null;
      }
      return (
        <text x={x} y={y} textAnchor="middle" dominantBaseline="central" fontSize={LABEL_SIZE} className={labelClass}>
          {text}
        </text>
      );
    }
    default:
      return null;
  }
}

// A line's label sits beside its midpoint, off the line: to the right of a
// vertical line, below a horizontal one, never on the stroke or the arrowhead.
function lineLabelPlacement(x1: number, y1: number, x2: number, y2: number) {
  const length = Math.hypot(x2 - x1, y2 - y1) || 1;
  let nx = -(y2 - y1) / length;
  let ny = (x2 - x1) / length;
  if (nx < -0.01 || (Math.abs(nx) <= 0.01 && ny < 0)) {
    nx = -nx;
    ny = -ny;
  }
  return {
    x: (x1 + x2) / 2 + nx * LABEL_GAP,
    y: (y1 + y2) / 2 + ny * LABEL_GAP,
    anchor: nx > 0.3 ? "start" : nx < -0.3 ? "end" : "middle",
    baseline: ny > 0.3 ? "hanging" : ny < -0.3 ? "auto" : "central",
  } as const;
}

function toneOf(shape: Shape): LiveTone {
  return LIVE_TONES.find((tone) => tone === shape.tone) ?? "default";
}

function shapeLabel(shape: Shape): string {
  return shapeText(shape.label);
}

function shapeText(value: unknown): string {
  if (typeof value === "number") {
    return Number.isFinite(value) ? String(Number(value.toFixed(2))) : "";
  }
  return typeof value === "string" ? value.slice(0, LABEL_MAX_LENGTH) : "";
}

// Models sometimes quote coordinates ("120"); a numeric string is accepted.
function finite(value: unknown): number | undefined {
  const number = typeof value === "string" && value.trim() !== "" ? Number(value) : value;
  return typeof number === "number" && Number.isFinite(number) ? number : undefined;
}

function clampSize(value: number): number {
  return Math.min(MAX_SIZE, Math.max(40, value));
}
