"use client";

import type { ResolvedNode, UIValue } from "@deeix/core";
import { ChevronDown, CircleAlert, Info, RotateCcw, TriangleAlert } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { cn } from "@/lib/utils";
import { CollapsibleMotionContent } from "@/shared/components/collapsible-motion-content";
import {
  ChipButton,
  displayValue,
  LIVE_TONES,
  LiveChildren,
  type LiveTone,
  optionIndex,
  readBoolean,
  readEnum,
  readNumber,
  readOptions,
  readText,
  TONE_TEXT_CLASS,
  useLiveBlock,
} from "./live-context";
import { LiveButton, LiveInput, LiveSegmented, LiveSelect, LiveSlider, LiveSwitch } from "./live-controls";
import { LiveFormula, LiveScene } from "./live-visuals";

// Renders one primitive. Unknown props are ignored; missing ones fall back to
// a sensible default, so a model slip changes the look, never breaks the block.
export function LivePrimitive({ node }: { node: ResolvedNode }) {
  switch (node.component) {
    case "panel":
      return <LivePanel node={node} />;
    case "stack":
      return (
        <div className="grid min-w-0 gap-3">
          <LiveChildren node={node} />
        </div>
      );
    case "row":
      return (
        <div className="flex min-w-0 flex-wrap items-stretch gap-3 [&>*]:min-w-[8rem] [&>*]:flex-1">
          <LiveChildren node={node} />
        </div>
      );
    case "grid":
      return <LiveGrid node={node} />;
    case "card":
      return <LiveCard node={node} />;
    case "tabs":
      return <LiveTabs node={node} />;
    case "collapse":
      return <LiveCollapse node={node} />;
    case "divider":
      return <hr className="border-0 border-t-[0.5px] border-border" />;
    case "text":
      return <LiveText node={node} />;
    case "stat":
      return <LiveStat node={node} />;
    case "badge":
      return <LiveBadge node={node} />;
    case "progress":
      return <LiveProgress node={node} />;
    case "callout":
      return <LiveCallout node={node} />;
    case "formula":
      return <LiveFormula node={node} />;
    case "scene":
      return <LiveScene node={node} />;
    case "slider":
      return <LiveSlider node={node} />;
    case "select":
      return <LiveSelect node={node} />;
    case "segmented":
      return <LiveSegmented node={node} />;
    case "switch":
      return <LiveSwitch node={node} />;
    case "input":
      return <LiveInput node={node} />;
    case "button":
      return <LiveButton node={node} />;
    default:
      return null;
  }
}

// The root panel carries the block's heading and, once the user changed
// something, a reset back to the declared state; a nested one is a titled stack.
function LivePanel({ node }: { node: ResolvedNode }) {
  const t = useTranslations("chat.markdown.uiBlock");
  const { changed, reset } = useLiveBlock();
  const title = readText(node.props, "title");
  const description = readText(node.props, "description");
  const root = node.path === "";
  const showReset = root && changed;
  return (
    <section className={cn("min-w-0", root && "px-4 pb-4 pt-5")}>
      {title || showReset ? (
        <div className="flex items-start justify-between gap-3">
          {title ? <h3 className={cn("min-w-0 font-semibold", root ? "text-lg leading-7" : "text-sm leading-6")}>{title}</h3> : <span />}
          {showReset ? (
            <button
              type="button"
              onClick={reset}
              className="inline-flex shrink-0 items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
            >
              <RotateCcw className="size-3" strokeWidth={1.8} />
              {t("reset")}
            </button>
          ) : null}
        </div>
      ) : null}
      {description ? <p className="mt-0.5 text-sm leading-6 text-muted-foreground">{description}</p> : null}
      <LiveFlow node={node} className={cn((title || description || showReset) && "mt-3")} />
    </section>
  );
}

const VISUAL_PRIMITIVES = new Set(["scene"]);

// The children of a panel or card. When they include a diagram next to a
// formula or controls, the section becomes an explorable: the diagram on the
// right, everything else on the left (stacked when the block is narrow), and a
// send button below both. Models write a flat list and still get a balanced
// layout; a row they write themselves is left alone.
function LiveFlow({ node, className }: { node: ResolvedNode; className?: string }) {
  const { renderNode } = useLiveBlock();
  const visuals = node.children.filter((child) => VISUAL_PRIMITIVES.has(child.component));
  const asks = node.children.filter((child) => child.component === "button" && typeof child.props.ask === "string");
  const main = node.children.filter((child) => !visuals.includes(child) && !asks.includes(child));
  const render = (child: ResolvedNode) => <React.Fragment key={child.path}>{renderNode(child)}</React.Fragment>;
  if (visuals.length === 0 || main.length === 0) {
    return (
      <div className={cn("grid min-w-0 gap-4", className)}>
        <LiveChildren node={node} />
      </div>
    );
  }
  return (
    <div className={cn("@container grid min-w-0 gap-4", className)}>
      <div className="grid min-w-0 gap-4 @xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] @xl:gap-6">
        <div className="grid min-w-0 content-center gap-4">{main.map(render)}</div>
        {/* The diagram sits on its own tile, so it reads as one picture, not loose lines on the card. */}
        <div className="flex min-w-0 flex-col items-center justify-center gap-4 rounded-lg bg-muted/45 px-4 py-5 dark:bg-muted/25">
          {visuals.map(render)}
        </div>
      </div>
      {asks.map(render)}
    </div>
  );
}

