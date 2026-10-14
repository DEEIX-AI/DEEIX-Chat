import type { UIBlockState, UIBlockStateBinding } from "@/shared/components/markdown/ui-blocks/ui-block-state";
import { isPersistableUIBlockName, isUIBlockState } from "@/shared/components/markdown/ui-blocks/ui-block-state";

// Per-block state of the interactive components in the conversation on screen.
//
// The store holds only local changes, keyed by message and block id; the server
// snapshot arrives with each message and is merged at read time, so restoring
// never has to wait for the store and the store never copies server data.
//
// Writes are debounced per message and merged server-side per block, so two tabs
// editing different blocks of one message do not overwrite each other. A message
// without a row yet (still streaming, or a temporary chat) keeps its changes in
// memory; they are written once the settled message gets a target.

const UI_BLOCK_STATE_DEBOUNCE_MS = 1000;

export type UIBlockStateTarget = {
  conversationPublicID: string;
  messagePublicID: string;
};

// `undefined` clears that block's stored state.
type UIBlockStatePatch = Record<string, UIBlockState | undefined>;

// `keepalive` is set when the write happens because the page is going away.
type UIBlockStatePersist = (target: UIBlockStateTarget, blocks: UIBlockStatePatch, options: { keepalive: boolean }) => Promise<void>;

type LocalChange = {
  // undefined: the user put the block back to its defaults.
  state: UIBlockState | undefined;
  title?: string;
};

export type ChatUIBlockStateStore = {
  // The binding a message's blocks read and write through. `serverState` is the
  // snapshot that came with the message; reads fall back to it.
  bindingFor: (messageKey: string, getServerState: () => UIBlockState | undefined) => UIBlockStateBinding;
  // Where a message's changes are written; undefined keeps them local.
  setTarget: (messageKey: string, target: UIBlockStateTarget | undefined) => void;
  // A message's current block state: server snapshot with local changes applied.
  stateFor: (messageKey: string, serverState: UIBlockState | undefined) => Map<string, { state: UIBlockState; title?: string }>;
  // Writes everything pending now. Safe to call repeatedly; the store stays usable.
  // `keepalive` lets the requests outlive a page that is being hidden or unloaded.
  flush: (options?: { keepalive?: boolean }) => Promise<void>;
};

