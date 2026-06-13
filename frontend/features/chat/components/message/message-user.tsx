"use client";

import * as React from "react";
import { CircleAlert } from "lucide-react";
import { motion } from "motion/react";
import { useTranslations } from "next-intl";

import { ChevronDown } from "@/components/animate-ui/icons/chevron-down";
import { ChevronUp } from "@/components/animate-ui/icons/chevron-up";
import { ChatMentionMenuPortal } from "@/features/chat/components/shared/chat-mention-menu";
import { MessageAttachmentRow } from "@/features/chat/components/message/message-attachment";
import { UserMessageMeta } from "@/features/chat/components/message/message-meta";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import {
  useChatMentionMenu,
  type ChatMentionMenuKind,
} from "@/features/chat/hooks/use-chat-mention-menu";
import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import type { FileContentResult } from "@/shared/api/file";
import type { PreviewDialogFile } from "@/shared/components/file-preview/file-preview-dialog";

const USER_MESSAGE_COLLAPSED_LINES = 6;
const USER_MESSAGE_LINE_HEIGHT_REM = 2;
const USER_MESSAGE_COLLAPSED_FALLBACK_HEIGHT = USER_MESSAGE_COLLAPSED_LINES * USER_MESSAGE_LINE_HEIGHT_REM * 16;
const USER_MESSAGE_EXPAND_TRANSITION = {
  duration: 0.36,
  ease: [0.16, 1, 0.3, 1] as const,
};
const EDIT_MESSAGE_MENTION_KINDS: readonly ChatMentionMenuKind[] = ["model", "prompt"];
const EDIT_MESSAGE_EMPTY_ATTACHMENTS = [];
const EDIT_MESSAGE_EMPTY_TOOLS = [];
const EDIT_MESSAGE_EMPTY_TOOL_IDS = [];

type ChatMessageUserProps = {
  item: ChatAreaMessage;
  busy: boolean;
  onRetryUserMessage: (message: ChatAreaMessage) => Promise<void> | void;
  onEditUserMessage: (message: ChatAreaMessage, content: string) => Promise<boolean> | boolean;
  modelOptions?: ChatModelOption[];
  selectedPlatformModelName?: string;
  onModelChange?: (platformModelName: string) => void;
  onModelCatalogRefresh?: () => void | Promise<void>;
  onCycleMessageBranch: (parentPublicID: string | null, direction: "previous" | "next") => void;
  onCopy: () => void;
  copySucceeded?: boolean;
  readOnly?: boolean;
  attachmentContentLoader?: (file: PreviewDialogFile) => Promise<FileContentResult>;
  showBranchNavigator?: boolean;
};