const GRID_COLUMNS: Record<number, string> = {
  1: "grid-cols-1",
  2: "grid-cols-1 sm:grid-cols-2",
  3: "grid-cols-1 sm:grid-cols-2 md:grid-cols-3",
  4: "grid-cols-2 md:grid-cols-4",
};

function LiveGrid({ node }: { node: ResolvedNode }) {
  const columns = Math.round(readNumber(node.props, "columns") ?? 0);
  return (
    <div className={cn("grid min-w-0 gap-3", GRID_COLUMNS[columns] ?? "grid-cols-[repeat(auto-fit,minmax(10rem,1fr))]")}>
      <LiveChildren node={node} />
    </div>
  );
}

function LiveCard({ node }: { node: ResolvedNode }) {
  const title = readText(node.props, "title");
  return (
    <div className="min-w-0 rounded-lg border-[0.5px] border-border bg-background p-3">
      {title ? <p className="mb-2 text-sm font-medium leading-6">{title}</p> : null}
      <LiveFlow node={node} />
    </div>
  );
}

// Tab i shows child i. With `bind` the open tab is block state (the model sees
// which one the user looked at); without it the choice stays local.
function LiveTabs({ node }: { node: ResolvedNode }) {
  const { state, update, renderNode } = useLiveBlock();
  const options = readOptions(node.props, "items");
  const [localIndex, setLocalIndex] = React.useState(0);
  const bound = node.bind ? optionIndex(options, state[node.bind]) : -1;
  const active = node.bind ? Math.max(0, bound) : Math.min(localIndex, Math.max(0, options.length - 1));
  const select = (index: number, value: UIValue) => {
    if (node.bind) {
      update({ [node.bind]: value });
    } else {
      setLocalIndex(index);
    }
  };
  const child = node.children[active];
  return (
    <div className="grid min-w-0 gap-3">
      <div role="tablist" className="flex flex-wrap gap-1.5">
        {options.map((option, index) => (
          <ChipButton key={`${node.path}-${option.label}`} active={index === active} onClick={() => select(index, option.value)}>
            {option.label}
          </ChipButton>
        ))}
      </div>
      {child ? <div role="tabpanel">{renderNode(child)}</div> : null}
    </div>
  );
}

function LiveCollapse({ node }: { node: ResolvedNode }) {
  const [open, setOpen] = React.useState(() => readBoolean(node.props, "open") ?? false);
  const contentID = React.useId();
  return (
    <div className="min-w-0 rounded-lg border-[0.5px] border-border">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={contentID}
        onClick={() => setOpen((value) => !value)}
        className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm font-medium transition-colors hover:bg-accent/40"
      >
        <span className="min-w-0 flex-1 truncate">{readText(node.props, "title") ?? ""}</span>
        <ChevronDown className={cn("size-3.5 shrink-0 text-muted-foreground transition-transform", open && "rotate-180")} strokeWidth={1.8} />
      </button>
      <CollapsibleMotionContent id={contentID} open={open}>
        <div className="grid min-w-0 gap-3 border-t-[0.5px] border-border p-3">
          <LiveChildren node={node} />
        </div>
      </CollapsibleMotionContent>
    </div>
  );
}

const TEXT_SIZE = { sm: "text-xs leading-5", base: "text-sm leading-6", lg: "text-base font-medium leading-7" } as const;
const TEXT_SIZES = ["sm", "base", "lg"] as const;

function LiveText({ node }: { node: ResolvedNode }) {
  const text = displayValue(node.props.text);
  if (!text) {
    return null;
  }
  const tone = readEnum(node.props, "tone", LIVE_TONES) ?? "default";
  const size = readEnum(node.props, "size", TEXT_SIZES) ?? "base";
  return <p className={cn("min-w-0 whitespace-pre-wrap break-words", TEXT_SIZE[size], TONE_TEXT_CLASS[tone])}>{text}</p>;
}

