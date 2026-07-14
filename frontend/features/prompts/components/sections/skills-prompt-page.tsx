"use client";

import * as React from "react";
import { Plus, ScrollText, Search, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { CenteredEmptyState } from "@/components/ui/empty-state";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "@/components/ui/input-group";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  SkillsSection,
  type SkillsSectionHandle,
} from "@/features/prompts/components/sections/skills-section";
import {
  promptPresetKey,
  useSkillsPromptPage,
} from "@/features/prompts/hooks/use-skills-prompt-page";
import { cn } from "@/lib/utils";
import type { PromptPresetDTO } from "@/shared/api/prompt-presets.types";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { PROMPT_PRESET_LIMITS } from "@/shared/model/prompt-presets";

function PromptPresetCard({
  item,
  onOpen,
  onDelete,
  onEnabledChange,
}: {
  item: PromptPresetDTO;
  onOpen: (item: PromptPresetDTO) => void;
  onDelete: (item: PromptPresetDTO) => void;
  onEnabledChange: (item: PromptPresetDTO, enabled: boolean) => void;
}) {
  const t = useTranslations("prompts");
  const editable = item.scope === "user";
  const displayName = item.trigger || item.title;
  const summary = item.description || item.content;

  function handleKeyDown(event: React.KeyboardEvent<HTMLElement>) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onOpen(item);
    }
  }

  return (
    <article
      role="button"
      tabIndex={0}
      className={cn(
        "group flex min-h-16 min-w-0 items-center gap-2.5 rounded-lg bg-muted/35 px-3 py-2.5 text-left transition-colors hover:bg-muted/55 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/45",
        "cursor-pointer",
        !item.enabled && "text-muted-foreground",
      )}
      onClick={() => onOpen(item)}
      onKeyDown={handleKeyDown}
    >
      <div className="flex size-7 shrink-0 items-center justify-center text-muted-foreground">
        <ScrollText className="size-4.5" strokeWidth={1.8} />
      </div>

      <div className="grid min-w-0 flex-1 gap-0.5">
        <div className="flex min-w-0 items-center gap-1.5">
          <h3 className={cn("min-w-0 truncate text-sm font-medium text-foreground", !item.enabled && "text-muted-foreground")}>
            {displayName}
          </h3>
          {item.scope === "builtin" ? (
            <Badge variant="secondary" className="h-5 rounded-md px-1.5 text-[10px] font-normal">
              {t("builtIn")}
            </Badge>
          ) : null}
        </div>
        <p className="min-w-0 truncate text-xs leading-5 text-muted-foreground">{summary}</p>
      </div>

      {editable ? (
        <div className="flex shrink-0 items-center gap-1">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-7 text-muted-foreground opacity-100 transition-opacity hover:bg-background/80 hover:text-destructive md:opacity-0 md:group-hover:opacity-100 md:group-focus-within:opacity-100"
            onClick={(event) => {
              event.stopPropagation();
              onDelete(item);
            }}
            aria-label={t("delete")}
          >
            <Trash2 className="h-3.5 w-3.5" strokeWidth={1.6} />
          </Button>
          <Switch
            size="sm"
            checked={item.enabled}
            onClick={(event) => event.stopPropagation()}
            onCheckedChange={(enabled) => onEnabledChange(item, enabled)}
            aria-label={t("enabled")}
          />
        </div>
      ) : null}
    </article>
  );
}

function PromptPresetListSkeleton() {
  return (
    <div className="grid gap-4 md:ml-13 md:w-[calc(100%-3.25rem)] md:grid-cols-2">
      {Array.from({ length: 8 }).map((_, index) => (
        <div key={index} className="flex min-h-16 items-center gap-2.5 rounded-lg bg-muted/35 px-3 py-2.5">
          <Skeleton className="size-7 shrink-0 rounded-md bg-muted/55" />
          <div className="min-w-0 flex-1 space-y-1.5">
            <Skeleton className="h-4 w-32 rounded-full bg-muted/55" />
            <Skeleton className="h-3 w-4/5 rounded-full bg-muted/35" />
          </div>
        </div>
      ))}
    </div>
  );
}

