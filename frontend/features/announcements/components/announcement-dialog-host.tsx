"use client";

import * as React from "react";
import { usePathname } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { Pin } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { StreamdownRender } from "@/shared/components/markdown/streamdown-render";
import { closeAnnouncement, dismissAnnouncementToday, listAnnouncements } from "@/shared/api/announcements";
import type { AnnouncementDTO } from "@/shared/api/announcements.types";
import { useAuthSession } from "@/shared/auth/auth-session-context";
import { dispatchAnnouncementUnreadChanged, subscribeOpenAnnouncements } from "@/shared/events/announcement-events";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { cn } from "@/lib/utils";

type AnnouncementSortMode = "default" | "type" | "time";
type AnnouncementDialogMode = "auto" | "manual";
type AnnouncementType = "critical" | "warning" | "info" | "normal" | "general";

function isSkippedPath(pathname: string | null): boolean {
  if (!pathname) {
    return false;
  }
  return pathname === "/share" || pathname.startsWith("/share/");
}

function formatAnnouncementDate(value: string, locale: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}

function formatAnnouncementTime(value: string, locale: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return new Intl.DateTimeFormat(locale, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

function isAnnouncementSortMode(value: string): value is AnnouncementSortMode {
  return value === "default" || value === "type" || value === "time";
}

function normalizeAnnouncementType(value: string): AnnouncementType {
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

function announcementTypeAccentClassName(value: string): string {
  switch (normalizeAnnouncementType(value)) {
    case "critical":
      return "before:bg-red-500 dark:before:bg-red-400";
    case "warning":
      return "before:bg-yellow-500 dark:before:bg-yellow-400";
    case "info":
      return "before:bg-blue-500 dark:before:bg-blue-400";
    case "normal":
      return "before:bg-emerald-500 dark:before:bg-emerald-400";
    default:
      return "before:bg-border";
  }
}

function announcementTime(value: string): number {
  const time = new Date(value).getTime();
  return Number.isNaN(time) ? 0 : time;
}

function isAnnouncementRead(item: AnnouncementDTO): boolean {
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

export function AnnouncementDialogHost() {
  const t = useTranslations("announcements");
  const locale = useLocale();
  const pathname = usePathname();
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
  const sortedQueue = React.useMemo(() => {
    if (sortMode === "time") {
      return [...queue].sort((a, b) => compareReadState(a, b) || compareAnnouncementByTime(a, b));
    }
    if (sortMode === "type") {
      return [...queue].sort((a, b) => compareReadState(a, b) || compareAnnouncementByType(a, b));
    }
    return queue;
  }, [queue, sortMode]);

  React.useEffect(() => {
    setActiveIndex(0);
  }, [sortMode]);

  const hasUnread = autoQueue.some((item) => !isAnnouncementRead(item));
  React.useEffect(() => {
    dispatchAnnouncementUnreadChanged(hasUnread);
  }, [hasUnread]);

  const open = manualOpen || autoOpen;
  const renderMode = useDialogSnapshot(open ? dialogMode : null) ?? dialogMode;
  const renderQueue = useDialogSnapshot(open ? sortedQueue : null) ?? sortedQueue;
  const renderActiveIndex = useDialogSnapshot(open ? activeIndex : null) ?? activeIndex;
  const renderManualLoading = useDialogSnapshot(open ? manualLoading : null) ?? manualLoading;
  const active = renderQueue[Math.min(renderActiveIndex, Math.max(renderQueue.length - 1, 0))] ?? null;
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

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="flex max-h-[min(84svh,720px)] min-w-0 flex-col overflow-hidden p-4 sm:max-w-[760px] sm:p-5">
        <DialogHeader className="shrink-0">
          <div className="min-w-0">
            <DialogTitle className="truncate">{t("title")}</DialogTitle>
          </div>
        </DialogHeader>
        <div className="grid h-[27rem] max-h-[calc(100svh-11rem)] min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] gap-0 overflow-hidden md:grid-cols-[13rem_minmax(0,1fr)] md:grid-rows-1">
          <div className="flex min-h-0 min-w-0 flex-col border-b border-border/60 md:border-b-0 md:border-r">
            <Tabs value={sortMode} onValueChange={handleSortModeChange} className="min-w-0 shrink-0 px-2 pt-2 pb-1">
              <TabsList className="grid h-7 w-full grid-cols-3">
                <TabsTrigger value="default" className="px-1.5">{t("sort.default")}</TabsTrigger>
                <TabsTrigger value="type" className="px-1.5">{t("sort.type")}</TabsTrigger>
                <TabsTrigger value="time" className="px-1.5">{t("sort.time")}</TabsTrigger>
              </TabsList>
            </Tabs>
            <div className="flex min-w-0 gap-2 overflow-x-auto px-2 py-2 md:block md:min-h-0 md:flex-1 md:space-y-0.5 md:overflow-y-auto">
              {renderQueue.length > 0 ? renderQueue.map((item, index) => (
                <button
                  key={`${item.id}:${item.updatedAt}`}
                  type="button"
                  aria-current={index === renderActiveIndex ? "true" : undefined}
                  className={cn(
                    "relative min-w-36 rounded-md py-1 pl-3.5 pr-8 text-left text-xs transition-colors outline-hidden ring-sidebar-ring focus-visible:ring-2 before:absolute before:left-1.5 before:top-2 before:bottom-2 before:w-0.5 before:rounded-full before:transition-opacity md:h-[3.125rem] md:w-full [--announcement-state-bg:color-mix(in_oklch,var(--sidebar-accent),var(--sidebar-foreground)_1%)]",
                    announcementTypeAccentClassName(item.type),
                    index === renderActiveIndex
                      ? "bg-[var(--announcement-state-bg)] text-sidebar-accent-foreground before:opacity-100"
                      : "text-muted-foreground before:opacity-70 hover:bg-[var(--announcement-state-bg)] hover:text-sidebar-accent-foreground",
                  )}
                  onClick={() => setActiveIndex(index)}
                >
                  <span className="absolute right-1.5 top-1.5 flex h-3.5 items-center gap-1">
                    {!isAnnouncementRead(item) ? <span aria-hidden="true" className="size-1.5 rounded-full bg-red-500" /> : null}
                    {item.pinned ? <Pin className="size-3 text-muted-foreground/70" /> : null}
                  </span>
                  <span className="block truncate font-medium">{item.title}</span>
                  <span className="mt-0.5 block text-xs text-muted-foreground">
                    {formatAnnouncementDate(item.updatedAt, locale)}
                  </span>
                </button>
              )) : (
                <div className="flex h-full min-h-24 items-center justify-center px-3 py-6 text-center text-xs text-muted-foreground">
                  {renderManualLoading ? t("loading") : t("empty")}
                </div>
              )}
            </div>
          </div>
          <div className="min-h-0 min-w-0 overflow-y-auto overflow-x-hidden px-3 py-3 sm:px-4">
            {active ? (
              <>
                <div className="mb-2 flex min-w-0 items-center justify-between gap-3 text-xs text-muted-foreground">
                  <span className="min-w-0 truncate">{active.title}</span>
                  <span className="shrink-0 tabular-nums">{formatAnnouncementTime(active.updatedAt, locale)}</span>
                </div>
                <StreamdownRender
                  content={active.contentMarkdown}
                  className={cn(
                    "max-w-full text-sm leading-7",
                    "[&_h1]:text-base [&_h1]:leading-7",
                    "[&_h2]:text-base [&_h2]:leading-7",
                    "[&_h3]:text-sm [&_h3]:leading-6",
                    "[&_li]:leading-7",
                  )}
                />
              </>
            ) : (
              <div className="flex min-h-full items-center justify-center text-center text-sm text-muted-foreground">
                {renderManualLoading ? t("loading") : t("empty")}
              </div>
            )}
          </div>
        </div>
        <DialogFooter className="shrink-0">
          {renderMode === "manual" ? (
            <Button type="button" onClick={() => unreadQueue.length > 0 ? void closeAll() : closeManualDialog()} disabled={stateSaving}>
              {t("close")}
            </Button>
          ) : (
            <>
              <Button type="button" variant="ghost" onClick={() => void dismissAllToday()} disabled={stateSaving}>
                {t("dismissAllToday")}
              </Button>
              <Button type="button" onClick={() => void closeAll()} disabled={stateSaving}>
                {t("close")}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
