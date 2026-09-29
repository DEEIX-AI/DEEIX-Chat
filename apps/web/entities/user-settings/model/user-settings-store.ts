"use client";

import * as React from "react";

import {
  getUserSettings,
  patchUserSettings,
  type UserSettingsMap,
} from "@/shared/api/user-settings";
import { useAuthSession } from "@/shared/auth/auth-session-context";

export type UserSettingsSnapshot = {
  settings: UserSettingsMap;
  loaded: boolean;
};

type SettingValueSnapshot = {
  exists: boolean;
  value?: string;
};

type UserSettingsEntry = {
  snapshot: UserSettingsSnapshot;
  listeners: Set<() => void>;
  pendingLoad: Promise<UserSettingsMap> | null;
  pendingMutations: number;
  mutationSequence: number;
  keySequences: Map<string, number>;
  cleanupTimer: ReturnType<typeof setTimeout> | null;
};

const ENTRY_RETENTION_MS = 60_000;
const EMPTY_SETTINGS: UserSettingsMap = {};
const LOGGED_OUT_SNAPSHOT: UserSettingsSnapshot = {
  settings: EMPTY_SETTINGS,
  loaded: true,
};
const LOADING_SNAPSHOT: UserSettingsSnapshot = {
  settings: EMPTY_SETTINGS,
  loaded: false,
};
const entries = new Map<string, UserSettingsEntry>();

function normalizedAccessToken(accessToken: string | null | undefined): string {
  return accessToken?.trim() || "";
}

function createEntry(): UserSettingsEntry {
  return {
    snapshot: LOADING_SNAPSHOT,
    listeners: new Set(),
    pendingLoad: null,
    pendingMutations: 0,
    mutationSequence: 0,
    keySequences: new Map(),
    cleanupTimer: null,
  };
}

function getEntry(accessToken: string): UserSettingsEntry {
  const existing = entries.get(accessToken);
  if (existing) {
    return existing;
  }
  const entry = createEntry();
  entries.set(accessToken, entry);
  return entry;
}

function notify(entry: UserSettingsEntry) {
  for (const listener of entry.listeners) {
    listener();
  }
}

function cancelCleanup(entry: UserSettingsEntry) {
  if (entry.cleanupTimer) {
    clearTimeout(entry.cleanupTimer);
    entry.cleanupTimer = null;
  }
}

function scheduleCleanup(accessToken: string, entry: UserSettingsEntry) {
  if (
    entry.listeners.size > 0 ||
    entry.pendingLoad ||
    entry.pendingMutations > 0 ||
    entry.cleanupTimer
  ) {
    return;
  }
  entry.cleanupTimer = setTimeout(() => {
    entry.cleanupTimer = null;
    if (
      entry.listeners.size === 0 &&
      !entry.pendingLoad &&
      entry.pendingMutations === 0 &&
      entries.get(accessToken) === entry
    ) {
      entries.delete(accessToken);
    }
  }, ENTRY_RETENTION_MS);
}

function mergeLoadedSettings(
  entry: UserSettingsEntry,
  serverSettings: UserSettingsMap,
  mutationSequenceAtStart: number,
): UserSettingsMap {
  const next = { ...serverSettings };
  for (const [key, sequence] of entry.keySequences) {
    if (sequence > mutationSequenceAtStart && Object.hasOwn(entry.snapshot.settings, key)) {
      next[key] = entry.snapshot.settings[key];
    }
  }
  return next;
}

function mergeMutationResponse(
  entry: UserSettingsEntry,
  serverSettings: UserSettingsMap,
  requestSequences: ReadonlyMap<string, number>,
): UserSettingsMap {
  const next = { ...serverSettings };
  for (const [key, sequence] of entry.keySequences) {
    if (sequence === requestSequences.get(key)) {
      continue;
    }
    if (Object.hasOwn(entry.snapshot.settings, key)) {
      next[key] = entry.snapshot.settings[key];
    } else {
      delete next[key];
    }
  }
  return next;
}

export function readUserSettingsSnapshot(
  accessToken: string | null | undefined,
): UserSettingsSnapshot {
  const token = normalizedAccessToken(accessToken);
  return token ? entries.get(token)?.snapshot ?? LOADING_SNAPSHOT : LOGGED_OUT_SNAPSHOT;
}

