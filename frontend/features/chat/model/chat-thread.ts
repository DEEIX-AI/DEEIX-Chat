import type { ChatAreaMessage, MessageAttachment } from "@/features/chat/types/messages";
import type { MessageDTO, UpstreamDebugInfo } from "@/shared/api/conversation.types";

function parseAttachments(raw: string): MessageAttachment[] {
  if (!raw) return [];
  try {
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return (parsed as Record<string, unknown>[])
      .map((item) => ({
        fileID: String(item.file_id ?? ""),
        fileName: String(item.file_name ?? ""),
        mimeType: String(item.mime_type ?? ""),
        detectedMime: String(item.detected_mime ?? ""),
        fileCategory: String(item.file_category ?? ""),
        sizeBytes: Number(item.file_size ?? 0),
        kind: item.kind === "image" ? ("image" as const) : ("file" as const),
        processingStatus: String(item.processing_status ?? ""),
        processingReady: Boolean(item.processing_ready),
        processingErrorCode: String(item.processing_error_code ?? ""),
        processingErrorMessage: String(item.processing_error_message ?? ""),
      }))
      .filter((item) => item.fileID && item.fileName);
  } catch {
    return [];
  }
}

function parseProcessTrace(item: MessageDTO) {
  const trace = item.processTrace;
  if (!trace?.enabled) {
    return undefined;
  }
  const mapBlock = (block: typeof trace.process) =>
    block
      ? {
          title: block.title,
          summary: block.summary,
          contentMarkdown: block.contentMarkdown,
          status: block.status,
          stage: block.stage,
          roundID: block.roundID,
          parentEventID: block.parentEventID,
          updatedAt: block.updatedAt,
          payloadJson: block.payloadJSON,
        }
      : undefined;
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
    process: mapBlock(trace.process),
    tools: mapBlock(trace.tools),
    upstreamThink: mapBlock(trace.upstreamThink),
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

function parseUpstreamDebugInfo(value: unknown): UpstreamDebugInfo | undefined {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return undefined;
  }
  const candidate = value as UpstreamDebugInfo;
  const hasRequest = Boolean(candidate.request && typeof candidate.request === "object" && !Array.isArray(candidate.request));
  const hasResponse = Boolean(candidate.response && typeof candidate.response === "object" && !Array.isArray(candidate.response));
  if (hasRequest || hasResponse) {
    return candidate;
  }
  return undefined;
}

function parseUpstreamDebugPayload(payloadJSON: string | undefined): UpstreamDebugInfo | undefined {
  if (!payloadJSON) {
    return undefined;
  }
  try {
    const parsed = JSON.parse(payloadJSON.trim()) as { upstream_debug?: unknown };
    return parseUpstreamDebugInfo(parsed.upstream_debug);
  } catch {
    return undefined;
  }
}

function upstreamDebugScore(value: UpstreamDebugInfo): number {
  let score = 0;
  if (value.request?.body?.trim()) score += 8;
  if (value.response?.body?.trim()) score += 4;
  if (value.request?.headers && Object.keys(value.request.headers).length > 0) score += 2;
  if (value.response?.headers && Object.keys(value.response.headers).length > 0) score += 1;
  return score;
}

function extractInlineAlertDetails(item: MessageDTO): UpstreamDebugInfo | undefined {
  const trace = item.processTrace;
  const payloads = [
    trace?.process?.payloadJSON,
    trace?.tools?.payloadJSON,
    trace?.upstreamThink?.payloadJSON,
    ...(trace?.events?.map((event) => event.payloadJSON) ?? []),
  ];
  return payloads.reduce<UpstreamDebugInfo | undefined>((best, payloadJSON) => {
    const current = parseUpstreamDebugPayload(payloadJSON);
    if (!current) {
      return best;
    }
    if (!best || upstreamDebugScore(current) > upstreamDebugScore(best)) {
      return current;
    }
    return best;
  }, undefined);
}

const ROOT_BRANCH_KEY = "__root__";

export type BranchSelectionPathItem = {
  parentPublicID?: string | null;
  publicID?: string | null;
};

type MessageLabels = {
  generationInterrupted: string;
  streamInterrupted?: string;
  imageRunning?: string;
  resolveErrorMessage?: (errorCode: string, fallback: string, details?: UpstreamDebugInfo) => string;
};

