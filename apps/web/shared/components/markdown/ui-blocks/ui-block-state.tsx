"use client";

import { readMessageUIBlocks, UI_BLOCK_FENCE_LANGUAGE } from "@deeix/core";
import * as React from "react";

import { isRecord } from "@/shared/lib/type-guards";

// Persisted per-block state for interactive UI blocks.
//
// A block owns its React state; the host stores only what the user changed — a
// diff against the block's initial values — so reopening a conversation shows
// the same view, and the next turn can tell the model what the user set. The
// host hands a binding down through `UIBlockStateProvider`. Without one (share
// pages, previews) blocks keep plain local state and nothing is stored.

type UIBlockStateValue =
  | string
  | number
  | boolean
  | null
  | UIBlockStateValue[]
  | { [key: string]: UIBlockStateValue };

export type UIBlockState = Record<string, UIBlockStateValue>;

export type UIBlockStateBinding = {
  // The latest state written through this binding (by any view of the block),
  // falling back to the saved snapshot. Writes are skipped when they equal it.
  read: (blockID: string) => UIBlockState | undefined;
  // `state` undefined clears the block. `title` is the block's own heading, used
  // to tell the model which component a state belongs to.
  write: (blockID: string, state: UIBlockState | undefined, title?: string) => void;
  // Called after any write to the block, from this view or another one showing
  // the same block (the side panel); readers then re-read. Optional: bindings
  // without other views need not support it.
  subscribe?: (blockID: string, listener: () => void) => () => void;
};

type UIBlockStateSpec<T extends object> = {
  // Initial state; the stored diff is computed against it.
  defaults: T;
  // Rebuilds the block's own shape from untrusted stored state: unknown keys,
  // out-of-range values and entries the current props no longer describe are
  // dropped. Called with `undefined` to produce the initial state.
  restore: (saved: UIBlockState | undefined, defaults: T) => T;
  title?: string;
};

// Limits shared with the server (backend/internal/application/conversation/
// service_message_ui_state.go). A block that breaks them stays local instead of
// failing the write for every other block of its message.
const NAME_PATTERN = /^[A-Za-z0-9_-]+$/;
const NAME_MAX_LENGTH = 64;
const STATE_MAX_BYTES = 4096;
const STATE_MAX_DEPTH = 4;

const UIBlockStateContext = React.createContext<UIBlockStateBinding | null>(null);

export function UIBlockStateProvider({ binding, children }: { binding: UIBlockStateBinding; children: React.ReactNode }) {
  return <UIBlockStateContext.Provider value={binding}>{children}</UIBlockStateContext.Provider>;
}

// Blocks below keep plain local state: catalog components nested in a live
// block (their state is part of the block's view, not a block of its own).
export function UIBlockLocalState({ children }: { children: React.ReactNode }) {
  return <UIBlockStateContext.Provider value={null}>{children}</UIBlockStateContext.Provider>;
}

// A message's binding, minus the keys used by more than one of its blocks:
// those blocks cannot be told apart, so they keep local state rather than
// overwrite each other. Stable while the set of repeated keys is unchanged.
export function useMessageUIBlockStateBinding(binding: UIBlockStateBinding | undefined, content: string): UIBlockStateBinding | undefined {
  const repeatedKeys = React.useMemo(() => {
    if (!binding || !content.includes(UI_BLOCK_FENCE_LANGUAGE)) {
      return "";
    }
    return Array.from(readMessageUIBlocks(content).duplicates).sort().join("\n");
  }, [binding, content]);
  return React.useMemo(() => {
    if (!binding || !repeatedKeys) {
      return binding;
    }
    const repeated = new Set(repeatedKeys.split("\n"));
    const subscribe = binding.subscribe;
    return {
      read: (blockID) => (repeated.has(blockID) ? undefined : binding.read(blockID)),
      write: (blockID, state, title) => {
        if (!repeated.has(blockID)) {
          binding.write(blockID, state, title);
        }
      },
      subscribe: subscribe ? (blockID, listener) => (repeated.has(blockID) ? () => {} : subscribe(blockID, listener)) : undefined,
    };
  }, [binding, repeatedKeys]);
}

// The restored initial state plus a `save` that stores the diff of a later
// state. For blocks whose working state has a richer shape than what is stored
// (row identities, animation state): they call `save` after a user change.
export function useUIBlockStatePersistence<T extends object>(id: string, spec: UIBlockStateSpec<T>): { initial: T; save: (next: T) => void } {
  const binding = React.useContext(UIBlockStateContext);
  const [initial] = React.useState<T>(() => spec.restore(binding?.read(id), spec.defaults));

  // Props can change after mount (the message streams in or is edited); the
  // diff is always taken against the latest defaults.
  const specRef = React.useRef(spec);
  specRef.current = spec;

  const save = React.useCallback(
    (next: T) => {
      if (!binding) {
        return;
      }
      const { defaults, title } = specRef.current;
      const diff = sanitizeUIBlockState(diffState(next, defaults));
      // Another view of the block may have written this already.
      if (deepEquals(diff, binding.read(id))) {
        return;
      }
      binding.write(id, diff, title);
    },
    [binding, id],
  );

  return { initial, save };
}

