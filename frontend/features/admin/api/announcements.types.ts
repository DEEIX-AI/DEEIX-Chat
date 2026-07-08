import type { PagePayload } from "@/shared/api/common.types";
import type { AnnouncementDTO } from "@/shared/api/announcements.types";

export type AdminAnnouncementDTO = AnnouncementDTO;

export type AdminAnnouncementPage = PagePayload<AdminAnnouncementDTO>;

export type CreateAdminAnnouncementRequest = {
  title: string;
  contentMarkdown: string;
  status?: "active" | "inactive";
  type?: "critical" | "warning" | "info" | "normal" | "general";
  pinned?: boolean;
  priority: number;
  startsAt?: string | null;
  expiresAt?: string | null;
};

export type UpdateAdminAnnouncementRequest = Partial<CreateAdminAnnouncementRequest>;

export type AdminAnnouncementData = {
  announcement: AdminAnnouncementDTO;
};

export type GenerateAdminAnnouncementDraftRequest = {
  requirement: string;
  locale?: string;
};

export type AdminAnnouncementDraftDTO = {
  title: string;
  contentMarkdown: string;
  type: "critical" | "warning" | "info" | "normal" | "general" | string;
  pinned: boolean;
  priority: number;
};

export type AdminAnnouncementDraftData = {
  draft: AdminAnnouncementDraftDTO;
};

export type AdminAnnouncementDeleteData = {
  deleted: boolean;
};
