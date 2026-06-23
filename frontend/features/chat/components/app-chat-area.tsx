"use client";

import * as React from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { ChatArea, ChatAreaLoadError, ChatAreaSkeleton } from "@/features/chat/components/sections/chat-area";
import { ChatArtifactWorkspace } from "@/features/chat/components/sections/chat-artifact";
import { ChatEmptyState } from "@/features/chat/components/sections/chat-empty";
import { useChatSession } from "@/features/chat/context/chat-session-context";
import { useChatArtifacts } from "@/features/chat/hooks/use-chat-artifacts";
import { useChatAttachments } from "@/features/chat/hooks/use-chat-attachments";
import { useChatComposerState } from "@/features/chat/hooks/use-chat-composer-state";
import type { ChatAreaMessage, MessageAttachment } from "@/features/chat/types/messages";
import { useChatModelOptions } from "@/features/chat/hooks/use-chat-model-options";
import { useChatRuntime } from "@/features/chat/hooks/use-chat-runtime";
import { useChatScrollController } from "@/features/chat/hooks/use-chat-scroll-controller";
import { useChatViewerProfile } from "@/features/chat/hooks/use-chat-viewer-profile";
import { useChatConversationExport } from "@/features/chat/hooks/use-chat-conversation-export";
import { useChatVisualPrompt } from "@/features/chat/hooks/use-chat-visual-prompt";
import { ChatInput } from "@/features/chat/components/sections/chat-input";
import { resolveChatContentWidthClassName } from "@/shared/model/chat-content-width";
import {
  ConversationShareDialog,
  sharePatchFromDTO,
} from "@/features/chat/components/sections/chat-share-dialog";
import { DeleteFilesOption } from "@/shared/components/delete-files-option";
import { useSettingsChatPreferences } from "@/features/settings/hooks/use-settings-chat-preferences";
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
  cloneConversationOptions,
  isConversationOptionsObject,
  sanitizeConversationOptions,
} from "@/features/chat/model/conversation-options";
import { useSidebarRecents } from "@/features/recent/context/sidebar-recents-context";
import { useChatData } from "@/features/chat/hooks/use-chat-data";
import { toPendingAttachment } from "@/features/chat/model/message-submit";
import { getConversation } from "@/shared/api/conversation";
import { listAvailableMCPTools } from "@/shared/api/mcp";
import { getUserSettings, patchUserSettings } from "@/shared/api/user-settings";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { ConversationDTO, ConversationOptions } from "@/shared/api/conversation.types";
import type { FileObjectDTO } from "@/shared/api/file.types";
import type { MCPToolDTO } from "@/shared/api/mcp.types";
import type { SkillSummaryDTO } from "@/shared/api/skills.types";
import { useTheme } from "@/shared/components/theme-provider";
import { cn } from "@/lib/utils";

const MODEL_OPTIONS_STORAGE_PREFIX = "deeix-chat:chat-model-options:";
const DEFAULT_MCP_TOOLS_SETTING_KEY = "chat.default_mcp_tool_ids";
const EMPTY_CONVERSATION_OPTIONS: ConversationOptions = {};
function dragEventContainsFiles(event: React.DragEvent<HTMLElement>): boolean {
  return Array.from(event.dataTransfer.types ?? []).includes("Files");
}

function droppedFiles(event: React.DragEvent<HTMLElement>): File[] {
  return Array.from(event.dataTransfer.files ?? []).filter((file) => file.name.trim() || file.size > 0);
}

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

function parseDefaultMCPToolIDs(raw: string | null | undefined): number[] {
  const value = raw?.trim();
  if (!value) {
    return [];
  }
  try {
    const parsed = JSON.parse(value) as unknown;
    if (!Array.isArray(parsed)) {
      return [];
    }
    const seen = new Set<number>();
    const result: number[] = [];
    for (const item of parsed) {
      const id = typeof item === "number" ? item : Number(item);
      if (Number.isSafeInteger(id) && id > 0 && !seen.has(id)) {
        seen.add(id);
        result.push(id);
      }
    }
    return result;
  } catch {
    return [];
  }
}

