import { useTranslations } from "next-intl";
import * as React from "react";
import { toast } from "sonner";

import {
  ANNOUNCEMENT_SORT_MODES,
  type AnnouncementSortMode,
  isAnnouncementRead,
  sortAnnouncements,
} from "@/features/announcements/model/announcement-order";
import { closeAnnouncement, dismissAnnouncementToday, listAnnouncements } from "@/shared/api/announcements";
import type { AnnouncementDTO } from "@/shared/api/announcements-types";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { dispatchAnnouncementUnreadChanged, subscribeOpenAnnouncements } from "@/entities/announcement";
import { isOneOf } from "@/shared/lib/type-guards";

export type AnnouncementDialogMode = "auto" | "manual";

const isAnnouncementSortMode = isOneOf(ANNOUNCEMENT_SORT_MODES);

function isSkippedPath(pathname: string | null): boolean {
  if (!pathname) {
    return false;
  }
  return pathname === "/share" || pathname.startsWith("/share/");
}

/**
 * Owns both announcement queues: the automatic one loaded on navigation (unread
 * only) and the manual one opened from the nav (including dismissed items).
 * Each load is guarded by its own request ID so a stale response never
 * overwrites a newer queue.
 */
export function useAnnouncementDialog(pathname: string | null) {
  const t = useTranslations("announcements");
  const { accessToken, user, userStatus } = useAuthSession();
  const [autoQueue, setAutoQueue] = React.useState<AnnouncementDTO[]>([]);
  const [manualQueue, setManualQueue] = React.useState<AnnouncementDTO[]>([]);
  const [activeIndex, setActiveIndex] = React.useState(0);
  const [sortMode, setSortMode] = React.useState<AnnouncementSortMode>("default");
  const [stateSaving, setStateSaving] = React.useState(false);
  const [autoOpen, setAutoOpen] = React.useState(false);
  const [manualOpen, setManualOpen] = React.useState(false);
  const [manualLoading, setManualLoading] = React.useState(false);
  const [dialogMode, setDialogMode] = React.useState<AnnouncementDialogMode>("auto");
  const autoLoadRequestIDRef = React.useRef(0);
  const manualLoadRequestIDRef = React.useRef(0);

  React.useEffect(() => {
    let cancelled = false;
    if (userStatus !== "ready" || !accessToken || user?.initialSecurityRequired || isSkippedPath(pathname)) {
      autoLoadRequestIDRef.current += 1;
      manualLoadRequestIDRef.current += 1;
      setAutoQueue([]);
      setManualQueue([]);
      setActiveIndex(0);
      setAutoOpen(false);
      setManualOpen(false);
      setManualLoading(false);
      setDialogMode("auto");
      return;
    }

    async function load() {
      const requestID = autoLoadRequestIDRef.current + 1;
      autoLoadRequestIDRef.current = requestID;
      try {
        const items = await listAnnouncements(accessToken);
        if (!cancelled && autoLoadRequestIDRef.current === requestID) {
          setAutoQueue(items);
          setAutoOpen(items.some((item) => !isAnnouncementRead(item)));
          setDialogMode((current) => (current === "manual" ? current : "auto"));
          setActiveIndex(0);
        }
      } catch {
        if (!cancelled && autoLoadRequestIDRef.current === requestID) {
          setAutoQueue([]);
          setAutoOpen(false);
          setActiveIndex(0);
        }
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [accessToken, pathname, user?.initialSecurityRequired, userStatus]);

  React.useEffect(() => {
    let cancelled = false;
    const unsubscribe = subscribeOpenAnnouncements(() => {
      if (userStatus !== "ready" || !accessToken || user?.initialSecurityRequired || isSkippedPath(pathname)) {
        return;
      }
      const requestID = manualLoadRequestIDRef.current + 1;
      manualLoadRequestIDRef.current = requestID;
      setDialogMode("manual");
      setAutoOpen(false);
      setManualOpen(true);
      setManualLoading(true);
      setManualQueue([]);
      setActiveIndex(0);
      setSortMode("default");

      void listAnnouncements(accessToken, { includeDismissed: true })
        .then((items) => {
          if (!cancelled && manualLoadRequestIDRef.current === requestID) {
            setManualQueue(items);
            setActiveIndex(0);
          }
        })
        .catch(() => {
          if (!cancelled && manualLoadRequestIDRef.current === requestID) {
            setManualQueue([]);
            toast.error(t("openFailed"));
          }
        })
        .finally(() => {
          if (!cancelled && manualLoadRequestIDRef.current === requestID) {
            setManualLoading(false);
          }
        });
    });

    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, [accessToken, pathname, t, user?.initialSecurityRequired, userStatus]);

  const queue = dialogMode === "manual" ? manualQueue : autoQueue;
  const sortedQueue = React.useMemo(() => sortAnnouncements(queue, sortMode), [queue, sortMode]);

  React.useEffect(() => {
    setActiveIndex(0);
  }, [sortMode]);

  const hasUnread = autoQueue.some((item) => !isAnnouncementRead(item));
  React.useEffect(() => {
    dispatchAnnouncementUnreadChanged(hasUnread);
  }, [hasUnread]);

  const open = manualOpen || autoOpen;
  const unreadQueue = React.useMemo(() => queue.filter((item) => !isAnnouncementRead(item)), [queue]);

  const closeDialog = React.useCallback(() => {
    setActiveIndex(0);
    setAutoOpen(false);
    setManualOpen(false);
    setManualLoading(false);
    dispatchAnnouncementUnreadChanged(false);
  }, []);

  const closeManualDialog = React.useCallback(() => {
    setManualOpen(false);
    setManualLoading(false);
    setActiveIndex(0);
  }, []);

  const hideAutoDialog = React.useCallback(() => {
    setAutoOpen(false);
    setActiveIndex(0);
  }, []);

  const handleOpenChange = React.useCallback((nextOpen: boolean) => {
    if (nextOpen) {
      return;
    }
    if (manualOpen) {
      closeManualDialog();
      return;
    }
    hideAutoDialog();
  }, [closeManualDialog, hideAutoDialog, manualOpen]);

  const handleSortModeChange = React.useCallback((value: string) => {
    if (isAnnouncementSortMode(value)) {
      setSortMode(value);
    }
  }, []);

  const dismissAllToday = React.useCallback(async () => {
    if (!accessToken || stateSaving) {
      return;
    }
    setStateSaving(true);
    try {
      await Promise.all(unreadQueue.map((item) => dismissAnnouncementToday(accessToken, item.id, item.updatedAt)));
      closeDialog();
    } catch {
      toast.error(t("dismissFailed"));
    } finally {
      setStateSaving(false);
    }
  }, [accessToken, closeDialog, stateSaving, t, unreadQueue]);

  const closeAll = React.useCallback(async () => {
    if (!accessToken || stateSaving) {
      return;
    }
    setStateSaving(true);
    try {
      await Promise.all(unreadQueue.map((item) => closeAnnouncement(accessToken, item.id, item.updatedAt)));
      closeDialog();
    } catch {
      toast.error(t("closeFailed"));
    } finally {
      setStateSaving(false);
    }
  }, [accessToken, closeDialog, stateSaving, t, unreadQueue]);

  return {
    open,
    dialogMode,
    sortedQueue,
    activeIndex,
    setActiveIndex,
    sortMode,
    handleSortModeChange,
    manualLoading,
    stateSaving,
    hasUnreadInQueue: unreadQueue.length > 0,
    handleOpenChange,
    closeManualDialog,
    dismissAllToday,
    closeAll,
  };
}
