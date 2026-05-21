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
} from "@/features/chat/types/messages";
import { StreamdownRender } from "@/features/chat/components/markdown/streamdown-render";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
} from "@/components/ui/accordion";
import {
  Alert,
  AlertDescription,
} from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { summarizeUpstreamError } from "@/features/chat/utils/chat-runtime";
import type { FileContentResult } from "@/shared/api/file";
import type { PreviewDialogFile } from "@/features/files/components/preview/file-preview-dialog";

const EMPTY_TRACE_EVENTS: NonNullable<ChatAreaMessage["processTrace"]>["events"] = [];

type ChatMessageBotProps = {
  item: ChatAreaMessage;
  busy: boolean;
  reaction: AssistantReaction;
  onRetryAssistantMessage: (message: ChatAreaMessage) => Promise<void> | void;
  onCycleMessageBranch: (parentPublicID: string | null, direction: "previous" | "next") => void;
  onReactAssistantMessage: (publicID: string, reaction: AssistantReaction) => void;
  onCopy: () => void;
  markdownRender?: boolean;
  showModelInfo?: boolean;
  showLatency?: boolean;
  showTokenUsage?: boolean;
  showBillingCost?: boolean;
  readOnly?: boolean;
  attachmentContentLoader?: (file: PreviewDialogFile) => Promise<FileContentResult>;
  showBranchNavigator?: boolean;
};

export function ChatMessageBot({
  item,
  busy,
  reaction,
  onRetryAssistantMessage,
  onCycleMessageBranch,
  onReactAssistantMessage,
  onCopy,
  markdownRender = true,
  showModelInfo = true,
  showLatency = true,
  showTokenUsage = true,
  showBillingCost = false,
  readOnly = false,
  attachmentContentLoader,
  showBranchNavigator = true,
}: ChatMessageBotProps) {
  const onRetry = React.useCallback(() => {
    void onRetryAssistantMessage(item);
  }, [item, onRetryAssistantMessage]);
  const upstreamThink = item.processTrace?.upstreamThink;
  const toolTrace = item.processTrace?.tools;
  const traceEvents = item.processTrace?.events ?? EMPTY_TRACE_EVENTS;
  const messageStreaming = Boolean(item.isStreaming);
  const upstreamThinkStreaming = messageStreaming && upstreamThink?.status === "streaming";
  const toolTraceStreaming = messageStreaming && toolTrace?.status === "streaming";
  const hasStreamdownContent = item.content.trim().length > 0;
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
  const activeThinkBlock = hasTraceEvents && upstreamThink?.status === "streaming" ? upstreamThink : undefined;
  const activeToolBlock = hasTraceEvents && toolTrace?.status === "streaming" ? toolTrace : undefined;
  const processAutoCollapseReady = Boolean(hasTraceEvents || upstreamThink || toolTrace || hasStreamdownContent || item.inlineAlert);
  const toolAutoCollapseReady = Boolean(upstreamThink || hasStreamdownContent || item.inlineAlert);

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
        ) : hasStreamdownContent && markdownRender ? (
          <StreamdownRender content={item.content} streaming={Boolean(item.isStreaming)} />
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
        onCopy={onCopy}
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
  const summaryText = [summary.statusCode ? `HTTP ${summary.statusCode}` : "", summary.reason].filter(Boolean).join(", ");
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
