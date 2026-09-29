import type { ConversationRuns } from "@deeix/api-contract";
import { authedFetch, authedRequest } from "@/shared/api/authed-client";
import type { PagePayload } from "@/shared/api/common-types";
import type {
  ActiveConversationRunEvent,
  ActiveConversationRunSnapshot,
  ConversationRunStatusDTO,
  BatchSetConversationProjectRequest,
  BatchSetConversationProjectResult,
  ContextArtifactDTO,
  ConversationDefaultModelCandidateDTO,
  ConversationDTO,
  ConversationExportDTO,
  ConversationPreviewMessageDTO,
  ConversationProjectDTO,
  ConversationProjectFilter,
  ConversationProjectStatusFilter,
  ConversationRunDTO,
  ConversationSearchPageDTO,
  ConversationShareDTO,
  ConversationShareFilter,
  ConversationStarredFilter,
  ConversationStatusFilter,
  ConversationToolCallDetailDTO,
  CreateConversationProjectRequest,
  CreateConversationRequest,
  CreateConversationShareRequest,
  DeleteConversationData,
  MediaImageRequest,
  MediaVideoExtensionRequest,
  MediaVideoRequest,
  MessageDeleteDTO,
  MessageDTO,
  MessageFeedbackResult,
  MessageProcessTraceDTO,
  PromptTraceBlockDTO,
  PromptTraceDTO,
  PromptTraceSourceDTO,
  PublicSharedConversationDTO,
  RenameConversationRequest,
  ReorderConversationProjectsRequest,
  RevokeConversationSharesRequest,
  RevokeConversationSharesResult,
  SendMessageRequest,
  SendMessageResult,
  SetConversationArchiveRequest,
  SetConversationProjectRequest,
  SetConversationStarRequest,
  SetMessageFeedbackRequest,
  StreamMessageEvent,
  TemporaryChatMessageRequest,
  TraceBlockDTO,
  TraceEventDTO,
  UpdateConversationLabelsRequest,
  UpdateConversationProjectRequest,
  UpdateMessageRequest,
} from "@/shared/api/conversation-types";
import { ApiError, apiRequest, pathParam } from "@/shared/api/http-client";
import {
  isOneOf,
  isRecord,
  parseJSON,
  readBoolean,
  readFiniteNumber,
  readString,
  type UnknownRecord,
} from "@/shared/lib/type-guards";

export const TEMPORARY_CHAT_MAX_ATTACHMENTS = 20;
export const TEMPORARY_CHAT_MAX_IMAGE_ATTACHMENTS = 10;

export type TemporaryChatRequestAttachment = {
  file: File;
  messageIndex: number;
  kind: "file" | "image";
};

function normalizeTraceBlock(block: unknown): TraceBlockDTO | undefined {
  if (!isRecord(block)) {
    return undefined;
  }
  return {
    title: readString(block, "title") ?? "",
    summary: readString(block, "summary") ?? "",
    contentMarkdown: readString(block, "contentMarkdown") ?? "",
    status: readString(block, "status") ?? "",
    stage: readString(block, "stage"),
    roundID: readString(block, "roundID"),
    parentEventID: readString(block, "parentEventID"),
    startedAt: readString(block, "startedAt"),
    updatedAt: readString(block, "updatedAt") ?? "",
    payloadJSON: readString(block, "payloadJSON"),
  };
}

function normalizeTraceEvent(event: unknown): TraceEventDTO | undefined {
  if (!isRecord(event)) {
    return undefined;
  }
  return {
    eventID: readString(event, "eventID") ?? "",
    eventType: readString(event, "eventType") ?? "",
    phase: readString(event, "phase") ?? "",
    stage: readString(event, "stage"),
    roundID: readString(event, "roundID"),
    parentEventID: readString(event, "parentEventID"),
    title: readString(event, "title") ?? "",
    summary: readString(event, "summary") ?? "",
    contentMarkdown: readString(event, "contentMarkdown") ?? "",
    status: readString(event, "status") ?? "",
    seq: readFiniteNumber(event, "seq") ?? 0,
    startedAt: readString(event, "startedAt") ?? "",
    endedAt: readString(event, "endedAt"),
    updatedAt: readString(event, "updatedAt") ?? "",
    payloadJSON: readString(event, "payloadJSON"),
  };
}

function normalizePromptTraceSource(source: UnknownRecord): PromptTraceSourceDTO {
  return {
    artifactID: readFiniteNumber(source, "artifactID"),
    sourceID: readString(source, "sourceID") ?? "",
    sourceType: readString(source, "sourceType") ?? "",
    title: readString(source, "title") ?? "",
  };
}

function normalizePromptTraceBlock(block: UnknownRecord): PromptTraceBlockDTO {
  return {
    cacheable: readBoolean(block, "cacheable") ?? false,
    kind: readString(block, "kind") ?? "",
    sourceCount: readFiniteNumber(block, "sourceCount") ?? 0,
    sourceRefs: Array.isArray(block.sourceRefs)
      ? block.sourceRefs.filter(isRecord).map(normalizePromptTraceSource)
      : undefined,
    title: readString(block, "title") ?? "",
    tokenEstimate: readFiniteNumber(block, "tokenEstimate") ?? 0,
  };
}

