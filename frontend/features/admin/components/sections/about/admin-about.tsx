"use client";

import { useSyncExternalStore, useState } from "react";
import { useTranslations } from "next-intl";
import { CircleArrowUp, RefreshCw } from "lucide-react";

import packageMeta from "@/package.json";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { AdminUpdateTooltipContent } from "@/features/admin/components/admin-update-tooltip-content";
import {
  compareReleaseVersions,
  formatReleaseVersion,
  getCachedLatestReleaseSnapshot,
  getServerLatestReleaseSnapshot,
  LATEST_RELEASE_ENDPOINT,
  resolveAvailableRelease,
  subscribeLatestReleaseChange,
  type ReleaseInfo,
  writeCachedLatestRelease,
} from "@/features/admin/model/update-check";
import { AboutSettingsContent } from "@/shared/components/about-settings-content";
import { cn } from "@/lib/utils";

type GitHubRelease = {
  tag_name?: string;
  html_url?: string;
};

type UpdateDialogState =
  | { type: "current" }
  | { type: "available"; release: ReleaseInfo }
  | { type: "failed" };

function AdminUpdateCheck() {
  const t = useTranslations("adminUsers.aboutPage");
  const [checking, setChecking] = useState(false);
  const [dialogState, setDialogState] = useState<UpdateDialogState | null>(null);

  async function handleCheckUpdate() {
    if (checking) return;

    setChecking(true);
    try {
      const response = await fetch(LATEST_RELEASE_ENDPOINT, {
        cache: "no-store",
        headers: { Accept: "application/vnd.github+json" },
      });

      if (!response.ok) {
        throw new Error(`Release check failed with HTTP ${response.status}`);
      }

      const release = (await response.json()) as GitHubRelease;
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

  return (
    <>
      <button
        type="button"
        className="inline-flex items-center gap-1 text-xs text-muted-foreground/80 transition-colors hover:text-foreground disabled:cursor-wait disabled:opacity-70"
        onClick={() => void handleCheckUpdate()}
        disabled={checking}
      >
        <RefreshCw className={cn("size-3", checking && "animate-spin")} />
        <span>{checking ? t("checkingUpdate") : t("checkUpdate")}</span>
      </button>
      <UpdateResultDialog
        state={dialogState}
        onOpenChange={(open) => {
          if (!open) setDialogState(null);
        }}
        onRetry={() => void handleCheckUpdate()}
      />
    </>
  );
}

function AdminAboutVersionBadge({ updateRelease }: { updateRelease: ReleaseInfo | null }) {
  const t = useTranslations("adminUsers.aboutPage");
  const currentVersion = formatReleaseVersion(packageMeta.version);

  return (
    <span className="inline-flex items-center gap-1.5">
      <span>{currentVersion}</span>
      {updateRelease ? (
        <CircleArrowUp className="size-3.5 text-rose-500" aria-label={t("updateAvailableIndicator")} />
      ) : null}
    </span>
  );
}

function UpdateResultDialog({
  state,
  onOpenChange,
  onRetry,
}: {
  state: UpdateDialogState | null;
  onOpenChange: (open: boolean) => void;
  onRetry: () => void;
}) {
  const t = useTranslations("adminUsers.aboutPage");
  const currentVersion = formatReleaseVersion(packageMeta.version);
  const latestVersion = state?.type === "available" ? formatReleaseVersion(state.release.version) : "";

  return (
    <Dialog open={state !== null} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(86vh,760px)] flex-col gap-0 overflow-hidden p-0 sm:max-w-[420px]">
        <DialogHeader className="shrink-0 px-4 py-4">
          <DialogTitle>
            {state?.type === "available"
              ? t("updateDialog.availableTitle")
              : state?.type === "failed"
                ? t("updateDialog.failedTitle")
                : t("updateDialog.currentTitle")}
          </DialogTitle>
          <DialogDescription>
            {state?.type === "available"
              ? t("updateDialog.availableDescription", { current: currentVersion, latest: latestVersion })
              : state?.type === "failed"
                ? t("updateDialog.failedDescription")
                : t("updateDialog.currentDescription", { current: currentVersion })}
          </DialogDescription>
        </DialogHeader>

        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-2">
          {state?.type === "available" ? (
            <div className="rounded-md bg-muted/50 px-3 py-2 text-xs">
              <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
                <span className="text-muted-foreground">{t("updateDialog.currentVersion")}</span>
                <span className="font-medium">{currentVersion}</span>
                <span className="text-muted-foreground">{t("updateDialog.latestVersion")}</span>
                <span className="font-medium">{latestVersion}</span>
              </div>
            </div>
          ) : null}
        </div>

        <DialogFooter className="shrink-0 px-4 py-3">
          <DialogClose asChild>
            <Button type="button" variant="ghost">
              {t("updateDialog.close")}
            </Button>
          </DialogClose>
          {state?.type === "failed" ? (
            <Button type="button" onClick={onRetry}>
              {t("updateDialog.retry")}
            </Button>
          ) : null}
          {state?.type === "available" ? (
            <Button asChild type="button">
              <a href={state.release.url} target="_blank" rel="noopener noreferrer">
                {t("updateDialog.openRelease")}
              </a>
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function AdminAboutPage() {
  const t = useTranslations("adminUsers.aboutPage");
  const cachedLatestRelease = useSyncExternalStore(
    subscribeLatestReleaseChange,
    getCachedLatestReleaseSnapshot,
    getServerLatestReleaseSnapshot,
  );
  const updateRelease = resolveAvailableRelease(packageMeta.version, cachedLatestRelease);

  return (
    <AboutSettingsContent
      title={t("title")}
      description={t("description")}
      consoleLabel={t("adminConsole")}
      versionBadgeContent={<AdminAboutVersionBadge updateRelease={updateRelease} />}
      versionBadgeTooltip={<AdminUpdateTooltipContent updateRelease={updateRelease} />}
      versionActions={<AdminUpdateCheck />}
      labels={{
        details: t("details"),
        official: t("official"),
        website: t("website"),
        repository: t("repository"),
        social: t("social"),
        blog: t("blog"),
        contact: t("contact"),
        copyright: t("copyright"),
        license: t("license"),
      }}
    />
  );
}
