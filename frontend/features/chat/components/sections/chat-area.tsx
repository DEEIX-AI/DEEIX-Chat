"use client";

import * as React from "react";
import { motion } from "motion/react";
import { ArrowDownToLine } from "lucide-react";
import { useTranslations } from "next-intl";

import { ChatLabel } from "@/features/chat/components/sections/chat-label";
import { useChatMessageFeedback } from "@/features/chat/hooks/use-chat-message-feedback";
import {
  AssistantMessageSkeleton,
  ChatInlineAlertCard,
  ChatMessageBot,
} from "@/features/chat/components/message/message-bot";
import { areChatAreaMessagesRenderEqual } from "@/features/chat/model/chat-message-render";
import { type AssistantReaction } from "@/features/chat/components/message/message-meta";
import type { ChatAreaMessage, MessageAttachment } from "@/features/chat/types/messages";
import { ChatMessageUser } from "@/features/chat/components/message/message-user";
import { StreamdownRender } from "@/shared/components/markdown/streamdown-render";
import type { OpenCodeArtifactInput } from "@/features/chat/model/chat-artifacts";
import { CenteredEmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ConversationShareExportIconDropdown } from "@/shared/components/conversation-share-export-menu";
import { ChatScreenshotSelectionBar } from "@/features/chat/components/sections/chat-screenshot-selection-bar";
import { Checkbox } from "@/components/ui/checkbox";
import { useCopyAction } from "@/shared/components/copy-action";
import type { ChatModelOption } from "@/features/chat/types/chat-runtime";
import type { BillingDisplayCurrency } from "@/shared/lib/billing-display";
import { cn } from "@/lib/utils";

function CompactDivider({ summaryPreview }: { summaryPreview: string }) {
  const t = useTranslations("chat.messages");
  const [expanded, setExpanded] = React.useState(false);
  return (
    <div className="my-4 flex flex-col items-center gap-1">
      <div className="flex w-full items-center gap-3">
        <div className="h-px flex-1 bg-border/50" />
        <button
          type="button"
          className="shrink-0 cursor-pointer text-[11px] text-muted-foreground/60 hover:text-muted-foreground"
          onClick={() => setExpanded((v) => !v)}
        >
          {t("contextCompressed")}
        </button>
        <div className="h-px flex-1 bg-border/50" />
      </div>
      {expanded && summaryPreview ? (
        <p className="max-w-lg text-center text-[11px] leading-relaxed text-muted-foreground/70">
          {summaryPreview}
        </p>
      ) : null}
    </div>
  );
}

const MESSAGE_SWITCH_TRANSITION = {
  layout: {
    duration: 0.22,
    ease: [0.16, 1, 0.3, 1] as const,
  },
  opacity: {
    duration: 0.16,
    ease: "easeOut" as const,
  },
};

type ChatAreaProps = {
  title: string;
  starred: boolean;
  canOperateConversation: boolean;
  messages: ChatAreaMessage[];
  busy: boolean;
  messageViewportRef: React.RefObject<HTMLDivElement | null>;
  messageContentRef: React.RefObject<HTMLDivElement | null>;
  messageEndRef: React.RefObject<HTMLDivElement | null>;
  onScroll: () => void;
  onScrollToLatest: () => void;
  showScrollToLatestButton: boolean;
  onRetryUserMessage: (message: ChatAreaMessage) => Promise<void> | void;
  onRetryAssistantMessage: (message: ChatAreaMessage) => Promise<void> | void;
  onContinueAssistantMessage?: (message: ChatAreaMessage) => Promise<void> | void;
  onEditAssistantMessage: (message: ChatAreaMessage, content: string) => Promise<boolean> | boolean;
  onEditUserMessage: (message: ChatAreaMessage, content: string) => Promise<boolean> | boolean;
  modelOptions: ChatModelOption[];
  selectedPlatformModelName: string;
  onModelChange: (platformModelName: string) => void;
  onModelCatalogRefresh?: () => void | Promise<void>;
  onEditImageAttachment?: (attachment: MessageAttachment, sourceModelName?: string) => void;
  onOpenCodeArtifact?: (message: ChatAreaMessage, artifact: OpenCodeArtifactInput) => void;
  onCycleMessageBranch: (parentPublicID: string | null, direction: "previous" | "next") => void;
  onToggleStar?: () => void | Promise<void>;
  onRename?: (title: string) => void | Promise<void>;
  onAutoRename?: () => void | Promise<void>;
  projectMenu?: React.ComponentProps<typeof ChatLabel>["projectMenu"];
  onShare?: () => void;
  shareActive?: boolean;
  onExport?: () => void | Promise<void>;
  onDelete?: () => void | Promise<void>;
  markdownRender?: boolean;
  showModelInfo?: boolean;
  showLatency?: boolean;
  showTokenUsage?: boolean;
  showBillingCost?: boolean;
  billingDisplayCurrency?: BillingDisplayCurrency;
  billingDisplayUsdToCnyRate?: number | null;
  splitRightInset?: boolean;
  contentWidthClassName?: string;
  onScreenshotFull?: () => void;
  onScreenshotSelect?: () => void;
  screenshot?: {
    selectionMode: boolean;
    selectedIDs: Set<string>;
    selectedCount: number;
    capturing: boolean;
    onToggleSelection: (publicID: string) => void;
    onSelectAll: (publicIDs: string[]) => void;
    onClearSelection: () => void;
    onCapture: () => void;
    onExit: () => void;
  };
};

