"use client";

import * as React from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { ChatArea, ChatAreaLoadError, ChatAreaSkeleton } from "@/features/chat/components/sections/chat-area";
import { ChatEmptyState } from "@/features/chat/components/sections/chat-empty";
import { useChatSession } from "@/features/chat/context/chat-session-context";
import { useChatAttachments } from "@/features/chat/hooks/use-chat-attachments";
import { useConversationComposerState } from "@/features/chat/hooks/use-conversation-composer-state";
import type { ChatAreaMessage, MessageAttachment } from "@/features/chat/types/messages";
import { useChatModelOptions } from "@/features/chat/hooks/use-chat-model-options";
import { useChatRuntime } from "@/features/chat/hooks/use-chat-runtime";
import { useChatScrollController } from "@/features/chat/hooks/use-chat-scroll-controller";
import { useChatViewerProfile } from "@/features/chat/hooks/use-chat-viewer-profile";
import { ChatInput } from "@/features/chat/components/sections/chat-input";
import {
  ConversationShareDialog,
  sharePatchFromDTO,
} from "@/features/chat/components/sections/conversation-share-dialog";
import {
  cloneConversationOptions,
  isConversationOptionsObject,
  sanitizeConversationOptions,
} from "@/features/chat/model/conversation-options";
import { useSidebarRecents } from "@/features/recent/context/sidebar-recents-context";
import { useChatData } from "@/features/chat/hooks/use-chat-data";
import { toPendingAttachment } from "@/features/chat/model/message-submit";
import { listAvailableMCPTools } from "@/shared/api/mcp";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { ConversationOptions } from "@/shared/api/conversation.types";
import type { MCPToolDTO } from "@/shared/api/mcp.types";

const MODEL_OPTIONS_STORAGE_PREFIX = "deeix-chat:chat-model-options:";
const EMPTY_CONVERSATION_OPTIONS: ConversationOptions = {};

function modelOptionsStorageKey(platformModelName: string): string {
  return `${MODEL_OPTIONS_STORAGE_PREFIX}${encodeURIComponent(platformModelName)}`;
}

function readCachedModelOptions(platformModelName: string): ConversationOptions | null {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    const raw = window.localStorage.getItem(modelOptionsStorageKey(platformModelName));
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as unknown;
    return isConversationOptionsObject(parsed) ? sanitizeConversationOptions(parsed) : null;
  } catch {
    return null;
  }
}

function writeCachedModelOptions(platformModelName: string, options: ConversationOptions): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(modelOptionsStorageKey(platformModelName), JSON.stringify(sanitizeConversationOptions(options)));
  } catch {
    // localStorage may be unavailable in private browsing or strict environments.
  }
}

function removeCachedModelOptions(platformModelName: string): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.removeItem(modelOptionsStorageKey(platformModelName));
  } catch {
    // localStorage may be unavailable in private browsing or strict environments.
  }
}

