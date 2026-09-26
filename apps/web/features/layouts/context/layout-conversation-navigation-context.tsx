"use client";

import * as React from "react";
import { usePathname, useSearchParams } from "next/navigation";

type LayoutConversationNavigationContextValue = {
  activeConversationID: string | null;
  beginConversationNavigation: (conversationID: string) => void;
};

const LayoutConversationNavigationContext = React.createContext<LayoutConversationNavigationContextValue | null>(null);
const PENDING_NAVIGATION_TIMEOUT_MS = 10_000;

export function LayoutConversationNavigationProvider({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const routeConversationID = searchParams.get("conversation_id")?.trim() || null;
  const routeKey = `${pathname}?${searchParams.toString()}`;
  const previousRouteKeyRef = React.useRef(routeKey);
  const [pendingConversationID, setPendingConversationID] = React.useState<string | null>(null);

  React.useEffect(() => {
    const routeChanged = previousRouteKeyRef.current !== routeKey;
    previousRouteKeyRef.current = routeKey;

    setPendingConversationID((current) => {
      if (!current || (!routeChanged && current !== routeConversationID)) {
        return current;
      }
      return null;
    });
  }, [routeConversationID, routeKey]);

  React.useEffect(() => {
    if (!pendingConversationID) {
      return;
    }
    const timerID = window.setTimeout(() => {
      setPendingConversationID((current) => current === pendingConversationID ? null : current);
    }, PENDING_NAVIGATION_TIMEOUT_MS);
    return () => window.clearTimeout(timerID);
  }, [pendingConversationID]);

  const beginConversationNavigation = React.useCallback((conversationID: string) => {
    const normalizedConversationID = conversationID.trim();
    setPendingConversationID(
      normalizedConversationID && normalizedConversationID !== routeConversationID
        ? normalizedConversationID
        : null,
    );
  }, [routeConversationID]);

  const value = React.useMemo(
    () => ({
      activeConversationID: pendingConversationID ?? routeConversationID,
      beginConversationNavigation,
    }),
    [beginConversationNavigation, pendingConversationID, routeConversationID],
  );

  return (
    <LayoutConversationNavigationContext.Provider value={value}>
      {children}
    </LayoutConversationNavigationContext.Provider>
  );
}

export function useLayoutConversationNavigation() {
  const context = React.useContext(LayoutConversationNavigationContext);
  if (!context) {
    throw new Error("useLayoutConversationNavigation must be used within LayoutConversationNavigationProvider");
  }
  return context;
}
