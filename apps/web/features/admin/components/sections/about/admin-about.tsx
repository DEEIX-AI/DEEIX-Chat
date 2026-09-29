"use client";

import { useSyncExternalStore } from "react";
import { useTranslations } from "next-intl";
import { CircleArrowUp, RefreshCw, Save } from "lucide-react";
import * as React from "react";

import packageMeta from "@/package.json";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogHeightTransition,
  DialogTitle,
} from "@/components/ui/dialog";
import { SettingsFieldEditor } from "@/features/admin/components/shared/settings-runtime-panel";
import { AdminUpdateTooltipContent } from "@/features/admin/components/shared/update-tooltip-content";
import { useAdminAboutDesktopDownload } from "@/features/admin/hooks/use-admin-about-desktop-download";
import {
  buildDesktopDownloadSettingsFields,
  desktopDownloadFieldID,
  toDesktopDownloadEditorField,
} from "@/features/admin/model/desktop-download-settings";
import {
  formatReleaseVersion,
  getCachedLatestReleaseSnapshot,
  getServerLatestReleaseSnapshot,
  resolveAvailableRelease,
  subscribeLatestReleaseChange,
  type ReleaseInfo,
} from "@/features/admin/model/update-check";
import { type AdminUpdateDialogState, useAdminAboutUpdateCheck } from "@/features/admin/hooks/use-admin-about-update-check";
import { IdentityProviderIcon } from "@/entities/identity-provider";
import { AboutSettingsContent } from "@/shared/components/about-settings-content";
import { SettingsFieldItem, SettingsFieldList, SettingsSection } from "@/shared/components/settings-layout";
import { useDialogSnapshot } from "@/shared/hooks/use-dialog-snapshot";
import { cn } from "@/lib/utils";

function AdminUpdateCheck() {
  const t = useTranslations("adminUsers.aboutPage");
  const { checking, dialogState, setDialogState, handleCheckUpdate } = useAdminAboutUpdateCheck();

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
  state: AdminUpdateDialogState | null;
  onOpenChange: (open: boolean) => void;
  onRetry: () => void;
}) {
  const t = useTranslations("adminUsers.aboutPage");
  const currentVersion = formatReleaseVersion(packageMeta.version);
  const stableState = useDialogSnapshot(state);
  const latestVersion = stableState?.type === "available" ? formatReleaseVersion(stableState.release.version) : "";

  return (
    <Dialog open={state !== null} onOpenChange={onOpenChange}>
      <DialogContent className="gap-0 overflow-hidden p-0 sm:max-w-[420px]">
        <DialogHeightTransition contentClassName="max-h-[min(86vh,760px)]">
          <DialogHeader className="shrink-0 px-4 py-4">
            <DialogTitle>
              {stableState?.type === "available"
                ? t("updateDialog.availableTitle")
                : stableState?.type === "failed"
                  ? t("updateDialog.failedTitle")
                  : t("updateDialog.currentTitle")}
            </DialogTitle>
            <DialogDescription>
              {stableState?.type === "available"
                ? t("updateDialog.availableDescription", { current: currentVersion, latest: latestVersion })
                : stableState?.type === "failed"
                  ? t("updateDialog.failedDescription")
                  : t("updateDialog.currentDescription", { current: currentVersion })}
            </DialogDescription>
          </DialogHeader>

          <div className="min-h-0 flex-1 overflow-y-auto px-4 py-2">
            {stableState?.type === "available" ? (
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
            {stableState?.type === "failed" ? (
              <Button type="button" onClick={onRetry}>
                {t("updateDialog.retry")}
              </Button>
            ) : null}
            {stableState?.type === "available" ? (
              <Button asChild type="button">
                <a href={stableState.release.url} target="_blank" rel="noopener noreferrer">
                  {t("updateDialog.openRelease")}
                </a>
              </Button>
            ) : null}
          </DialogFooter>
        </DialogHeightTransition>
      </DialogContent>
    </Dialog>
  );
}

function AdminAboutDesktopDownload() {
  const t = useTranslations("adminUsers.aboutPage");
  const commonT = useTranslations("common");
  const fields = React.useMemo(() => buildDesktopDownloadSettingsFields(t), [t]);
  const { loading, saving, settingsMap, savedMap, dirtyFieldIDs, updateValue, handleSave } = useAdminAboutDesktopDownload(fields);
  const enabled = settingsMap["desktop.download_enabled"] === "true";
  // The URL only matters while the entry is shown; keep it visible when it has unsaved edits.
  const visibleFields = fields.filter((field) => field.key !== "download_url" || enabled || dirtyFieldIDs.has(desktopDownloadFieldID(field)));

  return (
    <SettingsSection
      title={t("desktopDownload.title")}
      actions={dirtyFieldIDs.size > 0 ? (
        <Button type="button" size="sm" disabled={loading || saving} onClick={() => void handleSave()}>
          <Save className="size-3.5" />
          {commonT("actions.save")}
        </Button>
      ) : null}
    >
      <SettingsFieldList>
        {visibleFields.map((field, index) => {
          const id = desktopDownloadFieldID(field);
          return (
            <SettingsFieldItem key={id} index={index}>
              <SettingsFieldEditor
                field={toDesktopDownloadEditorField(field)}
                value={settingsMap[id] ?? ""}
                dirty={(settingsMap[id] ?? "") !== (savedMap[id] ?? "")}
                disabled={loading || saving}
                onChange={(value) => updateValue(field, value)}
              />
            </SettingsFieldItem>
          );
        })}
      </SettingsFieldList>
    </SettingsSection>
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
      brandIcon={IdentityProviderIcon}
      title={t("title")}
      description={t("description")}
      consoleLabel={t("adminConsole")}
      versionBadgeContent={<AdminAboutVersionBadge updateRelease={updateRelease} />}
      versionBadgeTooltip={<AdminUpdateTooltipContent updateRelease={updateRelease} />}
      versionActions={<AdminUpdateCheck />}
      extraSections={<AdminAboutDesktopDownload />}
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