export function createChatUIBlockStateStore({
  persist,
  debounceMs = UI_BLOCK_STATE_DEBOUNCE_MS,
}: {
  persist: UIBlockStatePersist;
  debounceMs?: number;
}): ChatUIBlockStateStore {
  // Every change the user made in this session, kept after it is written so the
  // view and the next turn see it even before the message list is reloaded.
  const changes = new Map<string, Map<string, LocalChange>>();
  // Changes not yet acknowledged by the server, per message.
  const unsent = new Map<string, Set<string>>();
  const targets = new Map<string, UIBlockStateTarget>();
  const timers = new Map<string, ReturnType<typeof setTimeout>>();
  const inFlight = new Set<string>();
  // Views of a block (inline and in the side panel) re-read after each write.
  const listeners = new Map<string, Set<() => void>>();
  const listenerKey = (messageKey: string, blockID: string) => `${messageKey}\u0000${blockID}`;

  function schedule(messageKey: string) {
    if (!targets.has(messageKey)) {
      return;
    }
    const existing = timers.get(messageKey);
    if (existing) {
      clearTimeout(existing);
    }
    timers.set(
      messageKey,
      setTimeout(() => {
        timers.delete(messageKey);
        void writeNow(messageKey, false);
      }, debounceMs),
    );
  }

  async function writeNow(messageKey: string, keepalive: boolean): Promise<void> {
    const target = targets.get(messageKey);
    const pending = unsent.get(messageKey);
    const messageChanges = changes.get(messageKey);
    if (!target || !pending?.size || !messageChanges || inFlight.has(messageKey)) {
      return;
    }

    const sent = new Map<string, LocalChange>();
    const blocks: UIBlockStatePatch = {};
    for (const blockID of pending) {
      const change = messageChanges.get(blockID);
      if (!change) {
        continue;
      }
      sent.set(blockID, change);
      blocks[blockID] = change.state;
    }
    if (sent.size === 0) {
      unsent.delete(messageKey);
      return;
    }

    inFlight.add(messageKey);
    try {
      await persist(target, blocks, { keepalive });
    } catch {
      // Block state is a convenience: a failed write never surfaces to the user
      // or interrupts the conversation. The next change retries it.
      return;
    } finally {
      inFlight.delete(messageKey);
    }

    // Only acknowledge what was sent: a block changed again while the request
    // was in flight stays unsent and goes out on the next pass.
    const stillPending = unsent.get(messageKey);
    if (stillPending) {
      sent.forEach((change, blockID) => {
        if (changes.get(messageKey)?.get(blockID) === change) {
          stillPending.delete(blockID);
        }
      });
      if (stillPending.size === 0) {
        unsent.delete(messageKey);
      } else {
        schedule(messageKey);
      }
    }
  }

  function record(messageKey: string, blockID: string, change: LocalChange) {
    let messageChanges = changes.get(messageKey);
    if (!messageChanges) {
      messageChanges = new Map();
      changes.set(messageKey, messageChanges);
    }
    messageChanges.set(blockID, change);
    let pending = unsent.get(messageKey);
    if (!pending) {
      pending = new Set();
      unsent.set(messageKey, pending);
    }
    pending.add(blockID);
    schedule(messageKey);
    // A copy: a listener may unsubscribe while being notified.
    for (const listener of Array.from(listeners.get(listenerKey(messageKey, blockID)) ?? [])) {
      listener();
    }
  }

  return {
    bindingFor(messageKey, getServerState) {
      return {
        read: (blockID) => {
          const change = changes.get(messageKey)?.get(blockID);
          if (change) {
            return change.state;
          }
          const saved = getServerState()?.[blockID];
          return isUIBlockState(saved) ? saved : undefined;
        },
        write: (blockID, state, title) => {
          // Ids the server would reject stay local instead of failing the
          // write for every other block of the message.
          if (!isPersistableUIBlockName(blockID)) {
            return;
          }
          if (state !== undefined && !isUIBlockState(state)) {
            return;
          }
          record(messageKey, blockID, { state, title });
        },
        subscribe: (blockID, listener) => {
          const key = listenerKey(messageKey, blockID);
          let set = listeners.get(key);
          if (!set) {
            set = new Set();
            listeners.set(key, set);
          }
          set.add(listener);
          return () => {
            set.delete(listener);
            if (set.size === 0) {
              listeners.delete(key);
            }
          };
        },
      };
    },
    setTarget(messageKey, target) {
      if (!target) {
        targets.delete(messageKey);
        return;
      }
      const previous = targets.get(messageKey);
      if (previous?.conversationPublicID === target.conversationPublicID && previous.messagePublicID === target.messagePublicID) {
        return;
      }
      targets.set(messageKey, target);
      // Changes made while the message was streaming go out now that it has a row.
      if (unsent.get(messageKey)?.size) {
        schedule(messageKey);
      }
    },
    stateFor(messageKey, serverState) {
      const result = new Map<string, { state: UIBlockState; title?: string }>();
      for (const [blockID, state] of Object.entries(serverState ?? {})) {
        if (isPersistableUIBlockName(blockID) && isUIBlockState(state) && Object.keys(state).length > 0) {
          result.set(blockID, { state });
        }
      }
      changes.get(messageKey)?.forEach((change, blockID) => {
        if (!change.state || Object.keys(change.state).length === 0) {
          result.delete(blockID);
          return;
        }
        result.set(blockID, { state: change.state, title: change.title });
      });
      return result;
    },
    async flush({ keepalive = false } = {}) {
      const keys = Array.from(unsent.keys()).filter((key) => targets.has(key));
      for (const key of keys) {
        const timer = timers.get(key);
        if (timer) {
          clearTimeout(timer);
          timers.delete(key);
        }
      }
      // One request per message: a failure on one must not block the others.
      await Promise.all(keys.map((key) => writeNow(key, keepalive)));
    },
  };
}
