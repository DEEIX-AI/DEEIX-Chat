"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import dynamic from "next/dynamic";
import { AnimatePresence, motion } from "motion/react";
import { Check, FileText, Image, ImageOff, ImagePlus, Wrench } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { AudioLines } from "@/components/animate-ui/icons/audio-lines";
import { Blocks } from "@/components/animate-ui/icons/blocks";
import { Pause } from "@/components/animate-ui/icons/pause";
import { Send } from "@/components/animate-ui/icons/send";
import { Link as LinkIcon } from "@/components/animate-ui/icons/link";
import { Crop } from "@/components/animate-ui/icons/crop";
import { X as XIcon } from "@/components/animate-ui/icons/x";
import { PlusIcon } from "@/components/ui/plus";
import type {
  ChatModelOption,
  PendingAttachment,
  UploadingAttachment,
} from "@/features/chat/types/chat-runtime";
import { useSpeechInput } from "@/features/chat/hooks/use-speech-input";
import {
  useChatMentionMenu,
  type ChatMentionMenuItem,
  type ChatMentionMenuKind,
  type ChatMentionMenuLayout,
  type ChatMentionMenuSection,
} from "@/features/chat/hooks/use-chat-mention-menu";
import { ChatMCP } from "@/features/chat/components/sections/chat-mcp";
import { ChatModelPicker } from "@/features/chat/components/sections/chat-model-picker";
import { ChatModelConfig } from "@/features/chat/components/sections/chat-model-config";
import { formatBytes, resolveFileIcon } from "@/features/files/utils/file-display";
import type { ChatSubmitDecision } from "@/features/chat/model/chat-task";
import { isMediaSubmitTask, resolveChatSubmitDecision } from "@/features/chat/model/chat-task";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupTextarea,
} from "@/components/ui/input-group";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { resolveFileProcessingBadge, resolveFileProcessingToneClass } from "@/shared/lib/file-processing";
import { cn } from "@/lib/utils";
import { LobeHubIcon } from "@/shared/components/lobehub-icon";
import { resolveLobeHubIconURL, resolveModelIdentity } from "@/shared/lib/model-identity";
import type { ConversationOptions } from "@/shared/api/conversation.types";
import type { FileObjectDTO } from "@/shared/api/file.types";
import type { MCPToolDTO } from "@/shared/api/mcp.types";
import type { ModelOptionPolicy } from "@/shared/lib/model-option-policy";
import type { SendShortcut } from "@/features/settings/types/settings";
import { isSendShortcutEvent } from "@/shared/lib/platform-shortcuts";

const FilePreviewDialog = dynamic(
  () => import("@/features/files/components/preview/file-preview-dialog").then((module) => module.FilePreviewDialog),
  { ssr: false },
);

type ChatInputProps = {
  draft: string;
  loading: boolean;
  sending: boolean;
  uploading: boolean;
  isConversationMode: boolean;
  maxFilesPerMessage: number;
  fileMode?: "auto" | "full_context" | "rag";
  sendShortcut?: SendShortcut;
  inputHeight?: "compact" | "standard" | "loose";
  attachments: PendingAttachment[];
  uploadingAttachments: UploadingAttachment[];
  modelOptions: ChatModelOption[];
  selectedPlatformModelName: string;
  availableTools: MCPToolDTO[];
  selectedToolIDs: number[];
  defaultToolIDs: number[];
  htmlVisualPromptEnabled: boolean;
  maxSelectedTools: number;
  toolsLoading: boolean;
  options: ConversationOptions;
  defaultOptions: ConversationOptions;
  modelOptionPolicy: ModelOptionPolicy | null;
  modelLoading: boolean;
  modelDisabled?: boolean;
  dropActive?: boolean;
  onDraftChange: (value: string) => void;
  onModelChange: (platformModelName: string) => void;
  onSelectedToolsChange: (toolIDs: number[]) => void;
  onDefaultToolsChange: (toolIDs: number[]) => void | Promise<void>;
  onHTMLVisualPromptChange: (enabled: boolean) => void;
  onOptionsChange: React.Dispatch<React.SetStateAction<ConversationOptions>>;
  onOptionsReset: (defaults?: ConversationOptions) => void;
  onOptionsDefaultRestore: () => Promise<ConversationOptions | null>;
  onAttachExistingFile: (file: FileObjectDTO) => void | Promise<void>;
  onUploadFiles: (files: File[]) => void | Promise<void>;
  onCaptureScreenshot: () => void | Promise<void>;
  onRemoveAttachment: (fileID: string) => void;
  onSendMessage: () => void | Promise<void>;
  onStopMessage: () => void;
};

