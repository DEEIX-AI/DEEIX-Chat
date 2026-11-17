"use client";

import * as React from "react";
import { Users } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";
import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import type { DiscussionPlaceholder, DiscussionTemplateDTO } from "@/shared/api/discussion-types";
import { discussionPlaceholderLabels, orderDiscussionPlaceholders } from "@/shared/lib/discussion";

export function ChatDiscussionConfig({
  template,
  modelOptions,
  bindings,
  onChange,
  disabled,
}: {
  template: DiscussionTemplateDTO;
  modelOptions: ChatModelOption[];
  bindings: Record<string, string>;
  onChange: (participant: string, model: string) => void;
  disabled?: boolean;
}) {
  const t = useTranslations("chat.discussion");
  const candidates = React.useMemo(
    () => modelOptions.filter((item) => !item.kinds.includes("discussion")),
    [modelOptions],
  );
  const missing = template.config.placeholders.some((item) => !bindings[item.key]);
  const summaryKey = template.config.summary.participant;
  const ordered = orderDiscussionPlaceholders(template.config.placeholders, summaryKey);
  const summaryPlaceholder = ordered.find((item) => item.key === summaryKey);
  const discussionPlaceholders = ordered.filter((item) => item.key !== summaryKey);
  const labels = discussionPlaceholderLabels(template.config.placeholders, summaryKey);

  const renderSelect = (placeholder: DiscussionPlaceholder, className?: string) => (
    <select
      aria-label={labels.get(placeholder.key) ?? placeholder.key}
      className={cn(
        "h-7 w-[min(11rem,50%)] shrink-0 rounded-md border border-border/70 bg-background px-1.5 text-[11px] text-foreground outline-none focus-visible:border-primary disabled:opacity-60",
        !bindings[placeholder.key] && "border-rose-400",
        className,
      )}
      value={bindings[placeholder.key] ?? ""}
      disabled={disabled}
      onChange={(event) => onChange(placeholder.key, event.target.value)}
    >
      <option value="">{t("chooseModel")}</option>
      {candidates.map((item) => (
        <option key={item.platformModelName} value={item.platformModelName}>
          {item.platformModelName}
        </option>
      ))}
    </select>
  );

  return (
    <div className="mb-2 rounded-xl border border-border/60 bg-muted/25 px-3 py-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="inline-flex items-center gap-1.5 text-xs font-medium text-foreground">
          <Users className="size-3.5 text-primary" />
          {template.name}
        </span>
        <span className="text-[11px] text-muted-foreground">
          {t("rounds", { count: template.config.rounds.length })}
        </span>
        {missing ? (
          <span className="rounded-full bg-rose-500/10 px-2 py-0.5 text-[11px] text-rose-600 dark:text-rose-300">
            {t("selectAllModels")}
          </span>
        ) : null}
      </div>
      {/* The summary model is the last node of the workflow, so it keeps a row of its own. */}
      {summaryPlaceholder ? (
        <div className="mt-2 flex min-w-0 items-center gap-2 rounded-md border border-primary/30 bg-primary/5 px-2 py-1">
          <span className="flex min-w-0 flex-1 items-center gap-1.5 text-[11px] text-muted-foreground">
            <span className="truncate">{labels.get(summaryPlaceholder.key) ?? summaryPlaceholder.key}</span>
            <span className="shrink-0 rounded-full bg-primary/10 px-1.5 text-[10px] text-primary">
              {t("summaryBadge")}
            </span>
          </span>
          {renderSelect(summaryPlaceholder, "w-[min(14rem,45%)]")}
        </div>
      ) : null}
      {/* Discussion models fill the row and wrap only when they run out of room. */}
      <div className="mt-1.5 flex flex-wrap gap-1.5">
        {discussionPlaceholders.map((placeholder) => (
          <div
            key={placeholder.key}
            className="flex min-w-[13rem] flex-1 items-center gap-2 rounded-md border border-border/50 bg-background/40 px-2 py-1"
          >
            <span className="min-w-0 flex-1 truncate text-[11px] text-muted-foreground">
              {labels.get(placeholder.key) ?? placeholder.key}
            </span>
            {renderSelect(placeholder)}
          </div>
        ))}
      </div>
    </div>
  );
}
