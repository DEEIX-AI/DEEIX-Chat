"use client";

import { BookOpen, Box, ChevronDown, Globe2, type LucideIcon, Search, SlidersHorizontal, Wrench } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogCollapsible,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Spinner } from "@/components/ui/spinner";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { listVisibleKnowledgeBases } from "@/shared/api/knowledge-bases";
import type { KnowledgeBaseDTO } from "@/shared/api/knowledge-bases.types";
import { listAvailableMCPTools } from "@/shared/api/mcp";
import type { MCPToolDTO } from "@/shared/api/mcp.types";
import { getMCPPolicy } from "@/shared/api/settings";
import { listVisibleSkills } from "@/shared/api/skills";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import {
  hasMultipleImageAttachmentProcessors,
  normalizeImageAttachmentProcessorSelection,
} from "@/shared/lib/mcp-tool-selection";

export type ProjectDraft = {
  publicID?: string;
  name: string;
  systemPrompt: string;
  mcpDefaultMode: "inherit" | "custom";
  defaultMCPToolIDs: number[];
  defaultSkillIDs: number[];
  defaultKnowledgeBaseIDs: string[];
};

type ProjectDefaultOption<T extends string | number> = {
  id: T;
  label: string;
  detail: string;
  disabled?: boolean;
};

type ProjectCatalogPage<Item> = {
  results: Item[];
  total: number;
};

type ProjectCatalogListOptions<ID extends string | number> = {
  ids?: ID[];
  query?: string;
  page?: number;
  pageSize?: number;
};

function getSkillID(skill: SkillSummaryDTO): number {
  return skill.id;
}

function getKnowledgeBaseID(knowledgeBase: KnowledgeBaseDTO): string {
  return knowledgeBase.publicID;
}

