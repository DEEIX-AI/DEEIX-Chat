"use client";

import { LazyFilePreviewDialog } from "@/entities/file";
import {
  AddKnowledgeBaseFilesDialog,
  BulkDeleteKnowledgeBasesDialog,
  DeleteKnowledgeBaseDialog,
  KnowledgeBaseEditorDialog,
} from "@/features/knowledge-bases/components/sections/knowledge-base-dialogs";
import type { KnowledgeBasePageModel } from "@/features/knowledge-bases/hooks/use-knowledge-base-page";
import type { KnowledgeBaseMode } from "@/features/knowledge-bases/types/knowledge-bases";

// Editor, file picker, delete confirmations and file preview shared by the user
// workspace and the admin knowledge base section.
export function KnowledgeBasePageDialogs({ mode, page }: { mode: KnowledgeBaseMode; page: KnowledgeBasePageModel }) {
  const { detail, editor, addFilesDialog, deleteDialog, bulkDeleteDialog, preview } = page;

  return (
    <>
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
        <LazyFilePreviewDialog
          file={preview.snapshot.file}
          open={preview.open}
          onOpenChange={(open) => {
            if (!open) preview.close();
          }}
          loadContent={preview.loadContent}
        />
      ) : null}
    </>
  );
}