type ComposerModeIndicator = {
  label: string;
  intro: string;
  description: string;
  icon: React.ComponentType<{ className?: string; strokeWidth?: number }>;
  tone: "default" | "warning";
};

function MentionMenuItem({
  item,
  active,
  onSelect,
}: {
  item: ChatMentionMenuItem;
  active: boolean;
  onSelect: () => void;
}) {
  const platformModelName = item.kind === "model" ? item.model.platformModelName.trim() : "";
  const identity = React.useMemo(() => {
    if (item.kind !== "model") {
      return null;
    }
    return resolveModelIdentity({
      code: item.model.platformModelName,
      vendor: item.model.vendor,
      icon: item.model.icon,
    });
  }, [item]);
  const iconURL = React.useMemo(() => identity ? resolveLobeHubIconURL(identity.modelIcon) : "", [identity]);

  return (
    <button
      type="button"
      role="option"
      aria-selected={active}
      data-active={active}
      className="flex h-8 w-full min-w-0 items-center gap-2 rounded-md px-2 text-left text-[11px] font-medium text-muted-foreground outline-none transition-colors hover:bg-accent hover:text-accent-foreground data-[active=true]:bg-accent data-[active=true]:text-accent-foreground"
      onMouseDown={(event) => {
        event.preventDefault();
        onSelect();
      }}
    >
      {item.kind === "model" ? (
        <LobeHubIcon iconUrl={iconURL} label={platformModelName} />
      ) : item.kind === "file" ? (
        <span className="flex size-4 shrink-0 items-center justify-center rounded-sm text-muted-foreground">
          <FileText className="size-3.5" strokeWidth={1.7} />
        </span>
      ) : (
        <span className="flex size-4 shrink-0 items-center justify-center rounded-sm text-muted-foreground">
          <Wrench className="size-3.5" strokeWidth={1.7} />
        </span>
      )}
      <span className="flex min-w-0 flex-1 items-baseline gap-2 overflow-hidden">
        <span
          className={cn(
            "text-foreground/90",
            item.kind === "tool" ? "shrink-0 whitespace-nowrap" : "min-w-0 truncate",
          )}
        >
          {item.label}
        </span>
        {item.description ? (
          <span className="min-w-0 truncate font-normal text-muted-foreground/80">{item.description}</span>
        ) : null}
      </span>
      <span className="flex size-3.5 shrink-0 items-center justify-center">
        {item.selected ? <Check className="size-3.5 text-current" strokeWidth={1.8} /> : null}
      </span>
    </button>
  );
}

const MentionMenuContent = React.memo(function MentionMenuContent({
  activeIndex,
  sectionOffsets,
  sections,
  t,
  onSelect,
}: {
  activeIndex: number;
  sectionOffsets: Map<ChatMentionMenuKind, number>;
  sections: ChatMentionMenuSection[];
  t: (key: string) => string;
  onSelect: (item: ChatMentionMenuItem) => void;
}) {
  return (
    <>
      {sections.map((section) => {
        const sectionOffset = sectionOffsets.get(section.kind) ?? 0;
        return (
          <div key={section.kind} className="space-y-0.5">
            <div className="px-2 pb-1 pt-1.5 text-[11px] font-semibold text-muted-foreground">
              {t(`mention.sections.${section.kind}`)}
            </div>
            {section.items.map((item, index) => (
              <MentionMenuItem
                key={item.id}
                item={item}
                active={sectionOffset + index === activeIndex}
                onSelect={() => onSelect(item)}
              />
            ))}
          </div>
        );
      })}
    </>
  );
});

