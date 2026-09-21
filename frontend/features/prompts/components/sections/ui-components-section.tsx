"use client";

import { LayoutGrid, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
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
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { cn } from "@/lib/utils";
import { createMyUIComponent, deleteMyUIComponent, listVisibleUIComponents, updateMyUIComponent } from "@/shared/api/ui-components";
import type { UIComponentDTO } from "@/shared/api/ui-components.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { UIComponentEditorDialog } from "@/shared/components/ui-component-editor-dialog";
import {
  EMPTY_UI_COMPONENT_FORM,
  type UIComponentFormValue,
  uiComponentFormFromDTO,
  uiComponentFormIsComplete,
  uiComponentFormIsWithinLimits,
  uiComponentPayloadFromForm,
  uiComponentSchemaIsValid,
} from "@/shared/model/ui-components";

export type UIComponentsSectionHandle = {
  openCreate: () => void;
};

function matchesQuery(item: UIComponentDTO, query: string): boolean {
  const normalized = query.trim().toLowerCase();
  return !normalized || `${item.name} ${item.description}`.toLowerCase().includes(normalized);
}

function ComponentCard({
  item,
  onOpen,
  onDelete,
  onEnabledChange,
}: {
  item: UIComponentDTO;
  onOpen: (item: UIComponentDTO) => void;
  onDelete: (item: UIComponentDTO) => void;
  onEnabledChange: (item: UIComponentDTO, enabled: boolean) => void;
}) {
  const t = useTranslations("uiComponents");
  const editable = item.scope === "user";

  return (
    <div
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
        <LayoutGrid className="size-4.5" strokeWidth={1.8} />
      </div>
      <div className="grid min-w-0 flex-1 gap-0.5">
        <div className="flex min-w-0 items-center gap-1.5">
          <h3 className={cn("min-w-0 truncate font-mono text-sm font-medium text-foreground", !item.enabled && "text-muted-foreground")}>{item.name}</h3>
          {item.scope !== "user" ? (
            <Badge variant="secondary" className="h-5 rounded-md px-1.5 text-[10px] font-normal">
              {t(`scope.${item.scope}`)}
            </Badge>
          ) : null}
        </div>
        <p className="min-w-0 truncate text-xs leading-5 text-muted-foreground">{item.description}</p>
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
            onCheckedChange={(checked) => onEnabledChange(item, checked)}
            aria-label={item.enabled ? t("disable") : t("enable")}
          />
        </div>
      ) : null}
    </div>
  );
}

export const UIComponentsSection = React.forwardRef<UIComponentsSectionHandle, { query: string }>(function UIComponentsSection({ query }, ref) {
  const t = useTranslations("uiComponents");
  const resolveError = useLocalizedErrorMessage();
  const [items, setItems] = React.useState<UIComponentDTO[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [saving, setSaving] = React.useState(false);
  const [form, setForm] = React.useState<UIComponentFormValue>(EMPTY_UI_COMPONENT_FORM);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [deleteTarget, setDeleteTarget] = React.useState<UIComponentDTO | null>(null);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }
      const page = await listVisibleUIComponents(token, { pageSize: 100 });
      setItems(page.results);
    } catch (error) {
      toast.error(t("toast.loadFailed"), { description: resolveError(error) });
    } finally {
      setLoading(false);
    }
  }, [resolveError, t]);

  React.useEffect(() => {
    void load();
  }, [load]);

  const openCreate = React.useCallback(() => {
    setForm(EMPTY_UI_COMPONENT_FORM);
    setDialogOpen(true);
  }, []);

  React.useImperativeHandle(ref, () => ({ openCreate }), [openCreate]);

  const openEdit = React.useCallback((item: UIComponentDTO) => {
    setForm(uiComponentFormFromDTO(item));
    setDialogOpen(true);
  }, []);

  const save = React.useCallback(async () => {
    if (form.scope && form.scope !== "user") {
      setDialogOpen(false);
      return;
    }
    if (!uiComponentFormIsComplete(form)) {
      toast.error(t("toast.invalid"));
      return;
    }
    if (!uiComponentSchemaIsValid(form.propsSchema)) {
      toast.error(t("toast.invalidSchema"));
      return;
    }
    if (!uiComponentFormIsWithinLimits(form)) {
      toast.error(t("toast.tooLong"));
      return;
    }
    setSaving(true);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }
      const payload = uiComponentPayloadFromForm(form);
      if (form.id) {
        await updateMyUIComponent(token, form.id, payload);
        toast.success(t("toast.updated"));
      } else {
        await createMyUIComponent(token, payload);
        toast.success(t("toast.created"));
      }
      setDialogOpen(false);
      await load();
    } catch (error) {
      toast.error(form.id ? t("toast.updateFailed") : t("toast.createFailed"), { description: resolveError(error) });
    } finally {
      setSaving(false);
    }
  }, [form, load, resolveError, t]);

  const toggleEnabled = React.useCallback(
    async (item: UIComponentDTO, enabled: boolean) => {
      setItems((current) => current.map((row) => (row.id === item.id ? { ...row, enabled } : row)));
      try {
        const token = await resolveAccessToken();
        if (!token) {
          return;
        }
        await updateMyUIComponent(token, item.id, { enabled });
      } catch (error) {
        setItems((current) => current.map((row) => (row.id === item.id ? item : row)));
        toast.error(t("toast.updateFailed"), { description: resolveError(error) });
      }
    },
    [resolveError, t],
  );

  const confirmDelete = React.useCallback(async () => {
    if (!deleteTarget) {
      return;
    }
    const target = deleteTarget;
    setDeleteTarget(null);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }
      await deleteMyUIComponent(token, target.id);
      setItems((current) => current.filter((row) => row.id !== target.id));
      toast.success(t("toast.deleted"));
    } catch (error) {
      toast.error(t("toast.deleteFailed"), { description: resolveError(error) });
    }
  }, [deleteTarget, resolveError, t]);

  const visible = React.useMemo(() => items.filter((item) => matchesQuery(item, query)), [items, query]);

  return (
    <section className="mt-6 flex min-h-0 flex-1 flex-col overflow-hidden">
      <div className="h-full min-h-0 flex-1 overflow-y-auto pr-2">
        {loading ? (
          <div className="space-y-2">
            {Array.from({ length: 4 }).map((_, index) => (
              <Skeleton key={`ui-component-skeleton-${index}`} className="h-16 rounded-lg" />
            ))}
          </div>
        ) : visible.length === 0 ? (
          <CenteredEmptyState title={t("empty")} />
        ) : (
          <div className="space-y-2">
            {visible.map((item) => (
              <ComponentCard key={item.id} item={item} onOpen={openEdit} onDelete={setDeleteTarget} onEnabledChange={(target, enabled) => void toggleEnabled(target, enabled)} />
            ))}
          </div>
        )}
      </div>

      <UIComponentEditorDialog
        key={form.id ?? "new"}
        open={dialogOpen}
        saving={saving}
        readOnly={Boolean(form.scope && form.scope !== "user")}
        form={form}
        onOpenChange={setDialogOpen}
        onFormChange={setForm}
        onSave={() => void save()}
      />

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
    </section>
  );
});
