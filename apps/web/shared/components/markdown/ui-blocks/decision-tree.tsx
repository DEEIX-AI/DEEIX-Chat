"use client";

import { ArrowLeft, CheckCircle2, RotateCcw } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { HeightTransition } from "@/components/ui/height-transition";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { SlideSwitch } from "@/shared/components/slide-switch";
import type { UIBlockDefinition, UIBlockRenderProps } from "./block";
import { joinTextBlocks, textTitle } from "./block-text";
import { s } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";
import { type UIBlockState, useUIBlockState } from "./ui-block-state";

type DecisionNode = {
  id: string;
  text: string;
  detail?: string;
  options?: { label: string; next: string }[];
};

export type DecisionTreeProps = {
  title?: string;
  start?: string;
  nodes: DecisionNode[];
};

const MAX_DEPTH = 64;

type DecisionTreeState = { path: string[] };

// Keeps the stored path only as far as every step still follows an option of
// the node before it; the tree may have changed since it was saved.
function restoreDecisionPath(byID: ReadonlyMap<string, DecisionNode>, startID: string, saved: UIBlockState | undefined): string[] {
  const stored = saved?.path;
  if (!Array.isArray(stored) || stored[0] !== startID) {
    return [startID];
  }
  const path = [startID];
  for (let index = 1; index < Math.min(stored.length, MAX_DEPTH); index += 1) {
    const step = stored[index];
    const previous = byID.get(path[path.length - 1]);
    if (typeof step !== "string" || !byID.has(step) || !previous?.options?.some((option) => option.next === step)) {
      break;
    }
    path.push(step);
  }
  return path;
}

// The label of the option that led from one step to the next.
function choiceLabel(byID: ReadonlyMap<string, DecisionNode>, fromID: string, toID: string | undefined): string {
  return byID.get(fromID)?.options?.find((option) => option.next === toID)?.label ?? "";
}

function DecisionTree({ id, props }: UIBlockRenderProps<DecisionTreeProps>) {
  const t = useTranslations("chat.markdown.uiBlock");
  const byID = React.useMemo(() => new Map(props.nodes.map((node) => [node.id, node])), [props.nodes]);
  const startID = props.start ?? props.nodes[0]?.id ?? "";
  const [{ path }, setState] = useUIBlockState<DecisionTreeState>(id, {
    defaults: { path: [startID] },
    restore: (saved) => ({ path: restoreDecisionPath(byID, startID, saved) }),
    title: props.title,
  });
  // Choosing slides forward (from the right); back / reset slide backward.
  const [direction, setDirection] = React.useState<1 | -1>(1);

  const currentID = path[path.length - 1] ?? "";
  const node = byID.get(currentID);
  const terminal = !node?.options || node.options.length === 0;

  const choose = (option: { label: string; next: string }) => {
    if (path.length >= MAX_DEPTH || !byID.has(option.next)) {
      return;
    }
    setDirection(1);
    setState((current) => ({ path: [...current.path, option.next] }));
  };

  return (
    <div className="px-4 pb-4 pt-5">
      <div className="flex items-baseline justify-between gap-3">
        {props.title ? <h3 className="text-lg font-semibold leading-7">{props.title}</h3> : <span />}
        <div className="flex shrink-0 items-center gap-3 text-xs text-muted-foreground">
          {path.length > 1 ? (
            <button type="button" onClick={() => { setDirection(-1); setState((current) => ({ path: current.path.slice(0, -1) })); }} className="inline-flex items-center gap-1 hover:text-foreground">
              <ArrowLeft className="size-3" strokeWidth={1.8} />
              {t("back")}
            </button>
          ) : null}
          {path.length > 1 ? (
            <button type="button" onClick={() => { setDirection(-1); setState(null); }} className="inline-flex items-center gap-1 hover:text-foreground">
              <RotateCcw className="size-3" strokeWidth={1.8} />
              {t("reset")}
            </button>
          ) : null}
        </div>
      </div>

      <HeightTransition className="mt-3" contentClassName="flex flex-col gap-3">
      {path.length > 1 ? (
        <ol className="flex flex-wrap items-center gap-1 text-[11px] text-muted-foreground">
          {path.slice(0, -1).map((stepID, index) => (
            <li key={`${id}-crumb-${index}`} className="inline-flex items-center gap-1">
              <span className="rounded-md bg-muted px-1.5 py-0.5">{choiceLabel(byID, stepID, path[index + 1])}</span>
              <span aria-hidden="true">›</span>
            </li>
          ))}
        </ol>
      ) : null}

      <SlideSwitch itemKey={`${path.length}:${currentID}`} direction={direction}>
      {node ? (
        <div className={cn(terminal && "rounded-lg border-[0.5px] border-primary/40 bg-primary/5 p-4")}>
          <div className="flex items-start gap-2">
            {terminal ? <CheckCircle2 className="mt-0.5 size-4 shrink-0 text-primary" strokeWidth={1.8} /> : null}
            <div className="min-w-0">
              <p className="text-sm font-medium leading-6">{node.text}</p>
              {node.detail ? <p className="mt-1 whitespace-pre-wrap text-xs leading-5 text-muted-foreground">{node.detail}</p> : null}
            </div>
          </div>
          {!terminal ? (
            <div className="mt-3 grid gap-2 sm:grid-cols-2">
              {node.options?.map((option, index) => (
                <button
                  key={`${id}-${node.id}-${index}`}
                  type="button"
                  disabled={!byID.has(option.next)}
                  onClick={() => choose(option)}
                  className="rounded-lg border-[0.5px] border-border bg-background px-3 py-2.5 text-left text-sm transition-colors hover:border-primary/40 hover:bg-primary/5 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {option.label}
                </button>
              ))}
            </div>
          ) : null}
        </div>
      ) : (
        <p className="text-xs text-destructive">{t("missingNode", { id: currentID })}</p>
      )}
      </SlideSwitch>
      </HeightTransition>
    </div>
  );
}

