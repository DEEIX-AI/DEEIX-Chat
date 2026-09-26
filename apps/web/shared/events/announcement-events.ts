"use client";

const OPEN_ANNOUNCEMENTS_EVENT = "deeix-chat:open-announcements";
const ANNOUNCEMENT_UNREAD_CHANGED_EVENT = "deeix-chat:announcement-unread-changed";

let announcementUnread = false;

export function dispatchOpenAnnouncements(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new Event(OPEN_ANNOUNCEMENTS_EVENT));
}

export function dispatchAnnouncementUnreadChanged(hasUnread: boolean): void {
  announcementUnread = hasUnread;
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent<boolean>(ANNOUNCEMENT_UNREAD_CHANGED_EVENT, { detail: hasUnread }));
}

export function getAnnouncementUnread(): boolean {
  return announcementUnread;
}

export function subscribeOpenAnnouncements(handler: () => void): () => void {
  if (typeof window === "undefined") {
    return () => {};
  }
  window.addEventListener(OPEN_ANNOUNCEMENTS_EVENT, handler);
  return () => window.removeEventListener(OPEN_ANNOUNCEMENTS_EVENT, handler);
}

export function subscribeAnnouncementUnreadChanged(handler: (hasUnread: boolean) => void): () => void {
  if (typeof window === "undefined") {
    return () => {};
  }
  const handleUnreadChanged = (event: Event) => {
    handler(Boolean((event as CustomEvent<boolean>).detail));
  };
  window.addEventListener(ANNOUNCEMENT_UNREAD_CHANGED_EVENT, handleUnreadChanged);
  return () => window.removeEventListener(ANNOUNCEMENT_UNREAD_CHANGED_EVENT, handleUnreadChanged);
}
