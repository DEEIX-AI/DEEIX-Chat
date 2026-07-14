"use client";

import * as React from "react";
import { Camera, Download, MousePointerClick, Share2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemIcon,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

type ConversationShareExportActionsProps = {
  shareLabel: string;
  exportLabel: string;
  onShare?: () => void;
  onExport?: () => void | Promise<void>;
  screenshotFullLabel?: string;
  screenshotSelectLabel?: string;
  onScreenshotFull?: () => void;
  onScreenshotSelect?: () => void;
};

type ConversationShareExportMenuItemsProps = ConversationShareExportActionsProps & {
  onCloseMenu?: () => void;
};

export function ConversationShareExportMenuItems({
  shareLabel,
  exportLabel,
  onShare,
  onExport,
  screenshotFullLabel,
  screenshotSelectLabel,
  onScreenshotFull,
  onScreenshotSelect,
  onCloseMenu,
}: ConversationShareExportMenuItemsProps) {
  const hasScreenshot = Boolean(onScreenshotFull || onScreenshotSelect);
  return (
    <>
      <DropdownMenuItem
        disabled={!onShare}
        onSelect={(event) => {
          event.preventDefault();
          if (!onShare) {
            return;
          }
          onCloseMenu?.();
          onShare();
        }}
      >
        <DropdownMenuItemIcon icon={Share2} className="text-current" />
        {shareLabel}
      </DropdownMenuItem>
      <DropdownMenuItem
        disabled={!onExport}
        onSelect={(event) => {
          event.preventDefault();
          if (!onExport) {
            return;
          }
          onCloseMenu?.();
          void onExport();
        }}
      >
        <DropdownMenuItemIcon icon={Download} className="text-current" />
        {exportLabel}
      </DropdownMenuItem>
      {hasScreenshot ? (
        <>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            disabled={!onScreenshotSelect}
            onSelect={(event) => {
              event.preventDefault();
              if (!onScreenshotSelect) {
                return;
              }
              onCloseMenu?.();
              onScreenshotSelect();
            }}
          >
            <DropdownMenuItemIcon icon={MousePointerClick} className="text-current" />
            {screenshotSelectLabel}
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={!onScreenshotFull}
            onSelect={(event) => {
              event.preventDefault();
              if (!onScreenshotFull) {
                return;
              }
              onCloseMenu?.();
              onScreenshotFull();
            }}
          >
            <DropdownMenuItemIcon icon={Camera} className="text-current" />
            {screenshotFullLabel}
          </DropdownMenuItem>
        </>
      ) : null}
    </>
  );
}

type ConversationShareExportIconDropdownProps = {
  label: string;
  active?: boolean;
  className?: string;
} & ConversationShareExportActionsProps;

export function ConversationShareExportIconDropdown({
  label,
  shareLabel,
  exportLabel,
  active = false,
  className,
  onShare,
  onExport,
  screenshotFullLabel,
  screenshotSelectLabel,
  onScreenshotFull,
  onScreenshotSelect,
}: ConversationShareExportIconDropdownProps) {
  const [open, setOpen] = React.useState(false);
  const hasAction = Boolean(onShare || onExport || onScreenshotFull || onScreenshotSelect);

  return (
    <DropdownMenu modal={false} open={open} onOpenChange={setOpen}>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={cn(
            "size-8 shrink-0 rounded-lg text-muted-foreground shadow-none hover:bg-muted hover:text-foreground",
            active && "text-foreground",
            className,
          )}
          disabled={!hasAction}
          aria-label={label}
          title={label}
        >
          <Share2 className="size-4 stroke-[1.8]" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" sideOffset={8} className="min-w-40">
        <ConversationShareExportMenuItems
          shareLabel={shareLabel}
          exportLabel={exportLabel}
          onShare={onShare}
          onExport={onExport}
          screenshotFullLabel={screenshotFullLabel}
          screenshotSelectLabel={screenshotSelectLabel}
          onScreenshotFull={onScreenshotFull}
          onScreenshotSelect={onScreenshotSelect}
          onCloseMenu={() => setOpen(false)}
        />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function ConversationShareExportSubmenu({
  label,
  shareLabel,
  exportLabel,
  onShare,
  onExport,
  screenshotFullLabel,
  screenshotSelectLabel,
  onScreenshotFull,
  onScreenshotSelect,
  onCloseMenu,
}: { label: string } & ConversationShareExportMenuItemsProps) {
  const hasAction = Boolean(onShare || onExport || onScreenshotFull || onScreenshotSelect);
  return (
    <DropdownMenuSub>
      <DropdownMenuSubTrigger disabled={!hasAction}>
        <DropdownMenuItemIcon icon={Share2} className="text-current" />
        {label}
      </DropdownMenuSubTrigger>
      <DropdownMenuSubContent className="min-w-40 p-1.5">
        <ConversationShareExportMenuItems
          shareLabel={shareLabel}
          exportLabel={exportLabel}
          onShare={onShare}
          onExport={onExport}
          screenshotFullLabel={screenshotFullLabel}
          screenshotSelectLabel={screenshotSelectLabel}
          onScreenshotFull={onScreenshotFull}
          onScreenshotSelect={onScreenshotSelect}
          onCloseMenu={onCloseMenu}
        />
      </DropdownMenuSubContent>
    </DropdownMenuSub>
  );
}
