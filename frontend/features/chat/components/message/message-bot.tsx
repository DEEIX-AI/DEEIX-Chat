"use client";

import * as React from "react";
import { ChevronDown, CircleAlert } from "lucide-react";
import { useTranslations } from "next-intl";

import { AssistantMessageMeta } from "@/features/chat/components/message/message-meta";
import { MessageAttachmentRow } from "@/features/chat/components/message/message-attachment";
import { MessageProcessTrace, MessageTraceEventBlocks, MessageToolTrace, MessageUpstreamThink } from "@/features/chat/components/message/message-process-trace";
import { GrainientBackground } from "@/components/reactbits/backgrounds/grainient";
import type { AssistantReaction } from "@/features/chat/components/message/message-meta";
import type {
  ChatAreaMessage,
  ChatInlineAlert,
  MessageAttachment,
} from "@/features/chat/types/messages";
import { MarkdownImage, type MarkdownArtifactActions } from "@/shared/components/markdown/streamdown-components";
import { StreamdownRender } from "@/shared/components/markdown/streamdown-render";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
} from "@/components/ui/accordion";
import {
  Alert,
  AlertDescription,
} from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { isUpstreamStreamingDebugBody, summarizeUpstreamError } from "@/features/chat/utils/chat-runtime";
import type { FileContentResult } from "@/shared/api/file";
import type { PreviewDialogFile } from "@/shared/components/file-preview/file-preview-dialog";
import { resolveLeadingImagePreview } from "@/features/chat/model/media-image-preview";

const EMPTY_TRACE_EVENTS: NonNullable<ChatAreaMessage["processTrace"]>["events"] = [];

function isEditableImageAttachment(attachment: MessageAttachment): boolean {
  const mimeType = attachment.mimeType.toLowerCase();
  const detectedMime = attachment.detectedMime?.toLowerCase() || "";
  return (
    attachment.kind === "image" ||
    attachment.fileCategory === "image" ||
    mimeType.startsWith("image/") ||
    detectedMime.startsWith("image/")
  );
}

function resolveFileIDFromImageSrc(src: string): string | null {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    const url = new URL(src, window.location.origin);
    const match = url.pathname.match(/\/api\/v1\/files\/([^/]+)\/content$/);
    return match?.[1] ? decodeURIComponent(match[1]) : null;
  } catch {
    return null;
  }
}

function resolveEditableImageAttachment(
  src: string,
  attachments: MessageAttachment[],
  contentType: string | undefined,
): MessageAttachment | null {
  if (attachments.length === 0) {
    return null;
  }

  const fileID = resolveFileIDFromImageSrc(src);
  if (fileID) {
    return attachments.find((attachment) => attachment.fileID === fileID) ?? null;
  }

  if (contentType === "image" && attachments.length === 1) {
    return attachments[0];
  }

  return null;
}

type ChatMessageBotProps = {
  item: ChatAreaMessage;
  busy: boolean;
  reaction: AssistantReaction;
  onRetryAssistantMessage: (message: ChatAreaMessage) => Promise<void> | void;
  onContinueAssistantMessage?: (message: ChatAreaMessage) => Promise<void> | void;
  onEditAssistantMessage: (message: ChatAreaMessage, content: string) => Promise<boolean> | boolean;
  onCycleMessageBranch: (parentPublicID: string | null, direction: "previous" | "next") => void;
  onReactAssistantMessage: (publicID: string, reaction: AssistantReaction) => void;
  onCopy: () => void;
  copySucceeded?: boolean;
  markdownRender?: boolean;
  showModelInfo?: boolean;
  showLatency?: boolean;
  showTokenUsage?: boolean;
  showBillingCost?: boolean;
  readOnly?: boolean;
  attachmentContentLoader?: (file: PreviewDialogFile) => Promise<FileContentResult>;
  onEditImageAttachment?: (attachment: MessageAttachment, sourceModelName?: string) => void;
  artifactActions?: MarkdownArtifactActions;
  showBranchNavigator?: boolean;
  contentWidthClassName?: string;
};

