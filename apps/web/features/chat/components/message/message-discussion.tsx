"use client";

import * as React from "react";
import { Bug, Check, ChevronDown, ChevronRight, Loader2, Minus, Users, X } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";
import { StreamdownRender } from "@/shared/components/markdown/streamdown-render";
import type {
  DiscussionRuntimeRound,
  DiscussionState,
  DiscussionTurn,
} from "@/shared/api/discussion-types";
import {
  discussionPlaceholderLabels,
  orderDiscussionPlaceholders,
} from "@/shared/lib/discussion";

const DISCUSSION_DEBUG_STORAGE_KEY = "deeix:discussion-debug";

type RoundTone = {
  title: string;
  badge: string;
  accent: string;
};

// Each round gets its own colour so 第 1 轮 … 第 n 轮 … 总结 read as separate blocks.
const ROUND_TONES: RoundTone[] = [
  { title: "text-sky-600 dark:text-sky-300", badge: "bg-sky-500/10 text-sky-700 dark:text-sky-300", accent: "border-l-sky-400/80" },
  { title: "text-violet-600 dark:text-violet-300", badge: "bg-violet-500/10 text-violet-700 dark:text-violet-300", accent: "border-l-violet-400/80" },
  { title: "text-amber-600 dark:text-amber-300", badge: "bg-amber-500/10 text-amber-700 dark:text-amber-300", accent: "border-l-amber-400/80" },
  { title: "text-emerald-600 dark:text-emerald-300", badge: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300", accent: "border-l-emerald-400/80" },
  { title: "text-rose-600 dark:text-rose-300", badge: "bg-rose-500/10 text-rose-700 dark:text-rose-300", accent: "border-l-rose-400/80" },
  { title: "text-cyan-600 dark:text-cyan-300", badge: "bg-cyan-500/10 text-cyan-700 dark:text-cyan-300", accent: "border-l-cyan-400/80" },
  { title: "text-indigo-600 dark:text-indigo-300", badge: "bg-indigo-500/10 text-indigo-700 dark:text-indigo-300", accent: "border-l-indigo-400/80" },
  { title: "text-lime-600 dark:text-lime-300", badge: "bg-lime-500/10 text-lime-700 dark:text-lime-300", accent: "border-l-lime-400/80" },
  { title: "text-orange-600 dark:text-orange-300", badge: "bg-orange-500/10 text-orange-700 dark:text-orange-300", accent: "border-l-orange-400/80" },
  { title: "text-fuchsia-600 dark:text-fuchsia-300", badge: "bg-fuchsia-500/10 text-fuchsia-700 dark:text-fuchsia-300", accent: "border-l-fuchsia-400/80" },
];

const SUMMARY_TONE: RoundTone = {
  title: "text-primary",
  badge: "bg-primary/10 text-primary",
  accent: "border-l-primary/80",
};

function toneFor(round: number): RoundTone {
  return ROUND_TONES[(Math.max(round, 1) - 1) % ROUND_TONES.length];
}

function statusClass(status: string): string {
  switch (status) {
    case "completed":
      return "bg-emerald-500";
    case "error":
      return "bg-rose-500";
    case "stopped":
      return "bg-amber-500";
    default:
      return "bg-sky-500";
  }
}

function usageSeconds(latencyMs?: number): number {
  return latencyMs && latencyMs > 0 ? Math.round(latencyMs / 100) / 10 : 0;
}

type UsageFormat = (input: number, output: number, seconds: number) => string;

function usageText(format: UsageFormat, turn: DiscussionTurn): string {
  return format(turn.inputTokens ?? 0, turn.outputTokens ?? 0, usageSeconds(turn.latencyMs));
}

function usageCellClass(status: string): string {
  switch (status) {
    case "completed":
      return "border-border/60 bg-background/60 hover:border-primary/40";
    case "error":
      return "border-rose-200 bg-rose-50/60 dark:border-rose-900/60 dark:bg-rose-950/20 hover:border-rose-300";
    case "running":
      return "border-sky-200 bg-sky-50/60 dark:border-sky-900/60 dark:bg-sky-950/20 hover:border-sky-300";
    default:
      return "border-dashed border-border/50 bg-transparent hover:border-border";
  }
}

function usageStatusMark(status: string) {
  switch (status) {
    case "completed":
      return <Check className="size-3 text-emerald-600 dark:text-emerald-400" />;
    case "error":
      return <X className="size-3 text-rose-500" />;
    case "running":
      return <Loader2 className="size-3 animate-spin text-sky-500" />;
    default:
      return <Minus className="size-3 text-muted-foreground/50" />;
  }
}

/** Collapsible view of the exact prompt sent to one model; debug only. */
function DebugPrompt({
  model,
  prompt,
  label,
}: {
  model: string;
  prompt?: string;
  label: string;
}) {
  const t = useTranslations("chat.discussion");
  const [open, setOpen] = React.useState(false);
  return (
    <div className="mt-1.5">
      <button
        type="button"
        className="inline-flex items-center gap-1 rounded-md border border-border/60 px-1.5 py-0.5 text-[10px] text-muted-foreground transition-colors hover:text-foreground"
        onClick={() => setOpen((value) => !value)}
        title={label}
      >
        {open ? <ChevronDown className="size-3" /> : <ChevronRight className="size-3" />}
        {open ? t("debugHidePrompt") : t("debugShowPrompt")}
        <span className="font-mono opacity-70">{model || "—"}</span>
      </button>
      {open ? (
        prompt ? (
          <pre className="mt-1 max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-md border border-border/50 bg-muted/40 p-2 font-mono text-[11px] leading-5 text-foreground/90">
            {prompt}
          </pre>
        ) : (
          <p className="mt-1 text-[10px] text-muted-foreground">{t("debugPromptMissing")}</p>
        )
      ) : null}
    </div>
  );
}

export function MessageDiscussion({ discussion }: { discussion?: DiscussionState }) {
  const t = useTranslations("chat.discussion");
  const [expanded, setExpanded] = React.useState(false);
  const [debug, setDebug] = React.useState(false);
  // Which model speaks for a round: absent means "follow the round" (first running or first model).
  const [selectedByRound, setSelectedByRound] = React.useState<Record<number, string>>({});
  const [roundExpansion, setRoundExpansion] = React.useState<Record<number, boolean>>({});
  const roundRefs = React.useRef(new Map<number, HTMLDivElement>());

  // The debug switch is a viewer preference, so it survives messages and reloads.
  React.useEffect(() => {
    try {
      setDebug(window.localStorage.getItem(DISCUSSION_DEBUG_STORAGE_KEY) === "1");
    } catch {
      setDebug(false);
    }
  }, []);
  const toggleDebug = () => {
    setDebug((value) => {
      const next = !value;
      try {
        window.localStorage.setItem(DISCUSSION_DEBUG_STORAGE_KEY, next ? "1" : "0");
      } catch {
        // Storage can be unavailable (private mode); the toggle still works for this view.
      }
      return next;
    });
  };

  const turns = React.useMemo(() => discussion?.turns ?? [], [discussion?.turns]);
  const placeholders = React.useMemo(() => discussion?.placeholders ?? [], [discussion?.placeholders]);
  // Bound models live on the runtime rounds; placeholders supply the names shown in the panel.
  const modelsByParticipant = React.useMemo(() => {
    const bound = new Map<string, string>();
    for (const round of discussion?.rounds ?? []) {
      for (const step of round.steps) {
        if (step.model && !bound.has(step.participant)) {
          bound.set(step.participant, step.model);
        }
      }
    }
    for (const turn of turns) {
      if (turn.model && !bound.has(turn.participant)) {
        bound.set(turn.participant, turn.model);
      }
    }
    return bound;
  }, [discussion?.rounds, turns]);
  // A resumed stream can lose discussion_start to the retention window: derive rounds from turns.
  const displayRounds = React.useMemo<DiscussionRuntimeRound[]>(() => {
    if (discussion?.rounds?.length) {
      return discussion.rounds;
    }
    const byRound = new Map<number, DiscussionRuntimeRound["steps"]>();
    for (const turn of turns) {
      const steps = byRound.get(turn.round) ?? [];
      if (!steps.some((step) => step.id === turn.stepId)) {
        steps.push({ id: turn.stepId, participant: turn.participant, model: turn.model, prompt: "" });
      }
      byRound.set(turn.round, steps);
    }
    return [...byRound.entries()]
      .sort((left, right) => left[0] - right[0])
      .map(([index, steps]) => ({ index, steps }));
  }, [discussion?.rounds, turns]);
  const labels = React.useMemo(
    () => discussionPlaceholderLabels(placeholders, discussion?.summary?.participant ?? ""),
    [discussion?.summary?.participant, placeholders],
  );
  const speakerName = React.useCallback(
    (participant: string, model?: string) => labels.get(participant) || model?.trim() || participant,
    [labels],
  );
  // Discussion models always read 模型 1 … 模型 n, whatever order a round stored its steps in.
  const placeholderOrder = React.useMemo(() => {
    const order = new Map<string, number>();
    orderDiscussionPlaceholders(placeholders, discussion?.summary?.participant ?? "").forEach((item, index) => {
      order.set(item.key, index);
    });
    return order;
  }, [discussion?.summary?.participant, placeholders]);
  const orderedSteps = React.useCallback(
    (round: NonNullable<DiscussionState["rounds"]>[number]) =>
      [...round.steps].sort(
        (left, right) =>
          (placeholderOrder.get(left.participant) ?? Number.MAX_SAFE_INTEGER) -
          (placeholderOrder.get(right.participant) ?? Number.MAX_SAFE_INTEGER),
      ),
    [placeholderOrder],
  );
  const turnOf = React.useCallback(
    (round: number, stepId: string, participant: string) =>
      turns.find(
        (turn) =>
          turn.round === round && (turn.stepId === stepId || (!turn.stepId && turn.participant === participant)),
      ),
    [turns],
  );

  if (!discussion?.enabled) {
    return null;
  }

  const status = discussion.status || "running";
  const hasSummary = Boolean(discussion.finalContent?.trim());
  const running = status === "running" || status === "summarizing";
  const statusLabel =
    status === "completed" || status === "completed_with_errors"
      ? t("statusCompleted")
      : status === "stopped"
        ? t("statusStopped")
        : status === "error"
          ? t("statusError")
          : status === "summarizing"
            ? t("statusSummarizing")
            : t("statusRunning");
  const participantCount = Math.max(
    new Set(modelsByParticipant.keys()).size,
    Math.max(labels.size - 1, 0),
  );

  const scrollToRound = (round: number) => {
    roundRefs.current.get(round)?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  };
  const showRoundModel = (round: number, participant: string) => {
    setSelectedByRound((current) => ({ ...current, [round]: participant }));
    setRoundExpansion((current) => ({ ...current, [round]: true }));
    scrollToRound(round);
  };
  const toggleRound = (round: number, expanded: boolean) => {
    setRoundExpansion((current) => ({ ...current, [round]: !expanded }));
  };
  const resetSelection = () => {
    setSelectedByRound({});
    const first = displayRounds[0]?.index;
    if (first !== undefined) {
      scrollToRound(first);
    }
  };

  return (
    <div className="my-3 overflow-hidden rounded-xl border border-border/60 bg-muted/20 text-sm">
      <div className="flex items-start justify-between gap-3 px-3 py-2.5">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <div className="flex items-center gap-1.5 font-medium text-foreground">
              <Users className="size-4 text-primary" />
              <span>{discussion.templateName || t("title")}</span>
            </div>
            <span className="inline-flex items-center gap-1.5 text-[11px] text-muted-foreground">
              <span className={cn("size-1.5 rounded-full", statusClass(status))} />
              {running ? <Loader2 className="size-3 animate-spin" /> : null}
              {statusLabel}
            </span>
          </div>
          <div className="mt-1.5 flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
            <span className="rounded-full bg-muted px-2 py-0.5">
              {t("rounds", { count: displayRounds.length })}
            </span>
            <span className="rounded-full bg-muted px-2 py-0.5">
              {t("participants", { count: participantCount })}
            </span>
            {discussion.summary?.model ? (
              <span className="rounded-full bg-muted px-2 py-0.5">
                {t("summaryModel", { model: discussion.summary.model })}
              </span>
            ) : null}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          <button
            type="button"
            aria-pressed={debug}
            className={cn(
              "inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-xs transition-colors",
              debug ? "bg-primary/10 text-primary" : "text-muted-foreground hover:text-foreground",
            )}
            onClick={toggleDebug}
            title={t("debug")}
          >
            <Bug className="size-3.5" />
            {t("debug")}
          </button>
          <button
            type="button"
            className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
            onClick={() => setExpanded((value) => !value)}
          >
            {expanded ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
            {expanded ? t("collapse") : t("expand")}
          </button>
        </div>
      </div>

      {/* Rounds stream expanded, then keep only their headers once the final answer arrives. */}
      {displayRounds.map((round) => {
        const steps = orderedSteps(round);
        const tone = toneFor(round.index);
        const turnsOfRound = steps
          .map((step) => ({ step, turn: turnOf(round.index, step.id, step.participant) }))
          .filter((item) => item.turn);
        const preferred = selectedByRound[round.index];
        const active =
          steps.find((step) => step.participant === preferred) ??
          turnsOfRound.find((item) => item.turn?.status === "running")?.step ??
          turnsOfRound.find((item) => item.turn?.content)?.step ??
          steps[0];
        const activeTurn = active ? turnOf(round.index, active.id, active.participant) : undefined;
        const roundStatuses = steps.map(
          (step) => turnOf(round.index, step.id, step.participant)?.status ?? "pending",
        );
        const roundStatus = roundStatuses.some((item) => item === "running")
          ? "running"
          : roundStatuses.every((item) => item === "error")
            ? "failed"
            : roundStatuses.every((item) => item === "completed" || item === "error")
              ? "done"
              : "pending";
        const roundExpanded = roundExpansion[round.index] ?? !hasSummary;
        return (
          <div
            key={`round-speech-${round.index}`}
            ref={(node) => {
              if (node) {
                roundRefs.current.set(round.index, node);
              } else {
                roundRefs.current.delete(round.index);
              }
            }}
            className="border-t border-border/50 px-3 py-2.5"
          >
            <div className={cn("border-l-2 pl-2.5", tone.accent)}>
              <div className={cn("flex flex-wrap items-center gap-x-2 gap-y-1", roundExpanded && "mb-1.5")}>
                <button
                  type="button"
                  aria-expanded={roundExpanded}
                  className="flex items-center gap-2 rounded-md text-left"
                  title={roundExpanded ? t("collapseRound") : t("expandRound")}
                  onClick={() => toggleRound(round.index, roundExpanded)}
                >
                  {roundExpanded ? (
                    <ChevronDown className="size-3.5 text-muted-foreground" />
                  ) : (
                    <ChevronRight className="size-3.5 text-muted-foreground" />
                  )}
                  <span className={cn("rounded-full px-2 py-0.5 text-xs font-medium", tone.badge)}>
                    {t("round", { round: round.index })}
                  </span>
                  <span className="text-[10px] text-muted-foreground">
                    {roundStatus === "running"
                      ? t("thinking")
                      : roundStatus === "failed"
                        ? t("failed")
                        : roundStatus === "done"
                          ? t("done")
                          : t("pendingTurn")}
                  </span>
                </button>
                <div className="ml-auto flex flex-wrap items-center justify-end gap-1">
                  {steps.map((step) => {
                    const turn = turnOf(round.index, step.id, step.participant);
                    const isActive = active?.participant === step.participant;
                    return (
                      <button
                        key={`round-${round.index}-${step.id}`}
                        type="button"
                        className={cn(
                          "inline-flex max-w-56 items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] transition-colors",
                          isActive
                            ? "border-primary/50 bg-primary/10 text-primary"
                            : "border-border/60 text-muted-foreground hover:text-foreground",
                        )}
                        title={step.model}
                        onClick={() => showRoundModel(round.index, step.participant)}
                      >
                        <span className="truncate">{speakerName(step.participant, step.model)}</span>
                        <span className="truncate font-mono text-[10px] opacity-70">{step.model || "—"}</span>
                        <span className="shrink-0">{usageStatusMark(turn?.status ?? "pending")}</span>
                      </button>
                    );
                  })}
                </div>
              </div>
              {roundExpanded ? (
                <>
                  {activeTurn?.error ? (
                    <p className="whitespace-pre-wrap break-words text-xs leading-6 text-rose-600 dark:text-rose-300">
                      {activeTurn.error}
                    </p>
                  ) : activeTurn?.content ? (
                    <div className="text-sm leading-7 text-foreground/90">
                      <StreamdownRender content={activeTurn.content} streaming={activeTurn.status === "running"} />
                    </div>
                  ) : (
                    <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
                      {activeTurn?.status === "running" ? <Loader2 className="size-3 animate-spin" /> : null}
                      {activeTurn?.status === "running" ? t("thinking") : t("pendingTurn")}
                    </p>
                  )}
                  {debug && active ? (
                    <DebugPrompt
                      model={active.model}
                      prompt={activeTurn?.prompt}
                      label={t("debugPromptFor", { model: active.model })}
                    />
                  ) : null}
                </>
              ) : null}
            </div>
          </div>
        );
      })}

      <div className="border-t border-border/50 px-3 py-2.5">
        <div className={cn("border-l-2 pl-2.5", SUMMARY_TONE.accent)}>
          <div className="flex flex-wrap items-center gap-2 text-[11px]">
            <span className={cn("rounded-full px-2 py-0.5 text-xs font-medium", SUMMARY_TONE.badge)}>
              {t("summaryBadge")}
            </span>
            <span className="truncate font-mono text-muted-foreground">{discussion.summary?.model || "—"}</span>
            <span className="ml-auto shrink-0 text-muted-foreground">
              {discussion.finalContent ? t("done") : status === "summarizing" ? t("statusSummarizing") : t("pendingTurn")}
            </span>
          </div>
          {debug ? (
            <DebugPrompt
              model={discussion.summary?.model ?? ""}
              prompt={discussion.summaryPrompt}
              label={t("debugSummaryPrompt")}
            />
          ) : null}
        </div>
      </div>

      {expanded ? (
        <div className="border-t border-border/50 px-3 py-2.5">
          <div className="mb-2 flex flex-wrap gap-1.5">
            <button
              type="button"
              className="rounded-full border border-primary/50 bg-primary/10 px-3 py-1.5 text-xs text-primary transition-colors"
              onClick={resetSelection}
            >
              {t("all")}
            </button>
          </div>

          {/* One row per round, one cell per model: clicking a cell opens that model's turn. */}
          <p className="mb-1 text-[10px] font-medium text-muted-foreground">{t("roundUsage")}</p>
          <div className="mb-2 space-y-1">
            {displayRounds.map((round) => {
              const tone = toneFor(round.index);
              return (
                <div key={`usage-${round.index}`} className="rounded-lg border border-border/50 bg-background/40 p-1.5">
                  <p className={cn("mb-1 text-[10px] font-medium", tone.title)}>
                    {t("round", { round: round.index })}
                  </p>
                  <div className="flex flex-wrap gap-1">
                    {orderedSteps(round).map((step) => {
                      const turn = turnOf(round.index, step.id, step.participant);
                      const turnStatus = turn?.status ?? "pending";
                      const isActive = selectedByRound[round.index] === step.participant;
                      return (
                        <button
                          key={`usage-${round.index}-${step.id}`}
                          type="button"
                          className={cn(
                            "flex min-w-36 flex-1 flex-col gap-0.5 rounded-md border px-1.5 py-1 text-left transition-colors",
                            usageCellClass(turnStatus),
                            isActive && "ring-1 ring-primary/50",
                          )}
                          onClick={() => showRoundModel(round.index, step.participant)}
                        >
                          <div className="flex min-w-0 items-center gap-1">
                            <span className="truncate text-[11px] font-medium text-foreground">
                              {speakerName(step.participant, step.model)}
                            </span>
                            <span className="truncate font-mono text-[10px] text-muted-foreground">
                              {step.model || "—"}
                            </span>
                            <span className="ml-auto shrink-0">{usageStatusMark(turnStatus)}</span>
                          </div>
                          <span className="tabular-nums text-[10px] text-muted-foreground">
                            {turn
                              ? usageText((input, output, seconds) => t("usage", { input, output, seconds }), turn)
                              : t("pendingTurn")}
                          </span>
                        </button>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      ) : null}
    </div>
  );
}
