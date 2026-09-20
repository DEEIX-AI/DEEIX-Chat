import { authedFetch, authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";
import type {
  InternalMessagingConversationPage,
  InternalMessagingMessage,
  InternalMessagingMessagePage,
  InternalMessagingStatus,
  InternalMessagingUserPage,
} from "@/shared/api/internal-messaging.types";

const ROOT = "/api/v1/internal-messaging";

export function getInternalMessagingStatus(accessToken: string) {
  return authedRequest<InternalMessagingStatus>(`${ROOT}/status`, { accessToken }, false);
}

export function listInternalMessagingUsers(accessToken: string, query = "", page = 1, signal?: AbortSignal) {
  const params = new URLSearchParams({ page: String(page), page_size: "50" });
  if (query.trim()) params.set("query", query.trim());
  return authedRequest<InternalMessagingUserPage>(`${ROOT}/users?${params}`, { accessToken, signal });
}

export function listInternalMessagingConversations(accessToken: string, page = 1, signal?: AbortSignal) {
  const params = new URLSearchParams({ page: String(page), page_size: "50" });
  return authedRequest<InternalMessagingConversationPage>(`${ROOT}/conversations?${params}`, {
    accessToken, signal,
  });
}

export function listInternalMessagingMessages(
  accessToken: string,
  publicID: string,
  before?: number,
  signal?: AbortSignal,
) {
  const suffix = before ? `?before=${before}` : "";
  return authedRequest<InternalMessagingMessagePage>(
    `${ROOT}/users/${pathParam(publicID)}/messages${suffix}`,
    { accessToken, signal },
  );
}

export function sendInternalMessagingMessage(accessToken: string, publicID: string, content: string) {
  return authedRequest<InternalMessagingMessage>(`${ROOT}/users/${pathParam(publicID)}/messages`, {
    method: "POST",
    accessToken,
    body: { content },
  });
}

export function replyInternalMessagingMessage(accessToken: string, publicID: string, replyToID: number, content: string) {
  return authedRequest<InternalMessagingMessage>(`${ROOT}/users/${pathParam(publicID)}/messages/reply`, {
    method: "POST",
    accessToken,
    body: { content, replyToID },
  });
}

export function editInternalMessagingMessage(accessToken: string, mid: number, content: string) {
  return authedRequest<InternalMessagingMessage>(`${ROOT}/messages/${pathParam(mid)}`, {
    method: "PUT",
    accessToken,
    body: { content },
  });
}

export function deleteInternalMessagingMessage(accessToken: string, mid: number) {
  return authedRequest<{ ok: boolean }>(`${ROOT}/messages/${pathParam(mid)}`, {
    method: "DELETE",
    accessToken,
  });
}

export function sendInternalMessagingFile(accessToken: string, publicID: string, file: File) {
  const body = new FormData();
  body.append("file", file);
  return authedRequest<InternalMessagingMessage>(`${ROOT}/users/${pathParam(publicID)}/files`, {
    method: "POST",
    accessToken,
    body,
  });
}

export async function downloadInternalMessagingFile(
  accessToken: string,
  mid: number,
  options: { thumbnail?: boolean; download?: boolean } = {},
) {
  const params = new URLSearchParams();
  if (options.thumbnail) params.set("thumbnail", "true");
  if (options.download) params.set("download", "true");
  const suffix = params.size ? `?${params}` : "";
  const response = await authedFetch(`${ROOT}/messages/${pathParam(mid)}/file${suffix}`, {
    method: "GET",
    accessToken,
  });
  return response.blob();
}

export function markInternalMessagingRead(accessToken: string, publicID: string, throughMID: number) {
  return authedRequest<{ ok: boolean }>(`${ROOT}/users/${pathParam(publicID)}/read`, {
    method: "PUT",
    accessToken,
    body: { throughMID },
  });
}

export function updateInternalMessagingPreferences(
  accessToken: string,
  publicID: string,
  preferences: { pinned?: boolean; muted?: boolean },
) {
  return authedRequest<{ ok: boolean }>(`${ROOT}/users/${pathParam(publicID)}/preferences`, {
    method: "PATCH",
    accessToken,
    body: preferences,
  });
}

export function searchInternalMessagingMessages(
  accessToken: string,
  query: string,
  publicID = "",
  before?: number,
) {
  const params = new URLSearchParams({ query, limit: "50" });
  if (publicID) params.set("public_id", publicID);
  if (before) params.set("before", String(before));
  return authedRequest<InternalMessagingMessagePage>(`${ROOT}/messages/search?${params}`, {
    accessToken,
  });
}

export function openInternalMessagingEvents(accessToken: string, after?: number, signal?: AbortSignal) {
  const suffix = after ? `?after=${after}` : "";
  return authedFetch(`${ROOT}/events${suffix}`, { method: "GET", accessToken, signal }, false);
}