function resolveAssistantErrorMessage(item: MessageDTO, labels: MessageLabels, details?: UpstreamDebugInfo): string {
  const fallback = item.errorMessage.trim();
  if (item.errorCode === "stream_interrupted" || item.errorCode === "conversation_run.stream_interrupted") {
    return labels.streamInterrupted || fallback;
  }
  const errorCode = item.errorCode.trim();
  if (errorCode && labels.resolveErrorMessage) {
    return labels.resolveErrorMessage(errorCode, fallback, details);
  }
  return fallback;
}

export function mapServerMessage(
  item: MessageDTO,
  labels: MessageLabels = {
    generationInterrupted: "Generation interrupted",
  },
  options: { liveRunIDs?: ReadonlySet<string> } = {},
): ChatAreaMessage {
  const publicID = item.publicID.trim();
  const msg: ChatAreaMessage = {
    key: `server-${publicID}`,
    publicID,
    parentPublicID: item.parentPublicID?.trim() || null,
    sourcePublicID: item.sourcePublicID?.trim() || null,
    role: item.role === "assistant" ? "assistant" : item.role === "system" ? "system" : "user",
    contentType: item.contentType,
    content: item.content,
    branchReason: item.branchReason || "default",
    status: item.status || "success",
    runID: item.runID || undefined,
    platformModelName: item.platformModelName?.trim() || undefined,
    serverMessageID: item.id,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
    editedAt: item.editedAt ?? null,
    myFeedback: item.myFeedback || null,
    thumbsUpCount: item.thumbsUpCount ?? 0,
    thumbsDownCount: item.thumbsDownCount ?? 0,
  };
  const parsedAttachments = parseAttachments(item.attachments);
  if (parsedAttachments.length > 0) {
    msg.attachments = parsedAttachments;
  }
  if (item.role === "user") {
    msg.inputTokens = item.inputTokens ?? 0;
    msg.cacheReadTokens = item.cacheReadTokens ?? 0;
    msg.cacheWriteTokens = item.cacheWriteTokens ?? 0;
  }
  if (item.role === "assistant") {
    msg.inputTokens = item.inputTokens ?? 0;
    msg.outputTokens = item.outputTokens ?? 0;
    msg.cacheReadTokens = item.cacheReadTokens ?? 0;
    msg.cacheWriteTokens = item.cacheWriteTokens ?? 0;
    msg.reasoningTokens = item.reasoningTokens ?? 0;
    msg.latencyMS = item.latencyMS ?? 0;
    msg.billingCost = item.billingCost;
    msg.processTrace = parseProcessTrace(item);
    if ((item.status === "error" || item.status === "interrupted") && item.errorMessage?.trim()) {
      const details = extractInlineAlertDetails(item);
      msg.inlineAlert = {
        title: labels.generationInterrupted,
        message: resolveAssistantErrorMessage(item, labels, details),
        details,
      };
    }
    if (item.status === "pending") {
      const liveRunID = item.runID?.trim() || "";
      const live = Boolean(liveRunID && options.liveRunIDs?.has(liveRunID));
      msg.isPending = live;
      msg.isStreaming = live;
      msg.activityLabel = live && item.contentType === "image" ? labels.imageRunning : undefined;
    }
  }
  return msg;
}

export function toBranchKey(publicID?: string | null): string {
  return publicID?.trim() || ROOT_BRANCH_KEY;
}

export function buildChildrenIndex(messages: ChatAreaMessage[]) {
  const children = new Map<string, ChatAreaMessage[]>();
  for (const item of messages) {
    const parentKey = toBranchKey(item.parentPublicID);
    const siblings = children.get(parentKey) ?? [];
    siblings.push(item);
    children.set(parentKey, siblings);
  }
  return children;
}

export function applyBranchSelectionPath(
  previous: Record<string, string>,
  path: BranchSelectionPathItem[],
  obsoletePublicIDs: Array<string | null | undefined> = [],
): Record<string, string> {
  const obsolete = new Set(obsoletePublicIDs.map((item) => item?.trim() || "").filter(Boolean));
  let changed = false;
  const next = { ...previous };

  for (const [key, value] of Object.entries(next)) {
    if (obsolete.has(key) || obsolete.has(value)) {
      delete next[key];
      changed = true;
    }
  }

  for (const item of path) {
    const publicID = item.publicID?.trim() || "";
    if (!publicID) {
      continue;
    }
    const parentKey = toBranchKey(item.parentPublicID);
    if (next[parentKey] !== publicID) {
      next[parentKey] = publicID;
      changed = true;
    }
  }

  return changed ? next : previous;
}

