"use client";

import { getBrandingSnapshot } from "@/shared/config/branding";
import { normalizeTrimmedString } from "@/shared/lib/string";
import { showDesktopNotification, usesDesktopNotifications } from "@/shared/platform/desktop-notifications";

const RESPONSE_COMPLETION_NOTIFICATIONS_STORAGE_KEY = "deeix-chat:response-completion-notifications";
const NOTIFICATION_BODY_MAX_LENGTH = 140;

type ResponseCompletionNotificationInput = {
  content?: string | null;
  conversationPublicID?: string | null;
  conversationTitle?: string | null;
};

function normalizeNotificationBody(value: string) {
  const compactValue = value.replace(/\s+/g, " ").trim();
  if (!compactValue) {
    return "Your reply is ready.";
  }
  if (compactValue.length <= NOTIFICATION_BODY_MAX_LENGTH) {
    return compactValue;
  }
  return `${compactValue.slice(0, NOTIFICATION_BODY_MAX_LENGTH - 1)}…`;
}

function isWebNotificationSupported() {
  return typeof window !== "undefined" && "Notification" in window;
}

// Browsers use the Web Notifications API and its per-site permission; the desktop shell
// shows native notifications and has no permission to ask for.
export function isSystemNotificationSupported() {
  return usesDesktopNotifications() || isWebNotificationSupported();
}

export function getSystemNotificationPermission(): NotificationPermission | "unsupported" {
  if (usesDesktopNotifications()) {
    return "granted";
  }
  if (!isWebNotificationSupported()) {
    return "unsupported";
  }
  return Notification.permission;
}

export function readResponseCompletionNotificationsEnabled() {
  if (typeof window === "undefined") {
    return false;
  }
  return window.localStorage.getItem(RESPONSE_COMPLETION_NOTIFICATIONS_STORAGE_KEY) === "true";
}

export function writeResponseCompletionNotificationsEnabled(enabled: boolean) {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(RESPONSE_COMPLETION_NOTIFICATIONS_STORAGE_KEY, String(enabled));
}

export async function enableResponseCompletionNotifications() {
  if (usesDesktopNotifications()) {
    writeResponseCompletionNotificationsEnabled(true);
    return {
      enabled: true,
      permission: "granted" as const,
    };
  }
  if (!isWebNotificationSupported()) {
    writeResponseCompletionNotificationsEnabled(false);
    return {
      enabled: false,
      permission: "unsupported" as const,
    };
  }

  let permission = Notification.permission;
  if (permission !== "granted") {
    permission = await Notification.requestPermission();
  }

  const enabled = permission === "granted";
  writeResponseCompletionNotificationsEnabled(enabled);

  return {
    enabled,
    permission,
  };
}

export function disableResponseCompletionNotifications() {
  writeResponseCompletionNotificationsEnabled(false);
}

export function notifyResponseCompletion(input: ResponseCompletionNotificationInput) {
  if (!readResponseCompletionNotificationsEnabled() || getSystemNotificationPermission() !== "granted") {
    return false;
  }
  if (typeof document !== "undefined" && document.visibilityState === "visible" && document.hasFocus()) {
    return false;
  }

  const conversationTitle = normalizeTrimmedString(input.conversationTitle);
  const branding = getBrandingSnapshot();
  const title = conversationTitle || branding.title;
  const body = normalizeNotificationBody(normalizeTrimmedString(input.content));
  if (usesDesktopNotifications()) {
    showDesktopNotification(title, body);
    return true;
  }

  const notification = new Notification(title, {
    body,
    tag: normalizeTrimmedString(input.conversationPublicID, `response-completion:${Date.now()}`),
    icon: branding.pwaIcon192URL,
  });

  notification.onclick = () => {
    if (typeof window !== "undefined") {
      void window.focus();
    }
    notification.close();
  };

  return true;
}
