"use client";

import { useTranslations } from "next-intl";

import packageMeta from "@/package.json";
import type { ReleaseInfo } from "@/features/admin/model/update-check";
import { formatReleaseVersion } from "@/features/admin/model/update-check";

export function AdminUpdateTooltipContent({ updateRelease }: { updateRelease: ReleaseInfo | null }) {
  const t = useTranslations("adminUsers.aboutPage");
  const currentVersion = formatReleaseVersion(packageMeta.version);

  if (!updateRelease) {
    return (
      <div className="space-y-1">
        <p>{t("updateTooltip.currentTitle")}</p>
        <p>{t("updateTooltip.currentVersion", { current: currentVersion })}</p>
      </div>
    );
  }

  return (
    <div className="space-y-1.5">
      <p>{t("updateTooltip.title")}</p>
      <div className="grid gap-1">
        <p>{t("updateTooltip.currentVersion", { current: currentVersion })}</p>
        <p>{t("updateTooltip.latestVersion", { latest: formatReleaseVersion(updateRelease.version) })}</p>
      </div>
      <p>{t("updateTooltip.description")}</p>
    </div>
  );
}
