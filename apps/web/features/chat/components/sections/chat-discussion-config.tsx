"use client";

import * as React from "react";
import { Users } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";
import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import type { DiscussionTemplateDTO } from "@/shared/api/discussion-types";

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
        <div className="ml-auto flex flex-wrap items-center gap-2">
          {template.config.placeholders.map((placeholder) => (
            <label key={placeholder.key} className="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
              <span className="max-w-24 truncate">{placeholder.label || placeholder.key}</span>
              <select
                className={cn(
                  "h-7 max-w-44 rounded-md border border-border/70 bg-background px-1.5 text-[11px] text-foreground outline-none focus-visible:border-primary disabled:opacity-60",
                  !bindings[placeholder.key] && "border-rose-400",
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
            </label>
          ))}
        </div>
      </div>
    </div>
  );
}
