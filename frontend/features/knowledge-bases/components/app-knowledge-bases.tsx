"use client";

import dynamic from "next/dynamic";

import { AdminKnowledgeBases } from "@/features/knowledge-bases/components/admin-knowledge-bases";
import { KnowledgeBaseDetail } from "@/features/knowledge-bases/components/knowledge-base-detail";
import {
  AddKnowledgeBaseFilesDialog,
  BulkDeleteKnowledgeBasesDialog,
  DeleteKnowledgeBaseDialog,
  KnowledgeBaseEditorDialog,
} from "@/features/knowledge-bases/components/knowledge-base-dialogs";
import { KnowledgeBaseSidebar } from "@/features/knowledge-bases/components/knowledge-base-sidebar";
import { useKnowledgeBasesPage } from "@/features/knowledge-bases/hooks/use-knowledge-bases-page";
import type { KnowledgeBaseMode } from "@/features/knowledge-bases/types/knowledge-bases";
import { useIsMobile } from "@/shared/hooks/use-mobile";

const FilePreviewDialog = dynamic(
  () => import("@/shared/components/file-preview/preview-dialog").then((mod) => mod.FilePreviewDialog),
  { ssr: false },
);

export function AppKnowledgeBases({ mode = "user" }: { mode?: KnowledgeBaseMode }) {
  const isMobileViewport = useIsMobile();
  const page = useKnowledgeBasesPage(mode);
  const { list, detail, editor, addFilesDialog, deleteDialog, bulkDeleteDialog, preview } = page;
  const sidebarCollapsed = !isMobileViewport && list.sidebarCollapsed;

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1 overflow-hidden">
      {mode === "admin" ? <AdminKnowledgeBases page={page} /> : <>
      <KnowledgeBaseSidebar
        mode={mode}
        items={list.items}
        loading={list.loading}
        loadingMore={list.loadingMore}
        hasMore={list.hasMore}
        mobileView={list.mobileView}
        collapsed={sidebarCollapsed}
        showCollapseButton={!isMobileViewport}
        selectedID={list.selectedID}
        selectedIDs={list.selectedIDs}
        sortKey={list.sortKey}
        query={list.query}
        searchOpen={list.searchOpen}
        bulkDeleting={list.bulkDeleting}
        onToggleCollapsed={list.toggleSidebarCollapsed}
        onLoadMore={() => void list.loadMore()}
        onToggleSearch={list.toggleSearch}
        onQueryChange={list.changeQuery}
        onCreate={list.create}
        onSelect={list.select}
        onToggleSelection={list.toggleSelection}
        onSelectAll={list.selectAll}
        onClearSelection={list.clearSelection}
        onSortChange={list.changeSort}
        onBulkDelete={list.requestBulkDelete}
        onEdit={list.edit}
        onDelete={list.requestDelete}
      />
      <KnowledgeBaseDetail
        mode={mode}
        mobileView={list.mobileView}
        selected={detail.selected}
        files={detail.files}
        filesTotal={detail.filesTotal}
        loading={detail.filesLoading}
        loadingMore={detail.filesLoadingMore}
        removingFileID={detail.removingFileID}
        toggling={detail.toggling}
        selectedFileIDs={detail.selectedFileIDs}
        vectorizingFileIDs={detail.vectorizingFileIDs}
        onBack={detail.back}
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
      </>}

      <KnowledgeBaseEditorDialog
        draft={editor.draft}
        saving={editor.saving}
        onDraftChange={editor.change}
        onClose={editor.close}
        onSave={() => void editor.save()}
      />
      <AddKnowledgeBaseFilesDialog
        open={addFilesDialog.open}
        platformFiles={mode === "admin"}
        knowledgeBaseName={detail.selected?.name ?? ""}
        files={addFilesDialog.files}
        loading={addFilesDialog.loading}
        loadingMore={addFilesDialog.loadingMore}
        hasMore={addFilesDialog.hasMore}
        query={addFilesDialog.query}
        selectedFileIDs={addFilesDialog.selectedFileIDs}
        adding={addFilesDialog.adding}
        uploading={addFilesDialog.uploading}
        deletingPlatformFileID={addFilesDialog.deletingPlatformFileID}
        onOpenChange={addFilesDialog.changeOpen}
        onQueryChange={addFilesDialog.changeQuery}
        onSelectedFileIDsChange={addFilesDialog.changeSelection}
        selectionLimit={addFilesDialog.selectionLimit}
        onLoadMore={() => void addFilesDialog.loadMore()}
        onUploadFiles={(files) => void addFilesDialog.upload(files)}
        onDeletePlatformFile={addFilesDialog.deletePlatformFile}
        onConfirm={() => void addFilesDialog.confirm()}
      />
      <DeleteKnowledgeBaseDialog
        target={deleteDialog.target}
        deleting={deleteDialog.deleting}
        deleteFiles={deleteDialog.deleteFiles}
        onClose={deleteDialog.close}
        onDeleteFilesChange={deleteDialog.changeDeleteFiles}
        onConfirm={() => void deleteDialog.confirm()}
      />
      <BulkDeleteKnowledgeBasesDialog
        open={bulkDeleteDialog.open}
        count={bulkDeleteDialog.count}
        hasFiles={bulkDeleteDialog.hasFiles}
        deleting={bulkDeleteDialog.deleting}
        deleteFiles={bulkDeleteDialog.deleteFiles}
        onClose={bulkDeleteDialog.close}
        onDeleteFilesChange={bulkDeleteDialog.changeDeleteFiles}
        onConfirm={() => void bulkDeleteDialog.confirm()}
      />
      {preview.snapshot ? (
        <FilePreviewDialog
          file={preview.snapshot.file}
          open={preview.open}
          onOpenChange={(open) => {
            if (!open) preview.close();
          }}
          loadContent={preview.loadContent}
        />
      ) : null}
    </div>
  );
}
