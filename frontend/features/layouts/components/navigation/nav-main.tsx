"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { SidebarGroup, SidebarMenu, useSidebar } from "@/components/ui/sidebar";
import {
  useLayoutNavigationSearch,
  useLayoutNavigationShortcuts,
} from "@/features/layouts/hooks/use-layout-navigation-search";
import { NAVIGATION_ITEMS } from "@/features/layouts/model/navigation-items";
import { NavigationSearch } from "@/features/layouts/components/navigation/navigation-search";
import { NavMainItem } from "@/features/layouts/components/navigation/nav-main-item";

export function NavMain({
  onCreateConversation,
}: {
  onCreateConversation: () => void;
}) {
  const t = useTranslations("common.navigation");
  const { state, isMobile, setOpenMobile } = useSidebar();
  const isCollapsed = !isMobile && state === "collapsed";

  const search = useLayoutNavigationSearch({
    untitled: t("newChat"),
  });

  const onCloseMobileSidebar = React.useCallback(() => {
    setOpenMobile(false);
  }, [setOpenMobile]);

  useLayoutNavigationShortcuts({
    onCreateConversation,
    onOpenSearch: search.openSearch,
  });

  return (
    <>
      <SidebarGroup className="px-2 py-2">
        <SidebarMenu className="gap-0.5">
          {NAVIGATION_ITEMS.filter((item) => item.group === "primary").map((item) => (
            <NavMainItem
              key={item.id}
              item={item}
              title={t(item.id)}
              isCollapsed={isCollapsed}
              isMobile={isMobile}
              onCreateConversation={onCreateConversation}
              onOpenSearch={search.openSearch}
              onCloseMobileSidebar={onCloseMobileSidebar}
            />
          ))}
        </SidebarMenu>

        <SidebarMenu className="mt-4 gap-0.5">
          {NAVIGATION_ITEMS.filter((item) => item.group === "secondary").map((item) => (
            <NavMainItem
              key={item.id}
              item={item}
              title={t(item.id)}
              isCollapsed={isCollapsed}
              isMobile={isMobile}
              onCreateConversation={onCreateConversation}
              onOpenSearch={search.openSearch}
              onCloseMobileSidebar={onCloseMobileSidebar}
            />
          ))}
        </SidebarMenu>
      </SidebarGroup>

      <NavigationSearch
        open={search.open}
        onOpenChange={search.setOpen}
        query={search.query}
        onQueryChange={search.setQuery}
        results={search.results}
        title={t("searchTitle")}
        description={t("searchDescription")}
        placeholder={t("searchPlaceholder")}
        loading={search.loading}
        loadingMore={search.loadingMore}
        loadFailed={search.loadFailed}
        loadMoreFailed={search.loadMoreFailed}
        hasMore={search.hasMore}
        loadingText={t("searchLoading")}
        loadingMoreText={t("searchLoadingMore")}
        loadFailedText={t("searchLoadFailed")}
        loadMoreFailedText={t("searchLoadMoreFailed")}
        emptyText={t("searchEmpty")}
        showPreviewPane
        previewConversationID={search.preview.conversationID}
        previewMessages={search.preview.messages}
        previewLoading={search.preview.loading}
        previewLoadFailed={search.preview.loadFailed}
        onLoadMore={search.loadMore}
        onRetry={search.retrySearch}
        onPreviewChange={search.previewResult}
        onPreviewRetry={search.retryPreview}
        onSelect={search.selectResult}
      />
    </>
  );
}