function DecisionTreeSkeleton() {
  return (
    <UIBlockFrame className="p-4">
      <Skeleton className="h-5 w-1/3 rounded-full" />
      <Skeleton className="mt-3 h-16 rounded-lg" />
      <div className="mt-3 grid gap-2 sm:grid-cols-2">
        <Skeleton className="h-9 rounded-md" />
        <Skeleton className="h-9 rounded-md" />
      </div>
    </UIBlockFrame>
  );
}

// The path the user took: each step with the choice made, then where it ended
// (or the open question with its options).
function decisionTreeText(props: DecisionTreeProps, state: UIBlockState | undefined): string {
  const byID = new Map(props.nodes.map((node) => [node.id, node]));
  const startID = props.start ?? props.nodes[0]?.id ?? "";
  const path = restoreDecisionPath(byID, startID, state);
  const lines: string[] = [];
  path.forEach((nodeID, index) => {
    const node = byID.get(nodeID);
    if (!node) {
      return;
    }
    const next = path[index + 1];
    lines.push(next ? `${index + 1}. ${node.text} → **${choiceLabel(byID, nodeID, next)}**` : `${index + 1}. ${node.text}`);
    if (!next) {
      if (node.detail?.trim()) {
        lines.push(`   ${node.detail.trim()}`);
      }
      for (const option of node.options ?? []) {
        lines.push(`   - ${option.label}`);
      }
    }
  });
  return joinTextBlocks([...textTitle(props.title), lines.join("\n")]);
}

export const decisionTreeDefinition: UIBlockDefinition<DecisionTreeProps> = {
  name: "decision-tree",
  version: 1,
  schema: s.object(
    {
      title: s.string(),
      start: s.string(),
      nodes: s.array(
        s.object(
          { id: s.string(), text: s.string(), detail: s.string(), options: s.array(s.object({ label: s.string(), next: s.string() }, ["label", "next"])) },
          ["id", "text"],
        ),
        { minItems: 1 },
      ),
    },
    ["nodes"],
  ),
  Component: DecisionTree,
  Skeleton: DecisionTreeSkeleton,
  toText: decisionTreeText,
};
