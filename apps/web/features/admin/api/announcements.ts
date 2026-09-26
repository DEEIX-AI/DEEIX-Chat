import { authedRequest } from "@/shared/api/authed-client";
import { pathParam } from "@/shared/api/http-client";
import type { PagePayload } from "@/shared/api/common.types";
import type {
  AdminAnnouncementDTO,
  AdminAnnouncementData,
  AdminAnnouncementDeleteData,
  AdminAnnouncementPage,
  CreateAdminAnnouncementRequest,
  UpdateAdminAnnouncementRequest,
} from "@/features/admin/api/announcements.types";
import { normalizeAdminPagePayload, resolveAdminPage, type AdminListQueryOptions } from "./shared";

export async function listAdminAnnouncements(
  accessToken: string,
  options: AdminListQueryOptions = {},
): Promise<AdminAnnouncementPage> {
  const { page, pageSize } = resolveAdminPage(options);
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  if (options.query?.trim()) params.set("q", options.query.trim());
  if (options.status?.trim()) params.set("status", options.status.trim());
  if (options.type?.trim()) params.set("type", options.type.trim());
  if (options.pinned?.trim()) params.set("pinned", options.pinned.trim());
  const data = await authedRequest<PagePayload<AdminAnnouncementDTO>>(
    `/api/v1/admin/announcements?${params.toString()}`,
    { accessToken },
    true,
  );
  return normalizeAdminPagePayload(data);
}

export async function createAdminAnnouncement(
  accessToken: string,
  payload: CreateAdminAnnouncementRequest,
): Promise<AdminAnnouncementData> {
  return authedRequest<AdminAnnouncementData>(
    "/api/v1/admin/announcements",
    { method: "POST", accessToken, body: payload },
    true,
  );
}

export async function updateAdminAnnouncement(
  accessToken: string,
  announcementID: number,
  payload: UpdateAdminAnnouncementRequest,
): Promise<AdminAnnouncementData> {
  return authedRequest<AdminAnnouncementData>(
    `/api/v1/admin/announcements/${pathParam(announcementID)}`,
    { method: "PATCH", accessToken, body: payload },
    true,
  );
}

export async function deleteAdminAnnouncement(
  accessToken: string,
  announcementID: number,
): Promise<AdminAnnouncementDeleteData> {
  return authedRequest<AdminAnnouncementDeleteData>(
    `/api/v1/admin/announcements/${pathParam(announcementID)}`,
    { method: "DELETE", accessToken },
    true,
  );
}