function useStableEvent<Args extends unknown[], Return>(callback: (...args: Args) => Return) {
  const callbackRef = React.useRef(callback);
  React.useLayoutEffect(() => {
    callbackRef.current = callback;
  }, [callback]);

  return React.useCallback((...args: Args) => callbackRef.current(...args), []);
}

const ChatMessageRow = React.memo(function ChatMessageRow({
  item,
  busy,
  reaction,
  onRetryUserMessage,
  onRetryAssistantMessage,
  onContinueAssistantMessage,
  onEditAssistantMessage,
  onEditUserMessage,
  modelOptions,
  selectedPlatformModelName,
  onModelChange,
  onModelCatalogRefresh,
  onEditImageAttachment,
  onCycleMessageBranch,
  onReactAssistantMessage,
  onOpenCodeArtifact,
  markdownRender,
  showModelInfo,
  showLatency,
  showTokenUsage,
  showBillingCost,
  billingDisplayCurrency,
  billingDisplayUsdToCnyRate,
  contentWidthClassName,
}: {
  item: ChatAreaMessage;
  busy: boolean;
  reaction: AssistantReaction;
  onRetryUserMessage: (message: ChatAreaMessage) => Promise<void> | void;
  onRetryAssistantMessage: (message: ChatAreaMessage) => Promise<void> | void;
  onContinueAssistantMessage?: (message: ChatAreaMessage) => Promise<void> | void;
  onEditAssistantMessage: (message: ChatAreaMessage, content: string) => Promise<boolean> | boolean;
  onEditUserMessage: (message: ChatAreaMessage, content: string) => Promise<boolean> | boolean;
  modelOptions: ChatModelOption[];
  selectedPlatformModelName: string;
  onModelChange: (platformModelName: string) => void;
  onModelCatalogRefresh?: () => void | Promise<void>;
  onEditImageAttachment?: (attachment: MessageAttachment, sourceModelName?: string) => void;
  onCycleMessageBranch: (parentPublicID: string | null, direction: "previous" | "next") => void;
  onReactAssistantMessage: (publicID: string, reaction: AssistantReaction) => void;
  onOpenCodeArtifact?: (message: ChatAreaMessage, artifact: OpenCodeArtifactInput) => void;
  markdownRender: boolean;
  showModelInfo: boolean;
  showLatency: boolean;
  showTokenUsage: boolean;
  showBillingCost: boolean;
  billingDisplayCurrency: BillingDisplayCurrency;
  billingDisplayUsdToCnyRate: number | null;
  contentWidthClassName: string;
}) {
  const t = useTranslations("chat.messages");
  const { copy, isCopied } = useCopyAction({
    messages: {
      copied: t("copied"),
      failed: t("copyFailed"),
      failedDescription: t("copyFailedDescription"),
    },
  });
  const isUser = item.role === "user";
  const isAssistant = item.role === "assistant";
  const artifactActions = React.useMemo(
    () =>
      isAssistant && onOpenCodeArtifact
        ? {
            onOpenCodeArtifact: (artifact: OpenCodeArtifactInput) => onOpenCodeArtifact(item, artifact),
          }
        : undefined,
    [isAssistant, item, onOpenCodeArtifact],
  );

  const copyKey = item.publicID || item.key;
  const onCopy = React.useCallback(async () => {
    await copy(item.content, { key: copyKey });
  }, [copy, copyKey, item.content]);

  if (isUser) {
    return (
      <ChatMessageUser
        item={item}
        busy={busy}
        onRetryUserMessage={onRetryUserMessage}
        onEditUserMessage={onEditUserMessage}
        modelOptions={modelOptions}
        selectedPlatformModelName={selectedPlatformModelName}
        onModelChange={onModelChange}
        onModelCatalogRefresh={onModelCatalogRefresh}
        onCycleMessageBranch={onCycleMessageBranch}
        onCopy={() => void onCopy()}
        copySucceeded={isCopied(copyKey)}
      />
    );
  }

  if (isAssistant) {
    return (
      <ChatMessageBot
        item={item}
        busy={busy}
        reaction={reaction}
        onRetryAssistantMessage={onRetryAssistantMessage}
        onContinueAssistantMessage={onContinueAssistantMessage}
        onEditAssistantMessage={onEditAssistantMessage}
        onCycleMessageBranch={onCycleMessageBranch}
        onReactAssistantMessage={onReactAssistantMessage}
        onCopy={() => void onCopy()}
        copySucceeded={isCopied(copyKey)}
        onEditImageAttachment={onEditImageAttachment}
        artifactActions={artifactActions}
        markdownRender={markdownRender}
        showModelInfo={showModelInfo}
        showLatency={showLatency}
        showTokenUsage={showTokenUsage}
        showBillingCost={showBillingCost}
        billingDisplayCurrency={billingDisplayCurrency}
        billingDisplayUsdToCnyRate={billingDisplayUsdToCnyRate}
        contentWidthClassName={contentWidthClassName}
      />
    );
  }

  return (
    <div className="min-w-0 max-w-none overflow-hidden text-sm leading-8 text-foreground [overflow-wrap:anywhere]">
      {item.content.trim() && markdownRender ? (
        <StreamdownRender content={item.content} streaming={Boolean(item.isStreaming)} />
      ) : item.content.trim() ? (
        <p className="whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{item.content}</p>
      ) : null}
      {item.inlineAlert ? (
        <ChatInlineAlertCard alert={item.inlineAlert} className={item.content.trim() ? "my-4" : "mb-4"} />
      ) : null}
    </div>
  );
}, (previous, next) => (
  previous.busy === next.busy &&
  previous.reaction === next.reaction &&
  previous.markdownRender === next.markdownRender &&
  previous.showModelInfo === next.showModelInfo &&
  previous.showLatency === next.showLatency &&
  previous.showTokenUsage === next.showTokenUsage &&
  previous.showBillingCost === next.showBillingCost &&
  previous.billingDisplayCurrency === next.billingDisplayCurrency &&
  previous.billingDisplayUsdToCnyRate === next.billingDisplayUsdToCnyRate &&
  previous.contentWidthClassName === next.contentWidthClassName &&
  previous.modelOptions === next.modelOptions &&
  previous.selectedPlatformModelName === next.selectedPlatformModelName &&
  previous.onModelChange === next.onModelChange &&
  previous.onModelCatalogRefresh === next.onModelCatalogRefresh &&
  previous.onEditImageAttachment === next.onEditImageAttachment &&
  previous.onOpenCodeArtifact === next.onOpenCodeArtifact &&
  areChatAreaMessagesRenderEqual(previous.item, next.item)
));