export function ChatMessageBot({
  item,
  busy,
  reaction,
  onRetryAssistantMessage,
  onContinueAssistantMessage,
  onEditAssistantMessage,
  onCycleMessageBranch,
  onReactAssistantMessage,
  onCopy,
  copySucceeded = false,
  markdownRender = true,
  showModelInfo = true,
  showLatency = true,
  showTokenUsage = true,
  showBillingCost = false,
  readOnly = false,
  attachmentContentLoader,
  onEditImageAttachment,
  artifactActions,
  showBranchNavigator = true,
  contentWidthClassName = "max-w-[1080px]",
}: ChatMessageBotProps) {
  const tCommon = useTranslations("common.actions");
  const submitT = useTranslations("chat.submit");
  const [isEditing, setIsEditing] = React.useState(false);
  const [editingValue, setEditingValue] = React.useState(item.content);
  const onRetry = React.useCallback(() => {
    void onRetryAssistantMessage(item);
  }, [item, onRetryAssistantMessage]);
  const onContinue = React.useCallback(() => {
    void onContinueAssistantMessage?.(item);
  }, [item, onContinueAssistantMessage]);
  const onEditSave = React.useCallback(async () => {
    const nextContent = editingValue.trim();
    if (!nextContent || nextContent === item.content.trim()) {
      return;
    }
    const ok = await onEditAssistantMessage(item, nextContent);
    if (ok !== false) {
      setIsEditing(false);
    }
  }, [editingValue, item, onEditAssistantMessage]);
  React.useEffect(() => {
    setIsEditing(false);
  }, [item.publicID]);
  React.useEffect(() => {
    if (!isEditing) {
      setEditingValue(item.content);
    }
  }, [isEditing, item.content]);
  const upstreamThink = item.processTrace?.upstreamThink;
  const toolTrace = item.processTrace?.tools;
  const traceEvents = item.processTrace?.events ?? EMPTY_TRACE_EVENTS;
  const messageStreaming = Boolean(item.isStreaming);
  const upstreamThinkStreaming = messageStreaming && upstreamThink?.status === "streaming";
  const toolTraceStreaming = messageStreaming && toolTrace?.status === "streaming";
  const hasStreamdownContent = item.content.trim().length > 0;
  const leadingImagePreview = React.useMemo(() => resolveLeadingImagePreview(item.content), [item.content]);
  const leadingImageAlt = React.useMemo(
    () => leadingImagePreview?.alt || submitT("imagePreviewAlt"),
    [leadingImagePreview?.alt, submitT],
  );
  const leadingImageReady = Boolean(leadingImagePreview?.complete);
  const leadingImagePending = Boolean(leadingImagePreview && item.isStreaming && !leadingImagePreview.complete);
  const streamdownContent = leadingImagePreview?.rest ?? item.content;
  const hasInlineContent = streamdownContent.trim().length > 0;
  const postProcessEvents = React.useMemo(
    () =>
      traceEvents.filter(
        (event) =>
          event.phase === "tools" ||
          event.phase === "upstream_think" ||
          event.eventType === "tool" ||
          event.eventType === "think",
      ),
    [traceEvents],
  );
  const hasTraceEvents = postProcessEvents.length > 0;
  const isImageGenerationLoading = item.contentType === "image" && item.isStreaming && !hasStreamdownContent;
  const editableImageAttachments = React.useMemo(
    () => (item.attachments ?? []).filter(isEditableImageAttachment),
    [item.attachments],
  );
  const getEditableImageAttachment = React.useCallback(
    (src: string) => resolveEditableImageAttachment(src, editableImageAttachments, item.contentType),
    [editableImageAttachments, item.contentType],
  );
  const markdownImageActions = React.useMemo(() => {
    if (readOnly || !onEditImageAttachment || editableImageAttachments.length === 0) {
      return undefined;
    }
    return {
      canEditImage: (src: string) => Boolean(getEditableImageAttachment(src)),
      onEditImage: (src: string) => {
        const attachment = getEditableImageAttachment(src);
        if (attachment) {
          onEditImageAttachment(attachment, item.platformModelName);
        }
      },
    };
  }, [
    editableImageAttachments.length,
    getEditableImageAttachment,
    item.platformModelName,
    onEditImageAttachment,
    readOnly,
  ]);
  const activeThinkBlock = hasTraceEvents ? upstreamThink : undefined;
  const activeToolBlock = hasTraceEvents ? toolTrace : undefined;
  const processAutoCollapseReady = Boolean(hasTraceEvents || upstreamThink || toolTrace || hasStreamdownContent || item.inlineAlert);
  const toolAutoCollapseReady = Boolean(upstreamThink || hasStreamdownContent || item.inlineAlert);

  if (!readOnly && isEditing) {
    const nextContent = editingValue.trim();
    const unchanged = nextContent === item.content.trim();

    return (
      <div className="flex justify-start">
        <div className={cn("w-full rounded-lg bg-muted/40 p-3 text-foreground", contentWidthClassName)}>
          <Textarea
            autoFocus
            value={editingValue}
            className="chat-font-content min-h-[160px] resize-none rounded-lg border-border border-[0.5px] bg-background px-3 py-2 text-sm leading-7 shadow-none focus-visible:border-primary focus-visible:ring-0"
            style={{ fontFamily: "var(--font-chat)", fontWeight: "var(--font-chat-weight)" }}
            onChange={(event) => setEditingValue(event.target.value)}
          />
          <div className="mt-3 flex items-center justify-end gap-2">
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
    );
  }

  return (
    <div className="group/assistant-message flex w-full flex-col items-start">
      {hasTraceEvents ? (
        <>
          <MessageProcessTrace
            trace={item.processTrace}
            active={messageStreaming}
            autoCollapseReady={processAutoCollapseReady}
          />
          <MessageTraceEventBlocks
            events={postProcessEvents}
            activeToolBlock={activeToolBlock}
            activeThinkBlock={activeThinkBlock}
            messageStreaming={messageStreaming}
            autoCollapseReady={hasStreamdownContent || Boolean(item.inlineAlert)}
          />
        </>
      ) : (
        <>
          <MessageProcessTrace
            trace={item.processTrace}
            active={messageStreaming}
            autoCollapseReady={processAutoCollapseReady}
          />

          <MessageToolTrace block={toolTrace} streaming={toolTraceStreaming} autoCollapseReady={toolAutoCollapseReady} />

          <MessageUpstreamThink block={upstreamThink} streaming={upstreamThinkStreaming} />
        </>
      )}

      <div
        className="w-full min-w-0 max-w-none overflow-hidden text-[15px] leading-8 text-foreground [overflow-wrap:anywhere]"
        style={{ fontFamily: "var(--font-chat)", fontWeight: "var(--font-chat-weight)" }}
      >
        {isImageGenerationLoading && !item.inlineAlert ? (
          <AssistantImageGenerationSkeleton label={item.activityLabel} aspectRatio={item.imageAspectRatio} />
        ) : item.isStreaming && !hasStreamdownContent && !item.inlineAlert ? (
          <AssistantMessageSkeleton fileProc={item.isFileProc} label={item.activityLabel} />
        ) : leadingImagePending ? (
          <AssistantImageGenerationSkeleton label={leadingImageAlt} aspectRatio={item.imageAspectRatio} />
        ) : leadingImagePreview && leadingImageReady ? (
          <>
            <MarkdownImage alt={leadingImageAlt} src={leadingImagePreview.source} />
            {hasInlineContent && markdownRender ? (
              <StreamdownRender
                content={streamdownContent}
                streaming={Boolean(item.isStreaming)}
                imageActions={markdownImageActions}
                artifactActions={artifactActions}
              />
            ) : hasInlineContent ? (
              <p className="whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{streamdownContent}</p>
            ) : null}
          </>
        ) : hasStreamdownContent && markdownRender ? (
          <StreamdownRender
            content={streamdownContent}
            streaming={Boolean(item.isStreaming)}
            imageActions={markdownImageActions}
            artifactActions={artifactActions}
          />
        ) : hasStreamdownContent ? (
          <p className="whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{item.content}</p>
        ) : null}
      </div>

      {item.inlineAlert ? (
        <ChatInlineAlertCard alert={item.inlineAlert} className={hasStreamdownContent ? "my-4" : "mb-4"} />
      ) : null}

      {item.attachments && item.attachments.length > 0 ? (
        <div className="mt-2 flex w-full justify-start">
          <MessageAttachmentRow
            attachments={item.attachments}
            loadContent={attachmentContentLoader}
            allowDownload={!readOnly}
            align="start"
          />
        </div>
      ) : null}

      <AssistantMessageMeta
        item={item}
        busy={busy}
        reaction={reaction}
        onCycleBranch={onCycleMessageBranch}
        onRetry={onRetry}
        onContinue={onContinueAssistantMessage ? onContinue : undefined}
        onEdit={() => setIsEditing(true)}
        onCopy={onCopy}
        copySucceeded={copySucceeded}
        onReact={(value) => onReactAssistantMessage(item.publicID, value)}
        showModelInfo={showModelInfo}
        showLatency={showLatency}
        showTokenUsage={showTokenUsage}
        showBillingCost={showBillingCost}
        readOnly={readOnly}
        alwaysVisible={readOnly}
        showBranchNavigator={showBranchNavigator}
      />
    </div>
  );
}

