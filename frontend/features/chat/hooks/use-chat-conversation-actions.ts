"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
import { useConversationExport, useSidebarConversationField } from "@/entities/conversation";
import type { ConversationDTO } from "@/shared/api/conversation.types";
import { parseConversationLabelsJSON } from "@/shared/lib/conversation-labels";

/**
 * 当前会话的操作集合：标题（手动/自动重命名）、星标、标签、归属项目、分享、导出与删除，
 * 以及分享/删除确认对话框的开合状态。
 */
export function useChatConversationActions({
  conversationID,
  currentConversation,
  deleteFilesByDefault,
}: {
  conversationID: string | null;
  currentConversation: ConversationDTO | null;
  deleteFilesByDefault: boolean;
}) {
  const t = useTranslations("chat");
  const router = useRouter();
  const renameByPublicID = useSidebarConversationField("renameByPublicID");
  const regenerateTitleByPublicID = useSidebarConversationField("regenerateTitleByPublicID");
  const updateLabelsByPublicID = useSidebarConversationField("updateLabelsByPublicID");
  const setStarByPublicID = useSidebarConversationField("setStarByPublicID");
  const setProjectByPublicID = useSidebarConversationField("setProjectByPublicID");
  const deleteByPublicID = useSidebarConversationField("deleteByPublicID");

  const [manualConversationTitle, setManualConversationTitle] = React.useState("");
  const [shareDialogOpen, setShareDialogOpen] = React.useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = React.useState(false);
  const [deleteFiles, setDeleteFiles] = React.useState(false);
  const deleteFilesID = React.useId();

  React.useEffect(() => {
    setManualConversationTitle("");
  }, [conversationID]);

  React.useEffect(() => {
    const nextTitle = currentConversation?.title?.trim();
    if (nextTitle) {
      setManualConversationTitle(nextTitle);
    }
  }, [currentConversation?.publicID, currentConversation?.title]);

  const actionConversationID = React.useMemo(() => (conversationID || "").trim(), [conversationID]);
  const canOperateConversation = actionConversationID.length > 0;
  const activeConversationTitle = React.useMemo(
    () => manualConversationTitle || currentConversation?.title?.trim() || t("untitledConversation"),
    [currentConversation?.title, manualConversationTitle, t],
  );
  const activeConversationStarred = Boolean(currentConversation?.isStarred);
  const activeConversationLabels = React.useMemo(
    () => parseConversationLabelsJSON(currentConversation?.labelsJSON ?? "[]"),
    [currentConversation?.labelsJSON],
  );
  const activeConversationShared =
    currentConversation?.shareStatus === "active" && Boolean(currentConversation.shareID?.trim());

  const onToggleActiveConversationStar = React.useCallback(async () => {
    if (!canOperateConversation) {
      return;
    }
    await setStarByPublicID(actionConversationID, !activeConversationStarred);
  }, [actionConversationID, activeConversationStarred, canOperateConversation, setStarByPublicID]);

  const onRenameActiveConversation = React.useCallback(
    async (title: string) => {
      if (!canOperateConversation) {
        return;
      }
      const normalized = title.trim();
      if (!normalized) {
        return;
      }
      const updated = await renameByPublicID(actionConversationID, normalized);
      setManualConversationTitle(updated?.title?.trim() || normalized);
    },
    [actionConversationID, canOperateConversation, renameByPublicID],
  );

  const onAutoRenameActiveConversation = React.useCallback(async () => {
    if (!canOperateConversation) {
      return;
    }
    try {
      const updated = await regenerateTitleByPublicID(actionConversationID);
      if (updated?.title?.trim()) {
        setManualConversationTitle(updated.title.trim());
      }
    } catch (error) {
      toast.error(t("labelMenu.autoRenameFailed"));
      throw error;
    }
  }, [actionConversationID, canOperateConversation, regenerateTitleByPublicID, t]);

  const onUpdateActiveConversationLabels = React.useCallback(
    async (labels: string[]) => {
      if (!canOperateConversation) {
        return;
      }
      const updated = await updateLabelsByPublicID(actionConversationID, labels);
      if (!updated) {
        throw new Error("conversation labels were not updated");
      }
    },
    [actionConversationID, canOperateConversation, updateLabelsByPublicID],
  );

  const onRequestDeleteActiveConversation = React.useCallback(() => {
    if (!canOperateConversation) {
      return;
    }
    setDeleteFiles(deleteFilesByDefault);
    setDeleteDialogOpen(true);
  }, [canOperateConversation, deleteFilesByDefault]);

  const onConfirmDeleteActiveConversation = React.useCallback(async () => {
    if (!canOperateConversation) {
      return;
    }
    const ok = await deleteByPublicID(actionConversationID, { deleteFiles });
    if (ok) {
      setDeleteDialogOpen(false);
      setDeleteFiles(false);
      router.push("/chat");
    }
  }, [actionConversationID, canOperateConversation, deleteByPublicID, deleteFiles, router]);

  const onSetActiveConversationProject = React.useCallback(
    async (projectID?: string) => {
      if (!canOperateConversation) {
        return;
      }
      await setProjectByPublicID(actionConversationID, projectID);
    },
    [actionConversationID, canOperateConversation, setProjectByPublicID],
  );

  const onShareActiveConversation = React.useCallback(() => {
    if (!canOperateConversation) {
      return;
    }
    setShareDialogOpen(true);
  }, [canOperateConversation]);

  const exportActiveConversation = useConversationExport({
    successMessage: t("exportJSONSuccess"),
    failureMessage: t("exportJSONFailed"),
  });

  const onExportActiveConversation = React.useCallback(async () => {
    if (!canOperateConversation) {
      return;
    }
    await exportActiveConversation(actionConversationID);
  }, [actionConversationID, canOperateConversation, exportActiveConversation]);

  return {
    actionConversationID,
    canOperateConversation,
    activeConversationTitle,
    activeConversationStarred,
    activeConversationLabels,
    activeConversationShared,
    shareDialogOpen,
    setShareDialogOpen,
    deleteDialogOpen,
    setDeleteDialogOpen,
    deleteFiles,
    setDeleteFiles,
    deleteFilesID,
    onToggleActiveConversationStar,
    onRenameActiveConversation,
    onAutoRenameActiveConversation,
    onUpdateActiveConversationLabels,
    onRequestDeleteActiveConversation,
    onConfirmDeleteActiveConversation,
    onSetActiveConversationProject,
    onShareActiveConversation,
    onExportActiveConversation,
  };
}