function normalizePromptTrace(trace: unknown): PromptTraceDTO | undefined {
  if (!isRecord(trace)) {
    return undefined;
  }
  return {
    blocks: Array.isArray(trace.blocks) ? trace.blocks.filter(isRecord).map(normalizePromptTraceBlock) : [],
    fullMessageCount: readFiniteNumber(trace, "fullMessageCount") ?? 0,
    mode: readString(trace, "mode") ?? "",
    promptFingerprint: readString(trace, "promptFingerprint") ?? "",
    sentMessageCount: readFiniteNumber(trace, "sentMessageCount") ?? 0,
    sentTokenEstimate: readFiniteNumber(trace, "sentTokenEstimate") ?? 0,
    statefulDisabledReason: readString(trace, "statefulDisabledReason") ?? "",
    statefulSavedMessages: readFiniteNumber(trace, "statefulSavedMessages") ?? 0,
    statefulSavedTokens: readFiniteNumber(trace, "statefulSavedTokens") ?? 0,
    statefulUsed: readBoolean(trace, "statefulUsed") ?? false,
    totalTokenEstimate: readFiniteNumber(trace, "totalTokenEstimate") ?? 0,
  };
}

function normalizeProcessTrace(trace: unknown): MessageProcessTraceDTO | undefined {
  if (!isRecord(trace)) {
    return undefined;
  }
  return {
    enabled: Boolean(trace.enabled),
    status: readString(trace, "status") ?? "",
    process: normalizeTraceBlock(trace.process),
    tools: normalizeTraceBlock(trace.tools),
    upstreamThink: normalizeTraceBlock(trace.upstreamThink),
    promptTrace: normalizePromptTrace(trace.promptTrace),
    events: Array.isArray(trace.events) ? trace.events.map(normalizeTraceEvent).filter((event): event is TraceEventDTO => Boolean(event)) : undefined,
  };
}

const STREAM_MESSAGE_EVENT_TYPES = [
  "file_proc",
  "rag_search",
  "process_update",
  "upstream_think_delta",
  "delta",
  "usage",
  "media_status",
  "media_image_delta",
  "completed",
  "moderation_checking",
  "moderation_blocked",
  "compact_done",
  "error",
] as const satisfies readonly StreamMessageEvent["type"][];

const isStreamMessageEventType = isOneOf(STREAM_MESSAGE_EVENT_TYPES);

// Stream events are the server's typed contract (@deeix/api-contract); only the discriminant
// is checked here. Trace payloads, which the UI walks deeply, are normalized field by field below.
function isStreamMessageEvent(value: unknown): value is StreamMessageEvent {
  return isRecord(value) && isStreamMessageEventType(value.type);
}

// An event type this client does not know. It ends the stream with an ApiError, exactly like the
// `error` fallthrough in handleStreamEvent, but never reaches the typed per-event callbacks.
type UnrecognizedStreamEvent = {
  type: "unrecognized";
  seq?: number;
  message?: string;
  status?: number;
  errorCode?: string;
  debug?: unknown;
};

type ParsedStreamEvent = StreamMessageEvent | UnrecognizedStreamEvent;

function normalizeStreamEvent(rawEvent: unknown): ParsedStreamEvent {
  if (!isRecord(rawEvent)) {
    throw new ApiError("stream event is invalid", 500);
  }

  if (!isStreamMessageEvent(rawEvent)) {
    return {
      type: "unrecognized",
      seq: readFiniteNumber(rawEvent, "seq"),
      message: readString(rawEvent, "message"),
      status: readFiniteNumber(rawEvent, "status"),
      errorCode: readString(rawEvent, "errorCode"),
      debug: rawEvent.debug,
    };
  }

  const event = rawEvent;
  if (event.type === "process_update" || event.type === "upstream_think_delta") {
    return {
      ...event,
      block: normalizeTraceBlock(event.block),
      trace: normalizeProcessTrace(event.trace),
    };
  }

  return event;
}

function streamEventSeq(event: ParsedStreamEvent): number {
  return typeof event.seq === "number" && Number.isFinite(event.seq) && event.seq > 0 ? event.seq : 0;
}

