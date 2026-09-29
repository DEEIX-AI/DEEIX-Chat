"use client";

import * as React from "react";

import { getConversation } from "@/shared/api/conversation";
import type { ConversationDTO } from "@/shared/api/conversation-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

/**
 * Resolves the conversation shown in the chat area. The sidebar list is preferred;
 * a conversation that is not listed there (e.g. opened by link) is fetched once.
 */
export function useChatLoadedConversation(
  conversationID: string | null | undefined,
  activeConversation: ConversationDTO | null,
): ConversationDTO | null {
  const [loadedConversation, setLoadedConversation] = React.useState<ConversationDTO | null>(null);
  React.useEffect(() => {
    const normalizedConversationID = conversationID?.trim() || "";
    if (!normalizedConversationID || activeConversation?.publicID === normalizedConversationID) {
      setLoadedConversation(null);
      return;
    }

    let cancelled = false;
    async function loadConversation() {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }
      const item = await getConversation(token, normalizedConversationID);
      if (cancelled) {
        return;
      }
      setLoadedConversation(item);
    }

    void loadConversation().catch(() => {
      if (!cancelled) {
        setLoadedConversation(null);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [activeConversation?.publicID, conversationID]);

  return activeConversation ?? (loadedConversation?.publicID === conversationID ? loadedConversation : null);
}
