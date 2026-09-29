import type { AnnouncementDTO } from "@/shared/api/announcements-types";

export const ANNOUNCEMENT_SORT_MODES = ["default", "type", "time"] as const;
export type AnnouncementSortMode = (typeof ANNOUNCEMENT_SORT_MODES)[number];
type AnnouncementType = "critical" | "warning" | "info" | "normal" | "general";

export function normalizeAnnouncementType(value: string): AnnouncementType {
  switch (value) {
    case "critical":
    case "warning":
    case "info":
    case "normal":
    case "general":
      return value;
    default:
      return "general";
  }
}

function announcementTypeRank(value: string): number {
  switch (normalizeAnnouncementType(value)) {
    case "critical":
      return 5;
    case "warning":
      return 4;
    case "info":
      return 3;
    case "normal":
      return 2;
    default:
      return 1;
  }
}

function announcementTime(value: string): number {
  const time = new Date(value).getTime();
  return Number.isNaN(time) ? 0 : time;
}

export function isAnnouncementRead(item: AnnouncementDTO): boolean {
  return Boolean(item.closedAt);
}

function compareReadState(a: AnnouncementDTO, b: AnnouncementDTO): number {
  return Number(isAnnouncementRead(a)) - Number(isAnnouncementRead(b));
}

function compareAnnouncementByTime(a: AnnouncementDTO, b: AnnouncementDTO): number {
  return announcementTime(b.updatedAt) - announcementTime(a.updatedAt) || b.id - a.id;
}

function compareAnnouncementByType(a: AnnouncementDTO, b: AnnouncementDTO): number {
  return announcementTypeRank(b.type) - announcementTypeRank(a.type) || compareAnnouncementByTime(a, b);
}

export function sortAnnouncements(queue: AnnouncementDTO[], sortMode: AnnouncementSortMode): AnnouncementDTO[] {
  if (sortMode === "time") {
    return [...queue].sort((a, b) => compareReadState(a, b) || compareAnnouncementByTime(a, b));
  }
  if (sortMode === "type") {
    return [...queue].sort((a, b) => compareReadState(a, b) || compareAnnouncementByType(a, b));
  }
  return queue;
}
