"use client";

import { Check, ChevronDown, ChevronUp, Columns2, Copy, Rows3 } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";

import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { writeClipboardText } from "@/shared/lib/clipboard";
import { type DiffLine, type DiffSegment, diffLines, diffWords, lineSimilarity } from "@/shared/lib/line-diff";
import type { UIBlockDefinition, UIBlockRenderProps } from "./block";
import { s } from "./schema";
import { UIBlockFrame } from "./ui-block-frame";

export type DiffProps = {
  title?: string;
  before: string;
  after: string;
  language?: string;
  beforeLabel?: string;
  afterLabel?: string;
};

const CONTEXT_LINES = 3;
const COPIED_MS = 1500;

type Row = DiffLine & { hunk?: number; segments?: DiffSegment[]; pair?: Row };
type DisplayRow = Row | { kind: "gap"; count: number };

// Number hunks (runs of changed lines) and attach word-level segments to
// removed/added pairs inside a hunk so edits within a line stand out.
function annotate(lines: DiffLine[]): { rows: Row[]; hunks: number } {
  const rows: Row[] = lines.map((line) => ({ ...line }));
  let hunk = -1;
  let index = 0;
  while (index < rows.length) {
    if (rows[index]?.kind === "same") {
      index += 1;
      continue;
    }
    hunk += 1;
    const removed: Row[] = [];
    const added: Row[] = [];
    while (index < rows.length && rows[index]?.kind !== "same") {
      const row = rows[index] as Row;
      row.hunk = hunk;
      (row.kind === "removed" ? removed : added).push(row);
      index += 1;
    }
    // Pair each removed line with the added line it most resembles, so an
    // inserted line in between does not shift every pairing by one.
    const candidates: { left: Row; right: Row; score: number }[] = [];
    for (const left of removed) {
      for (const right of added) {
        const score = lineSimilarity(left.text, right.text);
        if (score > 0) {
          candidates.push({ left, right, score });
        }
      }
    }
    candidates.sort((a, b) => b.score - a.score);
    const taken = new Set<Row>();
    for (const { left, right } of candidates) {
      if (taken.has(left) || taken.has(right)) {
        continue;
      }
      taken.add(left);
      taken.add(right);
      const words = diffWords(left.text, right.text);
      left.segments = words.before;
      right.segments = words.after;
      left.pair = right;
      right.pair = left;
    }
  }
  return { rows, hunks: hunk + 1 };
}

// Collapse long unchanged runs to CONTEXT_LINES on either side, like a unified diff.
function collapse(lines: Row[], expanded: boolean): DisplayRow[] {
  if (expanded) {
    return lines;
  }
  const result: DisplayRow[] = [];
  let run: Row[] = [];
  const flush = (isEnd: boolean, isStart: boolean) => {
    if (run.length <= CONTEXT_LINES * 2 || (isStart && run.length <= CONTEXT_LINES) || (isEnd && run.length <= CONTEXT_LINES)) {
      result.push(...run);
    } else {
      const head = isStart ? [] : run.slice(0, CONTEXT_LINES);
      const tail = isEnd ? [] : run.slice(-CONTEXT_LINES);
      result.push(...head, { kind: "gap", count: run.length - head.length - tail.length }, ...tail);
    }
    run = [];
  };
  let sawChange = false;
  for (const line of lines) {
    if (line.kind === "same") {
      run.push(line);
    } else {
      flush(false, !sawChange);
      sawChange = true;
      result.push(line);
    }
  }
  flush(true, !sawChange);
  return result;
}

// Side-by-side rows: removed/added lines inside a hunk share a row where they pair up.
type SplitRow = { kind: "gap"; count: number } | { kind: "pair"; left?: Row; right?: Row; hunk?: number };

function toSplit(rows: DisplayRow[]): SplitRow[] {
  const result: SplitRow[] = [];
  let index = 0;
  while (index < rows.length) {
    const row = rows[index] as DisplayRow;
    if (row.kind === "gap") {
      result.push(row);
      index += 1;
    } else if (row.kind === "same") {
      result.push({ kind: "pair", left: row, right: row });
      index += 1;
    } else {
      const removed: Row[] = [];
      const added: Row[] = [];
      while (index < rows.length && rows[index]?.kind !== "same" && rows[index]?.kind !== "gap") {
        const changed = rows[index] as Row;
        (changed.kind === "removed" ? removed : added).push(changed);
        index += 1;
      }
      const hunk = (removed[0] ?? added[0])?.hunk;
      // Walk both sides in order; a matched pair shares a row, everything else
      // takes its own row so line order on each side is preserved.
      let leftIndex = 0;
      let rightIndex = 0;
      while (leftIndex < removed.length || rightIndex < added.length) {
        const left = removed[leftIndex];
        const right = added[rightIndex];
        if (left && right && left.pair === right) {
          result.push({ kind: "pair", left, right, hunk });
          leftIndex += 1;
          rightIndex += 1;
        } else if (left && !left.pair) {
          result.push({ kind: "pair", left, hunk });
          leftIndex += 1;
        } else if (right && !right.pair) {
          result.push({ kind: "pair", right, hunk });
          rightIndex += 1;
        } else if (left) {
          // Both heads are paired further down (crossing pairs): let the left side lead.
          result.push({ kind: "pair", left, hunk });
          leftIndex += 1;
        } else if (right) {
          result.push({ kind: "pair", right, hunk });
          rightIndex += 1;
        }
      }
    }
  }
  return result;
}