export function ChatInlineAlertCard({
  alert,
  className,
}: {
  alert: ChatInlineAlert;
  className?: string;
}) {
  const t = useTranslations("chat.composer");
  const details = alert.details;
  const message = alert.message.trim();
  const summary = summarizeUpstreamError(message, details, t("retryLater"));
  const hasDetails = Boolean(details?.request || details?.response);
  const [detailsOpen, setDetailsOpen] = React.useState(false);
  const hasSuccessfulStreamDebug =
    Boolean(summary.statusCode && summary.statusCode >= 200 && summary.statusCode < 300) &&
    isUpstreamStreamingDebugBody(details?.response?.body || message);
  const summaryText = hasSuccessfulStreamDebug
    ? t("streamResponseParseFailed", { statusCode: summary.statusCode ?? 200 })
    : [summary.statusCode ? `HTTP ${summary.statusCode}` : "", summary.reason].filter(Boolean).join(", ");
  return (
    <Alert className={cn("min-w-0 max-w-full overflow-hidden", className)} variant="destructive">
      <CircleAlert className="size-4" />
      <button
        type="button"
        disabled={!hasDetails}
        aria-expanded={hasDetails ? detailsOpen : undefined}
        className={cn(
          "col-start-2 flex w-full min-w-0 max-w-full items-start gap-3 text-left",
          "rounded-sm outline-none transition-colors focus-visible:ring-[3px] focus-visible:ring-ring/35",
          hasDetails ? "cursor-pointer hover:text-destructive" : "cursor-default",
        )}
        onClick={() => {
          if (hasDetails) {
            setDetailsOpen((open) => !open);
          }
        }}
      >
        <span className="min-w-0 flex-1">
          <span className="block min-h-4 truncate font-medium tracking-tight">{alert.title}</span>
          <span className="mt-0.5 block whitespace-normal break-words text-sm leading-relaxed text-destructive/90 [overflow-wrap:anywhere]">
            {summaryText}
          </span>
        </span>
        {hasDetails ? (
          <ChevronDown className={cn("mt-0.5 size-4 shrink-0 text-destructive/70 transition-transform", detailsOpen && "rotate-180")} />
        ) : null}
      </button>
      {hasDetails ? (
        <AlertDescription className="w-full min-w-0 max-w-full justify-self-stretch justify-items-stretch break-words [overflow-wrap:anywhere]">
          <UpstreamExchangeDetails details={details} open={detailsOpen} onOpenChange={setDetailsOpen} />
        </AlertDescription>
      ) : null}
    </Alert>
  );
}

