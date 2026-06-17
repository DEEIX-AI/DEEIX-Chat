"use client";

import * as React from "react";
import { Box, FileBox, Plus, Trash2 } from "lucide-react";
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
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
import { useAdminSkills } from "@/features/admin/hooks/use-admin-skills";
import { useAdminPromptPresets } from "@/features/admin/hooks/use-admin-prompt-presets";
import { formatDateTime } from "@/features/admin/utils/account-display";
import type { PromptPresetDTO } from "@/shared/api/prompt-presets.types";
import type { SkillDTO } from "@/shared/api/skills.types";
import { PROMPT_PRESET_LIMITS } from "@/shared/model/prompt-presets";
import { SKILL_LIMITS } from "@/shared/model/skills";

const PROMPT_PRESET_TABLE_COLUMN_COUNT = 6;
type PromptLibraryType = "prompts" | "skills";

type PromptLibraryRow = {
  id: number;
  title: string;
  trigger: string;
  description: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
};

function PromptLibraryTable<T extends PromptLibraryRow>({
  emptyLabel,
  getSummary,
  icon: Icon,
  items,
  loading,
  onDelete,
  onEdit,
  onEnabledChange,
}: {
  emptyLabel: string;
  getSummary: (item: T) => string;
  icon: React.ComponentType<{ className?: string; strokeWidth?: number }>;
  items: T[];
  loading: boolean;
  onDelete: (item: T) => void;
  onEdit: (item: T) => void;
  onEnabledChange: (item: T, checked: boolean) => void;
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
          <TableEmptyRow colSpan={PROMPT_PRESET_TABLE_COLUMN_COUNT}>{emptyLabel}</TableEmptyRow>
        ) : (
          <>
            <VirtualTablePaddingRow colSpan={PROMPT_PRESET_TABLE_COLUMN_COUNT} height={virtualRows.paddingTop} />
            {virtualRows.rows.map(({ item }) => {
              const displayName = item.trigger || item.title;
              const summary = getSummary(item);

              return (
                <TableRow key={item.id} interactive onClick={() => onEdit(item)}>
                  <TableCell>
                    <div className="flex max-w-[200px] min-w-0 items-center gap-2">
                      <Icon className="size-4 shrink-0 text-muted-foreground" strokeWidth={1.8} />
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
  const [activeType, setActiveType] = React.useState<PromptLibraryType>("skills");
  const prompts = useAdminPromptPresets();
  const skills = useAdminSkills();
  const activeLoading = activeType === "skills" ? skills.loading : prompts.loading;
  const activeQuery = activeType === "skills" ? skills.query : prompts.query;
  const activePage = activeType === "skills" ? skills.page : prompts.page;
  const activePageCount = activeType === "skills" ? skills.pageCount : prompts.pageCount;
  const activePageSize = activeType === "skills" ? skills.pageSize : prompts.pageSize;
  const activeTotal = activeType === "skills" ? skills.total : prompts.total;
  const activeSearchPlaceholder = activeType === "skills" ? t("skillsSearchPlaceholder") : t("searchPlaceholder");
  const activeCreateLabel = activeType === "skills" ? t("createSkill") : t("create");

  return (
    <>
      <SettingsSection title={t("title")}>
        <div className="space-y-4">
          <div className="flex items-center px-0.5">
            <Tabs value={activeType} onValueChange={(value) => setActiveType(value as PromptLibraryType)}>
              <TabsList>
                <TabsTrigger value="skills">{t("types.skills")}</TabsTrigger>
                <TabsTrigger value="prompts">{t("types.prompts")}</TabsTrigger>
              </TabsList>
            </Tabs>
          </div>

          <TableToolbar
            query={activeQuery}
            queryPlaceholder={activeSearchPlaceholder}
            onQueryChange={activeType === "skills" ? skills.setQuery : prompts.setQuery}
            loading={activeLoading}
            onRefresh={() => void (activeType === "skills" ? skills.load() : prompts.load())}
          >
            <Button
              type="button"
              size="sm"
              className="h-7 gap-1 text-xs"
              onClick={activeType === "skills" ? skills.openCreate : prompts.openCreate}
              disabled={activeLoading}
            >
              <Plus className="size-3.5 stroke-1" />
              {activeCreateLabel}
            </Button>
          </TableToolbar>

          {activeType === "skills" ? (
            <PromptLibraryTable<SkillDTO>
              emptyLabel={t("skillsEmpty")}
              getSummary={(item: SkillDTO) => item.description || item.markdown}
              icon={Box}
              items={skills.items}
              loading={skills.loading}
              onEdit={skills.openEdit}
              onDelete={skills.setDeleteTarget}
              onEnabledChange={(target, checked) => void skills.toggleEnabled(target, checked)}
            />
          ) : (
            <PromptLibraryTable<PromptPresetDTO>
              emptyLabel={t("empty")}
              getSummary={(item: PromptPresetDTO) => item.description || item.content}
              icon={FileBox}
              items={prompts.items}
              loading={prompts.loading}
              onEdit={prompts.openEdit}
              onDelete={prompts.setDeleteTarget}
              onEnabledChange={(target, checked) => void prompts.toggleEnabled(target, checked)}
            />
          )}

          <TablePagination
            page={activePage}
            pageCount={activePageCount}
            pageSize={activePageSize}
            total={activeTotal}
            onPageChange={activeType === "skills" ? skills.setPage : prompts.setPage}
            onPageSizeChange={activeType === "skills" ? skills.setPageSize : prompts.setPageSize}
            loading={activeLoading}
          />
        </div>
      </SettingsSection>

      <Dialog open={prompts.dialogOpen} onOpenChange={(open) => !prompts.saving && prompts.setDialogOpen(open)}>
        <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]">
          <DialogHeader className="shrink-0 px-5 pb-3 pt-5">
            <DialogTitle>{prompts.form.id ? t("editTitle") : t("createTitle")}</DialogTitle>
            <DialogDescription>{t("dialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-5 py-2">
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
                className="h-64 resize-none overflow-y-auto [field-sizing:fixed]"
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
          <DialogFooter className="shrink-0 px-5 py-3">
            <Button variant="ghost" disabled={prompts.saving} onClick={() => prompts.setDialogOpen(false)}>{t("cancel")}</Button>
            <Button disabled={prompts.saving} onClick={() => void prompts.save()}>{prompts.saving ? t("saving") : t("save")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={skills.dialogOpen} onOpenChange={(open) => !skills.saving && skills.setDialogOpen(open)}>
        <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[560px]">
          <DialogHeader className="shrink-0 px-5 pb-3 pt-5">
            <DialogTitle>{skills.form.id ? t("editSkillTitle") : t("createSkillTitle")}</DialogTitle>
            <DialogDescription>{t("skillDialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="min-h-0 flex-1 space-y-3 overflow-y-auto px-5 py-2">
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("fields.name")}</p>
              <InputGroup>
                <InputGroupAddon>/</InputGroupAddon>
                <InputGroupInput
                  value={skills.form.name}
                  placeholder="review"
                  maxLength={SKILL_LIMITS.name}
                  onChange={(event) => skills.setForm((current) => ({ ...current, name: event.target.value }))}
                />
              </InputGroup>
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("fields.description")}</p>
              <Input
                value={skills.form.description}
                maxLength={SKILL_LIMITS.description}
                onChange={(event) => skills.setForm((current) => ({ ...current, description: event.target.value }))}
              />
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("fields.skillMarkdown")}</p>
              <Textarea
                value={skills.form.markdown}
                className="h-64 resize-none overflow-y-auto [field-sizing:fixed]"
                maxLength={SKILL_LIMITS.markdown}
                onChange={(event) => skills.setForm((current) => ({ ...current, markdown: event.target.value }))}
              />
            </div>
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground">{t("fields.enabled")}</p>
              <Switch
                size="sm"
                checked={skills.form.enabled}
                disabled={skills.saving}
                onCheckedChange={(enabled) => skills.setForm((current) => ({ ...current, enabled }))}
              />
            </div>
          </div>
          <DialogFooter className="shrink-0 px-5 py-3">
            <Button variant="ghost" disabled={skills.saving} onClick={() => skills.setDialogOpen(false)}>{t("cancel")}</Button>
            <Button disabled={skills.saving} onClick={() => void skills.save()}>{skills.saving ? t("saving") : t("save")}</Button>
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

      <AlertDialog open={skills.deleteTarget !== null} onOpenChange={(open) => !open && skills.setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteSkillTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("deleteSkillDescription")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void skills.confirmDelete()}>{t("delete")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