export function ChatMessageUser({
  item,
  busy,
  onRetryUserMessage,
  onEditUserMessage,
  modelOptions = [],
  selectedPlatformModelName = "",
  onModelChange = () => undefined,
  onModelCatalogRefresh,
  onCycleMessageBranch,
  onCopy,
  copySucceeded = false,
  readOnly = false,
  attachmentContentLoader,
  showBranchNavigator = true,
}: ChatMessageUserProps) {
  const tCommon = useTranslations("common.actions");
  const tComposer = useTranslations("chat.composer");
  const tMessages = useTranslations("chat.messages");
  const [isEditing, setIsEditing] = React.useState(false);
  const [editingValue, setEditingValue] = React.useState(item.content);
  const [expandedContentKey, setExpandedContentKey] = React.useState("");
  const [canCollapse, setCanCollapse] = React.useState(false);
  const [isToggleHovered, setIsToggleHovered] = React.useState(false);
  const [contentHeight, setContentHeight] = React.useState(0);
  const [collapsedHeight, setCollapsedHeight] = React.useState(USER_MESSAGE_COLLAPSED_FALLBACK_HEIGHT);
  const [measuredContentKey, setMeasuredContentKey] = React.useState("");
  const contentRef = React.useRef<HTMLDivElement>(null);
  const editInputGroupRef = React.useRef<HTMLDivElement | null>(null);
  const editTextareaRef = React.useRef<HTMLTextAreaElement | null>(null);
  const measurementKey = React.useMemo(
    () => `${item.publicID || item.key}:${item.content}`,
    [item.content, item.key, item.publicID],
  );
  const measured = measuredContentKey === measurementKey;
  const expanded = measured && expandedContentKey === measurementKey;
  const contentMaxHeight = expanded
    ? contentHeight
    : !measured || canCollapse
      ? collapsedHeight
      : undefined;

  React.useEffect(() => {
    setIsEditing(false);
  }, [item.publicID]);

  React.useEffect(() => {
    if (!isEditing) {
      setEditingValue(item.content);
    }
  }, [isEditing, item.content]);

  React.useLayoutEffect(() => {
    const element = contentRef.current;
    if (!element) {
      setCanCollapse(false);
      return;
    }

    const measure = () => {
      const lineHeight = Number.parseFloat(window.getComputedStyle(element).lineHeight);
      const collapsedHeight =
        Number.isFinite(lineHeight) && lineHeight > 0
          ? lineHeight * USER_MESSAGE_COLLAPSED_LINES
          : USER_MESSAGE_COLLAPSED_FALLBACK_HEIGHT;
      setContentHeight(element.scrollHeight);
      setCollapsedHeight(collapsedHeight);
      setCanCollapse(element.scrollHeight > collapsedHeight + 1);
      setMeasuredContentKey(measurementKey);
    };

    measure();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const resizeObserver = new ResizeObserver(measure);
    resizeObserver.observe(element);
    return () => resizeObserver.disconnect();
  }, [item.content, measurementKey]);

  const onRetry = React.useCallback(() => {
    void onRetryUserMessage(item);
  }, [item, onRetryUserMessage]);

  const onEditSave = React.useCallback(async () => {
    const nextContent = editingValue.trim();
    if (!nextContent || nextContent === item.content.trim()) {
      return;
    }
    const ok = await onEditUserMessage(item, nextContent);
    if (ok !== false) {
      setIsEditing(false);
    }
  }, [editingValue, item, onEditUserMessage]);
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
    attachments: EDIT_MESSAGE_EMPTY_ATTACHMENTS,
    availableTools: EDIT_MESSAGE_EMPTY_TOOLS,
    defaultFileLabel: "",
    disabled: busy || readOnly || !isEditing,
    draft: editingValue,
    enabledKinds: EDIT_MESSAGE_MENTION_KINDS,
    maxSelectedTools: 0,
    modelOptions,
    selectedPlatformModelName,
    selectedToolIDs: EDIT_MESSAGE_EMPTY_TOOL_IDS,
    anchorRef: editInputGroupRef,
    textareaRef: editTextareaRef,
    toolsDisabled: true,
    onDraftChange: setEditingValue,
    onFileSelect: () => undefined,
    onModelCatalogRefresh,
    onModelChange,
    onSelectedToolsChange: () => undefined,
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

  if (!readOnly && isEditing) {
    const nextContent = editingValue.trim();
    const unchanged = nextContent === item.content.trim();

    return (
      <div className="flex justify-end">
        <div className="w-full max-w-[640px] rounded-lg bg-muted/60 p-3 text-foreground">
          <div ref={editInputGroupRef}>
            <ChatMentionMenuPortal
              activeIndex={mentionActiveIndex}
              menuID={mentionMenuID}
              menuLayout={mentionMenuLayout}
              menuRef={mentionMenuRef}
              menuReady={mentionMenuReady}
              open={showMentionMenu}
              sectionOffsets={mentionSectionOffsets}
              sections={mentionSections}
              t={tComposer}
              onSelect={selectMentionItem}
            />
            <Textarea
              ref={editTextareaRef}
              autoFocus
              value={editingValue}
              className="chat-font-content min-h-[120px] resize-none rounded-lg border-border border-[0.5px] bg-background px-3 py-2 text-sm leading-7 shadow-none focus-visible:border-primary focus-visible:ring-0"
              style={{ fontFamily: "var(--font-chat)", fontWeight: "var(--font-chat-weight)" }}
              aria-controls={showMentionMenu ? mentionMenuID : undefined}
              aria-expanded={showMentionMenu}
              onBlur={handleMentionBlur}
              onChange={(event) => handleMentionChange(event.target.value)}
              onFocus={handleMentionFocus}
              onKeyDown={(event) => {
                if (handleMentionKeyDown(event)) {
                  return;
                }
              }}
            />
          </div>
          <div className="flex items-center justify-between gap-4">
            <div className="flex gap-2 pt-2 text-xs text-muted-foreground">
              <CircleAlert className="mt-0.5 size-3 shrink-0" />
              <span>{tMessages("editCreatesBranch")}</span>
            </div>
            <div className="mt-3 flex items-center justify-center gap-2">
              <Button
                variant="ghost"
                className="rounded-lg text-xs font-medium"
                onClick={() => setIsEditing(false)}
              >
                {tCommon("cancel")}
              </Button>
              <Button
                variant="default"
                className="rounded-lg text-xs font-medium shadow-none hover:bg-primary/60"
                disabled={busy || nextContent.length === 0 || unchanged}
                onClick={() => void onEditSave()}
              >
                {tCommon("save")}
              </Button>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="group/user-message flex min-w-0 max-w-full flex-col items-end gap-2">
      {item.attachments && item.attachments.length > 0 ? (
        <MessageAttachmentRow
          attachments={item.attachments}
          loadContent={attachmentContentLoader}
          allowDownload={!readOnly}
        />
      ) : null}
      <div
        className="chat-font-content min-w-0 max-w-[70%] overflow-hidden rounded-xl bg-muted/60 p-3 text-[15px] leading-8 text-foreground [overflow-wrap:anywhere] max-sm:max-w-[88%]"
        style={{ fontFamily: "var(--font-chat)", fontWeight: "var(--font-chat-weight)" }}
      >
        {item.content.trim() ? (
          <>
            <div className="relative">
              <motion.div
                ref={contentRef}
                className="overflow-hidden"
                initial={false}
                animate={measured && canCollapse ? { maxHeight: contentMaxHeight } : undefined}
                transition={USER_MESSAGE_EXPAND_TRANSITION}
                style={contentMaxHeight == null ? { maxHeight: "none" } : { maxHeight: contentMaxHeight }}
              >
                <p className="whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{item.content}</p>
              </motion.div>
            </div>
            {measured && canCollapse ? (
              <button
                type="button"
                className="mt-1 inline-flex items-center gap-1 rounded-md p-0 text-[15px] font-medium leading-8 text-foreground/80 transition-colors hover:text-foreground"
                aria-expanded={expanded}
                onClick={() =>
                  setExpandedContentKey((current) => (current === measurementKey ? "" : measurementKey))
                }
                onMouseEnter={() => setIsToggleHovered(true)}
                onMouseLeave={() => setIsToggleHovered(false)}
              >
                {expanded ? (
                  <ChevronUp className="size-4 shrink-0" animate={isToggleHovered ? "default" : undefined} />
                ) : (
                  <ChevronDown className="size-4 shrink-0" animate={isToggleHovered ? "default" : undefined} />
                )}
                <span>{expanded ? tMessages("collapseUserMessage") : tMessages("expandUserMessage")}</span>
              </button>
            ) : null}
          </>
        ) : null}
      </div>
      <UserMessageMeta
        item={item}
        busy={busy}
        showRetry={!busy && !item.isPending}
        onCycleBranch={onCycleMessageBranch}
        onRetry={onRetry}
        onEdit={() => setIsEditing(true)}
        onCopy={onCopy}
        copySucceeded={copySucceeded}
        readOnly={readOnly}
        alwaysVisible={readOnly}
        showBranchNavigator={showBranchNavigator}
      />
    </div>
  );
}
