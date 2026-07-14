"use client";

import * as React from "react";
import Link from "next/link";
import { Archive, PencilLine, Trash, type LucideIcon } from "lucide-react";
import { motion } from "motion/react";
import { useTranslations } from "next-intl";

import { Ellipsis } from "@/components/animate-ui/icons/ellipsis";
import { Sparkles } from "@/components/animate-ui/icons/sparkles";
import { AnimatedText } from "@/components/ui/animated-text";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemIcon,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SIDEBAR_TRANSFER_TRANSITION } from "@/features/layouts/model/sidebar-motion";
import { ConversationProjectSubmenu } from "@/shared/components/conversation-project-submenu";
import { ConversationShareExportSubmenu } from "@/shared/components/conversation-share-export-menu";
import { cn } from "@/lib/utils";

type SidebarConversationViewModel = {
  publicID: string;
  title: string;
  url: string;
  shareActive?: boolean;
};

type SidebarConversationStarAction = {
  label: string;
  icon: LucideIcon;
  onSelect: (publicID: string) => void;
};

type SidebarConversationProjectMenu = {
  label: string;
  unassignedLabel: string;
  currentProjectID?: string;
  projects: Array<{
    publicID: string;
    name: string;
  }>;
  onSelect: (publicID: string, projectID?: string) => void;
};

type SidebarConversationItemProps = {
  item: SidebarConversationViewModel;
  active: boolean;
  isTransferring: boolean;
  isRenaming: boolean;
  renameValue: string;
  menuTriggerID: string;
  starAction: SidebarConversationStarAction;
  projectMenu?: SidebarConversationProjectMenu;
  rowClassName?: string;
  linkClassName?: string;
  onRenameValueChange: (value: string) => void;
  onRenameCommit: (publicID: string, currentTitle: string) => void;
  onRenameCancel: () => void;
  onRename: (publicID: string, currentTitle: string) => void;
  onAutoRename?: (publicID: string) => void | Promise<void>;
  isAutoRenaming?: boolean;
  onArchive: (publicID: string) => void;
  onShare?: (publicID: string, title: string) => void;
  onExport?: (publicID: string) => void | Promise<void>;
  onDelete: (publicID: string, title: string) => void;
  onNavigate?: (url: string, event: React.MouseEvent<HTMLAnchorElement>) => void;
};

