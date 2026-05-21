"use client";

import * as React from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";

import { ADMIN_SECTIONS, type AdminSection } from "@/features/admin/model/admin-sections";
import { cn } from "@/lib/utils";

export function AdminSidebar({
  activeSection,
  basePath,
}: {
  activeSection: AdminSection;
  basePath: string;
}) {
  const t = useTranslations("adminUsers");
  const sectionLabel = React.useCallback(
    (id: AdminSection, fallback: string) => {
      const keyByID: Record<AdminSection, string> = {
        accounts: "sections.accounts",
        channels: "sections.channels",
        models: "sections.models",
        "tool-settings": "sections.toolSettings",
        billing: "sections.billing",
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
                href={`${basePath}${item.href}`}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "relative flex h-8 shrink-0 items-center whitespace-nowrap rounded-md px-3 text-sm font-medium transition-colors xl:h-9 xl:w-full xl:px-3.5",
                  active
                    ? "bg-sidebar-accent text-sidebar-accent-foreground"
                    : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                )}
              >
                {sectionLabel(item.id, item.label)}
              </Link>
            );
          })}
        </nav>
      </div>
    </aside>
  );
}