function resolveMentionMenuMotionStyle(layout: ChatMentionMenuLayout | null): React.CSSProperties | undefined {
  if (!layout) {
    return undefined;
  }
  return {
    bottom: layout.bottom,
    left: layout.left,
    top: layout.top,
    width: layout.width,
    contain: "layout paint",
    transformOrigin: layout.placement === "bottom" ? "top center" : "bottom center",
    willChange: "height, opacity, transform",
  };
}

function resolveComposerModeIndicator(
  decision: ChatSubmitDecision,
  t: (key: string) => string,
): ComposerModeIndicator | null {
  if (decision.blockedReason === "image_task_rejects_non_image_attachments") {
    return {
      label: t("mediaMode.invalidFile"),
      intro: t("mediaMode.invalidFileIntro"),
      description: t(`mediaMode.blockedDescriptions.${decision.blockedReason}`),
      icon: ImageOff,
      tone: "warning",
    };
  }
  if (decision.task === "image_generation") {
    return {
      label: t("mediaMode.imageGeneration"),
      intro: t("mediaMode.imageGenerationIntro"),
      description: decision.blockedReason
        ? t(`mediaMode.blockedDescriptions.${decision.blockedReason}`)
        : t("mediaMode.imageGenerationDescription"),
      icon: Image,
      tone: "default",
    };
  }
  if (decision.task === "image_edit") {
    return {
      label: t("mediaMode.imageEdit"),
      intro: t("mediaMode.imageEditIntro"),
      description: decision.blockedReason
        ? t(`mediaMode.blockedDescriptions.${decision.blockedReason}`)
        : t("mediaMode.imageEditDescription"),
      icon: ImagePlus,
      tone: "default",
    };
  }
  return null;
}

function clipboardFilesFromPaste(event: React.ClipboardEvent<HTMLTextAreaElement>): File[] {
  const itemFiles = Array.from(event.clipboardData.items ?? [])
    .filter((item) => item.kind === "file")
    .map((item) => item.getAsFile())
    .filter((file): file is File => file !== null);
  const sourceFiles = itemFiles.length > 0 ? itemFiles : Array.from(event.clipboardData.files ?? []);
  const pastedAt = Date.now();

  return sourceFiles.map((file, index) => {
    if (file.name.trim()) {
      return file;
    }
    const extension = file.type.startsWith("image/") ? ".png" : "";
    const prefix = file.type.startsWith("image/") ? "pasted-image" : "pasted-file";
    return new File([file], `${prefix}-${pastedAt}-${index + 1}${extension}`, {
      type: file.type,
      lastModified: file.lastModified,
    });
  });
}

