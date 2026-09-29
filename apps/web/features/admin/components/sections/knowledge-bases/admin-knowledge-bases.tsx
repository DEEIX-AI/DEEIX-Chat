"use client";

import { BookOpen, FolderOpen, HardDrive, MoreHorizontal, PencilLine, Plus, Trash2 } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import * as React from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldLabel } from "@/components/ui/field";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogHeightTransition,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Spinner } from "@/components/ui/spinner";
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
import { TableToolbar } from "@/components/ui/table-tools";
import { useAdminKnowledgeBasesFeature } from "@/features/admin/hooks/use-admin-knowledge-bases-feature";
import { AdminPlatformFilesDialog } from "@/features/admin/components/sections/knowledge-bases/knowledge-bases-platform-files-dialog";
import {
  KnowledgeBaseDetail,
  KnowledgeBasePageDialogs,
  type KnowledgeBasePageModel,
  useKnowledgeBasePage,
} from "@/features/knowledge-bases";
import { SettingsFieldRow, SettingsPage, SettingsSection } from "@/shared/components/settings-layout";
import { isOneOf } from "@/shared/lib/type-guards";

type KnowledgeBaseSortKey = KnowledgeBasePageModel["list"]["sortKey"];

const KNOWLEDGE_BASE_SORT_KEYS = ["default", "updated", "created", "name", "files"] as const satisfies readonly KnowledgeBaseSortKey[];
const isKnowledgeBaseSortKey = isOneOf(KNOWLEDGE_BASE_SORT_KEYS);

