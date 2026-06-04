import type {
  ChatAreaMessage,
  ChatInlineAlert,
  ChatMessageBranchNavigator,
  ChatMessageProcessTrace,
  MessageAttachment,
} from "@/features/chat/types/messages";

function areBranchNavigatorsEqual(
  previous: ChatMessageBranchNavigator | undefined,
  next: ChatMessageBranchNavigator | undefined,
) {
  if (previous === next) return true;
  if (!previous || !next) return false;
  return (
    previous.parentPublicID === next.parentPublicID &&
    previous.index === next.index &&
    previous.total === next.total &&
    previous.canPrevious === next.canPrevious &&
    previous.canNext === next.canNext
  );
}

function areInlineAlertsEqual(
  previous: ChatInlineAlert | undefined,
  next: ChatInlineAlert | undefined,
) {
  if (previous === next) return true;
  if (!previous || !next) return false;
  return previous.title === next.title && previous.message === next.message;
}

function areProcessTracesEqual(
  previous: ChatMessageProcessTrace | undefined,
  next: ChatMessageProcessTrace | undefined,
) {
  if (previous === next) return true;
  if (!previous || !next) return false;
  const previousEvents = previous.events ?? [];
  const nextEvents = next.events ?? [];
  const eventsEqual =
    previousEvents.length === nextEvents.length &&
    previousEvents.every((event, index) => {
      const nextEvent = nextEvents[index];
      return (
        event.eventID === nextEvent.eventID &&
        event.stage === nextEvent.stage &&
        event.roundID === nextEvent.roundID &&
        event.parentEventID === nextEvent.parentEventID &&
        event.status === nextEvent.status &&
        event.summary === nextEvent.summary &&
        event.contentMarkdown === nextEvent.contentMarkdown &&
        event.updatedAt === nextEvent.updatedAt &&
        event.payloadJson === nextEvent.payloadJson
      );
    });
  return (
    previous.enabled === next.enabled &&
    previous.status === next.status &&
    previous.process?.title === next.process?.title &&
    previous.process?.summary === next.process?.summary &&
    previous.process?.contentMarkdown === next.process?.contentMarkdown &&
    previous.process?.status === next.process?.status &&
    previous.process?.stage === next.process?.stage &&
    previous.process?.roundID === next.process?.roundID &&
    previous.process?.parentEventID === next.process?.parentEventID &&
    previous.process?.updatedAt === next.process?.updatedAt &&
    previous.process?.payloadJson === next.process?.payloadJson &&
    previous.tools?.title === next.tools?.title &&
    previous.tools?.summary === next.tools?.summary &&
    previous.tools?.contentMarkdown === next.tools?.contentMarkdown &&
    previous.tools?.status === next.tools?.status &&
    previous.tools?.stage === next.tools?.stage &&
    previous.tools?.roundID === next.tools?.roundID &&
    previous.tools?.parentEventID === next.tools?.parentEventID &&
    previous.tools?.updatedAt === next.tools?.updatedAt &&
    previous.tools?.payloadJson === next.tools?.payloadJson &&
    previous.upstreamThink?.title === next.upstreamThink?.title &&
    previous.upstreamThink?.summary === next.upstreamThink?.summary &&
    previous.upstreamThink?.contentMarkdown === next.upstreamThink?.contentMarkdown &&
    previous.upstreamThink?.status === next.upstreamThink?.status &&
    previous.upstreamThink?.stage === next.upstreamThink?.stage &&
    previous.upstreamThink?.roundID === next.upstreamThink?.roundID &&
    previous.upstreamThink?.parentEventID === next.upstreamThink?.parentEventID &&
    previous.upstreamThink?.updatedAt === next.upstreamThink?.updatedAt &&
    previous.upstreamThink?.payloadJson === next.upstreamThink?.payloadJson &&
    eventsEqual
  );
}

