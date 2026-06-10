"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";

import { PanelRight } from "@/components/animate-ui/icons/panel-right";
import { Button } from "@/components/ui/button";
import { PlusIcon } from "@/components/ui/plus";
import { useSidebar } from "@/components/ui/sidebar";
import { AppLogo } from "@/shared/components/app-logo";

export function MobileHeader() {
  const t = useTranslations("common.navigation");
  const { toggleSidebar } = useSidebar();

  return (
    <header className="grid h-12 shrink-0 grid-cols-[2rem_minmax(0,1fr)_2rem] items-center px-3 md:hidden">
      <div className="flex justify-start">
        <Button variant="ghost" size="icon" className="size-6" onClick={toggleSidebar}>
          <PanelRight size={18} strokeWidth={1.4} />
          <span className="sr-only">{t("openSidebar")}</span>
        </Button>
      </div>

      <div className="flex min-w-0 justify-center">
        <AppLogo
          width={64}
          height={48}
          priority
          className="h-5 w-auto object-contain"
        />
      </div>

      <div className="flex justify-end">
        <Button variant="ghost" size="icon" className="size-6" asChild>
          <Link href="/chat">
            <PlusIcon size={16} strokeWidth={1.6} />
            <span className="sr-only">{t("newChat")}</span>
          </Link>
        </Button>
      </div>
    </header>
  );
}