function UpstreamExchangeDetails({
  details,
  open,
  onOpenChange,
}: {
  details?: ChatInlineAlert["details"];
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("chat.messages");

  return (
    <Accordion
      type="single"
      collapsible
      value={open ? "upstream-debug" : ""}
      onValueChange={(value) => onOpenChange(value === "upstream-debug")}
      className="w-full min-w-0 max-w-full text-xs text-foreground"
    >
      <AccordionItem value="upstream-debug" className="w-full min-w-0 max-w-full border-b-0">
        <AccordionContent className="w-full min-w-0 max-w-full pb-0 pt-3">
          <Tabs defaultValue="request" className="min-w-0 w-full max-w-full overflow-hidden">
            <TabsList className="h-7 gap-1">
              <TabsTrigger value="request">{t("debugRequest")}</TabsTrigger>
              <TabsTrigger value="response">{t("debugResponse")}</TabsTrigger>
            </TabsList>
            <TabsContent value="request" className="min-w-0 w-full max-w-full overflow-hidden">
              <DebugCodeBlock value={rawRequestBody(details)} />
            </TabsContent>
            <TabsContent value="response" className="min-w-0 w-full max-w-full overflow-hidden">
              <DebugCodeBlock value={rawResponseBody(details)} />
            </TabsContent>
          </Tabs>
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  );
}

function rawRequestBody(details?: ChatInlineAlert["details"]): string {
  return details?.request?.body ?? "";
}

function rawResponseBody(details?: ChatInlineAlert["details"]): string {
  return details?.response?.body ?? "";
}

function DebugCodeBlock({ value }: { value: string }) {
  return (
    <pre className="block max-h-96 min-w-0 w-full max-w-full justify-self-stretch overflow-y-auto overflow-x-hidden rounded-md bg-muted/45 px-4 py-3 text-[12px] leading-6 whitespace-pre-wrap break-words text-foreground [overflow-wrap:anywhere]">
      <code>{formatDebugValue(value)}</code>
    </pre>
  );
}

function formatDebugValue(value: string): string {
  const raw = value.trim();
  if (!raw) {
    return "";
  }
  const parsedSSE = formatSSEData(raw);
  if (parsedSSE) {
    return parsedSSE;
  }
  return formatJSON(raw);
}

function formatSSEData(value: string): string {
  if (!/(^|\n)data:\s*/.test(value)) {
    return "";
  }
  const payloads = value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.startsWith("data:"))
    .map((line) => line.slice("data:".length).trim())
    .filter((line) => line && line !== "[DONE]");
  if (payloads.length === 0) {
    return value;
  }
  return payloads.map(formatJSON).join("\n\n");
}