function LineText({ row }: { row: Row }) {
  if (!row.segments) {
    return <>{row.text}</>;
  }
  return (
    <>
      {row.segments.map((segment, index) => (
        <span key={`${index}-${segment.text.length}`} className={cn(segment.changed && (row.kind === "added" ? "rounded-sm bg-green-500/25" : "rounded-sm bg-red-500/25"))}>
          {segment.text}
        </span>
      ))}
    </>
  );
}

function Diff({ id, props }: UIBlockRenderProps<DiffProps>) {
  const t = useTranslations("chat.markdown.uiBlock");
  const [expanded, setExpanded] = React.useState(false);
  const [split, setSplit] = React.useState(false);
  const [activeHunk, setActiveHunk] = React.useState<number | null>(null);
  const [copied, setCopied] = React.useState(false);
  const scrollRef = React.useRef<HTMLDivElement>(null);

  const { rows: annotated, hunks } = React.useMemo(() => annotate(diffLines(props.before, props.after)), [props.after, props.before]);
  const stats = React.useMemo(
    () => annotated.reduce((acc, line) => ({ added: acc.added + (line.kind === "added" ? 1 : 0), removed: acc.removed + (line.kind === "removed" ? 1 : 0) }), { added: 0, removed: 0 }),
    [annotated],
  );
  const rows = React.useMemo(() => collapse(annotated, expanded), [annotated, expanded]);
  const splitRows = React.useMemo(() => (split ? toSplit(rows) : []), [rows, split]);
  const hasGap = rows.some((row) => row.kind === "gap");

  const jump = (direction: 1 | -1) => {
    if (hunks === 0) {
      return;
    }
    const next = activeHunk === null ? (direction === 1 ? 0 : hunks - 1) : (activeHunk + direction + hunks) % hunks;
    setActiveHunk(next);
    // Scroll only this block's own viewport; scrollIntoView would also move the page.
    const container = scrollRef.current;
    const target = container?.querySelector<HTMLElement>(`[data-hunk="${next}"]`);
    if (container && target) {
      container.scrollTo({ top: Math.max(0, target.offsetTop - container.clientHeight / 2 + target.offsetHeight / 2), behavior: "smooth" });
    }
  };

  const copyAfter = async () => {
    try {
      await writeClipboardText(props.after);
      setCopied(true);
      window.setTimeout(() => setCopied(false), COPIED_MS);
    } catch {
      setCopied(false);
    }
  };

  const rowClass = (kind: DiffLine["kind"] | undefined, hunk: number | undefined) =>
    cn(
      kind === "added" && "bg-green-500/10 text-green-900 dark:text-green-200",
      kind === "removed" && "bg-red-500/10 text-red-900 dark:text-red-200",
      hunk !== undefined && hunk === activeHunk && "shadow-[inset_2px_0_0_var(--primary)]",
    );
  const numberClass = "w-10 min-w-10 select-none whitespace-nowrap border-r-[0.5px] border-border px-2 text-right text-[11px] text-muted-foreground/70 tabular-nums";
  const iconButton = "inline-flex size-6 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:opacity-40 disabled:hover:bg-transparent";

  return (
    <div>
      <div className="flex items-center justify-between gap-3 border-b-[0.5px] border-border px-4 pb-3 pt-5">
        <div className="min-w-0">
          {props.title ? <h3 className="truncate text-lg font-semibold leading-7">{props.title}</h3> : null}
          <p className="text-[11px] text-muted-foreground">
            {props.beforeLabel ?? t("diffBefore")} → {props.afterLabel ?? t("diffAfter")}
            <span className="ml-2 text-green-600 dark:text-green-400">+{stats.added}</span>
            <span className="ml-1 text-red-600 dark:text-red-400">−{stats.removed}</span>
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {hunks > 1 ? (
            <span className="flex items-center text-[11px] text-muted-foreground">
              <button type="button" aria-label={t("diffPrevChange")} onClick={() => jump(-1)} className={iconButton}>
                <ChevronUp className="size-3.5" strokeWidth={1.8} />
              </button>
              <span className="min-w-8 text-center tabular-nums">
                {activeHunk === null ? "–" : activeHunk + 1} / {hunks}
              </span>
              <button type="button" aria-label={t("diffNextChange")} onClick={() => jump(1)} className={iconButton}>
                <ChevronDown className="size-3.5" strokeWidth={1.8} />
              </button>
            </span>
          ) : null}
          {hasGap || expanded ? (
            <button type="button" onClick={() => setExpanded((value) => !value)} className="px-1.5 text-[11px] text-muted-foreground hover:text-foreground">
              {expanded ? t("diffCollapse") : t("diffExpand")}
            </button>
          ) : null}
          <button type="button" aria-pressed={split} aria-label={split ? t("diffUnified") : t("diffSplit")} title={split ? t("diffUnified") : t("diffSplit")} onClick={() => setSplit((value) => !value)} className={iconButton}>
            {split ? <Rows3 className="size-3.5" strokeWidth={1.8} /> : <Columns2 className="size-3.5" strokeWidth={1.8} />}
          </button>
          <button type="button" aria-label={t("diffCopyAfter")} title={t("diffCopyAfter")} onClick={copyAfter} className={iconButton}>
            {copied ? <Check className="size-3.5 text-green-600 dark:text-green-400" strokeWidth={2} /> : <Copy className="size-3.5" strokeWidth={1.8} />}
          </button>
        </div>
      </div>
      <div ref={scrollRef} className="relative max-h-[32rem] overflow-auto">
        {/* Split view grows to the longest line and scrolls horizontally as one, so both sides stay aligned. */}
        <table className={cn("border-collapse font-mono text-[12px] leading-5", split ? "w-max min-w-full" : "w-full")}>
          <tbody>
            {split
              ? splitRows.map((row, index) =>
                  row.kind === "gap" ? (
                    <tr key={`${id}-gap-${index}`}>
                      <td colSpan={4} className="bg-muted/40 px-4 py-1 text-center text-[11px] text-muted-foreground">
                        {t("diffHidden", { count: row.count })}
                      </td>
                    </tr>
                  ) : (
                    <tr key={`${id}-${index}`} data-hunk={row.hunk}>
                      <td className={cn(numberClass, rowClass(row.left?.kind, row.hunk))}>{row.left?.oldNo ?? ""}</td>
                      <td className={cn("w-1/2 whitespace-pre px-3", rowClass(row.left?.kind, undefined))}>{row.left ? <LineText row={row.left} /> : null}</td>
                      <td className={cn(numberClass, "border-l-[0.5px]", rowClass(row.right?.kind, undefined))}>{row.right?.newNo ?? ""}</td>
                      <td className={cn("w-1/2 whitespace-pre px-3", rowClass(row.right?.kind, undefined))}>{row.right ? <LineText row={row.right} /> : null}</td>
                    </tr>
                  ),
                )
              : rows.map((row, index) =>
                  row.kind === "gap" ? (
                    <tr key={`${id}-gap-${index}`}>
                      <td colSpan={3} className="bg-muted/40 px-4 py-1 text-center text-[11px] text-muted-foreground">
                        {t("diffHidden", { count: row.count })}
                      </td>
                    </tr>
                  ) : (
                    <tr key={`${id}-${index}`} data-hunk={row.hunk} className={rowClass(row.kind, row.hunk)}>
                      <td className={numberClass}>{row.oldNo ?? ""}</td>
                      <td className={numberClass}>{row.newNo ?? ""}</td>
                      <td className="whitespace-pre px-3">
                        <span className="mr-2 inline-block w-3 select-none text-muted-foreground/70">{row.kind === "added" ? "+" : row.kind === "removed" ? "−" : " "}</span>
                        <LineText row={row} />
                      </td>
                    </tr>
                  ),
                )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function DiffSkeleton() {
  return (
    <UIBlockFrame className="p-4">
      <Skeleton className="h-4 w-1/3 rounded-full" />
      <div className="mt-3 space-y-1.5">
        {Array.from({ length: 6 }).map((_, index) => (
          <Skeleton key={`diff-${index}`} className="h-4 rounded-sm" style={{ width: `${55 + ((index * 31) % 40)}%` }} />
        ))}
      </div>
    </UIBlockFrame>
  );
}

export const diffDefinition: UIBlockDefinition<DiffProps> = {
  name: "diff",
  version: 1,
  schema: s.object(
    {
      title: s.string(),
      before: s.string(),
      after: s.string(),
      language: s.string(),
      beforeLabel: s.string(),
      afterLabel: s.string(),
    },
    ["before", "after"],
  ),
  Component: Diff,
  Skeleton: DiffSkeleton,
};
