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