export function ChatArea({
  title,
  starred,
  canOperateConversation,
  messages,
  busy,
  messageViewportRef,
  messageContentRef,
  messageEndRef,
  onScroll,
  onScrollToLatest,
  showScrollToLatestButton,
  onRetryUserMessage,
  onRetryAssistantMessage,
  onContinueAssistantMessage,
  onEditAssistantMessage,
  onEditUserMessage,
  modelOptions,
  selectedPlatformModelName,
  onModelChange,
  onModelCatalogRefresh,
  onEditImageAttachment,
  onOpenCodeArtifact,
  onCycleMessageBranch,
  onToggleStar,
  onRename,
  onAutoRename,
  projectMenu,
  onShare,
  shareActive = false,
  onExport,
  onDelete,
  markdownRender = true,
  showModelInfo = true,
  showLatency = true,
  showTokenUsage = true,
  showBillingCost = false,
  billingDisplayCurrency = "USD",
  billingDisplayUsdToCnyRate = null,
  splitRightInset = false,
  contentWidthClassName = "max-w-[1080px]",
  onScreenshotFull,
  onScreenshotSelect,
  screenshot,
}: ChatAreaProps) {
  const t = useTranslations("chat");
  const { getReaction, onReactAssistantMessage } = useChatMessageFeedback(messages);
  const stableOnRetryUserMessage = useStableEvent(onRetryUserMessage);
  const stableOnRetryAssistantMessage = useStableEvent(onRetryAssistantMessage);
  const stableOnContinueAssistantMessage = useStableEvent(onContinueAssistantMessage ?? (() => undefined));
  const stableOnEditAssistantMessage = useStableEvent(onEditAssistantMessage);
  const stableOnEditUserMessage = useStableEvent(onEditUserMessage);
  const stableOnModelChange = useStableEvent(onModelChange);
  const stableOnModelCatalogRefresh = useStableEvent(onModelCatalogRefresh ?? (() => undefined));
  const stableOnEditImageAttachment = useStableEvent((attachment: MessageAttachment, sourceModelName?: string) => {
    onEditImageAttachment?.(attachment, sourceModelName);
  });
  const stableOnCycleMessageBranch = useStableEvent(onCycleMessageBranch);
  const stableOnReactAssistantMessage = useStableEvent(onReactAssistantMessage);
  const editImageAttachmentHandler = onEditImageAttachment ? stableOnEditImageAttachment : undefined;
  const shareLabel = shareActive ? t("manageShare") : t("shareConversation");
  const shareExportLabel = t("labelMenu.shareAndExport");
  const tScreenshot = useTranslations("chat.screenshot");
  const selectableMessagePublicIDs = React.useMemo(
    () =>
      messages
        .filter((item) => !item.isPending && Boolean(item.publicID?.trim()))
        .map((item) => item.publicID.trim()),
    [messages],
  );
  const selectionMode = screenshot?.selectionMode ?? false;
  const onSelectAllMessages = React.useCallback(() => {
    screenshot?.onSelectAll(selectableMessagePublicIDs);
  }, [screenshot, selectableMessagePublicIDs]);

  return (
    <>
      <div className={cn("px-3 py-2.5 md:pl-0", splitRightInset ? "md:pr-4" : "md:pr-0")}>
        <div className="flex w-full items-center justify-between gap-3">
          <ChatLabel
            title={title}
            starred={starred}
            onToggleStar={canOperateConversation ? onToggleStar : undefined}
            onRename={canOperateConversation ? onRename : undefined}
            onAutoRename={canOperateConversation ? onAutoRename : undefined}
            projectMenu={canOperateConversation ? projectMenu : undefined}
            onShare={canOperateConversation ? onShare : undefined}
            shareActive={shareActive}
            onExport={canOperateConversation ? onExport : undefined}
            onDelete={canOperateConversation ? onDelete : undefined}
            screenshotFullLabel={tScreenshot("captureFull")}
            screenshotSelectLabel={tScreenshot("captureSelect")}
            onScreenshotFull={onScreenshotFull}
            onScreenshotSelect={onScreenshotSelect}
          />
          {canOperateConversation ? (
            <ConversationShareExportIconDropdown
              label={shareExportLabel}
              shareLabel={shareLabel}
              exportLabel={t("labelMenu.exportJSON")}
              active={shareActive}
              onShare={onShare}
              onExport={onExport}
              screenshotFullLabel={tScreenshot("captureFull")}
              screenshotSelectLabel={tScreenshot("captureSelect")}
              onScreenshotFull={onScreenshotFull}
              onScreenshotSelect={onScreenshotSelect}
            />
          ) : null}
        </div>
      </div>

      {selectionMode && screenshot ? (
        <div className={cn("px-3 pb-1 md:px-6")} data-screenshot-exclude="true">
          <div className={cn("mx-auto w-full", contentWidthClassName)}>
            <ChatScreenshotSelectionBar
              selectedCount={screenshot.selectedCount}
              totalCount={selectableMessagePublicIDs.length}
              capturing={screenshot.capturing}
              onSelectAll={onSelectAllMessages}
              onClearSelection={screenshot.onClearSelection}
              onCapture={screenshot.onCapture}
              onExit={screenshot.onExit}
            />
          </div>
        </div>
      ) : null}

      <div className="relative min-h-0 flex-1 overflow-hidden">
        <div
          ref={messageViewportRef}
          className="h-full min-h-0 overflow-y-auto px-3 pb-8 pt-2 [overflow-anchor:none] md:px-6"
          onScroll={onScroll}
        >
          <div
            ref={messageContentRef}
            className={cn("mx-auto w-full", contentWidthClassName)}
            style={{ fontFamily: "var(--font-chat)", fontWeight: "var(--font-chat-weight)" }}
          >
            {messages.map((item, index) => {
              const previousItem = index > 0 ? messages[index - 1] : null;
              const spacingClass =
                !previousItem
                  ? ""
                  : previousItem.role === "assistant" && item.role === "user"
                    ? "mt-6 md:mt-12"
                    : "mt-4";
              const shouldAnimateLayout = !item.isPending && !item.isStreaming;
              const publicID = item.publicID?.trim() ?? "";
              const selectable = selectionMode && Boolean(publicID) && !item.isPending;
              const isSelected = selectable && (screenshot?.selectedIDs.has(publicID) ?? false);

              const row = (
                <ChatMessageRow
                  item={item}
                  busy={busy}
                  reaction={getReaction(item)}
                  onRetryUserMessage={stableOnRetryUserMessage}
                  onRetryAssistantMessage={stableOnRetryAssistantMessage}
                  onContinueAssistantMessage={onContinueAssistantMessage ? stableOnContinueAssistantMessage : undefined}
                  onEditAssistantMessage={stableOnEditAssistantMessage}
                  onEditUserMessage={stableOnEditUserMessage}
                  modelOptions={modelOptions}
                  selectedPlatformModelName={selectedPlatformModelName}
                  onModelChange={stableOnModelChange}
                  onModelCatalogRefresh={onModelCatalogRefresh ? stableOnModelCatalogRefresh : undefined}
                  onEditImageAttachment={editImageAttachmentHandler}
                  onCycleMessageBranch={stableOnCycleMessageBranch}
                  onReactAssistantMessage={stableOnReactAssistantMessage}
                  onOpenCodeArtifact={onOpenCodeArtifact}
                  markdownRender={markdownRender}
                  showModelInfo={showModelInfo}
                  showLatency={showLatency}
                  showTokenUsage={showTokenUsage}
                  showBillingCost={showBillingCost}
                  billingDisplayCurrency={billingDisplayCurrency}
                  billingDisplayUsdToCnyRate={billingDisplayUsdToCnyRate}
                  contentWidthClassName={contentWidthClassName}
                />
              );

              const rowContent = selectable ? (
                <div
                  data-screenshot-selectable="true"
                  className="chat-screenshot-selectable group relative rounded-xl"
                >
                  <div
                    className={cn(
                      "pointer-events-none absolute inset-0 z-0 rounded-xl transition-colors",
                      isSelected ? "bg-primary/5 ring-1 ring-primary/40" : "group-hover:bg-muted/40",
                    )}
                    data-screenshot-exclude="true"
                    aria-hidden="true"
                  />
                  <div
                    className="absolute left-1 top-2 z-10 flex items-center"
                    data-screenshot-exclude="true"
                  >
                    <Checkbox
                      checked={isSelected}
                      onCheckedChange={() => screenshot?.onToggleSelection(publicID)}
                      aria-label={tScreenshot("selectMessage")}
                    />
                  </div>
                  <div className="chat-screenshot-selectable-content pointer-events-none relative z-[1] pl-7" inert>
                    {row}
                  </div>
                </div>
              ) : (
                row
              );

              const compactDivider = item.compactDone ? (
                <CompactDivider summaryPreview={item.compactDone.summary_preview} />
              ) : null;

              if (!shouldAnimateLayout) {
                return (
                  <div key={item.key} className={spacingClass} data-message-public-id={publicID || undefined}>
                    {compactDivider}
                    {rowContent}
                  </div>
                );
              }

              return (
                <motion.div
                  key={item.key}
                  layout="position"
                  className={spacingClass}
                  data-message-public-id={publicID || undefined}
                  transition={MESSAGE_SWITCH_TRANSITION}
                  style={{ willChange: "transform" }}
                >
                  {compactDivider}
                  {rowContent}
                </motion.div>
              );
            })}
            <div ref={messageEndRef} aria-hidden="true" className="h-px" />
          </div>
        </div>

        {showScrollToLatestButton ? (
          <button
            type="button"
            className="absolute bottom-4 left-1/2 z-20 inline-flex size-8 -translate-x-1/2 items-center justify-center rounded-full border border-border/70 bg-background text-muted-foreground shadow-md transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
            aria-label={t("messages.scrollToBottom")}
            title={t("messages.scrollToBottom")}
            onClick={onScrollToLatest}
          >
            <ArrowDownToLine className="size-4" strokeWidth={1.8} />
          </button>
        ) : null}
      </div>
    </>
  );
}

