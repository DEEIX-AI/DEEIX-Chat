"use client";

import * as React from "react";

import { useRecentPage } from "@/features/recent/hooks/use-recent-page";
import { RecentDialogs } from "@/features/recent/components/sections/recent-dialogs";
import { RecentHeader } from "@/features/recent/components/sections/recent-header";
import { RecentList } from "@/features/recent/components/sections/recent-list";
import { RecentToolbar } from "@/features/recent/components/sections/recent-toolbar";
import type { RecentBulkConfirmAction } from "@/features/recent/types/recent";

export function AppRecent() {
  const controller = useRecentPage();
  const {
    allSelectedArchived,
    archiveSelected,
    revokeSelectedShares,
    selectedConversationIDs,
    selectedSharedCount,
  } = controller;
  const [bulkConfirmAction, setBulkConfirmAction] = React.useState<RecentBulkConfirmAction | null>(null);
  const [bulkConfirmPending, setBulkConfirmPending] = React.useState(false);
  const bulkConfirmCount = bulkConfirmAction === "revokeShares"
    ? selectedSharedCount
    : selectedConversationIDs.length;

  const requestArchiveSelected = React.useCallback(() => {
    setBulkConfirmAction(allSelectedArchived ? "unarchive" : "archive");
  }, [allSelectedArchived]);

  const requestRevokeSelectedShares = React.useCallback(() => {
    setBulkConfirmAction("revokeShares");
  }, []);

  const confirmBulkAction = React.useCallback(async () => {
    if (!bulkConfirmAction) {
      return;
    }

    setBulkConfirmPending(true);
    try {
      if (bulkConfirmAction === "revokeShares") {
        await revokeSelectedShares();
      } else {
        await archiveSelected();
      }
      setBulkConfirmAction(null);
    } finally {
      setBulkConfirmPending(false);
    }
  }, [archiveSelected, bulkConfirmAction, revokeSelectedShares]);

  return (
    <div className="flex h-full min-h-0 w-full flex-1 flex-col overflow-hidden">
      <div className="mx-auto flex h-full min-h-0 w-full max-w-[912px] flex-1 flex-col px-3 pb-8 pt-6 md:pt-15">
        <RecentHeader
          query={controller.query}
          onQueryChange={controller.setQuery}
          onCreateConversation={controller.onCreateConversation}
        />

        <RecentToolbar
          isSelectionMode={controller.isSelectionMode}
          selectedCount={controller.selectedConversationIDs.length}
          selectedSharedCount={controller.selectedSharedCount}
          pageSelectionState={controller.pageSelectionState}
          statusFilter={controller.statusFilter}
          starredFilter={controller.starredFilter}
          shareFilter={controller.shareFilter}
          allSelectedArchived={controller.allSelectedArchived}
          onToggleSelectionMode={controller.toggleSelectionMode}
          onEnterSelectionMode={controller.enterSelectionMode}
          onExitSelectionMode={controller.exitSelectionMode}
          onArchiveSelected={requestArchiveSelected}
          onRevokeSelectedShares={requestRevokeSelectedShares}
          onRequestDeleteSelected={controller.requestDeleteSelected}
          onStatusFilterChange={controller.setStatusFilter}
          onStarredFilterChange={controller.setStarredFilter}
          onShareFilterChange={controller.setShareFilter}
        />

        <RecentList
          loadingInitial={controller.loadingInitial}
          filteredItems={controller.filteredItems}
          normalizedQuery={controller.normalizedQuery}
          statusFilter={controller.statusFilter}
          starredFilter={controller.starredFilter}
          shareFilter={controller.shareFilter}
          projects={controller.projects}
          rowStates={controller.rowStates}
          isSelectionMode={controller.isSelectionMode}
          loadMoreRef={controller.loadMoreRef}
          hasMore={controller.hasMore}
          loadMoreFailed={controller.loadMoreFailed}
          loadingMore={controller.loadingMore}
          onHoverChange={controller.setHoveredConversationID}
          onToggleSelected={controller.onToggleSelected}
          onToggleStar={controller.onToggleStar}
          onRename={controller.onRename}
          onArchive={controller.onArchive}
          onShare={controller.onShare}
          onSetProject={controller.onSetProject}
          onRevokeShare={controller.onRevokeShare}
          onExport={controller.onExport}
          onDelete={controller.onDelete}
          onRetryLoadMore={controller.retryLoadMore}
        />
      </div>

      <RecentDialogs
        renameTarget={controller.renameTarget}
        renameValue={controller.renameValue}
        deleteTarget={controller.deleteTarget}
        deleteFiles={controller.deleteFiles}
        shareTarget={controller.shareTarget}
        bulkConfirmAction={bulkConfirmAction}
        bulkConfirmCount={bulkConfirmCount}
        bulkConfirmPending={bulkConfirmPending}
        onRenameValueChange={controller.setRenameValue}
        onRenameCommit={controller.onRenameCommit}
        onCloseRenameDialog={controller.closeRenameDialog}
        onDeleteFilesChange={controller.setDeleteFiles}
        onConfirmDelete={controller.confirmDelete}
        onCloseDeleteDialog={controller.closeDeleteDialog}
        onCloseShareDialog={controller.closeShareDialog}
        onShareChange={controller.onShareChange}
        onCloseBulkConfirm={() => setBulkConfirmAction(null)}
        onConfirmBulkAction={confirmBulkAction}
      />
    </div>
  );
}
