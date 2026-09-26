"use client";

import dynamic from "next/dynamic";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import * as React from "react";
import {
  SidebarInset,
  SidebarProvider,
  useSidebarActions,
  useSidebarIsMobile,
  useSidebarMobileOpen,
} from "@/components/ui/sidebar";
import { SidebarConversationsProvider } from "@/entities/conversation";
import { ChatSessionProvider, useChatSession } from "@/features/chat";
import { AppSidebar } from "@/features/layouts/components/navigation/app-sidebar";
import { MobileHeader } from "@/features/layouts/components/sections/mobile-header";
import { LayoutConversationNavigationProvider } from "@/features/layouts/context/layout-conversation-navigation-context";
import { MobileHeaderActionProvider } from "@/features/layouts/context/mobile-header-action-context";
import { AppearancePreferencesSync } from "@/features/settings";
import { UserLocaleSync } from "@/i18n/user-locale-sync";

const AnnouncementDialogHost = dynamic(
  () => import("@/features/announcements").then((mod) => mod.AnnouncementDialogHost),
  { ssr: false },
);

const InitialSecurityGuard = dynamic(
  () => import("@/features/auth").then((mod) => mod.InitialSecurityGuard),
  { ssr: false },
);

function ProjectLayoutShell({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const isMobile = useSidebarIsMobile();
  const openMobile = useSidebarMobileOpen();
  const { setOpenMobile } = useSidebarActions();
  const { requestNewConversation } = useChatSession();
  const routeKey = `${pathname}?${searchParams.toString()}`;
  const previousRouteKeyRef = React.useRef(routeKey);

  React.useEffect(() => {
    if (previousRouteKeyRef.current === routeKey) {
      return;
    }

    previousRouteKeyRef.current = routeKey;
    if (isMobile && openMobile) {
      setOpenMobile(false);
    }
  }, [isMobile, openMobile, routeKey, setOpenMobile]);

  const handleCreateConversation = React.useCallback(() => {
    requestNewConversation({ projectID: "" });
    if (pathname === "/chat") {
      window.history.pushState(null, "", "/chat");
      return;
    }
    router.push("/chat");
  }, [pathname, requestNewConversation, router]);

  return (
    <>
      <AppSidebar onCreateConversation={handleCreateConversation} />
      <SidebarInset>
        <MobileHeader onCreateConversation={handleCreateConversation} />
        <div className="flex h-full min-h-0 flex-1 flex-col gap-4 overflow-hidden pb-2 md:p-4 md:pt-0">
          {children}
        </div>
      </SidebarInset>
    </>
  );
}

export function ProjectLayout({
  children,
  defaultSidebarOpen = true,
}: {
  children: React.ReactNode;
  defaultSidebarOpen?: boolean;
}) {
  const tRecent = useTranslations("recent");

  return (
    <>
      <UserLocaleSync />
      <AppearancePreferencesSync />
      <InitialSecurityGuard />
      <AnnouncementDialogHost />
      <SidebarProvider className="h-svh overflow-hidden" defaultOpen={defaultSidebarOpen}>
        <LayoutConversationNavigationProvider>
          <SidebarConversationsProvider
            bulkPendingTitle={tRecent("dialogs.bulk.pending")}
            newConversationTitle={tRecent("newChat")}
          >
            <ChatSessionProvider>
              <MobileHeaderActionProvider>
                <ProjectLayoutShell>{children}</ProjectLayoutShell>
              </MobileHeaderActionProvider>
            </ChatSessionProvider>
          </SidebarConversationsProvider>
        </LayoutConversationNavigationProvider>
      </SidebarProvider>
    </>
  );
}
