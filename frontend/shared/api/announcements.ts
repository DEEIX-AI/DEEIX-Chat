import type { Announcements } from "@deeix/api-contract";
import { authedRequest } from "@/shared/api/authed-client";
import type { AnnouncementDTO } from "@/shared/api/announcements.types";

type ListAnnouncementsOptions = {
  includeDismissed?: boolean;
};

export async function listAnnouncements(accessToken: string, options: ListAnnouncementsOptions = {}): Promise<AnnouncementDTO[]> {
  const path = options.includeDismissed ? "/api/v1/announcements?include_dismissed=true" : "/api/v1/announcements";
  return authedRequest<AnnouncementDTO[]>(path, { accessToken }, true);
}

export async function dismissAnnouncementToday(accessToken: string, announcementID: number, updatedAt: string): Promise<void> {
  const body: Announcements.DismissTodayCreate.RequestBody = { updatedAt };
  await authedRequest<NonNullable<Announcements.DismissTodayCreate.ResponseBody["data"]>>(
    `/api/v1/announcements/${encodeURIComponent(String(announcementID))}/dismiss-today`,
    { method: "POST", accessToken, body },
    true,
  );
}

export async function closeAnnouncement(accessToken: string, announcementID: number, updatedAt: string): Promise<void> {
  const body: Announcements.CloseCreate.RequestBody = { updatedAt };
  await authedRequest<NonNullable<Announcements.CloseCreate.ResponseBody["data"]>>(
    `/api/v1/announcements/${encodeURIComponent(String(announcementID))}/close`,
    { method: "POST", accessToken, body },
    true,
  );
}