function extractJSONDocuments(source: string): { documents: string[]; remainder: string } {
  const documents: string[] = [];
  let startIndex = -1;
  let depth = 0;
  let inString = false;
  let escaped = false;
  let lastConsumedIndex = 0;

  for (let index = 0; index < source.length; index += 1) {
    const char = source[index];

    if (startIndex < 0) {
      if (char === "{") {
        startIndex = index;
        depth = 1;
        lastConsumedIndex = index;
      } else if (!/\s/.test(char)) {
        break;
      } else {
        lastConsumedIndex = index + 1;
      }
      continue;
    }

    if (inString) {
      if (escaped) {
        escaped = false;
      } else if (char === "\\") {
        escaped = true;
      } else if (char === "\"") {
        inString = false;
      }
      continue;
    }

    if (char === "\"") {
      inString = true;
      continue;
    }

    if (char === "{") {
      depth += 1;
      continue;
    }

    if (char !== "}") {
      continue;
    }

    depth -= 1;
    if (depth !== 0) {
      continue;
    }

    documents.push(source.slice(startIndex, index + 1));
    startIndex = -1;
    lastConsumedIndex = index + 1;
  }

  if (startIndex >= 0) {
    return {
      documents,
      remainder: source.slice(startIndex),
    };
  }

  return {
    documents,
    remainder: source.slice(lastConsumedIndex),
  };
}

function handleStreamEvent(event: ParsedStreamEvent, options: ConversationStreamOptions, responseStatus: number): SendMessageResult | null {
  const seq = streamEventSeq(event);
  if (seq > 0) {
    options.onEventSeq?.(seq);
  }

  if (event.type === "file_proc") {
    options.onFileProc?.(event.message);
    return null;
  }

  if (event.type === "rag_search") {
    options.onRagSearch?.(event.message);
    return null;
  }

  if (event.type === "compact_done") {
    options.onCompactDone?.({
      method: event.method,
      freed_tokens: event.freed_tokens,
      kept_turns: event.kept_turns,
      summary_preview: event.summary_preview,
    });
    return null;
  }

  if (event.type === "process_update") {
    options.onProcessUpdate?.(event);
    return null;
  }

  if (event.type === "upstream_think_delta") {
    options.onUpstreamThinkDelta?.(event);
    return null;
  }

  if (event.type === "delta") {
    if (event.replace) {
      options.onTextSnapshot?.(event.delta);
    } else {
      options.onDelta?.(event.delta);
    }
    return null;
  }

  if (event.type === "usage") {
    options.onUsage?.(event);
    return null;
  }

  if (event.type === "media_status") {
    options.onMediaStatus?.(event);
    return null;
  }

  if (event.type === "media_image_delta") {
    options.onMediaImageDelta?.(event);
    return null;
  }

  if (event.type === "moderation_checking") {
    options.onModerationChecking?.(event);
    return null;
  }

  if (event.type === "moderation_blocked") {
    options.onTerminal?.(event);
    options.onModerationBlocked?.(event);
    // Terminal event for blocked rounds; synthetic result is optional.
    return null;
  }

  if (event.type === "completed") {
    options.onTerminal?.(event);
    return event.data;
  }

  if (event.type === "error") {
    options.onTerminal?.(event);
    if (event.data) {
      options.onInterrupted?.(event);
      return event.data;
    }
  }

  throw new ApiError(event.message || "stream failed", event.status ?? responseStatus, event.debug, event.errorCode);
}

type ListConversationsOptions = {
  page?: number;
  pageSize?: number;
  status?: ConversationStatusFilter;
  starred?: ConversationStarredFilter;
  share?: ConversationShareFilter;
  project?: ConversationProjectFilter;
  query?: string;
};

type SearchConversationsOptions = {
  page?: number;
  pageSize?: number;
  query?: string;
  signal?: AbortSignal;
};

type ListConversationProjectsOptions = {
  status?: ConversationProjectStatusFilter;
};

type DeleteConversationProjectOptions = {
  deleteConversations?: boolean;
  deleteFiles?: boolean;
};

type DeleteConversationOptions = {
  deleteFiles?: boolean;
};

type ListConversationRunsOptions = {
  page?: number;
  pageSize?: number;
};

// Conversation metadata
export async function listConversations(
  accessToken: string,
  options: ListConversationsOptions = {},
): Promise<PagePayload<ConversationDTO>> {
  const page = options.page && options.page > 0 ? options.page : 1;
  const pageSize = options.pageSize && options.pageSize > 0 ? options.pageSize : 20;
  const status = options.status?.trim() || "active";
  const starred = options.starred?.trim() || "all";
  const share = options.share?.trim() || "all";
  const project = options.project?.trim() || "all";
  const query = options.query?.trim() || "";
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
    status,
    starred,
    share,
    project,
  });
  if (query) {
    params.set("q", query);
  }
  const data = await authedRequest<PagePayload<ConversationDTO>>(
    `/api/v1/conversations?${params.toString()}`,
    {
      accessToken,
    },
    true,
  );
  return {
    total: data.total ?? 0,
    results: data.results ?? [],
  };
}

export async function searchConversations(
  accessToken: string,
  options: SearchConversationsOptions = {},
): Promise<ConversationSearchPageDTO> {
  const page = options.page && options.page > 0 ? options.page : 1;
  const pageSize = options.pageSize && options.pageSize > 0 ? options.pageSize : 20;
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  const query = options.query?.trim() || "";
  if (query) {
    params.set("q", query);
  }
  const data = await authedRequest<ConversationSearchPageDTO>(
    `/api/v1/conversations/search?${params.toString()}`,
    { accessToken, signal: options.signal },
    true,
  );
  return {
    hasMore: data.hasMore ?? false,
    results: data.results ?? [],
  };
}

