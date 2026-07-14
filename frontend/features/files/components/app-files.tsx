"use client";

import { useTranslations } from "next-intl";

import { ContentHeader } from "@/features/files/components/sections/content/content-header";
import { ContentPreview } from "@/features/files/components/sections/content/content-preview";
import { SidebarHeader } from "@/features/files/components/sections/sidebar/sidebar-header";
import { SidebarList } from "@/features/files/components/sections/sidebar/sidebar-list";
import { StorageQuotaPanel } from "@/features/files/components/sections/storage/storage-quota-panel";
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
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { useIsMobile } from "@/shared/hooks/use-mobile";
import { cn } from "@/lib/utils";

export function AppFiles() {
  const tCommon = useTranslations("common.actions");
  const t = useTranslations("files");
  const isMobileViewport = useIsMobile();
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
  const stableDeleteTarget = useDialogSnapshot(deleteTarget);
  const selectedCount = selectedFileIDs.length;
  const sidebarCollapsed = !isMobileViewport && isSidebarCollapsed;
  const selectAllDisabled = loading || files.length === 0 || bulkDeleting;
  const contentDeleting = Boolean(selectedFile && deletingFileID === selectedFile.fileID);

  return (
    <>
      <div className="flex h-full min-h-0 w-full min-w-0 flex-1 overflow-hidden">
        <input ref={fileInputRef} type="file" multiple className="hidden" onChange={onFilesPicked} />

        <aside
          className={cn(
            "h-full min-h-0 min-w-0 shrink-0 overflow-hidden border-border/45 bg-transparent transition-[width,max-width,flex-basis] duration-200",
            "w-full border-r-0 md:border-r",
            sidebarCollapsed
              ? "md:w-12 md:basis-12 md:max-w-12"
              : "md:w-64 md:basis-64 md:max-w-64 lg:w-72 lg:basis-72 lg:max-w-72",
            mobileView === "detail" ? "hidden md:flex" : "flex",
          )}
        >
          <div className={cn("flex min-h-0 min-w-0 flex-1 flex-col px-3 md:px-2", sidebarCollapsed && "md:px-0")}>
            <SidebarHeader
              total={total}
              query={query}
              searchOpen={isSearchOpen}
              filterKeys={filterKeys}
              sortKey={sortKey}
              uploading={uploading}
              selectedCount={selectedCount}
              selectAllDisabled={selectAllDisabled}
              bulkDeleteDisabled={bulkDeleting}
              collapsed={sidebarCollapsed}
              showCollapseButton={!isMobileViewport}
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

            {!sidebarCollapsed ? (
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
            {!sidebarCollapsed ? <StorageQuotaPanel quota={quota} /> : null}
          </div>
        </aside>

        <section className={cn(
          "min-h-0 min-w-0 flex-1 flex-col overflow-hidden",
          mobileView === "detail" ? "flex" : "hidden md:flex",
        )}>
          <ContentHeader
            file={selectedFile}
            preview={preview}
            deleting={contentDeleting}
            onBack={mobileView === "detail" ? onBackToList : undefined}
            onOpen={openPreview}
            onDownload={downloadPreview}
            onDeleteRequest={onDeleteRequest}
            onToggleRagOptOut={onToggleRagOptOut}
          />
          <ContentPreview
            file={selectedFile}
            deferEmptyState={loading || syncing}
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
              {t("deleteDialog.description", { name: stableDeleteTarget?.fileName || t("deleteDialog.fallbackName") })}
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
              {t("bulkDeleteDialog.description", { count: selectedCount })}
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
