"use client";

import { PanelLeft, Plus } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { useSidebar } from "@/components/ui/sidebar";
import { AppLogo } from "@/shared/components/app-logo";

export function MobileHeader({
  onCreateConversation,
}: {
  onCreateConversation: () => void;
}) {
  const t = useTranslations("common.navigation");
  const { toggleSidebar } = useSidebar();

  return (
    <header className="grid h-12 shrink-0 grid-cols-[2rem_minmax(0,1fr)_2rem] items-center px-3 md:hidden">
      <div className="flex justify-start">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-6"
          aria-label={t("openSidebar")}
          onClick={toggleSidebar}
        >
          <PanelLeft aria-hidden className="size-[18px]" strokeWidth={1.4} />
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
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-6"
          aria-label={t("newChat")}
          onClick={onCreateConversation}
        >
          <Plus aria-hidden className="size-4" strokeWidth={1.6} />
        </Button>
      </div>
    </header>
  );
}
