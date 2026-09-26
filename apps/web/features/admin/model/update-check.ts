export type ReleaseInfo = {
  version: string;
  url: string;
};

export type VersionCompareResult = "available" | "current" | "unknown";

export const LATEST_RELEASE_ENDPOINT = "https://api.github.com/repos/DEEIX-AI/DEEIX-Chat/releases/latest";
export const RELEASE_PAGE_PREFIX = "https://github.com/DEEIX-AI/DEEIX-Chat/releases/tag/";

const LATEST_RELEASE_CACHE_KEY = "deeix-chat:latest-release";
const LATEST_RELEASE_CHANGED_EVENT = "deeix-chat:latest-release-changed";

let cachedRawValue: string | null | undefined;
let cachedSnapshot: ReleaseInfo | null = null;

export function normalizeVersion(value: string | undefined): string {
  return value?.trim().replace(/^v/i, "").match(/\d+(?:\.\d+){0,2}/)?.[0] ?? "";
}

export function formatReleaseVersion(version: string): string {
  return version.startsWith("v") ? version : `v${version}`;
}

export function releasePageURL(version: string): string {
  return `${RELEASE_PAGE_PREFIX}${encodeURIComponent(formatReleaseVersion(version))}`;
}

export function compareReleaseVersions(currentVersion: string, latestVersion: string): VersionCompareResult {
  const current = normalizeVersion(currentVersion);
  const latest = normalizeVersion(latestVersion);

  if (!current || !latest) return "unknown";

  const currentParts = current.split(".").map((part) => Number.parseInt(part, 10));
  const latestParts = latest.split(".").map((part) => Number.parseInt(part, 10));
  const length = Math.max(currentParts.length, latestParts.length);

  for (let index = 0; index < length; index += 1) {
    const currentPart = currentParts[index] ?? 0;
    const latestPart = latestParts[index] ?? 0;

    if (!Number.isFinite(currentPart) || !Number.isFinite(latestPart)) return "unknown";
    if (latestPart > currentPart) return "available";
    if (latestPart < currentPart) return "current";
  }

  return "current";
}

export function resolveAvailableRelease(currentVersion: string, release: ReleaseInfo | null): ReleaseInfo | null {
  if (!release) return null;
  return compareReleaseVersions(currentVersion, release.version) === "available" ? release : null;
}

function parseReleaseSnapshot(raw: string | null): ReleaseInfo | null {
  if (!raw) return null;

  try {
    const parsed = JSON.parse(raw) as Partial<ReleaseInfo>;
    if (!parsed.version || !parsed.url) return null;
    return { version: parsed.version, url: parsed.url };
  } catch {
    return null;
  }
}

export function getCachedLatestReleaseSnapshot(): ReleaseInfo | null {
  if (typeof window === "undefined") return null;

  const raw = window.localStorage.getItem(LATEST_RELEASE_CACHE_KEY);
  if (raw === cachedRawValue) return cachedSnapshot;

  cachedRawValue = raw;
  cachedSnapshot = parseReleaseSnapshot(raw);
  return cachedSnapshot;
}

export function getServerLatestReleaseSnapshot(): ReleaseInfo | null {
  return null;
}

export function writeCachedLatestRelease(release: ReleaseInfo): void {
  if (typeof window === "undefined") return;

  const raw = JSON.stringify(release);
  try {
    window.localStorage.setItem(LATEST_RELEASE_CACHE_KEY, raw);
    cachedRawValue = raw;
    cachedSnapshot = release;
    window.dispatchEvent(new Event(LATEST_RELEASE_CHANGED_EVENT));
  } catch {
    // Ignore storage failures; update checking still works for the current session.
  }
}

export function subscribeLatestReleaseChange(onStoreChange: () => void): () => void {
  if (typeof window === "undefined") return () => {};

  const onStorage = (event: StorageEvent) => {
    if (event.key === LATEST_RELEASE_CACHE_KEY) onStoreChange();
  };

  window.addEventListener(LATEST_RELEASE_CHANGED_EVENT, onStoreChange);
  window.addEventListener("storage", onStorage);

  return () => {
    window.removeEventListener(LATEST_RELEASE_CHANGED_EVENT, onStoreChange);
    window.removeEventListener("storage", onStorage);
  };
}
