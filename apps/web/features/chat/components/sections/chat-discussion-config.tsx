"use client";

import * as React from "react";
import { Check, Loader2, Power, RefreshCw, Undo2, Users } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";
import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import type { DiscussionPlaceholder, DiscussionTemplateDTO } from "@/shared/api/discussion-types";
import {
  canDisableDiscussionModel,
  discussionBindingsComplete,
  discussionPlaceholderLabels,
  orderDiscussionPlaceholders,
} from "@/shared/lib/discussion";

export function ChatDiscussionConfig({
  template,
  modelOptions,
  bindings,
  onChange,
  disabled,
  disabledModels = [],
  onDisabledChange,
  bindingsLocked = false,
  pending = false,
  snapshotBusy = false,
  snapshotError = "",
  snapshotNotice = "",
  canAdopt = false,
  refreshConfirmationPending = false,
  onRefresh,
  onCancelRefresh,
  onRollback,
  onAdopt,
  onUseDefaults,
}: {
  template: DiscussionTemplateDTO;
  modelOptions: ChatModelOption[];
  bindings: Record<string, string>;
  onChange: (participant: string, model: string) => void;
  disabled?: boolean;
  disabledModels?: string[];
  onDisabledChange?: (participant: string, disabled: boolean) => void;
  bindingsLocked?: boolean;
  pending?: boolean;
  snapshotBusy?: boolean;
  snapshotError?: string;
  snapshotNotice?: string;
  canAdopt?: boolean;
  refreshConfirmationPending?: boolean;
  onRefresh?: () => Promise<void>;
  onCancelRefresh?: () => void;
  onRollback?: () => Promise<void>;
  onAdopt?: () => Promise<void>;
  onUseDefaults?: () => void;
}) {
  const t = useTranslations("chat.discussion");
  const candidates = React.useMemo(
    () => modelOptions.filter((item) => !item.kinds.includes("discussion")),
    [modelOptions],
  );
  const disabledSet = React.useMemo(() => new Set(disabledModels), [disabledModels]);
  const missing = !discussionBindingsComplete(template.config, bindings, disabledModels);
  const summaryKey = template.config.summary.participant;
  const ordered = orderDiscussionPlaceholders(template.config.placeholders, summaryKey);
  const summaryPlaceholder = ordered.find((item) => item.key === summaryKey);
  const discussionPlaceholders = ordered.filter((item) => item.key !== summaryKey);
  const labels = discussionPlaceholderLabels(template.config.placeholders, summaryKey);

  const renderSelect = (placeholder: DiscussionPlaceholder, className?: string, modelDisabled = false) => {
    const value = bindings[placeholder.key] ?? "";
    const valueMissing = value && !candidates.some((item) => item.platformModelName === value);
    return (
      <select
        aria-label={labels.get(placeholder.key) ?? placeholder.key}
        className={cn(
          "h-7 w-[min(11rem,50%)] shrink-0 rounded-md border border-border/70 bg-background px-1.5 text-[11px] text-foreground outline-none focus-visible:border-primary disabled:opacity-60",
          !value && !modelDisabled && "border-rose-400",
          modelDisabled && "text-muted-foreground line-through",
          className,
        )}
        value={value}
        disabled={disabled || modelDisabled || bindingsLocked}
        onChange={(event) => onChange(placeholder.key, event.target.value)}
      >
        <option value="">{t("chooseModel")}</option>
        {valueMissing ? <option value={value}>{value}</option> : null}
        {candidates.map((item) => (
          <option key={item.platformModelName} value={item.platformModelName}>
            {item.platformModelName}
          </option>
        ))}
      </select>
    );
  };

  const actionButton = (
    label: string,
    icon: React.ReactNode,
    action?: () => Promise<void>,
    danger = false,
  ) => action ? (
    <button
      type="button"
      className={cn(
        "inline-flex h-6 items-center gap-1 rounded-md border border-border/60 px-2 text-[10px] text-muted-foreground hover:bg-background hover:text-foreground disabled:opacity-50",
        danger && "text-amber-600 dark:text-amber-300",
      )}
      disabled={disabled || snapshotBusy}
      onClick={() => void action()}
    >
      {snapshotBusy ? <Loader2 className="size-3 animate-spin" /> : icon}
      {label}
    </button>
  ) : null;

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
        {snapshotNotice ? <span className={cn("text-[10px]", pending ? "text-amber-600 dark:text-amber-300" : "text-emerald-600 dark:text-emerald-300")}>{snapshotNotice}</span> : null}
        {pending ? (
          <span className="rounded-full bg-amber-500/10 px-2 py-0.5 text-[10px] text-amber-700 dark:text-amber-300">
            {t("pendingBadge")}
          </span>
        ) : bindingsLocked ? (
          <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] text-primary">{t("activeBadge")}</span>
        ) : null}
        {missing ? (
          <span className="rounded-full bg-rose-500/10 px-2 py-0.5 text-[11px] text-rose-600 dark:text-rose-300">
            {t("selectAllModels")}
          </span>
        ) : null}
        <span className="ml-auto flex flex-wrap gap-1">
          {!bindingsLocked && onUseDefaults ? (
            <button
              type="button"
              className="inline-flex h-6 items-center gap-1 rounded-md border border-border/60 px-2 text-[10px] text-muted-foreground hover:bg-background hover:text-foreground disabled:opacity-50"
              disabled={disabled || snapshotBusy}
              onClick={onUseDefaults}
            >
              <Undo2 className="size-3" />
              {t("useDefaultModels")}
            </button>
          ) : null}
          {actionButton(t(refreshConfirmationPending ? "confirmRefresh" : "refreshConfiguration"), <RefreshCw className="size-3" />, onRefresh)}
          {refreshConfirmationPending ? (
            <button type="button" className="h-6 rounded-md px-2 text-[10px] text-muted-foreground hover:text-foreground" onClick={onCancelRefresh}>
              {t("cancelRefresh")}
            </button>
          ) : null}
          {pending ? actionButton(t("rollbackConfiguration"), <Undo2 className="size-3" />, onRollback, true) : null}
          {pending && canAdopt ? actionButton(t("adoptConfiguration"), <Check className="size-3" />, onAdopt) : null}
        </span>
      </div>
      {snapshotError ? <p className="mt-1 text-[10px] text-rose-600 dark:text-rose-300">{snapshotError}</p> : null}
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
      <div className="mt-1.5 flex flex-wrap gap-1.5">
        {discussionPlaceholders.map((placeholder) => {
          const modelDisabled = disabledSet.has(placeholder.key);
          const canDisable = canDisableDiscussionModel(template.config, placeholder.key, disabledModels);
          return (
            <div
              key={placeholder.key}
              className={cn(
                "flex min-w-[13rem] flex-1 items-center gap-2 rounded-md border border-border/50 bg-background/40 px-2 py-1",
                modelDisabled && "opacity-60",
              )}
            >
              <button
                type="button"
                aria-label={modelDisabled ? t("enableModel") : t("disableModel")}
                aria-pressed={!modelDisabled}
                title={modelDisabled ? t("enableModel") : canDisable ? t("disableModel") : t("lastModelRequired")}
                className="shrink-0 rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
                disabled={disabled || (!modelDisabled && !canDisable)}
                onClick={() => onDisabledChange?.(placeholder.key, !modelDisabled)}
              >
                <Power className={cn("size-3.5", !modelDisabled && "text-emerald-600 dark:text-emerald-400")} />
              </button>
              <span className={cn("min-w-0 flex-1 truncate text-[11px] text-muted-foreground", modelDisabled && "line-through")}>
                {labels.get(placeholder.key) ?? placeholder.key}
              </span>
              {renderSelect(placeholder, undefined, modelDisabled)}
            </div>
          );
        })}
      </div>
    </div>
  );
}