export function AppChatArea() {
  const t = useTranslations("chat");
  const router = useRouter();
  const searchParams = useSearchParams();
  const routeConversationID = searchParams.get("conversation_id")?.trim() || null;
  const { newConversationRevision, requestNewConversation } = useChatSession();
  const [locallyCreatedConversationID, setLocallyCreatedConversationID] = React.useState<string | null>(null);
  const [newConversationOverride, setNewConversationOverride] = React.useState<{
    ignoredConversationID: string | null;
  } | null>(null);
  const previousNewConversationRevisionRef = React.useRef(newConversationRevision);

  React.useEffect(() => {
    if (previousNewConversationRevisionRef.current === newConversationRevision) {
      return;
    }
    previousNewConversationRevisionRef.current = newConversationRevision;
    setLocallyCreatedConversationID(null);
    setNewConversationOverride({
      ignoredConversationID: routeConversationID,
    });
  }, [newConversationRevision, routeConversationID]);

  React.useEffect(() => {
    if (routeConversationID) {
      setLocallyCreatedConversationID(null);
    }
  }, [routeConversationID]);

  React.useEffect(() => {
    setNewConversationOverride((prev) =>
      prev && routeConversationID !== prev.ignoredConversationID ? null : prev,
    );
  }, [routeConversationID]);

  const resolvedRouteConversationID = routeConversationID ?? locallyCreatedConversationID;
  const conversationID =
    newConversationOverride && resolvedRouteConversationID === newConversationOverride.ignoredConversationID
      ? null
      : resolvedRouteConversationID;
  const onNewConversationFromLoadError = React.useCallback(() => {
    requestNewConversation();
    router.push("/chat");
  }, [requestNewConversation, router]);
  const activeGenerationRunsRef = React.useRef<Set<string>>(new Set());
  const {
    items,
    projects,
    prependNewConversation,
    touchByPublicID,
    renameByPublicID,
    setStarByPublicID,
    setProjectByPublicID,
    deleteByPublicID,
  } = useSidebarRecents();
  const {
    cancelResumedGeneration,
    loading,
    errorMsg,
    messages,
    reload,
    resumingRunID,
  } = useChatData(conversationID, {
    activeGenerationRunsRef,
  });
  const { greetingTitle } = useChatViewerProfile();
  const [manualConversationTitle, setManualConversationTitle] = React.useState("");
  const [shareDialogOpen, setShareDialogOpen] = React.useState(false);
  const activeConversation = React.useMemo(() => {
    if (!conversationID) {
      return null;
    }
    return items.find((item) => item.publicID === conversationID) ?? null;
  }, [conversationID, items]);

  const {
    modelOptions,
    modelsLoading,
    modelsErrorMsg,
    sendShortcut,
    restoreDraftOnFailure,
    preserveConversationDrafts,
    inputHeight,
    markdownRender,
    showModelInfo,
    showLatency,
    showTokenUsage,
    showBillingCost,
    modelOptionPolicy,
    selectedPlatformModelName,
    setSelectedPlatformModelName,
  } = useChatModelOptions({
    conversationPublicID: conversationID,
    conversationModel: activeConversation?.model ?? null,
  });
  const {
    conversationKey,
    draft,
    attachments,
    setDraft,
    setAttachments,
    appendAttachmentsForKey,
  } = useConversationComposerState(conversationID, {
    preserveDrafts: preserveConversationDrafts,
    resetToken: newConversationRevision,
  });
  const selectedModel = React.useMemo(
    () => modelOptions.find((item) => item.platformModelName === selectedPlatformModelName) ?? null,
    [modelOptions, selectedPlatformModelName],
  );
  const modelOptionPolicyDisabled = modelOptionPolicy?.mode?.trim() === "disabled";
  const [options, setOptions] = React.useState<ConversationOptions>({});
  const [availableTools, setAvailableTools] = React.useState<MCPToolDTO[]>([]);
  const [toolsLoading, setToolsLoading] = React.useState(true);
  const [selectedToolIDs, setSelectedToolIDs] = React.useState<number[]>([]);
  const initializedOptionsModelRef = React.useRef("");

  React.useEffect(() => {
    const platformModelName = selectedModel?.platformModelName.trim() || "";
    if (!platformModelName) {
      initializedOptionsModelRef.current = "";
      setOptions({});
      return;
    }
    if (initializedOptionsModelRef.current === platformModelName) {
      return;
    }
    initializedOptionsModelRef.current = platformModelName;
    const cachedOptions = readCachedModelOptions(platformModelName);
    setOptions(cloneConversationOptions(cachedOptions ?? selectedModel.defaultOptions));
  }, [selectedModel]);

  const setModelOptions = React.useCallback(
    (action: React.SetStateAction<ConversationOptions>) => {
      setOptions((previous) => {
        const next = typeof action === "function" ? action(previous) : action;
        const normalized = isConversationOptionsObject(next) ? sanitizeConversationOptions(next) : {};
        const platformModelName = selectedModel?.platformModelName.trim() || "";
        if (platformModelName) {
          writeCachedModelOptions(platformModelName, normalized);
        }
        return normalized;
      });
    },
    [selectedModel?.platformModelName],
  );

  const resetModelOptions = React.useCallback(() => {
    const platformModelName = selectedModel?.platformModelName.trim() || "";
    const defaults = cloneConversationOptions(selectedModel?.defaultOptions ?? {});
    if (platformModelName) {
      removeCachedModelOptions(platformModelName);
    }
    setOptions(defaults);
  }, [selectedModel]);

  React.useEffect(() => {
    let cancelled = false;

    async function loadTools() {
      setToolsLoading(true);
      try {
        const token = await resolveAccessToken();
        if (!token) {
          if (!cancelled) {
            setAvailableTools([]);
            setSelectedToolIDs([]);
          }
          return;
        }
        const tools = await listAvailableMCPTools(token);
        if (cancelled) {
          return;
        }
        setAvailableTools(tools);
        const availableIDs = new Set(tools.map((item) => item.id));
        setSelectedToolIDs((previous) => previous.filter((id) => availableIDs.has(id)));
      } catch {
        if (!cancelled) {
          setAvailableTools([]);
          setSelectedToolIDs([]);
        }
      } finally {
        if (!cancelled) {
          setToolsLoading(false);
        }
      }
    }

    void loadTools();
    return () => {
      cancelled = true;
    };
  }, []);

  const {
    uploading,
    uploadingAttachments,
    maxFilesPerMessage,
    fileMode,
    releaseAttachments,
    onRemoveAttachment,
    onUploadFiles,
    onCaptureScreenshot,
  } = useChatAttachments({
    conversationKey,
    attachments,
    setAttachments,
    appendAttachmentsForKey,
  });

  const {
    onCycleMessageBranch,
    onEditUserMessage,
    onRetryAssistantMessage,
    onRetryUserMessage,
    onSendMessage,
    onStopMessage,
    sending,
    showPendingAssistant,
    streamingText,
    streamingTraceText,
    visibleMessageCount,
    visibleMessages,
    isConversationMode,
  } = useChatRuntime({
    conversationID,
    resetToken: newConversationRevision,
    messages,
    activeConversation,
    selectedPlatformModelName,
    modelOptions,
    selectedToolIDs,
    options: modelOptionPolicyDisabled ? EMPTY_CONVERSATION_OPTIONS : options,
    draft,
    attachments,
    maxFilesPerMessage,
    uploading,
    restoreDraftOnFailure,
    prependNewConversation,
    onConversationCreated: setLocallyCreatedConversationID,
    touchByPublicID,
    reload,
    setDraft,
    setAttachments,
    releaseAttachments,
    activeGenerationRunsRef,
  });
  const generating = sending || Boolean(resumingRunID);
  const showLiveAssistant = showPendingAssistant || Boolean(resumingRunID);
  const onStopActiveMessage = React.useCallback(() => {
    if (sending) {
      onStopMessage();
      return;
    }
    void cancelResumedGeneration();
  }, [cancelResumedGeneration, onStopMessage, sending]);

  const {
    messageViewportRef,
    messageContentRef,
    onScroll,
    onScrollToLatest,
    showScrollToLatestButton,
  } = useChatScrollController({
    conversationID,
    loading,
    isConversationMode,
    visibleMessageCount,
    showPendingAssistant: showLiveAssistant,
    streamingText,
    streamingTraceText,
  });

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

      window.requestAnimationFrame(onScrollToLatest);
    },
    [
      attachments,
      maxFilesPerMessage,
      modelOptions,
      onScrollToLatest,
      selectedModel,
      setAttachments,
      setSelectedPlatformModelName,
      t,
    ],
  );

  React.useEffect(() => {
    setManualConversationTitle("");
  }, [conversationID]);

  React.useEffect(() => {
    const nextTitle = activeConversation?.title?.trim();
    if (nextTitle) {
      setManualConversationTitle(nextTitle);
    }
  }, [activeConversation?.publicID, activeConversation?.title]);

  const actionConversationID = React.useMemo(() => (conversationID || "").trim(), [conversationID]);
  const canOperateConversation = actionConversationID.length > 0;
  const activeConversationTitle = React.useMemo(
    () => manualConversationTitle || activeConversation?.title?.trim() || t("untitledConversation"),
    [activeConversation?.title, manualConversationTitle, t],
  );
  const activeConversationStarred = Boolean(activeConversation?.isStarred);
  const activeConversationShared = activeConversation?.shareStatus === "active" && Boolean(activeConversation.shareID?.trim());
  const shareDefaultMessagePublicIDs = React.useMemo(
    () =>
      visibleMessages
        .filter((item) => !item.isPending && Boolean(item.serverMessageID) && item.publicID.trim())
        .map((item) => item.publicID.trim()),
    [visibleMessages],
  );

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

  const onDeleteActiveConversation = React.useCallback(async () => {
    if (!canOperateConversation) {
      return;
    }
    const ok = await deleteByPublicID(actionConversationID);
    if (ok) {
      router.push("/chat");
    }
  }, [actionConversationID, canOperateConversation, deleteByPublicID, router]);

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

  const messagesWithInlineError = React.useMemo<ChatAreaMessage[]>(() => {
    const errors = [
      modelsErrorMsg.trim()
        ? {
            title: t("modelListLoadFailed"),
            message: modelsErrorMsg.trim(),
          }
        : null,
    ].filter((item): item is NonNullable<typeof item> => item !== null);

    if (errors.length === 0) {
      return visibleMessages;
    }

    return [
      ...visibleMessages,
      {
        key: `chat-inline-error-${conversationID ?? "current"}`,
        publicID: `chat-inline-error-${conversationID ?? "current"}`,
        parentPublicID: visibleMessages.at(-1)?.publicID ?? null,
        sourcePublicID: null,
        role: "system",
        content: "",
        branchReason: "default",
        isPending: false,
        isStreaming: false,
        inlineAlert: {
          title: errors.map((item) => item.title).join(" / "),
          message: errors.map((item) => item.message).join("\n"),
        },
      },
    ];
  }, [conversationID, modelsErrorMsg, t, visibleMessages]);

  const effectiveOptions = modelOptionPolicyDisabled ? EMPTY_CONVERSATION_OPTIONS : options;
  const selectedModelDefaultOptions = modelOptionPolicyDisabled
    ? EMPTY_CONVERSATION_OPTIONS
    : (selectedModel?.defaultOptions ?? EMPTY_CONVERSATION_OPTIONS);

  const chatInputProps = {
    draft,
    loading,
    sending: generating,
    uploading,
    isConversationMode,
    maxFilesPerMessage,
    fileMode,
    sendShortcut,
    inputHeight,
    attachments,
    uploadingAttachments,
    modelOptions,
    selectedPlatformModelName,
    availableTools,
    selectedToolIDs,
    toolsLoading,
    options: effectiveOptions,
    defaultOptions: selectedModelDefaultOptions,
    modelOptionPolicy,
    modelLoading: modelsLoading,
    onDraftChange: setDraft,
    onModelChange: setSelectedPlatformModelName,
    onSelectedToolsChange: setSelectedToolIDs,
    onOptionsChange: setModelOptions,
    onOptionsReset: resetModelOptions,
    onUploadFiles,
    onCaptureScreenshot,
    onRemoveAttachment,
    onSendMessage,
    onStopMessage: onStopActiveMessage,
  };
  const isConversationLoading = Boolean(conversationID) && loading && visibleMessageCount === 0 && messagesWithInlineError.length === 0;
  const isConversationLoadFailed = Boolean(conversationID) && !loading && errorMsg.trim().length > 0 && visibleMessageCount === 0;
  const shouldUseCenteredComposer =
    !isConversationLoading && !isConversationLoadFailed && !isConversationMode && messagesWithInlineError.length === 0;

  return (
    <div className="flex h-full min-h-0 w-full flex-1 flex-col overflow-hidden">
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        {shouldUseCenteredComposer ? (
          <ChatEmptyState greetingTitle={greetingTitle}>
            <ChatInput {...chatInputProps} />
          </ChatEmptyState>
        ) : (
          <>
            {isConversationLoading ? (
              <ChatAreaSkeleton />
            ) : isConversationLoadFailed ? (
              <ChatAreaLoadError onRefresh={reload} onNewConversation={onNewConversationFromLoadError} />
            ) : (
              <ChatArea
                title={activeConversationTitle}
                starred={activeConversationStarred}
                canOperateConversation={canOperateConversation}
                messages={messagesWithInlineError}
                busy={generating}
                messageViewportRef={messageViewportRef}
                messageContentRef={messageContentRef}
                onScroll={onScroll}
                onScrollToLatest={onScrollToLatest}
                showScrollToLatestButton={showScrollToLatestButton}
                onRetryUserMessage={onRetryUserMessage}
                onRetryAssistantMessage={onRetryAssistantMessage}
                onEditUserMessage={onEditUserMessage}
                onEditImageAttachment={onEditGeneratedImageAttachment}
                onCycleMessageBranch={onCycleMessageBranch}
                onToggleStar={onToggleActiveConversationStar}
                onRename={onRenameActiveConversation}
                projectMenu={{
                  label: t("labelMenu.moveToProject"),
                  unassignedLabel: t("labelMenu.unassignedProject"),
                  currentProjectID: activeConversation?.projectID,
                  projects,
                  onSelect: onSetActiveConversationProject,
                }}
                onShare={onShareActiveConversation}
                shareActive={activeConversationShared}
                onDelete={onDeleteActiveConversation}
                markdownRender={markdownRender}
                showModelInfo={showModelInfo}
                showLatency={showLatency}
                showTokenUsage={showTokenUsage}
                showBillingCost={showBillingCost}
              />
            )}
          </>
        )}
      </div>

      {!shouldUseCenteredComposer && !isConversationLoadFailed ? (
        <div className="relative z-10 shrink-0 px-3 pb-3 md:px-6">
          <div className="mx-auto w-full max-w-[800px]">
            <ChatInput {...chatInputProps} />
          </div>
        </div>
      ) : null}

      {canOperateConversation ? (
        <ConversationShareDialog
          open={shareDialogOpen}
          onOpenChange={setShareDialogOpen}
          conversationPublicID={actionConversationID}
          conversationTitle={activeConversationTitle}
          defaultMessagePublicIDs={shareDefaultMessagePublicIDs}
          onShareChange={(share) => {
            touchByPublicID(actionConversationID, sharePatchFromDTO(share));
          }}
        />
      ) : null}
    </div>
  );
}
