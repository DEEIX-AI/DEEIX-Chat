"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { useSidebarConversations } from "@/entities/conversation";
import { SidebarGroup, SidebarMenu, useSidebar } from "@/components/ui/sidebar";
import {
  useLayoutNavigationSearch,
  useLayoutNavigationShortcuts,
} from "@/features/layouts/hooks/use-layout-navigation-search";
import { NAVIGATION_ITEMS } from "@/features/layouts/model/navigation-items";
import { NavigationSearch } from "@/features/layouts/components/navigation/navigation-search";
import { NavMainItem } from "@/features/layouts/components/navigation/nav-main-item";

const MAX_SEARCH_RESULTS = 8;

export function NavMain({
  onCreateConversation,
}: {
  onCreateConversation: () => void;
}) {
  const t = useTranslations("common.navigation");
  const { state, isMobile, setOpenMobile } = useSidebar();
  const { items } = useSidebarConversations();
  const isCollapsed = !isMobile && state === "collapsed";

  const search = useLayoutNavigationSearch({
    items,
    untitled: t("newChat"),
    maxResults: MAX_SEARCH_RESULTS,
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
        loadingText={t("searchLoading")}
        emptyText={t("searchEmpty")}
        onSelect={search.selectResult}
      />
    </>
  );
}
