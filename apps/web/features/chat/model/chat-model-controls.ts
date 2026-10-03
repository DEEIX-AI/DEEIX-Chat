import {
  type ModelControl,
  type ModelControlValue,
  modelControlDefaultValue,
  normalizeModelControlValue,
} from "@/entities/model";
import type { ConversationOptions } from "@/shared/api/conversation-types";
import { isRecord } from "@/shared/lib/type-guards";

// Last control selections per model, remembered across conversations on this device. Only the
// user's explicit choices are stored; defaults are resolved from the current model controls, so a
// control the administrator removed (or whose value is no longer valid) silently disappears.
export type ChatModelControlSelections = Record<string, ModelControlValue>;

const STORAGE_PREFIX = "deeix-chat:chat-model-controls:";

function storageKey(platformModelName: string): string {
  return `${STORAGE_PREFIX}${encodeURIComponent(platformModelName)}`;
}

function parseSelections(value: unknown): ChatModelControlSelections {
  if (!isRecord(value)) return {};
  const result: ChatModelControlSelections = {};
  for (const [id, item] of Object.entries(value)) {
    if (typeof item === "string" || (typeof item === "number" && Number.isFinite(item))) {
      result[id] = item;
    }
  }
  return result;
}

export function readRememberedControlSelections(platformModelName: string): ChatModelControlSelections {
  if (typeof window === "undefined" || !platformModelName) return {};
  try {
    const raw = window.localStorage.getItem(storageKey(platformModelName));
    return raw ? parseSelections(JSON.parse(raw)) : {};
  } catch {
    return {};
  }
}

export function writeRememberedControlSelections(platformModelName: string, selections: ChatModelControlSelections): void {
  if (typeof window === "undefined" || !platformModelName) return;
  try {
    if (Object.keys(selections).length === 0) {
      window.localStorage.removeItem(storageKey(platformModelName));
    } else {
      window.localStorage.setItem(storageKey(platformModelName), JSON.stringify(selections));
    }
  } catch {
    // localStorage may be unavailable in private browsing or strict environments.
  }
}

/** Drops selections for controls that no longer exist, are locked, or no longer accept the value. */
export function pruneControlSelections(
  controls: readonly ModelControl[],
  selections: ChatModelControlSelections,
): ChatModelControlSelections {
  const result: ChatModelControlSelections = {};
  for (const control of controls) {
    if (control.locked || !(control.id in selections)) continue;
    const value = normalizeModelControlValue(control, selections[control.id]);
    if (value !== null) result[control.id] = value;
  }
  return result;
}

/** Effective value shown for a control: the user's selection, else the administrator default. */
export function resolveControlValue(
  control: ModelControl,
  selections: ChatModelControlSelections,
): ModelControlValue | null {
  if (!control.locked && control.id in selections) {
    const value = normalizeModelControlValue(control, selections[control.id]);
    if (value !== null) return value;
  }
  return modelControlDefaultValue(control);
}

/** True when the user's value differs from the default (used to highlight changed controls). */
export function isControlChanged(control: ModelControl, selections: ChatModelControlSelections): boolean {
  if (control.locked || !(control.id in selections)) return false;
  const value = normalizeModelControlValue(control, selections[control.id]);
  return value !== null && value !== modelControlDefaultValue(control);
}

/** Request payload: the user's valid choices; the server applies defaults for everything else. */
export function controlSelectionsForRequest(
  controls: readonly ModelControl[],
  selections: ChatModelControlSelections,
): ChatModelControlSelections | undefined {
  const pruned = pruneControlSelections(controls, selections);
  return Object.keys(pruned).length > 0 ? pruned : undefined;
}

// Client-only envelope key: the composer carries control selections inside its options object so
// queued submissions and "Regenerate with…" overrides keep them; the API layer moves them to the
// request's `controls` field and never sends this key upstream.
export const CHAT_CONTROLS_OPTION_KEY = "__controls";

export function withControlSelections(
  options: ConversationOptions,
  controls: ChatModelControlSelections | undefined,
): ConversationOptions {
  const { [CHAT_CONTROLS_OPTION_KEY]: _previous, ...rest } = options;
  return controls && Object.keys(controls).length > 0 ? { ...rest, [CHAT_CONTROLS_OPTION_KEY]: controls } : rest;
}

/** Splits the composer envelope into raw options and control selections for a request payload. */
export function splitControlSelections(options: ConversationOptions | undefined): {
  options: ConversationOptions | undefined;
  controls: ChatModelControlSelections | undefined;
} {
  if (!options) return { options: undefined, controls: undefined };
  const { [CHAT_CONTROLS_OPTION_KEY]: rawControls, ...rest } = options;
  const controls = parseSelections(rawControls);
  return {
    options: Object.keys(rest).length > 0 ? rest : undefined,
    controls: Object.keys(controls).length > 0 ? controls : undefined,
  };
}

/** Merges single-request overrides; control selections are merged per control instead of replaced. */
export function mergeChatOptionsOverride(
  options: ConversationOptions,
  override: ConversationOptions | undefined,
): ConversationOptions {
  if (!override) return options;
  const merged = { ...options, ...override };
  const baseControls = parseSelections(options[CHAT_CONTROLS_OPTION_KEY]);
  const overrideControls = parseSelections(override[CHAT_CONTROLS_OPTION_KEY]);
  if (Object.keys(baseControls).length > 0 || Object.keys(overrideControls).length > 0) {
    merged[CHAT_CONTROLS_OPTION_KEY] = { ...baseControls, ...overrideControls };
  }
  return merged;
}
