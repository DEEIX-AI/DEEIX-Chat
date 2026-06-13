"use client";

import * as React from "react";
import { FileBox, Plus, Trash2 } from "lucide-react";
import { useLocale } from "next-intl";
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
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "@/components/ui/input-group";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableEmptyRow,
  TableHead,
  TableHeader,
  TableLoadingRow,
  TableRow,
} from "@/components/ui/table";
import { TablePagination, TableToolbar } from "@/components/ui/table-tools";
import { Textarea } from "@/components/ui/textarea";
import { useVirtualTableRows, VirtualTablePaddingRow } from "@/components/ui/virtual-table";
import { SettingsSection } from "@/shared/components/settings-layout";
import { useAdminPromptPresets } from "@/features/admin/hooks/use-admin-prompt-presets";
import { formatDateTime } from "@/features/admin/utils/account-display";
import type { PromptPresetDTO } from "@/shared/api/prompt-presets.types";
import { PROMPT_PRESET_LIMITS } from "@/shared/model/prompt-presets";

const PROMPT_PRESET_TABLE_COLUMN_COUNT = 6;

function PromptPresetsTable({
  items,
  loading,
  onEdit,
  onDelete,
  onEnabledChange,
}: {
  items: PromptPresetDTO[];
  loading: boolean;
  onEdit: (item: PromptPresetDTO) => void;
  onDelete: (item: PromptPresetDTO) => void;
  onEnabledChange: (item: PromptPresetDTO, checked: boolean) => void;
}) {
  const t = useTranslations("adminPrompts");
  const locale = useLocale();
  const initialLoading = loading && items.length === 0;
  const virtualRows = useVirtualTableRows(items, {
    estimateSize: 40,
  });

  return (
    <Table
      viewportRef={virtualRows.viewportRef}
      viewportClassName={virtualRows.viewportClassName}
      viewportStyle={virtualRows.viewportStyle}
    >
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          <TableHead className="w-[220px]">{t("fields.name")}</TableHead>
          <TableHead className="w-[320px]">{t("fields.description")}</TableHead>
          <TableHead className="w-[96px] text-center">{t("fields.enabled")}</TableHead>
          <TableHead className="w-[160px]">{t("fields.createdAt")}</TableHead>
          <TableHead className="w-[160px]">{t("fields.updatedAt")}</TableHead>
          <TableHead className="w-[56px]" stickyEnd />
        </TableRow>
      </TableHeader>
      <TableBody>
        {initialLoading ? <TableLoadingRow colSpan={PROMPT_PRESET_TABLE_COLUMN_COUNT} /> : null}

        {items.length === 0 && !loading ? (
          <TableEmptyRow colSpan={PROMPT_PRESET_TABLE_COLUMN_COUNT}>{t("empty")}</TableEmptyRow>
        ) : (
          <>
            <VirtualTablePaddingRow colSpan={PROMPT_PRESET_TABLE_COLUMN_COUNT} height={virtualRows.paddingTop} />
            {virtualRows.rows.map(({ item }) => {
              const displayName = item.trigger || item.title;
              const summary = item.description || item.content;

              return (
                <TableRow key={item.id} interactive onClick={() => onEdit(item)}>
                  <TableCell>
                    <div className="flex max-w-[200px] min-w-0 items-center gap-2">
                      <FileBox className="size-4 shrink-0 text-muted-foreground" strokeWidth={1.8} />
                      <span className="min-w-0 truncate font-medium">{displayName}</span>
                    </div>
                  </TableCell>
                  <TableCell>
                    <p className="max-w-[300px] truncate text-muted-foreground">{summary}</p>
                  </TableCell>
                  <TableCell className="text-center">
                    <div className="flex h-7 items-center justify-center">
                      <Switch
                        size="sm"
                        checked={item.enabled}
                        onClick={(event) => event.stopPropagation()}
                        onCheckedChange={(checked) => onEnabledChange(item, checked)}
                        aria-label={item.enabled ? t("disable") : t("enable")}
                      />
                    </div>
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-muted-foreground">
                    {formatDateTime(item.createdAt, locale)}
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-muted-foreground">
                    {formatDateTime(item.updatedAt, locale)}
                  </TableCell>
                  <TableCell stickyEnd>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7 text-muted-foreground shadow-none hover:bg-muted hover:text-destructive"
                      onClick={(event) => {
                        event.stopPropagation();
                        onDelete(item);
                      }}
                      aria-label={t("delete")}
                    >
                      <Trash2 className="size-3.5" strokeWidth={1.6} />
                    </Button>
                  </TableCell>
                </TableRow>
              );
            })}
            <VirtualTablePaddingRow colSpan={PROMPT_PRESET_TABLE_COLUMN_COUNT} height={virtualRows.paddingBottom} />
          </>
        )}
      </TableBody>
    </Table>
  );
}

