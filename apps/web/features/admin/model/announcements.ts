import { isOneOf } from "@/shared/lib/type-guards";

export const ADMIN_ANNOUNCEMENT_TYPES = ["critical", "warning", "info", "normal", "general"] as const;
export type AdminAnnouncementType = (typeof ADMIN_ANNOUNCEMENT_TYPES)[number];
export const isAdminAnnouncementType = isOneOf(ADMIN_ANNOUNCEMENT_TYPES);

/** Unknown types from the server or a select fall back to "general". */
export function normalizeAdminAnnouncementType(value: string): AdminAnnouncementType {
  return isAdminAnnouncementType(value) ? value : "general";
}
