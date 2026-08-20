"use client";

import { BookOpen, Check, Search } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { Input } from "@/components/ui/input";
import { InputGroupButton } from "@/components/ui/input-group";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Spinner } from "@/components/ui/spinner";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { listAllVisibleKnowledgeBases } from "@/shared/api/knowledge-bases";
import type { KnowledgeBaseDTO } from "@/shared/api/knowledge-bases.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

const MAX_SELECTED_KNOWLEDGE_BASES = 8;

export function ChatKnowledgeBases({
  selectedIDs,
  disabled,
  available,
  unavailableReason,
  onChange,
}: {
  selectedIDs: string[];
  disabled: boolean;
  available: boolean | null;
  unavailableReason: string;
  onChange: (ids: string[]) => void;
}) {
  const t = useTranslations("chat.composer");
  const [open, setOpen] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const [items, setItems] = React.useState<KnowledgeBaseDTO[]>([]);
  const [query, setQuery] = React.useState("");
  const mountedRef = React.useRef(true);
  const openRef = React.useRef(open);
  const loadingRef = React.useRef(false);
  const loadedRef = React.useRef(false);
  const selectedIDsRef = React.useRef(selectedIDs);
  const onChangeRef = React.useRef(onChange);
  const translationRef = React.useRef(t);

  openRef.current = open;
  selectedIDsRef.current = selectedIDs;
  onChangeRef.current = onChange;
  translationRef.current = t;

  const loadCatalog = React.useCallback(async (force = false) => {
    if (loadingRef.current || (loadedRef.current && !force)) return;
    loadingRef.current = true;
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) throw new Error("missing access token");
      const results = await listAllVisibleKnowledgeBases(token);
      if (!mountedRef.current) return;
      loadedRef.current = true;
      setItems(results);

      const readyIDs = new Set(
        results.filter((item) => item.readyFileCount > 0).map((item) => item.publicID),
      );
      const currentIDs = selectedIDsRef.current;
      const nextIDs = currentIDs.filter((id) => readyIDs.has(id));
      if (nextIDs.length !== currentIDs.length) onChangeRef.current(nextIDs);
    } catch {
      if (mountedRef.current && openRef.current) {
        toast.error(translationRef.current("knowledgeBaseLoadFailed"));
      }
    } finally {
      if (mountedRef.current) setLoading(false);
      loadingRef.current = false;
    }
  }, []);

  React.useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  React.useEffect(() => {
    if (selectedIDs.length > 0) void loadCatalog();
  }, [loadCatalog, selectedIDs.length]);

  React.useEffect(() => {
    if (available === false && selectedIDs.length > 0) {
      onChange([]);
    }
  }, [available, onChange, selectedIDs.length]);

  const selectedSet = React.useMemo(() => new Set(selectedIDs), [selectedIDs]);
  const normalizedQuery = query.trim().toLowerCase();
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

  return (
    <Popover open={open} onOpenChange={(nextOpen) => {
      setOpen(nextOpen);
      openRef.current = nextOpen;
      if (nextOpen) void loadCatalog(true);
      if (!nextOpen) {
        setQuery("");
      }
    }}>
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

      <PopoverContent side="bottom" align="start" sideOffset={8} className="w-[min(22rem,calc(100vw-2rem))] p-1.5">
        <div className="flex items-center justify-between gap-3 px-2 pb-1.5 text-[11px] font-medium text-foreground/70">
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
          <p className="mb-1.5 rounded-md bg-muted/55 px-2 py-2 text-[11px] leading-4 text-muted-foreground">
            {unavailableDescription}
          </p>
        ) : null}
        <div className="relative mx-1 mb-1.5">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" strokeWidth={1.7} />
          <Input
            value={query}
            placeholder={t("searchKnowledgeBases")}
            className="h-8 pl-8 text-xs"
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>
        <div className="max-h-64 space-y-0.5 overflow-y-auto">
          {loading ? (
            <div className="flex items-center justify-center py-8"><Spinner className="size-4" /></div>
          ) : filteredItems.length > 0 ? filteredItems.map((item) => {
            const selected = selectedSet.has(item.publicID);
            const ready = item.readyFileCount > 0;
            return (
              <button
                key={item.publicID}
                type="button"
                className="flex min-h-10 w-full items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-45"
                disabled={available === false || (!ready && !selected)}
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
                <BookOpen className="size-4 shrink-0 text-muted-foreground" strokeWidth={1.6} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-xs font-medium">{item.name}</span>
                  <span className="block truncate text-[11px] text-muted-foreground">
                    {ready
                      ? t("knowledgeBaseReadyFiles", { count: item.readyFileCount })
                      : t("knowledgeBaseNotReady")}
                  </span>
                </span>
                <Check className={cn("size-3.5 shrink-0", selected ? "opacity-100" : "opacity-0")} strokeWidth={1.8} />
              </button>
            );
          }) : (
            <p className="px-2 py-8 text-center text-xs text-muted-foreground">{t("knowledgeBaseEmpty")}</p>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
