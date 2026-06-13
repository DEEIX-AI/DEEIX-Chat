"use client";

import * as React from "react";

import { useRecentSidebarRecentsController } from "@/features/recent/hooks/use-recent-sidebar-recents";
import type { SidebarRecentsControllerValue } from "@/features/recent/types/sidebar-recents";

const SidebarRecentsContext = React.createContext<SidebarRecentsControllerValue | null>(null);

export function SidebarRecentsProvider({ children }: { children: React.ReactNode }) {
  const value = useRecentSidebarRecentsController();
  return <SidebarRecentsContext.Provider value={value}>{children}</SidebarRecentsContext.Provider>;
}

export function useSidebarRecents() {
  const context = React.useContext(SidebarRecentsContext);
  if (!context) {
    throw new Error("useSidebarRecents must be used within SidebarRecentsProvider");
  }
  return context;
}