export function AdminKnowledgeBasesPage() {
  const page = useKnowledgeBasePage("admin");
  const t = useTranslations("knowledgeBases");
  const locale = useLocale();
  const [detailOpen, setDetailOpen] = React.useState(false);
  const [platformFilesOpen, setPlatformFilesOpen] = React.useState(false);
  const { list, detail } = page;
  const selectedIDs = React.useMemo(() => new Set(list.selectedIDs), [list.selectedIDs]);
  const dateFormatter = React.useMemo(
    () => new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }),
    [locale],
  );

  const openDetail = React.useCallback((publicID: string) => {
    list.select(publicID);
    setDetailOpen(true);
  }, [list]);

  const { featureEnabled, featureSaving, toggleFeature } = useAdminKnowledgeBasesFeature();

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1 overflow-hidden">
      <SettingsPage className="min-w-0 flex-1 overflow-y-auto">
        <SettingsSection title={t("title")}>
          <SettingsFieldRow
            title={t("featureToggle.label")}
            description={t("featureToggle.description")}
            controlClassName="items-end"
          >
            <Switch
              checked={featureEnabled ?? true}
              disabled={featureSaving || featureEnabled === null}
              onCheckedChange={(checked) => void toggleFeature(checked)}
              aria-label={t("featureToggle.label")}
            />
          </SettingsFieldRow>

          <Field className="gap-2">
            <FieldLabel>{t("adminTitle")}</FieldLabel>
            <TableToolbar
              query={list.query}
              onQueryChange={list.changeQuery}
              queryPlaceholder={t("searchPlaceholder")}
              sort={{
                value: list.sortKey,
                onValueChange: (value) => {
                  if (isKnowledgeBaseSortKey(value)) list.changeSort(value);
                },
                options: [
                  { value: "default", label: t("sort.default") },
                  { value: "updated", label: t("sort.updated") },
                  { value: "created", label: t("sort.created") },
                  { value: "name", label: t("sort.name") },
                  { value: "files", label: t("sort.files") },
                ],
              }}
              selectedCount={list.selectedIDs.length}
              bulkActions={[{
                key: "delete",
                label: t("bulkDeleteAction"),
                icon: <Trash2 className="size-3.5 stroke-1" />,
                onClick: list.requestBulkDelete,
                disabled: list.bulkDeleting,
              }]}
              loading={list.loading || list.bulkDeleting}
              onRefresh={list.refresh}
            >
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="h-7 gap-1 px-2 text-xs shadow-none"
                onClick={() => setPlatformFilesOpen(true)}
              >
                <HardDrive className="size-3.5 stroke-1" />
                {t("localFiles")}
              </Button>
              <Button type="button" size="sm" className="h-7 gap-1 px-2 text-xs" onClick={list.create}>
                <Plus className="size-3.5 stroke-1" />
                {t("create")}
              </Button>
            </TableToolbar>

            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-[44px] py-1.5 text-center">
                    <div className="flex h-7 items-center justify-center">
                      <Checkbox
                        checked={
                          list.items.length > 0 && list.selectedIDs.length === list.items.length
                            ? true
                            : list.selectedIDs.length > 0
                              ? "indeterminate"
                              : false
                        }
                        onCheckedChange={(checked) => checked ? list.selectAll() : list.clearSelection()}
                        aria-label={t("selectAll")}
                      />
                    </div>
                  </TableHead>
                  <TableHead className="min-w-[260px]">{t("columns.name")}</TableHead>
                  <TableHead className="w-[112px] text-center">{t("columns.sources")}</TableHead>
                  <TableHead className="w-[88px] text-center">{t("columns.status")}</TableHead>
                  <TableHead className="w-[160px]">{t("columns.updated")}</TableHead>
                  <TableHead stickyEnd className="w-[56px]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.loading && list.items.length === 0 ? <TableLoadingRow colSpan={6} /> : null}
                {!list.loading && list.items.length === 0 ? (
                  <TableEmptyRow colSpan={6}>{list.query.trim() ? t("searchEmpty") : t("empty")}</TableEmptyRow>
                ) : null}
                {list.items.map((item) => (
                  <TableRow
                    key={item.publicID}
                    interactive
                    selected={detail.selected?.publicID === item.publicID && detailOpen}
                    tone={!item.enabled ? "muted" : undefined}
                    onClick={() => openDetail(item.publicID)}
                  >
                    <TableCell className="w-[44px] py-1.5 text-center" onClick={(event) => event.stopPropagation()}>
                      <div className="flex h-7 items-center justify-center">
                        <Checkbox
                          checked={selectedIDs.has(item.publicID)}
                          onCheckedChange={(checked) => list.toggleSelection(item.publicID, checked === true)}
                          aria-label={t("selectKnowledgeBase")}
                        />
                      </div>
                    </TableCell>
                    <TableCell className="max-w-[460px] py-1.5">
                      <div className="flex min-w-0 items-center gap-2.5">
                        <BookOpen className="size-4 shrink-0 text-muted-foreground" strokeWidth={1.4} />
                        <div className="min-w-0">
                          <p className="truncate font-medium text-foreground">{item.name}</p>
                          <p className="truncate text-[11px] leading-4 text-muted-foreground">
                            {item.description || t("adminDefaultDescription")}
                          </p>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell className="py-1.5 text-center">
                      <span className="tabular-nums text-foreground">{item.readyFileCount}</span>
                      <span className="text-muted-foreground"> / {item.fileCount}</span>
                    </TableCell>
                    <TableCell className="py-1.5 text-center">
                      <Badge variant="secondary" className="border-0 font-normal shadow-none">
                        {item.enabled ? t("enabled") : t("disabled")}
                      </Badge>
                    </TableCell>
                    <TableCell className="py-1.5 text-muted-foreground">
                      {dateFormatter.format(new Date(item.updatedAt))}
                    </TableCell>
                    <TableCell stickyEnd className="w-[56px] py-1.5 text-right" onClick={(event) => event.stopPropagation()}>
                      <div className="flex h-7 items-center justify-end">
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button type="button" size="icon-xs" variant="ghost" className="text-muted-foreground shadow-none" aria-label={t("moreActions")}>
                              <MoreHorizontal className="size-3.5 stroke-1" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem onClick={() => openDetail(item.publicID)}>
                              <FolderOpen className="size-3.5 stroke-1" />
                              {t("manage")}
                            </DropdownMenuItem>
                            <DropdownMenuItem onClick={() => list.edit(item)}>
                              <PencilLine className="size-3.5 stroke-1" />
                              {t("editTitle")}
                            </DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem variant="destructive" onClick={() => list.requestDelete(item)}>
                              <Trash2 className="size-3.5 stroke-1" />
                              {t("delete")}
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {list.hasMore ? (
              <div className="flex justify-center">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 text-xs text-muted-foreground"
                  disabled={list.loadingMore}
                  onClick={() => void list.loadMore()}
                >
                  {list.loadingMore ? <Spinner className="size-3" /> : t("loadMore")}
                </Button>
              </div>
            ) : null}
          </Field>
        </SettingsSection>

        <Dialog open={detailOpen} onOpenChange={setDetailOpen}>
          <DialogContent className="gap-0 overflow-hidden p-0 sm:max-w-[920px]">
            <DialogHeightTransition contentClassName="h-[min(78dvh,720px)] max-h-[min(78dvh,720px)]">
              <DialogHeader className="sr-only">
                <DialogTitle>{detail.selected?.name ?? t("adminTitle")}</DialogTitle>
                <DialogDescription>{t("manageDescription")}</DialogDescription>
              </DialogHeader>
              <KnowledgeBaseDetail
                mode="admin"
                mobileView="detail"
                selected={detail.selected}
                files={detail.files}
                filesTotal={detail.filesTotal}
                loading={detail.filesLoading}
                loadingMore={detail.filesLoadingMore}
                removingFileID={detail.removingFileID}
                toggling={detail.toggling}
                selectedFileIDs={detail.selectedFileIDs}
                vectorizingFileIDs={detail.vectorizingFileIDs}
                onBack={() => setDetailOpen(false)}
                onAddFiles={detail.addFiles}
                onLoadMore={detail.loadMoreFiles}
                onRemoveFile={detail.removeFile}
                onToggleEnabled={detail.toggleBuiltinEnabled}
                onPreviewFile={detail.previewFile}
                onToggleFileSelection={detail.toggleFileSelection}
                onSelectVectorizableFiles={detail.selectVectorizableFiles}
                onClearFileSelection={detail.clearFileSelection}
                onVectorizeFile={detail.vectorizeFile}
                onVectorizeSelectedFiles={detail.vectorizeSelectedFiles}
              />
            </DialogHeightTransition>
          </DialogContent>
        </Dialog>
        <AdminPlatformFilesDialog open={platformFilesOpen} onOpenChange={setPlatformFilesOpen} />
      </SettingsPage>
      <KnowledgeBasePageDialogs mode="admin" page={page} />
    </div>
  );
}
