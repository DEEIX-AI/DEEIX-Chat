import { authedFetch, authedRequest } from "@/shared/api/authed-client";
import { apiRequest, ApiError, pathParam } from "@/shared/api/http-client";
import type { PagePayload } from "@/shared/api/common.types";
import type {
  ConversationDTO,
  ConversationExportDTO,
  ConversationProjectDTO,
  ConversationProjectFilter,
  ConversationProjectStatusFilter,
  ConversationShareDTO,
  ConversationRunDTO,
  ConversationShareFilter,
  ConversationStarredFilter,
  ConversationStatusFilter,
  ContextArtifactDTO,
  CreateConversationProjectRequest,
  CreateConversationRequest,
  CreateConversationShareRequest,
  BatchSetConversationProjectRequest,
  BatchSetConversationProjectResult,
  DeleteConversationData,
  MessageDTO,
  MessageFeedbackResult,
  MessageProcessTraceDTO,
  PublicSharedConversationDTO,
  RenameConversationRequest,
  RevokeConversationSharesRequest,
  RevokeConversationSharesResult,
  ReorderConversationProjectsRequest,
  SendMessageRequest,
  MediaImageRequest,
  SendMessageResult,
  SetConversationArchiveRequest,
  SetConversationProjectRequest,
  SetConversationStarRequest,
  SetMessageFeedbackRequest,
  UpdateMessageRequest,
  UpdateConversationProjectRequest,
  StreamMessageEvent,
  TraceBlockDTO,
} from "@/shared/api/conversation.types";

type RawTraceBlock = {
  title?: string;
  summary?: string;
  contentMarkdown?: string;
  status?: string;
  stage?: string;
  roundID?: string;
  parentEventID?: string;
  updatedAt?: string;
  payloadJSON?: string;
};

type RawProcessTrace = {
  enabled?: boolean;
  status?: string;
  process?: RawTraceBlock;
  tools?: RawTraceBlock;
  upstreamThink?: RawTraceBlock;
  promptTrace?: MessageProcessTraceDTO["promptTrace"];
  events?: RawTraceEvent[];
};

type RawTraceEvent = {
  eventID?: string;
  eventType?: string;
  phase?: string;
  stage?: string;
  roundID?: string;
  parentEventID?: string;
  title?: string;
  summary?: string;
  contentMarkdown?: string;
  status?: string;
  seq?: number;
  startedAt?: string;
  endedAt?: string;
  updatedAt?: string;
  payloadJSON?: string;
};

function normalizeTraceBlock(block: unknown): TraceBlockDTO | undefined {
  if (!block || typeof block !== "object") {
    return undefined;
  }
  const raw = block as RawTraceBlock;
  return {
    title: raw.title ?? "",
    summary: raw.summary ?? "",
    contentMarkdown: raw.contentMarkdown ?? "",
    status: raw.status ?? "",
    stage: raw.stage,
    roundID: raw.roundID,
    parentEventID: raw.parentEventID,
    updatedAt: raw.updatedAt ?? "",
    payloadJSON: raw.payloadJSON,
  };
}

function normalizeTraceEvent(event: unknown) {
  if (!event || typeof event !== "object") {
    return undefined;
  }
  const raw = event as RawTraceEvent;
  return {
    eventID: raw.eventID ?? "",
    eventType: raw.eventType ?? "",
    phase: raw.phase ?? "",
    stage: raw.stage,
    roundID: raw.roundID,
    parentEventID: raw.parentEventID,
    title: raw.title ?? "",
    summary: raw.summary ?? "",
    contentMarkdown: raw.contentMarkdown ?? "",
    status: raw.status ?? "",
    seq: raw.seq ?? 0,
    startedAt: raw.startedAt ?? "",
    endedAt: raw.endedAt,
    updatedAt: raw.updatedAt ?? "",
    payloadJSON: raw.payloadJSON,
  };
}

function normalizeProcessTrace(trace: unknown): MessageProcessTraceDTO | undefined {
  if (!trace || typeof trace !== "object") {
    return undefined;
  }
  const raw = trace as RawProcessTrace;
  return {
    enabled: Boolean(raw.enabled),
    status: raw.status ?? "",
    process: normalizeTraceBlock(raw.process),
    tools: normalizeTraceBlock(raw.tools),
    upstreamThink: normalizeTraceBlock(raw.upstreamThink),
    promptTrace: raw.promptTrace,
    events: Array.isArray(raw.events) ? raw.events.map(normalizeTraceEvent).filter((event): event is NonNullable<ReturnType<typeof normalizeTraceEvent>> => Boolean(event)) : undefined,
  };
}

