"use client";

import * as React from "react";

import { useRecentPage } from "@/features/recent/hooks/use-recent-page";
import { RecentDialogs } from "@/features/recent/components/sections/recent-dialogs";
import { RecentHeader } from "@/features/recent/components/sections/recent-header";
import { RecentList } from "@/features/recent/components/sections/recent-list";
import { RecentToolbar } from "@/features/recent/components/sections/recent-toolbar";

export function AppRecent() {
  const controller = useRecentPage();

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
          onArchiveSelected={controller.archiveSelected}
          onRevokeSelectedShares={controller.revokeSelectedShares}
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
          onRevokeShare={controller.onRevokeShare}
          onDelete={controller.onDelete}
          onRetryLoadMore={controller.retryLoadMore}
        />
      </div>

      <RecentDialogs
        renameTarget={controller.renameTarget}
        renameValue={controller.renameValue}
        deleteTarget={controller.deleteTarget}
        shareTarget={controller.shareTarget}
        onRenameValueChange={controller.setRenameValue}
        onRenameCommit={controller.onRenameCommit}
        onCloseRenameDialog={controller.closeRenameDialog}
        onConfirmDelete={controller.confirmDelete}
        onCloseDeleteDialog={controller.closeDeleteDialog}
        onCloseShareDialog={controller.closeShareDialog}
        onShareChange={controller.onShareChange}
      />
    </div>
  );
}
