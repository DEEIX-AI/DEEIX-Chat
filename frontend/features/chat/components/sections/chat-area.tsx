"use client";

import * as React from "react";
import { motion } from "motion/react";
import { ArrowDownToLine } from "lucide-react";
import { useTranslations } from "next-intl";

import { ChatLabel } from "@/features/chat/components/sections/chat-label";
import { useMessageFeedback } from "@/features/chat/hooks/use-message-feedback";
import {
  AssistantMessageSkeleton,
  ChatInlineAlertCard,
  ChatMessageBot,
} from "@/features/chat/components/message/message-bot";
import { areChatAreaMessagesRenderEqual } from "@/features/chat/model/chat-message-render";
import { type AssistantReaction } from "@/features/chat/components/message/message-meta";
import type { ChatAreaMessage, MessageAttachment } from "@/features/chat/types/messages";
import { ChatMessageUser } from "@/features/chat/components/message/message-user";
import { StreamdownRender } from "@/features/chat/components/markdown/streamdown-render";
import type { OpenCodeArtifactInput } from "@/features/chat/model/chat-artifacts";
import { CenteredEmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { ConversationShareExportIconDropdown } from "@/shared/components/conversation-share-export-menu";
import { useCopyAction } from "@/shared/components/copy-action";
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
  onEditImageAttachment?: (attachment: MessageAttachment, sourceModelName?: string) => void;
  onOpenCodeArtifact?: (message: ChatAreaMessage, artifact: OpenCodeArtifactInput) => void;
  onCycleMessageBranch: (parentPublicID: string | null, direction: "previous" | "next") => void;
  onToggleStar?: () => void | Promise<void>;
  onRename?: (title: string) => void | Promise<void>;
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
  splitRightInset?: boolean;
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
  onEditImageAttachment,
  onCycleMessageBranch,
  onReactAssistantMessage,
  onOpenCodeArtifact,
  markdownRender,
  showModelInfo,
  showLatency,
  showTokenUsage,
  showBillingCost,
}: {
  item: ChatAreaMessage;
  busy: boolean;
  reaction: AssistantReaction;
  onRetryUserMessage: (message: ChatAreaMessage) => Promise<void> | void;
  onRetryAssistantMessage: (message: ChatAreaMessage) => Promise<void> | void;
  onContinueAssistantMessage?: (message: ChatAreaMessage) => Promise<void> | void;
  onEditAssistantMessage: (message: ChatAreaMessage, content: string) => Promise<boolean> | boolean;
  onEditUserMessage: (message: ChatAreaMessage, content: string) => Promise<boolean> | boolean;
  onEditImageAttachment?: (attachment: MessageAttachment, sourceModelName?: string) => void;
  onCycleMessageBranch: (parentPublicID: string | null, direction: "previous" | "next") => void;
  onReactAssistantMessage: (publicID: string, reaction: AssistantReaction) => void;
  onOpenCodeArtifact?: (message: ChatAreaMessage, artifact: OpenCodeArtifactInput) => void;
  markdownRender: boolean;
  showModelInfo: boolean;
  showLatency: boolean;
  showTokenUsage: boolean;
  showBillingCost: boolean;
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
  onEditImageAttachment,
  onOpenCodeArtifact,
  onCycleMessageBranch,
  onToggleStar,
  onRename,
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
  splitRightInset = false,
}: ChatAreaProps) {
  const t = useTranslations("chat");
  const { getReaction, onReactAssistantMessage } = useMessageFeedback(messages);
  const stableOnRetryUserMessage = useStableEvent(onRetryUserMessage);
  const stableOnRetryAssistantMessage = useStableEvent(onRetryAssistantMessage);
  const stableOnContinueAssistantMessage = useStableEvent(onContinueAssistantMessage ?? (() => undefined));
  const stableOnEditAssistantMessage = useStableEvent(onEditAssistantMessage);
  const stableOnEditUserMessage = useStableEvent(onEditUserMessage);
  const stableOnEditImageAttachment = useStableEvent((attachment: MessageAttachment, sourceModelName?: string) => {
    onEditImageAttachment?.(attachment, sourceModelName);
  });
  const stableOnCycleMessageBranch = useStableEvent(onCycleMessageBranch);
  const stableOnReactAssistantMessage = useStableEvent(onReactAssistantMessage);
  const editImageAttachmentHandler = onEditImageAttachment ? stableOnEditImageAttachment : undefined;
  const shareLabel = shareActive ? t("manageShare") : t("shareConversation");
  const shareExportLabel = t("labelMenu.shareAndExport");

  return (
    <>
      <div className={cn("px-3 py-2.5 md:pl-0", splitRightInset ? "md:pr-4" : "md:pr-0")}>
        <div className="flex w-full items-center justify-between gap-3">
          <ChatLabel
            title={title}
            starred={starred}
            onToggleStar={canOperateConversation ? onToggleStar : undefined}
            onRename={canOperateConversation ? onRename : undefined}
            projectMenu={canOperateConversation ? projectMenu : undefined}
            onShare={canOperateConversation ? onShare : undefined}
            shareActive={shareActive}
            onExport={canOperateConversation ? onExport : undefined}
            onDelete={canOperateConversation ? onDelete : undefined}
          />
          {canOperateConversation ? (
            <ConversationShareExportIconDropdown
              label={shareExportLabel}
              shareLabel={shareLabel}
              exportLabel={t("labelMenu.exportJSON")}
              active={shareActive}
              onShare={onShare}
              onExport={onExport}
            />
          ) : null}
        </div>
      </div>

      <div className="relative min-h-0 flex-1 overflow-hidden">
        <div
          ref={messageViewportRef}
          className="h-full min-h-0 overflow-y-auto px-3 pb-8 pt-2 [overflow-anchor:none] md:px-6"
          onScroll={onScroll}
        >
          <div
            ref={messageContentRef}
            className="mx-auto w-full max-w-[760px]"
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
                  onEditImageAttachment={editImageAttachmentHandler}
                  onCycleMessageBranch={stableOnCycleMessageBranch}
                  onReactAssistantMessage={stableOnReactAssistantMessage}
                  onOpenCodeArtifact={onOpenCodeArtifact}
                  markdownRender={markdownRender}
                  showModelInfo={showModelInfo}
                  showLatency={showLatency}
                  showTokenUsage={showTokenUsage}
                  showBillingCost={showBillingCost}
                />
              );

              const compactDivider = item.compactDone ? (
                <CompactDivider summaryPreview={item.compactDone.summary_preview} />
              ) : null;

              if (!shouldAnimateLayout) {
                return (
                  <div key={item.key} className={spacingClass}>
                    {compactDivider}
                    {row}
                  </div>
                );
              }

              return (
                <motion.div
                  key={item.key}
                  layout="position"
                  className={spacingClass}
                  transition={MESSAGE_SWITCH_TRANSITION}
                  style={{ willChange: "transform" }}
                >
                  {compactDivider}
                  {row}
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
        <div className="mx-auto w-full max-w-[760px] space-y-6">
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
