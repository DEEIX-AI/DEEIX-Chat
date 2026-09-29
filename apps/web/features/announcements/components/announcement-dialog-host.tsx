"use client";

import { Pin } from "lucide-react";
import dynamic from "next/dynamic";
import { usePathname } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogHeightTransition,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useAnnouncementDialog } from "@/features/announcements/hooks/use-announcement-dialog";
import { isAnnouncementRead, normalizeAnnouncementType } from "@/features/announcements/model/announcement-order";
import { cn } from "@/lib/utils";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";

const StreamdownRender = dynamic(
  () => import("@/shared/components/markdown/streamdown-render").then((mod) => mod.StreamdownRender),
  {
    ssr: false,
    loading: () => (
      <div aria-hidden="true" className="space-y-2 pt-1">
        <div className="h-3 w-11/12 animate-pulse rounded bg-muted" />
        <div className="h-3 w-full animate-pulse rounded bg-muted" />
        <div className="h-3 w-4/5 animate-pulse rounded bg-muted" />
      </div>
    ),
  },
);

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

export function AnnouncementDialogHost() {
  const t = useTranslations("announcements");
  const locale = useLocale();
  const pathname = usePathname();
  const {
    open,
    dialogMode,
    sortedQueue,
    activeIndex,
    setActiveIndex,
    sortMode,
    handleSortModeChange,
    manualLoading,
    stateSaving,
    hasUnreadInQueue,
    handleOpenChange,
    closeManualDialog,
    dismissAllToday,
    closeAll,
  } = useAnnouncementDialog(pathname);

  const renderMode = useDialogSnapshot(open ? dialogMode : null) ?? dialogMode;
  const renderQueue = useDialogSnapshot(open ? sortedQueue : null) ?? sortedQueue;
  const renderActiveIndex = useDialogSnapshot(open ? activeIndex : null) ?? activeIndex;
  const renderManualLoading = useDialogSnapshot(open ? manualLoading : null) ?? manualLoading;
  const active = renderQueue[Math.min(renderActiveIndex, Math.max(renderQueue.length - 1, 0))] ?? null;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="overflow-hidden p-4 sm:max-w-[760px] sm:p-5">
        <DialogHeightTransition contentClassName="max-h-[calc(min(84svh,720px)-2rem)] gap-4 sm:max-h-[calc(min(84svh,720px)-2.5rem)]">
          <DialogHeader className="shrink-0">
            <div className="min-w-0">
              <DialogTitle className="truncate">{t("title")}</DialogTitle>
            </div>
          </DialogHeader>
          <div className="grid h-[27rem] max-h-[calc(100svh-11rem)] min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] gap-0 overflow-hidden md:grid-cols-[13rem_minmax(0,1fr)] md:grid-rows-1">
            <div className="relative flex min-h-0 min-w-0 flex-col border-b border-border/60 md:border-b-0 md:border-r">
              <Tabs value={sortMode} onValueChange={handleSortModeChange} className="relative z-10 min-w-0 shrink-0 px-2 pt-2 pb-1">
                <TabsList className="grid h-7 w-full grid-cols-3">
                  <TabsTrigger value="default" className="px-1.5">{t("sort.default")}</TabsTrigger>
                  <TabsTrigger value="type" className="px-1.5">{t("sort.type")}</TabsTrigger>
                  <TabsTrigger value="time" className="px-1.5">{t("sort.time")}</TabsTrigger>
                </TabsList>
              </Tabs>
              <div className={cn(
                "flex min-w-0 gap-2 overflow-x-auto px-2 py-2 md:block md:min-h-0 md:flex-1 md:space-y-0.5",
                renderQueue.length > 0 ? "md:overflow-y-auto" : "md:overflow-visible",
              )}>
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
                  <div className="pointer-events-none flex h-full min-h-24 items-center justify-center px-3 py-6 text-center text-xs text-muted-foreground md:absolute md:inset-0 md:h-auto md:min-h-0">
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
                <div className="flex min-h-full items-center justify-center px-3 py-6 text-center text-xs text-muted-foreground">
                  {renderManualLoading ? t("loading") : t("empty")}
                </div>
              )}
            </div>
          </div>
          <DialogFooter className="shrink-0">
            {renderMode === "manual" ? (
              <Button type="button" onClick={() => hasUnreadInQueue ? void closeAll() : closeManualDialog()} disabled={stateSaving}>
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
        </DialogHeightTransition>
      </DialogContent>
    </Dialog>
  );
}