function usePaginatedProjectCatalog<Item, ID extends string | number>({
  open,
  selectedIDs,
  loadPage,
  getID,
  onSelectedIDsResolved,
  onError,
}: {
  open: boolean;
  selectedIDs: ID[];
  loadPage: (
    accessToken: string,
    options: ProjectCatalogListOptions<ID>,
    signal: AbortSignal,
  ) => Promise<ProjectCatalogPage<Item>>;
  getID: (item: Item) => ID;
  onSelectedIDsResolved: (requestedIDs: ID[], availableIDs: ReadonlySet<ID>) => void;
  onError: () => void;
}) {
  const [items, setItems] = React.useState<Item[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const [page, setPage] = React.useState(1);
  const [total, setTotal] = React.useState(0);
  const requestControllerRef = React.useRef<AbortController | null>(null);
  const selectedIDsRef = React.useRef(selectedIDs);
  selectedIDsRef.current = selectedIDs;

  const load = React.useCallback(async (nextQuery: string, nextPage = 1) => {
    requestControllerRef.current?.abort();
    const controller = new AbortController();
    requestControllerRef.current = controller;
    if (nextPage === 1) {
      setLoading(true);
    } else {
      setLoadingMore(true);
    }

    const requestedSelectedIDs = nextPage === 1
      ? Array.from(new Set(selectedIDsRef.current))
      : [];
    try {
      const accessToken = await resolveAccessToken();
      if (!accessToken || controller.signal.aborted) {
        if (!accessToken) {
          throw new Error("missing access token");
        }
        return;
      }

      const selectedPagePromise = requestedSelectedIDs.length > 0
        ? loadPage(
            accessToken,
            { ids: requestedSelectedIDs, page: 1, pageSize: requestedSelectedIDs.length },
            controller.signal,
          ).then((selectedPage) => ({ selectedPage, loaded: true })).catch((error: unknown): { selectedPage: null; loaded: boolean } => {
            if (controller.signal.aborted) {
              throw error;
            }
            return { selectedPage: null, loaded: false };
          })
        : Promise.resolve({ selectedPage: null, loaded: false });
      const [catalogPage, selectedResult] = await Promise.all([
        loadPage(
          accessToken,
          { query: nextQuery, page: nextPage, pageSize: 50 },
          controller.signal,
        ),
        selectedPagePromise,
      ]);
      if (controller.signal.aborted) {
        return;
      }

      setItems((current) => {
        const next = nextPage === 1 ? [] : current.slice();
        const seen = new Set(next.map(getID));
        for (const item of catalogPage.results) {
          const id = getID(item);
          if (!seen.has(id)) {
            seen.add(id);
            next.push(item);
          }
        }
        for (const item of selectedResult.selectedPage?.results ?? []) {
          const id = getID(item);
          if (!seen.has(id)) {
            seen.add(id);
            next.push(item);
          }
        }
        return next;
      });
      setPage(nextPage);
      setTotal(catalogPage.total);
      if (selectedResult.loaded && selectedResult.selectedPage) {
        onSelectedIDsResolved(
          requestedSelectedIDs,
          new Set(selectedResult.selectedPage.results.map(getID)),
        );
      }
    } catch {
      if (!controller.signal.aborted) {
        onError();
      }
    } finally {
      if (requestControllerRef.current === controller) {
        requestControllerRef.current = null;
        setLoading(false);
        setLoadingMore(false);
      }
    }
  }, [getID, loadPage, onError, onSelectedIDsResolved]);

  React.useEffect(() => {
    if (!open) {
      requestControllerRef.current?.abort();
      requestControllerRef.current = null;
      setItems([]);
      setLoading(false);
      setLoadingMore(false);
      setQuery("");
      setPage(1);
      setTotal(0);
      return;
    }

    requestControllerRef.current?.abort();
    setLoading(true);
    setLoadingMore(false);
    const timer = window.setTimeout((): void => void load(query.trim(), 1), 200);
    return () => window.clearTimeout(timer);
  }, [load, open, query]);

  React.useEffect(() => () => requestControllerRef.current?.abort(), []);

  const loadMore = React.useCallback(() => {
    if (!loading && !loadingMore && page * 50 < total) {
      void load(query.trim(), page + 1);
    }
  }, [load, loading, loadingMore, page, query, total]);

  return {
    items,
    loading,
    loadingMore,
    setQuery,
    hasMore: page * 50 < total,
    loadMore,
  };
}

export function ProjectDialog({
  draft,
  setDraft,
  onOpenChange,
  onSubmit,
}: {
  draft: ProjectDraft | null;
  setDraft: React.Dispatch<React.SetStateAction<ProjectDraft | null>>;
  onOpenChange: (open: boolean) => void;
  onSubmit: () => void | Promise<void>;
}) {
  const t = useTranslations("recent.projects");
  const [submitting, setSubmitting] = React.useState(false);
  const [catalogLoading, setCatalogLoading] = React.useState(false);
  const [mcpTools, setMCPTools] = React.useState<MCPToolDTO[]>([]);
  const [selectionLimit, setSelectionLimit] = React.useState(1);
  const stableDraft = useDialogSnapshot(draft);
  const open = Boolean(draft);
  const nameInputID = React.useId();
  const systemPromptInputID = React.useId();

  React.useEffect(() => {
    if (!draft) {
      setSubmitting(false);
    }
  }, [draft]);

  React.useEffect(() => {
    if (!open) {
      setCatalogLoading(false);
      setMCPTools([]);
      return;
    }

    let cancelled = false;
    setCatalogLoading(true);
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) {
          throw new Error("missing access token");
        }
        const [tools, policy] = await Promise.all([
          listAvailableMCPTools(token),
          getMCPPolicy(token),
        ]);
        if (!cancelled) {
          setMCPTools(tools);
          setSelectionLimit(Math.max(1, policy.maxSelectedToolsPerMessage));
          const availableMCPToolIDs = new Set(tools.map((tool) => tool.id));
          setDraft((current) => {
            if (!current) {
              return current;
            }
            const defaultMCPToolIDs = normalizeImageAttachmentProcessorSelection(
              current.defaultMCPToolIDs.filter((id) => availableMCPToolIDs.has(id)).slice(0, Math.max(1, policy.maxSelectedToolsPerMessage)),
              tools,
            );
            const unchangedMCPTools = defaultMCPToolIDs.length === current.defaultMCPToolIDs.length;
            return unchangedMCPTools
              ? current
              : { ...current, defaultMCPToolIDs };
          });
        }
      } catch {
        if (!cancelled) {
          setMCPTools([]);
          toast.error(t("defaultsLoadFailed"));
        }
      } finally {
        if (!cancelled) {
          setCatalogLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, setDraft, t]);

  const handleCatalogLoadError = React.useCallback(() => {
    toast.error(t("defaultsLoadFailed"));
  }, [t]);
  const handleSkillIDsResolved = React.useCallback((
    requestedIDs: number[],
    availableIDs: ReadonlySet<number>,
  ) => {
    const requestedIDSet = new Set(requestedIDs);
    setDraft((current) => {
      if (!current) return current;
      const defaultSkillIDs = current.defaultSkillIDs.filter(
        (id) => !requestedIDSet.has(id) || availableIDs.has(id),
      );
      return defaultSkillIDs.length === current.defaultSkillIDs.length
        ? current
        : { ...current, defaultSkillIDs };
    });
  }, [setDraft]);
  const handleKnowledgeBaseIDsResolved = React.useCallback((
    requestedIDs: string[],
    availableIDs: ReadonlySet<string>,
  ) => {
    const requestedIDSet = new Set(requestedIDs);
    setDraft((current) => {
      if (!current) return current;
      const defaultKnowledgeBaseIDs = current.defaultKnowledgeBaseIDs
        .filter((id) => !requestedIDSet.has(id) || availableIDs.has(id))
        .slice(0, 8);
      return defaultKnowledgeBaseIDs.length === current.defaultKnowledgeBaseIDs.length &&
        defaultKnowledgeBaseIDs.every((id, index) => id === current.defaultKnowledgeBaseIDs[index])
        ? current
        : { ...current, defaultKnowledgeBaseIDs };
    });
  }, [setDraft]);
  const skillCatalog = usePaginatedProjectCatalog({
    open,
    selectedIDs: draft?.defaultSkillIDs ?? [],
    loadPage: listVisibleSkills,
    getID: getSkillID,
    onSelectedIDsResolved: handleSkillIDsResolved,
    onError: handleCatalogLoadError,
  });
  const knowledgeBaseCatalog = usePaginatedProjectCatalog({
    open,
    selectedIDs: draft?.defaultKnowledgeBaseIDs.slice(0, 8) ?? [],
    loadPage: listVisibleKnowledgeBases,
    getID: getKnowledgeBaseID,
    onSelectedIDsResolved: handleKnowledgeBaseIDsResolved,
    onError: handleCatalogLoadError,
  });

  const handleSubmit = React.useCallback<React.FormEventHandler<HTMLFormElement>>(
    async (event) => {
      event.preventDefault();
      if (!draft?.name.trim() || submitting) {
        return;
      }
      setSubmitting(true);
      try {
        await onSubmit();
      } finally {
        setSubmitting(false);
      }
    },
    [draft?.name, onSubmit, submitting],
  );

  const inheritGlobalMCPDefaults = (stableDraft?.mcpDefaultMode ?? "inherit") === "inherit";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(86vh,760px)] w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>{stableDraft?.publicID ? t("editTitle") : t("createTitle")}</DialogTitle>
          <DialogDescription>{stableDraft?.publicID ? t("editDescription") : t("createDescription")}</DialogDescription>
        </DialogHeader>

        <form className="flex min-h-0 flex-1 flex-col" onSubmit={handleSubmit}>
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-2">
            <div className="space-y-1">
              <label htmlFor={nameInputID} className="text-xs text-muted-foreground">
                {t("nameLabel")}
              </label>
              <Input
                id={nameInputID}
                autoFocus
                value={stableDraft?.name ?? ""}
                maxLength={80}
                placeholder={t("namePlaceholder")}
                onChange={(event) => {
                  setDraft((current) => current ? { ...current, name: event.target.value } : current);
                }}
                disabled={submitting}
                required
              />
            </div>
            <div className="space-y-1">
              <label htmlFor={systemPromptInputID} className="text-xs text-muted-foreground">
                {t("systemPromptLabel")}
              </label>
              <Textarea
                id={systemPromptInputID}
                value={stableDraft?.systemPrompt ?? ""}
                maxLength={12000}
                placeholder={t("systemPromptPlaceholder")}
                className="h-48 resize-none overflow-y-auto [field-sizing:fixed]"
                onChange={(event) => {
                  setDraft((current) => current ? { ...current, systemPrompt: event.target.value } : current);
                }}
                disabled={submitting}
              />
            </div>

            <div className="space-y-3 border-t border-border/60 pt-4">
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("mcpDefaultsLabel")}</p>
                <Tabs
                  value={stableDraft?.mcpDefaultMode ?? "inherit"}
                  onValueChange={(value) => {
                    setDraft((current) => (
                      current
                        ? { ...current, mcpDefaultMode: value === "custom" ? "custom" : "inherit" }
                        : current
                    ));
                  }}
                >
                  <TabsList aria-label={t("mcpDefaultsLabel")} className="grid w-full grid-cols-2">
                    <TabsTrigger value="inherit" className="min-w-0" disabled={submitting}>
                      <Globe2 strokeWidth={1.7} />
                      <span className="truncate">{t("inheritGlobalMCPDefaults")}</span>
                    </TabsTrigger>
                    <TabsTrigger value="custom" className="min-w-0" disabled={submitting}>
                      <SlidersHorizontal strokeWidth={1.7} />
                      <span className="truncate">{t("customMCPDefaults")}</span>
                    </TabsTrigger>
                  </TabsList>
                </Tabs>
                <DialogCollapsible open={!inheritGlobalMCPDefaults}>
                  <ProjectDefaultSelector
                    icon={Wrench}
                    label={t("selectMCPTools")}
                    description={t("mcpDefaultsDescription")}
                    emptyLabel={t("mcpDefaultsEmpty")}
                    searchPlaceholder={t("searchMCPTools")}
                    options={mcpTools.map((tool) => ({
                      id: tool.id,
                      label: tool.displayName || tool.name,
                      detail: tool.serverName,
                    }))}
                    selectedIDs={stableDraft?.defaultMCPToolIDs ?? []}
                    selectionLimit={selectionLimit}
                    loading={catalogLoading}
                    disabled={submitting}
                    onChange={(defaultMCPToolIDs) => {
                      if (hasMultipleImageAttachmentProcessors(defaultMCPToolIDs, mcpTools)) {
                        toast.error(t("imageProcessorLimitTitle"), {
                          description: t("imageProcessorLimitDescription"),
                        });
                        return;
                      }
                      setDraft((current) => current ? { ...current, defaultMCPToolIDs } : current);
                    }}
                  />
                </DialogCollapsible>
                <DialogCollapsible open={inheritGlobalMCPDefaults}>
                  <p className="pt-1 text-[11px] leading-4 text-muted-foreground">
                    {t("inheritGlobalMCPDefaultsDescription")}
                  </p>
                </DialogCollapsible>
              </div>

              <ProjectDefaultSelector
                icon={Box}
                label={t("selectSkills")}
                description={t("skillDefaultsDescription")}
                emptyLabel={t("skillDefaultsEmpty")}
                searchPlaceholder={t("searchSkills")}
                options={skillCatalog.items.map((skill) => ({
                  id: skill.id,
                  label: skill.title,
                  detail: skill.description.trim() || (skill.trigger ? `/${skill.trigger}` : ""),
                }))}
                selectedIDs={stableDraft?.defaultSkillIDs ?? []}
                selectionLimit={selectionLimit}
                loading={skillCatalog.loading && skillCatalog.items.length === 0}
                searching={skillCatalog.loading}
                loadingMore={skillCatalog.loadingMore}
                hasMore={skillCatalog.hasMore}
                disabled={submitting || catalogLoading}
                onQueryChange={skillCatalog.setQuery}
                onLoadMore={skillCatalog.loadMore}
                onChange={(defaultSkillIDs) => {
                  setDraft((current) => current ? { ...current, defaultSkillIDs } : current);
                }}
              />

              <ProjectDefaultSelector
                icon={BookOpen}
                label={t("selectKnowledgeBases")}
                description={t("knowledgeBaseDefaultsDescription")}
                emptyLabel={t("knowledgeBaseDefaultsEmpty")}
                searchPlaceholder={t("searchKnowledgeBases")}
                options={knowledgeBaseCatalog.items.map((item) => ({
                  id: item.publicID,
                  label: item.name,
                  detail: `${item.scope === "builtin" ? t("builtinKnowledgeBase") : t("personalKnowledgeBase")} · ${t("knowledgeBaseFileCount", { count: item.readyFileCount })}`,
                  disabled: item.readyFileCount === 0,
                }))}
                selectedIDs={stableDraft?.defaultKnowledgeBaseIDs ?? []}
                selectionLimit={8}
                loading={knowledgeBaseCatalog.loading && knowledgeBaseCatalog.items.length === 0}
                searching={knowledgeBaseCatalog.loading}
                loadingMore={knowledgeBaseCatalog.loadingMore}
                hasMore={knowledgeBaseCatalog.hasMore}
                disabled={submitting}
                onQueryChange={knowledgeBaseCatalog.setQuery}
                onLoadMore={knowledgeBaseCatalog.loadMore}
                onChange={(defaultKnowledgeBaseIDs) => {
                  setDraft((current) => current ? { ...current, defaultKnowledgeBaseIDs } : current);
                }}
              />
            </div>
          </div>

          <DialogFooter className="shrink-0 px-4 py-3">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={submitting}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={!draft?.name.trim() || submitting}>
              {t("save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function ProjectDefaultSelector<T extends string | number>({
  icon: Icon,
  label,
  description,
  emptyLabel,
  searchPlaceholder,
  options,
  selectedIDs,
  selectionLimit,
  loading,
  searching = false,
  loadingMore = false,
  hasMore = false,
  disabled,
  onQueryChange,
  onLoadMore,
  onChange,
}: {
  icon: LucideIcon;
  label: string;
  description: string;
  emptyLabel: string;
  searchPlaceholder: string;
  options: ProjectDefaultOption<T>[];
  selectedIDs: T[];
  selectionLimit: number;
  loading: boolean;
  searching?: boolean;
  loadingMore?: boolean;
  hasMore?: boolean;
  disabled: boolean;
  onQueryChange?: (query: string) => void;
  onLoadMore?: () => void;
  onChange: (ids: T[]) => void;
}) {
  const t = useTranslations("recent.projects");
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState("");
  const selectedIDSet = React.useMemo(() => new Set(selectedIDs), [selectedIDs]);
  const normalizedQuery = query.trim().toLowerCase();
  const filteredOptions = normalizedQuery
    ? options.filter((option) => `${option.label} ${option.detail}`.toLowerCase().includes(normalizedQuery))
    : options;

  return (
    <div className="space-y-1 pt-1">
      <p className="text-xs text-muted-foreground">{label}</p>
      <Popover
        modal
        open={open}
        onOpenChange={(nextOpen) => {
          setOpen(nextOpen);
          if (!nextOpen) {
            setQuery("");
            onQueryChange?.("");
          }
        }}
      >
        <PopoverTrigger asChild>
          <Button
            type="button"
            variant="outline"
            className="h-8 w-full justify-between px-3 font-normal shadow-none"
            disabled={disabled || loading}
          >
            <span className="flex min-w-0 items-center gap-2">
              {loading ? <Spinner className="size-3.5" /> : <Icon className="size-3.5 text-muted-foreground" strokeWidth={1.7} />}
              <span className="truncate">
                {selectedIDs.length > 0 ? t("defaultsSelected", { count: selectedIDs.length }) : emptyLabel}
              </span>
            </span>
            <ChevronDown className="size-3.5 shrink-0 text-muted-foreground" strokeWidth={1.7} />
          </Button>
        </PopoverTrigger>
        <PopoverContent align="start" sideOffset={6} className="w-[min(24rem,calc(100vw-3rem))] p-1.5">
          <div className="relative mb-1.5">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" strokeWidth={1.7} />
            <Input
              value={query}
              placeholder={searchPlaceholder}
              className="h-8 pl-8 text-xs"
              onChange={(event) => {
                setQuery(event.target.value);
                onQueryChange?.(event.target.value);
              }}
            />
          </div>
          <div className="max-h-64 touch-pan-y space-y-0.5 overflow-y-auto overscroll-contain">
            {filteredOptions.map((option) => {
              const selected = selectedIDSet.has(option.id);
              const unavailable = option.disabled === true && !selected;
              return (
                <label
                  key={option.id}
                  className={cn(
                    "flex min-h-9 w-full items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors",
                    unavailable ? "cursor-not-allowed opacity-45" : "cursor-pointer hover:bg-accent",
                  )}
                >
                  <Checkbox
                    checked={selected}
                    disabled={unavailable}
                    onCheckedChange={(checked) => {
                      if (checked !== true) {
                        onChange(selectedIDs.filter((id) => id !== option.id));
                        return;
                      }
                      if (selectedIDs.length >= selectionLimit) {
                        toast.error(t("defaultsSelectionLimit", { limit: selectionLimit }));
                        return;
                      }
                      onChange([...selectedIDs, option.id]);
                    }}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-xs font-medium text-foreground">{option.label}</span>
                    {option.detail ? <span className="block truncate text-[11px] text-muted-foreground">{option.detail}</span> : null}
                  </span>
                </label>
              );
            })}
            {searching ? (
              <div className="flex h-8 items-center justify-center">
                <Spinner className="size-3" />
              </div>
            ) : null}
            {!searching && filteredOptions.length === 0 ? (
              <p className="px-2 py-6 text-center text-xs text-muted-foreground">{t("defaultsNoResults")}</p>
            ) : null}
            {!searching && hasMore ? (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-8 w-full text-[11px] text-muted-foreground"
                disabled={loadingMore}
                onClick={onLoadMore}
              >
                {loadingMore ? <Spinner className="size-3" /> : t("loadMore")}
              </Button>
            ) : null}
          </div>
        </PopoverContent>
      </Popover>
      <p className="text-[11px] leading-4 text-muted-foreground">{description}</p>
    </div>
  );
}