export async function getConversationPreviewMessages(
  accessToken: string,
  conversationPublicID: string,
  signal?: AbortSignal,
): Promise<ConversationPreviewMessageDTO[]> {
  return authedRequest<ConversationPreviewMessageDTO[]>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/messages/preview`,
    { accessToken, signal },
    true,
  );
}

export async function getConversationDefaultModelCandidate(
  accessToken: string,
): Promise<ConversationDefaultModelCandidateDTO> {
  return authedRequest<ConversationDefaultModelCandidateDTO>(
    "/api/v1/conversations/default-model-candidate",
    {
      accessToken,
    },
    true,
  );
}

export async function listConversationProjects(
  accessToken: string,
  options: ListConversationProjectsOptions = {},
): Promise<ConversationProjectDTO[]> {
  const status = options.status?.trim() || "active";
  return authedRequest<ConversationProjectDTO[]>(
    `/api/v1/conversation-projects?status=${encodeURIComponent(status)}`,
    {
      accessToken,
    },
    true,
  );
}

export async function createConversationProject(
  accessToken: string,
  payload: CreateConversationProjectRequest,
): Promise<ConversationProjectDTO> {
  return authedRequest<ConversationProjectDTO>(
    "/api/v1/conversation-projects",
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function updateConversationProject(
  accessToken: string,
  projectPublicID: string,
  payload: UpdateConversationProjectRequest,
): Promise<ConversationProjectDTO> {
  return authedRequest<ConversationProjectDTO>(
    `/api/v1/conversation-projects/${pathParam(projectPublicID)}`,
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function deleteConversationProject(
  accessToken: string,
  projectPublicID: string,
  options: DeleteConversationProjectOptions = {},
): Promise<DeleteConversationData> {
  const params = new URLSearchParams();
  if (options.deleteConversations) {
    params.set("delete_conversations", "true");
  }
  if (options.deleteFiles) {
    params.set("delete_files", "true");
  }
  const query = params.toString();
  return authedRequest<DeleteConversationData>(
    `/api/v1/conversation-projects/${pathParam(projectPublicID)}${query ? `?${query}` : ""}`,
    {
      method: "DELETE",
      accessToken,
    },
    true,
  );
}

export async function reorderConversationProjects(
  accessToken: string,
  payload: ReorderConversationProjectsRequest,
): Promise<ConversationProjectDTO[]> {
  return authedRequest<ConversationProjectDTO[]>(
    "/api/v1/conversation-projects/reorder",
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function setConversationProject(
  accessToken: string,
  conversationPublicID: string,
  payload: SetConversationProjectRequest,
): Promise<ConversationDTO> {
  return authedRequest<ConversationDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/project`,
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function batchSetConversationProject(
  accessToken: string,
  payload: BatchSetConversationProjectRequest,
): Promise<BatchSetConversationProjectResult> {
  return authedRequest<BatchSetConversationProjectResult>(
    "/api/v1/conversations/project",
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function createConversation(
  accessToken: string,
  payload: CreateConversationRequest,
): Promise<ConversationDTO> {
  return authedRequest<ConversationDTO>(
    "/api/v1/conversations",
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function getConversation(
  accessToken: string,
  conversationPublicID: string,
): Promise<ConversationDTO> {
  return authedRequest<ConversationDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}`,
    {
      accessToken,
    },
    true,
  );
}

export async function exportConversation(
  accessToken: string,
  conversationPublicID: string,
): Promise<ConversationExportDTO> {
  return authedRequest<ConversationExportDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/export`,
    {
      accessToken,
    },
    true,
  );
}

export async function exportAllConversations(accessToken: string): Promise<Blob> {
  const response = await authedFetch("/api/v1/conversations/export", { accessToken });
  if (!response.ok) {
    throw new Error(`export failed: ${response.status}`);
  }
  return response.blob();
}

export async function renameConversation(
  accessToken: string,
  conversationPublicID: string,
  payload: RenameConversationRequest,
): Promise<ConversationDTO> {
  return authedRequest<ConversationDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/title`,
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function updateConversationLabels(
  accessToken: string,
  conversationPublicID: string,
  payload: UpdateConversationLabelsRequest,
): Promise<ConversationDTO> {
  return authedRequest<ConversationDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/labels`,
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function regenerateConversationTitle(
  accessToken: string,
  conversationPublicID: string,
): Promise<ConversationDTO> {
  return authedRequest<ConversationDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/title/regenerate`,
    {
      method: "POST",
      accessToken,
    },
    true,
  );
}

export async function setConversationStar(
  accessToken: string,
  conversationPublicID: string,
  payload: SetConversationStarRequest,
): Promise<ConversationDTO> {
  return authedRequest<ConversationDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/star`,
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function setConversationArchive(
  accessToken: string,
  conversationPublicID: string,
  payload: SetConversationArchiveRequest,
): Promise<ConversationDTO> {
  return authedRequest<ConversationDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/archive`,
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function deleteConversation(
  accessToken: string,
  conversationPublicID: string,
  options: DeleteConversationOptions = {},
): Promise<DeleteConversationData> {
  const params = new URLSearchParams();
  if (options.deleteFiles) {
    params.set("delete_files", "true");
  }
  const query = params.toString();
  return authedRequest<DeleteConversationData>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}${query ? `?${query}` : ""}`,
    {
      method: "DELETE",
      accessToken,
    },
    true,
  );
}

export async function getConversationShare(
  accessToken: string,
  conversationPublicID: string,
): Promise<ConversationShareDTO> {
  return authedRequest<ConversationShareDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/share`,
    {
      accessToken,
    },
    true,
  );
}

export async function createConversationShare(
  accessToken: string,
  conversationPublicID: string,
  payload: CreateConversationShareRequest = {},
): Promise<ConversationShareDTO> {
  return authedRequest<ConversationShareDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/share`,
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function regenerateConversationShare(
  accessToken: string,
  conversationPublicID: string,
  payload: CreateConversationShareRequest = {},
): Promise<ConversationShareDTO> {
  return authedRequest<ConversationShareDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/share/regenerate`,
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function revokeConversationShare(
  accessToken: string,
  conversationPublicID: string,
): Promise<ConversationShareDTO> {
  return authedRequest<ConversationShareDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/share`,
    {
      method: "DELETE",
      accessToken,
    },
    true,
  );
}

export async function revokeConversationShares(
  accessToken: string,
  payload: RevokeConversationSharesRequest,
): Promise<RevokeConversationSharesResult> {
  return authedRequest<RevokeConversationSharesResult>(
    "/api/v1/conversations/shares/revoke",
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function getSharedConversation(shareID: string): Promise<PublicSharedConversationDTO> {
  return apiRequest<PublicSharedConversationDTO>(
    `/api/v1/shared-conversations/${pathParam(shareID)}`,
  );
}

export async function cloneSharedConversation(
  accessToken: string,
  shareID: string,
): Promise<ConversationDTO> {
  return authedRequest<ConversationDTO>(
    `/api/v1/shared-conversations/${pathParam(shareID)}/clone`,
    {
      method: "POST",
      accessToken,
    },
    true,
  );
}

export async function listConversationRuns(
  accessToken: string,
  conversationPublicID: string,
  options: ListConversationRunsOptions = {},
): Promise<PagePayload<ConversationRunDTO>> {
  const page = options.page && options.page > 0 ? options.page : 1;
  const pageSize = options.pageSize && options.pageSize > 0 ? options.pageSize : 20;
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  const data = await authedRequest<PagePayload<ConversationRunDTO>>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/runs?${params.toString()}`,
    {
      accessToken,
    },
    true,
  );
  return {
    total: data.total ?? 0,
    results: data.results ?? [],
  };
}

export async function getConversationRunStatuses(
  accessToken: string,
  runIDs: string[],
  signal?: AbortSignal,
): Promise<ConversationRunStatusDTO[]> {
  const normalizedRunIDs = Array.from(new Set(runIDs.map((runID) => runID.trim()).filter(Boolean)));
  if (normalizedRunIDs.length === 0) {
    return [];
  }
  const requests: Promise<ConversationRunStatusDTO[]>[] = [];
  for (let index = 0; index < normalizedRunIDs.length; index += 100) {
    requests.push(authedRequest<ConversationRunStatusDTO[]>(
      "/api/v1/conversation-runs/statuses",
      {
        method: "POST",
        accessToken,
        body: { runIDs: normalizedRunIDs.slice(index, index + 100) },
        signal,
      },
      true,
    ));
  }
  return (await Promise.all(requests)).flat();
}

function parseActiveConversationRunEvent(value: unknown): ActiveConversationRunEvent | null {
  if (!isRecord(value)) {
    return null;
  }
  // A snapshot is authoritative (it clears runs it does not list), so a partially
  // readable one is dropped as a whole rather than applied with holes.
  if (value.type === "snapshot") {
    if (!Array.isArray(value.runs)) {
      return null;
    }
    const runs: ActiveConversationRunSnapshot[] = [];
    for (const run of value.runs) {
      const runID = isRecord(run) ? readString(run, "runID") : undefined;
      const conversationPublicID = isRecord(run) ? readString(run, "conversationPublicID") : undefined;
      if (runID === undefined || conversationPublicID === undefined) {
        return null;
      }
      runs.push({ runID, conversationPublicID });
    }
    return { type: "snapshot", runs };
  }
  const runID = readString(value, "runID");
  if ((value.type === "started" || value.type === "finished") && runID !== undefined) {
    return { type: value.type, runID, conversationPublicID: readString(value, "conversationPublicID") };
  }
  return null;
}

export async function streamActiveConversationRuns(
  accessToken: string,
  options: {
    signal?: AbortSignal;
    onEvent: (event: ActiveConversationRunEvent) => void;
  },
): Promise<void> {
  const response = await authedFetch(
    "/api/v1/conversation-runs/stream",
    {
      accessToken,
      headers: { Accept: "text/event-stream" },
      signal: options.signal,
    },
    true,
  );
  if (!response.body) {
    throw new ApiError("active conversation run stream is unavailable", response.status);
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  const consumeFrames = (flush: boolean) => {
    buffer += flush ? decoder.decode() : "";
    const frames = buffer.split(/\r?\n\r?\n/);
    buffer = flush ? "" : (frames.pop() ?? "");
    for (const frame of frames) {
      const data = frame
        .split(/\r?\n/)
        .filter((line) => line.startsWith("data:"))
        .map((line) => line.slice(5).trimStart())
        .join("\n")
        .trim();
      if (!data) {
        continue;
      }
      // Malformed or unknown events are ignored to keep the long-lived connection healthy.
      const event = parseActiveConversationRunEvent(parseJSON(data));
      if (event) {
        options.onEvent(event);
      }
    }
  };

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        consumeFrames(true);
        return;
      }
      buffer += decoder.decode(value, { stream: true });
      consumeFrames(false);
    }
  } finally {
    reader.releaseLock();
  }
}

export async function getContextArtifact(
  accessToken: string,
  artifactID: number,
): Promise<ContextArtifactDTO> {
  return authedRequest<ContextArtifactDTO>(
    `/api/v1/context-artifacts/${pathParam(artifactID)}`,
    {
      accessToken,
    },
    true,
  );
}

export async function getConversationToolCallDetail(
  accessToken: string,
  runID: string,
  toolCallID: string,
  signal?: AbortSignal,
): Promise<ConversationToolCallDetailDTO> {
  return authedRequest<ConversationToolCallDetailDTO>(
    `/api/v1/conversation-runs/${pathParam(runID)}/tool-calls/${pathParam(toolCallID)}`,
    {
      accessToken,
      signal,
    },
    true,
  );
}

// Messages
type ListMessagesOptions = {
  page?: number;
  pageSize?: number;
  tail?: boolean;
  beforeID?: number;
};

export async function listMessagesPage(
  accessToken: string,
  conversationPublicID: string,
  options: ListMessagesOptions = {},
): Promise<PagePayload<MessageDTO>> {
  const page = options.page && options.page > 0 ? options.page : 1;
  const pageSize = options.pageSize && options.pageSize > 0 ? options.pageSize : 100;
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  if (options.tail) {
    params.set("tail", "true");
  }
  if (options.beforeID && options.beforeID > 0) {
    params.set("before_id", String(options.beforeID));
  }
  const data = await authedRequest<PagePayload<MessageDTO>>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/messages?${params.toString()}`,
    {
      accessToken,
    },
    true,
  );
  return {
    total: data.total ?? 0,
    results: data.results ?? [],
  };
}

export async function listMessages(
  accessToken: string,
  conversationPublicID: string,
  page = 1,
  pageSize = 100,
): Promise<MessageDTO[]> {
  const data = await listMessagesPage(accessToken, conversationPublicID, {
    page,
    pageSize,
    tail: page === 1,
  });
  return data.results;
}

export async function sendMessage(
  accessToken: string,
  conversationPublicID: string,
  payload: SendMessageRequest,
): Promise<SendMessageResult> {
  return authedRequest<SendMessageResult>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/messages`,
    {
      method: "POST",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function cancelMessageGeneration(
  accessToken: string,
  runID: string,
): Promise<{ canceled: boolean }> {
  return authedRequest<{ canceled: boolean }>(
    `/api/v1/conversation-runs/${pathParam(runID)}/cancel`,
    {
      method: "POST",
      accessToken,
    },
    true,
  );
}

export async function resumeMessageGenerationStream(
  accessToken: string,
  runID: string,
  options: ConversationStreamOptions = {},
): Promise<SendMessageResult | null> {
  const afterSeq = options.afterSeq && options.afterSeq > 0 ? Math.floor(options.afterSeq) : 0;
  const requestQuery = {
    snapshot: true,
    ...(afterSeq > 0 ? { after: afterSeq } : {}),
  } satisfies ConversationRuns.StreamList.RequestQuery;
  const query = new URLSearchParams({ snapshot: String(requestQuery.snapshot) });
  if (requestQuery.after !== undefined) {
    query.set("after", String(requestQuery.after));
  }
  const response = await authedFetch(
    `/api/v1/conversation-runs/${pathParam(runID)}/stream?${query.toString()}`,
    {
      method: "GET",
      accessToken,
      signal: options.signal,
    },
    true,
  );

  if (!response.body) {
    return null;
  }

  const { completed, moderationBlocked } = await readConversationStream(response, options);
  if (moderationBlocked) {
    throw new ApiError(
      "content blocked by moderation",
      response.status,
      {
        eventID: moderationBlocked.eventID,
        direction: moderationBlocked.direction,
        categories: moderationBlocked.categories,
      },
      "content_moderation.blocked",
    );
  }
  return completed;
}

export async function setMessageFeedback(
  accessToken: string,
  messagePublicID: string,
  payload: SetMessageFeedbackRequest,
): Promise<MessageFeedbackResult> {
  return authedRequest<MessageFeedbackResult>(
    `/api/v1/messages/${pathParam(messagePublicID)}/feedback`,
    {
      method: "PUT",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function updateMessage(
  accessToken: string,
  messagePublicID: string,
  payload: UpdateMessageRequest,
): Promise<MessageDTO> {
  return authedRequest<MessageDTO>(
    `/api/v1/messages/${pathParam(messagePublicID)}`,
    {
      method: "PATCH",
      accessToken,
      body: payload,
    },
    true,
  );
}

export async function forkConversationFromMessage(
  accessToken: string,
  conversationPublicID: string,
  messagePublicID: string,
): Promise<ConversationDTO> {
  return authedRequest<ConversationDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/messages/${pathParam(messagePublicID)}/fork`,
    {
      method: "POST",
      accessToken,
    },
    true,
  );
}

export async function deleteConversationMessage(
  accessToken: string,
  conversationPublicID: string,
  messagePublicID: string,
): Promise<MessageDeleteDTO> {
  return authedRequest<MessageDeleteDTO>(
    `/api/v1/conversations/${pathParam(conversationPublicID)}/messages/${pathParam(messagePublicID)}`,
    {
      method: "DELETE",
      accessToken,
    },
    true,
  );
}

export type CompactDoneEvent = {
  method: string;
  freed_tokens: number;
  kept_turns: number;
  summary_preview: string;
};

export type ConversationStreamOptions = {
  signal?: AbortSignal;
  afterSeq?: number;
  onEventSeq?: (seq: number) => void;
  onDelta?: (delta: string) => void;
  onTextSnapshot?: (content: string) => void;
  onFileProc?: (message: string) => void;
  onRagSearch?: (message: string) => void;
  onMediaStatus?: (event: Extract<StreamMessageEvent, { type: "media_status" }>) => void;
  onMediaImageDelta?: (event: Extract<StreamMessageEvent, { type: "media_image_delta" }>) => void;
  onCompactDone?: (event: CompactDoneEvent) => void;
  onProcessUpdate?: (event: Extract<StreamMessageEvent, { type: "process_update" }>) => void;
  onUpstreamThinkDelta?: (event: Extract<StreamMessageEvent, { type: "upstream_think_delta" }>) => void;
  onUsage?: (event: Extract<StreamMessageEvent, { type: "usage" }>) => void;
  onInterrupted?: (event: Extract<StreamMessageEvent, { type: "error" }>) => void;
  onTerminal?: (event: Extract<StreamMessageEvent, { type: "completed" | "error" | "moderation_blocked" }>) => void;
  onModerationChecking?: (event: Extract<StreamMessageEvent, { type: "moderation_checking" }>) => void;
  onModerationBlocked?: (event: Extract<StreamMessageEvent, { type: "moderation_blocked" }>) => void;
};

type StreamReadResult = {
  completed: SendMessageResult | null;
  moderationBlocked: Extract<StreamMessageEvent, { type: "moderation_blocked" }> | null;
};

async function readConversationStream(
  response: Response,
  options: ConversationStreamOptions,
): Promise<StreamReadResult> {
  if (!response.body) {
    return { completed: null, moderationBlocked: null };
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let completed: SendMessageResult | null = null;
  let moderationBlocked: Extract<StreamMessageEvent, { type: "moderation_blocked" }> | null = null;

  const consumeEvent = (event: ParsedStreamEvent) => {
    if (event.type === "moderation_blocked") {
      moderationBlocked = event;
    }
    const nextCompleted = handleStreamEvent(event, options, response.status);
    if (nextCompleted) {
      completed = nextCompleted;
    }
  };

  while (true) {
    let readResult: ReadableStreamReadResult<Uint8Array>;
    try {
      readResult = await reader.read();
    } catch (error) {
      if (options.signal?.aborted) {
        throw new DOMException("Aborted", "AbortError");
      }
      throw error;
    }

    const { done, value } = readResult;
    buffer += decoder.decode(value ?? new Uint8Array(), { stream: !done });

    const { documents, remainder } = extractJSONDocuments(buffer);
    buffer = remainder;

    for (const document of documents) {
      consumeEvent(normalizeStreamEvent(JSON.parse(document)));
    }

    if (done) {
      break;
    }
  }

  const tail = buffer.trim();
  if (tail) {
    consumeEvent(normalizeStreamEvent(JSON.parse(tail)));
  }

  return { completed, moderationBlocked };
}

async function postMessageStream<TPayload>(
  accessToken: string,
  endpoint: string,
  payload: TPayload,
  options: ConversationStreamOptions,
  cache?: RequestCache,
): Promise<SendMessageResult> {
  return postMessageStreamRequest(accessToken, endpoint, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
    signal: options.signal,
  }, options, cache);
}

async function postMessageStreamRequest(
  accessToken: string,
  endpoint: string,
  request: RequestInit,
  options: ConversationStreamOptions,
  cache?: RequestCache,
): Promise<SendMessageResult> {
  const { signal, ...requestWithoutSignal } = request;
  const response = await authedFetch(endpoint, {
    ...requestWithoutSignal,
    accessToken,
    signal: signal ?? undefined,
    cache,
  }, true);

  if (!response.body) {
    throw new ApiError("stream body is empty", response.status);
  }

  const { completed, moderationBlocked } = await readConversationStream(response, options);
  if (moderationBlocked) {
    throw new ApiError(
      "content blocked by moderation",
      response.status,
      {
        eventID: moderationBlocked.eventID,
        direction: moderationBlocked.direction,
        categories: moderationBlocked.categories,
      },
      "content_moderation.blocked",
    );
  }
  if (completed) {
    return completed;
  }
  throw new ApiError("stream completed without final payload", response.status);
}

async function postConversationStream<TPayload>(
  accessToken: string,
  conversationPublicID: string,
  endpointSuffix: string,
  payload: TPayload,
  options: ConversationStreamOptions,
): Promise<SendMessageResult> {
  return postMessageStream(
    accessToken,
    `/api/v1/conversations/${pathParam(conversationPublicID)}${endpointSuffix}`,
    payload,
    options,
  );
}

export async function streamMessage(
  accessToken: string,
  conversationPublicID: string,
  payload: SendMessageRequest,
  options: ConversationStreamOptions = {},
): Promise<SendMessageResult> {
  return postConversationStream(accessToken, conversationPublicID, "/messages/stream", payload, options);
}

export async function streamTemporaryChatMessage(
  accessToken: string,
  payload: TemporaryChatMessageRequest,
  options: ConversationStreamOptions = {},
  attachments: TemporaryChatRequestAttachment[] = [],
): Promise<SendMessageResult> {
  if (attachments.length > TEMPORARY_CHAT_MAX_ATTACHMENTS) {
    throw new ApiError(`temporary chat supports at most ${TEMPORARY_CHAT_MAX_ATTACHMENTS} attachments`, 400);
  }
  if (attachments.filter((item) => item.kind === "image").length > TEMPORARY_CHAT_MAX_IMAGE_ATTACHMENTS) {
    throw new ApiError(`temporary chat supports at most ${TEMPORARY_CHAT_MAX_IMAGE_ATTACHMENTS} image attachments`, 400);
  }
  if (attachments.length > 0) {
    const body = new FormData();
    body.append("payload", JSON.stringify(payload));
    body.append("attachmentMessageIndexes", JSON.stringify(attachments.map((item) => item.messageIndex)));
    for (const attachment of attachments) {
      body.append("attachments", attachment.file, attachment.file.name);
    }
    return postMessageStreamRequest(
      accessToken,
      "/api/v1/temporary-chat/messages/stream",
      { method: "POST", body, signal: options.signal },
      options,
      "no-store",
    );
  }
  return postMessageStream(
    accessToken,
    "/api/v1/temporary-chat/messages/stream",
    payload,
    options,
    "no-store",
  );
}

export async function streamImageGeneration(
  accessToken: string,
  conversationPublicID: string,
  payload: MediaImageRequest,
  options: ConversationStreamOptions = {},
): Promise<SendMessageResult> {
  return postConversationStream(
    accessToken,
    conversationPublicID,
    "/media/images/generations/stream",
    payload,
    options,
  );
}

export async function streamImageEdit(
  accessToken: string,
  conversationPublicID: string,
  payload: MediaImageRequest,
  options: ConversationStreamOptions = {},
): Promise<SendMessageResult> {
  return postConversationStream(
    accessToken,
    conversationPublicID,
    "/media/images/edits/stream",
    payload,
    options,
  );
}

export async function streamVideoGeneration(
  accessToken: string,
  conversationPublicID: string,
  payload: MediaVideoRequest,
  options: ConversationStreamOptions = {},
): Promise<SendMessageResult> {
  return postConversationStream(
    accessToken,
    conversationPublicID,
    "/media/videos/generations/stream",
    payload,
    options,
  );
}

export async function streamVideoExtension(
  accessToken: string,
  conversationPublicID: string,
  payload: MediaVideoExtensionRequest,
  options: ConversationStreamOptions = {},
): Promise<SendMessageResult> {
  return postConversationStream(
    accessToken,
    conversationPublicID,
    "/media/videos/extensions/stream",
    payload,
    options,
  );
}
