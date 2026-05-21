"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import type { RecentDeleteTarget } from "@/features/recent/types/recent";
import {
  ConversationShareDialog,
} from "@/features/chat/components/sections/conversation-share-dialog";
import type { ConversationDTO, ConversationShareDTO } from "@/shared/api/conversation.types";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
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

type RecentDialogsProps = {
  renameTarget: ConversationDTO | null;
  renameValue: string;
  deleteTarget: RecentDeleteTarget;
  shareTarget: ConversationDTO | null;
  onRenameValueChange: (value: string) => void;
  onRenameCommit: () => void | Promise<void>;
  onCloseRenameDialog: () => void;
  onConfirmDelete: () => void | Promise<void>;
  onCloseDeleteDialog: () => void;
  onCloseShareDialog: () => void;
  onShareChange: (share: ConversationShareDTO) => void;
};

export function RecentDialogs({
  renameTarget,
  renameValue,
  deleteTarget,
  shareTarget,
  onRenameValueChange,
  onRenameCommit,
  onCloseRenameDialog,
  onConfirmDelete,
  onCloseDeleteDialog,
  onCloseShareDialog,
  onShareChange,
}: RecentDialogsProps) {
  const t = useTranslations("recent.dialogs");
  return (
    <>
      <Dialog open={Boolean(renameTarget)} onOpenChange={(open) => !open && onCloseRenameDialog()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("renameTitle")}</DialogTitle>
            <DialogDescription>{t("renameDescription")}</DialogDescription>
          </DialogHeader>
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              void onRenameCommit();
            }}
          >
            <Input
              autoFocus
              value={renameValue}
              onChange={(event) => onRenameValueChange(event.target.value)}
              placeholder={t("renamePlaceholder")}
            />
            <DialogFooter>
              <Button type="button" variant="ghost" onClick={onCloseRenameDialog}>
                {t("cancel")}
              </Button>
              <Button type="submit">{t("save")}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <AlertDialog open={Boolean(deleteTarget)} onOpenChange={(open) => !open && onCloseDeleteDialog()}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("deleteDescription", { label: deleteTarget?.label || t("thisConversation") })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => void onConfirmDelete()}>
              {t("delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {shareTarget ? (
        <ConversationShareDialog
          open={Boolean(shareTarget)}
          onOpenChange={(open) => !open && onCloseShareDialog()}
          conversationPublicID={shareTarget.publicID}
          conversationTitle={shareTarget.title || t("untitled")}
          onShareChange={onShareChange}
        />
      ) : null}
    </>
  );
}
