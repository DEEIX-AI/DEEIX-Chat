import type { ChatAreaMessage, ChatMessageProcessTrace, MessageAttachment } from "@/features/chat/types/messages";
import type { PendingAttachment } from "@/features/chat/types/chat-runtime";
import type {
  MessageProcessTraceDTO,
  TraceBlockDTO,
} from "@/shared/api/conversation.types";

export function toPendingAttachments(message: ChatAreaMessage | null | undefined): PendingAttachment[] {
  if (!message?.attachments || message.attachments.length === 0) {
    return [];
  }
  return message.attachments.map(toPendingAttachment);
}

export function toPendingAttachment(item: MessageAttachment): PendingAttachment {
  return {
    fileID: item.fileID,
    fileName: item.fileName,
    mimeType: item.mimeType,
    detectedMime: item.detectedMime,
    fileCategory: item.fileCategory,
    sizeBytes: item.sizeBytes,
    previewURL: item.previewURL,
    processingStatus: item.processingStatus,
    processingReady: item.processingReady,
    processingErrorCode: item.processingErrorCode,
    processingErrorMessage: item.processingErrorMessage,
    extractStatus: item.extractStatus,
    embedStatus: item.embedStatus,
    ragReady: item.ragReady,
    ragReason: item.ragReason,
    ocrUsed: item.ocrUsed,
  };
}

export function resolvePersistedPublicID(value: string | null | undefined): string | null {
  const normalized = value?.trim() || "";
  if (!normalized || normalized.startsWith("local-exchange-")) {
    return null;
  }
  return normalized;
}

export function resolveAssistantInputSideUsageValue(
  assistantOwnsUsage: boolean,
  assistantValue: number | null | undefined,
  userValue: number | null | undefined,
  liveValue: number | null | undefined,
): number {
  if (assistantOwnsUsage) {
    return typeof assistantValue === "number" && Number.isFinite(assistantValue) && assistantValue >= 0
      ? assistantValue
      : 0;
  }
  for (const value of [assistantValue, userValue, liveValue]) {
    if (typeof value === "number" && Number.isFinite(value) && value > 0) {
      return value;
    }
  }
  return 0;
}

function isSuccessfulContextMessage(message: ChatAreaMessage): boolean {
  const status = message.status?.trim().toLowerCase() || "success";
  return (
    (status === "success" || (message.role === "assistant" && status === "interrupted")) &&
    !message.isPending &&
    !message.isStreaming &&
    resolvePersistedPublicID(message.publicID) !== null
  );
}

export function resolveDefaultSubmissionParentMessage(messages: ChatAreaMessage[]): ChatAreaMessage | null {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message.role === "assistant" && isSuccessfulContextMessage(message)) {
      return message;
    }
  }
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message.role === "user" && isSuccessfulContextMessage(message)) {
      return message;
    }
  }
  return null;
}

function toPendingTraceBlock(block: TraceBlockDTO | undefined) {
  if (!block) {
    return undefined;
  }
  return {
    title: block.title,
    summary: block.summary,
    contentMarkdown: block.contentMarkdown,
    status: block.status,
    stage: block.stage,
    roundID: block.roundID,
    parentEventID: block.parentEventID,
    startedAt: block.startedAt,
    updatedAt: block.updatedAt,
    payloadJson: block.payloadJSON,
  };
}

type SnapshotThinkEvent = {
  eventID: string;
  eventType: string;
  phase: string;
  summary: string;
  contentMarkdown: string;
  endedAt?: string;
};

// 实时快照里的思考事件不带正文；用上一份快照或数据库加载的轨迹补全正文与结束时间，避免整体替换时丢失。
export function mergeProcessTraceSnapshot<T extends { events?: SnapshotThinkEvent[] }>(
  previous: T | undefined,
  next: T | undefined,
): T | undefined {
  const previousEvents = previous?.events;
  if (!next?.events?.length || !previousEvents?.length) {
    return next;
  }
  const previousByEventID = new Map(previousEvents.map((event) => [event.eventID, event]));
  let changed = false;
  const events = next.events.map((event) => {
    if (event.phase !== "upstream_think" && event.eventType !== "think") {
      return event;
    }
    const known = previousByEventID.get(event.eventID);
    if (!known) {
      return event;
    }
    const contentMarkdown = known.contentMarkdown.length > event.contentMarkdown.length ? known.contentMarkdown : event.contentMarkdown;
    const summary = event.summary || known.summary;
    const endedAt = event.endedAt ?? known.endedAt;
    if (contentMarkdown === event.contentMarkdown && summary === event.summary && endedAt === event.endedAt) {
      return event;
    }
    changed = true;
    return { ...event, contentMarkdown, summary, endedAt };
  });
  return changed ? { ...next, events } : next;
}

export function toPendingProcessTrace(trace: MessageProcessTraceDTO | undefined): ChatMessageProcessTrace | undefined {
  if (!trace?.enabled) {
    return undefined;
  }
  const promptTrace = trace.promptTrace
    ? {
        mode: trace.promptTrace.mode,
        promptFingerprint: trace.promptTrace.promptFingerprint,
        statefulUsed: trace.promptTrace.statefulUsed,
        statefulDisabledReason: trace.promptTrace.statefulDisabledReason,
        totalTokenEstimate: trace.promptTrace.totalTokenEstimate,
        sentTokenEstimate: trace.promptTrace.sentTokenEstimate,
        fullMessageCount: trace.promptTrace.fullMessageCount,
        sentMessageCount: trace.promptTrace.sentMessageCount,
        statefulSavedMessages: trace.promptTrace.statefulSavedMessages,
        statefulSavedTokens: trace.promptTrace.statefulSavedTokens,
        blocks: trace.promptTrace.blocks?.map((block) => ({
          kind: block.kind,
          title: block.title,
          tokenEstimate: block.tokenEstimate,
          cacheable: block.cacheable,
          sourceCount: block.sourceCount,
          sourceRefs: block.sourceRefs?.map((ref) => ({
            sourceType: ref.sourceType,
            sourceID: ref.sourceID,
            title: ref.title,
            artifactID: ref.artifactID,
          })),
        })) ?? [],
      }
    : undefined;
  return {
    enabled: true,
    status: trace.status,
    process: toPendingTraceBlock(trace.process),
    tools: toPendingTraceBlock(trace.tools),
    upstreamThink: toPendingTraceBlock(trace.upstreamThink),
    promptTrace,
    events: trace.events?.map((event) => ({
      eventID: event.eventID,
      eventType: event.eventType,
      phase: event.phase,
      stage: event.stage,
      roundID: event.roundID,
      parentEventID: event.parentEventID,
      title: event.title,
      summary: event.summary,
      contentMarkdown: event.contentMarkdown,
      status: event.status,
      seq: event.seq,
      startedAt: event.startedAt,
      endedAt: event.endedAt,
      updatedAt: event.updatedAt,
      payloadJson: event.payloadJSON,
    })),
  };
}
