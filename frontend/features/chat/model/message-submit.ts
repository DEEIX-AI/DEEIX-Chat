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

function isSuccessfulContextMessage(message: ChatAreaMessage): boolean {
  const status = message.status?.trim().toLowerCase() || "success";
  return (
    status === "success" &&
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
    updatedAt: block.updatedAt,
    payloadJson: block.payloadJSON,
  };
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