export function ChatAreaSkeleton() {
  return (
    <div aria-hidden="true" className="flex h-full min-h-0 flex-col">
      <div className="shrink-0 px-3 py-2.5 md:px-0">
        <div className="flex w-full items-center justify-between gap-3">
          <div className="inline-flex h-7 max-w-full items-center gap-1 rounded-lg">
            <Skeleton className="h-4 w-32 rounded-full bg-muted/35" />
            <Skeleton className="size-4 rounded-md bg-muted/35" />
          </div>
          <Skeleton className="size-8 shrink-0 rounded-full bg-muted/35" />
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-hidden px-3 pb-8 pt-2 md:px-6">
        <div className="mx-auto w-full max-w-[1080px] space-y-6">
          <ChatUserMessageSkeleton widthClassName="w-[min(26rem,70%)] max-sm:w-[88%]" />

          <ChatAssistantMessageSkeleton />

          <ChatUserMessageSkeleton widthClassName="w-[min(18rem,64%)] max-sm:w-[78%]" />
        </div>
      </div>
    </div>
  );
}

function ChatUserMessageSkeleton({ widthClassName }: { widthClassName: string }) {
  return (
    <div className="flex justify-end">
      <Skeleton className={cn("h-[54px] rounded-xl bg-muted/40", widthClassName)} />
    </div>
  );
}