function normalizeStreamEvent(rawEvent: unknown): StreamMessageEvent {
  if (!rawEvent || typeof rawEvent !== "object") {
    throw new ApiError("stream event is invalid", 500);
  }

  const event = rawEvent as StreamMessageEvent & {
    block?: unknown;
    trace?: unknown;
  };

  if (event.type === "process_update" || event.type === "upstream_think_delta") {
    return {
      ...event,
      block: normalizeTraceBlock(event.block),
      trace: normalizeProcessTrace(event.trace),
    };
  }

  return event;
}

function streamEventSeq(event: StreamMessageEvent): number {
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

function handleStreamEvent(event: StreamMessageEvent, options: ConversationStreamOptions, responseStatus: number): SendMessageResult | null {
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
    options.onDelta?.(event.delta);
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

  if (event.type === "completed") {
    return event.data;
  }

  if (event.type === "error" && event.data) {
    options.onInterrupted?.(event);
    return event.data;
  }

  throw new ApiError(event.message || "stream failed", responseStatus, event.debug, event.errorCode);
}

type ListConversationsOptions = {
  page?: number;
  pageSize?: number;
  status?: ConversationStatusFilter;
  starred?: ConversationStarredFilter;
  share?: ConversationShareFilter;
  project?: ConversationProjectFilter;
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
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
    status,
    starred,
    share,
    project,
  });
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
  const afterQuery = afterSeq > 0 ? `?after=${afterSeq}` : "";
  const response = await authedFetch(
    `/api/v1/conversation-runs/${pathParam(runID)}/stream${afterQuery}`,
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

  return readConversationStream(response, options);
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
  onFileProc?: (message: string) => void;
  onRagSearch?: (message: string) => void;
  onMediaStatus?: (event: Extract<StreamMessageEvent, { type: "media_status" }>) => void;
  onMediaImageDelta?: (event: Extract<StreamMessageEvent, { type: "media_image_delta" }>) => void;
  onCompactDone?: (event: CompactDoneEvent) => void;
  onProcessUpdate?: (event: Extract<StreamMessageEvent, { type: "process_update" }>) => void;
  onUpstreamThinkDelta?: (event: Extract<StreamMessageEvent, { type: "upstream_think_delta" }>) => void;
  onUsage?: (event: Extract<StreamMessageEvent, { type: "usage" }>) => void;
  onInterrupted?: (event: Extract<StreamMessageEvent, { type: "error" }>) => void;
};

async function readConversationStream(
  response: Response,
  options: ConversationStreamOptions,
): Promise<SendMessageResult | null> {
  if (!response.body) {
    return null;
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let completed: SendMessageResult | null = null;

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
      const event = normalizeStreamEvent(JSON.parse(document));
      const nextCompleted = handleStreamEvent(event, options, response.status);
      if (nextCompleted) {
        completed = nextCompleted;
      }
    }

    if (done) {
      break;
    }
  }

  const tail = buffer.trim();
  if (tail) {
    const event = normalizeStreamEvent(JSON.parse(tail));
    const nextCompleted = handleStreamEvent(event, options, response.status);
    if (nextCompleted) {
      completed = nextCompleted;
    }
  }

  return completed;
}

async function postConversationStream<TPayload>(
  accessToken: string,
  conversationPublicID: string,
  endpointSuffix: string,
  payload: TPayload,
  options: ConversationStreamOptions,
): Promise<SendMessageResult> {
  const response = await authedFetch(
    `/api/v1/conversations/${pathParam(conversationPublicID)}${endpointSuffix}`,
    {
      method: "POST",
      accessToken,
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
      signal: options.signal,
    },
    true,
  );

  if (!response.body) {
    throw new ApiError("stream body is empty", response.status);
  }

  const completed = await readConversationStream(response, options);
  if (!completed) {
    throw new ApiError("stream completed without final payload", response.status);
  }
  return completed;
}

export async function streamMessage(
  accessToken: string,
  conversationPublicID: string,
  payload: SendMessageRequest,
  options: ConversationStreamOptions = {},
): Promise<SendMessageResult> {
  return postConversationStream(accessToken, conversationPublicID, "/messages/stream", payload, options);
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
