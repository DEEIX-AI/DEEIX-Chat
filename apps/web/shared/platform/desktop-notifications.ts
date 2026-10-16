"use client";

import { sendNotification } from "@tauri-apps/plugin-notification";

import { isDesktopApp } from "./runtime";

/**
 * True when notifications go through the desktop shell. Its webviews do not deliver Web
 * Notifications, so the shell shows native ones. There is no web permission to ask for:
 * whether they appear is the app's notification setting in the operating system.
 */
export function usesDesktopNotifications(): boolean {
  return isDesktopApp();
}

/** Show a native notification; the operating system uses the app icon. */
export function showDesktopNotification(title: string, body: string): void {
  sendNotification({ title, body });
}