function ChatAssistantMessageSkeleton() {
  return (
    <div className="flex w-full flex-col items-start gap-1.5">
      <AssistantMessageSkeleton />
      <div className="flex max-w-full flex-col items-start gap-1 md:flex-row md:items-center">
        <div className="flex items-center gap-1">
          <Skeleton className="size-6 rounded-md bg-muted/35" />
          <Skeleton className="size-6 rounded-md bg-muted/35" />
          <Skeleton className="size-6 rounded-md bg-muted/35" />
          <Skeleton className="size-6 rounded-md bg-muted/35" />
        </div>
        <div className="flex min-w-0 max-w-full flex-wrap items-center gap-1">
          <Skeleton className="h-5 w-24 rounded bg-muted/35" />
          <Skeleton className="h-5 w-36 rounded bg-muted/35" />
          <Skeleton className="h-5 w-14 rounded bg-muted/35" />
        </div>
      </div>
    </div>
  );
}

export function ChatAreaLoadError({
  onRefresh,
  onNewConversation,
}: {
  onRefresh: () => void | Promise<void>;
  onNewConversation: () => void;
}) {
  const t = useTranslations("chat.loadError");
  return (
    <CenteredEmptyState
      className="flex-1 pb-20"
      title={t("title")}
      description={
        <>
          {t("prefix")}{" "}
          <button
            type="button"
            className="rounded-sm font-medium text-foreground underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            onClick={() => void onRefresh()}
          >
            {t("refresh")}
          </button>{" "}
          {t("or")}{" "}
          <button
            type="button"
            className="rounded-sm font-medium text-foreground underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            onClick={onNewConversation}
          >
            {t("newChat")}
          </button>
        </>
      }
    />
  );
}
