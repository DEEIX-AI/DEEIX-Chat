"use client";

import * as React from "react";
import { toast } from "sonner";

import { downloadConversationExport } from "@/features/chat/model/conversation-export";
import { exportConversation } from "@/shared/api/conversation";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type UseChatConversationExportOptions = {
  successMessage: string;
  failureMessage: string;
};

export function useChatConversationExport({
  successMessage,
  failureMessage,
}: UseChatConversationExportOptions) {
  return React.useCallback(
    async (conversationPublicID: string) => {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }

      try {
        const data = await exportConversation(token, conversationPublicID);
        downloadConversationExport(data);
        toast.success(successMessage);
      } catch (error) {
        toast.error(failureMessage, {
          description: error instanceof Error ? error.message : undefined,
        });
      }
    },
    [failureMessage, successMessage],
  );
}