function LiveStat({ node }: { node: ResolvedNode }) {
  const label = readText(node.props, "label");
  const unit = readText(node.props, "unit");
  const note = readText(node.props, "note");
  const tone = readEnum(node.props, "tone", LIVE_TONES) ?? "default";
  const highlight = readBoolean(node.props, "highlight") ?? false;
  const precision = readNumber(node.props, "precision");
  const raw = node.props.value;
  // A value that failed to compute shows a dash rather than disappearing.
  const value = raw === undefined ? "—" : typeof raw === "number" && precision !== undefined ? formatFixed(raw, precision) : displayValue(raw);
  return (
    <div className={cn("min-w-0 rounded-lg border-[0.5px] px-3 py-2.5", highlight ? "border-primary/40 bg-primary/5" : "border-border bg-background")}>
      {label ? <p className="truncate text-[11px] text-muted-foreground">{label}</p> : null}
      <p className={cn("mt-0.5 text-xl font-semibold leading-7 tabular-nums", TONE_TEXT_CLASS[tone])}>
        {value}
        {unit ? <span className="ml-1 text-xs font-normal text-muted-foreground">{unit}</span> : null}
      </p>
      {note ? <p className="mt-0.5 text-[11px] text-muted-foreground">{note}</p> : null}
    </div>
  );
}

function formatFixed(value: number, precision: number): string {
  if (!Number.isFinite(value)) {
    return "—";
  }
  const digits = Math.min(6, Math.max(0, Math.round(precision)));
  return new Intl.NumberFormat("en-US", { minimumFractionDigits: digits, maximumFractionDigits: digits }).format(value);
}

const BADGE_TONE_CLASS: Record<LiveTone, string> = {
  default: "border-border bg-muted text-foreground",
  muted: "border-border text-muted-foreground",
  positive: "border-green-600/30 bg-green-600/10 text-green-700 dark:text-green-400",
  negative: "border-red-600/30 bg-red-600/10 text-red-700 dark:text-red-400",
  warning: "border-amber-600/30 bg-amber-600/10 text-amber-700 dark:text-amber-400",
  info: "border-sky-600/30 bg-sky-600/10 text-sky-700 dark:text-sky-400",
};

function LiveBadge({ node }: { node: ResolvedNode }) {
  const text = displayValue(node.props.text ?? node.props.label);
  if (!text) {
    return null;
  }
  const tone = readEnum(node.props, "tone", LIVE_TONES) ?? "default";
  return (
    <span className={cn("inline-flex w-fit max-w-full items-center truncate rounded-full border-[0.5px] px-2 py-0.5 text-[11px] font-medium", BADGE_TONE_CLASS[tone])}>{text}</span>
  );
}

function LiveProgress({ node }: { node: ResolvedNode }) {
  const max = readNumber(node.props, "max") ?? 100;
  const value = readNumber(node.props, "value") ?? 0;
  const ratio = max > 0 ? Math.min(1, Math.max(0, value / max)) : 0;
  const label = readText(node.props, "label");
  return (
    <div className="min-w-0">
      <div className="mb-1.5 flex items-baseline justify-between gap-2 text-xs">
        <span className="truncate text-muted-foreground">{label ?? ""}</span>
        <span className="shrink-0 font-medium tabular-nums">{Math.round(ratio * 100)}%</span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-muted" role="progressbar" aria-valuemin={0} aria-valuemax={max} aria-valuenow={value} aria-label={label}>
        <div className="h-full rounded-full bg-primary transition-[width] duration-300" style={{ width: `${ratio * 100}%` }} />
      </div>
    </div>
  );
}

const CALLOUT_TONES = ["info", "positive", "warning", "negative"] as const;
const CALLOUT_CLASS: Record<(typeof CALLOUT_TONES)[number], string> = {
  info: "border-sky-600/25 bg-sky-600/5",
  positive: "border-green-600/25 bg-green-600/5",
  warning: "border-amber-600/25 bg-amber-600/5",
  negative: "border-red-600/25 bg-red-600/5",
};

function LiveCallout({ node }: { node: ResolvedNode }) {
  const tone = readEnum(node.props, "tone", CALLOUT_TONES) ?? "info";
  const title = readText(node.props, "title");
  const text = displayValue(node.props.text);
  const Icon = tone === "warning" ? TriangleAlert : tone === "negative" ? CircleAlert : Info;
  return (
    <div className={cn("flex min-w-0 gap-2.5 rounded-lg border-[0.5px] px-3 py-2.5", CALLOUT_CLASS[tone])}>
      <Icon className={cn("mt-0.5 size-4 shrink-0", TONE_TEXT_CLASS[tone])} strokeWidth={1.8} />
      <div className="min-w-0 text-sm leading-6">
        {title ? <p className="font-medium">{title}</p> : null}
        {text ? <p className="whitespace-pre-wrap break-words text-foreground/90">{text}</p> : null}
      </div>
    </div>
  );
}
