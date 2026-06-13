"use client"

import * as React from "react"
import { LayoutGroup, motion } from "motion/react"

import { useLayoutSidebarData } from "@/features/layouts/hooks/use-layout-sidebar-data"
import { NavControl } from "@/features/layouts/components/navigation/nav-control"
import { NavMain } from "@/features/layouts/components/navigation/nav-main"
import { NavProjects } from "@/features/layouts/components/navigation/nav-projects"
import { NavStarred } from "@/features/layouts/components/navigation/nav-starred"
import { NavRecents } from "@/features/layouts/components/navigation/nav-recents"
import { NavUser } from "@/features/layouts/components/navigation/nav-user"
import { Spinner } from "@/components/ui/spinner"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
} from "@/components/ui/sidebar"

const data = {
  user: {
    name: "DEEIX Chat",
    email: "deeix.com",
    avatar: "",
  },
}

function SidebarSectionFallback() {
  return (
    <div className="px-2 py-2">
      <Spinner className="size-3.5" />
    </div>
  )
}

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const sidebarData = useLayoutSidebarData()
  const user = sidebarData.user ?? data.user

  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader className="group-data-[collapsible=icon]:bg-background">
        <NavControl />
      </SidebarHeader>
      <SidebarContent className="min-h-0 overflow-hidden group-data-[collapsible=icon]:bg-background">
        <NavMain />
        <motion.div
          layoutScroll
          data-sidebar-scroll-root="true"
          className="min-h-0 flex-1 overflow-y-auto [overflow-anchor:none] [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden"
        >
          <LayoutGroup id="sidebar-conversations">
            <React.Suspense fallback={<SidebarSectionFallback />}>
              <NavProjects />
            </React.Suspense>
            <React.Suspense fallback={<SidebarSectionFallback />}>
              <NavStarred />
            </React.Suspense>
            <React.Suspense fallback={<SidebarSectionFallback />}>
              <NavRecents />
            </React.Suspense>
          </LayoutGroup>
        </motion.div>
      </SidebarContent>
      <SidebarFooter className="group-data-[collapsible=icon]:items-center group-data-[collapsible=icon]:bg-background group-data-[collapsible=icon]:px-0">
        <NavUser user={user} />
      </SidebarFooter>
    </Sidebar>
  )
}