export function resolveBranchSelectionPath(
  messages: ChatAreaMessage[],
  leafPublicID: string | null | undefined,
): BranchSelectionPathItem[] {
  const leafID = leafPublicID?.trim() || "";
  if (!leafID) {
    return [];
  }

  const byPublicID = new Map(messages.map((item) => [item.publicID, item]));
  const path: BranchSelectionPathItem[] = [];
  const visited = new Set<string>();
  let current = byPublicID.get(leafID) ?? null;

  while (current && !visited.has(current.publicID)) {
    visited.add(current.publicID);
    path.push({
      parentPublicID: current.parentPublicID,
      publicID: current.publicID,
    });
    current = current.parentPublicID ? byPublicID.get(current.parentPublicID) ?? null : null;
  }

  return path;
}

export function reconcileBranchSelections(messages: ChatAreaMessage[], previous: Record<string, string>) {
  const next: Record<string, string> = {};
  const children = buildChildrenIndex(messages);
  for (const [parentKey, siblings] of children.entries()) {
    const existing = previous[parentKey];
    if (existing && siblings.some((item) => item.publicID === existing)) {
      next[parentKey] = existing;
      continue;
    }
    const latest = siblings[siblings.length - 1];
    if (latest) {
      next[parentKey] = latest.publicID;
    }
  }
  return next;
}

export function buildVisibleMessages(
  messages: ChatAreaMessage[],
  selections: Record<string, string>,
): ChatAreaMessage[] {
  const children = buildChildrenIndex(messages);
  let visible: ChatAreaMessage[] = [];
  const visited = new Set<string>();
  let parentKey = ROOT_BRANCH_KEY;

  while (true) {
    const siblings = children.get(parentKey);
    if (!siblings || siblings.length === 0) {
      break;
    }

    const selectedPublicID = selections[parentKey] || siblings[siblings.length - 1]?.publicID;
    const selected = siblings.find((item) => item.publicID === selectedPublicID) ?? siblings[siblings.length - 1];
    if (!selected || visited.has(selected.publicID)) {
      break;
    }

    visited.add(selected.publicID);
    visible.push(selected);
    parentKey = selected.publicID;
  }

  if (visible.length === 0 && messages.length > 0) {
    visible = buildTailVisibleMessages(messages);
  }

  const withUserNavigators = visible.map((item) => {
    if (item.role !== "user") {
      return item;
    }
    const siblings = children.get(toBranchKey(item.parentPublicID)) ?? [];
    if (siblings.length <= 1) {
      return item;
    }
    const currentIndex = siblings.findIndex((candidate) => candidate.publicID === item.publicID);
    if (currentIndex < 0) {
      return item;
    }
    return {
      ...item,
      branchNavigator: {
        parentPublicID: item.parentPublicID,
        index: currentIndex + 1,
        total: siblings.length,
        canPrevious: currentIndex > 0,
        canNext: currentIndex < siblings.length - 1,
      },
    };
  });

  return withUserNavigators.map((item, index) => {
    if (item.role !== "assistant") {
      return item;
    }
    const previous = index > 0 ? withUserNavigators[index - 1] : null;
    if (!previous || previous.role !== "user") {
      return item;
    }
    return {
      ...item,
      inputTokens: item.inputTokens && item.inputTokens > 0 ? item.inputTokens : previous.inputTokens,
      cacheReadTokens: item.cacheReadTokens && item.cacheReadTokens > 0 ? item.cacheReadTokens : previous.cacheReadTokens,
      cacheWriteTokens: item.cacheWriteTokens && item.cacheWriteTokens > 0 ? item.cacheWriteTokens : previous.cacheWriteTokens,
      branchNavigator: previous.branchNavigator ?? item.branchNavigator,
    };
  });
}

function buildTailVisibleMessages(messages: ChatAreaMessage[]): ChatAreaMessage[] {
  const byPublicID = new Map(messages.map((item) => [item.publicID, item]));
  const visible: ChatAreaMessage[] = [];
  const visited = new Set<string>();
  let current = messages.at(-1) ?? null;

  while (current && !visited.has(current.publicID)) {
    visited.add(current.publicID);
    visible.push(current);
    const parentPublicID = current.parentPublicID?.trim() || "";
    current = parentPublicID ? byPublicID.get(parentPublicID) ?? null : null;
  }

  return visible.reverse();
}
