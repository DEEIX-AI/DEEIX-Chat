"use client"

import * as React from "react"
import { usePathname, useRouter } from "next/navigation"
import { useTranslations } from "next-intl"

import { SidebarGroup, SidebarMenu, useSidebar } from "@/components/ui/sidebar"
import { useChatSession } from "@/features/chat/context/chat-session-context"
import {
  useLayoutNavigationSearch,
  useLayoutNavigationShortcuts,
} from "@/features/layouts/hooks/use-layout-navigation-search"
import { NAVIGATION_ITEMS } from "@/features/layouts/model/navigation-items"
import { NavigationSearch } from "@/features/layouts/components/navigation/navigation-search"
import { NavMainItem } from "@/features/layouts/components/navigation/nav-main-item"
import { useSidebarRecents } from "@/features/recent/context/sidebar-recents-context"

const MAX_SEARCH_RESULTS = 8

export function NavMain() {
  const t = useTranslations("common.navigation")
  const { state, isMobile, setOpenMobile } = useSidebar()
  const router = useRouter()
  const pathname = usePathname()
  const { requestNewConversation } = useChatSession()
  const { items } = useSidebarRecents()
  const isCollapsed = !isMobile && state === "collapsed"

  const search = useLayoutNavigationSearch({
    items,
    maxResults: MAX_SEARCH_RESULTS,
  })

  const onCloseMobileSidebar = React.useCallback(() => {
    setOpenMobile(false)
  }, [setOpenMobile])

  const onCreateConversation = React.useCallback(() => {
    requestNewConversation({ projectID: "" })
    if (pathname === "/chat") {
      window.history.pushState(null, "", "/chat")
      return
    }
    router.push("/chat")
  }, [pathname, requestNewConversation, router])

  useLayoutNavigationShortcuts({
    onCreateConversation,
    onOpenSearch: search.openSearch,
  })

  const primaryItems = React.useMemo(
    () => NAVIGATION_ITEMS.filter((item) => item.group === "primary"),
    [],
  )

  const secondaryItems = React.useMemo(
    () => NAVIGATION_ITEMS.filter((item) => item.group === "secondary"),
    [],
  )

  return (
    <>
      <SidebarGroup>
        <SidebarMenu className="gap-0.2">
          {primaryItems.map((item) => (
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

        <SidebarMenu className="mt-4 gap-0.2">
          {secondaryItems.map((item) => (
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
  )
}
