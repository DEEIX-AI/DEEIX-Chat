"use client";

import * as React from "react";
import { Box, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

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
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { cn } from "@/lib/utils";
import { createMySkill, deleteMySkill, getVisibleSkill, listMySkills, listVisibleSkills, updateMySkill } from "@/shared/api/skills";
import type { SkillDTO, SkillSummaryDTO } from "@/shared/api/skills.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import {
  EMPTY_SKILL_FORM,
  SKILL_LIMITS,
  skillFormFromDTO,
  skillFormIsWithinLimits,
  skillPayloadFromForm,
  skillPayloadIsComplete,
  type SkillFormValue,
} from "@/shared/model/skills";

const SKILL_PAGE_SIZE = 100;

export type SkillsSectionHandle = {
  openCreate: () => void;
};

type SkillForm = SkillFormValue;
type SkillListItem = SkillDTO | SkillSummaryDTO;

function skillKey(item: SkillListItem): string {
  return `${item.scope}-${item.id}`;
}

function hasSkillMarkdown(item: SkillListItem): item is SkillDTO {
  return "markdown" in item;
}

function orderSkills<T extends SkillListItem>(items: T[]): T[] {
  return [...items].sort((a, b) => {
    const rank = (item: SkillListItem) => {
      if (!item.enabled) return 2;
      return item.scope === "builtin" ? 1 : 0;
    };
    return rank(a) - rank(b) || a.sortOrder - b.sortOrder || b.id - a.id;
  });
}

function skillMatchesQuery(item: SkillListItem, query: string): boolean {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return true;
  return [item.title, item.trigger, item.description].join(" ").toLowerCase().includes(normalized);
}

function SkillCard({
  item,
  onOpen,
  onDelete,
  onEnabledChange,
}: {
  item: SkillListItem;
  onOpen: (item: SkillListItem) => void;
  onDelete: (item: SkillDTO) => void;
  onEnabledChange: (item: SkillDTO, enabled: boolean) => void;
}) {
  const t = useTranslations("prompts");
  const editable = item.scope === "user";
  const summary = item.description || (hasSkillMarkdown(item) ? item.markdown : "");

  return (
    <article
      role="button"
      tabIndex={0}
      className={cn(
        "group flex min-h-16 min-w-0 cursor-pointer items-center gap-2.5 rounded-lg bg-muted/35 px-3 py-2.5 text-left transition-colors hover:bg-muted/55 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/45",
        !item.enabled && "text-muted-foreground",
      )}
      onClick={() => onOpen(item)}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onOpen(item);
        }
      }}
    >
      <div className="flex size-7 shrink-0 items-center justify-center text-muted-foreground">
        <Box className="size-4.5" strokeWidth={1.8} />
      </div>
      <div className="grid min-w-0 flex-1 gap-0.5">
        <div className="flex min-w-0 items-center gap-1.5">
          <h3 className={cn("min-w-0 truncate text-sm font-medium text-foreground", !item.enabled && "text-muted-foreground")}>
            {item.trigger || item.title}
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
              if (hasSkillMarkdown(item)) {
                onDelete(item);
              }
            }}
            aria-label={t("delete")}
          >
            <Trash2 className="h-3.5 w-3.5" strokeWidth={1.6} />
          </Button>
          <Switch
            size="sm"
            checked={item.enabled}
            onClick={(event) => event.stopPropagation()}
            onCheckedChange={(enabled) => {
              if (hasSkillMarkdown(item)) {
                onEnabledChange(item, enabled);
              }
            }}
            aria-label={t("enabled")}
          />
        </div>
      ) : null}
    </article>
  );
}

