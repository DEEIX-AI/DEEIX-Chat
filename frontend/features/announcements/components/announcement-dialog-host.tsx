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
import { cn } from "@/lib/utils";

type AnnouncementSortMode = "default" | "type" | "time";

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

function normalizeAnnouncementType(value: string): "critical" | "warning" | "info" | "normal" | "general" {
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
      return "before:bg-red-500/55 dark:before:bg-red-400/55";
    case "warning":
      return "before:bg-yellow-500/60 dark:before:bg-yellow-400/55";
    case "info":
      return "before:bg-blue-500/55 dark:before:bg-blue-400/55";
    case "normal":
      return "before:bg-emerald-500/55 dark:before:bg-emerald-400/55";
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

export function AnnouncementDialogHost() {
  const t = useTranslations("announcements");
  const locale = useLocale();
  const pathname = usePathname();
  const { accessToken, user, userStatus } = useAuthSession();
  const [queue, setQueue] = React.useState<AnnouncementDTO[]>([]);
  const [activeIndex, setActiveIndex] = React.useState(0);
  const [sortMode, setSortMode] = React.useState<AnnouncementSortMode>("default");
  const [stateSaving, setStateSaving] = React.useState(false);

  React.useEffect(() => {
    let cancelled = false;
    if (userStatus !== "ready" || !accessToken || user?.initialSecurityRequired || isSkippedPath(pathname)) {
      setQueue([]);
      setActiveIndex(0);
      return;
    }

    async function load() {
      try {
        const items = await listAnnouncements(accessToken);
        if (!cancelled) {
          setQueue(items);
          setActiveIndex(0);
        }
      } catch {
        if (!cancelled) {
          setQueue([]);
          setActiveIndex(0);
        }
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [accessToken, pathname, user?.initialSecurityRequired, userStatus]);

  const sortedQueue = React.useMemo(() => {
    if (sortMode === "time") {
      return [...queue].sort((a, b) => compareReadState(a, b) || announcementTime(b.updatedAt) - announcementTime(a.updatedAt) || b.id - a.id);
    }
    if (sortMode === "type") {
      return [...queue].sort((a, b) => compareReadState(a, b) || announcementTypeRank(b.type) - announcementTypeRank(a.type) || announcementTime(b.updatedAt) - announcementTime(a.updatedAt) || b.id - a.id);
    }
    return queue;
  }, [queue, sortMode]);

  React.useEffect(() => {
    setActiveIndex(0);
  }, [sortMode]);

  const hasUnread = queue.some((item) => !isAnnouncementRead(item));
  const active = hasUnread ? (sortedQueue[Math.min(activeIndex, Math.max(sortedQueue.length - 1, 0))] ?? null) : null;
  const open = hasUnread;

  const closeDialog = React.useCallback(() => {
    setQueue([]);
    setActiveIndex(0);
  }, []);

  const dismissAllToday = React.useCallback(async () => {
    if (!accessToken || stateSaving) {
      return;
    }
    setStateSaving(true);
    try {
      await Promise.all(queue.map((item) => dismissAnnouncementToday(accessToken, item.id, item.updatedAt)));
      closeDialog();
    } catch {
      toast.error(t("dismissFailed"));
    } finally {
      setStateSaving(false);
    }
  }, [accessToken, closeDialog, queue, stateSaving, t]);

  const closeAll = React.useCallback(async () => {
    if (!accessToken || stateSaving) {
      return;
    }
    setStateSaving(true);
    try {
      await Promise.all(queue.map((item) => closeAnnouncement(accessToken, item.id, item.updatedAt)));
      closeDialog();
    } catch {
      toast.error(t("closeFailed"));
    } finally {
      setStateSaving(false);
    }
  }, [accessToken, closeDialog, queue, stateSaving, t]);

  if (!active) {
    return null;
  }

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => {
      if (!nextOpen) {
        void closeAll();
      }
    }}>
      <DialogContent className="flex max-h-[min(84svh,720px)] flex-col overflow-hidden sm:max-w-[760px]">
        <DialogHeader className="shrink-0">
          <div className="min-w-0">
            <DialogTitle className="truncate">{t("title")}</DialogTitle>
          </div>
        </DialogHeader>
        <div className="grid h-[27rem] max-h-[calc(100svh-11rem)] min-h-0 grid-rows-[auto_minmax(0,1fr)] gap-0 overflow-hidden md:grid-cols-[13rem_minmax(0,1fr)] md:grid-rows-1">
          <div className="flex min-h-0 flex-col border-b border-border/60 md:border-b-0 md:border-r">
            <Tabs value={sortMode} onValueChange={(value) => setSortMode(value as AnnouncementSortMode)} className="shrink-0 px-2 pt-2 pb-1">
              <TabsList className="grid h-7 w-full grid-cols-3">
                <TabsTrigger value="default" className="px-1.5">{t("sort.default")}</TabsTrigger>
                <TabsTrigger value="type" className="px-1.5">{t("sort.type")}</TabsTrigger>
                <TabsTrigger value="time" className="px-1.5">{t("sort.time")}</TabsTrigger>
              </TabsList>
            </Tabs>
            <div className="flex gap-2 overflow-x-auto px-2 py-2 md:block md:min-h-0 md:flex-1 md:space-y-0.5 md:overflow-y-auto">
              {sortedQueue.map((item, index) => (
                <button
                  key={`${item.id}:${item.updatedAt}`}
                  type="button"
                  className={cn(
                    "relative min-w-36 rounded-md py-1 pl-3.5 pr-5 text-left text-xs transition-colors before:absolute before:left-1.5 before:top-2 before:bottom-2 before:w-0.5 before:rounded-full md:h-[3.125rem] md:w-full",
                    announcementTypeAccentClassName(item.type),
                    isAnnouncementRead(item) && "opacity-55",
                    index === activeIndex
                      ? "bg-muted text-foreground"
                      : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
                  )}
                  onClick={() => setActiveIndex(index)}
                >
                  {item.pinned ? (
                    <Pin className="absolute right-1.5 top-1.5 size-3 text-muted-foreground/70" />
                  ) : null}
                  <span className="block truncate font-medium">{item.title}</span>
                  <span className="mt-0.5 block text-xs text-muted-foreground">
                    {formatAnnouncementDate(item.updatedAt, locale)}
                  </span>
                </button>
              ))}
            </div>
          </div>
          <div className="min-h-0 overflow-y-auto px-4 py-3">
            <div className="mb-2 flex min-w-0 items-center justify-between gap-3 text-xs text-muted-foreground">
              <span className="min-w-0 truncate">{active.title}</span>
              <span className="shrink-0 tabular-nums">{formatAnnouncementTime(active.updatedAt, locale)}</span>
            </div>
            <StreamdownRender content={active.contentMarkdown} className="text-sm" />
          </div>
        </div>
        <DialogFooter className="shrink-0">
          <Button type="button" variant="ghost" onClick={() => void dismissAllToday()} disabled={stateSaving}>
            {t("dismissAllToday")}
          </Button>
          <Button type="button" onClick={() => void closeAll()} disabled={stateSaving}>
            {t("close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
