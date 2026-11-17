"use client";

import * as React from "react";
import { ChevronDown, ChevronRight, Loader2, Users } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";
import type { DiscussionState, DiscussionTurn } from "@/shared/api/discussion-types";

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

function turnStatusClass(status: string): string {
  switch (status) {
    case "error":
      return "border-rose-200 bg-rose-50/60 dark:border-rose-900/60 dark:bg-rose-950/20";
    case "running":
      return "border-sky-200 bg-sky-50/60 dark:border-sky-900/60 dark:bg-sky-950/20";
    default:
      return "border-border/60 bg-background/60";
  }
}

export function MessageDiscussion({ discussion }: { discussion?: DiscussionState }) {
  const t = useTranslations("chat.discussion");
  const [expanded, setExpanded] = React.useState(false);
  const [selected, setSelected] = React.useState<string>("all");

  const turns = React.useMemo(() => discussion?.turns ?? [], [discussion?.turns]);
  const participants = React.useMemo(() => {
    const seen = new Map<string, string>();
    for (const round of discussion?.rounds ?? []) {
      for (const step of round.steps) {
        if (!seen.has(step.participant)) {
          seen.set(step.participant, step.model);
        }
      }
    }
    return Array.from(seen.entries()).map(([key, model]) => ({ key, model }));
  }, [discussion?.rounds]);

  if (!discussion?.enabled) {
    return null;
  }

  const status = discussion.status || "running";
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
  const visibleTurns: DiscussionTurn[] =
    selected === "all" ? turns : turns.filter((turn) => turn.participant === selected);

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
              {t("rounds", { count: discussion.rounds.length })}
            </span>
            <span className="rounded-full bg-muted px-2 py-0.5">
              {t("participants", { count: participants.length })}
            </span>
            {discussion.summary?.model ? (
              <span className="rounded-full bg-muted px-2 py-0.5">
                {t("summaryModel", { model: discussion.summary.model })}
              </span>
            ) : null}
          </div>
        </div>
        <button
          type="button"
          className="inline-flex shrink-0 items-center gap-1 rounded-md px-1.5 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
          {expanded ? t("collapse") : t("expand")}
        </button>
      </div>

      {expanded ? (
        <div className="border-t border-border/50 px-3 py-2.5">
          <div className="mb-2 flex flex-wrap gap-1.5">
            <button
              type="button"
              className={cn(
                "rounded-full border px-2.5 py-1 text-[11px] transition-colors",
                selected === "all"
                  ? "border-primary/50 bg-primary/10 text-primary"
                  : "border-border/60 text-muted-foreground hover:text-foreground",
              )}
              onClick={() => setSelected("all")}
            >
              {t("all")}
            </button>
            {participants.map((participant) => (
              <button
                key={participant.key}
                type="button"
                className={cn(
                  "rounded-full border px-2.5 py-1 text-[11px] transition-colors",
                  selected === participant.key
                    ? "border-primary/50 bg-primary/10 text-primary"
                    : "border-border/60 text-muted-foreground hover:text-foreground",
                )}
                onClick={() => setSelected(participant.key)}
              >
                {participant.model}
              </button>
            ))}
          </div>

          <div className="max-h-[420px] space-y-2 overflow-y-auto pr-1">
            {visibleTurns.length === 0 ? (
              <p className="py-3 text-xs text-muted-foreground">{t("empty")}</p>
            ) : (
              visibleTurns.map((turn, index) => (
                <div
                  key={`${turn.round}-${turn.stepId}-${index}`}
                  className={cn("rounded-lg border px-2.5 py-2", turnStatusClass(turn.status))}
                >
                  <div className="flex flex-wrap items-center justify-between gap-2 text-[11px] text-muted-foreground">
                    <span className="font-medium text-foreground">
                      {t("round", { round: turn.round })} · {turn.participant}
                    </span>
                    <span>{turn.model}</span>
                    <span>
                      {turn.status === "running" ? t("thinking") : turn.status === "error" ? t("failed") : t("done")}
                      {turn.outputTokens ? ` · ${turn.outputTokens} tokens` : ""}
                      {turn.latencyMs ? ` · ${Math.round(turn.latencyMs / 100) / 10}s` : ""}
                    </span>
                  </div>
                  <div className="mt-1 whitespace-pre-wrap break-words text-xs leading-6 text-foreground/90">
                    {turn.error || turn.content || "…"}
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      ) : null}
    </div>
  );
}
