"use client";

import * as React from "react";

// What a block may do beyond its own state. `ask` sends text as the user's next
// message, always from a composer the user sees and submits; it is absent where
// nothing can be sent (share pages, previews), and asks there are read-only.
export type UIBlockActions = {
  ask?: (text: string) => void;
  // Puts text in the composer for the user to review and send. Used by custom
  // (sandboxed) components, whose clicks cannot be told apart from scripts.
  prefill?: (text: string) => void;
  // A reply is running; asks are disabled until it ends.
  busy?: boolean;
};

const NO_ACTIONS: UIBlockActions = {};

const UIBlockActionsContext = React.createContext<UIBlockActions>(NO_ACTIONS);

// Provided once above the message list, not per message: toggling `busy` then
// re-renders only the buttons, not every memoized message.
export function UIBlockActionsProvider({ actions, children }: { actions: UIBlockActions; children: React.ReactNode }) {
  return <UIBlockActionsContext.Provider value={actions}>{children}</UIBlockActionsContext.Provider>;
}

export function useUIBlockActions(): UIBlockActions {
  return React.useContext(UIBlockActionsContext);
}

// Per message: opens one of its blocks in the side panel. Absent inside the
// panel itself and wherever there is no panel.
const UIBlockOpenContext = React.createContext<((blockKey: string) => void) | null>(null);

export function UIBlockOpenProvider({ open, children }: { open: ((blockKey: string) => void) | null; children: React.ReactNode }) {
  return <UIBlockOpenContext.Provider value={open}>{children}</UIBlockOpenContext.Provider>;
}

export function useUIBlockOpen(): ((blockKey: string) => void) | null {
  return React.useContext(UIBlockOpenContext);
}
