"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";

import { cn } from "@/lib/utils";

export const SETTINGS_SIDEBAR_ITEMS = [
  { id: "general", labelKey: "general", href: "/general" },
  { id: "chat", labelKey: "chat", href: "/chat" },
  { id: "subscription", labelKey: "subscription", href: "/subscription" },
  { id: "account", labelKey: "account", href: "/account" },
  { id: "about", labelKey: "about", href: "/about" },
] as const;

export type SettingsSidebarSection = (typeof SETTINGS_SIDEBAR_ITEMS)[number]["id"];

export function resolveSettingsSidebarSection(section?: string | null): SettingsSidebarSection {
  if (SETTINGS_SIDEBAR_ITEMS.some((item) => item.id === section)) {
    return section as SettingsSidebarSection;
  }
  return "general";
}

export function SettingsSidebar({
  activeSection,
  basePath,
}: {
  activeSection: SettingsSidebarSection;
  basePath: string;
}) {
  const t = useTranslations("settings");

  return (
    <aside className="w-full shrink-0 xl:max-w-64">
      <div className="space-y-3 xl:sticky xl:top-6 xl:space-y-5">
        <div className="flex h-9 items-center px-1 xl:h-10">
          <h1 className="text-xl font-semibold tracking-normal xl:text-2xl">{t("title")}</h1>
        </div>

        <nav
          aria-label={t("navigation")}
          className="flex gap-1.5 overflow-x-auto overscroll-x-contain pb-1 [scrollbar-width:none] [-ms-overflow-style:none] xl:grid xl:gap-1 xl:overflow-visible xl:pb-0 [&::-webkit-scrollbar]:hidden"
        >
          {SETTINGS_SIDEBAR_ITEMS.map((item) => {
            const active = item.id === activeSection;

            return (
              <Link
                key={item.id}
                href={`${basePath}${item.href}`}
                prefetch={false}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "relative flex h-8 shrink-0 items-center whitespace-nowrap rounded-md px-3 text-sm font-medium transition-colors outline-hidden ring-sidebar-ring focus-visible:ring-2 xl:h-9 xl:w-full xl:px-3.5 [--settings-sidebar-state-bg:color-mix(in_oklch,var(--sidebar-accent),var(--sidebar-foreground)_1%)]",
                  active
                    ? "bg-[var(--settings-sidebar-state-bg)] text-sidebar-accent-foreground"
                    : "text-sidebar-foreground hover:bg-[var(--settings-sidebar-state-bg)] hover:text-sidebar-accent-foreground active:bg-[var(--settings-sidebar-state-bg)] active:text-sidebar-accent-foreground",
                )}
              >
                {t(item.labelKey)}
              </Link>
            );
          })}
        </nav>
      </div>
    </aside>
  );
}
