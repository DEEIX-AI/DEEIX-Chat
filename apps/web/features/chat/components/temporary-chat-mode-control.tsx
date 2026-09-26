"use client";

import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import * as React from "react";
import { createPortal } from "react-dom";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import {
  type HatGlassesIconHandle,
  HatGlassesIcon,
} from "@/components/ui/hat-glasses";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useMobileHeaderActionSlot } from "@/features/layouts/context/mobile-header-action-context";
import { cn } from "@/lib/utils";

function TemporaryModeButton({
  active,
  layout,
  iconRef,
  label,
  onClick,
}: {
  active: boolean;
  layout: "desktop" | "mobile";
  iconRef: React.RefObject<HatGlassesIconHandle | null>;
  label: string;
  onClick: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={cn(
            layout === "desktop"
              ? "size-8 shrink-0 rounded-lg text-muted-foreground shadow-none hover:bg-muted hover:text-foreground"
              : "size-6 rounded-full text-foreground hover:text-foreground",
            active && (layout === "desktop" ? "text-foreground" : "bg-muted"),
          )}
          aria-label={label}
          onMouseEnter={() => iconRef.current?.startAnimation()}
          onMouseLeave={() => iconRef.current?.stopAnimation()}
          onFocus={() => iconRef.current?.startAnimation()}
          onBlur={() => iconRef.current?.stopAnimation()}
          onClick={onClick}
        >
          <HatGlassesIcon
            ref={iconRef}
            size={layout === "desktop" ? 16 : 19}
            strokeWidth={layout === "desktop" ? 1.8 : 1.6}
            aria-hidden
          />
        </Button>
      </TooltipTrigger>
      <TooltipContent side="bottom" align="end" sideOffset={6}>
        {label}
      </TooltipContent>
    </Tooltip>
  );
}

export function TemporaryChatModeControl({
  active,
  requiresExitConfirmation,
}: {
  active: boolean;
  requiresExitConfirmation: boolean;
}) {
  const router = useRouter();
  const t = useTranslations("chat");
  const mobileHeaderSlot = useMobileHeaderActionSlot();
  const desktopIconRef = React.useRef<HatGlassesIconHandle>(null);
  const mobileIconRef = React.useRef<HatGlassesIconHandle>(null);
  const [exitDialogOpen, setExitDialogOpen] = React.useState(false);
  const label = t(active ? "temporary.historyChat" : "temporary.title");

  const changeMode = React.useCallback(() => {
    if (active && requiresExitConfirmation) {
      setExitDialogOpen(true);
      return;
    }
    router.push(active ? "/chat" : "/chat?temporary=true");
  }, [active, requiresExitConfirmation, router]);

  return (
    <>
      <div className="absolute right-0 top-2.5 z-30 hidden md:block">
        <TemporaryModeButton
          active={active}
          iconRef={desktopIconRef}
          label={label}
          layout="desktop"
          onClick={changeMode}
        />
      </div>
      {mobileHeaderSlot ? createPortal(
        <TemporaryModeButton
          active={active}
          iconRef={mobileIconRef}
          label={label}
          layout="mobile"
          onClick={changeMode}
        />,
        mobileHeaderSlot,
      ) : null}

      <AlertDialog open={exitDialogOpen} onOpenChange={setExitDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("temporary.exitTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("temporary.exitDescription")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("temporary.continue")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() => {
                setExitDialogOpen(false);
                router.push("/chat");
              }}
            >
              {t("temporary.exitAction")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