export function loadUserSettingsSnapshot(
  accessToken: string,
  options: { refresh?: boolean } = {},
): Promise<UserSettingsMap> {
  const token = normalizedAccessToken(accessToken);
  if (!token) {
    return Promise.resolve(EMPTY_SETTINGS);
  }

  const entry = getEntry(token);
  cancelCleanup(entry);
  if (entry.pendingLoad) {
    return entry.pendingLoad;
  }
  if (entry.snapshot.loaded && !options.refresh) {
    return Promise.resolve(entry.snapshot.settings);
  }

  const mutationSequenceAtStart = entry.mutationSequence;
  const request = getUserSettings(token);
  const pendingLoad = request
    .then((serverSettings) => {
      if (entry.pendingLoad !== pendingLoad) {
        return entry.snapshot.settings;
      }
      entry.snapshot = {
        settings: mergeLoadedSettings(entry, serverSettings, mutationSequenceAtStart),
        loaded: true,
      };
      notify(entry);
      return entry.snapshot.settings;
    })
    .catch(() => {
      if (entry.pendingLoad !== pendingLoad) {
        return entry.snapshot.settings;
      }
      entry.snapshot = { ...entry.snapshot, loaded: true };
      notify(entry);
      return entry.snapshot.settings;
    })
    .finally(() => {
      if (entry.pendingLoad === pendingLoad) {
        entry.pendingLoad = null;
      }
      scheduleCleanup(token, entry);
    });
  entry.pendingLoad = pendingLoad;

  return pendingLoad;
}

export function subscribeUserSettings(
  accessToken: string | null | undefined,
  listener: () => void,
): () => void {
  const token = normalizedAccessToken(accessToken);
  if (!token) {
    return () => undefined;
  }

  const entry = getEntry(token);
  cancelCleanup(entry);
  entry.listeners.add(listener);
  void loadUserSettingsSnapshot(token);

  return () => {
    entry.listeners.delete(listener);
    scheduleCleanup(token, entry);
  };
}

export async function updateUserSettings(
  accessToken: string,
  changes: UserSettingsMap,
): Promise<UserSettingsMap> {
  const token = normalizedAccessToken(accessToken);
  if (!token) {
    throw new Error("missing access token");
  }

  const keys = Object.keys(changes);
  if (keys.length === 0) {
    return readUserSettingsSnapshot(token).settings;
  }

  const entry = getEntry(token);
  cancelCleanup(entry);
  entry.pendingMutations += 1;
  const previous = new Map<string, SettingValueSnapshot>();
  const requestSequences = new Map<string, number>();
  const optimisticSettings = { ...entry.snapshot.settings };

  for (const key of keys) {
    previous.set(key, {
      exists: Object.hasOwn(optimisticSettings, key),
      value: optimisticSettings[key],
    });
    const sequence = entry.mutationSequence + 1;
    entry.mutationSequence = sequence;
    entry.keySequences.set(key, sequence);
    requestSequences.set(key, sequence);
    optimisticSettings[key] = changes[key];
  }

  entry.snapshot = {
    settings: optimisticSettings,
    loaded: entry.snapshot.loaded,
  };
  notify(entry);

  try {
    const serverSettings = await patchUserSettings(token, changes);
    entry.snapshot = {
      settings: mergeMutationResponse(entry, serverSettings, requestSequences),
      loaded: true,
    };
    notify(entry);
    return entry.snapshot.settings;
  } catch (error) {
    const next = { ...entry.snapshot.settings };
    let rolledBack = false;
    for (const key of keys) {
      if (entry.keySequences.get(key) !== requestSequences.get(key)) {
        continue;
      }
      const prior = previous.get(key);
      if (prior?.exists) {
        next[key] = prior.value ?? "";
      } else {
        delete next[key];
      }
      rolledBack = true;
    }
    if (rolledBack) {
      entry.snapshot = { settings: next, loaded: entry.snapshot.loaded };
      notify(entry);
      throw error;
    }
    return entry.snapshot.settings;
  } finally {
    entry.pendingMutations -= 1;
    scheduleCleanup(token, entry);
  }
}

export function useUserSettings(): UserSettingsSnapshot {
  const { accessToken } = useAuthSession();
  const subscribe = React.useCallback(
    (listener: () => void) => subscribeUserSettings(accessToken, listener),
    [accessToken],
  );
  const getSnapshot = React.useCallback(
    () => readUserSettingsSnapshot(accessToken),
    [accessToken],
  );
  return React.useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}
