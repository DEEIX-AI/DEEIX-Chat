"use client";

import * as React from "react";
import Link from "next/link";
import { ArrowBigUp, Command as CommandIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Kbd, KbdGroup } from "@/components/ui/kbd";
import { SidebarMenuItem } from "@/components/ui/sidebar";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { NavigationItem, ShortcutKey } from "@/features/layouts/types/navigation";
import { platformModifierLabel } from "@/shared/lib/platform-shortcuts";
import { cn } from "@/lib/utils";

function ShortcutGlyph({
  value,
  modifierLabel,
}: {
  value: ShortcutKey;
  modifierLabel: "Command" | "Ctrl";
}) {
  if (value === "shift") {
    return <ArrowBigUp size={6} strokeWidth={1.6} className="size-3 shrink-0" />;
  }

  if (value === "command") {
    if (modifierLabel === "Command") {
      return <CommandIcon size={6} strokeWidth={1.6} className="size-3 shrink-0" />;
    }
    return <span className="text-[11px] leading-none font-medium text-sidebar-foreground/60">Ctrl</span>;
  }

  return <span className="text-[12px] leading-none font-medium text-sidebar-foreground/60">{value}</span>;
}

export function NavMainItem({
  item,
  title,
  isCollapsed,
  isMobile,
  onCreateConversation,
  onOpenSearch,
  onCloseMobileSidebar,
}: {
  item: NavigationItem;
  title: string;
  isCollapsed: boolean;
  isMobile: boolean;
  onCreateConversation: () => void;
  onOpenSearch: () => void;
  onCloseMobileSidebar: () => void;
}) {
  const Icon = item.icon;
  const isPrimary = item.variant === "primary";
  const [isHovered, setIsHovered] = React.useState(false);
  const [modifierLabel, setModifierLabel] = React.useState<"Command" | "Ctrl">("Command");

  React.useEffect(() => {
    setModifierLabel(platformModifierLabel());
  }, []);

  const itemClassName = cn(
    "group/item h-8 gap-0 rounded-md px-0 text-left text-sm font-normal text-sidebar-foreground transition-colors focus-visible:ring-2 focus-visible:ring-sidebar-ring",
    isCollapsed
      ? "w-8 justify-center hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
      : "w-full justify-start hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
  );

  const itemContent = (
    <>
      <span className="flex w-8 items-center justify-center">
        {isPrimary ? (
          <span
            className={cn(
              "flex size-6 items-center justify-center rounded-full bg-muted transition-[width,height,transform,background-color] duration-200",
              isHovered && "size-6 scale-105 bg-background",
            )}
          >
            <Icon
              aria-hidden
              strokeWidth={1.6}
              className={cn("size-4 text-current", isHovered && "scale-105")}
              animate={isHovered ? "default" : undefined}
            />
          </span>
        ) : isCollapsed ? (
          <span className="flex size-8 items-center justify-center rounded-md transition-colors hover:bg-accent">
            <Icon aria-hidden strokeWidth={1.6} className="size-4 text-current" animate={isHovered ? "default" : undefined} />
          </span>
        ) : (
          <Icon aria-hidden strokeWidth={1.6} className="size-4 text-current" animate={isHovered ? "default" : undefined} />
        )}
      </span>

      <span
        className={cn(
          "ml-1 flex min-w-0 flex-1 items-center overflow-hidden transition-[opacity,max-width,margin-left] duration-200 ease-linear",
          isCollapsed && "ml-0",
        )}
      >
        <span className="flex-1 truncate">{title}</span>
        {item.shortcut && !isCollapsed ? (
          <KbdGroup className="mr-2 shrink-0 opacity-0 transition-opacity duration-150 group-hover/item:opacity-100">
            {item.shortcut.map((value) => (
              <Kbd
                key={value}
                className="h-auto min-w-0 bg-transparent px-0"
              >
                <ShortcutGlyph value={value} modifierLabel={modifierLabel} />
              </Kbd>
            ))}
          </KbdGroup>
        ) : null}
      </span>
    </>
  );

  return (
    <SidebarMenuItem key={item.id}>
      <Tooltip>
        <TooltipTrigger asChild>
          {item.kind === "command" ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className={itemClassName}
              onMouseEnter={() => setIsHovered(true)}
              onMouseLeave={() => setIsHovered(false)}
              onClick={() => {
                if (item.id === "newChat") {
                  onCreateConversation();
                  if (isMobile) onCloseMobileSidebar();
                } else {
                  onOpenSearch();
                }
              }}
            >
              {itemContent}
            </Button>
          ) : (
            <Button
              asChild
              variant="ghost"
              size="sm"
              className={itemClassName}
              onMouseEnter={() => setIsHovered(true)}
              onMouseLeave={() => setIsHovered(false)}
            >
              <Link
                href={item.href}
                prefetch={false}
                onClick={() => {
                  if (isMobile) onCloseMobileSidebar();
                }}
              >
                {itemContent}
              </Link>
            </Button>
          )}
        </TooltipTrigger>
        <TooltipContent side="right" hidden={!isCollapsed}>
          <span>{title}</span>
        </TooltipContent>
      </Tooltip>
    </SidebarMenuItem>
  );
}
