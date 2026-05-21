"use client";

import * as React from "react";

export const CHAT_FONT_STORAGE_KEY = "deeix-chat:chat-font";
export const CHAT_FONT_UPDATED_EVENT = "deeix-chat:chat-font-updated";
export const CHAT_FONT_WEIGHT_STORAGE_KEY = "deeix-chat:chat-font-weight";
export const CHAT_FONT_WEIGHT_UPDATED_EVENT = "deeix-chat:chat-font-weight-updated";

export type ChatFontOption = "default" | "songti" | "heiti" | "mono";
export type ChatFontWeightOption = "regular" | "medium" | "semibold" | "bold";

let currentChatFontPreference: ChatFontOption = "default";
let chatFontPreferenceLoaded = false;
let currentChatFontWeightPreference: ChatFontWeightOption = "regular";
let chatFontWeightPreferenceLoaded = false;

export function isChatFontOption(value: unknown): value is ChatFontOption {
  return value === "default" || value === "songti" || value === "heiti" || value === "mono";
}

export function isChatFontWeightOption(value: unknown): value is ChatFontWeightOption {
  return value === "regular" || value === "medium" || value === "semibold" || value === "bold";
}

function getStoredChatFontPreference(): ChatFontOption {
  if (typeof window === "undefined") {
    return "default";
  }

  const storedValue = window.localStorage.getItem(CHAT_FONT_STORAGE_KEY);
  return isChatFontOption(storedValue) ? storedValue : "default";
}

function getStoredChatFontWeightPreference(): ChatFontWeightOption {
  if (typeof window === "undefined") {
    return "regular";
  }

  const storedValue = window.localStorage.getItem(CHAT_FONT_WEIGHT_STORAGE_KEY);
  return isChatFontWeightOption(storedValue) ? storedValue : "regular";
}

export function readChatFontPreference(): ChatFontOption {
  if (!chatFontPreferenceLoaded) {
    currentChatFontPreference = getStoredChatFontPreference();
    chatFontPreferenceLoaded = true;
  }

  return currentChatFontPreference;
}

export function applyChatFontPreference(value: ChatFontOption) {
  if (typeof document === "undefined") {
    return;
  }

  if (value === "default") {
    delete document.documentElement.dataset.chatFont;
    return;
  }

  document.documentElement.dataset.chatFont = value;
}

export function readChatFontWeightPreference(): ChatFontWeightOption {
  if (!chatFontWeightPreferenceLoaded) {
    currentChatFontWeightPreference = getStoredChatFontWeightPreference();
    chatFontWeightPreferenceLoaded = true;
  }

  return currentChatFontWeightPreference;
}

export function applyChatFontWeightPreference(value: ChatFontWeightOption) {
  if (typeof document === "undefined") {
    return;
  }

  if (value === "regular") {
    delete document.documentElement.dataset.chatFontWeight;
    return;
  }

  document.documentElement.dataset.chatFontWeight = value;
}

function dispatchChatFontUpdated(value: ChatFontOption) {
  if (typeof window === "undefined") {
    return;
  }

  window.dispatchEvent(
    new CustomEvent<ChatFontOption>(CHAT_FONT_UPDATED_EVENT, {
      detail: value,
    }),
  );
}

export function writeChatFontPreference(value: ChatFontOption) {
  currentChatFontPreference = value;
  chatFontPreferenceLoaded = true;

  if (typeof window !== "undefined") {
    window.localStorage.setItem(CHAT_FONT_STORAGE_KEY, value);
  }

  applyChatFontPreference(value);
  dispatchChatFontUpdated(value);
}

function dispatchChatFontWeightUpdated(value: ChatFontWeightOption) {
  if (typeof window === "undefined") {
    return;
  }

  window.dispatchEvent(
    new CustomEvent<ChatFontWeightOption>(CHAT_FONT_WEIGHT_UPDATED_EVENT, {
      detail: value,
    }),
  );
}

export function writeChatFontWeightPreference(value: ChatFontWeightOption) {
  currentChatFontWeightPreference = value;
  chatFontWeightPreferenceLoaded = true;

  if (typeof window !== "undefined") {
    window.localStorage.setItem(CHAT_FONT_WEIGHT_STORAGE_KEY, value);
  }

  applyChatFontWeightPreference(value);
  dispatchChatFontWeightUpdated(value);
}

function subscribeChatFontPreference(onStoreChange: () => void) {
  if (typeof window === "undefined") {
    return () => undefined;
  }

  function handleStorage(event: StorageEvent) {
    if (event.key !== CHAT_FONT_STORAGE_KEY) {
      return;
    }

    currentChatFontPreference = isChatFontOption(event.newValue) ? event.newValue : "default";
    chatFontPreferenceLoaded = true;
    applyChatFontPreference(currentChatFontPreference);
    onStoreChange();
  }

  function handleChatFontUpdated() {
    onStoreChange();
  }

  window.addEventListener("storage", handleStorage);
  window.addEventListener(CHAT_FONT_UPDATED_EVENT, handleChatFontUpdated);

  return () => {
    window.removeEventListener("storage", handleStorage);
    window.removeEventListener(CHAT_FONT_UPDATED_EVENT, handleChatFontUpdated);
  };
}

function subscribeChatFontWeightPreference(onStoreChange: () => void) {
  if (typeof window === "undefined") {
    return () => undefined;
  }

  function handleStorage(event: StorageEvent) {
    if (event.key !== CHAT_FONT_WEIGHT_STORAGE_KEY) {
      return;
    }

    currentChatFontWeightPreference = isChatFontWeightOption(event.newValue) ? event.newValue : "regular";
    chatFontWeightPreferenceLoaded = true;
    applyChatFontWeightPreference(currentChatFontWeightPreference);
    onStoreChange();
  }

  function handleChatFontWeightUpdated() {
    onStoreChange();
  }

  window.addEventListener("storage", handleStorage);
  window.addEventListener(CHAT_FONT_WEIGHT_UPDATED_EVENT, handleChatFontWeightUpdated);

  return () => {
    window.removeEventListener("storage", handleStorage);
    window.removeEventListener(CHAT_FONT_WEIGHT_UPDATED_EVENT, handleChatFontWeightUpdated);
  };
}

export function useChatFontPreference() {
  return React.useSyncExternalStore(
    subscribeChatFontPreference,
    readChatFontPreference,
    (): ChatFontOption => "default",
  );
}

export function useChatFontWeightPreference() {
  return React.useSyncExternalStore(
    subscribeChatFontWeightPreference,
    readChatFontWeightPreference,
    (): ChatFontWeightOption => "regular",
  );
}
