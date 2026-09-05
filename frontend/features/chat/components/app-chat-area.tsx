"use client";

import { Glasses } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";
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
import {
  ConversationShareDialog,
  sharePatchFromDTO,
  useSidebarConversationField,
} from "@/entities/conversation";
import { ChatArea, ChatAreaLoadError, ChatAreaSkeleton } from "@/features/chat/components/sections/chat-area";
import { ChatArtifactWorkspace } from "@/features/chat/components/sections/chat-artifact";
import { ChatEmptyState } from "@/features/chat/components/sections/chat-empty";
import { ChatInput } from "@/features/chat/components/sections/chat-input";
import { ChatScreenshotPreviewDialog } from "@/features/chat/components/sections/chat-screenshot-preview-dialog";
import { TemporaryChatModeControl } from "@/features/chat/components/temporary-chat-mode-control";
import { useChatSession } from "@/features/chat/context/chat-session-context";
import { useChatArtifactResize } from "@/features/chat/hooks/use-chat-artifact-resize";
import { useChatArtifacts } from "@/features/chat/hooks/use-chat-artifacts";
import { useChatAttachments } from "@/features/chat/hooks/use-chat-attachments";
import { useChatComposerSelection } from "@/features/chat/hooks/use-chat-composer-selection";
import { useChatConversationActions } from "@/features/chat/hooks/use-chat-conversation-actions";
import { useChatFileDrag } from "@/features/chat/hooks/use-chat-file-drag";
import { useChatMCPTools } from "@/features/chat/hooks/use-chat-mcp-tools";
import { useChatMediaAttachmentActions } from "@/features/chat/hooks/use-chat-media-attachment-actions";
import { useChatModelOptionState } from "@/features/chat/hooks/use-chat-model-option-state";
import { useChatScreenshotPreview } from "@/features/chat/hooks/use-chat-screenshot-preview";
import {
  resolveConversationComposerKey,
  useChatComposerState,
} from "@/features/chat/hooks/use-chat-composer-state";
import { useChatData } from "@/features/chat/hooks/use-chat-data";
import { useChatModelOptions } from "@/features/chat/hooks/use-chat-model-options";
import { useChatRuntime } from "@/features/chat/hooks/use-chat-runtime";
import { useChatScreenshot } from "@/features/chat/hooks/use-chat-screenshot";
import { useChatViewerProfile } from "@/features/chat/hooks/use-chat-viewer-profile";
import { useChatVisualPrompt } from "@/features/chat/hooks/use-chat-visual-prompt";
import { useChatConversationDefaults } from "@/features/chat/hooks/use-chat-conversation-defaults";
import { useChatTemporaryRuntime } from "@/features/chat/hooks/use-chat-temporary-runtime";
import { filterAvailableMCPToolIDs } from "@/features/chat/model/chat-mcp-tool-defaults";
import type { ChatAreaMessage, } from "@/features/chat/types/messages";
import { useSettingsChatPreferences } from "@/features/settings";
import { cn } from "@/lib/utils";
import { getConversation } from "@/shared/api/conversation";
import type { ConversationDTO, ConversationOptions } from "@/shared/api/conversation.types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { DeleteFilesOption } from "@/shared/components/delete-files-option";
import {
  hasMultipleImageAttachmentProcessors,
  normalizeImageAttachmentProcessorSelection,
} from "@/shared/lib/mcp-tool-selection";
import { resolveChatContentWidthClassName } from "@/shared/model/chat-content-width";

const EMPTY_CONVERSATION_OPTIONS: ConversationOptions = {};
const EMPTY_LIST: never[] = [];
const TOP_LOAD_OLDER_MESSAGES_THRESHOLD_PX = 48;

