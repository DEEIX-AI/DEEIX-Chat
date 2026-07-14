"use client";

import * as React from "react";
import dynamic from "next/dynamic";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";

import { SidebarConversationsProvider } from "@/entities/conversation";
import { AppSidebar } from "@/features/layouts/components/navigation/app-sidebar";
import { MobileHeader } from "@/features/layouts/components/sections/mobile-header";
import { ChatSessionProvider, useChatSession } from "@/features/chat";
import { AppearancePreferencesSync } from "@/features/settings";
import { SidebarInset, SidebarProvider, useSidebar } from "@/components/ui/sidebar";
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
  const { isMobile, openMobile, setOpenMobile } = useSidebar();
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
        <SidebarConversationsProvider
          bulkPendingTitle={tRecent("dialogs.bulk.pending")}
          newConversationTitle={tRecent("newChat")}
        >
          <ChatSessionProvider>
            <ProjectLayoutShell>{children}</ProjectLayoutShell>
          </ChatSessionProvider>
        </SidebarConversationsProvider>
      </SidebarProvider>
    </>
  );
}
