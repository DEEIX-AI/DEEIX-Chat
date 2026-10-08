import type { ConversationOptions } from "@/shared/api/conversation-types";
import { isRecord } from "@/shared/lib/type-guards";

const RESERVED_CONVERSATION_OPTION_KEYS = new Set([
  "contents",
  "instructions",
  "input",
  "messages",
  "model",
  "prompt",
  "stream",
  "system",
  "systemInstruction",
]);

export function isConversationOptionsObject(value: unknown): value is ConversationOptions {
  return isRecord(value);
}

export function isReservedConversationOptionKey(key: string): boolean {
  return RESERVED_CONVERSATION_OPTION_KEYS.has(key);
}

export function sanitizeConversationOptions(options: ConversationOptions): ConversationOptions {
  return Object.fromEntries(
    Object.entries(options).filter(([key]) => !isReservedConversationOptionKey(key)),
  );
}

export function cloneConversationOptions(options: ConversationOptions): ConversationOptions {
  try {
    const cloned: unknown = JSON.parse(JSON.stringify(options));
    return sanitizeConversationOptions(isConversationOptionsObject(cloned) ? cloned : { ...options });
  } catch {
    return sanitizeConversationOptions({ ...options });
  }
}

/** Splits a dotted parameter path ("imageConfig.aspectRatio") into its segments. */
export function optionPathSegments(path: string): string[] {
  return path
    .split(".")
    .map((segment) => segment.trim())
    .filter(Boolean);
}

export function getOptionAtPath(options: ConversationOptions, path: string[]): unknown {
  let current: unknown = options;
  for (const segment of path) {
    if (!isRecord(current)) {
      return undefined;
    }
    current = current[segment];
  }
  return current;
}

export function hasOptionAtPath(options: ConversationOptions, path: string[]): boolean {
  let current: unknown = options;
  for (const segment of path) {
    if (!isRecord(current) || !Object.hasOwn(current, segment)) {
      return false;
    }
    current = current[segment];
  }
  return true;
}

export function setOptionAtPath(options: ConversationOptions, path: string[], value: unknown): ConversationOptions {
  if (path.length === 0) {
    return options;
  }
  const [segment, ...rest] = path;
  if (rest.length === 0) {
    return { ...options, [segment]: value };
  }
  const current = options[segment];
  return {
    ...options,
    [segment]: setOptionAtPath(isRecord(current) ? current : {}, rest, value),
  };
}

/** Removes the value at a path, along with parent objects it leaves empty. */
export function removeOptionAtPath(options: ConversationOptions, path: string[]): ConversationOptions {
  const [segment, ...rest] = path;
  if (segment === undefined || !Object.hasOwn(options, segment)) {
    return options;
  }
  const { [segment]: current, ...others } = options;
  if (rest.length === 0 || !isRecord(current)) {
    return others;
  }
  const child = removeOptionAtPath(current, rest);
  return Object.keys(child).length === 0 ? others : { ...others, [segment]: child };
}
