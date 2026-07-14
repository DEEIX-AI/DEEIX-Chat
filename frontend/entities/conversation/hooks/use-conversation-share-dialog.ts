"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  createConversationShare,
  getConversationShare,
  regenerateConversationShare,
  revokeConversationShare,
} from "@/shared/api/conversation";
import type { ConversationShareDTO } from "@/shared/api/conversation.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";

function resolveShareURL(shareID: string): string {
  const path = `/share?conversation_id=${encodeURIComponent(shareID)}`;
  if (typeof window === "undefined") {
    return path;
  }
  return `${window.location.origin}${path}`;
}

function isActiveShare(share: ConversationShareDTO | null): share is ConversationShareDTO {
  return Boolean(share?.status === "active" && share.shareID.trim());
}

export function sharePatchFromDTO(share: ConversationShareDTO) {
  const active = isActiveShare(share);
  return {
    shareStatus: share.status,
    shareID: active ? share.shareID : "",
    sharedAt: active ? share.createdAt : null,
    lastShareAccessedAt: share.lastAccessedAt,
  };
}

export function useConversationShareDialog({
  conversationPublicID,
  conversationTitle,
  defaultMessagePublicIDs,
  onShareChange,
  open,
}: {
  conversationPublicID: string;
  conversationTitle: string;
  defaultMessagePublicIDs?: string[];
  onShareChange?: (share: ConversationShareDTO) => void;
  open: boolean;
}) {
  const tCommon = useTranslations("common.actions");
  const tConversation = useTranslations("conversation");
  const t = useTranslations("conversation.shareDialog");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [share, setShare] = React.useState<ConversationShareDTO | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [working, setWorking] = React.useState<"create" | "revoke" | "regenerate" | null>(null);
  const active = isActiveShare(share);
  const currentURL = active ? resolveShareURL(share.shareID) : "";
  const snapshotMessageCount = active ? share.messageCount : (defaultMessagePublicIDs?.length ?? 0);
  const normalizedTitle = conversationTitle.trim() || tConversation("untitled");
  const headerDescription = snapshotMessageCount > 0
    ? t("snapshotMessages", { title: normalizedTitle, count: snapshotMessageCount })
    : normalizedTitle;
  const hasDefaultBranch = defaultMessagePublicIDs === undefined || defaultMessagePublicIDs.length > 0;
  const onShareChangeRef = React.useRef(onShareChange);

  React.useEffect(() => {
    onShareChangeRef.current = onShareChange;
  }, [onShareChange]);

  const applyShare = React.useCallback((next: ConversationShareDTO) => {
    setShare(next);
    onShareChangeRef.current?.(next);
  }, []);

  React.useEffect(() => {
    if (!open || !conversationPublicID.trim()) {
      return;
    }

    let cancelled = false;
    async function loadShare() {
      setLoading(true);
      try {
        const token = await resolveAccessToken();
        if (!token || cancelled) {
          return;
        }
        const data = await getConversationShare(token, conversationPublicID);
        if (!cancelled) {
          applyShare(data);
        }
      } catch (error) {
        if (!cancelled) {
          toast.error(t("loadFailed"), {
            description: resolveErrorMessage(error, tCommon("retry")),
          });
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    void loadShare();
    return () => {
      cancelled = true;
    };
  }, [applyShare, conversationPublicID, open, resolveErrorMessage, t, tCommon]);

  const runMutation = React.useCallback(
    async (mode: "create" | "revoke" | "regenerate") => {
      if (!conversationPublicID.trim() || working) {
        return;
      }
      if ((mode === "create" || mode === "regenerate") && !hasDefaultBranch) {
        toast.error(t("noMessages"));
        return;
      }

      const token = await resolveAccessToken();
      if (!token) {
        toast.error(t("signInRequired"));
        return;
      }

      setWorking(mode);
      try {
        const payload = { defaultMessagePublicIDs };
        const next =
          mode === "create"
            ? await createConversationShare(token, conversationPublicID, payload)
            : mode === "regenerate"
              ? await regenerateConversationShare(token, conversationPublicID, payload)
              : await revokeConversationShare(token, conversationPublicID);
        applyShare(next);
        toast.success(
          mode === "revoke"
            ? t("closed")
            : mode === "regenerate"
              ? t("regenerated")
              : t("created"),
        );
      } catch (error) {
        toast.error(t("operationFailed"), {
          description: resolveErrorMessage(error, tCommon("retry")),
        });
      } finally {
        setWorking(null);
      }
    },
    [applyShare, conversationPublicID, defaultMessagePublicIDs, hasDefaultBranch, resolveErrorMessage, t, tCommon, working],
  );

  return {
    active,
    currentURL,
    hasDefaultBranch,
    headerDescription,
    loading,
    runMutation,
    working,
  };
}
