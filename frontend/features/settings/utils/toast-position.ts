"use client";

import * as React from "react";

// Device-level preference: where sonner places toasts. Values match sonner's
// `position` prop so no mapping is needed at the toaster.
export const TOAST_POSITION_STORAGE_KEY = "deeix-chat:toast-position";
export const TOAST_POSITION_UPDATED_EVENT = "deeix-chat:toast-position-updated";

export const TOAST_POSITIONS = ["top-left", "top-center", "top-right", "bottom-left", "bottom-center", "bottom-right"] as const;
export type ToastPosition = (typeof TOAST_POSITIONS)[number];
export const DEFAULT_TOAST_POSITION: ToastPosition = "top-right";

let currentToastPosition: ToastPosition = DEFAULT_TOAST_POSITION;
let toastPositionLoaded = false;

export function isToastPosition(value: unknown): value is ToastPosition {
  return typeof value === "string" && (TOAST_POSITIONS as readonly string[]).includes(value);
}

function getStoredToastPosition(): ToastPosition {
  if (typeof window === "undefined") {
    return DEFAULT_TOAST_POSITION;
  }
  try {
    const stored = window.localStorage.getItem(TOAST_POSITION_STORAGE_KEY);
    return isToastPosition(stored) ? stored : DEFAULT_TOAST_POSITION;
  } catch {
    return DEFAULT_TOAST_POSITION;
  }
}

export function readToastPosition(): ToastPosition {
  if (!toastPositionLoaded) {
    currentToastPosition = getStoredToastPosition();
    toastPositionLoaded = true;
  }
  return currentToastPosition;
}

export function writeToastPosition(value: ToastPosition) {
  currentToastPosition = value;
  toastPositionLoaded = true;
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(TOAST_POSITION_STORAGE_KEY, value);
  } catch {
    // localStorage may be unavailable in private browsing or strict environments.
  }
  window.dispatchEvent(new CustomEvent<ToastPosition>(TOAST_POSITION_UPDATED_EVENT, { detail: value }));
}

function subscribeToastPosition(onStoreChange: () => void) {
  if (typeof window === "undefined") {
    return (): void => undefined;
  }
  function handleStorage(event: StorageEvent) {
    if (event.key !== TOAST_POSITION_STORAGE_KEY) {
      return;
    }
    currentToastPosition = isToastPosition(event.newValue) ? event.newValue : DEFAULT_TOAST_POSITION;
    toastPositionLoaded = true;
    onStoreChange();
  }
  window.addEventListener("storage", handleStorage);
  window.addEventListener(TOAST_POSITION_UPDATED_EVENT, onStoreChange);
  return () => {
    window.removeEventListener("storage", handleStorage);
    window.removeEventListener(TOAST_POSITION_UPDATED_EVENT, onStoreChange);
  };
}

export function useToastPosition() {
  return React.useSyncExternalStore(subscribeToastPosition, readToastPosition, (): ToastPosition => DEFAULT_TOAST_POSITION);
}
