import type {
  Admin,
  ContentModerationConfigDataResponse,
  ContentModerationConfigUpdateDataResponse,
  ContentModerationDailyStatResponse,
  ContentModerationEventDetailResponse,
  ContentModerationEventListDataResponse,
  ContentModerationEventResponse,
  ContentModerationProbeResponse,
  ContentModerationServiceConfigResponse,
  ContentModerationStatsDataResponse,
  ContentModerationUpdateConfigRequest,
} from "@deeix/api-contract";

import { authedFetch, authedRequest } from "@/shared/api/authed-client";

export type ContentModerationConfig = ContentModerationServiceConfigResponse;
export type DailyStat = ContentModerationDailyStatResponse;
export type ModerationEvent = ContentModerationEventResponse;
export type ContentModerationEventDetail = ContentModerationEventDetailResponse;

type ContentModerationEventListQuery = Admin.ContentModerationEventsList.RequestQuery;

export async function getContentModerationConfig(accessToken: string) {
  return authedRequest<ContentModerationConfigDataResponse>(
    "/api/v1/admin/content-moderation/config",
    { method: "GET", accessToken },
    true,
  );
}

export async function updateContentModerationConfig(
  accessToken: string,
  payload: ContentModerationUpdateConfigRequest,
) {
  return authedRequest<ContentModerationConfigUpdateDataResponse>(
    "/api/v1/admin/content-moderation/config",
    { method: "PUT", accessToken, body: payload },
    true,
  );
}

export async function probeContentModeration(accessToken: string) {
  return authedRequest<ContentModerationProbeResponse>(
    "/api/v1/admin/content-moderation/probe",
    { method: "POST", accessToken },
    true,
  );
}

export async function getContentModerationStats(accessToken: string) {
  return authedRequest<ContentModerationStatsDataResponse>(
    "/api/v1/admin/content-moderation/stats",
    { method: "GET", accessToken },
    true,
  );
}

export async function listContentModerationEvents(
  accessToken: string,
  params: ContentModerationEventListQuery = {},
) {
  const query = new URLSearchParams();
  if (params.page) query.set("page", String(params.page));
  if (params.pageSize) query.set("pageSize", String(params.pageSize));
  if (params.query) query.set("query", params.query);
  if (params.result) query.set("result", params.result);
  if (params.direction) query.set("direction", params.direction);
  if (params.modality) query.set("modality", params.modality);
  if (params.category) query.set("category", params.category);
  if (params.userId) query.set("userId", String(params.userId));
  if (params.runId) query.set("runId", params.runId);
  if (params.from) query.set("from", params.from);
  if (params.to) query.set("to", params.to);
  const suffix = query.toString() ? `?${query.toString()}` : "";
  return authedRequest<ContentModerationEventListDataResponse>(
    `/api/v1/admin/content-moderation/events${suffix}`,
    { method: "GET", accessToken },
    true,
  );
}

export async function getContentModerationEvent(accessToken: string, eventID: string) {
  return authedRequest<ContentModerationEventDetail>(
    `/api/v1/admin/content-moderation/events/${encodeURIComponent(eventID)}`,
    { method: "GET", accessToken },
    true,
  );
}

export async function fetchContentModerationEventImage(
  accessToken: string,
  eventID: string,
  index: number,
): Promise<{ blob: Blob; mimeType: string }> {
  const response = await authedFetch(
    `/api/v1/admin/content-moderation/events/${encodeURIComponent(eventID)}/images/${index}`,
    {
      method: "GET",
      accessToken,
      cache: "no-store",
    },
  );
  const mimeType = response.headers.get("Content-Type") || "image/png";
  const blob = await response.blob();
  return { blob, mimeType };
}