function normalizeAvailableMCPTools(tools: MCPToolDTO[]): MCPToolDTO[] {
  const seen = new Set<number>();
  return tools.filter((tool) => {
    if (!Number.isSafeInteger(tool.id) || tool.id <= 0 || seen.has(tool.id)) {
      return false;
    }
    const status = typeof tool.status === "string" ? tool.status.trim() : "";
    if (status && status !== "active") {
      return false;
    }
    seen.add(tool.id);
    return true;
  });
}

function filterAvailableMCPToolIDs(toolIDs: number[], tools: MCPToolDTO[], limit?: number): number[] {
  const availableIDs = new Set(tools.map((tool) => tool.id));
  const result = toolIDs.filter((id) => availableIDs.has(id));
  return typeof limit === "number" && limit >= 0 ? result.slice(0, limit) : result;
}

export function AppChatArea() {
  const t = useTranslations("chat");
  const tRecent = useTranslations("recent");
  const router = useRouter();
  const searchParams = useSearchParams();
  const routeConversationID = searchParams.get("conversation_id")?.trim() || null;
  const routeProjectID = searchParams.get("project_id")?.trim() || null;
  const { newConversationRevision, newConversationProjectID: requestedNewConversationProjectID, requestNewConversation } = useChatSession();
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
    const projectID = routeProjectID ?? "";
    requestNewConversation({ projectID });
    router.push(projectID ? `/chat?project_id=${encodeURIComponent(projectID)}` : "/chat");
  }, [requestNewConversation, routeProjectID, router]);
  const activeGenerationRunsRef = React.useRef<Set<string>>(new Set());
  const failedGenerationRunsRef = React.useRef<Set<string>>(new Set());
  const {
    deleteFilesByDefault,
    loaded: chatPreferencesLoaded,
    reuseModelOptions,
  } = useSettingsChatPreferences();
  const {
    items,
    projects,
    prependNewConversation,
    touchByPublicID,
    renameByPublicID,
    regenerateTitleByPublicID,
    setStarByPublicID,
    setProjectByPublicID,
    deleteByPublicID,
  } = useSidebarRecents();
  const {
    cancelResumedGeneration,
    loading,
    loadingOlder,
    errorMsg,
    hasOlder,
    loadOlderMessages,
    messages,
    reload,
    replaceMessage,
    resumingRunID,
  } = useChatData(conversationID, {
    activeGenerationRunsRef,
    failedGenerationRunsRef,
  });
  const { greetingTitle } = useChatViewerProfile();
  const [manualConversationTitle, setManualConversationTitle] = React.useState("");
  const [shareDialogOpen, setShareDialogOpen] = React.useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = React.useState(false);
  const [deleteFiles, setDeleteFiles] = React.useState(false);
  const deleteFilesID = React.useId();
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
  const prependNewConversationInContext = React.useCallback(
    (platformModelName?: string) => prependNewConversation(platformModelName, newConversationProjectID || undefined),
    [newConversationProjectID, prependNewConversation],
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
    modelOptionPolicy,
    mcpMaxSelectedTools,
    selectedPlatformModelName,
    setSelectedPlatformModelName,
  } = useChatModelOptions({
    conversationPublicID: conversationID,
    conversationModel: currentConversation?.model ?? null,
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
    resetToken: newConversationRevision,
  });
  const selectedModel = React.useMemo(
    () => modelOptions.find((item) => item.platformModelName === selectedPlatformModelName) ?? null,
    [modelOptions, selectedPlatformModelName],
  );
  const modelOptionPolicyDisabled = modelOptionPolicy?.mode?.trim() === "disabled";
  const refreshModelCatalogForComposer = React.useCallback(async () => {
    await refreshModelCatalog();
  }, [refreshModelCatalog]);
  const [options, setOptions] = React.useState<ConversationOptions>({});
  const [availableTools, setAvailableTools] = React.useState<MCPToolDTO[]>([]);
  const [toolsLoading, setToolsLoading] = React.useState(true);
  const [selectedToolIDs, setSelectedToolIDs] = React.useState<number[]>([]);
  const [selectedSkills, setSelectedSkills] = React.useState<SkillSummaryDTO[]>([]);
  const [defaultToolIDs, setDefaultToolIDs] = React.useState<number[]>([]);
  const defaultToolIDsRef = React.useRef<number[]>([]);
  const htmlVisualPrompt = useChatVisualPrompt();
  const { resolvedTheme } = useTheme();
  const initializedOptionsModelRef = React.useRef("");
  const selectedModelDefaultOptionsRef = React.useRef<ConversationOptions>({});
  const fileDragDepthRef = React.useRef(0);
  const [fileDragActive, setFileDragActive] = React.useState(false);

  React.useEffect(() => {
    setSelectedToolIDs((current) => {
      if (current.length <= mcpMaxSelectedTools) {
        return current;
      }
      return current.slice(0, mcpMaxSelectedTools);
    });
  }, [mcpMaxSelectedTools]);

  React.useEffect(() => {
    const platformModelName = selectedModel?.platformModelName.trim() || "";
    if (!platformModelName) {
      initializedOptionsModelRef.current = "";
      selectedModelDefaultOptionsRef.current = {};
      setOptions({});
      return;
    }
    if (!chatPreferencesLoaded) {
      return;
    }
    const nextDefaultOptions = cloneConversationOptions(selectedModel.defaultOptions);
    const previousDefaultOptions = selectedModelDefaultOptionsRef.current;
    if (initializedOptionsModelRef.current !== platformModelName) {
      initializedOptionsModelRef.current = platformModelName;
      selectedModelDefaultOptionsRef.current = nextDefaultOptions;
      const cachedOptions = reuseModelOptions ? readCachedModelOptions(platformModelName) : null;
      setOptions(cloneConversationOptions(cachedOptions ?? nextDefaultOptions));
      return;
    }
    selectedModelDefaultOptionsRef.current = nextDefaultOptions;
    const previousDefaultOptionsJSON = JSON.stringify(previousDefaultOptions);
    if (previousDefaultOptionsJSON === JSON.stringify(nextDefaultOptions)) {
      return;
    }
    setOptions((currentOptions) => {
      if (JSON.stringify(currentOptions) !== previousDefaultOptionsJSON) {
        return currentOptions;
      }
      removeCachedModelOptions(platformModelName);
      return cloneConversationOptions(nextDefaultOptions);
    });
  }, [chatPreferencesLoaded, reuseModelOptions, selectedModel]);

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

  const resetModelOptions = React.useCallback((defaults?: ConversationOptions) => {
    const platformModelName = selectedModel?.platformModelName.trim() || "";
    const nextDefaults = cloneConversationOptions(defaults ?? selectedModel?.defaultOptions ?? {});
    if (platformModelName) {
      removeCachedModelOptions(platformModelName);
    }
    setOptions(nextDefaults);
  }, [selectedModel]);

  const restoreBackendDefaultModelOptions = React.useCallback(async () => {
    const platformModelName = selectedModel?.platformModelName.trim() || selectedPlatformModelName.trim();
    if (!platformModelName) {
      return null;
    }
    const refreshedModel = await refreshModelOption(platformModelName);
    return refreshedModel ? cloneConversationOptions(refreshedModel.defaultOptions) : null;
  }, [refreshModelOption, selectedModel?.platformModelName, selectedPlatformModelName]);

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
        const [toolsResult, settings] = await Promise.all([
          listAvailableMCPTools(token),
          getUserSettings(token).catch(() => ({} as Record<string, string>)),
        ]);
        if (cancelled) {
          return;
        }
        const tools = normalizeAvailableMCPTools(toolsResult);
        const userDefaultToolIDs = filterAvailableMCPToolIDs(
          parseDefaultMCPToolIDs(settings[DEFAULT_MCP_TOOLS_SETTING_KEY]),
          tools,
        );
        setAvailableTools(tools);
        setDefaultToolIDs(userDefaultToolIDs);
        const availableIDs = new Set(tools.map((item) => item.id));
        setSelectedToolIDs((previous) => {
          const retained = previous.filter((id) => availableIDs.has(id));
          if (retained.length > 0 || conversationID) {
            return retained;
          }
          return userDefaultToolIDs.slice(0, mcpMaxSelectedTools);
        });
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
  }, [conversationID, mcpMaxSelectedTools]);

  React.useEffect(() => {
    defaultToolIDsRef.current = defaultToolIDs;
  }, [defaultToolIDs]);

  React.useEffect(() => {
    if (conversationID) {
      return;
    }
    setSelectedToolIDs(filterAvailableMCPToolIDs(defaultToolIDsRef.current, availableTools, mcpMaxSelectedTools));
  }, [availableTools, conversationID, mcpMaxSelectedTools, newConversationRevision]);

  const onDefaultToolIDsChange = React.useCallback(async (nextToolIDs: number[]) => {
    const nextDefaults = filterAvailableMCPToolIDs(nextToolIDs, availableTools, mcpMaxSelectedTools);
    const previousDefaults = defaultToolIDs;
    setDefaultToolIDs(nextDefaults);
    try {
      const token = await resolveAccessToken();
      if (!token) {
        throw new Error(t("composer.sessionExpired"));
      }
      await patchUserSettings(token, {
        [DEFAULT_MCP_TOOLS_SETTING_KEY]: JSON.stringify(nextDefaults),
      });
      toast.success(t("composer.defaultMCPToolsSaved"));
    } catch (error) {
      setDefaultToolIDs(previousDefaults);
      toast.error(t("composer.defaultMCPToolsSaveFailed"), {
        description: error instanceof Error ? error.message : t("composer.retryLater"),
      });
    }
  }, [availableTools, defaultToolIDs, mcpMaxSelectedTools, t]);

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
    onEditAssistantMessage,
    onEditUserMessage,
    onContinueAssistantMessage,
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
    activeConversation: currentConversation,
    selectedPlatformModelName,
    modelOptions,
    selectedToolIDs,
    selectedSkills,
    htmlVisualPromptEnabled: htmlVisualPrompt.enabled,
    htmlVisualColorMode: resolvedTheme,
    options: modelOptionPolicyDisabled ? EMPTY_CONVERSATION_OPTIONS : options,
    draft,
    attachments,
    maxFilesPerMessage,
    uploading,
    restoreDraftOnFailure,
    prependNewConversation: prependNewConversationInContext,
    onConversationCreated: setLocallyCreatedConversationID,
    touchByPublicID,
    reload,
    replaceMessage,
    setDraft,
    setAttachments,
    setSelectedSkills,
    releaseAttachments,
    activeGenerationRunsRef,
    failedGenerationRunsRef,
    resumingRunID,
  });
  const generating = sending || Boolean(resumingRunID);
  const uploadDropDisabled = generating || loading || uploading;
  const showLiveAssistant = showPendingAssistant || Boolean(resumingRunID);
  const latestMessageKey = visibleMessages.at(-1)?.key ?? "";
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
    messageEndRef,
    onScroll,
    onScrollToLatest,
    showScrollToLatestButton,
  } = useChatScrollController({
    conversationID,
    loading,
    isConversationMode,
    visibleMessageCount,
    latestMessageKey,
    showPendingAssistant: showLiveAssistant,
    streamingText,
    streamingTraceText,
    hasOlderMessages: hasOlder,
    loadingOlderMessages: loadingOlder,
    onLoadOlderMessages: loadOlderMessages,
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
      window.requestAnimationFrame(onScrollToLatest);
    },
    [attachments, maxFilesPerMessage, onScrollToLatest, setAttachments, t],
  );

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
  const activeConversationShared = currentConversation?.shareStatus === "active" && Boolean(currentConversation.shareID?.trim());
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

  const exportActiveConversation = useChatConversationExport({
    successMessage: t("exportJSONSuccess"),
    failureMessage: t("exportJSONFailed"),
  });

  const onExportActiveConversation = React.useCallback(async () => {
    if (!canOperateConversation) {
      return;
    }
    await exportActiveConversation(actionConversationID);
  }, [actionConversationID, canOperateConversation, exportActiveConversation]);

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

  const artifactWorkspace = useChatArtifacts({
    conversationID,
    messages: messagesWithInlineError,
  });
  const workspaceRef = React.useRef<HTMLDivElement | null>(null);
  const artifactResizeCleanupRef = React.useRef<(() => void) | null>(null);
  const [artifactResizing, setArtifactResizing] = React.useState(false);
  const hasInlineArtifact = Boolean(artifactWorkspace.activeArtifact && artifactWorkspace.isInlineViewport);
  const workspaceGridColumns = hasInlineArtifact
    ? `minmax(0, ${1 - artifactWorkspace.artifactRatio}fr) minmax(0, ${artifactWorkspace.artifactRatio}fr)`
    : "minmax(0, 1fr) minmax(0, 0fr)";

  React.useEffect(() => () => {
    artifactResizeCleanupRef.current?.();
  }, []);

  const onArtifactResizeStart = React.useCallback((event: React.PointerEvent<HTMLButtonElement>) => {
    const workspace = workspaceRef.current;
    if (!workspace || event.button !== 0) {
      return;
    }

    event.preventDefault();
    artifactResizeCleanupRef.current?.();
    setArtifactResizing(true);
    const resizeHandle = event.currentTarget;
    const pointerID = event.pointerId;
    const startClientX = event.clientX;
    const startRatio = artifactWorkspace.artifactRatio;

    const previousCursor = document.body.style.cursor;
    const previousUserSelect = document.body.style.userSelect;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";

    let stopped = false;
    const stopResize = () => {
      if (stopped) {
        return;
      }

      stopped = true;
      artifactResizeCleanupRef.current = null;
      setArtifactResizing(false);
      document.body.style.cursor = previousCursor;
      document.body.style.userSelect = previousUserSelect;
      if (resizeHandle.hasPointerCapture(pointerID)) {
        resizeHandle.releasePointerCapture(pointerID);
      }
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", stopResize);
      window.removeEventListener("pointercancel", stopResize);
      window.removeEventListener("blur", stopResize);
      document.removeEventListener("visibilitychange", stopResizeWhenHidden);
      resizeHandle.removeEventListener("lostpointercapture", stopResize);
    };
    const updateRatio = (clientX: number) => {
      const rect = workspace.getBoundingClientRect();
      if (rect.width <= 0) {
        stopResize();
        return;
      }

      const ratio = startRatio - ((clientX - startClientX) / rect.width);
      artifactWorkspace.setArtifactRatio(ratio);
    };
    const onPointerMove = (moveEvent: PointerEvent) => updateRatio(moveEvent.clientX);
    const stopResizeWhenHidden = () => {
      if (document.visibilityState === "hidden") {
        stopResize();
      }
    };

    resizeHandle.setPointerCapture(pointerID);
    artifactResizeCleanupRef.current = stopResize;
    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", stopResize);
    window.addEventListener("pointercancel", stopResize);
    window.addEventListener("blur", stopResize);
    document.addEventListener("visibilitychange", stopResizeWhenHidden);
    resizeHandle.addEventListener("lostpointercapture", stopResize);
  }, [artifactWorkspace]);

  const effectiveOptions = modelOptionPolicyDisabled ? EMPTY_CONVERSATION_OPTIONS : options;
  const selectedModelDefaultOptions = modelOptionPolicyDisabled
    ? EMPTY_CONVERSATION_OPTIONS
    : (selectedModel?.defaultOptions ?? EMPTY_CONVERSATION_OPTIONS);
  const resetFileDragState = React.useCallback(() => {
    fileDragDepthRef.current = 0;
    setFileDragActive(false);
  }, []);
  const onFileDragEnter = React.useCallback((event: React.DragEvent<HTMLDivElement>) => {
    if (!dragEventContainsFiles(event)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    if (uploadDropDisabled) {
      return;
    }
    fileDragDepthRef.current += 1;
    setFileDragActive(true);
  }, [uploadDropDisabled]);
  const onFileDragOver = React.useCallback((event: React.DragEvent<HTMLDivElement>) => {
    if (!dragEventContainsFiles(event)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = uploadDropDisabled ? "none" : "copy";
  }, [uploadDropDisabled]);
  const onFileDragLeave = React.useCallback((event: React.DragEvent<HTMLDivElement>) => {
    if (!dragEventContainsFiles(event)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    fileDragDepthRef.current = Math.max(0, fileDragDepthRef.current - 1);
    if (fileDragDepthRef.current === 0) {
      setFileDragActive(false);
    }
  }, []);
  const onFileDrop = React.useCallback((event: React.DragEvent<HTMLDivElement>) => {
    if (!dragEventContainsFiles(event)) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    const files = droppedFiles(event);
    resetFileDragState();
    if (uploadDropDisabled || files.length === 0) {
      return;
    }
    void onUploadFiles(files);
  }, [onUploadFiles, resetFileDragState, uploadDropDisabled]);
  React.useEffect(() => {
    if (uploadDropDisabled) {
      resetFileDragState();
    }
  }, [resetFileDragState, uploadDropDisabled]);

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
    selectedSkills,
    defaultToolIDs,
    htmlVisualPromptEnabled: htmlVisualPrompt.enabled,
    maxSelectedTools: mcpMaxSelectedTools,
    toolsLoading,
    options: effectiveOptions,
    defaultOptions: selectedModelDefaultOptions,
    modelOptionPolicy,
    modelLoading: modelsLoading,
    dropActive: fileDragActive,
    onDraftChange: setDraft,
    onModelChange: setSelectedPlatformModelName,
    onModelCatalogRefresh: refreshModelCatalogForComposer,
    onSelectedToolsChange: setSelectedToolIDs,
    maxSelectedSkills: mcpMaxSelectedTools,
    onSelectedSkillsChange: setSelectedSkills,
    onDefaultToolsChange: onDefaultToolIDsChange,
    onHTMLVisualPromptChange: htmlVisualPrompt.setEnabled,
    onOptionsChange: setModelOptions,
    onOptionsReset: resetModelOptions,
    onOptionsDefaultRestore: restoreBackendDefaultModelOptions,
    onAttachExistingFile,
    onUploadFiles,
    onCaptureScreenshot,
    onRemoveAttachment,
    onSendMessage,
    onStopMessage: onStopActiveMessage,
  };
  const chatContentWidthClassName = resolveChatContentWidthClassName(contentWidth);
  const isConversationLoading = Boolean(conversationID) && loading && visibleMessageCount === 0 && messagesWithInlineError.length === 0;
  const isConversationLoadFailed = Boolean(conversationID) && !loading && errorMsg.trim().length > 0 && visibleMessageCount === 0;
  const shouldUseCenteredComposer =
    !isConversationLoading && !isConversationLoadFailed && !isConversationMode && messagesWithInlineError.length === 0;

  return (
    <div
      className="relative flex h-full min-h-0 w-full flex-1 flex-col overflow-hidden md:overflow-visible"
      onDragEnter={onFileDragEnter}
      onDragOver={onFileDragOver}
      onDragLeave={onFileDragLeave}
      onDrop={onFileDrop}
    >
      {shouldUseCenteredComposer ? (
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <ChatEmptyState
            greetingTitle={activeRouteProject?.name || greetingTitle}
            badgeLabel={activeRouteProject ? t("projectMode") : undefined}
            badgeTooltip={activeRouteProject ? t("projectModeTooltip") : undefined}
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
                  messageEndRef={messageEndRef}
                  onScroll={onScroll}
                  onScrollToLatest={onScrollToLatest}
                  showScrollToLatestButton={showScrollToLatestButton}
                  onRetryUserMessage={onRetryUserMessage}
                  onRetryAssistantMessage={onRetryAssistantMessage}
                  onContinueAssistantMessage={onContinueAssistantMessage}
                  onEditAssistantMessage={onEditAssistantMessage}
                  onEditUserMessage={onEditUserMessage}
                  modelOptions={modelOptions}
                  selectedPlatformModelName={selectedPlatformModelName}
                  onModelChange={setSelectedPlatformModelName}
                  onModelCatalogRefresh={refreshModelCatalogForComposer}
                  onEditImageAttachment={onEditGeneratedImageAttachment}
                  onOpenCodeArtifact={artifactWorkspace.openArtifact}
                  onCycleMessageBranch={onCycleMessageBranch}
                  onToggleStar={onToggleActiveConversationStar}
                  onRename={onRenameActiveConversation}
                  onAutoRename={onAutoRenameActiveConversation}
                  projectMenu={{
                    label: t("labelMenu.moveToProject"),
                    unassignedLabel: t("labelMenu.unassignedProject"),
                    currentProjectID: currentConversation?.projectID,
                    projects,
                    onSelect: onSetActiveConversationProject,
                  }}
                  onShare={onShareActiveConversation}
                  shareActive={activeConversationShared}
                  onExport={onExportActiveConversation}
                  onDelete={onRequestDeleteActiveConversation}
                  markdownRender={markdownRender}
                  showModelInfo={showModelInfo}
                  showLatency={showLatency}
                  showTokenUsage={showTokenUsage}
                  showBillingCost={showBillingCost}
                  splitRightInset={hasInlineArtifact}
                  contentWidthClassName={chatContentWidthClassName}
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
