"use client";

import * as React from "react";

import { useShellConversationNavigation } from "@/features/shell/context/shell-conversation-navigation-context";

export function useShellActiveConversation() {
  const { activeConversationID } = useShellConversationNavigation();
  const previousActiveConversationIDRef = React.useRef<string | null>(null);

  React.useEffect(() => {
    if (!activeConversationID || previousActiveConversationIDRef.current === activeConversationID) {
      return;
    }
    previousActiveConversationIDRef.current = activeConversationID;

    const activeItem = document.querySelector<HTMLElement>(
      `[data-sidebar-conversation-id="${CSS.escape(activeConversationID)}"][data-sidebar-active="true"]`,
    );
    if (!activeItem) {
      return;
    }

    const frameID = requestAnimationFrame(() => {
      activeItem.scrollIntoView({
        block: "nearest",
        inline: "nearest",
      });
    });

    return () => cancelAnimationFrame(frameID);
  }, [activeConversationID]);

  return activeConversationID;
}
