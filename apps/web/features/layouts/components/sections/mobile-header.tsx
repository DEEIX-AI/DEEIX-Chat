"use client";

import { PanelLeft, Plus } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { useSidebarActions } from "@/components/ui/sidebar";
import { MobileHeaderActionSlot } from "@/features/layouts/context/mobile-header-action-context";
import { AppLogo } from "@/shared/components/app-logo";

export function MobileHeader({
  onCreateConversation,
}: {
  onCreateConversation: () => void;
}) {
  const t = useTranslations("common.navigation");
  const { toggleSidebar } = useSidebarActions();

  return (
    <header className="grid h-12 shrink-0 grid-cols-[1fr_auto_1fr] items-center px-3 md:hidden">
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

      <div className="flex items-center justify-end gap-1">
        <MobileHeaderActionSlot />
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
