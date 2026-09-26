"use client";

import * as React from "react";

import { useSidebarConversationsController } from "@/entities/conversation/hooks/use-sidebar-conversations";
import type { SidebarConversationsControllerValue } from "@/entities/conversation/types/sidebar-conversations";

type SidebarConversationsStore = {
  getSnapshot: () => SidebarConversationsControllerValue;
  publish: (value: SidebarConversationsControllerValue) => void;
  subscribe: (key: keyof SidebarConversationsControllerValue, listener: () => void) => () => void;
};

function createSidebarConversationsStore(
  initialValue: SidebarConversationsControllerValue,
): SidebarConversationsStore {
  let snapshot = initialValue;
  const listeners = new Map<keyof SidebarConversationsControllerValue, Set<() => void>>();

  return {
    getSnapshot: () => snapshot,
    publish: (value) => {
      if (Object.is(snapshot, value)) {
        return;
      }
      const previous = snapshot;
      snapshot = value;
      listeners.forEach((keyListeners, key) => {
        if (!Object.is(previous[key], value[key])) {
          keyListeners.forEach((listener) => {
            listener();
          });
        }
      });
    },
    subscribe: (key, listener) => {
      const keyListeners = listeners.get(key) ?? new Set<() => void>();
      keyListeners.add(listener);
      listeners.set(key, keyListeners);
      return () => {
        keyListeners.delete(listener);
        if (keyListeners.size === 0) {
          listeners.delete(key);
        }
      };
    },
  };
}

const SidebarConversationsContext = React.createContext<SidebarConversationsStore | null>(null);

export function SidebarConversationsProvider({
  bulkPendingTitle,
  children,
  newConversationTitle,
}: {
  bulkPendingTitle: string;
  children: React.ReactNode;
  newConversationTitle: string;
}) {
  const value = useSidebarConversationsController({ bulkPendingTitle, newConversationTitle });
  const storeRef = React.useRef<SidebarConversationsStore | null>(null);
  if (storeRef.current === null) {
    storeRef.current = createSidebarConversationsStore(value);
  }
  const store = storeRef.current;

  // Publish after commit so controller renders never mutate the external store during render.
  React.useLayoutEffect(() => {
    store.publish(value);
  }, [store, value]);

  return <SidebarConversationsContext.Provider value={store}>{children}</SidebarConversationsContext.Provider>;
}

function useSidebarConversationsStore() {
  const context = React.useContext(SidebarConversationsContext);
  if (!context) {
    throw new Error("Sidebar conversation hooks must be used within SidebarConversationsProvider");
  }
  return context;
}

export function useSidebarConversationField<Key extends keyof SidebarConversationsControllerValue>(
  key: Key,
): SidebarConversationsControllerValue[Key] {
  const store = useSidebarConversationsStore();
  const subscribe = React.useCallback((listener: () => void) => store.subscribe(key, listener), [key, store]);
  const getFieldSnapshot = React.useCallback(() => store.getSnapshot()[key], [key, store]);
  return React.useSyncExternalStore(subscribe, getFieldSnapshot, getFieldSnapshot);
}
