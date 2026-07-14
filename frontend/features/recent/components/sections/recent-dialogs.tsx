"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import type { RecentBulkConfirmAction, RecentDeleteTarget } from "@/features/recent/types/recent";
import { ConversationShareDialog } from "@/entities/conversation";
import { DeleteFilesOption } from "@/shared/components/delete-files-option";
import type { ConversationDTO, ConversationShareDTO } from "@/shared/api/conversation.types";
import { Sparkles } from "@/components/animate-ui/icons/sparkles";
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
import { Spinner } from "@/components/ui/spinner";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";

type RecentDialogsProps = {
  renameTarget: ConversationDTO | null;
  renameValue: string;
  renamingAutomatically: boolean;
  deleteTarget: RecentDeleteTarget;
  deleteFiles: boolean;
  shareTarget: ConversationDTO | null;
  bulkConfirmAction: RecentBulkConfirmAction | null;
  bulkConfirmCount: number;
  bulkConfirmPending: boolean;
  onRenameValueChange: (value: string) => void;
  onRenameCommit: () => void | Promise<void>;
  onAutoRename: () => void | Promise<void>;
  onCloseRenameDialog: () => void;
  onDeleteFilesChange: (checked: boolean) => void;
  onConfirmDelete: () => void | Promise<void>;
  onCloseDeleteDialog: () => void;
  onCloseShareDialog: () => void;
  onShareChange: (share: ConversationShareDTO) => void;
  onCloseBulkConfirm: () => void;
  onConfirmBulkAction: () => void | Promise<void>;
};

export function RecentDialogs({
  renameTarget,
  renameValue,
  renamingAutomatically,
  deleteTarget,
  deleteFiles,
  shareTarget,
  bulkConfirmAction,
  bulkConfirmCount,
  bulkConfirmPending,
  onRenameValueChange,
  onRenameCommit,
  onAutoRename,
  onCloseRenameDialog,
  onDeleteFilesChange,
  onConfirmDelete,
  onCloseDeleteDialog,
  onCloseShareDialog,
  onShareChange,
  onCloseBulkConfirm,
  onConfirmBulkAction,
}: RecentDialogsProps) {
  const t = useTranslations("recent.dialogs");
  const deleteFilesID = React.useId();
  const stableRenameValue = useDialogSnapshot(renameTarget ? renameValue : null) ?? "";
  const stableDeleteTarget = useDialogSnapshot(deleteTarget);
  const stableShareTarget = useDialogSnapshot(shareTarget);
  const bulkConfirmSnapshot = React.useMemo(
    () => bulkConfirmAction ? { action: bulkConfirmAction, count: bulkConfirmCount } : null,
    [bulkConfirmAction, bulkConfirmCount],
  );
  const stableBulkConfirm = useDialogSnapshot(bulkConfirmSnapshot);
  const bulkConfirmCopy = React.useMemo(() => {
    switch (stableBulkConfirm?.action) {
      case "archive":
        return {
          title: t("bulk.archive.title"),
          description: t("bulk.archive.description", { count: stableBulkConfirm.count }),
          confirm: t("bulk.archive.confirm"),
        };
      case "unarchive":
        return {
          title: t("bulk.unarchive.title"),
          description: t("bulk.unarchive.description", { count: stableBulkConfirm.count }),
          confirm: t("bulk.unarchive.confirm"),
        };
      case "revokeShares":
        return {
          title: t("bulk.revokeShares.title"),
          description: t("bulk.revokeShares.description", { count: stableBulkConfirm.count }),
          confirm: t("bulk.revokeShares.confirm"),
        };
      default:
        return { title: "", description: "", confirm: "" };
    }
  }, [stableBulkConfirm, t]);

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
            <div className="relative">
              <Input
                autoFocus
                value={stableRenameValue}
                className="pr-10"
                onChange={(event) => onRenameValueChange(event.target.value)}
                placeholder={t("renamePlaceholder")}
              />
              <button
                type="button"
                className="absolute right-1 top-1/2 flex size-7 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground disabled:pointer-events-none disabled:opacity-60"
                aria-label={t("autoRename")}
                title={t("autoRename")}
                disabled={renamingAutomatically}
                onClick={(event) => {
                  event.preventDefault();
                  void onAutoRename();
                }}
              >
                {renamingAutomatically ? (
                  <Spinner className="size-3.5" />
                ) : (
                  <Sparkles size={15} strokeWidth={1.5} animateOnHover="default" />
                )}
              </button>
            </div>
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
              {t("deleteDescription", { label: stableDeleteTarget?.label || t("thisConversation") })}
            </AlertDialogDescription>
            <DeleteFilesOption
              id={deleteFilesID}
              checked={deleteFiles}
              onCheckedChange={onDeleteFilesChange}
            />
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => void onConfirmDelete()}>
              {t("delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={Boolean(bulkConfirmAction)}
        onOpenChange={(open) => !open && !bulkConfirmPending && onCloseBulkConfirm()}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{bulkConfirmCopy.title}</AlertDialogTitle>
            <AlertDialogDescription>{bulkConfirmCopy.description}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={bulkConfirmPending}>{t("cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={bulkConfirmPending || bulkConfirmCount === 0}
              onClick={(event) => {
                event.preventDefault();
                void onConfirmBulkAction();
              }}
            >
              {bulkConfirmPending ? t("bulk.pending") : bulkConfirmCopy.confirm}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {stableShareTarget ? (
        <ConversationShareDialog
          open={Boolean(shareTarget)}
          onOpenChange={(open) => !open && onCloseShareDialog()}
          conversationPublicID={stableShareTarget.publicID}
          conversationTitle={stableShareTarget.title || t("untitled")}
          onShareChange={onShareChange}
        />
      ) : null}
    </>
  );
}
