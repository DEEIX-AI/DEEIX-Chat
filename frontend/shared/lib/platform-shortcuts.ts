export function isApplePlatform(): boolean {
  if (typeof navigator === "undefined") return false;
  const nav = navigator as Navigator & { userAgentData?: { platform?: string } };
  const candidates = [
    nav.userAgentData?.platform,
    navigator.platform,
    navigator.userAgent,
  ].filter(Boolean);
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