function SkillListSkeleton() {
  return (
    <div className="grid gap-4 md:ml-13 md:w-[calc(100%-3.25rem)] md:grid-cols-2">
      {Array.from({ length: 6 }).map((_, index) => (
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

export const SkillsSection = React.forwardRef<SkillsSectionHandle, { query: string }>(function SkillsSection({ query }, ref) {
  const t = useTranslations("prompts");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [items, setItems] = React.useState<SkillListItem[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [form, setForm] = React.useState<SkillForm>(EMPTY_SKILL_FORM);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [viewTarget, setViewTarget] = React.useState<SkillDTO | null>(null);
  const [deleteTarget, setDeleteTarget] = React.useState<SkillDTO | null>(null);
  const stableViewTarget = useDialogSnapshot(viewTarget);

  const filteredItems = React.useMemo(
    () => orderSkills(items.filter((item) => skillMatchesQuery(item, query))),
    [items, query],
  );

  const reload = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        setItems([]);
        return;
      }
      const [mine, visible] = await Promise.all([
        listMySkills(token, { page: 1, pageSize: SKILL_PAGE_SIZE }),
        listVisibleSkills(token, { page: 1, pageSize: SKILL_PAGE_SIZE }),
      ]);
      setItems(orderSkills([...mine.results, ...visible.results.filter((item) => item.scope === "builtin")]));
    } catch (error) {
      toast.error(t("skillsLoadFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setLoading(false);
    }
  }, [resolveErrorMessage, t]);

  React.useEffect(() => {
    void reload();
  }, [reload]);

  const openCreate = React.useCallback(() => {
    setForm(EMPTY_SKILL_FORM);
    setDialogOpen(true);
  }, []);

  React.useImperativeHandle(ref, () => ({ openCreate }), [openCreate]);

  const openSkill = React.useCallback(async (item: SkillListItem) => {
    if (item.scope === "user") {
      if (hasSkillMarkdown(item)) {
        setForm(skillFormFromDTO(item));
        setDialogOpen(true);
      }
      return;
    }
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      const data = await getVisibleSkill(token, item.id);
      setViewTarget(data.skill);
    } catch (error) {
      toast.error(t("skillsLoadFailed"), { description: resolveErrorMessage(error) });
    }
  }, [resolveErrorMessage, t]);

  const save = React.useCallback(async () => {
    const payload = skillPayloadFromForm(form);
    if (!skillPayloadIsComplete(payload)) {
      toast.error(t("skillInvalid"));
      return;
    }
    if (!skillFormIsWithinLimits(form)) {
      toast.error(t("skillTooLong"));
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      if (form.id) {
        const data = await updateMySkill(token, form.id, payload);
        setItems((current) => orderSkills(current.map((item) => (skillKey(item) === skillKey(data.skill) ? data.skill : item))));
        toast.success(t("skillUpdated"));
      } else {
        const data = await createMySkill(token, payload);
        setItems((current) => orderSkills([...current, data.skill]));
        toast.success(t("skillCreated"));
      }
      setDialogOpen(false);
    } catch (error) {
      toast.error(form.id ? t("skillUpdateFailed") : t("skillCreateFailed"), { description: resolveErrorMessage(error) });
    } finally {
      setSaving(false);
    }
  }, [form, resolveErrorMessage, t]);

  const confirmDelete = React.useCallback(async () => {
    if (!deleteTarget) return;
    const target = deleteTarget;
    setDeleteTarget(null);
    try {
      const token = await resolveAccessToken();
      if (!token) return;
      await deleteMySkill(token, target.id);
      setItems((current) => current.filter((item) => skillKey(item) !== skillKey(target)));
      toast.success(t("skillDeleted"));
    } catch (error) {
      toast.error(t("skillDeleteFailed"), { description: resolveErrorMessage(error) });
    }
  }, [deleteTarget, resolveErrorMessage, t]);

  const toggleEnabled = React.useCallback(
    async (item: SkillDTO, enabled: boolean) => {
      if (item.scope !== "user") return;
      const previous = item;
      setItems((current) => orderSkills(current.map((row) => (skillKey(row) === skillKey(item) ? { ...row, enabled } : row))));
      try {
        const token = await resolveAccessToken();
        if (!token) return;
        const data = await updateMySkill(token, item.id, { enabled });
        setItems((current) => orderSkills(current.map((row) => (skillKey(row) === skillKey(data.skill) ? data.skill : row))));
      } catch (error) {
        setItems((current) => orderSkills(current.map((row) => (skillKey(row) === skillKey(previous) ? previous : row))));
        toast.error(t("skillUpdateFailed"), { description: resolveErrorMessage(error) });
      }
    },
    [resolveErrorMessage, t],
  );

  return (
    <div className="mt-6 flex min-h-0 flex-1 flex-col overflow-hidden">
      <div className="min-h-0 flex-1 overflow-hidden">
        {loading ? (
          <div className="h-full min-h-0 overflow-y-auto pr-2">
            <SkillListSkeleton />
          </div>
        ) : (
          <div className="h-full min-h-0 overflow-y-auto pr-2" data-sidebar-scroll-root="true">
            {filteredItems.length === 0 ? (
              <div className="flex h-full min-h-0 w-full items-center justify-center">
                <CenteredEmptyState
                  title={items.length === 0 ? t("skillsEmpty") : t("skillsNoResults")}
                  description={items.length === 0 ? t("skillsEmptyDescription") : t("noResultsDescription")}
                />
              </div>
            ) : (
              <div className="grid gap-4 md:ml-13 md:w-[calc(100%-3.25rem)] md:grid-cols-2">
                {filteredItems.map((item) => (
                  <SkillCard
                    key={skillKey(item)}
                    item={item}
                    onOpen={openSkill}
                    onDelete={setDeleteTarget}
                    onEnabledChange={(target, enabled) => void toggleEnabled(target, enabled)}
                  />
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={(open) => !saving && setDialogOpen(open)}>
        <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]">
          <DialogHeader className="shrink-0 px-5 pb-3 pt-5">
            <DialogTitle>{form.id ? t("editSkillTitle") : t("createSkillTitle")}</DialogTitle>
            <DialogDescription>{t("skillDialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-5 py-2">
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("name")}</p>
              <InputGroup>
                <InputGroupAddon>/</InputGroupAddon>
                <InputGroupInput
                  value={form.name}
                  maxLength={SKILL_LIMITS.name}
                  onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))}
                />
              </InputGroup>
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("promptDescription")}</p>
              <Input
                value={form.description}
                maxLength={SKILL_LIMITS.description}
                onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))}
              />
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("skillMarkdown")}</p>
              <Textarea
                value={form.markdown}
                className="h-64 resize-none overflow-y-auto [field-sizing:fixed]"
                maxLength={SKILL_LIMITS.markdown}
                onChange={(event) => setForm((current) => ({ ...current, markdown: event.target.value }))}
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
            <DialogDescription>{t("skillViewDescription")}</DialogDescription>
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
              <p className="text-xs text-muted-foreground">{t("skillMarkdown")}</p>
              <Textarea value={stableViewTarget?.markdown || ""} className="h-64 resize-none overflow-y-auto [field-sizing:fixed]" readOnly />
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
            <AlertDialogTitle>{t("deleteSkillTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("deleteSkillDescription")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void confirmDelete()}>{t("delete")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
});
