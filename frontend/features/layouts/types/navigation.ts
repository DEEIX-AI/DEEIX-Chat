import type * as React from "react";

export type ShortcutKey = "command" | "shift" | "K" | "O";

export type NavigationIconProps = {
  size?: number;
  strokeWidth?: number;
  className?: string;
  animate?: "default";
  "aria-hidden"?: boolean;
};

type NavigationItemBase = {
  icon: React.ComponentType<NavigationIconProps>;
  group: "primary" | "secondary";
  variant?: "primary";
  shortcut?: readonly ShortcutKey[];
};

type NavigationCommandItem = NavigationItemBase & {
  id: "newChat" | "search";
  kind: "command";
};

type NavigationLinkItem = NavigationItemBase & {
  id: "recent" | "files" | "skillsPrompt";
  kind: "link";
  href: string;
};

export type NavigationItem = NavigationCommandItem | NavigationLinkItem;

export type ConversationSearchResult = {
  publicID: string;
  title: string;
  searchText: string;
  href: string;
  updatedAt: string;
};

export type SidebarConversationRenameTarget = {
  publicID: string;
  currentTitle: string;
} | null;

export type SidebarConversationDeleteTarget = {
  publicID: string;
  title: string;
} | null;
