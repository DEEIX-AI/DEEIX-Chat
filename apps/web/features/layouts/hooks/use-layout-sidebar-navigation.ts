"use client";

import { useRouter } from "next/navigation";
import * as React from "react";

import { useSidebarActions, useSidebarIsMobile } from "@/components/ui/sidebar";
import { useLayoutConversationNavigation } from "@/features/layouts/context/layout-conversation-navigation-context";

function shouldUseNativeNavigation(event: React.MouseEvent<HTMLAnchorElement>) {
  const target = event.currentTarget.getAttribute("target");
  return event.defaultPrevented ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey ||
    Boolean(target && target !== "_self");
}

export function useLayoutSidebarNavigation() {
  const router = useRouter();
  const isMobile = useSidebarIsMobile();
  const { setOpenMobile } = useSidebarActions();
  const { beginConversationNavigation } = useLayoutConversationNavigation();

  return React.useCallback((conversationID: string, href: string, event: React.MouseEvent<HTMLAnchorElement>) => {
    if (shouldUseNativeNavigation(event)) {
      return;
    }

    event.preventDefault();
    beginConversationNavigation(conversationID);
    router.push(href);
    if (isMobile) {
      setOpenMobile(false);
    }
  }, [beginConversationNavigation, isMobile, router, setOpenMobile]);
}
