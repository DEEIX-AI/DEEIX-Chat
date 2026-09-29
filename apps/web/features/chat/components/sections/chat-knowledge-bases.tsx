"use client";

import { BookOpen, Check } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { Input } from "@/components/ui/input";
import { InputGroupButton } from "@/components/ui/input-group";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Spinner } from "@/components/ui/spinner";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  MAX_SELECTED_KNOWLEDGE_BASES,
  useChatKnowledgeBaseCatalog,
} from "@/features/chat/hooks/use-chat-knowledge-base-catalog";
import { cn } from "@/lib/utils";
import { useFeaturePolicy } from "@/shared/hooks/use-feature-policy";

export function ChatKnowledgeBases({
  selectedIDs,
  placementPreference,
  disabled,
  available,
  unavailableReason,
  onChange,
}: {
  selectedIDs: string[];
  placementPreference: "top" | "bottom";
  disabled: boolean;
  available: boolean | null;
  unavailableReason: string;
  onChange: (ids: string[]) => void;
}) {
  const t = useTranslations("chat.composer");
  const { knowledgeBaseEnabled } = useFeaturePolicy();
  const { open, handleOpenChange, query, setQuery, items, loading, loadingMore, hasMore, loadMore } =
    useChatKnowledgeBaseCatalog({ selectedIDs, onChange });

  React.useEffect(() => {
    if (available === false && selectedIDs.length > 0) {
      onChange([]);
    }
  }, [available, onChange, selectedIDs.length]);

  const selectedSet = React.useMemo(() => new Set(selectedIDs), [selectedIDs]);
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const filteredItems = normalizedQuery
    ? items.filter((item) => `${item.name} ${item.description}`.toLowerCase().includes(normalizedQuery))
    : items;
  let unavailableDescription = t("knowledgeBaseUnavailableDescription");
  switch (unavailableReason) {
    case "rag_disabled":
      unavailableDescription = t("knowledgeBaseUnavailableRAGDisabled");
      break;
    case "embedding_disabled":
      unavailableDescription = t("knowledgeBaseUnavailableEmbeddingDisabled");
      break;
    case "embedding_host_missing":
      unavailableDescription = t("knowledgeBaseUnavailableEmbeddingHostMissing");
      break;
    case "embedding_model_missing":
      unavailableDescription = t("knowledgeBaseUnavailableEmbeddingModelMissing");
      break;
    case "embedding_client_missing":
      unavailableDescription = t("knowledgeBaseUnavailableEmbeddingClientMissing");
      break;
    case "vector_store_unavailable":
    case "vector_store_error":
      unavailableDescription = t("knowledgeBaseUnavailableVectorStore");
      break;
  }

  if (!knowledgeBaseEnabled) return null;

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <Tooltip>
        <TooltipTrigger asChild>
          <PopoverTrigger asChild>
            <InputGroupButton
              type="button"
              variant="ghost"
              size="icon-sm"
              className={cn(
                "relative size-7 rounded-md text-muted-foreground hover:text-foreground sm:size-8",
                selectedIDs.length > 0 && "bg-primary/10 text-primary hover:bg-primary/10 hover:text-primary",
              )}
              disabled={disabled}
              aria-label={t("knowledgeBases")}
            >
              <BookOpen className="size-4" strokeWidth={1.6} />
              {selectedIDs.length > 0 ? (
                <span className="absolute -right-0.5 -top-0.5 min-w-3 rounded-full bg-primary px-0.5 text-center text-[8px] font-semibold leading-3 text-primary-foreground">
                  {selectedIDs.length}
                </span>
              ) : null}
            </InputGroupButton>
          </PopoverTrigger>
        </TooltipTrigger>
        <TooltipContent side="top" className="text-xs">
          {available === false
            ? t("knowledgeBaseUnavailable")
            : selectedIDs.length > 0
              ? t("knowledgeBasesSelected", { count: selectedIDs.length })
              : t("knowledgeBases")}
        </TooltipContent>
      </Tooltip>

      <PopoverContent
        side={placementPreference}
        align="start"
        sideOffset={8}
        avoidCollisions={false}
        collisionPadding={8}
        data-knowledge-bases-popover-content
        className="flex max-h-[var(--radix-popover-content-available-height)] w-[min(20rem,calc(100vw-1rem))] flex-col p-1.5"
        onPointerDown={(event) => event.stopPropagation()}
        onMouseDown={(event) => event.stopPropagation()}
        onClick={(event) => event.stopPropagation()}
        onPointerDownOutside={(event) => {
          const target = event.target;
          if (target instanceof Element && target.closest("[data-knowledge-bases-popover-content]")) {
            event.preventDefault();
          }
        }}
        onFocusOutside={(event) => {
          const target = event.target;
          if (target instanceof Element && target.closest("[data-knowledge-bases-popover-content]")) {
            event.preventDefault();
          }
        }}
      >
        <div className="flex h-7 shrink-0 items-center justify-between gap-3 px-2 text-[11px] font-medium text-foreground/70">
          <span>{t("knowledgeBases")}</span>
          {selectedIDs.length > 0 ? (
            <button
              type="button"
              className="text-[11px] leading-none text-foreground/55 outline-none transition-colors hover:text-foreground focus-visible:text-foreground"
              onClick={() => onChange([])}
            >
              {t("clear")}
            </button>
          ) : null}
        </div>
        {available === false ? (
          <p className="mx-1 mb-1 shrink-0 rounded-md bg-muted/45 px-2.5 py-2 text-[11px] leading-4 text-muted-foreground dark:bg-muted/35">
            {unavailableDescription}
          </p>
        ) : null}
        <div className="mx-1 mb-1 shrink-0">
          <Input
            value={query}
            placeholder={t("searchKnowledgeBases")}
            className="h-7 border-0 bg-muted/45 px-2.5 text-xs shadow-none dark:bg-muted/35"
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => event.stopPropagation()}
          />
        </div>
        <div className="min-h-0 max-h-72 space-y-0.5 overflow-y-auto px-0.5">
          {loading && items.length === 0 ? (
            <div className="flex items-center justify-center py-6"><Spinner className="size-4" /></div>
          ) : filteredItems.length > 0 ? filteredItems.map((item) => {
            const selected = selectedSet.has(item.publicID);
            const ready = item.readyFileCount > 0;
            return (
              <button
                key={item.publicID}
                type="button"
                data-selected={selected}
                className="flex h-7 w-full items-center gap-1.5 rounded-md px-1.5 text-left text-foreground/80 transition-colors hover:bg-accent hover:text-accent-foreground data-[selected=true]:bg-accent data-[selected=true]:text-accent-foreground disabled:cursor-not-allowed disabled:opacity-45"
                disabled={available === false || (!ready && !selected)}
                aria-pressed={selected}
                onClick={() => {
                  if (selected) {
                    onChange(selectedIDs.filter((id) => id !== item.publicID));
                    return;
                  }
                  if (selectedIDs.length >= MAX_SELECTED_KNOWLEDGE_BASES) {
                    toast.error(t("knowledgeBaseLimit", { limit: MAX_SELECTED_KNOWLEDGE_BASES }));
                    return;
                  }
                  onChange([...selectedIDs, item.publicID]);
                }}
              >
                <span className="flex min-w-0 flex-1 items-center gap-1.5">
                  {selected ? (
                    <Check className="size-3.5 shrink-0 text-primary" strokeWidth={1.8} />
                  ) : (
                    <BookOpen className="size-3.5 shrink-0 text-muted-foreground" strokeWidth={1.6} />
                  )}
                  <span className="truncate text-xs font-medium text-current" title={item.name}>{item.name}</span>
                </span>
                <span className="shrink-0 text-[10px] leading-none tabular-nums text-muted-foreground">
                  {ready
                    ? t("knowledgeBaseReadyFiles", { count: item.readyFileCount })
                    : t("knowledgeBaseNotReady")}
                </span>
              </button>
            );
          }) : (
            <p className="px-2 py-6 text-center text-xs text-muted-foreground">{t("knowledgeBaseEmpty")}</p>
          )}
          {hasMore ? (
            <button
              type="button"
              className="flex h-7 w-full items-center justify-center rounded-md text-[11px] text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:pointer-events-none"
              disabled={loadingMore}
              onClick={loadMore}
            >
              {loadingMore ? <Spinner className="size-3" /> : t("knowledgeBaseLoadMore")}
            </button>
          ) : null}
        </div>
      </PopoverContent>
    </Popover>
  );
}
