"use client";

import { Ban, CirclePlay, Link2, MoreHorizontal, PencilLine, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useRouter } from "next/navigation";
import * as React from "react";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Table, TableBody, TableCell, TableEmptyRow, TableHead, TableHeader, TableLoadingRow, TableRow } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ModelIcon, resolveModelIconURL, resolveModelProviderIcon } from "@/entities/model";
import { useSettingsModelProviders } from "@/features/settings/hooks/use-settings-model-providers";
import { modelProviderProtocolLabel } from "@/features/settings/model/model-provider-protocols";
import { toErrorMessagePath } from "@/i18n/resolve-error-message";
import type { PersonalProviderDTO } from "@/shared/api/personal-providers-types";
import { SettingsPage, SettingsSection } from "@/shared/components/settings-layout";
import { usePersonalProviderAccess } from "@/shared/hooks/use-personal-provider-access";
import { ModelsEditDialog } from "./models-edit-dialog";
import { ModelsImportLinkDialog } from "./models-import-link-dialog";
import { ModelsProviderDialog } from "./models-provider-dialog";

export function SettingsModels() {
  const t = useTranslations("settings.modelsPage");
  const commonT = useTranslations("common");
  const router = useRouter();
  const access = usePersonalProviderAccess();
  const { providers, loading, busyID, probe, create, update, remove, listModels, check } = useSettingsModelProviders(access.enabled);
  const [addOpen, setAddOpen] = React.useState(false);
  const [linkOpen, setLinkOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<PersonalProviderDTO | null>(null);
  const [deleting, setDeleting] = React.useState<PersonalProviderDTO | null>(null);

  // The section exists only while the admin allows user keys for this account.
  React.useEffect(() => {
    if (access.loaded && !access.enabled) router.replace("/settings/general");
  }, [access.enabled, access.loaded, router]);

  if (!access.loaded || !access.enabled) return null;

  const atLimit = providers.length >= access.maxPerUser;

  return (
    <SettingsPage>
      <SettingsSection
        title={t("title")}
        actions={
          <>
          <Button type="button" size="sm" variant="ghost" className="text-muted-foreground" onClick={() => setLinkOpen(true)}>
            <Link2 className="size-3.5 stroke-1" />
            {t("importLink.open")}
          </Button>
          <Tooltip>
            <TooltipTrigger asChild>
              {/* A disabled button swallows pointer events; the span keeps the limit tooltip reachable. */}
              <span>
                <Button type="button" size="sm" disabled={loading || atLimit} onClick={() => setAddOpen(true)}>
                  <Plus className="size-3.5 stroke-1" />
                  {t("add")}
                </Button>
              </span>
            </TooltipTrigger>
            {atLimit ? <TooltipContent>{t("limitReached", { max: access.maxPerUser })}</TooltipContent> : null}
          </Tooltip>
          </>
        }
      >
        <Table className="table-fixed" style={{ minWidth: 720 }}>
          <colgroup>
            <col style={{ width: 160 }} />
            <col />
            <col style={{ width: 208 }} />
            <col style={{ width: 64 }} />
            <col style={{ width: 80 }} />
            <col style={{ width: 48 }} />
          </colgroup>
          <TableHeader>
            <TableRow>
              <TableHead>{t("columns.name")}</TableHead>
              <TableHead>{t("columns.baseURL")}</TableHead>
              <TableHead>{t("columns.protocol")}</TableHead>
              <TableHead className="text-center">{t("columns.models")}</TableHead>
              <TableHead>{t("columns.status")}</TableHead>
              <TableHead className="w-[48px]" stickyEnd />
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading && providers.length === 0 ? <TableLoadingRow colSpan={6} /> : null}
            {!loading && providers.length === 0 ? <TableEmptyRow colSpan={6}>{t("empty")}</TableEmptyRow> : null}
            {providers.map((provider) => (
              <ProviderRow
                key={provider.id}
                provider={provider}
                busy={busyID === provider.id}
                onEdit={() => setEditing(provider)}
                onCheck={() => void check(provider.id)}
                onToggle={(enabled) => void update(provider.id, { enabled }, enabled ? t("toasts.enabled") : t("toasts.disabled"))}
                onDelete={() => setDeleting(provider)}
              />
            ))}
          </TableBody>
        </Table>
        <p className="text-[11px] leading-relaxed text-muted-foreground">{t("footnote", { max: access.maxPerUser })}</p>
      </SettingsSection>

      <ModelsProviderDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        protocols={access.protocols}
        onProbe={probe}
        onCreate={async (payload) => {
          const created = await create(payload);
          toast.success(t("toasts.added"));
          return created;
        }}
      />

      <ModelsImportLinkDialog open={linkOpen} onOpenChange={setLinkOpen} />

      <ModelsEditDialog
        provider={editing}
        onOpenChange={(open) => !open && setEditing(null)}
        onLoadModels={listModels}
        onSave={(id, payload) => update(id, payload, t("toasts.saved"))}
      />

      <AlertDialog open={deleting !== null} onOpenChange={(open) => !open && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>{t("deleteDialog.description", { name: deleting?.name ?? "" })}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{commonT("actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() => {
                if (deleting) void remove(deleting.id);
                setDeleting(null);
              }}
            >
              {commonT("actions.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsPage>
  );
}

function ProviderRow({
  provider,
  busy,
  onEdit,
  onCheck,
  onToggle,
  onDelete,
}: {
  provider: PersonalProviderDTO;
  busy: boolean;
  onEdit: () => void;
  onCheck: () => void;
  onToggle: (enabled: boolean) => void;
  onDelete: () => void;
}) {
  const t = useTranslations("settings.modelsPage");
  const suspended = provider.status === "suspended";
  const iconSlug = resolveModelProviderIcon(provider.icon, provider.baseURL);
  const iconURL = iconSlug ? resolveModelIconURL(iconSlug) : null;
  const protocolLabel = modelProviderProtocolLabel(provider.protocol, (key) => t(key));

  return (
    <TableRow tone={provider.status === "active" ? undefined : "muted"}>
      <TableCell className="max-w-0">
        <div className="flex min-w-0 items-center gap-2">
          <ModelIcon iconUrl={iconURL} label={provider.name} />
          <span className="truncate font-medium" title={provider.name}>
            {provider.name}
          </span>
        </div>
      </TableCell>
      <TableCell className="max-w-0">
        <span className="block truncate font-mono text-[11px] text-muted-foreground" title={provider.baseURL}>
          {provider.baseURL}
        </span>
      </TableCell>
      <TableCell className="max-w-0">
        <span className="block truncate text-muted-foreground" title={protocolLabel}>
          {protocolLabel}
        </span>
      </TableCell>
      <TableCell className="text-center tabular-nums text-muted-foreground">
        {provider.models.length === 0 ? (
          0
        ) : (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="cursor-default">{provider.models.length}</span>
            </TooltipTrigger>
            <TooltipContent className="max-w-72 whitespace-pre-line">{provider.models.join("\n")}</TooltipContent>
          </Tooltip>
        )}
      </TableCell>
      <TableCell>
        <ProviderStatus provider={provider} />
      </TableCell>
      <TableCell className="w-[48px] whitespace-nowrap" stickyEnd>
        <div className="flex justify-end">
          <DropdownMenu modal={false}>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="ghost" size="icon-xs" className="text-muted-foreground shadow-none" disabled={busy} aria-label={t("actions.more")}>
                <MoreHorizontal className="size-3.5 stroke-1" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem disabled={suspended} onClick={onEdit}>
                <PencilLine className="size-3.5 stroke-1" />
                {t("actions.edit")}
              </DropdownMenuItem>
              <DropdownMenuItem disabled={suspended} onClick={onCheck}>
                <RefreshCw className="size-3.5 stroke-1" />
                {t("actions.check")}
              </DropdownMenuItem>
              {provider.status === "disabled" ? (
                <DropdownMenuItem onClick={() => onToggle(true)}>
                  <CirclePlay className="size-3.5 stroke-1" />
                  {t("actions.enable")}
                </DropdownMenuItem>
              ) : (
                <DropdownMenuItem disabled={suspended} onClick={() => onToggle(false)}>
                  <Ban className="size-3.5 stroke-1" />
                  {t("actions.disable")}
                </DropdownMenuItem>
              )}
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onClick={onDelete}>
                <Trash2 className="size-3.5 stroke-1" />
                {t("actions.delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </TableCell>
    </TableRow>
  );
}

function ProviderStatus({ provider }: { provider: PersonalProviderDTO }) {
  const t = useTranslations("settings.modelsPage");
  const errors = useTranslations("errors");
  const errorKey = provider.lastError ? toErrorMessagePath(provider.lastError).join(".") : "";
  const badgeClass = "border-0 font-normal shadow-none";

  if (provider.status === "suspended") {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge variant="secondary" className={`${badgeClass} text-destructive`}>{t("status.suspended")}</Badge>
        </TooltipTrigger>
        <TooltipContent className="max-w-64">{t("suspendedHint")}</TooltipContent>
      </Tooltip>
    );
  }
  if (provider.status === "disabled") {
    return <Badge variant="secondary" className={`${badgeClass} text-muted-foreground`}>{t("status.disabled")}</Badge>;
  }
  if (errorKey) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge variant="secondary" className={`${badgeClass} text-amber-700 dark:text-amber-300`}>{t("status.error")}</Badge>
        </TooltipTrigger>
        <TooltipContent className="max-w-64">{errors.has(errorKey) ? errors(errorKey) : provider.lastError}</TooltipContent>
      </Tooltip>
    );
  }
  return <Badge variant="secondary" className={badgeClass}>{t("status.active")}</Badge>;
}
