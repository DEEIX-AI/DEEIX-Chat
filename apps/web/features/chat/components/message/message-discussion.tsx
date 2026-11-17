"use client";

import * as React from "react";
import { ChevronDown, ChevronRight, Loader2, Users } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";
import type { DiscussionState, DiscussionTurn } from "@/shared/api/discussion-types";
import {
  discussionSpeakerLabel,
  orderDiscussionPlaceholders,
} from "@/shared/lib/discussion";

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
    return bound;
  }, [discussion?.rounds]);
  const participants = React.useMemo(() => {
    // The summary model is a placeholder too, but it never speaks inside a round: only speakers get a tab.
    const speakers = new Set(modelsByParticipant.keys());
    for (const turn of turns) {
      speakers.add(turn.participant);
    }
    const ordered = orderDiscussionPlaceholders(placeholders, discussion?.summary?.participant ?? "");
    const names = ordered
      .filter((placeholder) => speakers.has(placeholder.key))
      .map((placeholder) => ({
        key: placeholder.key,
        label: discussionSpeakerLabel(placeholders, placeholder.key, modelsByParticipant.get(placeholder.key)),
        model: modelsByParticipant.get(placeholder.key) ?? placeholder.defaultModel ?? "",
      }));
    const named = new Set(names.map((item) => item.key));
    for (const [key, model] of modelsByParticipant) {
      if (!named.has(key)) {
        names.push({ key, label: model || key, model });
      }
    }
    return names;
  }, [discussion?.summary?.participant, modelsByParticipant, placeholders, turns]);

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
                title={participant.model || undefined}
              >
                {participant.label}
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
                      {t("round", { round: turn.round })} ·{" "}
                      {discussionSpeakerLabel(placeholders, turn.participant, turn.model)}
                    </span>
                    {turn.model ? <span className="font-mono text-[10px]">{turn.model}</span> : null}
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
