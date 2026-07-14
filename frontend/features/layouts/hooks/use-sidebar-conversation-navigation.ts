"use client";

import * as React from "react";
import { useRouter } from "next/navigation";

import { useSidebar } from "@/components/ui/sidebar";

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

export function useSidebarConversationNavigation() {
  const router = useRouter();
  const { isMobile, setOpenMobile } = useSidebar();

  return React.useCallback((href: string, event: React.MouseEvent<HTMLAnchorElement>) => {
    if (shouldUseNativeNavigation(event)) {
      return;
    }

    event.preventDefault();
    router.push(href);
    if (isMobile) {
      setOpenMobile(false);
    }
  }, [isMobile, router, setOpenMobile]);
}