function formatJSON(value: string): string {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}

export function AssistantMessageSkeleton({ fileProc, label }: { fileProc?: boolean; label?: string } = {}) {
  const t = useTranslations("chat.messages");
  if (fileProc) {
    return (
      <div className="flex items-center gap-2 pt-1 text-[13px] text-muted-foreground">
        <span className="inline-block size-3.5 animate-spin rounded-full border-2 border-muted border-t-foreground/50" />
        {label?.trim() || t("processing")}
      </div>
    );
  }
  return (
    <div className="w-full max-w-[680px] space-y-2.5 pt-1">
      <Skeleton className="h-4 w-[72%] rounded-full bg-muted/35" />
      <Skeleton className="h-4 w-[96%] rounded-full bg-muted/35" />
      <Skeleton className="h-4 w-[88%] rounded-full bg-muted/35" />
      <Skeleton className="h-4 w-[64%] rounded-full bg-muted/35" />
    </div>
  );
}

export function AssistantImageGenerationSkeleton({
  label,
  aspectRatio = "wide",
}: {
  label?: string;
  aspectRatio?: ChatAreaMessage["imageAspectRatio"];
}) {
  const t = useTranslations("chat.messages");
  const frameClassName =
    aspectRatio === "portrait" ? "max-w-[18rem]" : aspectRatio === "square" ? "max-w-[24rem]" : "max-w-[32rem]";
  const aspectClassName =
    aspectRatio === "portrait" ? "aspect-[9/16]" : aspectRatio === "square" ? "aspect-square" : "aspect-video";
  return (
    <div className={cn("my-4 w-full space-y-2.5", frameClassName)}>
      <div className="flex items-center gap-2 pt-1 text-[13px] text-muted-foreground">
        <span className="inline-block size-3.5 animate-spin rounded-full border-2 border-muted border-t-foreground/50" />
        {label?.trim() || t("processing")}
      </div>
      <div className={cn("relative w-full overflow-hidden rounded-xl bg-muted/20 text-primary", aspectClassName)}>
        <GrainientBackground
          className="absolute inset-0 text-primary/75"
          color1="#BAE6FD"
          color2="#60A5FA"
          color3="#A78BFA"
          contrast={1.48}
          saturation={1.0}
          timeSpeed={2.6}
          warpAmplitude={72}
          warpSpeed={2.1}
        />
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <span className="select-none text-[clamp(1.75rem,7vw,4rem)] font-semibold tracking-[0.18em] text-white/30 mix-blend-overlay drop-shadow-sm">
            DEEIX
          </span>
        </div>
      </div>
    </div>
  );
}
