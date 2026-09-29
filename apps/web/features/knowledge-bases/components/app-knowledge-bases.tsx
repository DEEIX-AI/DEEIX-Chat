"use client";

import { KnowledgeBaseDetail } from "@/features/knowledge-bases/components/sections/knowledge-base-detail";
import { KnowledgeBasePageDialogs } from "@/features/knowledge-bases/components/sections/knowledge-base-page-dialogs";
import { KnowledgeBaseSidebar } from "@/features/knowledge-bases/components/sections/knowledge-base-sidebar";
import { useKnowledgeBasePage } from "@/features/knowledge-bases/hooks/use-knowledge-base-page";
import { useIsMobile } from "@/shared/hooks/use-mobile";

const mode = "user";

// User knowledge base workspace. The admin surface lives in features/admin and
// reuses the page model and dialogs through the feature entry point.
export function AppKnowledgeBases() {
  const isMobileViewport = useIsMobile();
  const page = useKnowledgeBasePage(mode);
  const { list, detail } = page;
  const sidebarCollapsed = !isMobileViewport && list.sidebarCollapsed;

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1 overflow-hidden">
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

      <KnowledgeBasePageDialogs mode={mode} page={page} />
    </div>
  );
}
