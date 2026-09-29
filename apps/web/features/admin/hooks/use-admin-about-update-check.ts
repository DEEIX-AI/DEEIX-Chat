import { useState } from "react";

import packageMeta from "@/package.json";
import {
  compareReleaseVersions,
  LATEST_RELEASE_ENDPOINT,
  type ReleaseInfo,
  writeCachedLatestRelease,
} from "@/features/admin/model/update-check";
import { fetchWithHeaderTimeout } from "@/shared/lib/fetch-timeout";
import { isRecord, isString } from "@/shared/lib/type-guards";

type GitHubRelease = {
  tag_name?: string;
  html_url?: string;
};

// GitHub can be slow or unreachable (e.g. behind a firewall); give up if it does not start responding.
const RELEASE_CHECK_TIMEOUT_MS = 10_000;

function isGitHubRelease(value: unknown): value is GitHubRelease {
  if (!isRecord(value)) return false;
  const { tag_name: tagName, html_url: htmlURL } = value;
  return (tagName === undefined || isString(tagName)) && (htmlURL === undefined || isString(htmlURL));
}

export type AdminUpdateDialogState =
  | { type: "current" }
  | { type: "available"; release: ReleaseInfo }
  | { type: "failed" };

/**
 * Manual "check for updates" against the latest GitHub release. The result is
 * cached (see update-check) so the sidebar badge updates too; any failure,
 * including a malformed payload, shows the "failed" dialog state.
 */
export function useAdminAboutUpdateCheck() {
  const [checking, setChecking] = useState(false);
  const [dialogState, setDialogState] = useState<AdminUpdateDialogState | null>(null);

  async function handleCheckUpdate() {
    if (checking) return;

    setChecking(true);
    try {
      const response = await fetchWithHeaderTimeout(LATEST_RELEASE_ENDPOINT, {
        cache: "no-store",
        headers: { Accept: "application/vnd.github+json" },
        timeoutMs: RELEASE_CHECK_TIMEOUT_MS,
      });

      if (!response.ok) {
        throw new Error(`Release check failed with HTTP ${response.status}`);
      }

      const release: unknown = await response.json();
      if (!isGitHubRelease(release)) {
        throw new Error("Latest release payload is malformed");
      }
      const latestVersion = release.tag_name?.trim();
      const releaseURL = release.html_url?.trim();

      if (!latestVersion || !releaseURL) {
        throw new Error("Latest release payload is incomplete");
      }

      const currentVersion = packageMeta.version;
      const compareResult = compareReleaseVersions(currentVersion, latestVersion);

      if (compareResult === "available" || compareResult === "unknown") {
        const release = { version: latestVersion, url: releaseURL };
        writeCachedLatestRelease(release);
        setDialogState({ type: "available", release });
        return;
      }

      writeCachedLatestRelease({ version: latestVersion, url: releaseURL });
      setDialogState({ type: "current" });
    } catch {
      setDialogState({ type: "failed" });
    } finally {
      setChecking(false);
    }
  }

  return { checking, dialogState, setDialogState, handleCheckUpdate };
}
