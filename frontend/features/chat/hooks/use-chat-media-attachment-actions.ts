"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
import { toPendingAttachment } from "@/features/chat/model/message-submit";
import type { ChatModelOption, PendingAttachment } from "@/features/chat/types/chat-runtime";
import type { MessageAttachment } from "@/features/chat/types/messages";
import type { FileObjectDTO } from "@/shared/api/file.types";

/**
 * 生成媒体的再利用：把已生成的图片挂回输入框并在当前模型不支持图片编辑时自动切换到可用模型；
 * 把已生成的视频设为延展输入并切换到支持视频延展的模型；附加历史文件（含数量上限校验）。
 */
export function useChatMediaAttachmentActions({
  attachments,
  maxFilesPerMessage,
  modelOptions,
  selectedModel,
  selectedPlatformModelName,
  setAttachments,
  setSelectedPlatformModelName,
  releaseAttachments,
}: {
  attachments: PendingAttachment[];
  maxFilesPerMessage: number;
  modelOptions: ChatModelOption[];
  selectedModel: ChatModelOption | null;
  selectedPlatformModelName: string;
  setAttachments: React.Dispatch<React.SetStateAction<PendingAttachment[]>>;
  setSelectedPlatformModelName: (platformModelName: string) => void;
  releaseAttachments: (items: PendingAttachment[]) => void;
}) {
  const t = useTranslations("chat");

  const onEditGeneratedImageAttachment = React.useCallback(
    (attachment: MessageAttachment, sourceModelName?: string) => {
      const alreadyAttached = attachments.some((item) => item.fileID === attachment.fileID);
      if (!alreadyAttached && maxFilesPerMessage > 0 && attachments.length >= maxFilesPerMessage) {
        toast.error(t("attachments.limitReached"), {
          description: t("attachments.maxUploadFiles", { count: maxFilesPerMessage }),
        });
        return;
      }

      const pendingAttachment = toPendingAttachment(attachment);
      setAttachments((previous) => {
        if (previous.some((item) => item.fileID === pendingAttachment.fileID)) {
          return previous;
        }
        return [...previous, pendingAttachment];
      });

      const selectedSupportsImageEdit = selectedModel?.kinds.includes("image_edit") ?? false;
      if (!selectedSupportsImageEdit) {
        const normalizedSourceModelName = sourceModelName?.trim() || "";
        const sourceModel = modelOptions.find(
          (item) => item.platformModelName === normalizedSourceModelName && item.kinds.includes("image_edit"),
        );
        const fallbackModel = sourceModel ?? modelOptions.find((item) => item.kinds.includes("image_edit"));
        if (fallbackModel) {
          setSelectedPlatformModelName(fallbackModel.platformModelName);
        }
      }

    },
    [
      attachments,
      maxFilesPerMessage,
      modelOptions,
      selectedModel,
      setAttachments,
      setSelectedPlatformModelName,
      t,
    ],
  );

  const onExtendGeneratedVideoAttachment = React.useCallback(
    (attachment: MessageAttachment, sourceModelName?: string) => {
      const normalizedSourceModelName = sourceModelName?.trim() || "";
      const sourceModel = modelOptions.find(
        (item) =>
          item.platformModelName === normalizedSourceModelName &&
          item.videoExtension?.enabled,
      );
      const extensionModel =
        sourceModel ??
        (selectedModel?.videoExtension?.enabled ? selectedModel : undefined) ??
        modelOptions.find((item) => item.videoExtension?.enabled);

      if (!extensionModel) {
        toast.error(t("submit.mediaMode.blockedDescriptions.video_extension_unsupported"));
        return;
      }

      releaseAttachments(attachments);
      setAttachments([toPendingAttachment(attachment)]);
      if (extensionModel.platformModelName !== selectedPlatformModelName) {
        setSelectedPlatformModelName(extensionModel.platformModelName);
      }
    },
    [
      attachments,
      modelOptions,
      releaseAttachments,
      selectedModel,
      selectedPlatformModelName,
      setAttachments,
      setSelectedPlatformModelName,
      t,
    ],
  );

  const onAttachExistingFile = React.useCallback(
    (file: FileObjectDTO) => {
      const alreadyAttached = attachments.some((item) => item.fileID === file.fileID);
      if (alreadyAttached) {
        return;
      }
      if (maxFilesPerMessage > 0 && attachments.length >= maxFilesPerMessage) {
        toast.error(t("attachments.limitReached"), {
          description: t("attachments.maxUploadFiles", { count: maxFilesPerMessage }),
        });
        return;
      }
      setAttachments((previous) => {
        if (previous.some((item) => item.fileID === file.fileID)) {
          return previous;
        }
        return [
          ...previous,
          {
            fileID: file.fileID,
            fileName: file.fileName,
            mimeType: file.mimeType,
            detectedMime: file.detectedMIME,
            fileCategory: file.fileCategory,
            sizeBytes: file.sizeBytes,
            processingStatus: file.processingStatus,
            processingReady: file.processingReady,
            processingErrorCode: file.processingErrorCode,
            processingErrorMessage: file.processingErrorMessage,
            extractStatus: file.extractStatus,
            embedStatus: file.embedStatus,
            ragReady: false,
            ragReason: "",
            ocrUsed: false,
            ragOptOut: file.ragOptOut,
          },
        ];
      });
    },
    [attachments, maxFilesPerMessage, setAttachments, t],
  );

  return {
    onEditGeneratedImageAttachment,
    onExtendGeneratedVideoAttachment,
    onAttachExistingFile,
  };
}
