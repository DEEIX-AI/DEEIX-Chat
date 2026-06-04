"use client";

import { useTranslations } from "next-intl";

import { ContentHeader } from "@/features/files/components/sections/content-header";
import { ContentPreview } from "@/features/files/components/sections/content-preview";
import { SidebarHeader } from "@/features/files/components/sections/sidebar-header";
import { SidebarList } from "@/features/files/components/sections/sidebar-list";
import { useFilesPage } from "@/features/files/hooks/use-files-page";
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
import { cn } from "@/lib/utils";

const FILES_SIDEBAR_WIDTH_CLASS = "md:w-64 md:basis-64 md:max-w-64 lg:w-72 lg:basis-72 lg:max-w-72";
const FILES_SIDEBAR_COLLAPSED_WIDTH_CLASS = "md:w-12 md:basis-12 md:max-w-12";

export function AppFiles() {
  const tCommon = useTranslations("common.actions");
  const t = useTranslations("files");
  const {
    fileInputRef,
    mobileView,
    files,
    total,
    selectedFile,
    selectedFileID,
    quota,
    loading,
    syncing,
    loadingMore,
    uploading,
    deletingFileID,
    selectedFileIDs,
    bulkDeleteOpen,
    bulkDeleting,
    hasMore,
    query,
    sortKey,
    filterKeys,
    isSidebarCollapsed,
    isSearchOpen,
    renamingFileID,
    renameValue,
    deleteTarget,
    preview,
    extract,
    contentTab,
    openPreview,
    downloadPreview,
    onContentTabChange,
    onOpenUploadPicker,
    onFilesPicked,
    onLoadMore,
    onSelectFile,
    onToggleSidebarCollapsed,
    onToggleSearch,
    onQueryChange,
    onFilterToggle,
    onSortChange,
    onRenameStart,
    onRenameValueChange,
    onRenameCommit,
    onRenameCancel,
    onDeleteRequest,
    onClearDeleteTarget,
    onConfirmDeleteTarget,
    onToggleFileSelection,
    onSelectLoadedFiles,
    onClearFileSelection,
    onBulkDeleteRequest,
    onClearBulkDelete,
    onConfirmBulkDelete,
    onBackToList,
    onToggleRagOptOut,
  } = useFilesPage();

  return (
    <>
      <div className="flex h-full min-h-0 w-full min-w-0 flex-1 overflow-hidden">
        <input ref={fileInputRef} type="file" multiple className="hidden" onChange={onFilesPicked} />

        <aside
          className={cn(
            "h-full min-h-0 min-w-0 shrink-0 overflow-hidden border-border/45 bg-transparent transition-[width,max-width,flex-basis] duration-200",
            "w-full border-r-0 md:border-r",
            isSidebarCollapsed ? FILES_SIDEBAR_COLLAPSED_WIDTH_CLASS : FILES_SIDEBAR_WIDTH_CLASS,
            mobileView === "detail" ? "hidden md:flex" : "flex",
          )}
        >
          <div className={cn("flex min-h-0 min-w-0 flex-1 flex-col px-3 md:px-2", isSidebarCollapsed && "md:px-0")}>
            <div className="hidden md:block">
              <SidebarHeader
                collapsed={isSidebarCollapsed}
                total={total}
                quota={quota}
                query={query}
                searchOpen={isSearchOpen}
                filterKeys={filterKeys}
                sortKey={sortKey}
                uploading={uploading}
                selectedCount={selectedFileIDs.length}
                selectAllDisabled={loading || files.length === 0 || bulkDeleting}
                bulkDeleteDisabled={bulkDeleting}
                onToggleCollapsed={onToggleSidebarCollapsed}
                onToggleSearch={onToggleSearch}
                onQueryChange={onQueryChange}
                onFilterToggle={onFilterToggle}
                onSortChange={onSortChange}
                onSelectLoaded={onSelectLoadedFiles}
                onClearSelection={onClearFileSelection}
                onBulkDeleteRequest={onBulkDeleteRequest}
                onUpload={onOpenUploadPicker}
              />
            </div>

            <div className="md:hidden">
              <SidebarHeader
                collapsed={false}
                showCollapseButton={false}
                total={total}
                quota={quota}
                query={query}
                searchOpen={isSearchOpen}
                filterKeys={filterKeys}
                sortKey={sortKey}
                uploading={uploading}
                selectedCount={selectedFileIDs.length}
                selectAllDisabled={loading || files.length === 0 || bulkDeleting}
                bulkDeleteDisabled={bulkDeleting}
                onToggleCollapsed={onToggleSidebarCollapsed}
                onToggleSearch={onToggleSearch}
                onQueryChange={onQueryChange}
                onFilterToggle={onFilterToggle}
                onSortChange={onSortChange}
                onSelectLoaded={onSelectLoadedFiles}
                onClearSelection={onClearFileSelection}
                onBulkDeleteRequest={onBulkDeleteRequest}
                onUpload={onOpenUploadPicker}
              />
            </div>

            {!isSidebarCollapsed ? (
              <SidebarList
                items={files}
                selectedFileID={selectedFileID}
                selectedFileIDs={selectedFileIDs}
                loading={loading}
                loadingMore={loadingMore}
                hasMore={hasMore}
                syncing={syncing}
                renamingFileID={renamingFileID}
                renameValue={renameValue}
                onSelect={onSelectFile}
                onToggleSelection={onToggleFileSelection}
                onLoadMore={onLoadMore}
                onRenameStart={onRenameStart}
                onRenameValueChange={onRenameValueChange}
                onRenameCommit={onRenameCommit}
                onRenameCancel={onRenameCancel}
                onDeleteRequest={onDeleteRequest}
              />
            ) : null}
          </div>
        </aside>

        <section className={cn(
          "min-h-0 min-w-0 flex-1 flex-col overflow-hidden",
          mobileView === "detail" ? "flex" : "hidden md:flex",
        )}>
          <ContentHeader
            file={selectedFile}
            preview={preview}
            deleting={Boolean(selectedFile && deletingFileID === selectedFile.fileID)}
            onBack={mobileView === "detail" ? onBackToList : undefined}
            onOpen={openPreview}
            onDownload={downloadPreview}
            onDeleteRequest={onDeleteRequest}
            onToggleRagOptOut={onToggleRagOptOut}
          />
          <ContentPreview
            file={selectedFile}
            preview={preview}
            extract={extract}
            contentTab={contentTab}
            onContentTabChange={onContentTabChange}
          />
        </section>
      </div>

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(openState) => {
          if (!openState) {
            onClearDeleteTarget();
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deleteDialog.description", { name: deleteTarget?.fileName || t("deleteDialog.fallbackName") })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{tCommon("cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={onConfirmDeleteTarget}
            >
              {tCommon("delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={bulkDeleteOpen}
        onOpenChange={(openState) => {
          if (!openState) {
            onClearBulkDelete();
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("bulkDeleteDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("bulkDeleteDialog.description", { count: selectedFileIDs.length })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={bulkDeleting}>{tCommon("cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={bulkDeleting}
              onClick={onConfirmBulkDelete}
            >
              {t("bulkDeleteDialog.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
