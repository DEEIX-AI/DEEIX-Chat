// Public entry of the announcement entity; code outside entities/announcement/ imports only this file.
// Unused re-exports are tree-shaken (package.json "sideEffects"), so UI and logic share one entry.
export {
  dispatchAnnouncementUnreadChanged,
  dispatchOpenAnnouncements,
  getAnnouncementUnread,
  subscribeAnnouncementUnreadChanged,
  subscribeOpenAnnouncements,
} from "@/entities/announcement/events/announcement-events";
