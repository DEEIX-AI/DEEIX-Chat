import { isRecord, readString } from "@/shared/lib/type-guards";

export function isApplePlatform(): boolean {
  if (typeof navigator === "undefined") return false;
  // User-Agent Client Hints are Chromium-only and absent from lib.dom, so read them defensively.
  const userAgentData: unknown = "userAgentData" in navigator ? navigator.userAgentData : undefined;
  const candidates = [
    isRecord(userAgentData) ? readString(userAgentData, "platform") : undefined,
    navigator.platform,
    navigator.userAgent,
  ].filter((value): value is string => Boolean(value));
  return candidates.some((value) => /Mac|iPhone|iPad|iPod|Darwin/i.test(value)) || (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
}

export function platformModifierLabel(): "Command" | "Ctrl" {
  return isApplePlatform() ? "Command" : "Ctrl";
}

export function hasPlatformModifierKey(event: { ctrlKey: boolean; metaKey: boolean }): boolean {
  if (isApplePlatform()) {
    return event.metaKey && !event.ctrlKey;
  }
  return event.ctrlKey && !event.metaKey;
}

export function platformSendShortcut(): "ctrl_enter" | "meta_enter" {
  return isApplePlatform() ? "meta_enter" : "ctrl_enter";
}

/**
 * True while an IME is still composing, when Enter belongs to the candidate
 * list rather than to the field. `composing` covers the browsers that end the
 * composition before the Enter keydown reaches the handler; `Process` and
 * `keyCode` 229 are the older reports of the same state.
 */
export function isImeComposingKeyEvent(
  event: { key: string; keyCode?: number; nativeEvent?: { isComposing?: boolean } },
  composing = false,
): boolean {
  return composing || event.nativeEvent?.isComposing === true || event.key === "Process" || event.keyCode === 229;
}

export function isSendShortcutEvent(
  shortcut: "enter" | "ctrl_enter" | "meta_enter",
  event: {
    key: string;
    shiftKey: boolean;
    altKey: boolean;
    ctrlKey: boolean;
    metaKey: boolean;
  },
): boolean {
  if (event.key !== "Enter" || event.shiftKey || event.altKey) {
    return false;
  }
  if (shortcut === "enter") {
    return !event.ctrlKey && !event.metaKey;
  }
  return hasPlatformModifierKey(event);
}