export function SidebarConversationItem({
  item,
  active,
  isTransferring,
  isRenaming,
  renameValue,
  menuTriggerID,
  starAction,
  projectMenu,
  rowClassName,
  linkClassName,
  onRenameValueChange,
  onRenameCommit,
  onRenameCancel,
  onRename,
  onAutoRename,
  isAutoRenaming = false,
  onArchive,
  onShare,
  onExport,
  onDelete,
  onNavigate,
}: SidebarConversationItemProps) {
  const t = useTranslations("recent.row");
  const [isMenuOpen, setIsMenuOpen] = React.useState(false);

  const content = isRenaming ? (
    <div className="relative flex h-8 items-center rounded-md bg-sidebar-accent text-sm text-sidebar-accent-foreground">
      <input
        autoFocus
        value={renameValue}
        className={cn("h-8 w-full bg-transparent pl-2 outline-none", onAutoRename ? "pr-8" : "pr-2")}
        onChange={(event) => onRenameValueChange(event.target.value)}
        onClick={(event) => event.stopPropagation()}
        onMouseDown={(event) => event.stopPropagation()}
        onKeyDown={(event) => {
          event.stopPropagation();
          if (event.key === "Enter") {
            event.preventDefault();
            onRenameCommit(item.publicID, item.title);
          } else if (event.key === "Escape") {
            event.preventDefault();
            onRenameCancel();
          }
        }}
        onBlur={() => onRenameCommit(item.publicID, item.title)}
      />
      {onAutoRename ? (
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="absolute right-1 text-sidebar-foreground/45 hover:bg-transparent hover:text-sidebar-foreground disabled:opacity-60 dark:hover:bg-transparent [&_svg]:pointer-events-auto"
          aria-label={t("autoRename")}
          title={t("autoRename")}
          disabled={isAutoRenaming}
          onMouseDown={(event) => {
            event.preventDefault();
            event.stopPropagation();
          }}
          onClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            void onAutoRename(item.publicID);
          }}
        >
          {isAutoRenaming ? (
            <Spinner className="size-3.5" />
          ) : (
            <Sparkles aria-hidden size={14} strokeWidth={1.5} animateOnHover="default" />
          )}
        </Button>
      ) : null}
    </div>
  ) : (
    <div
      className={cn(
        "group relative flex h-8 items-center rounded-md text-sm transition-colors",
        active
          ? "bg-sidebar-accent text-sidebar-accent-foreground"
          : "text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
        rowClassName,
      )}
    >
      <Link
        href={item.url}
        prefetch={false}
        className={cn("flex h-full min-w-0 flex-1 items-center pl-2 pr-9", linkClassName)}
        onClick={(event) => onNavigate?.(item.url, event)}
      >
        <AnimatedText
          text={item.title}
          className="flex-1"
          textClassName="text-current"
        />
      </Link>

      <DropdownMenu modal={false} open={isMenuOpen} onOpenChange={setIsMenuOpen}>
        <DropdownMenuTrigger asChild>
          <Button
            id={menuTriggerID}
            type="button"
            variant="ghost"
            size="icon"
            aria-label={t("menu")}
            title={t("menu")}
            className={cn(
              "absolute right-0 text-sidebar-foreground/45 opacity-0 transition-[color,opacity] duration-150 hover:bg-transparent hover:text-sidebar-foreground group-hover/conversation-row:opacity-100 group-focus-within/conversation-row:opacity-100 data-[state=open]:text-sidebar-foreground dark:hover:bg-transparent [&_svg]:pointer-events-auto",
              isMenuOpen && "opacity-100",
            )}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
            }}
          >
            <Ellipsis aria-hidden size={16} strokeWidth={1.4} animateOnHover="pulse" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-max min-w-36 max-w-[calc(100vw-2rem)]">
          <DropdownMenuItem
            onSelect={(event) => {
              event.preventDefault();
              starAction.onSelect(item.publicID);
            }}
          >
            <DropdownMenuItemIcon icon={starAction.icon} className="text-current" />
            {starAction.label}
          </DropdownMenuItem>
          <DropdownMenuItem
            onSelect={(event) => {
              event.preventDefault();
              onRename(item.publicID, item.title);
            }}
          >
            <DropdownMenuItemIcon icon={PencilLine} className="text-current" />
            {t("rename")}
          </DropdownMenuItem>
          {projectMenu ? (
            <ConversationProjectSubmenu
              label={projectMenu.label}
              unassignedLabel={projectMenu.unassignedLabel}
              currentProjectID={projectMenu.currentProjectID}
              projects={projectMenu.projects}
              onSelect={(projectID) => projectMenu.onSelect(item.publicID, projectID)}
            />
          ) : null}
          <ConversationShareExportSubmenu
            label={t("shareAndExport")}
            shareLabel={item.shareActive ? t("manageShare") : t("share")}
            exportLabel={t("exportJSON")}
            onShare={onShare ? () => onShare(item.publicID, item.title) : undefined}
            onExport={onExport ? () => onExport(item.publicID) : undefined}
            onCloseMenu={() => setIsMenuOpen(false)}
          />
          <DropdownMenuItem
            onSelect={(event) => {
              event.preventDefault();
              onArchive(item.publicID);
            }}
          >
            <DropdownMenuItemIcon icon={Archive} className="text-current" />
            {t("archive")}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            variant="destructive"
            onSelect={(event) => {
              event.preventDefault();
              onDelete(item.publicID, item.title);
            }}
          >
            <DropdownMenuItemIcon icon={Trash} className="text-current" />
            {t("delete")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
  const itemProps = {
    className: "group/conversation-row relative",
    "data-sidebar-conversation-id": item.publicID,
    "data-sidebar-active": active ? "true" : "false",
  };

  if (!isTransferring) {
    return <li {...itemProps}>{content}</li>;
  }

  return (
    <motion.li
      layout="position"
      layoutId={`sidebar-conversation-${item.publicID}`}
      initial={false}
      transition={SIDEBAR_TRANSFER_TRANSITION}
      style={{ willChange: "transform" }}
      {...itemProps}
    >
      {content}
    </motion.li>
  );
}
