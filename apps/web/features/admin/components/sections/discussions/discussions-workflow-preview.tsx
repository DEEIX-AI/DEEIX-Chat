"use client";

import * as React from "react";
import { ArrowDown, Workflow, X } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";
import type { DiscussionTemplateConfig } from "@/shared/api/discussion-types";
import { discussionPlaceholderLabels } from "@/shared/lib/discussion";

function Connector() {
  return (
    <div className="flex justify-center py-0.5">
      <ArrowDown className="size-3 text-muted-foreground/60" strokeWidth={1.6} />
    </div>
  );
}

/**
 * Read-only shape of the template: question -> rounds -> summary. It follows the editor state, so
 * dragging a round or renaming a placeholder is visible here without saving first.
 */
export function DiscussionsWorkflowPreview({
  config,
  onClose,
  className,
}: {
  config: DiscussionTemplateConfig;
  onClose?: () => void;
  className?: string;
}) {
  const t = useTranslations("adminDiscussions");
  const summaryModel = config.placeholders.find((item) => item.key === config.summary.participant)?.defaultModel;
  const labels = React.useMemo(
    () => discussionPlaceholderLabels(config.placeholders, config.summary.participant),
    [config.placeholders, config.summary.participant],
  );

  return (
    <div className={cn("rounded-xl border border-border/60 bg-background p-3", className)}>
      <h3 className="mb-2 flex items-center gap-1.5 text-sm font-semibold">
        <Workflow className="size-4 text-primary" />
        {t("workflow")}
        {onClose ? (
          <button
            type="button"
            className="ml-auto rounded-md p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label={t("close")}
            onClick={onClose}
          >
            <X className="size-3.5" />
          </button>
        ) : null}
      </h3>
      <div className="rounded-lg border border-border/50 bg-muted/30 px-2 py-1.5 text-xs text-foreground">
        {t("workflowQuestion")}
      </div>
      <Connector />
      {config.rounds.map((round, roundIndex) => (
        <React.Fragment key={`preview-round-${roundIndex}`}>
          <div className="rounded-lg border border-border/60 bg-background px-2 py-1.5">
            <div className="flex items-center justify-between gap-2 text-xs font-medium text-foreground">
              <span>{t("round", { round: roundIndex + 1 })}</span>
              <span className="rounded-full bg-muted px-1.5 text-[10px] font-normal text-muted-foreground">
                {round.parallel ? t("parallel") : t("serial")}
              </span>
            </div>
            <div className="mt-1 space-y-0.5">
              {round.steps.map((step) => {
                const placeholder = config.placeholders.find((item) => item.key === step.participant);
                return (
                  <div key={step.id} className="flex min-w-0 items-baseline gap-1.5 text-[11px]">
                    <span className="min-w-0 truncate text-foreground">
                      {labels.get(step.participant) ?? step.participant}
                    </span>
                    <span className="shrink-0 truncate font-mono text-[10px] text-muted-foreground">
                      {placeholder?.defaultModel || t("chooseAtChat")}
                    </span>
                  </div>
                );
              })}
              {round.steps.length === 0 ? (
                <p className="text-[11px] text-muted-foreground">—</p>
              ) : null}
            </div>
            {roundIndex > 0 ? (
              <p className="mt-1 text-[10px] leading-4 text-muted-foreground">{t("roundSeesPrevious")}</p>
            ) : null}
          </div>
          <Connector />
        </React.Fragment>
      ))}
      <div
        className={cn(
          "rounded-lg border border-primary/40 bg-primary/5 px-2 py-1.5 text-xs",
          config.rounds.length === 0 && "mt-0",
        )}
      >
        <div className="font-medium text-foreground">{t("workflowSummary")}</div>
        <div className="mt-0.5 flex min-w-0 items-baseline gap-1.5 text-[11px]">
          <span className="min-w-0 truncate">
            {labels.get(config.summary.participant) ?? config.summary.participant}
          </span>
          <span className="shrink-0 truncate font-mono text-[10px] text-muted-foreground">
            {summaryModel || t("chooseAtChat")}
          </span>
        </div>
      </div>
      <p className="mt-2 text-[11px] leading-5 text-muted-foreground">{t("workflowHint")}</p>
    </div>
  );
}