// Calls `onChange` with the stored state whenever the block is written, from
// this view or another one, so two views of one block stay in step.
export function useUIBlockStateChanges(id: string, onChange: (saved: UIBlockState | undefined) => void): void {
  const binding = React.useContext(UIBlockStateContext);
  const onChangeRef = React.useRef(onChange);
  onChangeRef.current = onChange;
  React.useEffect(() => {
    if (!binding?.subscribe) {
      return undefined;
    }
    return binding.subscribe(id, () => onChangeRef.current(binding.read(id)));
  }, [binding, id]);
}

// Block state with a setter that takes a partial patch, a functional update, or
// `null` to reset to the defaults. Changes are stored as a diff; the first
// write happens only after the user changes something.
export function useUIBlockState<T extends object>(id: string, spec: UIBlockStateSpec<T>): [T, (next: Partial<T> | ((current: T) => T) | null) => void] {
  const { initial, save } = useUIBlockStatePersistence(id, spec);
  const [state, setState] = React.useState<T>(initial);
  const changedRef = React.useRef(false);
  const specRef = React.useRef(spec);
  specRef.current = spec;

  React.useEffect(() => {
    if (changedRef.current) {
      save(state);
    }
  }, [save, state]);

  useUIBlockStateChanges(id, (saved) => {
    const next = specRef.current.restore(saved, specRef.current.defaults);
    setState((current) => (deepEquals(current, next) ? current : next));
  });

  const update = React.useCallback((next: Partial<T> | ((current: T) => T) | null) => {
    changedRef.current = true;
    setState((current) => {
      if (next === null) {
        return specRef.current.restore(undefined, specRef.current.defaults);
      }
      return typeof next === "function" ? next(current) : { ...current, ...next };
    });
  }, []);

  return [state, update];
}

// diffState returns the keys whose value differs from `defaults`, or undefined
// when none does — the host then clears the stored block.
function diffState<T extends object>(state: T, defaults: T): UIBlockState | undefined {
  const base = new Map<string, unknown>(Object.entries(defaults));
  const diff: UIBlockState = {};
  for (const [key, value] of Object.entries(state)) {
    if (!deepEquals(value, base.get(key)) && isStateValue(value)) {
      setOwn(diff, key, value);
    }
  }
  return Object.keys(diff).length > 0 ? diff : undefined;
}

// Drops keys the server would reject and the whole block when it is too large
// or too deep; the block then simply is not stored.
export function sanitizeUIBlockState(state: UIBlockState | undefined): UIBlockState | undefined {
  if (!state) {
    return undefined;
  }
  const sanitized: UIBlockState = {};
  for (const [key, value] of Object.entries(state)) {
    if (isPersistableUIBlockName(key) && stateDepth(value, 1) <= STATE_MAX_DEPTH) {
      setOwn(sanitized, key, value);
    }
  }
  if (Object.keys(sanitized).length === 0) {
    return undefined;
  }
  return new TextEncoder().encode(JSON.stringify(sanitized)).length <= STATE_MAX_BYTES ? sanitized : undefined;
}

export function isPersistableUIBlockName(value: string): boolean {
  return value.length > 0 && value.length <= NAME_MAX_LENGTH && NAME_PATTERN.test(value);
}

// Narrows an untrusted value (a parsed JSON payload) into block state.
export function isUIBlockState(value: unknown): value is UIBlockState {
  return isStateRecord(value);
}

export function deepEquals(left: unknown, right: unknown): boolean {
  if (left === right) {
    return true;
  }
  if (Array.isArray(left) || Array.isArray(right)) {
    return Array.isArray(left) && Array.isArray(right) && left.length === right.length && left.every((item, index) => deepEquals(item, right[index]));
  }
  if (isRecord(left) && isRecord(right)) {
    const leftEntries = Object.entries(left);
    return (
      leftEntries.length === Object.keys(right).length &&
      leftEntries.every(([key, value]) => Object.hasOwn(right, key) && deepEquals(value, right[key]))
    );
  }
  return false;
}

// State keys of live blocks come from model output; "__proto__" must become a
// field, not replace the prototype.
function setOwn(target: UIBlockState, key: string, value: UIBlockStateValue): void {
  Object.defineProperty(target, key, { value, enumerable: true, writable: true, configurable: true });
}

function isStateRecord(value: unknown): value is Record<string, UIBlockStateValue> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return false;
  }
  return Object.values(value).every(isStateValue);
}

function isStateValue(value: unknown): value is UIBlockStateValue {
  if (value === null) {
    return true;
  }
  switch (typeof value) {
    case "string":
    case "boolean":
      return true;
    case "number":
      return Number.isFinite(value);
    case "object":
      return Array.isArray(value) ? value.every(isStateValue) : isStateRecord(value);
    default:
      return false;
  }
}

function stateDepth(value: UIBlockStateValue, depth: number): number {
  if (Array.isArray(value)) {
    return value.reduce<number>((deepest, item) => Math.max(deepest, stateDepth(item, depth + 1)), depth);
  }
  if (value && typeof value === "object") {
    return Object.values(value).reduce<number>((deepest, item) => Math.max(deepest, stateDepth(item, depth + 1)), depth);
  }
  return depth;
}
