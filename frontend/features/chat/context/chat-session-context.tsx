"use client";

import * as React from "react";

type ChatSessionContextValue = {
  newConversationRevision: number;
  requestNewConversation: () => void;
};

const ChatSessionContext = React.createContext<ChatSessionContextValue | null>(null);

export function ChatSessionProvider({ children }: { children: React.ReactNode }) {
  const [newConversationRevision, requestNewConversation] = React.useReducer((value: number) => value + 1, 0);

  const value = React.useMemo(
    () => ({
      newConversationRevision,
      requestNewConversation,
    }),
    [newConversationRevision],
  );

  return <ChatSessionContext.Provider value={value}>{children}</ChatSessionContext.Provider>;
}

export function useChatSession() {
  const context = React.useContext(ChatSessionContext);
  if (!context) {
    throw new Error("useChatSession must be used within ChatSessionProvider");
  }
  return context;
}