export function SkillsPromptPage() {
  const t = useTranslations("prompts");
  const commonActionsT = useTranslations("common.actions");
  const commonStatesT = useTranslations("common.states");
  const [activeTab, setActiveTab] = React.useState("skills");
  const skillsSectionRef = React.useRef<SkillsSectionHandle>(null);
  const {
    items,
    filteredItems,
    loading,
    loadingMore,
    loadMoreFailed,
    hasMore,
    loadMoreRef,
    query,
    setQuery,
    saving,
    form,
    setForm,
    dialogOpen,
    setDialogOpen,
    viewTarget,
    setViewTarget,
    deleteTarget,
    setDeleteTarget,
    openCreate,
    openPrompt,
    save,
    confirmDelete,
    retryLoadMore,
    toggleEnabled,
  } = useSkillsPromptPage();
  const stableViewTarget = useDialogSnapshot(viewTarget);

  const emptyState = (
    <div className="flex h-full min-h-0 w-full items-center justify-center">
      <CenteredEmptyState
        title={items.length === 0 ? t("empty") : t("noResults")}
        description={items.length === 0 ? t("emptyDescription") : t("noResultsDescription")}
      />
    </div>
  );
  const listContent = (
    <div className="h-full min-h-0 w-full flex-1 overflow-y-auto pr-2" data-sidebar-scroll-root="true">
      {filteredItems.length === 0 ? (
        emptyState
      ) : (
        <div className="grid gap-4 md:ml-13 md:w-[calc(100%-3.25rem)] md:grid-cols-2">
          {filteredItems.map((item) => (
            <PromptPresetCard
              key={promptPresetKey(item)}
              item={item}
              onOpen={openPrompt}
              onDelete={setDeleteTarget}
              onEnabledChange={(target, enabled) => void toggleEnabled(target, enabled)}
            />
          ))}
        </div>
      )}

      {hasMore && !loadMoreFailed ? <div ref={loadMoreRef} className="h-4" aria-hidden="true" /> : null}

      {loadingMore ? (
        <div className="flex items-center justify-center py-4">
          <Spinner className="size-4 text-muted-foreground" label={commonStatesT("loading")} />
        </div>
      ) : null}

      {loadMoreFailed ? (
        <div className="flex items-center justify-center gap-3 px-3 py-4 text-xs text-muted-foreground">
          <span>{t("loadFailed")}</span>
          <button
            type="button"
            className="underline underline-offset-4 transition-colors hover:text-foreground"
            onClick={() => {
              void retryLoadMore();
            }}
          >
            {commonActionsT("retry")}
          </button>
        </div>
      ) : null}
    </div>
  );

  return (
    <div className="flex h-full min-h-0 w-full flex-1 flex-col overflow-hidden">
      <div className="mx-auto flex h-full min-h-0 w-full max-w-[912px] flex-1 flex-col px-3 pb-8 pt-6 md:pt-15">
        <header className="ml-0 md:ml-13 md:w-[calc(100%-3.25rem)]">
          <div className="flex items-start justify-between gap-4">
            <h1 className="min-w-0 text-xl font-semibold tracking-[-0.03em] text-foreground md:text-2xl">{t("pageTitle")}</h1>
            {activeTab === "skills" ? (
              <Button
                size="sm"
                variant="default"
                className="shrink-0"
                onClick={() => skillsSectionRef.current?.openCreate()}
              >
                <Plus className="size-4" />
                {t("add")}
              </Button>
            ) : (
              <Button size="sm" variant="default" className="shrink-0" disabled={loading} onClick={openCreate}>
                <Plus className="size-4" />
                {t("add")}
              </Button>
            )}
          </div>

          <Tabs value={activeTab} onValueChange={setActiveTab} className="mt-5">
            <TabsList>
              <TabsTrigger value="skills">{t("skillsTab")}</TabsTrigger>
              <TabsTrigger value="prompts">{t("promptsTab")}</TabsTrigger>
            </TabsList>
          </Tabs>

          <div className="relative mt-5 md:mt-8">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={activeTab === "skills" ? t("skillsSearchPlaceholder") : t("searchPlaceholder")}
              className="rounded-xl bg-background pl-9"
            />
          </div>
        </header>

        <Tabs value={activeTab} onValueChange={setActiveTab} className="min-h-0 flex-1">
          <TabsContent value="skills" className="flex h-full min-h-0">
            <SkillsSection ref={skillsSectionRef} query={query} />
          </TabsContent>
          <TabsContent value="prompts" className="flex h-full min-h-0">
            <section className="mt-6 flex min-h-0 flex-1 flex-col overflow-hidden">
              {loading ? (
                <div className="h-full min-h-0 flex-1 overflow-y-auto pr-2">
                  <PromptPresetListSkeleton />
                </div>
              ) : (
                listContent
              )}
            </section>
          </TabsContent>
        </Tabs>
      </div>

      <Dialog open={dialogOpen} onOpenChange={(open) => !saving && setDialogOpen(open)}>
        <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]">
          <DialogHeader className="shrink-0 px-5 pb-3 pt-5">
            <DialogTitle>{form.id ? t("editTitle") : t("createTitle")}</DialogTitle>
            <DialogDescription>{t("dialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-5 py-2">
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("name")}</p>
              <InputGroup>
                <InputGroupAddon>/</InputGroupAddon>
                <InputGroupInput
                  value={form.name}
                  placeholder="musk"
                  maxLength={PROMPT_PRESET_LIMITS.name}
                  onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))}
                />
              </InputGroup>
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("promptDescription")}</p>
              <Input
                value={form.description}
                maxLength={PROMPT_PRESET_LIMITS.description}
                onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))}
              />
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("content")}</p>
              <Textarea
                value={form.content}
                className="h-64 resize-none overflow-y-auto [field-sizing:fixed]"
                maxLength={PROMPT_PRESET_LIMITS.content}
                onChange={(event) => setForm((current) => ({ ...current, content: event.target.value }))}
              />
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("enabled")}</p>
              <Switch
                size="sm"
                checked={form.enabled}
                disabled={saving}
                onCheckedChange={(enabled) => setForm((current) => ({ ...current, enabled }))}
              />
            </div>
          </div>
          <DialogFooter className="shrink-0 px-5 py-3">
            <Button variant="ghost" disabled={saving} onClick={() => setDialogOpen(false)}>
              {t("cancel")}
            </Button>
            <Button disabled={saving} onClick={() => void save()}>
              {saving ? t("saving") : t("save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={viewTarget !== null} onOpenChange={(open) => !open && setViewTarget(null)}>
        <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]">
          <DialogHeader className="shrink-0 px-5 pb-3 pt-5">
            <DialogTitle>{stableViewTarget?.trigger || stableViewTarget?.title}</DialogTitle>
            <DialogDescription>{t("viewDescription")}</DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-5 py-2">
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("name")}</p>
              <Input value={stableViewTarget?.trigger || stableViewTarget?.title || ""} readOnly />
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("promptDescription")}</p>
              <Input value={stableViewTarget?.description || ""} readOnly />
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("content")}</p>
              <Textarea value={stableViewTarget?.content || ""} className="h-64 resize-none overflow-y-auto [field-sizing:fixed]" readOnly />
            </div>
          </div>
          <DialogFooter className="shrink-0 px-5 py-3">
            <Button variant="ghost" onClick={() => setViewTarget(null)}>
              {t("close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteTarget !== null} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("deleteDescription")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void confirmDelete()}>{t("delete")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