export function ConversationPromptPresetsSection() {
  const t = useTranslations("adminPrompts");
  const prompts = useAdminPromptPresets();

  return (
    <>
      <SettingsSection title={t("title")}>
        <div className="space-y-4">
          <TableToolbar
            query={prompts.query}
            queryPlaceholder={t("searchPlaceholder")}
            onQueryChange={prompts.setQuery}
            loading={prompts.loading}
            onRefresh={() => void prompts.load()}
          >
            <Button
              type="button"
              size="sm"
              className="h-7 gap-1 text-xs"
              onClick={prompts.openCreate}
              disabled={prompts.loading}
            >
              <Plus className="size-3.5 stroke-1" />
              {t("create")}
            </Button>
          </TableToolbar>

          <PromptPresetsTable
            items={prompts.items}
            loading={prompts.loading}
            onEdit={prompts.openEdit}
            onDelete={prompts.setDeleteTarget}
            onEnabledChange={(target, checked) => void prompts.toggleEnabled(target, checked)}
          />

          <TablePagination
            page={prompts.page}
            pageCount={prompts.pageCount}
            pageSize={prompts.pageSize}
            total={prompts.total}
            onPageChange={prompts.setPage}
            onPageSizeChange={prompts.setPageSize}
            loading={prompts.loading}
          />
        </div>
      </SettingsSection>

      <Dialog open={prompts.dialogOpen} onOpenChange={(open) => !prompts.saving && prompts.setDialogOpen(open)}>
        <DialogContent className="sm:max-w-[560px]">
          <DialogHeader>
            <DialogTitle>{prompts.form.id ? t("editTitle") : t("createTitle")}</DialogTitle>
            <DialogDescription>{t("dialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("fields.name")}</p>
              <InputGroup>
                <InputGroupAddon>/</InputGroupAddon>
                <InputGroupInput
                  value={prompts.form.name}
                  placeholder="musk"
                  maxLength={PROMPT_PRESET_LIMITS.name}
                  onChange={(event) => prompts.setForm((current) => ({ ...current, name: event.target.value }))}
                />
              </InputGroup>
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("fields.description")}</p>
              <Input
                value={prompts.form.description}
                maxLength={PROMPT_PRESET_LIMITS.description}
                onChange={(event) => prompts.setForm((current) => ({ ...current, description: event.target.value }))}
              />
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("fields.content")}</p>
              <Textarea
                value={prompts.form.content}
                className="min-h-40 resize-y"
                maxLength={PROMPT_PRESET_LIMITS.content}
                onChange={(event) => prompts.setForm((current) => ({ ...current, content: event.target.value }))}
              />
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("fields.enabled")}</p>
              <Switch
                size="sm"
                checked={prompts.form.enabled}
                disabled={prompts.saving}
                onCheckedChange={(enabled) => prompts.setForm((current) => ({ ...current, enabled }))}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" disabled={prompts.saving} onClick={() => prompts.setDialogOpen(false)}>{t("cancel")}</Button>
            <Button disabled={prompts.saving} onClick={() => void prompts.save()}>{prompts.saving ? t("saving") : t("save")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={prompts.deleteTarget !== null} onOpenChange={(open) => !open && prompts.setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("deleteDescription")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void prompts.confirmDelete()}>{t("delete")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
