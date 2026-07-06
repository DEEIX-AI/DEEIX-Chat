const LAST_SELECTED_MODEL_STORAGE_KEY = "deeix-chat:last-selected-model:v1";

export function readLastSelectedModel(): string {
  if (typeof window === "undefined") {
    return "";
  }
  try {
    return window.localStorage.getItem(LAST_SELECTED_MODEL_STORAGE_KEY)?.trim() ?? "";
  } catch {
    // localStorage may be unavailable in private browsing or strict environments.
    return "";
  }
}

export function writeLastSelectedModel(platformModelName: string): void {
  if (typeof window === "undefined") {
    return;
  }
  const normalized = platformModelName.trim();
  if (!normalized) {
    return;
  }
  try {
    window.localStorage.setItem(LAST_SELECTED_MODEL_STORAGE_KEY, normalized);
  } catch {
    // localStorage may be unavailable in private browsing or strict environments.
  }
}