function ChatInputComponent({
  draft,
  loading,
  sending,
  uploading,
  fileMode,
  sendShortcut = "enter",
  inputHeight = "standard",
  attachments,
  uploadingAttachments,
  modelOptions,
  selectedPlatformModelName,
  availableTools,
  selectedToolIDs,
  defaultToolIDs,
  htmlVisualPromptEnabled,
  maxSelectedTools,
  toolsLoading,
  options,
  defaultOptions,
  modelOptionPolicy,
  modelLoading,
  modelDisabled = false,
  dropActive = false,
  onDraftChange,
  onModelChange,
  onSelectedToolsChange,
  onDefaultToolsChange,
  onHTMLVisualPromptChange,
  onOptionsChange,
  onOptionsReset,
  onOptionsDefaultRestore,
  onAttachExistingFile,
  onUploadFiles,
  onCaptureScreenshot,
  onRemoveAttachment,
  onSendMessage,
  onStopMessage,
}: ChatInputProps) {
  const tChat = useTranslations("chat");
  const tComposer = useTranslations("chat.composer");
  const tFileStatus = useTranslations("files.status");
  const [isBlocksHovered, setIsBlocksHovered] = React.useState(false);
  const [isVoiceHovered, setIsVoiceHovered] = React.useState(false);
  const speechInput = useSpeechInput({
    draft,
    listeningPlaceholder: tComposer("voiceListeningPlaceholder"),
    onDraftChange,
    placeholder: tComposer("inputPlaceholder"),
  });
  const [hoveredTool, setHoveredTool] = React.useState<"upload" | "screenshot" | null>(null);
  const [ragWarnDismissed, setRagWarnDismissed] = React.useState(false);
  const [previewAttachment, setPreviewAttachment] = React.useState<PendingAttachment | null>(null);
  const fileInputRef = React.useRef<HTMLInputElement | null>(null);
  const inputGroupRef = React.useRef<HTMLDivElement | null>(null);
  const textareaRef = React.useRef<HTMLTextAreaElement | null>(null);
  const composingRef = React.useRef(false);
  const hasDraftText = draft.trim().length > 0;
  const canSend = (draft.trim().length > 0 || attachments.length > 0) && !sending && !loading && !uploading;
  const inputHeightClassName =
    inputHeight === "compact" ? "max-h-32" : inputHeight === "loose" ? "max-h-64" : "max-h-44";

  // Only relevant in RAG mode: all document attachments opted out of RAG.
  const docAttachments = attachments.filter((a) => a.fileCategory !== "image");
  const allRagOptOut =
    fileMode === "rag" &&
    docAttachments.length > 0 &&
    docAttachments.every((a) => a.ragOptOut === true);
  const showRagWarn = allRagOptOut && !ragWarnDismissed;

  const closePreviewDialog = React.useCallback((open: boolean) => {
    if (!open) {
      setPreviewAttachment(null);
    }
  }, []);

  const selectedModel = React.useMemo(
    () => modelOptions.find((item) => item.platformModelName === selectedPlatformModelName) ?? null,
    [modelOptions, selectedPlatformModelName],
  );
  const selectedProtocol = selectedModel?.protocols[0]?.trim() ?? "";
  const selectedModelName = selectedModel?.platformModelName || selectedPlatformModelName;
  const submitDecision = resolveChatSubmitDecision(selectedModel, attachments);
  const submitTask = submitDecision.task;
  const isMediaMode = isMediaSubmitTask(submitTask);
  const composerModeIndicator = resolveComposerModeIndicator(submitDecision, tComposer);
  const ComposerModeIcon = composerModeIndicator?.icon;
  const modelOptionPolicyDisabled = modelOptionPolicy?.mode?.trim() === "disabled";
  const showMCPToolsButton = availableTools.length > 0 && !isMediaMode;
  const showHTMLVisualPromptButton = !isMediaMode;
  const {
    activeIndex: mentionActiveIndex,
    handleBlur: handleMentionBlur,
    handleChange: handleMentionChange,
    handleFocus: handleMentionFocus,
    handleKeyDown: handleMentionKeyDown,
    menuID: mentionMenuID,
    menuLayout: mentionMenuLayout,
    menuRef: mentionMenuRef,
    menuReady: mentionMenuReady,
    open: showMentionMenu,
    sections: mentionSections,
    select: selectMentionItem,
  } = useChatMentionMenu({
    attachments,
    availableTools,
    defaultFileLabel: tComposer("mention.fileFallback"),
    disabled: sending || loading || uploading || modelLoading || modelDisabled,
    draft,
    maxSelectedTools,
    modelOptions,
    selectedPlatformModelName,
    selectedToolIDs,
    anchorRef: inputGroupRef,
    textareaRef,
    toolsDisabled: isMediaMode,
    onDraftChange,
    onFileSelect: onAttachExistingFile,
    onModelChange,
    onSelectedToolsChange,
    onToolLimitReached: () => {
      toast.error(tComposer("mcpToolLimitTitle"), {
        description: tComposer("mcpToolLimitDescription", { limit: maxSelectedTools }),
      });
    },
  });
  const mentionSectionOffsets = React.useMemo(() => {
    const offsets = new Map<ChatMentionMenuKind, number>();
    let offset = 0;
    for (const section of mentionSections) {
      offsets.set(section.kind, offset);
      offset += section.items.length;
    }
    return offsets;
  }, [mentionSections]);
  const mentionMenuMotionStyle = React.useMemo(
    () => resolveMentionMenuMotionStyle(mentionMenuLayout),
    [mentionMenuLayout],
  );
  const mentionMenuHeight = mentionMenuLayout?.height ?? 0;
  const shouldRenderMentionMenu = showMentionMenu && mentionMenuReady && mentionMenuMotionStyle !== undefined;
  const onSelectUploadTool = React.useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const onSelectScreenshotTool = React.useCallback(() => {
    void onCaptureScreenshot();
  }, [onCaptureScreenshot]);

  return (
    <div className="w-full">
      <input
        ref={fileInputRef}
        type="file"
        multiple
        className="sr-only "
        onChange={(event) => {
          const files = Array.from(event.target.files ?? []);
          if (files.length > 0) {
            void onUploadFiles(files);
          }
          event.currentTarget.value = "";
        }}
      />

      <InputGroup
        ref={inputGroupRef}
        className={cn(
          "bg-pure rounded-3xl border-[0.5px] border-border/70 shadow-xs has-[[data-slot=input-group-control]:focus-visible]:ring-0 has-[[data-slot=input-group-control]:focus-visible]:border-border",
          dropActive && "border-dashed border-foreground/30 bg-muted/20 shadow-none",
        )}
      >
        {attachments.length > 0 || uploadingAttachments.length > 0 ? (
          <div className="w-full space-y-2 px-2.5 pt-2">
            {showRagWarn ? (
              <div className="flex items-center gap-2 rounded-lg border border-amber-200/70 bg-amber-50/70 px-3 py-2 text-[11px] text-amber-700 dark:border-amber-700/40 dark:bg-amber-950/30 dark:text-amber-400">
                <span className="shrink-0">⚠</span>
                <span className="flex-1">{tComposer("ragAllDisabled")}</span>
                <button
                  type="button"
                  className="shrink-0 text-amber-500 hover:text-amber-700 dark:text-amber-500 dark:hover:text-amber-300"
                  onClick={() => setRagWarnDismissed(true)}
                  aria-label={tComposer("closeHint")}
                >
                  ✕
                </button>
              </div>
            ) : null}
            <div className="w-full overflow-hidden sm:overflow-x-auto">
              <div className="flex max-h-[196px] w-full flex-col gap-2 overflow-y-auto pb-1 pl-1.5 pr-2 pt-2 sm:max-h-none sm:w-max sm:flex-row sm:overflow-y-visible sm:pr-1.5">
                {attachments.map((item) => (
                  <div
                    key={item.fileID}
                    className="group relative flex h-14 w-full shrink-0 items-center gap-1.5 rounded-lg bg-muted/35 px-2 text-left transition-colors hover:bg-muted/50 dark:bg-white/[0.06] dark:hover:bg-white/[0.09] sm:w-[228px] sm:px-2.5"
                  >
                    <button
                      type="button"
                      className="flex min-w-0 flex-1 items-center gap-2.5 rounded-md py-1 text-left outline-none transition-colors focus-visible:ring-[3px] focus-visible:ring-ring/35"
                      onClick={() => setPreviewAttachment(item)}
                      aria-label={tComposer("previewAttachment", { name: item.fileName })}
                    >
                    {(() => {
                      const badge = resolveFileProcessingBadge(item, (key, values) => tFileStatus(key, values));
                      const FileIcon = resolveFileIcon(item);
                      return (
                        <>
                          <div className="flex size-6 shrink-0 items-center justify-center">
                            <FileIcon className="size-5 text-muted-foreground" strokeWidth={1.6} />
                          </div>
                          <div className="flex min-w-0 flex-1 flex-col justify-center">
                            <p className="truncate text-[12px] font-medium leading-4 text-foreground/90" title={item.fileName}>
                              {item.fileName}
                            </p>
                            <div className="mt-1 flex min-w-0 items-center gap-1.5">
                              <span className="min-w-0 shrink truncate text-[10px] leading-none text-muted-foreground">
                                {formatBytes(item.sizeBytes)}
                              </span>
                              <span
                                className={cn(
                                  "inline-flex max-w-[82px] shrink-0 items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium leading-none",
                                  resolveFileProcessingToneClass(badge.tone),
                                )}
                                title={badge.detail}
                              >
                                <span className="truncate">{badge.label}</span>
                              </span>
                              {item.ragOptOut && item.fileCategory !== "image" ? (
                                <span
                                  className="shrink-0 rounded-md bg-muted/60 px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground/65"
                                  title={tComposer("ragDisabledTitle")}
                                >
                                  {tComposer("ragOff")}
                                </span>
                              ) : null}
                            </div>
                          </div>
                        </>
                      );
                    })()}
                    </button>
                    <button
                      type="button"
                      className="inline-flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/35 sm:size-7"
                      onClick={() => onRemoveAttachment(item.fileID)}
                      aria-label={tComposer("removeAttachment", { name: item.fileName })}
                    >
                      <XIcon size={15} strokeWidth={1.8} animateOnHover="default" />
                    </button>
                  </div>
                ))}
                {uploadingAttachments.map((item) => (
                  <div
                    key={item.tempID}
                    className="relative flex h-14 w-full shrink-0 items-center gap-2.5 rounded-lg bg-muted/35 px-2.5 dark:bg-white/[0.06] sm:w-[228px]"
                    aria-label={tComposer("uploadingAttachment", { name: item.fileName })}
                  >
                    <Skeleton className="size-5 shrink-0 rounded-sm" />
                    <div className="min-w-0 flex-1 space-y-2">
                      <Skeleton className="h-3 w-[78%]" />
                      <div className="flex items-center gap-1.5">
                        <Skeleton className="h-2.5 w-10" />
                        <Skeleton className="h-4 w-12 rounded-md" />
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
            {previewAttachment ? (
              <FilePreviewDialog
                file={previewAttachment}
                open={previewAttachment !== null}
                onOpenChange={closePreviewDialog}
              />
            ) : null}
          </div>
        ) : null}

        {typeof document !== "undefined" ? createPortal(
          <AnimatePresence initial={false}>
            {shouldRenderMentionMenu ? (
              <motion.div
                ref={mentionMenuRef}
                id={mentionMenuID}
                key="chat-mention-menu"
                role="listbox"
                className="bg-pure fixed z-[60] overflow-hidden rounded-xl border-[0.5px] border-border/70 text-popover-foreground shadow-xs"
                style={mentionMenuMotionStyle}
                initial={{
                  height: Math.min(mentionMenuHeight, 12),
                  opacity: 0,
                  scale: 0.99,
                  y: mentionMenuLayout?.placement === "top" ? 4 : -4,
                }}
                animate={{ height: mentionMenuHeight, opacity: 1, scale: 1, y: 0 }}
                exit={{
                  height: Math.min(mentionMenuHeight, 12),
                  opacity: 0,
                  scale: 0.99,
                  y: mentionMenuLayout?.placement === "top" ? 4 : -4,
                }}
                transition={{
                  height: { type: "spring", stiffness: 520, damping: 42, mass: 0.75 },
                  opacity: { duration: 0.1, ease: "easeOut" },
                  scale: { duration: 0.12, ease: "easeOut" },
                  y: { duration: 0.12, ease: "easeOut" },
                }}
              >
                <div data-mention-menu-scroll className="h-full overflow-y-auto p-1.5">
                  <MentionMenuContent
                    activeIndex={mentionActiveIndex}
                    sectionOffsets={mentionSectionOffsets}
                    sections={mentionSections}
                    t={tComposer}
                    onSelect={selectMentionItem}
                  />
                </div>
              </motion.div>
            ) : null}
          </AnimatePresence>,
          document.body,
        ) : null}

        <InputGroupTextarea
          ref={textareaRef}
          value={draft}
          disabled={sending || loading || uploading}
          readOnly={speechInput.active}
          placeholder={dropActive ? tChat("attachments.dropTitle") : speechInput.placeholder}
          rows={1}
          aria-controls={showMentionMenu ? mentionMenuID : undefined}
          aria-expanded={showMentionMenu ? true : undefined}
          style={{ fontFamily: "var(--font-chat)", fontWeight: "var(--font-chat-weight)" }}
          className={cn(
            "rounded-3xl min-h-12 overflow-y-auto px-5 pt-4 text-[15px] leading-6 placeholder:text-muted-foreground placeholder:font-[inherit] placeholder:leading-[inherit]",
            inputHeightClassName,
            speechInput.active ? "placeholder:font-normal placeholder:text-muted-foreground" : "",
          )}
          onFocus={handleMentionFocus}
          onBlur={handleMentionBlur}
          onChange={(event) => handleMentionChange(event.target.value)}
          onPaste={(event) => {
            const files = clipboardFilesFromPaste(event);
            if (files.length === 0) {
              return;
            }
            if (!event.clipboardData.getData("text/plain")) {
              event.preventDefault();
            }
            void onUploadFiles(files);
          }}
          onCompositionStart={() => {
            composingRef.current = true;
          }}
          onCompositionEnd={() => {
            composingRef.current = false;
          }}
          onKeyDown={(event) => {
            if (event.nativeEvent.isComposing || composingRef.current || event.key === "Process" || event.keyCode === 229) {
              return;
            }
            const shouldSend = isSendShortcutEvent(sendShortcut, event);

            if (handleMentionKeyDown(event)) {
              return;
            }

            if (shouldSend) {
              event.preventDefault();
              if (canSend) {
                void onSendMessage();
              }
            }
          }}
        />

        <InputGroupAddon align="block-end" className="items-center justify-between pt-2">
          <div className="flex shrink-0 items-center gap-0.5 sm:gap-1">
            <DropdownMenu modal={false}>
              <DropdownMenuTrigger asChild>
                <InputGroupButton
                  id="chat-tools-menu-trigger"
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="size-7 rounded-md text-muted-foreground hover:text-foreground sm:size-8"
                  disabled={sending || loading || uploading}
                  aria-label={tComposer("openTools")}
                >
                  <PlusIcon
                    size={20}
                    strokeWidth={1.4}
                  />
                </InputGroupButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="bottom" align="start" sideOffset={8} className="w-36">
                <DropdownMenuItem
                  onMouseEnter={() => setHoveredTool("upload")}
                  onMouseLeave={() => setHoveredTool((prev) => (prev === "upload" ? null : prev))}
                  onSelect={(event) => {
                    event.preventDefault();
                    onSelectUploadTool();
                  }}
                >
                  <LinkIcon size={12} strokeWidth={1.5} animate={hoveredTool === "upload" ? "default" : undefined} />
                  {tComposer("uploadFile")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  onMouseEnter={() => setHoveredTool("screenshot")}
                  onMouseLeave={() => setHoveredTool((prev) => (prev === "screenshot" ? null : prev))}
                  onSelect={(event) => {
                    event.preventDefault();
                    onSelectScreenshotTool();
                  }}
                >
                  <Crop size={12} strokeWidth={1.5} animate={hoveredTool === "screenshot" ? "default" : undefined} />
                  {tComposer("screenshot")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            {!modelOptionPolicyDisabled ? (
              <ChatModelConfig
                disabled={sending || loading || uploading || modelLoading}
                options={options}
                defaultOptions={defaultOptions}
                optionControls={selectedModel?.optionControls ?? []}
                nativeToolKeys={selectedModel?.nativeToolKeys ?? []}
                nativeTools={selectedModel?.nativeTools ?? []}
                modelOptionPolicy={modelOptionPolicy}
                selectedProtocol={selectedProtocol}
                selectedModelName={selectedModelName}
                onOptionsChange={onOptionsChange}
                onOptionsReset={onOptionsReset}
                onDefaultOptionsRestore={onOptionsDefaultRestore}
              />
            ) : null}

            {showMCPToolsButton ? (
              <ChatMCP
                availableTools={availableTools}
                selectedToolIDs={selectedToolIDs}
                defaultToolIDs={defaultToolIDs}
                maxSelectedTools={maxSelectedTools}
                disabled={sending || loading || uploading || toolsLoading}
                onSelectedToolsChange={onSelectedToolsChange}
                onDefaultToolsChange={onDefaultToolsChange}
              />
            ) : null}

            {showHTMLVisualPromptButton ? (
              <Tooltip>
                <TooltipTrigger asChild>
                  <InputGroupButton
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className={cn(
                      "size-7 rounded-md text-muted-foreground hover:text-foreground sm:size-8",
                      htmlVisualPromptEnabled && "bg-primary/10 text-primary hover:bg-primary/10 hover:text-primary",
                    )}
                    disabled={sending || loading || uploading}
                    aria-label={tComposer("htmlVisualPrompt")}
                    aria-pressed={htmlVisualPromptEnabled}
                    onClick={() => onHTMLVisualPromptChange(!htmlVisualPromptEnabled)}
                    onMouseEnter={() => setIsBlocksHovered(true)}
                    onMouseLeave={() => setIsBlocksHovered(false)}
                  >
                    <Blocks
                      size={20}
                      strokeWidth={1.4}
                      animate={htmlVisualPromptEnabled ? "default" : isBlocksHovered ? "default" : undefined}
                    />
                  </InputGroupButton>
                </TooltipTrigger>
                <TooltipContent side="top" className="max-w-72 text-xs leading-5">
                  {htmlVisualPromptEnabled
                    ? tComposer("htmlVisualPromptEnabled")
                    : tComposer("htmlVisualPromptDisabled")}
                </TooltipContent>
              </Tooltip>
            ) : null}
          </div>

          <div className="flex min-w-0 flex-1 items-center justify-end gap-1 overflow-hidden sm:gap-1.5">
            {composerModeIndicator && ComposerModeIcon ? (
              <Tooltip>
                <TooltipTrigger asChild>
                  <span
                    className={cn(
                      "inline-flex h-8 shrink-0 items-center gap-1.5 rounded-lg px-2 text-[11px] font-medium transition-colors",
                      composerModeIndicator.tone === "warning"
                        ? "bg-destructive/10 text-destructive"
                        : "bg-muted/60 text-muted-foreground",
                    )}
                  >
                    <ComposerModeIcon className="size-3.5" strokeWidth={1.7} />
                    <span className="hidden sm:inline">{composerModeIndicator.label}</span>
                  </span>
                </TooltipTrigger>
                <TooltipContent side="top" align="end" className="max-w-72 text-xs leading-5">
                  {composerModeIndicator.intro} {composerModeIndicator.description}
                </TooltipContent>
              </Tooltip>
            ) : null}
            <ChatModelPicker
              modelOptions={modelOptions}
              selectedPlatformModelName={selectedPlatformModelName}
              loading={modelLoading}
              disabled={modelDisabled}
              onModelChange={onModelChange}
            />

            <InputGroupButton
              type="button"
              variant="ghost"
              size="icon-sm"
              className="size-7 rounded-md text-muted-foreground hover:text-foreground sm:size-8"
              disabled={loading || uploading || (!sending && !hasDraftText && !speechInput.supported)}
              onClick={sending ? onStopMessage : hasDraftText ? onSendMessage : speechInput.toggle}
              onMouseEnter={() => setIsVoiceHovered(true)}
              onMouseLeave={() => setIsVoiceHovered(false)}
              aria-label={sending ? tComposer("pauseGeneration") : hasDraftText ? tChat("send") : speechInput.active ? tComposer("cancelVoiceInput") : tComposer("voiceInput")}
              title={sending ? tComposer("pauseGeneration") : hasDraftText ? tChat("send") : speechInput.supported ? (speechInput.active ? tComposer("cancelVoiceInput") : tComposer("voiceInput")) : tComposer("voiceUnsupported")}
            >
              {sending ? (
                <Pause
                  size={20}
                  strokeWidth={1.4}
                  animate="default-loop"
                />
              ) : speechInput.active ? (
                <AudioLines
                  size={20}
                  strokeWidth={1.4}
                  animate="default"
                />
              ) : hasDraftText ? (
                <Send
                  size={20}
                  strokeWidth={1.4}
                  animate={isVoiceHovered ? "default" : undefined}
                />
              ) : (
                <AudioLines
                  size={20}
                  strokeWidth={1.4}
                  animate={isVoiceHovered ? "default" : undefined}
                />
              )}
            </InputGroupButton>
          </div>
        </InputGroupAddon>
      </InputGroup>
    </div>
  );
}

export const ChatInput = React.memo(ChatInputComponent);
ChatInput.displayName = "ChatInput";
