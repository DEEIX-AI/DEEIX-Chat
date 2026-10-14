"use client";

import { readMessageUIBlocks } from "@deeix/core";
import * as React from "react";

import {
  type ChatUIBlockStateStore,
  createChatUIBlockStateStore,
  type UIBlockStateTarget,
} from "@/features/chat/model/chat-ui-block-state-store";
import type { ChatAreaMessage } from "@/features/chat/types/messages";
import { setMessageUIState } from "@/shared/api/conversation";
import type { MessageUIStateInput } from "@/shared/api/conversation-types";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import type { UIBlockState, UIBlockStateBinding } from "@/shared/components/markdown/ui-blocks/ui-block-state";

// How many blocks one turn carries. The server keeps the most recent few within
// its own budget; sending more only spends request bytes.
const MAX_TURN_UI_STATES = 32;

type ChatTurnUIState = {
  blockID: string;
  title?: string;
  state: UIBlockState;
};

type ChatUIBlockStateRuntime = {
  store: ChatUIBlockStateStore;
  // Block state of one message as the user currently sees it.
  statesFor: (message: Pick<ChatAreaMessage, "key" | "content" | "uiState">) => ChatTurnUIState[];
  // Block state of the messages on the branch being continued, for the turn
  // being sent. The server keeps only messages on the new message's own branch.
  collectTurnUIStates: (messages: readonly ChatAreaMessage[]) => MessageUIStateInput[];
};

// One store for the lifetime of the chat area. Changes stay in memory after they
// are written, so switching away and back never shows an older server snapshot.
export function useChatUIBlockStateRuntime(): ChatUIBlockStateRuntime {
  const [store] = React.useState(() =>
    createChatUIBlockStateStore({
      persist: async (target, blocks, { keepalive }) => {
        const token = await resolveAccessToken();
        if (!token) {
          return;
        }
        await setMessageUIState(
          token,
          target.conversationPublicID,
          target.messagePublicID,
          { blocks: Object.fromEntries(Object.entries(blocks).map(([blockID, state]) => [blockID, state ?? null])) },
          { keepalive },
        );
      },
    }),
  );

  // Pending changes are written when the page is hidden or unloaded, with
  // keepalive so the browser does not cancel them; a reload then restores the
  // same view. Each request carries one message's blocks (a few KB at most),
  // well under the browser's keepalive budget.
  React.useEffect(() => {
    const flushOnLeave = () => {
      void store.flush({ keepalive: true });
    };
    const onVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        flushOnLeave();
      }
    };
    document.addEventListener("visibilitychange", onVisibilityChange);
    window.addEventListener("pagehide", flushOnLeave);
    return () => {
      document.removeEventListener("visibilitychange", onVisibilityChange);
      window.removeEventListener("pagehide", flushOnLeave);
      // Leaving the chat area: the page stays alive, a normal write is enough.
      // The store itself is reused if React re-runs this effect in development.
      void store.flush();
    };
  }, [store]);

  const statesFor = React.useCallback(
    (message: Pick<ChatAreaMessage, "key" | "content" | "uiState">): ChatTurnUIState[] => {
      const states = store.stateFor(message.key, message.uiState);
      if (states.size === 0) {
        return [];
      }
      // A block the message no longer contains (the reply was edited) is not
      // something the model can relate the state to; neither is a key that
      // several of its blocks share.
      const blocks = new Map(readMessageUIBlocks(message.content).blocks.map((block) => [block.key, block]));
      const result: ChatTurnUIState[] = [];
      states.forEach(({ state, title }, blockID) => {
        const block = blocks.get(blockID);
        if (block) {
          result.push({ blockID, title: title ?? block.title, state });
        }
      });
      return result;
    },
    [store],
  );

  const collectTurnUIStates = React.useCallback(
    (messages: readonly ChatAreaMessage[]): MessageUIStateInput[] => {
      const entries: MessageUIStateInput[] = [];
      for (const message of messages) {
        if (message.role !== "assistant" || message.isPending || message.isStreaming) {
          continue;
        }
        const messagePublicID = message.publicID.trim();
        if (!messagePublicID) {
          continue;
        }
        for (const { blockID, title, state } of statesFor(message)) {
          entries.push({ messagePublicID, blockID, title, state });
        }
      }
      // Closest to the turn first when the budget is exceeded.
      return entries.length > MAX_TURN_UI_STATES ? entries.slice(entries.length - MAX_TURN_UI_STATES) : entries;
    },
    [statesFor],
  );

  return React.useMemo(() => ({ store, statesFor, collectTurnUIStates }), [collectTurnUIStates, statesFor, store]);
}

// The binding a message row hands to its blocks, plus where the message's state
// is written. A message still streaming has no row yet: its changes wait in
// memory until it settles. Without a conversation (temporary chat) or without
// `persist`, changes stay local and only feed the next turn.
export function useChatUIBlockStateBinding(
  store: ChatUIBlockStateStore | undefined,
  message: Pick<ChatAreaMessage, "key" | "publicID" | "role" | "status" | "isPending" | "isStreaming" | "uiState">,
  conversationPublicID: string | null | undefined,
  persist: boolean,
): UIBlockStateBinding | undefined {
  const serverStateRef = React.useRef(message.uiState);
  serverStateRef.current = message.uiState;

  const settled = !message.isPending && !message.isStreaming && message.status !== "pending";
  const target = React.useMemo<UIBlockStateTarget | undefined>(() => {
    const conversationID = conversationPublicID?.trim() ?? "";
    const messagePublicID = message.publicID.trim();
    if (!persist || !settled || message.role !== "assistant" || !conversationID || !messagePublicID) {
      return undefined;
    }
    return { conversationPublicID: conversationID, messagePublicID };
  }, [conversationPublicID, message.publicID, message.role, persist, settled]);

  React.useEffect(() => {
    store?.setTarget(message.key, target);
  }, [message.key, store, target]);

  // Stable per message: StreamdownRender is memoized, and a new binding would
  // re-render every block on each stream tick.
  return React.useMemo(
    () => (store && message.role === "assistant" ? store.bindingFor(message.key, () => serverStateRef.current) : undefined),
    [message.key, message.role, store],
  );
}
