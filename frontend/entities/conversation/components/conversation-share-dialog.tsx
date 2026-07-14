"use client";

import { ExternalLink } from "lucide-react";
import { useTranslations } from "next-intl";

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
import type { ConversationShareDTO } from "@/shared/api/conversation.types";
import { CopyActionButton } from "@/shared/components/copy-action";
import { useConversationShareDialog } from "@/entities/conversation/hooks/use-conversation-share-dialog";

type ConversationShareDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  conversationPublicID: string;
  conversationTitle: string;
  defaultMessagePublicIDs?: string[];
  onShareChange?: (share: ConversationShareDTO) => void;
};

export function ConversationShareDialog({
  open,
  onOpenChange,
  conversationPublicID,
  conversationTitle,
  defaultMessagePublicIDs,
  onShareChange,
}: ConversationShareDialogProps) {
  const tCommon = useTranslations("common.actions");
  const t = useTranslations("conversation.shareDialog");
  const {
    active,
    currentURL,
    hasDefaultBranch,
    headerDescription,
    loading,
    runMutation,
    working,
  } = useConversationShareDialog({
    conversationPublicID,
    conversationTitle,
    defaultMessagePublicIDs,
    onShareChange,
    open,
  });

  const openLink = () => {
    if (!currentURL) {
      return;
    }
    window.open(currentURL, "_blank", "noopener,noreferrer");
  };

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