function areAttachmentsEqual(
  previous: MessageAttachment[] | undefined,
  next: MessageAttachment[] | undefined,
) {
  if (previous === next) return true;
  if (!previous || !next || previous.length !== next.length) return false;

  return previous.every((item, index) => {
    const nextItem = next[index];
    return (
      item.fileID === nextItem.fileID &&
      item.fileName === nextItem.fileName &&
      item.mimeType === nextItem.mimeType &&
      item.detectedMime === nextItem.detectedMime &&
      item.fileCategory === nextItem.fileCategory &&
      item.sizeBytes === nextItem.sizeBytes &&
      item.kind === nextItem.kind &&
      item.previewURL === nextItem.previewURL &&
      item.processingStatus === nextItem.processingStatus &&
      item.processingReady === nextItem.processingReady &&
      item.processingErrorCode === nextItem.processingErrorCode &&
      item.processingErrorMessage === nextItem.processingErrorMessage &&
      item.extractStatus === nextItem.extractStatus &&
      item.embedStatus === nextItem.embedStatus &&
      item.ragReady === nextItem.ragReady &&
      item.ragReason === nextItem.ragReason &&
      item.ocrUsed === nextItem.ocrUsed
    );
  });
}

function areCompactDoneEqual(
  previous: ChatAreaMessage["compactDone"],
  next: ChatAreaMessage["compactDone"],
) {
  if (previous === next) return true;
  if (!previous || !next) return false;
  return (
    previous.method === next.method &&
    previous.freed_tokens === next.freed_tokens &&
    previous.summary_preview === next.summary_preview
  );
}

function areBillingCostsEqual(
  previous: ChatAreaMessage["billingCost"],
  next: ChatAreaMessage["billingCost"],
) {
  if (previous === next) return true;
  if (!previous || !next) return false;
  return (
    previous.billingMode === next.billingMode &&
    previous.billedCurrency === next.billedCurrency &&
    previous.billedNanousd === next.billedNanousd &&
    previous.billedUSD === next.billedUSD &&
    previous.pricingSnapshotJSON === next.pricingSnapshotJSON
  );
}

export function areChatAreaMessagesRenderEqual(
  previous: ChatAreaMessage,
  next: ChatAreaMessage,
) {
  return (
    previous.key === next.key &&
    previous.publicID === next.publicID &&
    previous.parentPublicID === next.parentPublicID &&
    previous.sourcePublicID === next.sourcePublicID &&
    previous.role === next.role &&
    previous.contentType === next.contentType &&
    previous.content === next.content &&
    previous.branchReason === next.branchReason &&
    previous.platformModelName === next.platformModelName &&
    previous.serverMessageID === next.serverMessageID &&
    previous.createdAt === next.createdAt &&
    previous.updatedAt === next.updatedAt &&
    previous.editedAt === next.editedAt &&
    previous.isPending === next.isPending &&
    previous.isStreaming === next.isStreaming &&
    previous.isFileProc === next.isFileProc &&
    previous.activityLabel === next.activityLabel &&
    previous.imageAspectRatio === next.imageAspectRatio &&
    previous.myFeedback === next.myFeedback &&
    previous.thumbsUpCount === next.thumbsUpCount &&
    previous.thumbsDownCount === next.thumbsDownCount &&
    previous.inputTokens === next.inputTokens &&
    previous.outputTokens === next.outputTokens &&
    previous.cacheReadTokens === next.cacheReadTokens &&
    previous.cacheWriteTokens === next.cacheWriteTokens &&
    previous.reasoningTokens === next.reasoningTokens &&
    previous.latencyMS === next.latencyMS &&
    areBillingCostsEqual(previous.billingCost, next.billingCost) &&
    areBranchNavigatorsEqual(previous.branchNavigator, next.branchNavigator) &&
    areAttachmentsEqual(previous.attachments, next.attachments) &&
    areProcessTracesEqual(previous.processTrace, next.processTrace) &&
    areInlineAlertsEqual(previous.inlineAlert, next.inlineAlert) &&
    areCompactDoneEqual(previous.compactDone, next.compactDone)
  );
}
