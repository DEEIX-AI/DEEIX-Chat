"use client";

import type { ComponentProps } from "react";
import { LayoutGroup, motion } from "motion/react";
import { useTranslations } from "next-intl";

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
} from "@/components/ui/sidebar";
import { NavControl } from "@/features/layouts/components/navigation/nav-control";
import { NavMain } from "@/features/layouts/components/navigation/nav-main";
import { NavProjects } from "@/features/layouts/components/navigation/nav-projects";
import { NavRecents } from "@/features/layouts/components/navigation/nav-recents";
import { NavStarred } from "@/features/layouts/components/navigation/nav-starred";
import { NavUser } from "@/features/layouts/components/navigation/nav-user";
import { useOptionalAuthSession } from "@/shared/auth/auth-session-context";
import { resolveAvatarImageSrc } from "@/shared/lib/avatar";
import { brandText } from "@/shared/lib/branding";

export function AppSidebar({
  onCreateConversation,
  ...props
}: ComponentProps<typeof Sidebar> & {
  onCreateConversation: () => void;
}) {
  const t = useTranslations("common.navigation");
  const sessionUser = useOptionalAuthSession()?.user;
  const username = sessionUser?.username.trim() ?? "";
  const user = sessionUser
    ? {
        name: sessionUser.displayName || username || t("fallbackUser"),
        email: sessionUser.email || username || t("fallbackUser"),
        avatar: resolveAvatarImageSrc(sessionUser.avatarURL, sessionUser),
        role: sessionUser.role,
      }
    : {
        name: brandText.title,
        email: "deeix.com",
        avatar: "",
      };

  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader className="group-data-[collapsible=icon]:bg-background">
        <NavControl />
      </SidebarHeader>
      <SidebarContent className="min-h-0 gap-0 overflow-hidden group-data-[collapsible=icon]:bg-background">
        <NavMain onCreateConversation={onCreateConversation} />
        <motion.div
          layoutScroll
          data-sidebar-scroll-root="true"
          className="min-h-0 flex-1 overflow-y-auto [overflow-anchor:none] [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden"
        >
          <LayoutGroup id="sidebar-conversations">
            <NavProjects />
            <NavStarred />
            <NavRecents />
          </LayoutGroup>
        </motion.div>
      </SidebarContent>
      <SidebarFooter className="group-data-[collapsible=icon]:items-center group-data-[collapsible=icon]:bg-background group-data-[collapsible=icon]:px-0">
        <NavUser user={user} />
      </SidebarFooter>
    </Sidebar>
  );
}
