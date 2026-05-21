import { authedRequest } from "@/shared/api/authed-client";
import type { AdminAuditLogDTO, AdminSystemEventDTO, AdminUsageLogDTO, AdminUserAuthEventDTO } from "@/features/admin/api/admin.types";
import type { PagePayload } from "@/shared/api/common.types";

import { normalizeAdminPagePayload, resolveAdminPage, type AdminPageOptions } from "./shared";

type ListAdminUserAuthEventsOptions = AdminPageOptions & {
  userID?: number;
  eventType?: string;
  result?: string;
};

type ListAdminAuditLogsOptions = AdminPageOptions & {
  query?: string;
  resource?: string;
  action?: string;
  actorUserID?: number;
  createdFrom?: string;
  createdTo?: string;
  sort?: string;
};

type ListAdminSystemEventsOptions = AdminPageOptions & {
  query?: string;
  level?: string;
  source?: string;
  event?: string;
  createdFrom?: string;
  createdTo?: string;
  sort?: string;
};

type ListAdminUsageLogsOptions = AdminPageOptions & {
  query?: string;
  platformModelName?: string;
  billingMode?: string;
  userID?: number;
  createdFrom?: string;
  createdTo?: string;
  sort?: string;
};

export async function listAdminUserAuthEvents(
  accessToken: string,
  options: ListAdminUserAuthEventsOptions = {},
): Promise<PagePayload<AdminUserAuthEventDTO>> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams();
  params.set("page", String(page));
  params.set("page_size", String(pageSize));

  if (options.userID && options.userID > 0) {
    params.set("user_id", String(options.userID));
  }
  if (options.eventType) {
    params.set("event_type", options.eventType);
  }
  if (options.result) {
    params.set("result", options.result);
  }

  const data = await authedRequest<PagePayload<AdminUserAuthEventDTO>>(
    `/api/v1/admin/user-auth-events?${params.toString()}`,
    { accessToken },
    true,
  );

  return normalizeAdminPagePayload(data);
}

export async function listAdminAuditLogs(
  accessToken: string,
  options: ListAdminAuditLogsOptions = {},
): Promise<PagePayload<AdminAuditLogDTO>> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams();
  params.set("page", String(page));
  params.set("page_size", String(pageSize));
  if (options.query?.trim()) {
    params.set("query", options.query.trim());
  }
  if (options.resource?.trim()) {
    params.set("resource", options.resource.trim());
  }
  if (options.action?.trim()) {
    params.set("action", options.action.trim());
  }
  if (options.actorUserID && options.actorUserID > 0) {
    params.set("actor_user_id", String(options.actorUserID));
  }
  if (options.createdFrom?.trim()) {
    params.set("created_from", options.createdFrom.trim());
  }
  if (options.createdTo?.trim()) {
    params.set("created_to", options.createdTo.trim());
  }
  if (options.sort?.trim()) {
    params.set("sort", options.sort.trim());
  }
  const data = await authedRequest<PagePayload<AdminAuditLogDTO>>(
    `/api/v1/admin/audit-logs?${params.toString()}`,
    { accessToken },
    true,
  );

  return normalizeAdminPagePayload(data);
}

export async function listAdminSystemEvents(
  accessToken: string,
  options: ListAdminSystemEventsOptions = {},
): Promise<PagePayload<AdminSystemEventDTO>> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams();
  params.set("page", String(page));
  params.set("page_size", String(pageSize));
  if (options.query?.trim()) params.set("query", options.query.trim());
  if (options.level?.trim()) params.set("level", options.level.trim());
  if (options.source?.trim()) params.set("source", options.source.trim());
  if (options.event?.trim()) params.set("event", options.event.trim());
  if (options.createdFrom?.trim()) params.set("created_from", options.createdFrom.trim());
  if (options.createdTo?.trim()) params.set("created_to", options.createdTo.trim());
  if (options.sort?.trim()) params.set("sort", options.sort.trim());

  const data = await authedRequest<PagePayload<AdminSystemEventDTO>>(
    `/api/v1/admin/system-events?${params.toString()}`,
    { accessToken },
    true,
  );

  return normalizeAdminPagePayload(data);
}

export async function listAdminUsageLogs(
  accessToken: string,
  options: ListAdminUsageLogsOptions = {},
): Promise<PagePayload<AdminUsageLogDTO>> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams();
  params.set("page", String(page));
  params.set("page_size", String(pageSize));
  if (options.query?.trim()) params.set("query", options.query.trim());
  if (options.platformModelName?.trim()) params.set("platform_model_name", options.platformModelName.trim());
  if (options.billingMode?.trim()) params.set("billing_mode", options.billingMode.trim());
  if (options.userID && options.userID > 0) params.set("user_id", String(options.userID));
  if (options.createdFrom?.trim()) params.set("created_from", options.createdFrom.trim());
  if (options.createdTo?.trim()) params.set("created_to", options.createdTo.trim());
  if (options.sort?.trim()) params.set("sort", options.sort.trim());

  const data = await authedRequest<PagePayload<AdminUsageLogDTO>>(
    `/api/v1/admin/call-logs?${params.toString()}`,
    { accessToken },
    true,
  );

  return normalizeAdminPagePayload(data);
}