export function AppChatArea() {
  const t = useTranslations("chat");
  const tRecent = useTranslations("recent");
  const tScreenshot = useTranslations("chat.screenshot");
  const router = useRouter();
  const searchParams = useSearchParams();
  const { user } = useAuthSession();
  const temporaryMode = searchParams.get("temporary") === "true";
  const routeConversationID = temporaryMode ? null : searchParams.get("conversation_id")?.trim() || null;
  const routeProjectID = temporaryMode ? null : searchParams.get("project_id")?.trim() || null;
  const {
    detachConversationRun,
    finishConversationRun,
    newConversationRevision,
    newConversationProjectID: requestedNewConversationProjectID,
    registerConversationRun,
    requestNewConversation,
  } = useChatSession();
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

  const resolvedRouteConversationID = temporaryMode
    ? null
    : routeConversationID ?? locallyCreatedConversationID;
  const conversationID =
    newConversationOverride && resolvedRouteConversationID === newConversationOverride.ignoredConversationID
      ? null
      : resolvedRouteConversationID;
  const onNewConversationFromLoadError = React.useCallback(() => {
    const projectID = routeProjectID ?? "";
    requestNewConversation({ projectID });
    router.push(projectID ? `/chat?project_id=${encodeURIComponent(projectID)}` : "/chat");
  }, [requestNewConversation, routeProjectID, router]);
  const activeGenerationRunsRef = React.useRef<Set<string>>(new Set());
  // Set 的原地增删不会触发 effect，revision 用于同步断流恢复判断。
  const [activeGenerationRunsRevision, setActiveGenerationRunsRevision] = React.useState(0);
  const onActiveGenerationRunsChange = React.useCallback(() => {
    setActiveGenerationRunsRevision((current) => current + 1);
  }, []);
  const {
    autoExpandThinking,
    autoExpandToolCalls,
    autoGenerateLabels,
    deleteFilesByDefault,
    loaded: chatPreferencesLoaded,
    reuseModelOptions,
  } = useSettingsChatPreferences();
  const items = useSidebarConversationField("items");
  const projects = useSidebarConversationField("projects");
  const projectsLoading = useSidebarConversationField("projectsLoading");
  const prependNewConversation = useSidebarConversationField("prependNewConversation");
  const touchByPublicID = useSidebarConversationField("touchByPublicID");
  const renameByPublicID = useSidebarConversationField("renameByPublicID");
  const upsertConversation = useSidebarConversationField("upsertConversation");
  const {
    cancelResumedGeneration,
    conversationPublicID: messageDataConversationID,
    loading,
    loadingOlder,
    errorMsg,
    hasOlder,
    loadOlderMessages,
    messages,
    reload,
    replaceMessage,
    resumingActivityLabel,
    resumingRunID,
  } = useChatData(conversationID, {
    activeGenerationRunsRef,
    activeGenerationRunsRevision,
    onConversationRunFinished: finishConversationRun,
  });
  const { greetingTitle } = useChatViewerProfile();
  const activeConversation = React.useMemo(() => {
    if (!conversationID) {
      return null;
    }
    return items.find((item) => item.publicID === conversationID) ?? null;
  }, [conversationID, items]);
  const [loadedConversation, setLoadedConversation] = React.useState<ConversationDTO | null>(null);
  React.useEffect(() => {
    const normalizedConversationID = conversationID?.trim() || "";
    if (!normalizedConversationID || activeConversation?.publicID === normalizedConversationID) {
      setLoadedConversation(null);
      return;
    }

    let cancelled = false;
    async function loadConversation() {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }
      const item = await getConversation(token, normalizedConversationID);
      if (cancelled) {
        return;
      }
      setLoadedConversation(item);
    }

    void loadConversation().catch(() => {
      if (!cancelled) {
        setLoadedConversation(null);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [activeConversation?.publicID, conversationID]);
  const currentConversation =
    activeConversation ?? (loadedConversation?.publicID === conversationID ? loadedConversation : null);
  const activeRouteProject = React.useMemo(() => {
    if (!routeProjectID || conversationID) {
      return null;
    }
    return projects.find((item) => item.publicID === routeProjectID) ?? null;
  }, [conversationID, projects, routeProjectID]);
  const newConversationProjectID = !conversationID ? routeProjectID ?? requestedNewConversationProjectID : "";
  const newConversationProject = React.useMemo(
    () => projects.find((item) => item.publicID === newConversationProjectID) ?? null,
    [newConversationProjectID, projects],
  );
  const newConversationDefaultsPending = Boolean(newConversationProjectID && projectsLoading);
  const prependNewConversationInContext = React.useCallback(
    (platformModelName?: string) => prependNewConversation(platformModelName, newConversationProjectID || undefined),
    [newConversationProjectID, prependNewConversation],
  );

  const handleConversationForked = React.useCallback(
    async (forked: ConversationDTO) => {
      const baseTitle = forked.title?.trim() || "";
      let listed = false;
      if (baseTitle) {
        try {
          const suffix = t("messages.forkTitle", { title: "" });
          const title = `${Array.from(baseTitle)
            .slice(0, Math.max(0, 255 - Array.from(suffix).length))
            .join("")}${suffix}`;
          listed = Boolean(await renameByPublicID(forked.publicID, title));
        } catch {
          listed = false;
        }
      }
      if (!listed) {
        upsertConversation(forked);
      }
      router.push(`/chat?conversation_id=${forked.publicID}`);
    },
    [renameByPublicID, router, t, upsertConversation],
  );

  const {
    modelOptions,
    refreshModelCatalog,
    refreshModelOption,
    modelsLoading,
    modelsErrorMsg,
    sendShortcut,
    restoreDraftOnFailure,
    preserveConversationDrafts,
    inputHeight,
    contentWidth,
    markdownRender,
    showModelInfo,
    showLatency,
    showTokenUsage,
    showBillingCost,
    billingDisplayCurrency,
    billingDisplayUsdToCnyRate,
    modelOptionPolicy,
    mcpMaxSelectedTools,
    selectedPlatformModelName,
    setSelectedPlatformModelName,
  } = useChatModelOptions({
    conversationPublicID: conversationID,
    conversationModel: currentConversation?.model ?? null,
    newConversationDefaultModel: newConversationProject?.defaultModel ?? "",
    newConversationDefaultsPending,
    resetToken: newConversationRevision,
  });
  const {
    conversationKey,
    draft,
    attachments,
    setDraft,
    setAttachments,
    appendAttachmentsForKey,
  } = useChatComposerState(conversationID, {
    preserveDrafts: preserveConversationDrafts,
    storageScope: user?.publicID ?? "",
    transient: temporaryMode,
  });
  const selectionConversationKey = resolveConversationComposerKey(conversationID);
  const selectedModel = React.useMemo(
    () => modelOptions.find((item) => item.platformModelName === selectedPlatformModelName) ?? null,
    [modelOptions, selectedPlatformModelName],
  );
  const modelOptionPolicyDisabled = modelOptionPolicy?.mode?.trim() === "disabled";
  const refreshModelCatalogForComposer = React.useCallback(async () => {
    await refreshModelCatalog();
  }, [refreshModelCatalog]);
  const {
    options,
    setModelOptions,
    resetModelOptions,
    restoreBackendDefaultModelOptions,
  } = useChatModelOptionState({
    selectedModel,
    selectedPlatformModelName,
    chatPreferencesLoaded,
    reuseModelOptions,
    refreshModelOption,
  });
  const {
    selectedToolIDs,
    selectedSkills,
    selectedKnowledgeBaseIDs,
    setSelectedToolIDs,
    setSelectedSkills,
    setSelectedKnowledgeBaseIDs,
  } = useChatComposerSelection({
    conversationKey: selectionConversationKey,
    createdConversationID: locallyCreatedConversationID,
    resetToken: newConversationRevision,
    hasConversation: Boolean(conversationID),
    storageScope: user?.publicID ?? "",
  });
  const {
    availableTools,
    toolsLoading,
    defaultToolIDs,
    defaultToolsReady,
    onDefaultToolIDsChange,
  } = useChatMCPTools({
    mcpMaxSelectedTools,
    selectedToolIDs,
    setSelectedToolIDs,
  });
  const newConversationSelectionKey = `${newConversationRevision}:${newConversationProjectID || "unassigned"}`;
  const warnedUnavailableProjectModelRef = React.useRef("");
  React.useEffect(() => {
    const configuredModel = newConversationProject?.defaultModel.trim() ?? "";
    if (
      conversationID ||
      !configuredModel ||
      modelsLoading ||
      modelOptions.length === 0 ||
      modelsErrorMsg.trim() ||
      modelOptions.some((model) => model.platformModelName === configuredModel)
    ) {
      return;
    }

    const warningKey = `${newConversationSelectionKey}:${configuredModel}`;
    if (warnedUnavailableProjectModelRef.current === warningKey) {
      return;
    }
    warnedUnavailableProjectModelRef.current = warningKey;
    toast.warning(t("projectDefaultModelUnavailable", { model: configuredModel }));
  }, [
    conversationID,
    modelOptions,
    modelsErrorMsg,
    modelsLoading,
    newConversationProject?.defaultModel,
    newConversationSelectionKey,
    t,
  ]);
  const newConversationDefaultMCPToolIDs = React.useMemo(
    () => normalizeImageAttachmentProcessorSelection(
      filterAvailableMCPToolIDs(
        newConversationProject?.mcpDefaultMode === "custom"
          ? newConversationProject.defaultMCPToolIDs
          : defaultToolIDs,
        availableTools,
        mcpMaxSelectedTools,
      ),
      availableTools,
    ),
    [availableTools, defaultToolIDs, mcpMaxSelectedTools, newConversationProject],
  );
  const newConversationDefaultSkillIDs = React.useMemo(
    () => (newConversationProject?.defaultSkillIDs ?? []).slice(0, mcpMaxSelectedTools),
    [mcpMaxSelectedTools, newConversationProject],
  );
  const newConversationDefaultKnowledgeBaseIDs = React.useMemo(
    () => (newConversationProject?.defaultKnowledgeBaseIDs ?? []).slice(0, 8),
    [newConversationProject],
  );
  const { onSelectedKnowledgeBasesChange, onSelectedSkillsChange, onSelectedToolsChange: applySelectedToolsChange } = useChatConversationDefaults({
    conversationID,
    contextKey: newConversationSelectionKey,
    defaultsPending: newConversationDefaultsPending,
    defaultMCPToolIDs: newConversationDefaultMCPToolIDs,
    defaultSkillIDs: newConversationDefaultSkillIDs,
    defaultKnowledgeBaseIDs: newConversationDefaultKnowledgeBaseIDs,
    mcpDefaultsPending: toolsLoading || !defaultToolsReady,
    setSelectedToolIDs,
    setSelectedSkills,
    setSelectedKnowledgeBaseIDs,
  });
  const onSelectedToolsChange = React.useCallback((nextToolIDs: number[]) => {
    if (hasMultipleImageAttachmentProcessors(nextToolIDs, availableTools)) {
      toast.error(t("composer.mcpImageProcessorLimitTitle"), {
        description: t("composer.mcpImageProcessorLimitDescription"),
      });
      return;
    }
    applySelectedToolsChange(nextToolIDs);
  }, [applySelectedToolsChange, availableTools, t]);
  const htmlVisualPrompt = useChatVisualPrompt();

  const {
    uploading,
    uploadingAttachments,
    maxFilesPerMessage,
    fileMode,
    ragAvailable,
    ragAvailabilityReason,
    releaseAttachments,
    transferAttachments,
    onRemoveAttachment,
    onUploadFiles,
    onCaptureScreenshot,
  } = useChatAttachments({
    conversationKey,
    attachments,
    setAttachments,
    appendAttachmentsForKey,
    temporary: temporaryMode,
  });

  const onTemporaryAttachmentsConsumed = React.useCallback((items: typeof attachments) => {
    transferAttachments(items);
    const consumedIDs = new Set(items.map((item) => item.fileID));
    setAttachments((current) => current.filter((item) => !consumedIDs.has(item.fileID)));
  }, [setAttachments, transferAttachments]);

  const {
    currentLeafMessage,
    onCycleMessageBranch,
    onEditAssistantMessage,
    onEditUserMessage,
    onContinueAssistantMessage,
    onForkMessage,
    onRetryAssistantMessage,
    onRetryUserMessage,
    onSendMessage,
    onStopMessage,
    onDeleteQueuedMessage,
    onEditQueuedMessage,
    onGuideQueuedMessage,
    queuedMessages,
    sending,
    visibleMessageCount,
    visibleMessages,
    isConversationMode,
  } = useChatRuntime({
    conversationID,
    resetToken: newConversationRevision,
    messages,
    activeConversation: currentConversation,
    selectedPlatformModelName,
    modelOptions,
    selectedToolIDs,
    selectedSkills,
    selectedKnowledgeBaseIDs,
    htmlVisualPromptEnabled: htmlVisualPrompt.enabled,
    options: modelOptionPolicyDisabled ? EMPTY_CONVERSATION_OPTIONS : options,
    draft,
    attachments,
    maxFilesPerMessage,
    uploading,
    restoreDraftOnFailure,
    autoGenerateLabels,
    prependNewConversation: prependNewConversationInContext,
    onConversationCreated: setLocallyCreatedConversationID,
    onConversationForked: handleConversationForked,
    touchByPublicID,
    reload,
    replaceMessage,
    setDraft,
    setAttachments,
    releaseAttachments,
    transferAttachments,
    activeGenerationRunsRef,
    activeGenerationRunsRevision,
    onActiveGenerationRunsChange,
    onConversationRunDetached: detachConversationRun,
    onConversationRunFinished: finishConversationRun,
    onConversationRunStarted: registerConversationRun,
    resumingActivityLabel,
    resumingRunID,
  });
  const generating = sending;
  const uploadDropDisabled = loading || uploading;
  const onStopActiveMessage = React.useCallback(() => {
    const visibleRunID = currentLeafMessage?.runID?.trim() || "";
    if (resumingRunID && visibleRunID === resumingRunID) {
      void cancelResumedGeneration();
      return;
    }
    if (onStopMessage()) {
      return;
    }
  }, [
    cancelResumedGeneration,
    currentLeafMessage?.runID,
    onStopMessage,
    resumingRunID,
  ]);

  const messageContentRef = React.useRef<HTMLDivElement | null>(null);
  const loadingOlderInFlightRef = React.useRef(false);
  const onScroll = React.useCallback(
    (event: React.UIEvent<HTMLDivElement>) => {
      const viewport = event.currentTarget;
      const distanceFromBottom = viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight;
      if (
        viewport.scrollTop > TOP_LOAD_OLDER_MESSAGES_THRESHOLD_PX ||
        distanceFromBottom <= TOP_LOAD_OLDER_MESSAGES_THRESHOLD_PX ||
        !hasOlder ||
        loadingOlder ||
        loadingOlderInFlightRef.current
      ) {
        return;
      }

      loadingOlderInFlightRef.current = true;
      Promise.resolve(loadOlderMessages())
        .catch((): undefined => undefined)
        .finally(() => {
          loadingOlderInFlightRef.current = false;
        });
    },
    [hasOlder, loadOlderMessages, loadingOlder],
  );

  const {
    onEditGeneratedImageAttachment,
    onExtendGeneratedVideoAttachment,
    onAttachExistingFile,
  } = useChatMediaAttachmentActions({
    attachments,
    maxFilesPerMessage,
    modelOptions,
    selectedModel,
    selectedPlatformModelName,
    setAttachments,
    setSelectedPlatformModelName,
    releaseAttachments,
  });

  const {
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
  } = useChatConversationActions({
    conversationID,
    currentConversation,
    deleteFilesByDefault,
  });
  const shareDefaultMessagePublicIDs = React.useMemo(
    () =>
      visibleMessages
        .filter((item) => !item.isPending && Boolean(item.serverMessageID) && item.publicID.trim())
        .map((item) => item.publicID.trim()),
    [visibleMessages],
  );

  const screenshotMessages = React.useMemo(
    () => ({
      emptySelection: tScreenshot("emptySelection"),
      selectionLimitReached: tScreenshot("selectionLimitReached"),
      generating: tScreenshot("generating"),
      ready: tScreenshot("ready"),
      failed: tScreenshot("failed"),
      tooLarge: tScreenshot("tooLarge"),
      downloaded: tScreenshot("downloaded"),
      copied: tScreenshot("copied"),
      copyFailed: tScreenshot("copyFailed"),
      copyUnsupported: tScreenshot("copyUnsupported"),
    }),
    [tScreenshot],
  );
  const screenshot = useChatScreenshot({
    conversationID: actionConversationID || null,
    messageContentRef,
    conversationTitle: activeConversationTitle,
    messages: screenshotMessages,
  });
  const screenshotPreview = screenshot.preview;
  const { screenshotPreviewOpen, closeScreenshotPreviewDialog } = useChatScreenshotPreview({
    preview: screenshotPreview,
    closePreview: screenshot.closePreview,
  });

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
  const temporaryAvailableTools = React.useMemo(
    () => availableTools.filter((tool) => tool.attachmentInputMode !== "image"),
    [availableTools],
  );
  const temporarySelectedToolIDs = React.useMemo(() => {
    const supportedIDs = new Set(temporaryAvailableTools.map((tool) => tool.id));
    return selectedToolIDs.filter((id) => supportedIDs.has(id));
  }, [selectedToolIDs, temporaryAvailableTools]);
  const temporarySelectedSkillIDs = React.useMemo(
    () => selectedSkills.map((skill) => skill.id),
    [selectedSkills],
  );
  const temporaryRuntime = useChatTemporaryRuntime({
    active: temporaryMode,
    draft,
    model: selectedPlatformModelName,
    options: effectiveOptions,
    selectedToolIDs: temporarySelectedToolIDs,
    selectedSkillIDs: temporarySelectedSkillIDs,
    selectedKnowledgeBaseIDs,
    htmlVisualPromptEnabled: htmlVisualPrompt.enabled,
    attachments,
    onDraftChange: setDraft,
    onAttachmentsConsumed: onTemporaryAttachmentsConsumed,
    releaseAttachments,
  });
  const displayMessages = temporaryMode ? temporaryRuntime.messages : messagesWithInlineError;
  const artifactWorkspace = useChatArtifacts({
    scopeKey: conversationID,
    transient: temporaryMode,
    messages: displayMessages,
  });
  const { workspaceRef, artifactResizing, onArtifactResizeStart } = useChatArtifactResize(artifactWorkspace);
  const hasInlineArtifact = Boolean(artifactWorkspace.activeArtifact && artifactWorkspace.isInlineViewport);
  const workspaceGridColumns = hasInlineArtifact
    ? `minmax(0, ${1 - artifactWorkspace.artifactRatio}fr) minmax(0, ${artifactWorkspace.artifactRatio}fr)`
    : "minmax(0, 1fr) minmax(0, 0fr)";

  const selectedModelDefaultOptions = modelOptionPolicyDisabled
    ? EMPTY_CONVERSATION_OPTIONS
    : (selectedModel?.defaultOptions ?? EMPTY_CONVERSATION_OPTIONS);
  const {
    fileDragActive,
    onFileDragEnter,
    onFileDragOver,
    onFileDragLeave,
    onFileDrop,
  } = useChatFileDrag({
    disabled: uploadDropDisabled,
    onUploadFiles,
  });

  const composerSending = temporaryMode ? temporaryRuntime.sending : generating;
  const composerConversationMode = temporaryMode ? temporaryRuntime.messages.length > 0 : isConversationMode;
  const composerLoading =
    !temporaryMode &&
    Boolean(conversationID) &&
    (loading || messageDataConversationID !== conversationID);
  const chatInputProps = {
    draft,
    loading: composerLoading,
    sending: composerSending,
    uploading: temporaryMode ? false : uploading,
    isConversationMode: composerConversationMode,
    fileMode,
    ragAvailable,
    ragAvailabilityReason,
    sendShortcut,
    inputHeight,
    attachments,
    uploadingAttachments,
    modelOptions,
    billingDisplayCurrency,
    billingDisplayUsdToCnyRate,
    selectedPlatformModelName,
    availableTools: temporaryMode ? temporaryAvailableTools : availableTools,
    selectedToolIDs: temporaryMode ? temporarySelectedToolIDs : selectedToolIDs,
    selectedSkills,
    selectedKnowledgeBaseIDs,
    defaultToolIDs,
    queuedMessages: temporaryMode ? EMPTY_LIST : queuedMessages,
    htmlVisualPromptEnabled: htmlVisualPrompt.enabled,
    maxSelectedTools: mcpMaxSelectedTools,
    toolsLoading,
    options: effectiveOptions,
    defaultOptions: selectedModelDefaultOptions,
    modelOptionPolicy,
    modelLoading: modelsLoading,
    dropActive: fileDragActive,
    temporaryMode,
    autoFocusKey: conversationID ?? `${conversationKey}:${newConversationRevision}`,
    onDraftChange: setDraft,
    onModelChange: setSelectedPlatformModelName,
    onModelCatalogRefresh: refreshModelCatalogForComposer,
    onSelectedToolsChange,
    maxSelectedSkills: mcpMaxSelectedTools,
    onSelectedSkillsChange,
    onSelectedKnowledgeBasesChange,
    onDefaultToolsChange: onDefaultToolIDsChange,
    onHTMLVisualPromptChange: htmlVisualPrompt.setEnabled,
    onOptionsChange: setModelOptions,
    onOptionsReset: resetModelOptions,
    onOptionsDefaultRestore: restoreBackendDefaultModelOptions,
    onAttachExistingFile,
    onUploadFiles,
    onCaptureScreenshot,
    onRemoveAttachment,
    onSendMessage: temporaryMode ? temporaryRuntime.send : onSendMessage,
    onStopMessage: temporaryMode ? temporaryRuntime.stop : onStopActiveMessage,
    onDeleteQueuedMessage,
    onEditQueuedMessage,
    onGuideQueuedMessage,
  };
  const chatContentWidthClassName = resolveChatContentWidthClassName(contentWidth);
  const isConversationLoading = !temporaryMode && Boolean(conversationID) && loading && visibleMessageCount === 0 && displayMessages.length === 0;
  const isConversationLoadFailed = !temporaryMode && Boolean(conversationID) && !loading && errorMsg.trim().length > 0 && visibleMessageCount === 0;
  const shouldUseCenteredComposer =
    !isConversationLoading && !isConversationLoadFailed && !composerConversationMode && displayMessages.length === 0;

  return (
    <div
      className="relative flex h-full min-h-0 w-full flex-1 flex-col overflow-hidden md:overflow-visible"
      onDragEnter={onFileDragEnter}
      onDragOver={onFileDragOver}
      onDragLeave={onFileDragLeave}
      onDrop={onFileDrop}
    >
      {!conversationID ? (
        <TemporaryChatModeControl
          active={temporaryMode}
          requiresExitConfirmation={temporaryRuntime.sending || temporaryRuntime.messages.length > 0}
        />
      ) : null}
      {shouldUseCenteredComposer ? (
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <ChatEmptyState
            greetingTitle={activeRouteProject?.name || greetingTitle}
            badgeLabel={activeRouteProject ? t("projectMode") : undefined}
            badgeTooltip={activeRouteProject ? t("projectModeTooltip") : undefined}
            titleAdornment={temporaryMode ? (
              <Glasses
                aria-hidden
                className="size-5 shrink-0 text-muted-foreground md:size-[22px]"
                strokeWidth={1.6}
              />
            ) : undefined}
            contentWidthClassName={chatContentWidthClassName}
          >
            <ChatInput {...chatInputProps} />
          </ChatEmptyState>
        </div>
      ) : (
        <div
          ref={workspaceRef}
          className={cn(
            "relative grid min-h-0 flex-1 overflow-hidden",
            artifactResizing
              ? "transition-none"
              : "transition-[grid-template-columns] duration-500 ease-[cubic-bezier(0.16,1,0.3,1)]",
            hasInlineArtifact && "md:overflow-visible",
          )}
          style={{ gridTemplateColumns: workspaceGridColumns }}
        >
          <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
            <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
              {isConversationLoading ? (
                <ChatAreaSkeleton contentWidthClassName={chatContentWidthClassName} />
              ) : isConversationLoadFailed ? (
                <ChatAreaLoadError onRefresh={reload} onNewConversation={onNewConversationFromLoadError} />
              ) : (
                <ChatArea
                  title={temporaryMode ? t("temporary.title") : activeConversationTitle}
                  starred={activeConversationStarred}
                  canOperateConversation={temporaryMode ? false : canOperateConversation}
                  messages={displayMessages}
                  attachmentContentLoader={temporaryMode ? temporaryRuntime.loadAttachmentContent : undefined}
                  persistMessageFeedback={!temporaryMode}
                  allowFullToolResults={!temporaryMode}
                  busy={composerSending}
                  messageContentRef={messageContentRef}
                  onScroll={onScroll}
                  onRetryUserMessage={temporaryMode ? temporaryRuntime.onRetryUserMessage : onRetryUserMessage}
                  onRetryAssistantMessage={temporaryMode ? temporaryRuntime.onRetryAssistantMessage : onRetryAssistantMessage}
                  onContinueAssistantMessage={temporaryMode ? undefined : onContinueAssistantMessage}
                  onEditAssistantMessage={temporaryMode ? temporaryRuntime.onEditAssistantMessage : onEditAssistantMessage}
                  onEditUserMessage={temporaryMode ? temporaryRuntime.onEditUserMessage : onEditUserMessage}
                  onForkMessage={temporaryMode ? undefined : onForkMessage}
                  modelOptions={modelOptions}
                  selectedPlatformModelName={selectedPlatformModelName}
                  onModelChange={setSelectedPlatformModelName}
                  onModelCatalogRefresh={refreshModelCatalogForComposer}
                  onEditImageAttachment={onEditGeneratedImageAttachment}
                  onExtendVideoAttachment={onExtendGeneratedVideoAttachment}
                  onOpenCodeArtifact={artifactWorkspace.openArtifact}
                  onCycleMessageBranch={onCycleMessageBranch}
                  onToggleStar={temporaryMode ? undefined : onToggleActiveConversationStar}
                  onRename={temporaryMode ? undefined : onRenameActiveConversation}
                  onAutoRename={temporaryMode ? undefined : onAutoRenameActiveConversation}
                  labels={temporaryMode ? EMPTY_LIST : activeConversationLabels}
                  onUpdateLabels={temporaryMode ? undefined : onUpdateActiveConversationLabels}
                  projectMenu={temporaryMode ? undefined : {
                    label: t("labelMenu.moveToProject"),
                    unassignedLabel: t("labelMenu.unassignedProject"),
                    currentProjectID: currentConversation?.projectID,
                    projects,
                    onSelect: onSetActiveConversationProject,
                  }}
                  onShare={temporaryMode ? undefined : onShareActiveConversation}
                  shareActive={activeConversationShared}
                  onExport={temporaryMode ? undefined : onExportActiveConversation}
                  onDelete={temporaryMode ? undefined : onRequestDeleteActiveConversation}
                  markdownRender={markdownRender}
                  autoExpandThinking={autoExpandThinking}
                  autoExpandToolCalls={autoExpandToolCalls}
                  showModelInfo={showModelInfo}
                  showLatency={showLatency}
                  showTokenUsage={showTokenUsage}
                  showBillingCost={showBillingCost}
                  billingDisplayCurrency={billingDisplayCurrency}
                  billingDisplayUsdToCnyRate={billingDisplayUsdToCnyRate}
                  splitRightInset={hasInlineArtifact}
                  contentWidthClassName={chatContentWidthClassName}
                  onScreenshotLatest={screenshot.captureLatestMessages}
                  onScreenshotSelect={screenshot.startSelectionScreenshot}
                  screenshot={{
                    selectionMode: screenshot.selectionMode,
                    selectedIDs: screenshot.selectedIDs,
                    selectedCount: screenshot.selectedCount,
                    capturing: screenshot.capturing,
                    onToggleSelection: screenshot.toggleSelection,
                    onSelectAll: screenshot.selectMany,
                    onClearSelection: screenshot.clearSelection,
                    onPruneSelection: screenshot.pruneSelection,
                    onCapture: screenshot.captureSelectedMessages,
                    onExit: screenshot.exitSelectionMode,
                  }}
                />
              )}
            </div>

            {!isConversationLoadFailed ? (
              <div className="relative z-10 shrink-0 px-3 pb-3 md:px-6">
                <div className={cn("mx-auto w-full", chatContentWidthClassName)}>
                  <ChatInput {...chatInputProps} />
                </div>
              </div>
            ) : null}
          </div>

          <ChatArtifactWorkspace
            artifact={artifactWorkspace.activeArtifact}
            artifacts={artifactWorkspace.artifacts}
            isInlineViewport={artifactWorkspace.isInlineViewport}
            onArtifactChange={artifactWorkspace.selectArtifact}
            onClose={artifactWorkspace.closeArtifact}
            onResizeReset={artifactWorkspace.resetArtifactRatio}
            onResizeStart={onArtifactResizeStart}
          />
        </div>
      )}

      <ChatScreenshotPreviewDialog
        open={screenshotPreviewOpen}
        onOpenChange={(open) => {
          if (!open) {
            closeScreenshotPreviewDialog();
          }
        }}
        previewURL={screenshotPreview?.url ?? null}
        clipboardSupported={screenshot.clipboardSupported}
        onDownload={screenshot.downloadPreview}
        onCopy={screenshot.copyPreviewToClipboard}
      />

      {canOperateConversation ? (
        <>
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

          <AlertDialog
            open={deleteDialogOpen}
            onOpenChange={(open) => {
              setDeleteDialogOpen(open);
              if (!open) {
                setDeleteFiles(false);
              }
            }}
          >
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{tRecent("dialogs.deleteTitle")}</AlertDialogTitle>
                <AlertDialogDescription>
                  {tRecent("dialogs.deleteDescription", {
                    label: tRecent("deleteConversationLabel", { title: activeConversationTitle }),
                  })}
                </AlertDialogDescription>
                <DeleteFilesOption
                  id={deleteFilesID}
                  checked={deleteFiles}
                  onCheckedChange={setDeleteFiles}
                />
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{tRecent("dialogs.cancel")}</AlertDialogCancel>
                <AlertDialogAction variant="destructive" onClick={() => void onConfirmDeleteActiveConversation()}>
                  {tRecent("dialogs.delete")}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </>
      ) : null}
    </div>
  );
}
