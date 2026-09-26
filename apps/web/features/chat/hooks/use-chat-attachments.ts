"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
import type {
  PendingAttachment,
  UploadingAttachment,
} from "@/features/chat/types/chat-runtime";
import {
  inferUploadCategory,
  normalizeUploadMime,
  resolveUploadPolicyRejection,
} from "@/features/chat/utils/attachments";
import { captureScreenshotFile } from "@/features/chat/utils/browser-media";
import { resolveMaxFilesPerMessage } from "@/features/chat/utils/chat-runtime";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import {
  getChatFilePolicy,
  uploadFile,
} from "@/shared/api/file";
import type { ChatFilePolicyDTO, FileProcessingStatusDTO } from "@/shared/api/file.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  type FileStatusPollingResult,
  useFileProcessingStatusPolling,
} from "@/shared/hooks/use-file-processing-status-polling";
import { runSettledItemsWithConcurrency } from "@/shared/lib/bulk-action";
import { isFileProcessing } from "@/shared/lib/file-processing";
import { createSecureUUID } from "@/shared/lib/secure-id";

function revokeAttachmentPreview(item: PendingAttachment) {
  if (item.previewURL) {
    URL.revokeObjectURL(item.previewURL);
  }
}

export function useChatAttachments({
  conversationKey,
  attachments,
  setAttachments,
  appendAttachmentsForKey,
  temporary = false,
}: {
  conversationKey: string;
  attachments: PendingAttachment[];
  setAttachments: React.Dispatch<React.SetStateAction<PendingAttachment[]>>;
  appendAttachmentsForKey: (conversationKey: string, items: PendingAttachment[]) => void;
  temporary?: boolean;
}) {
  const t = useTranslations("chat.attachments");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [uploadingByKey, setUploadingByKey] = React.useState<Record<string, UploadingAttachment[]>>({});
  const [maxFilesPerMessage, setMaxFilesPerMessage] = React.useState(() => resolveMaxFilesPerMessage());
  const [chatFilePolicy, setChatFilePolicy] = React.useState<ChatFilePolicyDTO | null>(null);
  const attachmentsRef = React.useRef<PendingAttachment[]>(attachments);
  const previousAttachmentsRef = React.useRef<PendingAttachment[]>(attachments);
  const transferredPreviewURLsRef = React.useRef(new Set<string>());
  const uploadControllersRef = React.useRef(new Set<AbortController>());
  const mountedRef = React.useRef(true);
  const currentConversationKeyRef = React.useRef(conversationKey);
  const uploadingAttachments = uploadingByKey[conversationKey] ?? [];
  const uploading = uploadingAttachments.length > 0;
  const processingFileIDs = React.useMemo(
    () => attachments
      .filter(isFileProcessing)
      .map((item) => item.fileID),
    [attachments],
  );

  const onProcessingResult = React.useCallback(({
    statuses,
    missingFileIDs,
  }: FileStatusPollingResult<FileProcessingStatusDTO>) => {
    const statusesByID = new Map(statuses.map((status) => [status.fileID, status]));
    const missingFileIDSet = new Set(missingFileIDs);
    setAttachments((current) => {
      let changed = false;
      const next: PendingAttachment[] = [];
      for (const item of current) {
        if (missingFileIDSet.has(item.fileID)) {
          changed = true;
          continue;
        }
        const status = statusesByID.get(item.fileID);
        if (!status) {
          next.push(item);
          continue;
        }
        if (
          item.detectedMime === status.detectedMIME &&
          item.fileCategory === status.fileCategory &&
          item.processingStatus === status.processingStatus &&
          item.processingReady === status.processingReady &&
          item.processingErrorCode === status.errorCode &&
          item.processingErrorMessage === status.errorMessage &&
          item.extractStatus === status.extractStatus &&
          item.embedStatus === status.embedStatus &&
          item.ragReady === status.ragReady &&
          item.ragReason === status.ragReason &&
          item.ocrUsed === status.ocrUsed
        ) {
          next.push(item);
          continue;
        }
        changed = true;
        next.push({
          ...item,
          detectedMime: status.detectedMIME,
          fileCategory: status.fileCategory,
          processingStatus: status.processingStatus,
          processingReady: status.processingReady,
          processingErrorCode: status.errorCode,
          processingErrorMessage: status.errorMessage,
          extractStatus: status.extractStatus,
          embedStatus: status.embedStatus,
          ragReady: status.ragReady,
          ragReason: status.ragReason,
          ocrUsed: status.ocrUsed,
        });
      }
      return changed ? next : current;
    });
  }, [setAttachments]);

  useFileProcessingStatusPolling({
    fileIDs: processingFileIDs,
    intervalMs: 1500,
    onResult: onProcessingResult,
  });

  React.useEffect(() => {
    const controller = new AbortController();
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token || controller.signal.aborted) {
          return;
        }
        const policy = await getChatFilePolicy(token, controller.signal);
        if (controller.signal.aborted) {
          return;
        }
        setChatFilePolicy(policy);
        if (policy.maxMessageFiles > 0) {
          setMaxFilesPerMessage(policy.maxMessageFiles);
        }
      } catch {
        // Keep fallback value.
      }
    })();
    return () => controller.abort();
  }, []);

  React.useEffect(() => {
    attachmentsRef.current = attachments;
  }, [attachments]);

  React.useEffect(() => {
    currentConversationKeyRef.current = conversationKey;
  }, [conversationKey]);

  React.useEffect(() => {
    const previous = previousAttachmentsRef.current;
    const currentPreviewURLs = new Map(attachments.map((item) => [item.fileID, item.previewURL]));
    for (const item of previous) {
      if (!item.previewURL || currentPreviewURLs.get(item.fileID) === item.previewURL) {
        continue;
      }
      if (!transferredPreviewURLsRef.current.has(item.previewURL)) {
        revokeAttachmentPreview(item);
      }
    }
    for (const previewURL of currentPreviewURLs.values()) {
      if (previewURL) {
        transferredPreviewURLsRef.current.delete(previewURL);
      }
    }
    previousAttachmentsRef.current = attachments;
  }, [attachments]);

  const transferAttachments = React.useCallback((items: PendingAttachment[]) => {
    for (const item of items) {
      if (item.previewURL) {
        transferredPreviewURLsRef.current.add(item.previewURL);
      }
    }
  }, []);

  const releaseAttachments = React.useCallback((items: PendingAttachment[]) => {
    for (const item of items) {
      if (item.previewURL) {
        transferredPreviewURLsRef.current.delete(item.previewURL);
      }
      revokeAttachmentPreview(item);
    }
  }, []);

  const onRemoveAttachment = React.useCallback((fileID: string) => {
    setAttachments((current) => current.filter((item) => item.fileID !== fileID));
  }, [setAttachments]);

  const onUploadFiles = React.useCallback(
    async (files: File[]) => {
      if (files.length === 0 || uploading) {
        return;
      }
      const targetConversationKey = conversationKey;
      const targetUploadingCount = uploadingByKey[targetConversationKey]?.length ?? 0;
      const remainingSlots = maxFilesPerMessage - attachments.length - targetUploadingCount;
      if (remainingSlots <= 0) {
        toast.error(t("limitReached"), {
          description: t("maxUploadFiles", { count: maxFilesPerMessage }),
        });
        return;
      }
      const policyAcceptedFiles: File[] = [];
      let overflowCount = 0;
      const policyLabels = {
        mimeNotAllowed: t("policy.mimeNotAllowed"),
        fullContextLimitExceeded: (limit: string) => t("policy.fullContextLimitExceeded", { limit }),
        sizeLimitExceeded: (limit: string) => t("policy.sizeLimitExceeded", { limit }),
      };
      for (const file of files) {
        const rejection = temporary && normalizeUploadMime(file).startsWith("video/")
          ? t("temporaryVideoUnsupported")
          : resolveUploadPolicyRejection(file, chatFilePolicy, policyLabels);
        if (rejection) {
          toast.error(t("policyRejected"), {
            description: t("fileRejected", { name: file.name, reason: rejection }),
          });
          continue;
        }
        if (policyAcceptedFiles.length >= remainingSlots) {
          overflowCount += 1;
          continue;
        }
        policyAcceptedFiles.push(file);
      }
      if (overflowCount > 0) {
        toast(t("autoTruncated"), {
          description: t("autoTruncatedDescription", { max: maxFilesPerMessage, count: overflowCount }),
        });
      }
      if (policyAcceptedFiles.length === 0) {
        return;
      }

      if (temporary) {
        const localAttachments = policyAcceptedFiles.map((file): PendingAttachment => {
          const category = inferUploadCategory(file);
          return {
            fileID: `temporary_${createSecureUUID()}`,
            fileName: file.name,
            mimeType: file.type || "application/octet-stream",
            detectedMime: file.type || "application/octet-stream",
            fileCategory: category,
            sizeBytes: file.size,
            previewURL: category === "image" ? URL.createObjectURL(file) : undefined,
            processingStatus: "ready",
            processingReady: true,
            extractStatus: "none",
            embedStatus: "none",
            ragReady: false,
            ragOptOut: false,
            localFile: file,
          };
        });
        appendAttachmentsForKey(targetConversationKey, localAttachments);
        return;
      }

      const batchPrefix = `${Date.now()}-${Math.random().toString(16).slice(2)}`;
      const placeholders = policyAcceptedFiles.map((file, index) => ({
        tempID: `${batchPrefix}-${index}`,
        fileName: file.name,
        sizeBytes: file.size,
      }));
      setUploadingByKey((prev) => ({
        ...prev,
        [targetConversationKey]: [...(prev[targetConversationKey] ?? []), ...placeholders],
      }));

      const controller = new AbortController();
      uploadControllersRef.current.add(controller);
      try {
        const token = await resolveAccessToken();
        if (controller.signal.aborted || !mountedRef.current) {
          return;
        }
        if (!token) {
          toast.error(t("uploadFailed"), { description: t("uploadSignInRequired") });
          return;
        }

        const results = await runSettledItemsWithConcurrency({
          items: policyAcceptedFiles,
          signal: controller.signal,
          runItem: (file) => uploadFile(token, file, {
            purpose: "conversation_attachment",
            signal: controller.signal,
          }),
        });
        if (controller.signal.aborted || !mountedRef.current) {
          return;
        }
        const reusedCount = results.filter((result) => result.status === "fulfilled" && result.value.reused).length;

        const uploaded = results.flatMap((result) => {
          if (result.status !== "fulfilled") {
            return [];
          }
          const sourceFile = result.item;
          const previewURL = sourceFile.type.startsWith("image/") ? URL.createObjectURL(sourceFile) : undefined;
          return [
            {
              fileID: result.value.file.fileID,
              fileName: result.value.file.fileName,
              mimeType: result.value.file.mimeType,
              detectedMime: result.value.file.detectedMIME,
              fileCategory: result.value.file.fileCategory,
              sizeBytes: result.value.file.sizeBytes,
              previewURL,
              processingStatus: result.value.file.processingStatus,
              processingReady: result.value.file.processingReady,
              processingErrorCode: result.value.file.processingErrorCode,
              processingErrorMessage: result.value.file.processingErrorMessage,
              extractStatus: result.value.file.extractStatus,
              embedStatus: result.value.file.embedStatus,
              ragReady: false,
              ragReason: "",
              ocrUsed: false,
              ragOptOut: result.value.file.ragOptOut,
            },
          ];
        });
        if (uploaded.length > 0) {
          const existingIDs = new Set(
            currentConversationKeyRef.current === targetConversationKey
              ? attachmentsRef.current.map((item) => item.fileID)
              : [],
          );
          const nextUploaded = uploaded.filter((item) => {
            if (existingIDs.has(item.fileID)) {
              revokeAttachmentPreview(item);
              return false;
            }
            existingIDs.add(item.fileID);
            return true;
          });
          appendAttachmentsForKey(targetConversationKey, nextUploaded);
          if (currentConversationKeyRef.current !== targetConversationKey) {
            releaseAttachments(uploaded);
          }
        }
        if (reusedCount > 0) {
          toast.success(t("duplicateReused"));
        }
        if (uploaded.length < policyAcceptedFiles.length) {
          toast.error(t("partialUploadFailed"), { description: t("retryFailedFiles") });
        }
      } catch (error) {
        if (!controller.signal.aborted && mountedRef.current) {
          const description = resolveErrorMessage(error, t("retryLater"));
          toast.error(t("uploadFailed"), { description });
        }
      } finally {
        uploadControllersRef.current.delete(controller);
        if (mountedRef.current) {
          setUploadingByKey((prev) => {
            const tempIDs = new Set(placeholders.map((item) => item.tempID));
            const nextItems = (prev[targetConversationKey] ?? []).filter((item) => !tempIDs.has(item.tempID));
            if (nextItems.length === 0) {
              const { [targetConversationKey]: _removed, ...rest } = prev;
              return rest;
            }
            return {
              ...prev,
              [targetConversationKey]: nextItems,
            };
          });
        }
      }
    },
    [
      appendAttachmentsForKey,
      attachments.length,
      chatFilePolicy,
      conversationKey,
      maxFilesPerMessage,
      releaseAttachments,
      resolveErrorMessage,
      t,
      temporary,
      uploading,
      uploadingByKey,
    ],
  );

  const onCaptureScreenshot = React.useCallback(async () => {
    if (typeof navigator === "undefined" || !navigator.mediaDevices?.getDisplayMedia) {
      toast.error(t("screenshotUnsupported"));
      return;
    }

    let stream: MediaStream | null = null;
    try {
      stream = await navigator.mediaDevices.getDisplayMedia({
        video: true,
        audio: false,
      });
      const screenshot = await captureScreenshotFile(stream);
      await onUploadFiles([screenshot]);
    } catch {
      toast.error(t("screenshotFailed"), { description: t("retry") });
    } finally {
      stream?.getTracks().forEach((track) => {
        track.stop();
      });
    }
  }, [onUploadFiles, t]);

  React.useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      for (const controller of uploadControllersRef.current) {
        controller.abort();
      }
      uploadControllersRef.current.clear();
      const previewURLs = new Set(transferredPreviewURLsRef.current);
      for (const item of attachmentsRef.current) {
        if (item.previewURL) {
          previewURLs.add(item.previewURL);
        }
      }
      for (const previewURL of previewURLs) {
        URL.revokeObjectURL(previewURL);
      }
      transferredPreviewURLsRef.current.clear();
    };
  }, []);

  return {
    attachments,
    uploading,
    uploadingAttachments,
    maxFilesPerMessage,
    fileMode: temporary ? "full_context" : (chatFilePolicy?.fileMode ?? "auto"),
    ragAvailable: chatFilePolicy?.ragAvailable ?? null,
    ragAvailabilityReason: chatFilePolicy?.ragAvailabilityReason ?? "",
    releaseAttachments,
    transferAttachments,
    onRemoveAttachment,
    onUploadFiles,
    onCaptureScreenshot,
  };
}
