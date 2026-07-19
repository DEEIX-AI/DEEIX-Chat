import type {
  AnnouncementDataResponse,
  AnnouncementDeleteDataResponse,
  CreateAnnouncementRequest,
  PatchAnnouncementRequestDoc,
} from "@deeix/api-contract";
import type { PagePayload } from "@/shared/api/common.types";
import type { AnnouncementDTO } from "@/shared/api/announcements.types";

export type AdminAnnouncementDTO = AnnouncementDTO;

export type AdminAnnouncementPage = PagePayload<AdminAnnouncementDTO>;

export type CreateAdminAnnouncementRequest = CreateAnnouncementRequest;

export type UpdateAdminAnnouncementRequest = PatchAnnouncementRequestDoc;

export type AdminAnnouncementData = Omit<AnnouncementDataResponse, "announcement"> & {
  announcement: AdminAnnouncementDTO;
};

export type AdminAnnouncementDeleteData = AnnouncementDeleteDataResponse;
