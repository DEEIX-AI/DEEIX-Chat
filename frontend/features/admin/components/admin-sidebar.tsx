"use client";

import * as React from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { CircleArrowUp } from "lucide-react";

import packageMeta from "@/package.json";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ADMIN_SECTIONS, type AdminSection } from "@/features/admin/model/admin-sections";
import { AdminUpdateTooltipContent } from "@/features/admin/components/admin-update-tooltip-content";
import {
  getCachedLatestReleaseSnapshot,
  getServerLatestReleaseSnapshot,
  resolveAvailableRelease,
  subscribeLatestReleaseChange,
} from "@/features/admin/model/update-check";
import { cn } from "@/lib/utils";

export function AdminSidebar({
  activeSection,
  basePath,
}: {
  activeSection: AdminSection;
  basePath: string;
}) {
  const t = useTranslations("adminUsers");
  const tAbout = useTranslations("adminUsers.aboutPage");
  const activeLinkRef = React.useRef<HTMLAnchorElement | null>(null);
  const cachedLatestRelease = React.useSyncExternalStore(
    subscribeLatestReleaseChange,
    getCachedLatestReleaseSnapshot,
    getServerLatestReleaseSnapshot,
  );
  const updateRelease = resolveAvailableRelease(packageMeta.version, cachedLatestRelease);
  const sectionLabel = React.useCallback(
    (id: AdminSection, fallback: string) => {
      const keyByID: Record<AdminSection, string> = {
        accounts: "sections.accounts",
        channels: "sections.channels",
        models: "sections.models",
        "tool-settings": "sections.toolSettings",
        billing: "sections.billing",
        announcements: "sections.announcements",
        logs: "sections.logs",
        "login-settings": "sections.loginSettings",
        "conversation-settings": "sections.conversationSettings",
        "chat-files": "sections.chatFiles",
        about: "sections.about",
      };
      return t(keyByID[id]) || fallback;
    },
    [t],
  );

  React.useEffect(() => {
    activeLinkRef.current?.scrollIntoView({
      block: "nearest",
      inline: "center",
    });
  }, [activeSection]);

  return (
    <aside className="w-full shrink-0 xl:max-w-64">
      <div className="space-y-3 xl:sticky xl:top-6 xl:space-y-5">
        <div className="flex h-9 items-center px-1 xl:h-10">
          <h1 className="text-xl font-semibold tracking-normal xl:text-2xl">{t("adminTitle")}</h1>
        </div>

        <nav
          aria-label={t("adminTitle")}
          className="flex gap-1.5 overflow-x-auto overscroll-x-contain pb-1 [scrollbar-width:none] [-ms-overflow-style:none] xl:grid xl:gap-1 xl:overflow-visible xl:pb-0 [&::-webkit-scrollbar]:hidden"
        >
          {ADMIN_SECTIONS.map((item) => {
            const active = item.id === activeSection;

            return (
              <Link
                key={item.id}
                ref={active ? activeLinkRef : undefined}
                href={`${basePath}${item.href}`}
                prefetch={false}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "relative flex h-8 shrink-0 scroll-mx-3 items-center justify-between gap-2 whitespace-nowrap rounded-md px-3 text-sm font-medium transition-colors xl:h-9 xl:w-full xl:px-3.5",
                  active
                    ? "bg-sidebar-accent text-sidebar-accent-foreground"
                    : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                )}
              >
                <span className="truncate">{sectionLabel(item.id, item.label)}</span>
                {item.id === "about" && updateRelease ? (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span className="ml-auto inline-flex size-4 shrink-0 items-center justify-center text-rose-500">
                        <CircleArrowUp className="size-3.5" aria-label={tAbout("updateAvailableIndicator")} />
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>
                      <AdminUpdateTooltipContent updateRelease={updateRelease} />
                    </TooltipContent>
                  </Tooltip>
                ) : null}
              </Link>
            );
          })}
        </nav>
      </div>
    </aside>
  );
}
