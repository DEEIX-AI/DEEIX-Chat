"use client";

import * as React from "react";
import { ExternalLink } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import {
  createConversationShare,
  getConversationShare,
  regenerateConversationShare,
  revokeConversationShare,
} from "@/shared/api/conversation";
import type { ConversationShareDTO } from "@/shared/api/conversation.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { CopyActionButton } from "@/shared/components/copy-action";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";

type ConversationShareDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  conversationPublicID: string;
  conversationTitle: string;
  defaultMessagePublicIDs?: string[];
  onShareChange?: (share: ConversationShareDTO) => void;
};

function shareURL(shareID: string): string {
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

export function ConversationShareDialog({
  open,
  onOpenChange,
  conversationPublicID,
  conversationTitle,
  defaultMessagePublicIDs,
  onShareChange,
}: ConversationShareDialogProps) {
  const tCommon = useTranslations("common.actions");
  const tChat = useTranslations("chat");
  const t = useTranslations("chat.shareDialog");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [share, setShare] = React.useState<ConversationShareDTO | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [working, setWorking] = React.useState<"create" | "revoke" | "regenerate" | null>(null);
  const active = isActiveShare(share);
  const currentURL = active ? shareURL(share.shareID) : "";
  const snapshotMessageCount = active ? share.messageCount : (defaultMessagePublicIDs?.length ?? 0);
  const normalizedTitle = conversationTitle.trim() || tChat("untitledConversation");
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

  const openLink = React.useCallback(() => {
    if (!currentURL) {
      return;
    }
    window.open(currentURL, "_blank", "noopener,noreferrer");
  }, [currentURL]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <div className="flex items-start justify-between gap-4">
          <DialogHeader className="min-w-0 flex-1">
            <DialogTitle>{t("title")}</DialogTitle>
            <DialogDescription>{headerDescription}</DialogDescription>
          </DialogHeader>
          <Badge variant="secondary">{active ? t("statusShared") : t("statusNotShared")}</Badge>
        </div>

        <div className="space-y-4">
          <div className="space-y-1">
            <p className="text-xs text-muted-foreground">{t("publicLink")}</p>
            <div className="flex items-center gap-2">
              <Input
                readOnly
                value={currentURL || t("emptyLink")}
                className={!currentURL ? "text-muted-foreground" : undefined}
              />
              <CopyActionButton
                type="button"
                variant="ghost"
                size="icon"
                disabled={!active}
                value={currentURL}
                messages={{ copied: t("linkCopied"), failed: t("copyFailed") }}
                iconClassName="size-4"
                aria-label={t("copyLink")}
              />
              <Button
                type="button"
                variant="ghost"
                size="icon"
                disabled={!active}
                onClick={openLink}
                aria-label={t("openLink")}
              >
                <ExternalLink className="size-4" />
              </Button>
            </div>
          </div>
        </div>

        <DialogFooter>
          {active ? (
            <>
              <Button
                type="button"
                variant="ghost"
                onClick={() => void runMutation("revoke")}
                disabled={Boolean(working) || loading}
              >
                {working === "revoke" ? <SpinnerLabel>{t("closing")}</SpinnerLabel> : t("closeShare")}
              </Button>
              <Button
                type="button"
                variant="ghost"
                onClick={() => void runMutation("regenerate")}
                disabled={Boolean(working) || loading || !hasDefaultBranch}
              >
                {working === "regenerate" ? <SpinnerLabel>{t("regenerating")}</SpinnerLabel> : t("regenerate")}
              </Button>
              <CopyActionButton
                type="button"
                value={currentURL}
                messages={{ copied: t("linkCopied"), failed: t("copyFailed") }}
                onCopied={() => onOpenChange(false)}
                disabled={Boolean(working) || loading || !active}
              >
                {t("copyAndClose")}
              </CopyActionButton>
            </>
          ) : (
            <>
              <Button type="button" variant="ghost" onClick={() => onOpenChange(false)} disabled={Boolean(working)}>
                {tCommon("cancel")}
              </Button>
              <Button
                type="button"
                onClick={() => void runMutation("create")}
                disabled={Boolean(working) || loading || !hasDefaultBranch}
              >
                {working === "create" ? <SpinnerLabel>{t("creating")}</SpinnerLabel> : t("createLink")}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
